//go:build !edgeagent

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

package regular

import (
	"context"
	"errors"
	"log/slog"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/auth/authclient"
	authorizedkeysreporter "github.com/gravitational/teleport/lib/secretsscanner/authorizedkeys"
)

// startAuthorizedKeysManager starts the authorized keys manager.
func (s *Server) startAuthorizedKeysManager(ctx context.Context, auth authclient.ClientI) error {
	authorizedKeysWatcher, err := authorizedkeysreporter.NewWatcher(
		ctx,
		authorizedkeysreporter.WatcherConfig{
			Client: auth,
			Logger: slog.Default(),
			HostID: s.uuid,
			Clock:  s.clock,
		},
	)
	if errors.Is(err, authorizedkeysreporter.ErrUnsupportedPlatform) {
		return nil
	} else if err != nil {
		return trace.Wrap(err)
	}

	go func() {
		if err := authorizedKeysWatcher.Run(ctx); err != nil {
			s.logger.WarnContext(ctx, "Failed to start authorized keys watcher", "error", err)
		}
	}()
	return nil
}
