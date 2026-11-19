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

// Package edgeagent is a reduced Teleport SSH Node runtime for constrained
// devices (OpenWrt routers and similar). It reuses upstream packages for
// reverse tunnels, token join, SSH sessions, SFTP/SCP, TCP port
// forwarding, and ssh_service.commands device-tag labels, and replaces
// the sqlite-backed process storage with files.
//
// Reverse tunnels are recreated in-process when the default route or the
// source address on that path changes, or when auth pings fail repeatedly.
// Link up/down of LAN/switch ports does not restart the process or the
// tunnel.
//
// Cluster recording should use session_recording: node-sync so session data
// is not buffered on flash. If async recording is required, put data_dir/log
// on tmpfs or external storage.
//
// Logic that was copied from lib/service is listed in
// build.assets/edge/WATCH_SOURCES so upstream security fixes can be reviewed
// after each merge.
package edgeagent
