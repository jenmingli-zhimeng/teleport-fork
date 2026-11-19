//go:build !edgeagent

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
	"os"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud/imds/gcp"
	"github.com/gravitational/teleport/lib/join/azuredevops"
	"github.com/gravitational/teleport/lib/join/bitbucket"
	"github.com/gravitational/teleport/lib/join/circleci"
	"github.com/gravitational/teleport/lib/join/env0"
	"github.com/gravitational/teleport/lib/join/githubactions"
	"github.com/gravitational/teleport/lib/join/gitlab"
	"github.com/gravitational/teleport/lib/join/internal/messages"
	"github.com/gravitational/teleport/lib/join/spacelift"
	"github.com/gravitational/teleport/lib/join/terraformcloud"
	kubetoken "github.com/gravitational/teleport/lib/kube/token"
)

func joinWithMethod(
	ctx context.Context,
	stream messages.ClientStream,
	joinParams JoinParams,
	clientParams messages.ClientParams,
	method string,
) (messages.Response, error) {
	var err error

	switch types.JoinMethod(method) {
	case types.JoinMethodToken:
		return tokenJoin(stream, clientParams, joinParams.TokenSecret)
	case types.JoinMethodAzure:
		return azureJoin(ctx, stream, joinParams, clientParams)
	case types.JoinMethodAzureDevops:
		if joinParams.IDToken == "" {
			joinParams.IDToken, err = azuredevops.NewIDTokenSource(os.Getenv).GetIDToken(ctx)
			if err != nil {
				return nil, trace.Wrap(err)
			}
		}
		return oidcJoin(stream, joinParams, clientParams)
	case types.JoinMethodBitbucket:
		if joinParams.IDToken == "" {
			joinParams.IDToken, err = bitbucket.NewIDTokenSource(os.Getenv).GetIDToken()
			if err != nil {
				return nil, trace.Wrap(err)
			}
		}
		return oidcJoin(stream, joinParams, clientParams)
	case types.JoinMethodBoundKeypair:
		return boundKeypairJoin(ctx, stream, joinParams, clientParams)
	case types.JoinMethodCircleCI:
		if joinParams.IDToken == "" {
			joinParams.IDToken, err = circleci.GetIDToken(os.Getenv)
			if err != nil {
				return nil, trace.Wrap(err)
			}
		}
		return oidcJoin(stream, joinParams, clientParams)
	case types.JoinMethodIAM:
		return iamJoin(ctx, stream, joinParams, clientParams)
	case types.JoinMethodEC2:
		return ec2Join(ctx, stream, joinParams, clientParams)
	case types.JoinMethodEnv0:
		if joinParams.IDToken == "" {
			joinParams.IDToken, err = env0.NewIDTokenSource(os.Getenv).GetIDToken()
			if err != nil {
				return nil, trace.Wrap(err)
			}
		}
		return oidcJoin(stream, joinParams, clientParams)
	case types.JoinMethodOracle:
		return oracleJoin(ctx, stream, joinParams, clientParams)
	case types.JoinMethodGCP:
		if joinParams.IDToken == "" {
			joinParams.IDToken, err = gcp.GetIDToken(ctx)
			if err != nil {
				return nil, trace.Wrap(err)
			}
		}

		return oidcJoin(stream, joinParams, clientParams)
	case types.JoinMethodGitHub:
		if joinParams.IDToken == "" {
			joinParams.IDToken, err = githubactions.NewIDTokenSource().GetIDToken(ctx)
			if err != nil {
				return nil, trace.Wrap(err)
			}
		}
		return oidcJoin(stream, joinParams, clientParams)
	case types.JoinMethodGitLab:
		if joinParams.IDToken == "" {
			joinParams.IDToken, err = gitlab.NewIDTokenSource(gitlab.IDTokenSourceConfig{
				EnvVarName: joinParams.GitlabParams.EnvVarName,
			}).GetIDToken()
			if err != nil {
				return nil, trace.Wrap(err)
			}
		}

		return oidcJoin(stream, joinParams, clientParams)
	case types.JoinMethodKubernetes:
		if joinParams.IDToken == "" {
			joinParams.IDToken, err = kubetoken.GetIDToken(os.Getenv, joinParams.KubernetesReadFileFunc)
			if err != nil {
				return nil, trace.Wrap(err)
			}
		}

		return oidcJoin(stream, joinParams, clientParams)
	case types.JoinMethodSpacelift:
		if joinParams.IDToken == "" {
			joinParams.IDToken, err = spacelift.NewIDTokenSource(os.Getenv).GetIDToken()
			if err != nil {
				return nil, trace.Wrap(err)
			}
		}

		return oidcJoin(stream, joinParams, clientParams)
	case types.JoinMethodTPM:
		return tpmJoin(ctx, stream, joinParams, clientParams)
	case types.JoinMethodTerraformCloud:
		if joinParams.IDToken == "" {
			joinParams.IDToken, err = terraformcloud.NewIDTokenSource(joinParams.TerraformCloudAudienceTag, os.Getenv).GetIDToken()
			if err != nil {
				return nil, trace.Wrap(err)
			}
		}

		return oidcJoin(stream, joinParams, clientParams)
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
