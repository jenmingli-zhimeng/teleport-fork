/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package clusters

import (
	"context"
	"crypto/tls"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client/proto"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/mfa"
	"github.com/gravitational/teleport/lib/client"
	libmfa "github.com/gravitational/teleport/lib/client/mfa"
	"github.com/gravitational/teleport/lib/client/sso"
	"github.com/gravitational/teleport/lib/teleterm/api/uri"
	"github.com/gravitational/teleport/lib/teleterm/gateway"
	"github.com/gravitational/teleport/lib/tlsca"
)

type CreateGatewayParams struct {
	// TargetURI is the cluster resource URI
	TargetURI uri.ResourceURI
	// TargetUser is the target user name
	TargetUser string
	// TargetSubresourceName points at a subresource of the remote resource, for example a database
	// name on a database server.
	TargetSubresourceName string
	// LocalPort is the gateway local port
	LocalPort            string
	TCPPortAllocator     gateway.TCPPortAllocator
	OnExpiredCert        gateway.OnExpiredCertFunc
	KubeconfigsDir       string
	MFAPromptConstructor func(cfg *libmfa.PromptConfig) mfa.Prompt
	ClusterClient        *client.ClusterClient
}

// CreateGateway creates a gateway
func (c *Cluster) CreateGateway(ctx context.Context, params CreateGatewayParams) (gateway.Gateway, error) {
	c.clusterClient.MFAPromptConstructor = params.MFAPromptConstructor
	c.clusterClient.MFACeremonyConstructor = sso.NewConnectMFACeremony

	switch {
	case params.TargetURI.IsDB():
		gateway, err := c.createDBGateway(ctx, params)
		return gateway, trace.Wrap(err)

	case params.TargetURI.IsKube():
		gateway, err := c.createKubeGateway(ctx, params)
		return gateway, trace.Wrap(err)

	case params.TargetURI.IsApp():
		gateway, err := c.createAppGateway(ctx, params)
		return gateway, trace.Wrap(err)

	case params.TargetURI.IsServer():
		gateway, err := c.createSSHGateway(ctx, params)
		return gateway, trace.Wrap(err)

	default:
		return nil, trace.NotImplemented("gateway not supported for %v", params.TargetURI)
	}
}

func (c *Cluster) createDBGateway(ctx context.Context, params CreateGatewayParams) (gateway.Gateway, error) {
	db, err := c.GetDatabase(ctx, params.ClusterClient.AuthClient, params.TargetURI)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	routeToDatabase := tlsca.RouteToDatabase{
		ServiceName: db.GetName(),
		Protocol:    db.GetProtocol(),
		Username:    params.TargetUser,
	}

	var cert tls.Certificate

	if err := AddMetadataToRetryableError(ctx, func() error {
		cert, err = c.reissueDBCerts(ctx, params.ClusterClient, routeToDatabase)
		return trace.Wrap(err)
	}); err != nil {
		return nil, trace.Wrap(err)
	}

	gw, err := gateway.New(gateway.Config{
		LocalPort:                     params.LocalPort,
		TargetURI:                     params.TargetURI,
		TargetUser:                    params.TargetUser,
		TargetName:                    db.GetName(),
		TargetSubresourceName:         params.TargetSubresourceName,
		Protocol:                      db.GetProtocol(),
		Cert:                          cert,
		Insecure:                      c.clusterClient.InsecureSkipVerify,
		WebProxyAddr:                  c.clusterClient.WebProxyAddr,
		Logger:                        c.Logger,
		TCPPortAllocator:              params.TCPPortAllocator,
		OnExpiredCert:                 params.OnExpiredCert,
		Clock:                         c.clock,
		TLSRoutingConnUpgradeRequired: c.clusterClient.TLSRoutingConnUpgradeRequired,
		RootClusterCACertPoolFunc:     c.clusterClient.RootClusterCACertPool,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return gw, nil
}

func (c *Cluster) createKubeGateway(ctx context.Context, params CreateGatewayParams) (gateway.Gateway, error) {
	kube := params.TargetURI.GetKubeName()

	// Check if this kube exists and the user has access to it.
	if _, err := c.getKube(ctx, params.ClusterClient.AuthClient, kube); err != nil {
		return nil, trace.Wrap(err)
	}

	var cert tls.Certificate
	var err error

	if err := AddMetadataToRetryableError(ctx, func() error {
		cert, err = c.reissueKubeCert(ctx, params.ClusterClient, kube)
		return trace.Wrap(err)
	}); err != nil {
		return nil, trace.Wrap(err)
	}

	// TODO(ravicious): Support TargetUser (--as), TargetGroups (--as-groups), TargetSubresourceName (--kube-namespace).
	gw, err := gateway.New(gateway.Config{
		LocalPort:                     params.LocalPort,
		TargetURI:                     params.TargetURI,
		TargetName:                    kube,
		Cert:                          cert,
		Insecure:                      c.clusterClient.InsecureSkipVerify,
		WebProxyAddr:                  c.clusterClient.WebProxyAddr,
		RelayAddr:                     c.clusterClient.RelayAddr,
		Logger:                        c.Logger,
		TCPPortAllocator:              params.TCPPortAllocator,
		OnExpiredCert:                 params.OnExpiredCert,
		Clock:                         c.clock,
		TLSRoutingConnUpgradeRequired: c.clusterClient.TLSRoutingConnUpgradeRequired,
		RootClusterCACertPoolFunc:     c.clusterClient.RootClusterCACertPool,
		ClusterName:                   c.Name,
		Username:                      c.status.Username,
		KubeconfigsDir:                params.KubeconfigsDir,
	})
	return gw, trace.Wrap(err)
}

func (c *Cluster) createAppGateway(ctx context.Context, params CreateGatewayParams) (gateway.Gateway, error) {
	appName := params.TargetURI.GetAppName()
	app, err := GetApp(ctx, params.ClusterClient.AuthClient, appName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	routeToApp := proto.RouteToApp{
		Name:        app.GetName(),
		PublicAddr:  app.GetPublicAddr(),
		ClusterName: c.clusterClient.SiteName,
		URI:         app.GetURI(),
	}
	if params.TargetSubresourceName != "" {
		targetPort, err := ValidateTargetPort(app, params.TargetSubresourceName)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		routeToApp.TargetPort = targetPort
	}

	var cert tls.Certificate
	if err := AddMetadataToRetryableError(ctx, func() error {
		cert, err = c.ReissueAppCert(ctx, params.ClusterClient, routeToApp)
		return trace.Wrap(err)
	}); err != nil {
		return nil, trace.Wrap(err)
	}

	gw, err := gateway.New(gateway.Config{
		LocalPort:                     params.LocalPort,
		TargetURI:                     params.TargetURI,
		TargetSubresourceName:         params.TargetSubresourceName,
		TargetName:                    appName,
		Cert:                          cert,
		Protocol:                      app.GetProtocol(),
		Insecure:                      c.clusterClient.InsecureSkipVerify,
		WebProxyAddr:                  c.clusterClient.WebProxyAddr,
		Logger:                        c.Logger,
		TCPPortAllocator:              params.TCPPortAllocator,
		OnExpiredCert:                 params.OnExpiredCert,
		Clock:                         c.clock,
		TLSRoutingConnUpgradeRequired: c.clusterClient.TLSRoutingConnUpgradeRequired,
		RootClusterCACertPoolFunc:     c.clusterClient.RootClusterCACertPool,
		ClusterName:                   c.Name,
		Username:                      c.status.Username,
		// For multi-port TCP apps, the target port is stored in the target subresource name. Whenever
		// that field is updated, the local proxy needs to generate a new cert which includes that port.
		ClearCertsOnTargetSubresourceNameChange: true,
	})
	return gw, trace.Wrap(err)
}

func (c *Cluster) createSSHGateway(ctx context.Context, params CreateGatewayParams) (gateway.Gateway, error) {
	if params.TargetUser == "" {
		return nil, trace.BadParameter("missing SSH login")
	}

	serverUUID := params.TargetURI.GetServerUUID()
	if serverUUID == "" {
		return nil, trace.BadParameter("server URI does not include server UUID")
	}

	remoteAddr, idle, err := gateway.ParseSSHForwardTarget(params.TargetSubresourceName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	targetName := serverUUID
	if node, err := params.ClusterClient.AuthClient.GetNode(ctx, apidefaults.Namespace, serverUUID); err == nil {
		if hostname := node.GetHostname(); hostname != "" {
			targetName = hostname
		}
	}

	// ConnectToNode / SSH resumption bind the SSH connection to ctx for its
	// entire lifetime. The CreateGateway RPC context is canceled as soon as
	// the RPC returns, which would immediately tear down the tunnel. Detach
	// from that RPC context and keep the SSH session until the gateway closes.
	sshCtx, sshCancel := context.WithCancel(context.WithoutCancel(ctx))

	var nodeClient *client.NodeClient
	if err := AddMetadataToRetryableError(ctx, func() error {
		var connectErr error
		nodeClient, connectErr = c.clusterClient.ConnectToNode(sshCtx, params.ClusterClient, client.NodeDetails{
			Addr:    serverUUID + ":0",
			Cluster: c.Name,
		}, params.TargetUser)
		return trace.Wrap(connectErr)
	}); err != nil {
		sshCancel()
		return nil, trace.Wrap(err)
	}

	idleTimeout := time.Duration(0)
	if idle {
		idleTimeout = gateway.SSHIdleTimeout
	}

	dialer := &swappingSSHDialer{dialer: nodeClient.Client}
	closer := newSSHNodeCloser(nodeClient, sshCancel)
	gw, err := gateway.New(gateway.Config{
		LocalPort:             params.LocalPort,
		TargetURI:             params.TargetURI,
		TargetUser:            params.TargetUser,
		TargetName:            targetName,
		TargetSubresourceName: remoteAddr,
		Protocol:              gateway.ProtocolFromSSHRemoteAddr(remoteAddr),
		SSHDialer:             dialer,
		SSHCloser:             closer,
		IdleTimeout:           idleTimeout,
		TCPPortAllocator:      params.TCPPortAllocator,
		Clock:                 c.clock,
		Logger:                c.Logger,
	})
	if err != nil {
		_ = closer.Close()
		return nil, trace.Wrap(err)
	}

	go c.keepSSHGatewaySession(gw, closer, dialer, params, serverUUID)
	return gw, nil
}

// sshReconnectDelays is used when the SSH session behind a port forward
// drops (common on reverse-tunnel / WAN path changes). Keep the local
// listener and try to rebuild the session before giving up.
var sshReconnectDelays = []time.Duration{time.Second, 2 * time.Second, 4 * time.Second}

func (c *Cluster) keepSSHGatewaySession(gw gateway.Gateway, closer *sshNodeCloser, dialer *swappingSSHDialer, params CreateGatewayParams, serverUUID string) {
	for {
		node := closer.current()
		if node == nil {
			return
		}
		_ = node.Client.Wait()
		if closer.isClosed() {
			return
		}

		c.Logger.InfoContext(context.Background(), "SSH port forward lost its SSH session, reconnecting")
		if c.reconnectSSHGatewaySession(closer, dialer, params, serverUUID) {
			continue
		}
		c.Logger.WarnContext(context.Background(), "Giving up SSH port forward reconnect")
		if closeErr := gw.Close(); closeErr != nil {
			c.Logger.DebugContext(context.Background(), "SSH port forward closed after SSH reconnect failed", "error", closeErr)
		}
		return
	}
}

func (c *Cluster) reconnectSSHGatewaySession(closer *sshNodeCloser, dialer *swappingSSHDialer, params CreateGatewayParams, serverUUID string) bool {
	for i, delay := range sshReconnectDelays {
		select {
		case <-closer.done():
			return false
		case <-time.After(delay):
		}

		sshCtx, ok := closer.beginReconnect()
		if !ok {
			return false
		}

		var newClient *client.NodeClient
		err := AddMetadataToRetryableError(sshCtx, func() error {
			var connectErr error
			newClient, connectErr = c.clusterClient.ConnectToNode(sshCtx, params.ClusterClient, client.NodeDetails{
				Addr:    serverUUID + ":0",
				Cluster: c.Name,
			}, params.TargetUser)
			return trace.Wrap(connectErr)
		})
		if err != nil {
			c.Logger.WarnContext(context.Background(), "Failed to reconnect SSH port forward",
				"attempt", i+1,
				"error", err,
			)
			continue
		}
		if !closer.finishReconnect(newClient) {
			_ = newClient.Close()
			return false
		}
		dialer.set(newClient.Client)
		c.Logger.InfoContext(context.Background(), "SSH port forward reconnected")
		return true
	}
	return false
}

type swappingSSHDialer struct {
	mu     sync.RWMutex
	dialer gateway.SSHDialer
}

func (s *swappingSSHDialer) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	s.mu.RLock()
	d := s.dialer
	s.mu.RUnlock()
	if d == nil {
		return nil, trace.ConnectionProblem(nil, "SSH port forward is not connected")
	}
	return d.DialContext(ctx, network, addr)
}

func (s *swappingSSHDialer) set(d gateway.SSHDialer) {
	s.mu.Lock()
	s.dialer = d
	s.mu.Unlock()
}

type sshNodeCloser struct {
	mu       sync.Mutex
	node     *client.NodeClient
	cancel   context.CancelFunc
	closed   bool
	closedCh chan struct{}
}

func newSSHNodeCloser(node *client.NodeClient, cancel context.CancelFunc) *sshNodeCloser {
	return &sshNodeCloser{
		node:     node,
		cancel:   cancel,
		closedCh: make(chan struct{}),
	}
}

func (c *sshNodeCloser) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	c.closed = true
	close(c.closedCh)
	if c.cancel != nil {
		c.cancel()
		c.cancel = nil
	}
	if c.node == nil {
		return nil
	}
	err := c.node.Close()
	c.node = nil
	return trace.Wrap(err)
}

func (c *sshNodeCloser) isClosed() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.closed
}

func (c *sshNodeCloser) done() <-chan struct{} {
	return c.closedCh
}

func (c *sshNodeCloser) current() *client.NodeClient {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.node
}

func (c *sshNodeCloser) beginReconnect() (context.Context, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil, false
	}
	if c.cancel != nil {
		c.cancel()
		c.cancel = nil
	}
	if c.node != nil {
		_ = c.node.Close()
		c.node = nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	c.cancel = cancel
	return ctx, true
}

func (c *sshNodeCloser) finishReconnect(node *client.NodeClient) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return false
	}
	c.node = node
	return true
}

// ReissueGatewayCerts reissues certificate for the provided gateway.
func (c *Cluster) ReissueGatewayCerts(ctx context.Context, clusterClient *client.ClusterClient, g gateway.Gateway) (tls.Certificate, error) {
	switch {
	case g.TargetURI().IsDB():
		db, err := gateway.AsDatabase(g)
		if err != nil {
			return tls.Certificate{}, trace.Wrap(err)
		}
		cert, err := c.reissueDBCerts(ctx, clusterClient, db.RouteToDatabase())
		return cert, trace.Wrap(err)
	case g.TargetURI().IsKube():
		cert, err := c.reissueKubeCert(ctx, clusterClient, g.TargetName())
		return cert, trace.Wrap(err)
	case g.TargetURI().IsApp():
		appName := g.TargetURI().GetAppName()
		app, err := GetApp(ctx, clusterClient.AuthClient, appName)
		if err != nil {
			return tls.Certificate{}, trace.Wrap(err)
		}
		routeToApp := proto.RouteToApp{
			Name:        app.GetName(),
			PublicAddr:  app.GetPublicAddr(),
			ClusterName: c.clusterClient.SiteName,
			URI:         app.GetURI(),
		}
		if g.TargetSubresourceName() != "" {
			targetPort, err := parseTargetPort(g.TargetSubresourceName())
			if err != nil {
				return tls.Certificate{}, trace.BadParameter("%s", err)
			}
			routeToApp.TargetPort = targetPort
		}

		// The cert is returned from this function and finally set on LocalProxy by the middleware.
		cert, err := c.ReissueAppCert(ctx, clusterClient, routeToApp)
		return cert, trace.Wrap(err)
	default:
		return tls.Certificate{}, trace.NotImplemented("ReissueGatewayCerts does not support this gateway kind %v", g.TargetURI().String())
	}
}

func parseTargetPort(rawTargetPort string) (uint32, error) {
	targetPort, err := strconv.ParseUint(rawTargetPort, 10, 32)
	if err != nil {
		return 0, trace.BadParameter("%s", err)
	}
	return uint32(targetPort), nil
}
