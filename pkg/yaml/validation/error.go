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
	"fmt"
	"strings"
)

type ErrorKind int

const (
	ErrKindChangesValidationFailed ErrorKind = iota + 1
	ErrKindValidationFailed
	ErrKindInvalidYAML
)

func (k ErrorKind) String() string {
	switch k {
	case ErrKindChangesValidationFailed:
		return "ChangesValidationFailed"
	case ErrKindValidationFailed:
		return "ValidationFailed"
	case ErrKindInvalidYAML:
		return "InvalidYAML"
	default:
		return "unknown"
	}
}

type ValidationError struct {
	Kind   ErrorKind
	Errors []Error
}

func (v *ValidationError) Append(kind ErrorKind, e Error) {
	if v.Kind < kind {
		v.Kind = kind
	}
	v.Errors = append(v.Errors, e)
}

func (v *ValidationError) Error() string {
	if v == nil {
		return ""
	}
	errs := make([]string, 0, len(v.Errors))
	for _, e := range v.Errors {
		b := strings.Builder{}
		if e.Index != nil {
			b.WriteString(fmt.Sprintf("[%d]", *e.Index))
		}

		if e.Group != "" {
			b.WriteString(fmt.Sprintf("%s/%s, Kind=%s", e.Group, e.Version, e.Kind))
		}
		if e.Name != "" {
			b.WriteString(fmt.Sprintf(" %q", e.Name))
		}
		if b.Len() != 0 {
			b.WriteString(": ")
		}
		b.WriteString(strings.Join(e.Messages, "; "))

		errs = append(errs, b.String())
	}

	return fmt.Sprintf("%s: %s", v.Kind, strings.Join(errs, "\n"))
}

func (v *ValidationError) ErrorOrNil() error {
	if v == nil {
		return nil
	}
	if len(v.Errors) == 0 {
		return nil
	}

	return v
}

type Error struct {
	Index    *int
	Group    string
	Version  string
	Kind     string
	Name     string
	Messages []string
}

type namedIndex struct {
	Kind     string `json:"kind"`
	Version  string `json:"apiVersion"`
	Metadata struct {
		Name string `json:"name"`
	} `json:"metadata"`
}

func (i *namedIndex) IsValid() bool {
	return i.Kind != "" && i.Version != ""
}

func (i *namedIndex) String() string {
	if i.Metadata.Name != "" {
		return fmt.Sprintf("%s, %s", i.Kind, i.Version)
	}
	return fmt.Sprintf("%s, %s, metadata.name: %q", i.Kind, i.Version, i.Metadata.Name)
}
