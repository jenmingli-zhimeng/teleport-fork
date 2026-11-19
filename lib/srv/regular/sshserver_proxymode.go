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

package regular

import (
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/proxy"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
)

// SetProxyMode starts this server in SSH proxying mode
func SetProxyMode(peerAddr string, clusterGetter reversetunnelclient.ClusterGetter, ap authclient.ReadProxyAccessPoint, router *proxy.Router) ServerOption {
	return func(s *Server) error {
		// always set proxy mode to true,
		// because in some tests reverse tunnel is disabled,
		// but proxy is still used without it.
		s.proxyMode = true
		s.proxyClusterGetter = clusterGetter
		s.proxyAccessPoint = ap
		s.peerAddr = peerAddr
		s.router = router
		return nil
	}
}
