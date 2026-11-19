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
	"encoding/json"
	"net"
	"net/netip"

	"github.com/gravitational/teleport/api/utils/sshutils"
)

// parseDialReq parses the dial request. Is backward compatible with legacy
// payload.
func parseDialReq(payload []byte) *sshutils.DialReq {
	var req sshutils.DialReq
	err := json.Unmarshal(payload, &req)
	if err != nil {
		// For backward compatibility, if the request is not a *DialReq, it is just
		// a raw string with the target host as the payload.
		return &sshutils.DialReq{
			Address: string(payload),
		}
	}
	return &req
}

// getTCPAddr converts net.Addr to *net.TCPAddr.
//
// SSH package requires net.TCPAddr for source-address check.
func getTCPAddr(addr net.Addr) *net.TCPAddr {
	ap, err := netip.ParseAddrPort(addr.String())
	if err != nil {
		return nil
	}
	return net.TCPAddrFromAddrPort(ap)
}
