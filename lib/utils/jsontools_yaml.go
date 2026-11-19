//go:build !edgeagent

/*
Copyright 2014 The Kubernetes Authors.

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

package utils

import (
	"errors"
	"io"

	"github.com/gravitational/trace"
	kyaml "k8s.io/apimachinery/pkg/util/yaml"
)

// ReadYAML can unmarshal a stream of documents, used in tests.
func ReadYAML(reader io.Reader) (interface{}, error) {
	decoder := kyaml.NewYAMLOrJSONDecoder(reader, 32*1024)
	var values []interface{}
	for {
		var val interface{}
		err := decoder.Decode(&val)
		if err != nil {
			if errors.Is(err, io.EOF) {
				if len(values) == 0 {
					return nil, trace.BadParameter("no resources found, empty input?")
				}
				if len(values) == 1 {
					return values[0], nil
				}
				return values, nil
			}
			return nil, trace.Wrap(err)
		}
		values = append(values, val)
	}
}
