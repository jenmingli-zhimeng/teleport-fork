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
	"os"
	"runtime/debug"

	"github.com/gravitational/trace"
)

const defaultGOGC = 50

// ApplyMemoryLimit sets Go's soft memory limit from config unless GOMEMLIMIT
// is already set in the environment. GOGC is lowered to reduce idle RSS unless
// the process already defined it.
func ApplyMemoryLimit(cfg *Config) error {
	if os.Getenv("GOGC") == "" {
		debug.SetGCPercent(defaultGOGC)
	}
	if os.Getenv("GOMEMLIMIT") != "" {
		return nil
	}
	limit, err := cfg.MemoryLimitBytes()
	if err != nil {
		return trace.Wrap(err)
	}
	debug.SetMemoryLimit(limit)
	return nil
}
