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
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
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
	require.Nil(t, cfg.CmdLabels())
	n, err := cfg.MemoryLimitBytes()
	require.NoError(t, err)
	require.Equal(t, int64(64<<20), n)
	require.Equal(t, int64(4), cfg.Edge.MaxConnections)
}

func TestParseSSHCommands(t *testing.T) {
	cfg, err := Parse([]byte(`
version: v3
teleport:
  proxy_server: proxy.example.com:443
  join_params:
    token_name: tok
    method: token
ssh_service:
  enabled: yes
  commands:
    - name: hostname
      command: [hostname]
      period: 1m0s
    - name: arch
      command: ['/bin/uname', '-m']
      period: 1h
`))
	require.NoError(t, err)
	require.Len(t, cfg.SSH.Commands, 2)
	require.Equal(t, "hostname", cfg.SSH.Commands[0].Name)
	require.Equal(t, []string{"hostname"}, cfg.SSH.Commands[0].Command)
	require.Equal(t, time.Minute, cfg.SSH.Commands[0].Period)

	labels := cfg.CmdLabels()
	require.Equal(t, []string{"hostname"}, labels["hostname"].GetCommand())
	require.Equal(t, time.Minute, labels["hostname"].GetPeriod())
	require.Equal(t, []string{"/bin/uname", "-m"}, labels["arch"].GetCommand())
	require.Equal(t, time.Hour, labels["arch"].GetPeriod())
	require.IsType(t, (*types.CommandLabelV2)(nil), labels["hostname"])
}

func TestParseRejectsInvalidSSHCommands(t *testing.T) {
	t.Run("empty command", func(t *testing.T) {
		_, err := Parse([]byte(`
version: v3
teleport:
  proxy_server: proxy.example.com:443
  join_params:
    token_name: tok
    method: token
ssh_service:
  enabled: yes
  commands:
    - name: hostname
      command: []
      period: 1m0s
`))
		require.Error(t, err)
		require.Contains(t, err.Error(), "command is required")
	})

	t.Run("empty name", func(t *testing.T) {
		_, err := Parse([]byte(`
version: v3
teleport:
  proxy_server: proxy.example.com:443
  join_params:
    token_name: tok
    method: token
ssh_service:
  enabled: yes
  commands:
    - name: ""
      command: [hostname]
      period: 1m0s
`))
		require.Error(t, err)
		require.Contains(t, err.Error(), "name is required")
	})

	t.Run("duplicate name", func(t *testing.T) {
		_, err := Parse([]byte(`
version: v3
teleport:
  proxy_server: proxy.example.com:443
  join_params:
    token_name: tok
    method: token
ssh_service:
  enabled: yes
  commands:
    - name: hostname
      command: [hostname]
      period: 1m0s
    - name: hostname
      command: [uname]
      period: 1m0s
`))
		require.Error(t, err)
		require.Contains(t, err.Error(), "duplicate name")
	})

	t.Run("unknown field", func(t *testing.T) {
		_, err := Parse([]byte(`
version: v3
teleport:
  proxy_server: proxy.example.com:443
  join_params:
    token_name: tok
    method: token
ssh_service:
  enabled: yes
  commands:
    - name: hostname
      command: [hostname]
      period: 1m0s
      extra: no
`))
		require.Error(t, err)
		require.Contains(t, err.Error(), `unsupported config field "ssh_service.commands[0].extra"`)
	})
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
