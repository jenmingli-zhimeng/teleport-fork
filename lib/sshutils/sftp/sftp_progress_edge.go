//go:build edgeagent

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

package sftp

import "io"

type discardProgress struct{}

func (discardProgress) Read([]byte) (int, error)    { return 0, io.EOF }
func (discardProgress) Write(p []byte) (int, error) { return len(p), nil }

func newProgressBar(int64, string, io.Writer) io.ReadWriter {
	return discardProgress{}
}
