/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package gateway

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/teleterm/api/uri"
)

type staticDialer struct {
	addr string
}

func (d staticDialer) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	var nd net.Dialer
	return nd.DialContext(ctx, "tcp", d.addr)
}

func TestParseSSHForwardTarget(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in       string
		wantAddr string
		wantIdle bool
		wantErr  bool
	}{
		{in: "localhost:80", wantAddr: "localhost:80"},
		{in: "80", wantAddr: "localhost:80"},
		{in: "localhost:80#idle", wantAddr: "localhost:80", wantIdle: true},
		{in: "443#idle", wantAddr: "localhost:443", wantIdle: true},
		{in: "example.com:8080", wantAddr: "example.com:8080"},
		{in: "", wantErr: true},
		{in: "not-a-port", wantErr: true},
		{in: "localhost:0", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			addr, idle, err := ParseSSHForwardTarget(tt.in)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.wantAddr, addr)
			require.Equal(t, tt.wantIdle, idle)
		})
	}
}

func TestSSHGatewayProxiesTraffic(t *testing.T) {
	t.Parallel()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "ok")
	}))
	t.Cleanup(upstream.Close)

	gw, err := New(Config{
		TargetName:            "node",
		TargetURI:             uri.NewClusterURI("foo").AppendServer("node-id"),
		TargetUser:            "alice",
		TargetSubresourceName: "localhost:80",
		SSHDialer:             staticDialer{addr: upstream.Listener.Addr().String()},
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = gw.Close() })

	serveErr := make(chan error, 1)
	go func() {
		serveErr <- gw.Serve()
	}()

	address := net.JoinHostPort(gw.LocalAddress(), gw.LocalPort())
	require.Eventually(t, func() bool {
		conn, err := net.DialTimeout("tcp", address, time.Second)
		if err != nil {
			return false
		}
		_ = conn.Close()
		return true
	}, 5*time.Second, 50*time.Millisecond)

	resp, err := http.Get("http://" + address)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "ok", string(body))

	require.Equal(t, ProtocolHTTP, gw.Protocol())
	require.NoError(t, gw.Close())
	require.NoError(t, <-serveErr)
}

func TestSSHGatewayIdleTimeout(t *testing.T) {
	t.Parallel()

	clock := clockwork.NewFakeClock()
	gw, err := New(Config{
		TargetName:            "node",
		TargetURI:             uri.NewClusterURI("foo").AppendServer("node-id"),
		TargetUser:            "alice",
		TargetSubresourceName: "localhost:80#idle",
		SSHDialer:             staticDialer{addr: "127.0.0.1:1"},
		IdleTimeout:           SSHIdleTimeout,
		Clock:                 clock,
	})
	require.NoError(t, err)

	serveErr := make(chan error, 1)
	go func() {
		serveErr <- gw.Serve()
	}()

	clock.BlockUntil(1)
	clock.Advance(SSHIdleTimeout)
	require.ErrorIs(t, <-serveErr, ErrIdleTimeout)
}

func TestSSHGatewayIdleTimeoutFromMarker(t *testing.T) {
	t.Parallel()

	clock := clockwork.NewFakeClock()
	gw, err := New(Config{
		TargetName:            "node",
		TargetURI:             uri.NewClusterURI("foo").AppendServer("node-id"),
		TargetUser:            "alice",
		TargetSubresourceName: "localhost:80#idle",
		SSHDialer:             staticDialer{addr: "127.0.0.1:1"},
		Clock:                 clock,
	})
	require.NoError(t, err)

	serveErr := make(chan error, 1)
	go func() {
		serveErr <- gw.Serve()
	}()

	clock.BlockUntil(1)
	clock.Advance(SSHIdleTimeout)
	require.ErrorIs(t, <-serveErr, ErrIdleTimeout)
}

func TestSSHGatewayIdleTimeoutDisabled(t *testing.T) {
	t.Parallel()

	clock := clockwork.NewFakeClock()
	gw, err := New(Config{
		TargetName:            "node",
		TargetURI:             uri.NewClusterURI("foo").AppendServer("node-id"),
		TargetUser:            "alice",
		TargetSubresourceName: "localhost:22",
		SSHDialer:             staticDialer{addr: "127.0.0.1:1"},
		Clock:                 clock,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = gw.Close() })

	serveErr := make(chan error, 1)
	go func() {
		serveErr <- gw.Serve()
	}()

	clock.Advance(SSHIdleTimeout)
	select {
	case err := <-serveErr:
		t.Fatalf("gateway closed unexpectedly: %v", err)
	case <-time.After(100 * time.Millisecond):
	}

	require.NoError(t, gw.Close())
	require.NoError(t, <-serveErr)
}

func TestProtocolFromSSHRemoteAddr(t *testing.T) {
	t.Parallel()
	require.Equal(t, ProtocolHTTP, ProtocolFromSSHRemoteAddr("localhost:80"))
	require.Equal(t, ProtocolHTTPS, ProtocolFromSSHRemoteAddr("localhost:443"))
	require.Equal(t, ProtocolTCP, ProtocolFromSSHRemoteAddr("localhost:22"))
}
