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

package log

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLnLoggerWrapper(t *testing.T) {
	logger := NewInMemoryLoggerWithParent(NewSimpleLogger(LoggerOptions{IsDebug: true}))

	assertAddNewLine := func(t *testing.T, msg string) {
		matches, err := logger.AllMatches(&Match{
			Prefix: []string{fmt.Sprintf("%s\n", msg)},
		})

		require.NoError(t, err)
		require.Len(t, matches, 1, msg)
	}

	wrapper := newFormatWithNewLineLoggerWrapper(logger)

	wrapper.ErrorF("Error")
	assertAddNewLine(t, "Error")

	wrapper.WarnF("Warn")
	assertAddNewLine(t, "Warn")

	wrapper.InfoF("Info")
	assertAddNewLine(t, "Info")

	wrapper.DebugF("Debug")
	assertAddNewLine(t, "Debug")
}
