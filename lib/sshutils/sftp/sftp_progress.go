//go:build !edgeagent

/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package sftp

import (
	"fmt"
	"io"
	"time"

	"github.com/schollz/progressbar/v3"
)

// unboundedProgressBar is a wrapper for a progress bar that increases its max
// value when its internal count gets too big, instead of failing.
type unboundedProgressBar struct {
	pb *progressbar.ProgressBar
}

func (u *unboundedProgressBar) checkMax(n int) {
	state := u.pb.State()
	newNum := state.CurrentNum + int64(n)
	if newNum > state.Max {
		u.pb.ChangeMax64(newNum)
	}
}

func (u *unboundedProgressBar) Read(p []byte) (int, error) {
	u.checkMax(len(p))
	return u.pb.Read(p)
}

func (u *unboundedProgressBar) Write(p []byte) (int, error) {
	u.checkMax(len(p))
	return u.pb.Write(p)
}

// newProgressBar returns a new progress bar that writes to writer.
func newProgressBar(size int64, desc string, writer io.Writer) io.ReadWriter {
	// this is necessary because progressbar.DefaultBytes doesn't allow
	// the caller to specify a writer
	return &unboundedProgressBar{pb: progressbar.NewOptions64(
		size,
		progressbar.OptionSetDescription(desc),
		progressbar.OptionSetWriter(writer),
		progressbar.OptionShowBytes(true),
		progressbar.OptionSetWidth(10),
		progressbar.OptionThrottle(100*time.Millisecond),
		progressbar.OptionShowCount(),
		progressbar.OptionOnCompletion(func() {
			fmt.Fprint(writer, "\n")
		}),
		progressbar.OptionSpinnerType(14),
		progressbar.OptionFullWidth(),
		progressbar.OptionSetRenderBlankState(true),
	)}
}
