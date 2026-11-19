//go:build linux && !edgeagent

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

package reexec

import (
	"runtime"

	"github.com/gravitational/trace"
	ocselinux "github.com/opencontainers/selinux/go-selinux"

	"github.com/gravitational/teleport/session/selinux"
)

func setChildSELinuxContext(login string) error {
	seContext, err := selinux.UserContext(login)
	if err != nil {
		return trace.Wrap(err, "failed to get SELinux context of login user")
	}

	// SetExecLabel changes the SELinux exec context for the
	// calling thread only, so we need to ensure that is the
	// thread that will create the child. We don't ever unlock
	// the thread as we're exiting after the child exits, and
	// we want to avoid another goroutine getting denied due to
	// running on this thread with a different (likely much more
	// restrictive)SELinux context.
	runtime.LockOSThread()
	if err := ocselinux.SetExecLabel(seContext); err != nil {
		return trace.Wrap(err, "failed to set SELinux context")
	}
	return nil
}
