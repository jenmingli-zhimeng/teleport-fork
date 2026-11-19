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

package aws

import (
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/gravitational/trace"

	apiawsutils "github.com/gravitational/teleport/api/utils/aws"
)

const iamServiceName = "iam"

// BuildRoleARN constructs a string AWS ARN from a username, region, and account ID.
// If username is an AWS ARN, this function checks that the ARN is an AWS IAM Role ARN
// in the correct partition and account.
func BuildRoleARN(username, region, accountID string) (string, error) {
	partition := apiawsutils.GetPartitionFromRegion(region)
	if arn.IsARN(username) {
		// sanity check the given username role ARN.
		parsed, err := ParseRoleARN(username)
		if err != nil {
			return "", trace.Wrap(err)
		}
		// don't check for empty accountID - callers do not always pass an account ID,
		// and it's only absolutely required if we need to build the role ARN below.
		if err := CheckARNPartitionAndAccount(parsed, partition, accountID); err != nil {
			return "", trace.Wrap(err)
		}
		return username, nil
	}
	resource := username
	if !IsPartialRoleARN(resource) {
		resource = fmt.Sprintf("role/%s", username)
	}
	roleARN := arn.ARN{
		Partition: partition,
		Service:   iamServiceName,
		AccountID: accountID,
		Resource:  resource,
	}
	if err := apiawsutils.CheckRoleARN(roleARN.String()); err != nil {
		return "", trace.Wrap(err)
	}
	return roleARN.String(), nil
}

// ValidateRoleARNAndExtractRoleName validates the role ARN and extracts the
// short role name from it.
func ValidateRoleARNAndExtractRoleName(roleARN, wantPartition, wantAccountID string) (string, error) {
	role, err := ParseRoleARN(roleARN)
	if err != nil {
		return "", trace.Wrap(err)
	}
	if err := CheckARNPartitionAndAccount(role, wantPartition, wantAccountID); err != nil {
		return "", trace.Wrap(err)
	}
	return strings.TrimPrefix(role.Resource, "role/"), nil
}

// ParseRoleARN parses an AWS ARN and checks that the ARN is
// for an IAM Role resource.
func ParseRoleARN(roleARN string) (*arn.ARN, error) {
	role, err := arn.Parse(roleARN)
	if err != nil {
		return nil, trace.BadParameter("invalid AWS ARN: %v", err)
	}
	if err := checkRoleARN(&role); err != nil {
		return nil, trace.Wrap(err)
	}
	return &role, nil
}

// checkRoleARN returns whether a parsed ARN is for an IAM Role resource.
// Example role ARN: arn:aws:iam::123456789012:role/some-role-name
func checkRoleARN(parsed *arn.ARN) error {
	parts := strings.Split(parsed.Resource, "/")
	if parts[0] != "role" || parsed.Service != iamServiceName {
		return trace.BadParameter("%q is not an AWS IAM role ARN", parsed)
	}
	if len(parts) < 2 || len(parts[len(parts)-1]) == 0 {
		return trace.BadParameter("%q is missing AWS IAM role name", parsed)
	}
	if err := apiawsutils.IsValidAccountID(parsed.AccountID); err != nil {
		return trace.BadParameter("%q invalid account ID: %v", parsed, err)
	}
	return nil
}

// CheckARNPartitionAndAccount checks an AWS ARN against an expected AWS partition and account ID.
// An empty expected AWS partition or account ID is not checked.
func CheckARNPartitionAndAccount(ARN *arn.ARN, wantPartition, wantAccountID string) error {
	if ARN.Partition != wantPartition && wantPartition != "" {
		return trace.BadParameter("expected AWS partition %q but got %q", wantPartition, ARN.Partition)
	}
	if ARN.AccountID != wantAccountID && wantAccountID != "" {
		return trace.BadParameter("expected AWS account ID %q but got %q", wantAccountID, ARN.AccountID)
	}
	return nil
}

// IsRoleARN returns true if the provided string is a AWS role ARN.
func IsRoleARN(roleARN string) bool {
	if _, err := ParseRoleARN(roleARN); err == nil {
		return true
	}

	return IsPartialRoleARN(roleARN)
}

// IsPartialRoleARN returns true if the provided role ARN only contains the
// resource name.
func IsPartialRoleARN(roleARN string) bool {
	return strings.HasPrefix(roleARN, "role/")
}

// IsUserARN returns true if the provided string is a AWS user ARN.
func IsUserARN(userARN string) bool {
	resourceName := userARN
	if parsed, err := arn.Parse(userARN); err == nil {
		resourceName = parsed.Resource
	}

	return strings.HasPrefix(resourceName, "user/")
}

// PolicyARN returns the ARN representation of an AWS IAM Policy.
func PolicyARN(partition, accountID, policy string) string {
	return iamResourceARN(partition, accountID, "policy", policy)
}

// RoleARN returns the ARN representation of an AWS IAM Role.
func RoleARN(partition, accountID, role string) string {
	return iamResourceARN(partition, accountID, "role", role)
}

func iamResourceARN(partition, accountID, resourceType, resourceName string) string {
	return arn.ARN{
		Partition: partition,
		Service:   "iam",
		AccountID: accountID,
		Resource:  fmt.Sprintf("%s/%s", resourceType, resourceName),
	}.String()
}
