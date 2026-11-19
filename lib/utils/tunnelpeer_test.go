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

import "testing"

func TestPublicHost(t *testing.T) {
	t.Parallel()
	cases := []struct {
		addr string
		want string
	}{
		{addr: "203.0.113.10:443", want: "203.0.113.10"},
		{addr: "203.0.113.10", want: "203.0.113.10"},
		{addr: "[2001:db8::1]:443", want: "2001:db8::1"},
		{addr: "127.0.0.1:443", want: ""},
		{addr: "localhost:443", want: ""},
		{addr: "0.0.0.0:443", want: ""},
		{addr: "[::1]:443", want: ""},
		{addr: "", want: ""},
	}
	for _, tc := range cases {
		if got := PublicHost(tc.addr); got != tc.want {
			t.Errorf("PublicHost(%q) = %q, want %q", tc.addr, got, tc.want)
		}
	}
}

func TestTunnelPeerAddr(t *testing.T) {
	const id = "tunnel-peer-test-host"
	SetTunnelPeerAddr(id, "127.0.0.1:9")
	if got := TunnelPeerAddr(id); got != "" {
		t.Fatalf("loopback peer should be ignored, got %q", got)
	}
	SetTunnelPeerAddr(id, "198.51.100.20:12345")
	if got := TunnelPeerAddr(id); got != "198.51.100.20:12345" {
		t.Fatalf("TunnelPeerAddr = %q", got)
	}
}

func TestTunnelPeerAddrNormalizesClusterSuffix(t *testing.T) {
	const uuid = "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
	SetTunnelPeerAddr(uuid+".example.com", "203.0.113.9:27443")
	if got := TunnelPeerAddr(uuid); got != "203.0.113.9:27443" {
		t.Fatalf("lookup by uuid = %q", got)
	}
	if got := TunnelPeerAddr(uuid + ".example.com"); got != "203.0.113.9:27443" {
		t.Fatalf("lookup by uuid.cluster = %q", got)
	}
}
