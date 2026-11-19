//go:build linux && !cgo

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

package metadata

import (
	"fmt"
	"strings"
)

func (c *fetchConfig) fetchOSVersion() string {
	filename := "/etc/os-release"
	out, err := c.read(filename)
	if err != nil {
		return ""
	}

	id := "linux"
	versionID := "(unknown)"
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		key, value, found := strings.Cut(line, "=")
		if !found {
			continue
		}
		switch key {
		case "ID":
			id = strings.Trim(value, `"`)
		case "VERSION_ID":
			versionID = strings.Trim(value, `"`)
		}
	}
	return fmt.Sprintf("%s %s", id, versionID)
}

func (c *fetchConfig) fetchGlibcVersion() string {
	return ""
}
