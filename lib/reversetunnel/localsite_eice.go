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

package reversetunnel

import (
	"context"
	"net"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/integrations/awsoidc"
)

func (s *localSite) setupTunnelForOpenSSHEICENode(ctx context.Context, targetServer types.Server) (net.Conn, error) {
	awsInfo := targetServer.GetAWSInfo()
	if awsInfo == nil {
		return nil, trace.BadParameter("missing aws cloud metadata")
	}

	token, err := s.client.GenerateAWSOIDCToken(ctx, awsInfo.Integration)
	if err != nil {
		return nil, trace.BadParameter("failed to generate aws token: %v", err)
	}

	integration, err := s.client.GetIntegration(ctx, awsInfo.Integration)
	if err != nil {
		return nil, trace.BadParameter("failed to fetch integration details: %v", err)
	}

	if integration.GetAWSOIDCIntegrationSpec() == nil {
		return nil, trace.BadParameter("integration does not have aws oidc spec fields %q", awsInfo.Integration)
	}

	openTunnelClt, err := awsoidc.NewOpenTunnelEC2Client(ctx, &awsoidc.AWSClientRequest{
		Token:   token,
		RoleARN: integration.GetAWSOIDCIntegrationSpec().RoleARN,
		Region:  awsInfo.Region,
	})
	if err != nil {
		return nil, trace.BadParameter("failed to create the ec2 open tunnel client: %v", err)
	}

	openTunnelResp, err := awsoidc.OpenTunnelEC2(ctx, openTunnelClt, awsoidc.OpenTunnelEC2Request{
		Region:        awsInfo.Region,
		VPCID:         awsInfo.VPCID,
		EC2InstanceID: awsInfo.InstanceID,
		EC2Address:    targetServer.GetAddr(),
	})
	if err != nil {
		return nil, trace.BadParameter("failed to open AWS EC2 Instance Connect Endpoint tunnel: %v", err)
	}

	return openTunnelResp.Tunnel, nil
}
