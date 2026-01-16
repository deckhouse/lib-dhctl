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
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadSchemas(t *testing.T) {
	t.Run("return ErrRead if reader returns error", func(t *testing.T) {
		_, err := LoadSchemas(errorReader{})
		require.Error(t, err, "reader returns error")
		require.ErrorIs(t, err, ErrRead, "reader should returns ErrRead error")
	})
}
