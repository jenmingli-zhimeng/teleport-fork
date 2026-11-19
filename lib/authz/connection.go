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

package authz

import (
	apievents "github.com/gravitational/teleport/api/types/events"
)

// ConnectionIdentity contains the identifying properties of a
// client connection required to enforce connection limits.
type ConnectionIdentity struct {
	// Username is the name of the user
	Username string
	// MaxConnections the upper limit to number of open connections for a user
	MaxConnections int64
	// LocalAddr is the local address for the connection
	LocalAddr string
	// RemoteAddr is the remote address for the connection
	RemoteAddr string
	// UserMetadata contains metadata for a user
	UserMetadata apievents.UserMetadata
}
