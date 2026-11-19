// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package joinclient

import (
	"context"
	"crypto"
	"crypto/x509"
	"encoding/pem"
	"errors"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/api/client/proto"
	joiningv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/joining/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/join/internal/messages"
)

func tokenJoin(
	stream messages.ClientStream,
	clientParams messages.ClientParams,
	secret string,
) (messages.Response, error) {
	// The token join method is relatively simple, the flow is
	//
	// client->server ClientInit
	// client<-server ServerInit
	// client->server Tokeninit
	// client<-server Result
	//
	// At this point the ServerInit message has already been received, all
	// that's left is to send the TokenInit message and receive the final result.
	tokenInitMsg := &messages.TokenInit{
		ClientParams: clientParams,
		Secret:       secret,
	}
	if err := stream.Send(tokenInitMsg); err != nil {
		return nil, trace.Wrap(err)
	}
	// Receive and return the final result.
	result, err := stream.Recv()
	return result, trace.Wrap(err)
}

func makeClientParams(params JoinParams, publicKeys *messages.PublicKeys) messages.ClientParams {
	if params.ID.Role == types.RoleBot {
		return messages.ClientParams{
			BotParams: &messages.BotParams{
				PublicKeys: *publicKeys,
				Expires:    params.Expires,
			},
		}
	}
	return messages.ClientParams{
		HostParams: &messages.HostParams{
			PublicKeys:           *publicKeys,
			HostName:             params.ID.NodeName,
			AdditionalPrincipals: params.AdditionalPrincipals,
			DNSNames:             params.DNSNames,
		},
	}
}

func makeJoinResult(signer crypto.Signer, certs messages.Certificates, immutableLabels *joiningv1.ImmutableLabels) (*JoinResult, error) {
	// Callers expect proto.Certs with PEM-formatted TLS certs and
	// authorized_keys formated SSH certs/keys.
	sshCert, err := toAuthorizedKey(certs.SSHCert)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sshCAKeys, err := toAuthorizedKeys(certs.SSHCAKeys)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &JoinResult{
		Certs: &proto.Certs{
			TLS:        pemEncodeTLSCert(certs.TLSCert),
			TLSCACerts: pemEncodeTLSCerts(certs.TLSCACerts),
			SSH:        sshCert,
			SSHCACerts: sshCAKeys, // SSHCACerts is a misnomer, SSH CAs are just public keys.
		},
		PrivateKey:      signer,
		ImmutableLabels: immutableLabels,
	}, nil
}

func toAuthorizedKeys(wireFormats [][]byte) ([][]byte, error) {
	out := make([][]byte, len(wireFormats))
	for i, wireFormat := range wireFormats {
		var err error
		out[i], err = toAuthorizedKey(wireFormat)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return out, nil
}

func toAuthorizedKey(wireFormat []byte) ([]byte, error) {
	sshPub, err := ssh.ParsePublicKey(wireFormat)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return ssh.MarshalAuthorizedKey(sshPub), nil
}

func pemEncodeTLSCerts(rawCerts [][]byte) [][]byte {
	out := make([][]byte, len(rawCerts))
	for i, rawCert := range rawCerts {
		out[i] = pemEncodeTLSCert(rawCert)
	}
	return out
}

func pemEncodeTLSCert(rawCert []byte) []byte {
	return pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: rawCert,
	})
}

// GenerateKeys generates host keys appropriate for a cluster join request
// according to the cluster's configured signature algorithm suite.
func GenerateKeys(ctx context.Context, suite types.SignatureAlgorithmSuite) (crypto.Signer, *messages.PublicKeys, error) {
	signer, err := cryptosuites.GenerateKey(
		ctx,
		cryptosuites.StaticAlgorithmSuite(suite),
		cryptosuites.HostIdentity,
	)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	tlsPub, err := x509.MarshalPKIXPublicKey(signer.Public())
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	sshPub, err := ssh.NewPublicKey(signer.Public())
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	return signer, &messages.PublicKeys{
		PublicTLSKey: tlsPub,
		PublicSSHKey: sshPub.Marshal(),
	}, nil
}

type connectionError struct {
	wrapped error
}

func (e *connectionError) Error() string {
	return e.wrapped.Error()
}

func (e *connectionError) Unwrap() error {
	return e.wrapped
}

func isConnectionError(err error) bool {
	var ce *connectionError
	return errors.As(err, &ce)
}

// LegacyJoinError is returned when the join attempt failed while attempting to
// join via the legacy join service.
type LegacyJoinError struct {
	wrapped error
}

func (e *LegacyJoinError) Error() string {
	return e.wrapped.Error()
}

func (e *LegacyJoinError) Unwrap() error {
	return e.wrapped
}
