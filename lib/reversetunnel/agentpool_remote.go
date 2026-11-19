//go:build !edgeagent

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

package reversetunnel

import (
	"context"

	"github.com/gravitational/teleport/api/utils/sshutils"
	"golang.org/x/crypto/ssh"
)

func (p *AgentPool) handleRemoteTransport(ctx context.Context, channel ssh.Channel, requests <-chan *ssh.Request, conn sshutils.Conn) {
	t := &transport{
		closeContext:         ctx,
		component:            p.Component,
		localClusterName:     p.LocalCluster,
		kubeDialAddr:         p.KubeDialAddr,
		authClient:           p.Client,
		reverseTunnelServer:  p.ReverseTunnelServer,
		server:               p.Server,
		emitter:              p.Client,
		sconn:                conn,
		channel:              channel,
		requestCh:            requests,
		logger:               p.logger,
		authServers:          p.LocalAuthAddresses,
		proxySigner:          p.PROXYSigner,
		forwardClientAddress: true,
	}

	// If the AgentPool is being used for Proxy to Proxy communication between two clusters, then
	// we check if the reverse tunnel server is capable of tracking user connections. This allows
	// the leaf proxy to track sessions that are initiated via the root cluster. Without providing
	// the user tracker the leaf cluster metrics will be incorrect and graceful shutdown will not
	// wait for user sessions to be terminated prior to proceeding with the shutdown operation.
	if p.ReverseTunnelServer != nil {
		t.trackUserConnection = p.ReverseTunnelServer.TrackUserConnection
	}

	t.start()
}
