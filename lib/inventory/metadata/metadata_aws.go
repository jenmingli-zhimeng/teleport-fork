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

package metadata

import (
	"context"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils/aws"
)

func (c *fetchConfig) setCloudMetadataDefault() {
	if c.fetchCloudMetadata != nil {
		return
	}
	c.fetchCloudMetadata = func(ctx context.Context, cloudEnvironment string) *types.CloudMetadata {
		switch cloudEnvironment {
		case "aws":
			iid, err := aws.GetEC2InstanceIdentityDocument(ctx)
			if err != nil {
				return nil
			}
			return &types.CloudMetadata{
				AWS: &types.AWSInfo{
					AccountID:  iid.AccountID,
					InstanceID: iid.InstanceID,
				},
			}
		default:
			return nil
		}
	}
}
