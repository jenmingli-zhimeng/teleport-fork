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
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWebassetsOverlayServesNonEmptyFiles(t *testing.T) {
	overlay := t.TempDir()
	baseDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(overlay, "app"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(baseDir, "app"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(overlay, "app", "logo-light.svg"), []byte("<svg id='custom'/>"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(baseDir, "app", "logo-light.svg"), []byte("<svg id='baked'/>"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(baseDir, "app", "logo-light.svg.br"), []byte("baked-br"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(baseDir, "app", "other.js"), []byte("baked-js"), 0o644))
	// Empty overlay file must not hide the baked asset.
	require.NoError(t, os.WriteFile(filepath.Join(overlay, "app", "other.js"), []byte{}, 0o644))

	t.Setenv("TELEPORT_WEBASSETS_OVERLAY", overlay)
	fs := withWebassetsOverlay(http.Dir(baseDir))

	got := readFS(t, fs, "/app/logo-light.svg")
	require.Equal(t, "<svg id='custom'/>", got)

	_, err := fs.Open("/app/logo-light.svg.br")
	require.Error(t, err, "overlay must hide baked brotli so the custom SVG is served")

	got = readFS(t, fs, "/app/other.js")
	require.Equal(t, "baked-js", got)
}

func readFS(t *testing.T, fs http.FileSystem, name string) string {
	t.Helper()
	f, err := fs.Open(name)
	require.NoError(t, err)
	defer f.Close()
	b, err := io.ReadAll(f)
	require.NoError(t, err)
	return string(b)
}
