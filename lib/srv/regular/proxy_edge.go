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

package regular

import (
	"context"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/lib/srv"
)

type proxySubsysRequest struct {
	namespace   string
	host        string
	port        string
	clusterName string
}

type proxySubsys struct{}

func (p *proxySubsys) Start(context.Context, *ssh.ServerConn, ssh.Channel, *ssh.Request, *srv.ServerContext) error {
	return trace.NotImplemented("proxy subsystem is not supported on teleport-edge")
}

func (p *proxySubsys) Wait() error { return nil }

func (s *Server) parseProxySubsys(context.Context, string, *srv.ServerContext) (*proxySubsys, error) {
	return nil, trace.NotImplemented("proxy subsystem is not supported on teleport-edge")
}

func newProxySubsys(context.Context, *srv.ServerContext, *Server, proxySubsysRequest) (*proxySubsys, error) {
	return nil, trace.NotImplemented("proxy jump is not supported on teleport-edge")
}
