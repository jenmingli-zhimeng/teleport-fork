//go:build !windows && !cgo

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

package shell

import (
	"bufio"
	"os"
	"os/user"
	"strings"

	"github.com/gravitational/trace"
)

func getLoginShell(username string) (string, error) {
	if _, err := user.Lookup(username); err != nil {
		return "", trace.Wrap(err)
	}
	f, err := os.Open("/etc/passwd")
	if err != nil {
		return "", trace.ConvertSystemError(err)
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Split(line, ":")
		if len(parts) < 7 {
			continue
		}
		if parts[0] != username {
			continue
		}
		shellCmd := strings.TrimSpace(parts[6])
		if shellCmd == "" {
			return "", trace.NotFound("no shell specified for %v", username)
		}
		return shellCmd, nil
	}
	if err := scanner.Err(); err != nil {
		return "", trace.ConvertSystemError(err)
	}
	return "", trace.NotFound("no shell specified for %v", username)
}
