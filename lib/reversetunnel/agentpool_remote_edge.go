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

package reversetunnel

import (
	"context"
	"io"

	"github.com/gravitational/teleport/api/utils/sshutils"
	"golang.org/x/crypto/ssh"
)

func (p *AgentPool) handleRemoteTransport(ctx context.Context, channel ssh.Channel, requests <-chan *ssh.Request, _ sshutils.Conn) {
	defer channel.Close()
	go io.Copy(io.Discard, channel.Stderr())
	go ssh.DiscardRequests(requests)
	p.logger.ErrorContext(ctx, "Remote-cluster reverse tunnels are not supported on teleport-edge")
}
