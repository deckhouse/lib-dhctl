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
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExtractGroupAndGroupVersion(t *testing.T) {
	type testCase struct {
		name  string
		input SchemaIndex

		expectedGroup        string
		expectedGroupVersion string
	}

	assertGroup := func(t *testing.T, test testCase, group string) {
		require.Equal(t, test.expectedGroup, group, "should group correct")
	}

	assertGroupVersion := func(t *testing.T, test testCase, gv string) {
		require.Equal(t, test.expectedGroupVersion, gv, "should group version correct")
	}

	tests := []testCase{
		{
			name:                 "empty",
			input:                SchemaIndex{},
			expectedGroup:        "",
			expectedGroupVersion: "",
		},

		{
			name: "only group version",
			input: SchemaIndex{
				Version: "v1",
			},
			expectedGroup:        "",
			expectedGroupVersion: "v1",
		},

		{
			name: "group and group version",
			input: SchemaIndex{
				Version: "dhctl.deckhouse.io/v1",
			},
			expectedGroup:        "dhctl.deckhouse.io",
			expectedGroupVersion: "v1",
		},

		{
			name: "invalid version",
			input: SchemaIndex{
				Version: "deckhouse.io/dhctl/v1",
			},
			expectedGroup:        "invalid: deckhouse.io/dhctl/v1",
			expectedGroupVersion: "invalid: deckhouse.io/dhctl/v1",
		},
	}

	t.Run("group and group version", func(t *testing.T) {
		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				group, groupVersion := test.input.GroupAndGroupVersion()
				assertGroup(t, test, group)
				assertGroupVersion(t, test, groupVersion)
			})
		}
	})

	t.Run("group", func(t *testing.T) {
		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				group := test.input.Group()
				assertGroup(t, test, group)
			})
		}
	})

	t.Run("group version", func(t *testing.T) {
		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				gv := test.input.GroupVersion()
				assertGroupVersion(t, test, gv)
			})
		}
	})
}

func TestParseIndex(t *testing.T) {
	const noIndexDoc = `
key: key
value: 1
`

	tests := []struct {
		name   string
		reader io.Reader
		errs   []error
		opts   []ParseIndexOption
	}{
		{
			name:   "invalid read",
			reader: &errorReader{},
			errs:   []error{ErrRead},
		},

		{
			name:   "invalid yaml",
			reader: strings.NewReader("{invalid"),
			errs:   []error{ErrKindInvalidYAML, ErrKindValidationFailed},
		},

		{
			name:   "without index strict",
			reader: strings.NewReader(noIndexDoc),
			errs:   []error{ErrKindValidationFailed},
		},

		{
			name:   "without index no strict",
			reader: strings.NewReader(noIndexDoc),
			errs:   nil,
			opts:   []ParseIndexOption{ParseIndexWithoutCheckValid()},
		},

		{
			name: "happy case",
			reader: strings.NewReader(`
apiVersion: deckhouse.io/v1
kind: TestKind
sshUser: ubuntu
sshPort: 2200
`),
			errs: nil,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			index, err := ParseIndex(test.reader, test.opts...)
			if len(test.errs) == 0 {
				require.NoError(t, err, "should not have an error")
				return
			}

			require.Error(t, err, "should have errors")

			require.Nil(t, index, "should have nil index is invalid")

			for _, expectedErr := range test.errs {
				require.ErrorIs(t, err, expectedErr, "should have errored")
			}
		})
	}
}

type errorReader struct{}

func (e errorReader) Read(p []byte) (n int, err error) {
	return 0, fmt.Errorf("error")
}
