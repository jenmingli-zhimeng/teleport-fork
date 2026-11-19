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

package services

import (
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
)

func init() {
	RegisterResourceMarshaler(types.KindGithubConnector, func(resource types.Resource, opts ...MarshalOption) ([]byte, error) {
		githubConnector, ok := resource.(types.GithubConnector)
		if !ok {
			return nil, trace.BadParameter("expected GithubConnector, got %T", resource)
		}
		bytes, err := MarshalOSSGithubConnector(githubConnector, opts...)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return bytes, nil
	})
	RegisterResourceUnmarshaler(types.KindGithubConnector, func(bytes []byte, opts ...MarshalOption) (types.Resource, error) {
		githubConnector, err := UnmarshalOSSGithubConnector(bytes, opts...)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return githubConnector, nil
	})

	RegisterResourceMarshaler(types.KindSAMLConnector, func(resource types.Resource, opts ...MarshalOption) ([]byte, error) {
		samlConnector, ok := resource.(types.SAMLConnector)
		if !ok {
			return nil, trace.BadParameter("expected SAMLConnector, got %T", resource)
		}
		bytes, err := MarshalSAMLConnector(samlConnector, opts...)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return bytes, nil
	})
	RegisterResourceUnmarshaler(types.KindSAMLConnector, func(bytes []byte, opts ...MarshalOption) (types.Resource, error) {
		samlConnector, err := UnmarshalSAMLConnector(bytes, opts...)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return samlConnector, nil
	})

	RegisterResourceMarshaler(types.KindOIDCConnector, func(resource types.Resource, opts ...MarshalOption) ([]byte, error) {
		oidConnector, ok := resource.(types.OIDCConnector)
		if !ok {
			return nil, trace.BadParameter("expected OIDCConnector, got %T", resource)
		}
		bytes, err := MarshalOIDCConnector(oidConnector, opts...)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return bytes, nil
	})
	RegisterResourceUnmarshaler(types.KindOIDCConnector, func(bytes []byte, opts ...MarshalOption) (types.Resource, error) {
		oidcConnector, err := UnmarshalOIDCConnector(bytes, opts...)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return oidcConnector, nil
	})
}
