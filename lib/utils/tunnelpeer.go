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
 * along with this program.  If not, see <https://www.gnu.org/licenses/>.
 */

package utils

import (
	"net"
	"strings"
	"sync"
)

// tunnelPeers records the most recent reverse-tunnel TCP peer address per
// host UUID, as observed by the proxy. Inventory heartbeats use this to
// surface the agent's NAT public IP (the cluster's view of the connection).
var tunnelPeers sync.Map // map[string]string

// hostUUIDKey maps reverse-tunnel SSH usernames (uuid.cluster) and inventory
// server IDs (uuid) onto the same key.
func hostUUIDKey(hostID string) string {
	if hostID == "" {
		return ""
	}
	return strings.SplitN(hostID, ".", 2)[0]
}

// SetTunnelPeerAddr records the remote address of a reverse tunnel from hostID.
func SetTunnelPeerAddr(hostID, addr string) {
	id := hostUUIDKey(hostID)
	if id == "" || PublicHost(addr) == "" {
		return
	}
	tunnelPeers.Store(id, addr)
}

// TunnelPeerAddr returns the last recorded reverse-tunnel peer address for hostID.
func TunnelPeerAddr(hostID string) string {
	id := hostUUIDKey(hostID)
	if id == "" {
		return ""
	}
	v, ok := tunnelPeers.Load(id)
	if !ok {
		return ""
	}
	addr, _ := v.(string)
	return addr
}

// PublicHost returns the host part of addr if it is not loopback or unspecified.
func PublicHost(addr string) string {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
	}
	if host == "" || IsLocalhost(host) {
		return ""
	}
	return host
}

// PublicHostPort returns addr when its host is not loopback or unspecified.
func PublicHostPort(addr string) string {
	if PublicHost(addr) == "" {
		return ""
	}
	return addr
}
