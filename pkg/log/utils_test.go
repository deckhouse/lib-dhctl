// Copyright 2025 Flant JSC
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

package log

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPrepareList(t *testing.T) {
	tests := []struct {
		name  string
		input []any
		out   string
	}{
		{
			name:  "no values",
			input: nil,
			out:   "",
		},

		{
			name:  "one value string",
			input: []any{"string"},
			out:   "string",
		},

		{
			name:  "one value not string",
			input: []any{42},
			out:   "42",
		},

		{
			name:  "multiple strings",
			input: []any{"a", "b", "c"},
			out:   "[a b c]",
		},

		{
			name:  "multiple different values",
			input: []any{"a", 42, "c"},
			out:   "[a 42 c]",
		},
	}

	for _, tst := range tests {
		t.Run(tst.name, func(t *testing.T) {
			out := listToString(tst.input...)
			require.Equal(t, tst.out, out)
		})
	}
}
