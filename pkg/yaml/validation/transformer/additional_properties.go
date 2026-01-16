// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package transformer

import "github.com/go-openapi/spec"

type AdditionalPropertiesTransformer struct {
	disallowFull bool
}

func NewAdditionalPropertiesTransformer() *AdditionalPropertiesTransformer {
	return &AdditionalPropertiesTransformer{}
}

func NewAdditionalPropertiesTransformerDisallowFull() *AdditionalPropertiesTransformer {
	return &AdditionalPropertiesTransformer{
		disallowFull: true,
	}
}

func (t *AdditionalPropertiesTransformer) shouldDisallowAdditionalProperties(s *spec.Schema) bool {
	if s.AdditionalProperties == nil {
		return true
	}

	return t.disallowFull
}

func disallowAdditionalProperties(s *spec.Schema) {
	s.AdditionalProperties = &spec.SchemaOrBool{
		Allows: false,
	}
}

// Transform sets undefined AdditionalProperties to false recursively.
func (t *AdditionalPropertiesTransformer) Transform(s *spec.Schema) *spec.Schema {
	if s == nil {
		return nil
	}

	if t.shouldDisallowAdditionalProperties(s) {
		disallowAdditionalProperties(s)
	}

	for k, prop := range s.Properties {
		if t.shouldDisallowAdditionalProperties(&prop) {
			ts := prop
			disallowAdditionalProperties(&ts)
			s.Properties[k] = *t.Transform(&ts)
		}
	}

	if s.Items != nil {
		if s.Items.Schema != nil {
			s.Items.Schema = t.Transform(s.Items.Schema)
		}
		for i, item := range s.Items.Schemas {
			ts := item
			s.Items.Schemas[i] = *t.Transform(&ts)
		}
	}

	return s
}
