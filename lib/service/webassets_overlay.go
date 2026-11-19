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

package service

import (
	"net/http"
	"os"
	"strings"
)

const defaultWebassetsOverlay = "/etc/teleport/web-overlay"

// overlayFileSystem serves regular non-empty files from overlay, and falls
// back to base for everything else (including directories). Directory fallback
// is required so an overlay that only contains logo SVGs cannot hide the rest
// of the embedded UI under /web/app/.
type overlayFileSystem struct {
	overlay http.FileSystem
	base    http.FileSystem
}

func (o overlayFileSystem) Open(name string) (http.File, error) {
	uncompressed := strings.TrimSuffix(name, ".br")
	if uncompressed != name && overlayHasFile(o.overlay, uncompressed) {
		// Hide the baked *.br so makeBrotliHandler falls through to the
		// uncompressed overlay file (custom logos are not pre-compressed).
		return nil, os.ErrNotExist
	}
	if f, ok := openOverlayFile(o.overlay, name); ok {
		return f, nil
	}
	return o.base.Open(name)
}

func overlayHasFile(overlay http.FileSystem, name string) bool {
	f, ok := openOverlayFile(overlay, name)
	if !ok {
		return false
	}
	_ = f.Close()
	return true
}

func openOverlayFile(overlay http.FileSystem, name string) (http.File, bool) {
	if overlay == nil {
		return nil, false
	}
	f, err := overlay.Open(name)
	if err != nil {
		return nil, false
	}
	st, err := f.Stat()
	if err != nil || st.IsDir() || st.Size() == 0 {
		_ = f.Close()
		return nil, false
	}
	return f, true
}

func withWebassetsOverlay(base http.FileSystem) http.FileSystem {
	dir := os.Getenv("TELEPORT_WEBASSETS_OVERLAY")
	if dir == "" {
		dir = defaultWebassetsOverlay
	}
	st, err := os.Stat(dir)
	if err != nil || !st.IsDir() {
		return base
	}
	return overlayFileSystem{overlay: http.Dir(dir), base: base}
}
