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

package regular

import (
	"context"
	"net"

	"github.com/gravitational/teleport/api/types"
)

// PROXYHeaderSigner allows to sign PROXY headers for securely propagating original client IP information
type PROXYHeaderSigner interface {
	SignPROXYHeader(source, destination net.Addr) ([]byte, error)
}

// CertAuthorityGetter allows to get cluster's host CA for verification of signed PROXY headers.
// We define our own version to avoid circular dependencies in multiplexer package (it can't depend on 'services'),
// where this function is used.
type CertAuthorityGetter = func(ctx context.Context, id types.CertAuthID, loadKeys bool) (types.CertAuthority, error)
