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
		return nil, err
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
