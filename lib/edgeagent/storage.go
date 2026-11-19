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
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/state"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/hostid"
)

const (
	identitiesDir = "identities"
	statesDir     = "states"
	stateFileName = "state.json"
)

// Storage persists process identity and rotation state as JSON files.
// It is a sqlite-free replacement for lib/auth/storage.ProcessStorage.
type Storage struct {
	dataDir string
}

// NewStorage creates a file-backed process store under dataDir.
func NewStorage(dataDir string) (*Storage, error) {
	if dataDir == "" {
		return nil, trace.BadParameter("missing data dir")
	}
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return nil, trace.ConvertSystemError(err)
	}
	return &Storage{dataDir: dataDir}, nil
}

func (s *Storage) identityPath(role types.SystemRole, name string) string {
	return filepath.Join(s.dataDir, identitiesDir, strings.ToLower(role.String()), name+".json")
}

func (s *Storage) statePath(role types.SystemRole) string {
	return filepath.Join(s.dataDir, statesDir, strings.ToLower(role.String()), stateFileName)
}

// GetState reads rotation state from disk.
func (s *Storage) GetState(_ context.Context, role types.SystemRole) (*state.StateV2, error) {
	raw, err := os.ReadFile(s.statePath(role))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, trace.NotFound("no local state for role %s", role)
		}
		return nil, trace.ConvertSystemError(err)
	}
	var res state.StateV2
	if err := utils.FastUnmarshal(raw, &res); err != nil {
		return nil, trace.BadParameter("%s", err)
	}
	if res.Spec.InitialLocalVersion == "" {
		res.Spec.InitialLocalVersion = state.UnknownLocalVersion
	}
	if err := res.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &res, nil
}

// WriteState writes local cluster state to disk.
func (s *Storage) WriteState(_ context.Context, role types.SystemRole, st state.StateV2) error {
	if err := st.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	raw, err := json.Marshal(st)
	if err != nil {
		return trace.Wrap(err)
	}
	return writeFileAtomic(s.statePath(role), raw)
}

// ReadIdentity reads identity using identity name and role.
func (s *Storage) ReadIdentity(_ context.Context, name string, role types.SystemRole) (*state.Identity, error) {
	if name == "" {
		return nil, trace.BadParameter("missing parameter name")
	}
	raw, err := os.ReadFile(s.identityPath(role, name))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, trace.NotFound("identity %q for role %s not found", name, role)
		}
		return nil, trace.ConvertSystemError(err)
	}
	var res state.IdentityV2
	if err := utils.FastUnmarshal(raw, &res); err != nil {
		return nil, trace.BadParameter("%s", err)
	}
	if err := res.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	identity, err := state.ReadIdentityFromKeyPair(res.Spec.Key, &proto.Certs{
		SSH:        res.Spec.SSHCert,
		TLS:        res.Spec.TLSCert,
		TLSCACerts: res.Spec.TLSCACerts,
		SSHCACerts: res.Spec.SSHCACerts,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	identity.ImmutableLabels = res.Spec.ImmutableLabels
	return identity, nil
}

// WriteIdentity writes identity to disk.
func (s *Storage) WriteIdentity(_ context.Context, name string, id state.Identity) error {
	res := state.IdentityV2{
		ResourceHeader: types.ResourceHeader{
			Kind:    types.KindIdentity,
			Version: types.V2,
			Metadata: types.Metadata{
				Name: name,
			},
		},
		Spec: state.IdentitySpecV2{
			Key:             id.KeyBytes,
			SSHCert:         id.CertBytes,
			TLSCert:         id.TLSCertBytes,
			TLSCACerts:      id.TLSCACertsBytes,
			SSHCACerts:      id.SSHCACertBytes,
			ImmutableLabels: id.ImmutableLabels,
		},
	}
	if err := res.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	raw, err := json.Marshal(res)
	if err != nil {
		return trace.Wrap(err)
	}
	return writeFileAtomic(s.identityPath(id.ID.Role, name), raw)
}

// PersistHostID writes the assigned host UUID to host_uuid.
func (s *Storage) PersistHostID(hostID string) error {
	return trace.Wrap(hostid.WriteFile(s.dataDir, hostID))
}

func writeFileAtomic(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return trace.ConvertSystemError(err)
	}
	tmp := path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return trace.ConvertSystemError(err)
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return trace.ConvertSystemError(err)
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return trace.ConvertSystemError(err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return trace.ConvertSystemError(err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return trace.ConvertSystemError(err)
	}
	return nil
}
