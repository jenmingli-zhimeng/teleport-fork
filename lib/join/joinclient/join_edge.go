//go:build edgeagent

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

package joinclient

import (
	"context"
	"crypto"
	"log/slog"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/api/client/proto"
	joiningv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/joining/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/state"
	proxyinsecureclient "github.com/gravitational/teleport/lib/client/proxy/insecure"
	"github.com/gravitational/teleport/lib/join/internal/messages"
	"github.com/gravitational/teleport/lib/join/joinv1"
	"github.com/gravitational/teleport/lib/utils"
)

// JoinParams is the token-only subset of RegisterParams used by teleport-edge.
type JoinParams struct {
	Token                string
	TokenSecret          string
	ID                   state.IdentityID
	ProxyServer          utils.NetAddr
	AdditionalPrincipals []string
	DNSNames             []string
	CipherSuites         []uint16
	CAPins               []string
	Clock                clockwork.Clock
	JoinMethod           types.JoinMethod
	Insecure             bool
	Expires              *time.Time
	Log                  *slog.Logger
}

// JoinResult is the token-join result returned to teleport-edge.
type JoinResult struct {
	Certs           *proto.Certs
	PrivateKey      crypto.Signer
	ImmutableLabels *joiningv1.ImmutableLabels
}

func (r *JoinParams) CheckAndSetDefaults() error {
	if r.Clock == nil {
		r.Clock = clockwork.NewRealClock()
	}
	if r.Log == nil {
		r.Log = slog.Default()
	}
	if r.ProxyServer.IsEmpty() {
		return trace.BadParameter("no proxy server set")
	}
	if r.JoinMethod == "" {
		r.JoinMethod = types.JoinMethodToken
	}
	if r.JoinMethod != types.JoinMethodToken {
		return trace.NotImplemented("teleport-edge only supports the token join method")
	}
	return nil
}

// Join joins a cluster through the Proxy using the token join method only.
func Join(ctx context.Context, params JoinParams) (*JoinResult, error) {
	if err := params.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	if params.ID.Role != types.RoleInstance && params.ID.Role != types.RoleBot {
		return nil, trace.BadParameter("Only Instance and Bot roles may be used for direct join attempts")
	}
	params.Log.InfoContext(ctx, "Attempting to join cluster via Proxy")
	return joinViaProxy(ctx, params, params.ProxyServer.String())
}

func joinViaProxy(ctx context.Context, params JoinParams, proxyAddr string) (*JoinResult, error) {
	conn, err := proxyinsecureclient.NewConnection(ctx,
		proxyinsecureclient.ConnectionConfig{
			ProxyServer:  proxyAddr,
			CipherSuites: params.CipherSuites,
			Clock:        params.Clock,
			Insecure:     params.Insecure,
			Log:          params.Log,
		},
	)
	if err != nil {
		return nil, &connectionError{trace.Wrap(err, "building proxy client")}
	}
	defer conn.Close()
	return joinWithClient(ctx, params, joinv1.NewClientFromConn(conn))
}

func joinWithClient(ctx context.Context, params JoinParams, client *joinv1.Client) (*JoinResult, error) {
	joinMethod := string(params.JoinMethod)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	stream, err := client.Join(ctx)
	if err != nil {
		return nil, &connectionError{trace.Wrap(err, "initiating join stream")}
	}
	defer stream.CloseSend()

	if err := stream.Send(&messages.ClientInit{
		JoinMethod: &joinMethod,
		TokenName:  params.Token,
		SystemRole: params.ID.Role.String(),
	}); err != nil {
		return nil, &connectionError{trace.Wrap(err, "sending ClientInit message")}
	}

	serverInit, err := messages.RecvResponse[*messages.ServerInit](stream)
	if err != nil {
		err = trace.Wrap(err, "receiving ServerInit message")
		if !trace.IsAccessDenied(err) && !trace.IsBadParameter(err) {
			err = &connectionError{err}
		}
		return nil, err
	}

	signer, publicKeys, err := GenerateKeys(ctx, serverInit.SignatureAlgorithmSuite)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	clientParams := makeClientParams(params, publicKeys)
	resultMsg, err := joinWithMethod(ctx, stream, params, clientParams, serverInit.JoinMethod)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	typedResult, ok := resultMsg.(*messages.HostResult)
	if !ok {
		return nil, trace.BadParameter("unhandled result message type %T", resultMsg)
	}
	return makeJoinResult(signer, typedResult.Certificates, typedResult.ImmutableLabels)
}
