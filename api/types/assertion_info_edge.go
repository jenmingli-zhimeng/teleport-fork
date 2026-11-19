//go:build edgeagent

/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package types

import (
	"encoding/json"

	"github.com/gravitational/trace"
)

// AssertionInfo is a protobuf customtype placeholder. teleport-edge does not
// implement SAML, so this is JSON-opaque and does not import gosaml2.
type AssertionInfo struct{}

func (a *AssertionInfo) Size() int {
	out, err := json.Marshal(a)
	if err != nil {
		return 0
	}
	return len(out)
}

func (a *AssertionInfo) Unmarshal(bytes []byte) error {
	if len(bytes) == 0 {
		return nil
	}
	return trace.Wrap(json.Unmarshal(bytes, a))
}

func (a *AssertionInfo) MarshalTo(bytes []byte) (int, error) {
	out, err := json.Marshal(a)
	if err != nil {
		return 0, trace.Wrap(err)
	}
	if len(out) > cap(bytes) {
		return 0, trace.BadParameter("capacity too low: %v, need %v", cap(bytes), len(out))
	}
	copy(bytes, out)
	return len(out), nil
}
