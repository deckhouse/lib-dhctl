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

	"github.com/deckhouse/lib-dhctl/pkg/log"
	"github.com/deckhouse/lib-dhctl/pkg/yaml/validation/transformer"

	"github.com/go-openapi/spec"
	"github.com/go-openapi/strfmt"
	"github.com/go-openapi/validate"
	"github.com/go-openapi/validate/post"
	"github.com/hashicorp/go-multierror"
	"github.com/name212/govalue"
	"sigs.k8s.io/yaml"
)

var (
	ErrSchemaNotFound = fmt.Errorf("schema not found")
)

type validateOptions struct {
	omitDocInError  bool
	strictUnmarshal bool
	noPrettyError   bool
}

type ValidateOption func(o *validateOptions)

func ValidateWithOmitDocInError(v bool) ValidateOption {
	return func(o *validateOptions) {
		o.omitDocInError = v
	}
}

func ValidateWithStrictUnmarshal(v bool) ValidateOption {
	return func(o *validateOptions) {
		o.strictUnmarshal = v
	}
}

func ValidateWithNoPrettyError(v bool) ValidateOption {
	return func(o *validateOptions) {
		o.noPrettyError = v
	}
}

type PreValidator interface {
	// Validate
	// currentSchema can be nil
	// if validator does not provide our own schema please return currentSchema
	Validate(doc []byte, currentSchema *spec.Schema, logger log.Logger) (*spec.Schema, error)
}

type Validator struct {
	schemas              map[SchemaIndex]*spec.Schema
	preValidators        map[SchemaIndex]PreValidator
	loggerProvider       log.LoggerProvider
	versionFallbacks     map[string]string
	transformers         map[SchemaIndex][]transformer.SchemaTransformer
	defaultTransformers  []transformer.SchemaTransformer
	extensionsValidators []*ExtensionsValidator
}

func NewValidator(schemas map[SchemaIndex]*spec.Schema) *Validator {
	return NewValidatorWithLogger(schemas, log.SilentLoggerProvider())
}

func NewValidatorWithLogger(schemas map[SchemaIndex]*spec.Schema, loggerProvider log.LoggerProvider) *Validator {
	if len(schemas) == 0 {
		schemas = make(map[SchemaIndex]*spec.Schema)
	}
	return &Validator{
		schemas:        schemas,
		loggerProvider: loggerProvider,
		preValidators:  make(map[SchemaIndex]PreValidator),
		versionFallbacks: map[string]string{
			"deckhouse.io/v1alpha1": "deckhouse.io/v1",
		},
		defaultTransformers:  make([]transformer.SchemaTransformer, 0),
		transformers:         make(map[SchemaIndex][]transformer.SchemaTransformer),
		extensionsValidators: make([]*ExtensionsValidator, 0),
	}
}

func (v *Validator) AddSchema(index SchemaIndex, schema *spec.Schema) *Validator {
	v.schemas[index] = schema
	return v
}

func (v *Validator) LoadSchemas(reader io.Reader) error {
	schemas, err := LoadSchemas(reader)
	if err != nil {
		return err
	}

	for _, sc := range schemas {
		v.AddSchema(sc.Index, sc.Schema)
	}

	return nil
}

func (v *Validator) AddPreValidator(index SchemaIndex, validator PreValidator) *Validator {
	v.preValidators[index] = validator
	return v
}

func (v *Validator) AddExtensionsValidators(validators ...*ExtensionsValidator) *Validator {
	v.extensionsValidators = append(v.extensionsValidators, validators...)
	return v
}

func (v *Validator) AddVersionFallback(failVersion, fallback string) *Validator {
	v.versionFallbacks[failVersion] = fallback
	return v
}

func (v *Validator) SetLogger(loggerProvider log.LoggerProvider) *Validator {
	v.loggerProvider = loggerProvider

	return v
}

func (v *Validator) AddTransformers(index SchemaIndex, t ...transformer.SchemaTransformer) *Validator {
	res := make([]transformer.SchemaTransformer, 0, len(v.transformers))
	v.transformers[index] = append(res, t...)
	return v
}

func (v *Validator) SetDefaultTransformers(transformers ...transformer.SchemaTransformer) *Validator {
	res := make([]transformer.SchemaTransformer, 0, len(v.transformers))
	v.defaultTransformers = append(res, transformers...)

	return v
}

func (v *Validator) Get(index *SchemaIndex) *spec.Schema {
	return v.schemas[*index]
}

func (v *Validator) Validate(doc *[]byte, opts ...ValidateOption) (*SchemaIndex, error) {
	var index SchemaIndex

	err := yaml.Unmarshal(*doc, &index)
	if err != nil {
		return nil, fmt.Errorf("Schema index unmarshal failed: %w", err)
	}

	err = v.ValidateWithIndex(&index, doc, opts...)
	return &index, err
}

// ValidateWithIndex
// validate one document with schema
// if schema not fount then return ErrSchemaNotFound
func (v *Validator) ValidateWithIndex(index *SchemaIndex, doc *[]byte, opts ...ValidateOption) error {
	options := &validateOptions{}
	for _, opt := range opts {
		opt(options)
	}

	if !index.IsValid() {
		return fmt.Errorf(
			"document must contain \"kind\" and \"apiVersion\" fields:\n\tapiVersion: %s\n\tkind: %s\n\n%s",
			index.Version, index.Kind, string(*doc),
		)
	}

	logger := log.SafeProvideLogger(v.loggerProvider)

	docForValidate := *doc

	schema := v.getSchemaWithFallback(index, logger)

	preValidator, ok := v.preValidators[*index]
	if ok && !govalue.IsNil(preValidator) {
		var err error
		schema, err = preValidator.Validate(docForValidate, schema, logger)
		if err != nil {
			return err
		}
	}

	if schema == nil {
		logger.DebugF("No schema for index %s. Skip it", index.String())
		// we need return error because on top level we want filter documents without index and move into resources
		return ErrSchemaNotFound
	}

	schema = v.addTransformersForSchema(index, schema)

	isValid, err := v.openAPIValidate(&docForValidate, schema, options)
	if !isValid {
		if options.omitDocInError || options.noPrettyError {
			return fmt.Errorf("%q document validation failed: %w", index.String(), err)
		}
		return fmt.Errorf("Document validation failed:\n---\n%s\n\n%w", string(*doc), err)
	}

	*doc = docForValidate

	return nil
}

func (v *Validator) addTransformersForSchema(index *SchemaIndex, schema *spec.Schema) *spec.Schema {
	transformers := v.transformers[*index]
	if len(transformers) == 0 {
		transformers = v.defaultTransformers
	}

	if len(transformers) == 0 {
		return schema
	}

	for _, t := range transformers {
		if govalue.IsNil(t) {
			continue
		}

		schema = t.Transform(schema)
	}

	return schema
}

func (v *Validator) getSchemaWithFallback(index *SchemaIndex, logger log.Logger) *spec.Schema {
	schema := v.Get(index)
	if schema != nil {
		return schema
	}

	fallback, ok := v.versionFallbacks[index.Version]
	if !ok || fallback == "" {
		logger.DebugF("No fallback schema for version %s", index.Version)
		return nil
	}

	index.Version = fallback

	return v.Get(index)
}

func (v *Validator) openAPIValidate(dataObj *[]byte, schema *spec.Schema, options *validateOptions) (isValid bool, multiErr error) {
	validator := validate.NewSchemaValidator(schema, nil, "", strfmt.Default)

	var blank map[string]interface{}

	dataBytes := *dataObj

	if options.strictUnmarshal {
		err := yaml.UnmarshalStrict(dataBytes, &blank)
		if err != nil {
			return false, fmt.Errorf("openAPIValidate json unmarshal strict: %v", err)
		}
	} else {
		err := yaml.Unmarshal(dataBytes, &blank)
		if err != nil {
			return false, fmt.Errorf("openAPIValidate json unmarshal: %v", err)
		}
	}

	result := validator.Validate(blank)
	if !result.IsValid() {
		var allErrs *multierror.Error
		allErrs = multierror.Append(allErrs, result.Errors...)

		return false, allErrs.ErrorOrNil()
	}

	for _, extensionsValidator := range v.extensionsValidators {
		if err := extensionsValidator.Validate(dataBytes, *schema); err != nil {
			return false, err
		}
	}

	// Add default values from openAPISpec
	post.ApplyDefaults(result)
	*dataObj, _ = json.Marshal(result.Data())

	return true, nil
}
