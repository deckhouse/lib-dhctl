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
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExtractErrors(t *testing.T) {
	tests := []struct {
		name           string
		input          error
		expectedErrors []ErrorKind
	}{
		{
			name:           "one error",
			input:          fmt.Errorf("some %w error", ErrSchemaNotFound),
			expectedErrors: []ErrorKind{ErrSchemaNotFound},
		},

		{
			name:           "multiple errors in order",
			input:          fmt.Errorf("some %w error: %w", ErrKindInvalidYAML, ErrKindValidationFailed),
			expectedErrors: []ErrorKind{ErrKindValidationFailed, ErrKindInvalidYAML},
		},

		{
			name:           "no validation error must return ErrUnknown",
			input:          fmt.Errorf("some error"),
			expectedErrors: []ErrorKind{ErrUnknown},
		},
	}

	t.Run("extract all errors", func(t *testing.T) {
		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				errs := ExtractValidationErrors(test.input)
				require.Len(t, errs, len(test.expectedErrors), "unexpected error count")
				require.Equal(t, test.expectedErrors, errs, "should extract all errors")
			})
		}
	})

	t.Run("extract one error", func(t *testing.T) {
		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				err := ExtractValidationError(test.input)
				require.Error(t, err, "should extract validation error")
				require.Equal(t, test.expectedErrors[0], err, "should extract valid error")
			})
		}
	})
}
