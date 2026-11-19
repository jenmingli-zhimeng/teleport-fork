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
	"log/slog"
	"slices"

	"github.com/crewjam/saml"
	"github.com/crewjam/saml/samlsp"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
)

// supportedACSBindings is the set of AssertionConsumerService bindings that teleport supports.
var supportedACSBindings = map[string]struct{}{
	saml.HTTPPostBinding:     {},
	saml.HTTPRedirectBinding: {},
}

// ValidateAssertionConsumerService checks if a given assertion consumer service is usable by teleport. Note that
// it is permissible for a service provider to include acs endpoints that are not compatible with teleport, so long
// as at least one _is_ compatible.
func ValidateAssertionConsumerService(acs saml.IndexedEndpoint) error {
	if _, ok := supportedACSBindings[acs.Binding]; !ok {
		return trace.BadParameter("unsupported acs binding: %q", acs.Binding)
	}

	if acs.Location == "" {
		return trace.BadParameter("acs location endpoint is missing or could not be decoded for %q binding", acs.Binding)
	}

	return trace.Wrap(validateAssertionConsumerServicesEndpoint(acs.Location))
}

// FilterSAMLEntityDescriptor performs a filter in place to remove unsupported and/or insecure fields from
// a saml entity descriptor. Specifically, it removes acs endpoints that are either of an unsupported kind,
// or are using a non-https endpoint. We perform filtering rather than outright rejection because it is generally
// expected that a service provider will successfully support a given ACS so long as they have at least one
// compatible binding.
func FilterSAMLEntityDescriptor(ed *saml.EntityDescriptor, quiet bool) error {
	var originalCount int
	var filteredCount int
	for i := range ed.SPSSODescriptors {
		filtered := slices.DeleteFunc(ed.SPSSODescriptors[i].AssertionConsumerServices, func(acs saml.IndexedEndpoint) bool {
			if err := ValidateAssertionConsumerService(acs); err != nil {
				if !quiet {
					slog.WarnContext(context.Background(), "AssertionConsumerService binding for entity is invalid and will be ignored",
						"entity_id", ed.EntityID,
						"error", err,
					)
				}
				return true
			}

			return false
		})

		originalCount += len(ed.SPSSODescriptors[i].AssertionConsumerServices)
		filteredCount += len(filtered)

		ed.SPSSODescriptors[i].AssertionConsumerServices = filtered
	}

	if filteredCount != originalCount {
		return trace.BadParameter("Entity descriptor for entity %q contains unsupported AssertionConsumerService binding or location", ed.EntityID)
	}

	return nil
}

// SAMLACSInputFilteringThreshold defines level of strictness for entity descriptor filtering.
type SAMLACSInputFilteringThreshold string

const (
	// SAMLACSInputStrictFilter indicates ValidateAndFilterEntityDescriptor to return an error on
	// any instance of unsupported ACS value.
	SAMLACSInputStrictFilter SAMLACSInputFilteringThreshold = "SAMLACSInputStrictFilter"
	// SAMLACSInputPermissiveFilter indicates ValidateAndFilterEntityDescriptor to ignore an error on
	// any instance of unsupported ACS value.
	SAMLACSInputPermissiveFilter SAMLACSInputFilteringThreshold = "SAMLACSInputPermissiveFilter"
)

// ValidateAndFilterEntityDescriptor validates entity id and ACS value. It specifically:
//   - checks for a valid entity descriptor XML format.
//   - checks for a matching entity ID field in both the entity_id field and entity ID contained in the value of
//     entity_descriptor field.
//   - performs filtering on the Assertion Consumer service (ACS) binding format or its location URL endpoint.
//     filterThreshold dictates if ValidateAndFilterEntityDescriptor should return or ignore error on filtering result.
func ValidateAndFilterEntityDescriptor(sp types.SAMLIdPServiceProvider, filterThreshold SAMLACSInputFilteringThreshold) error {
	edOriginal, err := samlsp.ParseMetadata([]byte(sp.GetEntityDescriptor()))
	if err != nil {
		return trace.BadParameter("invalid entity descriptor for SAML IdP Service Provider %q: %v", sp.GetEntityID(), err)
	}

	if edOriginal.EntityID != sp.GetEntityID() {
		return trace.BadParameter("entity ID parsed from the entity descriptor does not match the entity ID in the SAML IdP service provider object")
	}

	if err := FilterSAMLEntityDescriptor(edOriginal, false /* quiet */); err != nil {
		if filterThreshold == SAMLACSInputStrictFilter {
			return trace.BadParameter("Entity descriptor for SAML IdP Service Provider %q contains unsupported ACS bindings: %v", sp.GetEntityID(), err)
		}
	}

	return nil
}
