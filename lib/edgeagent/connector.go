// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package edgeagent

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log/slog"
	"sync/atomic"

	"github.com/coreos/go-semver/semver"
	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
	apiclient "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	apisshutils "github.com/gravitational/teleport/api/utils/sshutils"
	"github.com/gravitational/teleport/lib"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/auth/state"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/join/joinclient"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

// Connector holds credentials and an authenticated auth client. Copied from
// lib/service.Connector (WATCH: lib/service/service.go).
type Connector struct {
	clusterName string
	hostID      string
	role        types.SystemRole

	clientState atomic.Pointer[connectorState]
	serverState atomic.Pointer[connectorState]

	Client *authclient.Client
}

type connectorState struct {
	identity     *state.Identity
	tlsCert      *tls.Certificate
	pool         *x509.CertPool
	sshCert      *ssh.Certificate
	sshCertSigner ssh.Signer
	hostCheckers []ssh.PublicKey
}

func newConnector(clientIdentity, serverIdentity *state.Identity) (*Connector, error) {
	clientState, err := newConnectorState(clientIdentity)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	serverState := clientState
	if serverIdentity != clientIdentity {
		s, err := newConnectorState(serverIdentity)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		serverState = s
	}
	c := &Connector{
		clusterName: clientIdentity.ClusterName,
		hostID:      clientIdentity.ID.HostUUID,
		role:        clientIdentity.ID.Role,
	}
	c.clientState.Store(clientState)
	c.serverState.Store(serverState)
	return c, nil
}

func newConnectorState(identity *state.Identity) (*connectorState, error) {
	cs := &connectorState{identity: identity}
	if identity.Cert != nil {
		hostCheckers, err := apisshutils.ParseAuthorizedKeys(identity.SSHCACertBytes)
		if err != nil {
			return nil, trace.Wrap(err, "parsing SSH host CAs")
		}
		cs.hostCheckers = hostCheckers
		cs.sshCert = identity.Cert
		cs.sshCertSigner = identity.KeySigner
	}
	if identity.HasTLSConfig() {
		tlsCert, err := keys.X509KeyPair(identity.TLSCertBytes, identity.KeyBytes)
		if err != nil {
			return nil, trace.Wrap(err, "parsing X.509 certificate")
		}
		tlsCert.Leaf = identity.XCert
		certPool := x509.NewCertPool()
		for j := range identity.TLSCACertsBytes {
			parsedCert, err := tlsca.ParseCertificatePEM(identity.TLSCACertsBytes[j])
			if err != nil {
				return nil, trace.Wrap(err, "parsing X.509 host CA")
			}
			certPool.AddCert(parsedCert)
		}
		cs.tlsCert = &tlsCert
		cs.pool = certPool
	}
	return cs, nil
}

func (c *Connector) ClusterName() string { return c.clusterName }

func (c *Connector) HostID() string { return c.hostID }

func (c *Connector) HostUUID() string {
	return c.clientState.Load().identity.ID.HostID()
}

func (c *Connector) Role() types.SystemRole { return c.role }

func (c *Connector) ClientGetCertificate() (*tls.Certificate, error) {
	tlsCert := c.clientState.Load().tlsCert
	if tlsCert == nil {
		return nil, trace.NotFound("no TLS credentials setup for this identity")
	}
	return tlsCert, nil
}

func (c *Connector) ClientGetPool() (*x509.CertPool, error) {
	roots := c.clientState.Load().pool
	if roots == nil {
		return nil, trace.NotFound("no TLS credentials setup for this identity")
	}
	return roots, nil
}

func (c *Connector) ClientAuthMethods() []ssh.AuthMethod {
	return []ssh.AuthMethod{
		ssh.PublicKeysCallback(func() (signers []ssh.Signer, err error) {
			// Reverse tunnels authenticate as the Node identity. Instance
			// certs are rejected by the proxy ("unsupported role Instance").
			st := c.serverState.Load()
			if st == nil || st.sshCertSigner == nil {
				st = c.clientState.Load()
			}
			if st == nil || st.sshCertSigner == nil {
				return nil, nil
			}
			return []ssh.Signer{st.sshCertSigner}, nil
		}),
	}
}

func (c *Connector) ServerGetHostSigners() []ssh.Signer {
	sshCertSigner := c.serverState.Load().sshCertSigner
	if sshCertSigner == nil {
		return nil
	}
	return []ssh.Signer{sshCertSigner}
}

func (c *Connector) clientSSHClientConfig() (*ssh.ClientConfig, error) {
	hostKeyCallback, err := apisshutils.NewHostKeyCallback(apisshutils.HostKeyCallbackConfig{
		GetHostCheckers: func() ([]ssh.PublicKey, error) {
			return c.clientState.Load().hostCheckers, nil
		},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &ssh.ClientConfig{
		User: c.hostID,
		Auth: []ssh.AuthMethod{ssh.PublicKeysCallback(func() (signers []ssh.Signer, err error) {
			return []ssh.Signer{c.clientState.Load().sshCertSigner}, nil
		})},
		HostKeyCallback: hostKeyCallback,
		Timeout:         apidefaults.DefaultIOTimeout,
	}, nil
}

func (c *Connector) TunnelProxyResolver() reversetunnelclient.Resolver {
	if c.Client == nil || c.Client.Dialer() == nil {
		return nil
	}
	switch dialer := c.Client.Dialer().(type) {
	case *reversetunnelclient.TunnelAuthDialer:
		return dialer.Resolver
	default:
		return nil
	}
}

func (c *Connector) Close() error {
	if c.Client != nil {
		return c.Client.Close()
	}
	return nil
}

func (p *Process) join(ctx context.Context) (*state.Identity, error) {
	id := state.IdentityID{
		Role:     types.RoleInstance,
		NodeName: p.cfg.Teleport.Nodename,
	}
	proxyAddr, err := utils.ParseAddr(p.cfg.Teleport.ProxyServer)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	joinParams := joinclient.JoinParams{
		Token:       p.cfg.Token(),
		ID:          id,
		ProxyServer: *proxyAddr,
		JoinMethod:  types.JoinMethodToken,
		Insecure:    p.cfg.Teleport.Insecure || lib.IsInsecureDevMode(),
		CAPins:      []string(p.cfg.Teleport.CAPin),
		Log:         p.logger,
	}
	p.logger.InfoContext(ctx, "Joining the cluster with a secure token.")
	result, err := joinclient.Join(ctx, joinParams)
	if err != nil {
		if utils.IsUntrustedCertErr(err) {
			return nil, trace.WrapWithMessage(err, utils.SelfSignedCertsMsg)
		}
		return nil, trace.Wrap(err)
	}
	privateKeyPEM, err := keys.MarshalPrivateKey(result.PrivateKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	identity, err := state.ReadIdentityFromKeyPair(privateKeyPEM, result.Certs)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	identity.ImmutableLabels = result.ImmutableLabels
	return identity, nil
}

func (p *Process) reRegister(ctx context.Context, clt *authclient.Client, current *state.Identity, role types.SystemRole, rotation types.Rotation) (*state.Identity, error) {
	key, err := cryptosuites.GenerateKey(ctx, func(ctx context.Context) (types.SignatureAlgorithmSuite, error) {
		pr, err := clt.Ping(ctx)
		if err != nil {
			return types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_UNSPECIFIED, trace.Wrap(err, "pinging auth to determine signature algorithm suite")
		}
		return pr.GetSignatureAlgorithmSuite(), nil
	}, cryptosuites.HostIdentity)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	privateKeyPEM, err := keys.MarshalPrivateKey(key)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sshPub, err := ssh.NewPublicKey(key.Public())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tlsPub, err := keys.MarshalPublicKey(key.Public())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var rot *types.Rotation
	if !rotation.IsZero() {
		rot = &rotation
	}
	req := &proto.HostCertsRequest{
		HostID:               current.ID.HostID(),
		NodeName:             p.cfg.Teleport.Nodename,
		Role:                 role,
		AdditionalPrincipals: []string{p.cfg.Teleport.Nodename},
		PublicTLSKey:         tlsPub,
		PublicSSHKey:         ssh.MarshalAuthorizedKey(sshPub),
		Rotation:             rot,
	}
	// Additional system roles may only be encoded on Instance certs. A Node
	// cert request with SystemRoles set is rejected by auth.
	if role == types.RoleInstance {
		req.SystemRoles = []types.SystemRole{types.RoleNode}
	}
	certs, err := clt.GenerateHostCerts(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	identity, err := state.ReadIdentityFromKeyPair(privateKeyPEM, certs)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	identity.ImmutableLabels = current.ImmutableLabels
	return identity, nil
}

func (p *Process) newClientThroughTunnel(ctx context.Context, conn *Connector) (*authclient.Client, *proto.PingResponse, error) {
	tlsConfig := utils.TLSConfig(nil)
	tlsConfig.GetClientCertificate = func(*tls.CertificateRequestInfo) (*tls.Certificate, error) {
		return conn.ClientGetCertificate()
	}
	tlsConfig.InsecureSkipVerify = true
	tlsConfig.VerifyConnection = utils.VerifyConnectionWithRoots(conn.ClientGetPool)

	sshClientConfig, err := conn.clientSSHClientConfig()
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	dialer, err := reversetunnelclient.NewTunnelAuthDialer(reversetunnelclient.TunnelAuthDialerConfig{
		Resolver:              p.resolver,
		ClientConfig:          sshClientConfig,
		Log:                   p.logger,
		InsecureSkipTLSVerify: p.cfg.Teleport.Insecure || lib.IsInsecureDevMode(),
		GetClusterCAs: func(context.Context) (*x509.CertPool, error) {
			return conn.ClientGetPool()
		},
	})
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	clt, err := authclient.NewClient(apiclient.Config{
		Context:         ctx,
		Dialer:          dialer,
		KeepAlivePeriod: edgeKeepAliveInterval,
		KeepAliveCount:  edgeKeepAliveCount,
		Credentials: []apiclient.Credentials{
			apiclient.LoadTLS(tlsConfig),
		},
	})
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	pingCtx, cancel := context.WithTimeout(ctx, apidefaults.DefaultIOTimeout)
	defer cancel()
	resp, err := clt.Ping(pingCtx)
	if err != nil {
		return nil, nil, trace.NewAggregate(err, clt.Close())
	}
	if err := authServerTooOld(&resp); err != nil {
		return nil, nil, trace.NewAggregate(err, clt.Close())
	}
	return clt, &resp, nil
}

func authServerTooOld(resp *proto.PingResponse) error {
	serverVersion, err := semver.NewVersion(resp.ServerVersion)
	if err != nil {
		return trace.BadParameter("failed to parse reported auth server version as semver: %v", err)
	}
	teleportVersion, err := semver.NewVersion(teleport.Version)
	if err != nil {
		return trace.BadParameter("failed to parse local teleport version as semver: %v", err)
	}
	if serverVersion.Major < teleportVersion.Major {
		return trace.Errorf("Teleport instance is too new. This instance is running v%d. The auth server is running v%d and only supports instances on v%d or v%d.", teleportVersion.Major, serverVersion.Major, serverVersion.Major, serverVersion.Major-1)
	}
	return nil
}

func (p *Process) connect(ctx context.Context) (*Connector, error) {
	instanceIdentity, err := p.storage.ReadIdentity(ctx, state.IdentityCurrent, types.RoleInstance)
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}
	if trace.IsNotFound(err) {
		instanceIdentity, err = p.join(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if err := p.storage.WriteIdentity(ctx, state.IdentityCurrent, *instanceIdentity); err != nil {
			return nil, trace.Wrap(err)
		}
		if err := p.storage.PersistHostID(instanceIdentity.ID.HostID()); err != nil {
			return nil, trace.Wrap(err, "persisting host ID")
		}
		p.logger.InfoContext(ctx, "Wrote instance credentials to disk.", "host_uuid", instanceIdentity.ID.HostID())
	} else {
		p.logger.InfoContext(ctx, "Loaded existing instance identity.", "host_uuid", instanceIdentity.ID.HostID())
	}

	conn, err := newConnector(instanceIdentity, instanceIdentity)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	clt, _, err := p.newClientThroughTunnel(ctx, conn)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	conn.Client = clt

	ca, err := clt.GetCertAuthority(ctx, types.CertAuthID{
		DomainName: conn.ClusterName(),
		Type:       types.HostCA,
	}, false)
	if err != nil {
		return nil, trace.NewAggregate(err, conn.Close())
	}
	if err := p.storage.WriteState(ctx, types.RoleInstance, state.StateV2{
		Spec: state.StateSpecV2{
			Rotation:            ca.GetRotation(),
			InitialLocalVersion: teleport.Version,
		},
	}); err != nil {
		return nil, trace.NewAggregate(err, conn.Close())
	}

	nodeIdentity, err := p.storage.ReadIdentity(ctx, state.IdentityCurrent, types.RoleNode)
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.NewAggregate(err, conn.Close())
	}
	if trace.IsNotFound(err) {
		nodeIdentity, err = p.reRegister(ctx, clt, instanceIdentity, types.RoleNode, ca.GetRotation())
		if err != nil {
			return nil, trace.NewAggregate(err, conn.Close())
		}
		if err := p.storage.WriteIdentity(ctx, state.IdentityCurrent, *nodeIdentity); err != nil {
			return nil, trace.NewAggregate(err, conn.Close())
		}
		if err := p.storage.WriteState(ctx, types.RoleNode, state.StateV2{
			Spec: state.StateSpecV2{
				Rotation:            ca.GetRotation(),
				InitialLocalVersion: teleport.Version,
			},
		}); err != nil {
			return nil, trace.NewAggregate(err, conn.Close())
		}
	}

	nodeConn, err := newConnector(instanceIdentity, nodeIdentity)
	if err != nil {
		return nil, trace.NewAggregate(err, conn.Close())
	}
	nodeConn.Client = clt
	p.logger.InfoContext(ctx, "Connected to cluster.",
		"cluster", conn.ClusterName(),
		"host_uuid", conn.HostUUID(),
		"identity", slog.StringValue(fmt.Sprintf("%s/%s", instanceIdentity.ID.Role, nodeIdentity.ID.Role)),
	)
	return nodeConn, nil
}
