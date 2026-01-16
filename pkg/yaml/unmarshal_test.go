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

package yaml

import (
	"testing"

	"github.com/stretchr/testify/require"
)

type testType struct {
	StringValue string      `yaml:"string"`
	IntValue    int         `yaml:"int"`
	Sub         testSubType `yaml:"sub"`
}

type testSubType struct {
	SliceValue []string `yaml:"slice"`
}

func TestUnmarshal(t *testing.T) {
	doc := `
string: str
int: 42
sub:
  slice: 
  - "first" 
  - "second"
  - "third"
`

	assertResult := func(t *testing.T, result testType) {
		require.Equal(t, "str", result.StringValue)
		require.Equal(t, 42, result.IntValue)
		require.Len(t, result.Sub.SliceValue, 3)
		require.Equal(t, "first", result.Sub.SliceValue[0])
		require.Equal(t, "second", result.Sub.SliceValue[1])
		require.Equal(t, "third", result.Sub.SliceValue[2])
	}

	t.Run("val", func(t *testing.T) {
		res, err := UnmarshalString[testType](doc)
		require.NoError(t, err)
		assertResult(t, res)
	})

	t.Run("pointer", func(t *testing.T) {
		res, err := UnmarshalString[*testType](doc)
		require.NoError(t, err)
		require.NotNil(t, res)
		assertResult(t, *res)
	})
}
