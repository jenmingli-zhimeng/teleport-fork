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
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/gravitational/trace"
	"gopkg.in/yaml.v2"

	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils"
)

const (
	// DefaultMemoryLimitBytes is applied when the config does not set a
	// memory limit. 128MiB is a conservative default for 512MiB-class devices.
	DefaultMemoryLimitBytes int64 = 128 << 20
	// DefaultMaxConnections is the default concurrent SSH connection cap.
	DefaultMaxConnections int64 = 16
)

// Config is the supported subset of teleport.yaml for teleport-edge.
type Config struct {
	Version  string         `yaml:"version,omitempty"`
	Teleport TeleportConfig `yaml:"teleport,omitempty"`
	SSH      SSHConfig      `yaml:"ssh_service,omitempty"`
	Edge     EdgeConfig     `yaml:"edge_service,omitempty"`
}

// TeleportConfig holds the teleport: section.
type TeleportConfig struct {
	Nodename    string            `yaml:"nodename,omitempty"`
	DataDir     string            `yaml:"data_dir,omitempty"`
	ProxyServer string            `yaml:"proxy_server,omitempty"`
	AuthToken   string            `yaml:"auth_token,omitempty"`
	JoinParams  JoinParams        `yaml:"join_params,omitempty"`
	Log         LogConfig         `yaml:"log,omitempty"`
	CAPin       apiutils.Strings  `yaml:"ca_pin,omitempty"`
	Insecure    bool              `yaml:"insecure,omitempty"`
	Cache       CacheConfig       `yaml:"cache,omitempty"`
	Labels      map[string]string `yaml:"labels,omitempty"`
}

// JoinParams is the teleport.join_params subset. Only the token method is
// supported.
type JoinParams struct {
	TokenName string `yaml:"token_name,omitempty"`
	Method    string `yaml:"method,omitempty"`
}

// LogConfig is the teleport.log subset. Logs are written to stderr so that
// OpenWrt procd/logd can collect them.
type LogConfig struct {
	Output   string    `yaml:"output,omitempty"`
	Severity string    `yaml:"severity,omitempty"`
	Format   LogFormat `yaml:"format,omitempty"`
}

// LogFormat is teleport.log.format.
type LogFormat struct {
	Output      string   `yaml:"output,omitempty"`
	ExtraFields []string `yaml:"extra_fields,omitempty"`
}

// CacheConfig toggles the node cache. Disabled by default to save memory.
type CacheConfig struct {
	Enabled bool `yaml:"enabled,omitempty"`
}

// SSHConfig is the ssh_service subset.
type SSHConfig struct {
	Enabled  string            `yaml:"enabled,omitempty"`
	Labels   map[string]string `yaml:"labels,omitempty"`
	Commands []CommandLabel    `yaml:"commands,omitempty"`
}

// CommandLabel is one ssh_service.commands entry. The command is executed
// periodically and its stdout is reported as a node label (device tag).
type CommandLabel struct {
	Name    string        `yaml:"name"`
	Command []string      `yaml:"command,flow"`
	Period  time.Duration `yaml:"period"`
}

// EdgeConfig holds teleport-edge specific knobs that are not in upstream yaml.
type EdgeConfig struct {
	// MemoryLimit is a Go GOMEMLIMIT-style budget, for example "128MiB".
	MemoryLimit string `yaml:"memory_limit,omitempty"`
	// MaxConnections caps concurrent SSH connections (0 uses the default).
	MaxConnections int64 `yaml:"max_connections,omitempty"`
}

var allowedTopLevel = map[string]struct{}{
	"version":      {},
	"teleport":     {},
	"ssh_service":  {},
	"edge_service": {},
}

var allowedTeleport = map[string]struct{}{
	"nodename":     {},
	"data_dir":     {},
	"proxy_server": {},
	"auth_token":   {},
	"join_params":  {},
	"log":          {},
	"ca_pin":       {},
	"insecure":     {},
	"cache":        {},
	"labels":       {},
}

var allowedSSH = map[string]struct{}{
	"enabled":  {},
	"labels":   {},
	"commands": {},
}

var allowedCommand = map[string]struct{}{
	"name":    {},
	"command": {},
	"period":  {},
}

var allowedEdge = map[string]struct{}{
	"memory_limit":    {},
	"max_connections": {},
}

var allowedJoin = map[string]struct{}{
	"token_name": {},
	"method":     {},
}

var allowedLog = map[string]struct{}{
	"output":   {},
	"severity": {},
	"format":   {},
}

// LoadFile reads and validates a teleport-edge YAML config.
func LoadFile(path string) (*Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, trace.ConvertSystemError(err)
	}
	return Parse(raw)
}

// Parse validates YAML and returns a Config with defaults applied.
func Parse(raw []byte) (*Config, error) {
	var generic map[string]any
	if err := yaml.Unmarshal(raw, &generic); err != nil {
		return nil, trace.BadParameter("invalid yaml: %v", err)
	}
	if err := rejectUnknown(generic, allowedTopLevel, ""); err != nil {
		return nil, err
	}
	if t, ok := generic["teleport"].(map[any]any); ok {
		if err := rejectUnknownMap(t, allowedTeleport, "teleport"); err != nil {
			return nil, err
		}
		if jp, ok := t["join_params"].(map[any]any); ok {
			if err := rejectUnknownMap(jp, allowedJoin, "teleport.join_params"); err != nil {
				return nil, err
			}
		}
		if log, ok := t["log"].(map[any]any); ok {
			if err := rejectUnknownMap(log, allowedLog, "teleport.log"); err != nil {
				return nil, err
			}
		}
	}
	if ssh, ok := generic["ssh_service"].(map[any]any); ok {
		if err := rejectUnknownMap(ssh, allowedSSH, "ssh_service"); err != nil {
			return nil, err
		}
		if cmds, ok := ssh["commands"]; ok {
			if err := rejectUnknownCommands(cmds); err != nil {
				return nil, err
			}
		}
	}
	if edge, ok := generic["edge_service"].(map[any]any); ok {
		if err := rejectUnknownMap(edge, allowedEdge, "edge_service"); err != nil {
			return nil, err
		}
	}

	cfg := &Config{}
	if err := yaml.Unmarshal(raw, cfg); err != nil {
		return nil, trace.BadParameter("invalid yaml: %v", err)
	}
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func rejectUnknown(m map[string]any, allowed map[string]struct{}, prefix string) error {
	for k := range m {
		if _, ok := allowed[k]; !ok {
			if prefix == "" {
				return trace.BadParameter("unsupported config field %q (teleport-edge only supports Shell, file transfer, and TCP port forwarding)", k)
			}
			return trace.BadParameter("unsupported config field %q", prefix+"."+k)
		}
	}
	return nil
}

func rejectUnknownMap(m map[any]any, allowed map[string]struct{}, prefix string) error {
	for k := range m {
		key, ok := k.(string)
		if !ok {
			return trace.BadParameter("invalid key under %s", prefix)
		}
		if _, ok := allowed[key]; !ok {
			return trace.BadParameter("unsupported config field %q", prefix+"."+key)
		}
	}
	return nil
}

func rejectUnknownCommands(cmds any) error {
	list, ok := cmds.([]any)
	if !ok {
		return trace.BadParameter("ssh_service.commands must be a list")
	}
	for i, item := range list {
		m, ok := item.(map[any]any)
		if !ok {
			return trace.BadParameter("ssh_service.commands[%d] must be a map", i)
		}
		if err := rejectUnknownMap(m, allowedCommand, fmt.Sprintf("ssh_service.commands[%d]", i)); err != nil {
			return err
		}
	}
	return nil
}

// CheckAndSetDefaults validates Config and fills in defaults.
func (c *Config) CheckAndSetDefaults() error {
	if c.Version != "" && c.Version != defaults.TeleportConfigVersionV3 && c.Version != "v3" {
		return trace.BadParameter("teleport-edge requires version: v3")
	}
	c.Version = defaults.TeleportConfigVersionV3

	if c.Teleport.ProxyServer == "" {
		return trace.BadParameter("teleport.proxy_server is required")
	}
	if _, err := utils.ParseAddr(c.Teleport.ProxyServer); err != nil {
		return trace.Wrap(err, "parsing teleport.proxy_server")
	}

	token := c.Token()
	if token == "" {
		return trace.BadParameter("a join token is required (teleport.join_params.token_name, teleport.auth_token, or TELEPORT_JOIN_TOKEN)")
	}

	method := types.JoinMethod(c.Teleport.JoinParams.Method)
	if method == "" {
		method = types.JoinMethodToken
		c.Teleport.JoinParams.Method = string(method)
	}
	if method != types.JoinMethodToken {
		return trace.BadParameter("teleport-edge only supports join method %q, got %q", types.JoinMethodToken, method)
	}

	if c.Teleport.DataDir == "" {
		c.Teleport.DataDir = defaults.DataDir
	}
	if c.Teleport.Nodename == "" {
		hostname, err := os.Hostname()
		if err != nil {
			return trace.Wrap(err, "resolving hostname")
		}
		c.Teleport.Nodename = hostname
	}
	if c.Teleport.Log.Output == "" {
		c.Teleport.Log.Output = "stderr"
	}
	if c.Teleport.Log.Output != "stderr" && c.Teleport.Log.Output != "stdout" {
		return trace.BadParameter("teleport-edge only supports log output stderr or stdout (use procd/logd for collection)")
	}
	if c.Teleport.Log.Severity == "" {
		c.Teleport.Log.Severity = "info"
	}
	if c.Teleport.Log.Format.Output == "" {
		c.Teleport.Log.Format.Output = "text"
	}

	enabled := strings.ToLower(c.SSH.Enabled)
	if enabled != "" && enabled != "yes" && enabled != "true" && enabled != "on" {
		return trace.BadParameter("ssh_service must be enabled")
	}
	c.SSH.Enabled = "yes"

	if err := c.validateCommands(); err != nil {
		return err
	}

	if c.Edge.MaxConnections <= 0 {
		c.Edge.MaxConnections = DefaultMaxConnections
	}
	return nil
}

func (c *Config) validateCommands() error {
	seen := make(map[string]struct{}, len(c.SSH.Commands))
	for i, cmd := range c.SSH.Commands {
		if cmd.Name == "" {
			return trace.BadParameter("ssh_service.commands[%d].name is required", i)
		}
		if !types.IsValidLabelKey(cmd.Name) {
			return trace.BadParameter("ssh_service.commands[%d] has invalid label key %q", i, cmd.Name)
		}
		if len(cmd.Command) == 0 {
			return trace.BadParameter("ssh_service.commands[%d] (%s) command is required", i, cmd.Name)
		}
		if _, dup := seen[cmd.Name]; dup {
			return trace.BadParameter("ssh_service.commands has duplicate name %q", cmd.Name)
		}
		seen[cmd.Name] = struct{}{}
	}
	return nil
}

// Token returns the join token from config or the environment.
func (c *Config) Token() string {
	if c.Teleport.JoinParams.TokenName != "" {
		return c.Teleport.JoinParams.TokenName
	}
	if c.Teleport.AuthToken != "" {
		return c.Teleport.AuthToken
	}
	if t := os.Getenv("TELEPORT_JOIN_TOKEN"); t != "" {
		return t
	}
	return os.Getenv("TELEPORT_AUTH_TOKEN")
}

// Labels returns static SSH labels from both teleport.labels and ssh_service.labels.
func (c *Config) Labels() map[string]string {
	out := make(map[string]string, len(c.Teleport.Labels)+len(c.SSH.Labels))
	for k, v := range c.Teleport.Labels {
		out[k] = v
	}
	for k, v := range c.SSH.Labels {
		out[k] = v
	}
	return out
}

// CmdLabels converts ssh_service.commands into the command labels used to
// report periodic device tags on the SSH server.
func (c *Config) CmdLabels() map[string]types.CommandLabel {
	if len(c.SSH.Commands) == 0 {
		return nil
	}
	out := make(map[string]types.CommandLabel, len(c.SSH.Commands))
	for _, cmd := range c.SSH.Commands {
		out[cmd.Name] = &types.CommandLabelV2{
			Period:  types.NewDuration(cmd.Period),
			Command: append([]string(nil), cmd.Command...),
		}
	}
	return out
}

// MemoryLimitBytes returns the process memory budget.
func (c *Config) MemoryLimitBytes() (int64, error) {
	if c.Edge.MemoryLimit == "" {
		return DefaultMemoryLimitBytes, nil
	}
	n, err := parseByteSize(c.Edge.MemoryLimit)
	if err != nil {
		return 0, trace.Wrap(err, "parsing edge_service.memory_limit")
	}
	if n <= 0 {
		return 0, trace.BadParameter("edge_service.memory_limit must be positive")
	}
	return n, nil
}

func parseByteSize(s string) (int64, error) {
	s = strings.TrimSpace(s)
	multipliers := []struct {
		suffix string
		n      int64
	}{
		{"GiB", 1 << 30},
		{"GB", 1_000_000_000},
		{"MiB", 1 << 20},
		{"MB", 1_000_000},
		{"KiB", 1 << 10},
		{"KB", 1_000},
		{"B", 1},
	}
	for _, m := range multipliers {
		if strings.HasSuffix(s, m.suffix) {
			var v int64
			if _, err := fmt.Sscanf(strings.TrimSuffix(s, m.suffix), "%d", &v); err != nil {
				return 0, trace.BadParameter("invalid size %q", s)
			}
			return v * m.n, nil
		}
	}
	var v int64
	if _, err := fmt.Sscanf(s, "%d", &v); err != nil {
		return 0, trace.BadParameter("invalid size %q", s)
	}
	return v, nil
}
