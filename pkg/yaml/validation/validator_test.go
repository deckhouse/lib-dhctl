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
	"strings"
	"testing"

	"github.com/deckhouse/lib-dhctl/pkg/log"
	"github.com/deckhouse/lib-dhctl/pkg/yaml/validation/transformer"
	"github.com/go-openapi/spec"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

const (
	testSchemaTestKind = `
kind: TestKind
apiVersions:
- apiVersion: deckhouse.io/v1
  openAPISpec:
    type: object
    additionalProperties: false
    anyOf:
      - required: [apiVersion, kind, sshUser, sshAgentPrivateKeys]
      - required: [apiVersion, kind, sshUser, sudoPassword]
    properties:
      kind:
        type: string
      apiVersion:
        type: string
      sshUser:
        type: string
        description: SSH username.
      sudoPassword:
        description: |
          A sudo password for the user.
        type: string
      sshPort:
        default: 22
        type: integer
        description: SSH port.
      sshAgentPrivateKeys:
        type: array
        minItems: 1
        items:
          type: object
          additionalProperties: false
          required: [key]
          x-rules: [passphrase]
          properties:
            key:
              type: string
              description: Private SSH key.
            passphrase:
              type: string
              description: Password for SSH key.
`
	testSchemaAnotherTestKind = `
kind: AnotherTestKind
apiVersions:
- apiVersion: test
  openAPISpec:
    type: object
    additionalProperties: false
    properties:
      kind:
        type: string
      apiVersion:
        type: string
      key:
        type: string
      value:
        type: object
        additionalProperties: true
        default: {"valueEnum": "AWS", "valueBool": true}
        properties:
          valueEnum:
            type: string
            enum:
            - "OpenStack"
            - "AWS"
          valueBool:
            type: boolean
`
)

var (
	indexTestKind = SchemaIndex{
		Kind:    "TestKind",
		Version: "deckhouse.io/v1",
	}

	indexAnotherTestKind = SchemaIndex{
		Kind:    "AnotherTestKind",
		Version: "test",
	}
)

func TestValidator(t *testing.T) {
	t.Run("One schema", func(t *testing.T) {
		logger := testGetLogger()

		getValidatorTestKind := func(t *testing.T) *Validator {
			validator := NewValidator(nil).SetLogger(logger)
			err := validator.LoadSchemas(strings.NewReader(testSchemaTestKind))
			require.NoError(t, err, "failed to load schema")
			return validator
		}

		getValidatorAnotherTestKind := func(t *testing.T) *Validator {
			validator := NewValidator(nil).SetLogger(logger)
			err := validator.LoadSchemas(strings.NewReader(testSchemaAnotherTestKind))
			require.NoError(t, err, "failed to load schema")
			return validator
		}

		t.Run("happy case", func(t *testing.T) {
			validatorTestKind := getValidatorTestKind(t)

			docPassword := `
apiVersion: deckhouse.io/v1
kind: TestKind
sshUser: ubuntu
sudoPassword: "no secret"
sshPort: 2200
`
			bytesValidate := []byte(docPassword)
			index, err := validatorTestKind.Validate(&bytesValidate, ValidateWithNoPrettyError(true))
			require.NoError(t, err)
			asserTestKindIndex(t, *index)

			expectedTestKind := &testKind{
				SchemaIndex:  indexTestKind,
				SSHUser:      "ubuntu",
				SudoPassword: "no secret",
				SSHPort:      2200,
			}

			asserTestKind(t, bytesValidate, expectedTestKind)

			asserValidateTestKind(t, validatorTestKind, docPassword, false, expectedTestKind)

			docKey := `
apiVersion: deckhouse.io/v1
kind: TestKind
sshUser: ubuntu
sshPort: 2200
sshAgentPrivateKeys:
- key: "mykey"
`
			asserValidateTestKind(t, validatorTestKind, docKey, false, &testKind{
				SSHUser: "ubuntu",
				SSHPort: 2200,
				SSHAgentPrivateKeys: []testPrivateKey{
					{Key: "mykey"},
				},
			})
		})

		t.Run("set defaults", func(t *testing.T) {
			doc := `
apiVersion: deckhouse.io/v1
kind: TestKind
sshUser: ubuntu
sudoPassword: "no secret"
`

			asserValidateTestKind(t, getValidatorTestKind(t), doc, false, &testKind{
				SSHUser:      "ubuntu",
				SudoPassword: "no secret",
				SSHPort:      22,
			})

			anotherTestKindDoc := `
apiVersion: test
kind: AnotherTestKind
key: "mykey"
`
			asserValidateAnotherTestKind(
				t,
				getValidatorAnotherTestKind(t),
				anotherTestKindDoc,
				false,
				&testAnotherKind{
					Key: "mykey",
					Value: testAnotherKindValue{
						ValueEnum: "AWS",
						ValueBool: true,
					},
				})
		})

		t.Run("version fallback", func(t *testing.T) {
			validatorTestKind := getValidatorTestKind(t).
				AddVersionFallback("test", indexTestKind.Version)
			// copy
			index := indexTestKind
			index.Version = "test"

			doc := `
apiVersion: deckhouse.io/v1
kind: TestKind
sshUser: ubuntu
sudoPassword: "no secret"
`

			bytesValidateWithIndex := []byte(doc)
			err := validatorTestKind.ValidateWithIndex(&index, &bytesValidateWithIndex)
			require.NoError(t, err)
			asserTestKindIndex(t, index)
		})

		t.Run("prevalidator", func(t *testing.T) {
			t.Run("doc valid", func(t *testing.T) {
				doc := `
apiVersion: deckhouse.io/v1
kind: TestKind
sshUser: ubuntu
sudoPassword: "no secret"
sshPort: 22456
`
				validator := getValidatorTestKind(t)
				validator.AddPreValidator(indexTestKind, newTestKindPreValidator(t, ""))

				asserValidateTestKind(t, validator, doc, false, &testKind{
					SSHUser:      "ubuntu",
					SudoPassword: "no secret",
					SSHPort:      22456,
				})
			})

			t.Run("doc invalid", func(t *testing.T) {
				doc := `
apiVersion: deckhouse.io/v1
kind: TestKind
sshUser: ubuntu
sudoPassword: "no secret"
sshPort: 23
`
				validator := getValidatorTestKind(t)
				validator.AddPreValidator(indexTestKind, newTestKindPreValidator(t, ""))

				asserValidateTestKind(t, validator, doc, true, nil)
			})

			t.Run("our schema valid", func(t *testing.T) {
				doc := `
apiVersion: deckhouse.io/v1
kind: TestKind
sshUser: ubuntu
sudoPassword: "no secret"
sshPort: 22456
`
				validator := NewValidator(nil)
				validator.SetLogger(logger)

				validator.AddPreValidator(indexTestKind, newTestKindPreValidator(t, testSchemaTestKind))

				asserValidateTestKind(t, validator, doc, false, &testKind{
					SSHUser:      "ubuntu",
					SudoPassword: "no secret",
					SSHPort:      22456,
				})
			})

			t.Run("our schema invalid", func(t *testing.T) {
				doc := `
apiVersion: deckhouse.io/v1
kind: TestKind
sshUser: ubuntu
sudoPassword: "no secret"
sshPort: 22456
`
				validator := NewValidator(nil)
				validator.SetLogger(logger)

				validator.AddPreValidator(indexTestKind, newTestKindPreValidator(t, testSchemaAnotherTestKind))

				asserValidateTestKind(t, validator, doc, true, nil)
			})
		})

		t.Run("add transformers", func(t *testing.T) {
			t.Run("for index", func(t *testing.T) {
				validator := getValidatorAnotherTestKind(t)
				validator.AddTransformers(
					indexAnotherTestKind,
					transformer.NewAdditionalPropertiesTransformerDisallowFull(),
				)

				assertValidationWithTransformers(t, validator, true)
			})

			t.Run("default", func(t *testing.T) {
				validator := getValidatorAnotherTestKind(t)
				validator.SetDefaultTransformers(
					transformer.NewAdditionalPropertiesTransformerDisallowFull(),
				)

				assertValidationWithTransformers(t, validator, true)
			})

			t.Run("additional properties enabled is set in schema", func(t *testing.T) {
				validator := getValidatorAnotherTestKind(t)
				validator.SetDefaultTransformers(
					transformer.NewAdditionalPropertiesTransformer(),
				)

				assertValidationWithTransformers(t, validator, false)
			})
		})

		t.Run("with extensions", func(t *testing.T) {
			tests := []struct {
				name        string
				keyPassword string
				shouldError bool
			}{
				{
					name:        "invalid value",
					keyPassword: `["a", "b"]`,
					shouldError: true,
				},

				{
					name:        "value does not valid password string",
					keyPassword: `"not secret"`,
					shouldError: true,
				},

				{
					name:        "valid password string",
					keyPassword: `"!not@secret."`,
					shouldError: false,
				},
			}

			for _, test := range tests {
				t.Run(test.name, func(t *testing.T) {
					assertPassphraseExtensions(
						t,
						getValidatorTestKind(t),
						test.keyPassword,
						test.shouldError,
					)
				})
			}
		})

		t.Run("error case", func(t *testing.T) {
			tests := []struct {
				name         string
				doc          string
				errSubstring string
				opts         []ValidateOption
			}{
				{
					name:         "invalid yaml",
					doc:          `{invalid`,
					errSubstring: "error converting YAML to JSON: yaml: line 1",
				},
				{
					name: "no schema fields",
					doc: `
sshUser: ubuntu
sudoPassword: "no secret"
sshPort: 22456
`,
					errSubstring: `document must contain "kind" and "apiVersion"`,
				},
				{
					name: "no schema found",
					doc: `
apiVersion: some
kind: MyKind
sshUser: ubuntu
sudoPassword: "no secret"
sshPort: 22456
`,
					errSubstring: ErrSchemaNotFound.Error(),
				},
				{
					name: "not valid by schema",
					doc: `
apiVersion: deckhouse.io/v1
kind: TestKind
sshUser: ubuntu
sshPort: "port"
sshAgentPrivateKeys: {"a": "b"}
`,
					errSubstring: "Document validation failed:\n---",
				},
				{
					name: "not valid by schema no pretty error",
					doc: `
apiVersion: deckhouse.io/v1
kind: TestKind
sshUser: ubuntu
sshPort: "port"
sshAgentPrivateKeys: {"a": "b"}
`,
					errSubstring: `"TestKind, deckhouse.io/v1" document validation failed:`,
					opts:         []ValidateOption{ValidateWithNoPrettyError(true)},
				},
			}

			for _, test := range tests {
				t.Run(test.name, func(t *testing.T) {
					asserNoValidateTestKind(t, getValidatorTestKind(t), test.doc, test.errSubstring, test.opts...)
				})
			}
		})
	})
}

func testGetLogger() log.LoggerProvider {
	return log.SimpleLoggerProvider(
		log.NewInMemoryLoggerWithParent(
			log.NewPrettyLogger(log.LoggerOptions{IsDebug: true}),
		),
	)
}

type testPrivateKey struct {
	Key        string `json:"key"`
	Passphrase string `json:"passphrase"`
}

type testKind struct {
	SchemaIndex
	SSHUser             string           `yaml:"sshUser"`
	SudoPassword        string           `yaml:"sudoPassword"`
	SSHPort             int              `yaml:"sshPort"`
	SSHAgentPrivateKeys []testPrivateKey `yaml:"sshAgentPrivateKeys"`
}

type testAnotherKindValue struct {
	ValueEnum string `yaml:"valueEnum"`
	ValueBool bool   `yaml:"valueBool"`
}

type testAnotherKind struct {
	SchemaIndex

	Key   string               `json:"key"`
	Value testAnotherKindValue `json:"value"`
}

type testKindPreValidator struct {
	mySchema *spec.Schema
}

func newTestKindPreValidator(t *testing.T, schema string) *testKindPreValidator {
	r := &testKindPreValidator{}
	if schema != "" {
		ss, err := LoadSchemas(strings.NewReader(schema))
		require.NoError(t, err)
		require.Len(t, ss, 1)
		r.mySchema = ss[0].Schema
	}

	return r
}

func (p *testKindPreValidator) Validate(doc []byte, currentSchema *spec.Schema, _ log.Logger) (*spec.Schema, error) {
	v := testKind{}
	err := yaml.Unmarshal(doc, &v)
	if err != nil {
		return nil, err
	}

	if v.SSHPort >= 22000 && v.SSHPort < 30000 {
		if currentSchema == nil {
			return p.mySchema, nil
		}
		return currentSchema, nil
	}

	return nil, fmt.Errorf("invalid SSH port: %d", v.SSHPort)
}

func asserTestKind(t *testing.T, data []byte, expected *testKind) {
	result := testKind{}

	expected.SchemaIndex = indexTestKind

	err := yaml.Unmarshal(data, &result)
	require.NoError(t, err, "failed to unmarshal test kind")

	require.Equal(t, expected.SchemaIndex, result.SchemaIndex, "invalid schema index")
	require.Equal(t, expected.SSHUser, result.SSHUser, "invalid SSH user")
	require.Equal(t, expected.SSHPort, result.SSHPort, "invalid SSH port")
	require.Equal(t, expected.SSHAgentPrivateKeys, result.SSHAgentPrivateKeys, "invalid SSH agent keys")
	require.Equal(t, expected.SudoPassword, result.SudoPassword, "invalid sudo password")
}

func doValidate(t *testing.T, validator *Validator, expectedIndex SchemaIndex, doc string, shouldError bool, opts ...ValidateOption) []byte {
	index := expectedIndex

	if len(opts) == 0 {
		opts = []ValidateOption{ValidateWithNoPrettyError(true)}
	}

	bytesForError := []byte(doc)
	bytesValidateWithIndex := []byte(doc)
	err := validator.ValidateWithIndex(&index, &bytesValidateWithIndex, opts...)
	if shouldError {
		require.Error(t, err, "should validation error for test another kind")
		require.Equal(t, bytesForError, bytesValidateWithIndex, "should not change input")
		return nil
	}
	require.NoError(t, err, "should not validation error for test another kind")

	require.True(t, index.IsValid(), "invalid index")
	require.Equal(t, expectedIndex, index, "invalid index value")

	return bytesValidateWithIndex
}

func asserValidateAnotherTestKind(t *testing.T, validator *Validator, doc string, shouldError bool, expected *testAnotherKind, opts ...ValidateOption) []byte {
	bytes := doValidate(t, validator, indexAnotherTestKind, doc, shouldError, opts...)
	if shouldError {
		return nil
	}

	result := testAnotherKind{}

	expected.SchemaIndex = indexAnotherTestKind

	err := yaml.Unmarshal(bytes, &result)
	require.NoError(t, err, "failed to unmarshal test another kind")

	require.Equal(t, expected.SchemaIndex, result.SchemaIndex, "invalid schema index")
	require.Equal(t, expected.Key, result.Key, "invalid key value")
	require.Equal(t, expected.Value, result.Value, "invalid value value")

	return bytes
}

func asserTestKindIndex(t *testing.T, index SchemaIndex) {
	require.True(t, index.IsValid(), "invalid index")
	require.Equal(t, indexTestKind, index, "invalid index value")
}

func asserValidateTestKind(t *testing.T, validator *Validator, doc string, shouldError bool, expected *testKind, opts ...ValidateOption) {
	bytes := doValidate(t, validator, indexTestKind, doc, shouldError, opts...)
	if shouldError {
		return
	}
	asserTestKind(t, bytes, expected)
}

func asserNoValidateTestKind(t *testing.T, validator *Validator, doc string, errorSubstring string, opts ...ValidateOption) {
	bytesForError := []byte(doc)
	bytesValidateWithIndex := []byte(doc)

	_, err := validator.Validate(&bytesValidateWithIndex, opts...)

	require.Error(t, err, "should not validate")
	require.Equal(t, bytesForError, bytesValidateWithIndex, "should not change input")
	require.Contains(t, err.Error(), errorSubstring)
}

func assertValidationWithTransformers(t *testing.T, validator *Validator, shouldError bool) {
	doc := `
apiVersion: test
kind: AnotherTestKind
key: "mykey"
value:
  additionalPropertyInvalidWithTransformer: "invalid"
  valueEnum: "OpenStack"
  valueBool: true
`
	bytes := asserValidateAnotherTestKind(t, validator, doc, shouldError, &testAnotherKind{
		Key: "mykey",
		Value: testAnotherKindValue{
			ValueBool: true,
			ValueEnum: "OpenStack",
		},
	})

	if shouldError {
		return
	}

	obj := map[string]any{}
	err := yaml.Unmarshal(bytes, &obj)
	require.NoError(t, err, "failed to unmarshal test another kind to map")
	require.Contains(t, obj, "value")
	value := obj["value"].(map[string]any)
	require.Equal(t, value["additionalPropertyInvalidWithTransformer"], "invalid", "additional field should present")
}

func assertPassphraseExtensions(t *testing.T, validator *Validator, keyPassword string, shouldError bool) {
	validators := map[string]ExtensionsValidatorHandler{
		"passphrase": func(oldValue json.RawMessage) error {
			key := testPrivateKey{}
			err := json.Unmarshal(oldValue, &key)
			if err != nil {
				return err
			}

			shouldPresent := ".!@"

			if !strings.ContainsAny(key.Passphrase, shouldPresent) {
				return fmt.Errorf("invalid passphrase: should contain %s", shouldPresent)
			}

			return nil
		},
	}

	validator.AddExtensionsValidators(NewXRulesExtensionsValidator(validators))

	doc := fmt.Sprintf(`
apiVersion: deckhouse.io/v1
kind: TestKind
sshUser: ubuntu
sshPort: 2200
sshAgentPrivateKeys:
- key: "mykey"
  passphrase: %s
`, keyPassword)

	asserValidateTestKind(t, validator, doc, shouldError, &testKind{
		SSHUser: "ubuntu",
		SSHPort: 2200,
		SSHAgentPrivateKeys: []testPrivateKey{
			{
				Key:        "mykey",
				Passphrase: strings.Trim(keyPassword, `"`),
			},
		},
	})
}
