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

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/join/internal/messages"
)

func joinWithMethod(
	_ context.Context,
	stream messages.ClientStream,
	joinParams JoinParams,
	clientParams messages.ClientParams,
	method string,
) (messages.Response, error) {
	switch types.JoinMethod(method) {
	case types.JoinMethodToken:
		return tokenJoin(stream, clientParams, joinParams.TokenSecret)
	default:
		sendGivingUpErr := stream.Send(&messages.GivingUp{
			Reason: messages.GivingUpReasonUnsupportedJoinMethod,
			Msg:    "join method " + method + " is not supported by this client",
		})
		return nil, trace.NewAggregate(
			trace.NotImplemented("server selected join method %v which is not supported by this client", method),
			trace.Wrap(sendGivingUpErr, "sending GivingUp message to server"),
		)
	}
}
