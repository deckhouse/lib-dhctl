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
	"bytes"
	"fmt"
	"io"
	"regexp"
	"strings"

	"sigs.k8s.io/yaml"
)

const (
	InvalidGroupPrefix = "invalid:"
)

type SchemaIndex struct {
	Kind    string `json:"kind"`
	Version string `json:"apiVersion"`
}

type parseIndexOption struct {
	noCheckIsValid bool
}

type ParseIndexOption func(*parseIndexOption)

func ParseIndexWithoutCheckValid() ParseIndexOption {
	return func(o *parseIndexOption) {
		o.noCheckIsValid = true
	}
}

var parseIndexNoCheckValidOpt = ParseIndexWithoutCheckValid()

// ParseIndex
// parse SchemaIndex from reader
// if reader returns error - wrap reader error with ErrRead
// also function validate is SchemaIndex is valid. Is invalid returns ErrKindValidationFailed
// with pretty error with input doc in error
// if content was not unmarshal wrap unmarshal error with ErrKindValidationFailed ErrKindInvalidYAML
func ParseIndex(reader io.Reader, opts ...ParseIndexOption) (*SchemaIndex, error) {
	options := &parseIndexOption{}
	for _, o := range opts {
		o(options)
	}

	content, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrRead, err)
	}

	// we cannot use yaml.UnmarshalStrict here
	// because strict unmarshal also verify that another keys not present
	if err := contentHasMultipleSchemaKeys(content); err != nil {
		return nil, err
	}

	index := SchemaIndex{}

	err = yaml.Unmarshal(content, &index)
	if err != nil {
		return nil, fmt.Errorf("%w %w: schema index unmarshal failed: %w", ErrKindValidationFailed, ErrKindInvalidYAML, err)
	}

	if !options.noCheckIsValid && !index.IsValid() {
		return nil, index.invalidIndexErr(content)
	}

	return &index, nil
}

func (i *SchemaIndex) IsValid() bool {
	return i.Kind != "" && i.Version != ""
}

func (i *SchemaIndex) String() string {
	return fmt.Sprintf("%s, %s", i.Kind, i.Version)
}

// Group
// returns group (deckhouse.io if passed like deckhouse.io/v1)
// if Version is invalid (for example deckhouse.io/dhctl/v1)
// returns invalid: deckhouse.io/dhctl/v1 string
// check is invalid as strings.HasPrefix(s, InvalidGroupPrefix)
// if Version does not contain group (if Version is v1 for example)
// returns empty string
func (i *SchemaIndex) Group() string {
	g, _ := i.GroupAndGroupVersion()
	return g
}

// GroupVersion
// returns group version (v1 if passed like deckhouse.io/v1)
// if Version is invalid (for example deckhouse.io/dhctl/v1)
// returns invalid: deckhouse.io/dhctl/v1 string
// check is invalid as strings.HasPrefix(s, InvalidGroupPrefix)
func (i *SchemaIndex) GroupVersion() string {
	_, gv := i.GroupAndGroupVersion()
	return gv
}

// GroupAndGroupVersion
// returns group (like deckhouse.io) as first value
// and group version (like v1) as second value
// if Version is invalid (for example deckhouse.io/dhctl/v1)
// returns invalid: deckhouse.io/dhctl/v1 string as all arguments
// check is invalid as strings.HasPrefix(s, InvalidGroupPrefix)
// if Version contains only group version (if Version is v1 for example)
// returns empty string as first value and version as second
func (i *SchemaIndex) GroupAndGroupVersion() (string, string) {
	v := i.Version
	if v == "" {
		return "", ""
	}

	switch strings.Count(i.Version, "/") {
	case 0:
		return "", v
	case 1:
		i := strings.Index(v, "/")
		return v[:i], v[i+1:]
	default:
		invalid := fmt.Sprintf("%s %s", InvalidGroupPrefix, i.Version)
		return invalid, invalid
	}
}

func (i *SchemaIndex) invalidIndexErr(doc []byte) error {
	return fmt.Errorf(
		"%w: document must contain \"kind\" and \"apiVersion\" fields:\n\tapiVersion: %s\n\tkind: %s\n\n%s",
		ErrKindValidationFailed, i.Version, i.Kind, string(doc),
	)
}

var (
	apiVersionRegex = regexp.MustCompile(`(?m)^apiVersion:.*$`)
	kindRegex       = regexp.MustCompile(`(?m)^kind:.*$`)
	errSeparator    = []byte(" ")
)

func multipleKeysErr(keyName string, keys [][]byte) error {
	joinedKeys := bytes.Join(keys, errSeparator)
	return fmt.Errorf("%w: multiple %s keys found: %s", ErrKindValidationFailed, keyName, string(joinedKeys))
}

func contentHasMultipleSchemaKeys(content []byte) error {
	if res := apiVersionRegex.FindAll(content, 2); len(res) > 1 {
		return multipleKeysErr("apiVersion", res)
	}

	if res := kindRegex.FindAll(content, 2); len(res) > 1 {
		return multipleKeysErr("kind", res)
	}

	return nil
}
