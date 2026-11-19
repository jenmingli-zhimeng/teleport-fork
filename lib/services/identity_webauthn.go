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

package services

import (
	"context"

	wantypes "github.com/gravitational/teleport/lib/auth/webauthntypes"
)

// webauthnSessionData stores WebAuthn login/registration challenges.
type webauthnSessionData interface {
	// UpsertWebauthnSessionData creates or updates WebAuthn session data in
	// storage, for the purpose of later verifying an authentication or
	// registration challenge.
	// Session data is expected to expire according to backend settings.
	UpsertWebauthnSessionData(ctx context.Context, user, sessionID string, sd *wantypes.SessionData) error

	// GetWebauthnSessionData retrieves a previously-stored session data by ID,
	// if it exists and has not expired.
	GetWebauthnSessionData(ctx context.Context, user, sessionID string) (*wantypes.SessionData, error)

	// DeleteWebauthnSessionData deletes session data by ID, if it exists and has
	// not expired.
	DeleteWebauthnSessionData(ctx context.Context, user, sessionID string) error

	// UpsertGlobalWebauthnSessionData creates or updates WebAuthn session data in
	// storage, for the purpose of later verifying an authentication challenge.
	// Session data is expected to expire according to backend settings.
	// Used for passwordless challenges.
	UpsertGlobalWebauthnSessionData(ctx context.Context, scope, id string, sd *wantypes.SessionData) error

	// GetGlobalWebauthnSessionData retrieves previously-stored session data by ID,
	// if it exists and has not expired.
	// Used for passwordless challenges.
	GetGlobalWebauthnSessionData(ctx context.Context, scope, id string) (*wantypes.SessionData, error)

	// DeleteGlobalWebauthnSessionData deletes session data by ID, if it exists
	// and has not expired.
	DeleteGlobalWebauthnSessionData(ctx context.Context, scope, id string) error
}
