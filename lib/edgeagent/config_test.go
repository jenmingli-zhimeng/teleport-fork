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
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseRejectsUnsupportedFields(t *testing.T) {
	_, err := Parse([]byte(`
version: v3
teleport:
  proxy_server: proxy.example.com:443
  join_params:
    token_name: tok
    method: token
auth_service:
  enabled: yes
`))
	require.Error(t, err)
	require.Contains(t, err.Error(), "auth_service")
}

func TestParseHappyPath(t *testing.T) {
	cfg, err := Parse([]byte(`
version: v3
teleport:
  nodename: router-1
  data_dir: /tmp/teleport-edge
  proxy_server: proxy.example.com:443
  join_params:
    token_name: tok
    method: token
  log:
    output: stderr
    severity: info
ssh_service:
  enabled: yes
  labels:
    env: lab
edge_service:
  memory_limit: 64MiB
  max_connections: 4
`))
	require.NoError(t, err)
	require.Equal(t, "router-1", cfg.Teleport.Nodename)
	require.Equal(t, "tok", cfg.Token())
	require.Equal(t, "lab", cfg.Labels()["env"])
	n, err := cfg.MemoryLimitBytes()
	require.NoError(t, err)
	require.Equal(t, int64(64<<20), n)
	require.Equal(t, int64(4), cfg.Edge.MaxConnections)
}

func TestParseRejectsNonTokenJoin(t *testing.T) {
	_, err := Parse([]byte(`
version: v3
teleport:
  proxy_server: proxy.example.com:443
  join_params:
    token_name: tok
    method: iam
ssh_service:
  enabled: yes
`))
	require.Error(t, err)
	require.Contains(t, err.Error(), "join method")
}
