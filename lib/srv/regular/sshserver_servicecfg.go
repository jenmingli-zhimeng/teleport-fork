//go:build !edgeagent

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

package regular

import (
	"strings"

	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/srv"
	"github.com/gravitational/teleport/session/reexec"
)

// SetChildLogConfig sets the config that will be used to handle logs
// from child processes.
func SetChildLogConfig(cfg *servicecfg.Config) ServerOption {
	return SetChildLogConfigFrom(&srv.ChildLogConfig{
		ExecLogConfig: reexec.ExecLogConfig{
			Level:        cfg.LoggerLevel,
			Format:       strings.ToLower(cfg.LogConfig.Format),
			ExtraFields:  cfg.LogConfig.ExtraFields,
			EnableColors: cfg.LogConfig.EnableColors,
			Padding:      cfg.LogConfig.Padding,
		},
		Writer: cfg.LogWriter,
	})
}
