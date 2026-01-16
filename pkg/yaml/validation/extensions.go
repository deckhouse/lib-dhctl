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

	"github.com/go-openapi/spec"
	"sigs.k8s.io/yaml"
)

const (
	xRulesExtension = "x-rules"
)

type (
	ExtensionsValidatorHandler func(oldValue json.RawMessage) error
)

type ExtensionsValidator struct {
	name       string
	validators map[string]ExtensionsValidatorHandler
}

func NewExtensionsValidator(extensionName string, validators map[string]ExtensionsValidatorHandler) *ExtensionsValidator {
	if len(validators) == 0 {
		validators = make(map[string]ExtensionsValidatorHandler)
	}

	return &ExtensionsValidator{
		name:       extensionName,
		validators: validators,
	}
}

func NewXRulesExtensionsValidator(validators map[string]ExtensionsValidatorHandler) *ExtensionsValidator {
	return NewExtensionsValidator(xRulesExtension, validators)
}

func (v *ExtensionsValidator) ExtensionName() string {
	return v.name
}

func (v *ExtensionsValidator) Validate(data json.RawMessage, schema spec.Schema) error {
	if schema.Properties == nil {
		return nil
	}

	var properties map[string]json.RawMessage

	err := yaml.Unmarshal(data, &properties)
	if err != nil {
		return err
	}

	err = v.validateData(data, schema)
	if err != nil {
		return err
	}

	for field, fieldSchema := range schema.Properties {
		err = v.validateData(properties[field], fieldSchema)
		if err != nil {
			return fmt.Errorf("%s: %w", field, err)
		}

		err = v.Validate(properties[field], fieldSchema)
		if err != nil {
			return fmt.Errorf("%s: %w", field, err)
		}
	}

	return nil
}

func (v *ExtensionsValidator) validateData(data json.RawMessage, schema spec.Schema) error {
	if rules, ok := schema.Extensions.GetStringSlice(v.name); ok {
		for _, rule := range rules {
			validator, ok := v.validators[rule]
			if !ok {
				continue
			}

			err := validator(data)
			if err != nil {
				return err
			}
		}
	}

	if schema.SchemaProps.Items != nil && schema.SchemaProps.Items.Schema != nil {
		if rules, ok := schema.SchemaProps.Items.Schema.Extensions.GetStringSlice(v.name); ok {
			for _, rule := range rules {
				validator, ok := v.validators[rule]
				if !ok {
					continue
				}

				// avoid validation exception by validation empty data
				if len(data) == 0 {
					return nil
				}

				var items []json.RawMessage
				err := json.Unmarshal(data, &items)
				if err != nil {
					return err
				}

				for _, item := range items {
					err = validator(item)
					if err != nil {
						return err
					}
				}
			}
		}
	}

	return nil
}
