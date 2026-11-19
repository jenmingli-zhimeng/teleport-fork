//go:build linux

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

package edgeagent

import (
	"context"

	"golang.org/x/sys/unix"
)

// Address and route groups only: link up/down of LAN/switch ports must not
// wake the watcher. The process stays up; tunnels are rebuilt only when the
// default path actually changes.
const routeNetlinkGroups = unix.RTMGRP_IPV4_IFADDR | unix.RTMGRP_IPV4_ROUTE | unix.RTMGRP_IPV6_IFADDR | unix.RTMGRP_IPV6_ROUTE

// subscribeRouteEvents notifies when addresses or routes change. Failures
// fall back to polling in networkWatcher.
func subscribeRouteEvents(ctx context.Context) <-chan struct{} {
	fd, err := unix.Socket(unix.AF_NETLINK, unix.SOCK_RAW|unix.SOCK_CLOEXEC, unix.NETLINK_ROUTE)
	if err != nil {
		return nil
	}
	if err := unix.Bind(fd, &unix.SockaddrNetlink{
		Family: unix.AF_NETLINK,
		Groups: routeNetlinkGroups,
	}); err != nil {
		_ = unix.Close(fd)
		return nil
	}

	ch := make(chan struct{}, 1)
	go func() {
		defer unix.Close(fd)
		buf := make([]byte, 8192)
		tv := unix.Timeval{Sec: 1}
		for {
			if ctx.Err() != nil {
				return
			}
			_ = unix.SetsockoptTimeval(fd, unix.SOL_SOCKET, unix.SO_RCVTIMEO, &tv)
			n, err := unix.Read(fd, buf)
			if ctx.Err() != nil {
				return
			}
			if err != nil || n <= 0 {
				continue
			}
			select {
			case <-ctx.Done():
				return
			case ch <- struct{}{}:
			default:
			}
		}
	}()
	return ch
}
