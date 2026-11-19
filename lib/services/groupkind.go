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

package services

import "strings"

// parseGroupKind splits "Kind.group.example.com" into kind and API group.
// Matches k8s.io/apimachinery/pkg/runtime/schema.ParseGroupKind so SSH
// access checking does not import the Kubernetes apimachinery.
func parseGroupKind(gk string) (kind, group string) {
	if i := strings.Index(gk, "."); i >= 0 {
		return gk[:i], gk[i+1:]
	}
	return gk, ""
}
