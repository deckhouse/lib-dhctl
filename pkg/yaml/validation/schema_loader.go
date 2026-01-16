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

package validation

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/deckhouse/lib-dhctl/pkg/yaml/validation/transformer"

	"github.com/go-openapi/spec"
	"sigs.k8s.io/yaml"
)

type OpenAPISchema struct {
	Kind     string                 `json:"kind"`
	Versions []OpenAPISchemaVersion `json:"apiVersions"`
}

type OpenAPISchemaVersion struct {
	Version string      `json:"apiVersion"`
	Schema  interface{} `json:"openAPISpec"`
}

type SchemaWithIndex struct {
	Schema *spec.Schema
	Index  SchemaIndex
}

func LoadSchemas(reader io.Reader) ([]*SchemaWithIndex, error) {
	fileContent, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrRead, err)
	}

	openAPISchema := new(OpenAPISchema)
	if err := yaml.UnmarshalStrict(fileContent, openAPISchema); err != nil {
		return nil, fmt.Errorf("Failed unmarshal openapi schema: %v", err)
	}

	res := make([]*SchemaWithIndex, 0)

	for _, parsedSchema := range openAPISchema.Versions {
		schema := new(spec.Schema)

		d, err := json.Marshal(parsedSchema.Schema)
		if err != nil {
			return nil, fmt.Errorf("expand the schema: %v", err)
		}

		if err := json.Unmarshal(d, schema); err != nil {
			return nil, fmt.Errorf("json marshal schema: %v", err)
		}

		err = spec.ExpandSchema(schema, schema, nil)
		if err != nil {
			return nil, fmt.Errorf("expand the schema: %v", err)
		}

		schema = transformer.TransformSchema(
			schema,
			&transformer.AdditionalPropertiesTransformer{},
		)

		res = append(res, &SchemaWithIndex{
			Schema: schema,
			Index: SchemaIndex{
				Kind:    openAPISchema.Kind,
				Version: parsedSchema.Version,
			},
		})
	}

	return res, nil
}
