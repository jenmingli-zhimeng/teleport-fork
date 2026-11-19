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

package edgeagent

import (
	"context"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/state"
)

func TestStorageStateRoundTrip(t *testing.T) {
	dir := t.TempDir()
	s, err := NewStorage(dir)
	require.NoError(t, err)

	ctx := context.Background()
	_, err = s.GetState(ctx, types.RoleNode)
	require.True(t, trace.IsNotFound(err))

	st := state.StateV2{
		Spec: state.StateSpecV2{
			Rotation:            types.Rotation{State: types.RotationStateStandby},
			InitialLocalVersion: teleport.Version,
		},
	}
	require.NoError(t, s.WriteState(ctx, types.RoleNode, st))
	got, err := s.GetState(ctx, types.RoleNode)
	require.NoError(t, err)
	require.Equal(t, types.RotationStateStandby, got.Spec.Rotation.State)
}

func TestPersistHostID(t *testing.T) {
	dir := t.TempDir()
	s, err := NewStorage(dir)
	require.NoError(t, err)
	require.NoError(t, s.PersistHostID("abc-123"))
}
