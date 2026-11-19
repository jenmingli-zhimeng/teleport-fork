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
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/utils"
)

const (
	// SSHIdleMarker is appended to CreateGatewayRequest.target_subresource_name
	// to request a 15-minute idle timeout on an SSH port forward.
	SSHIdleMarker = "#idle"
	// SSHIdleTimeout is the inactivity period after which a one-click HTTP/HTTPS
	// SSH port forward is closed.
	SSHIdleTimeout = 15 * time.Minute
	// ProtocolHTTP is used when forwarding to remote port 80.
	ProtocolHTTP = "HTTP"
	// ProtocolHTTPS is used when forwarding to remote port 443.
	ProtocolHTTPS = "HTTPS"
	// ProtocolTCP is used for SSH port forwards that are not HTTP/HTTPS.
	ProtocolTCP = "TCP"
)

// ErrIdleTimeout is returned from Serve when the SSH gateway is closed after
// sitting idle.
var ErrIdleTimeout = errors.New("ssh port forward closed after idle timeout")

// ErrSSHGatewayClosed is reported to the Electron app when an SSH port
// forward's Serve loop ends for any reason other than idle timeout (for
// example the SSH session dropped and reconnect failed). The UI treats this
// as "the gateway is already gone".
var ErrSSHGatewayClosed = errors.New("ssh port forward closed")

type sshGateway struct {
	*base
	listener       net.Listener
	ownsNodeClient atomic.Bool
	idleClosed     atomic.Bool
	closeOnce      sync.Once
	connsMu        sync.Mutex
	activeConns    int
	lastActivity   time.Time
}

func makeSSHGateway(cfg Config) (Gateway, error) {
	base, err := newBase(cfg)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	listener, err := base.cfg.makeListener()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	s := &sshGateway{
		base:         base,
		listener:     listener,
		lastActivity: base.cfg.Clock.Now(),
	}
	s.ownsNodeClient.Store(true)
	s.onCloseFuncs = append(s.onCloseFuncs, listener.Close, s.closeSSHNodeClient)
	return s, nil
}

func (s *sshGateway) SetTargetSubresourceName(value string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if remoteAddr, _, err := ParseSSHForwardTarget(value); err == nil {
		value = remoteAddr
	}
	s.cfg.TargetSubresourceName = value
	s.cfg.Protocol = ProtocolFromSSHRemoteAddr(value)
}

func (s *sshGateway) Close() error {
	var err error
	s.closeOnce.Do(func() {
		err = s.base.Close()
	})
	return err
}

func (s *sshGateway) Serve() error {
	s.cfg.Logger.InfoContext(s.closeContext, "SSH gateway is open")
	defer s.cfg.Logger.InfoContext(s.closeContext, "SSH gateway has closed")

	if s.cfg.IdleTimeout > 0 {
		go s.watchIdle()
	}

	for {
		conn, err := s.listener.Accept()
		if err != nil {
			if s.closeContext.Err() != nil {
				if s.idleClosed.Load() {
					return ErrIdleTimeout
				}
				return nil
			}
			return trace.Wrap(err)
		}
		go s.handleConn(conn)
	}
}

func (s *sshGateway) handleConn(conn net.Conn) {
	s.trackConn(1)
	defer s.trackConn(-1)
	defer conn.Close()

	remoteAddr := s.TargetSubresourceName()
	if parsed, _, err := ParseSSHForwardTarget(remoteAddr); err == nil {
		remoteAddr = parsed
	}
	remoteConn, err := s.cfg.SSHDialer.DialContext(s.closeContext, "tcp", remoteAddr)
	if err != nil {
		s.cfg.Logger.WarnContext(s.closeContext, "Failed to dial remote address through SSH",
			"remote_addr", remoteAddr,
			"error", err,
		)
		return
	}
	if err := utils.ProxyConn(s.closeContext, conn, remoteConn); err != nil && !utils.IsOKNetworkError(err) {
		s.cfg.Logger.WarnContext(s.closeContext, "Failed to proxy SSH forwarded connection",
			"remote_addr", remoteAddr,
			"error", err,
		)
	}
}

func (s *sshGateway) trackConn(delta int) {
	s.connsMu.Lock()
	defer s.connsMu.Unlock()
	s.activeConns += delta
	s.lastActivity = s.cfg.Clock.Now()
}

func (s *sshGateway) watchIdle() {
	timeout := s.cfg.IdleTimeout
	for {
		s.connsMu.Lock()
		deadline := s.lastActivity.Add(timeout)
		active := s.activeConns
		s.connsMu.Unlock()

		wait := deadline.Sub(s.cfg.Clock.Now())
		if active > 0 {
			wait = timeout
		}
		if wait < 0 {
			wait = 0
		}

		select {
		case <-s.closeContext.Done():
			return
		case <-s.cfg.Clock.After(wait):
			s.connsMu.Lock()
			idle := s.activeConns == 0 && !s.cfg.Clock.Now().Before(s.lastActivity.Add(timeout))
			s.connsMu.Unlock()
			if idle {
				s.idleClosed.Store(true)
				if err := s.Close(); err != nil {
					s.cfg.Logger.WarnContext(s.closeContext, "Failed to close idle SSH gateway", "error", err)
				}
				return
			}
		}
	}
}

func (s *sshGateway) closeSSHNodeClient() error {
	if !s.ownsNodeClient.Load() || s.cfg.SSHCloser == nil {
		return nil
	}
	return trace.Wrap(s.cfg.SSHCloser.Close())
}

func (s *sshGateway) releaseSSHNodeClient() {
	s.ownsNodeClient.Store(false)
}

// ParseSSHForwardTarget parses a remote forward target.
//
// Accepted forms:
//   - "host:port"
//   - "port" (host defaults to localhost)
//   - either form with a trailing "#idle" to request SSHIdleTimeout
func ParseSSHForwardTarget(raw string) (remoteAddr string, idle bool, err error) {
	raw = strings.TrimSpace(raw)
	if strings.HasSuffix(raw, SSHIdleMarker) {
		idle = true
		raw = strings.TrimSuffix(raw, SSHIdleMarker)
	}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", false, trace.BadParameter("missing remote address")
	}

	host, port, splitErr := net.SplitHostPort(raw)
	if splitErr != nil {
		if _, convErr := strconv.ParseUint(raw, 10, 16); convErr != nil {
			return "", false, trace.Wrap(splitErr, "invalid remote address %q", raw)
		}
		host = "localhost"
		port = raw
	}
	if host == "" {
		host = "localhost"
	}
	portNum, err := strconv.ParseUint(port, 10, 16)
	if err != nil || portNum == 0 {
		return "", false, trace.BadParameter("invalid remote port %q", port)
	}
	return net.JoinHostPort(host, strconv.FormatUint(portNum, 10)), idle, nil
}

// ProtocolFromSSHRemoteAddr returns HTTP, HTTPS or TCP based on the remote port.
func ProtocolFromSSHRemoteAddr(remoteAddr string) string {
	_, port, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return ProtocolTCP
	}
	switch port {
	case "80":
		return ProtocolHTTP
	case "443":
		return ProtocolHTTPS
	default:
		return ProtocolTCP
	}
}

// SSHForwardCLIPreview returns a curl command targeting the local listener.
func SSHForwardCLIPreview(localAddress, localPort, protocol string) string {
	scheme := "http"
	if protocol == ProtocolHTTPS {
		scheme = "https"
	}
	return fmt.Sprintf("curl %s://%s", scheme, net.JoinHostPort(localAddress, localPort))
}
