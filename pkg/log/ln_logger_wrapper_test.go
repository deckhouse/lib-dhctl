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
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLnLoggerWrapper(t *testing.T) {
	logger := NewInMemoryLoggerWithParent(NewSimpleLogger(LoggerOptions{IsDebug: true}))

	assertAddNewLine := func(t *testing.T, msg string) string {
		matches, err := logger.AllMatches(&Match{
			Prefix: []string{fmt.Sprintf("%s\n", msg)},
		})

		require.NoError(t, err)
		require.Len(t, matches, 1, msg)

		return matches[0]
	}

	wrapper := newFormatWithNewLineLoggerWrapper(logger)

	wrapper.ErrorF("Error")
	assertAddNewLine(t, "Error")

	wrapper.ErrorF("VariablesError %s %v", "msg", true)
	assertAddNewLine(t, "VariablesError msg true")

	wrapper.WarnF("Warn")
	assertAddNewLine(t, "Warn")

	wrapper.WarnF("VariablesWarn %s %v", "msg", true)
	assertAddNewLine(t, "VariablesWarn msg true")

	wrapper.InfoF("Info")
	assertAddNewLine(t, "Info")

	wrapper.InfoF("VariablesInfo %s %v", "msg", errors.New("error"))
	assertAddNewLine(t, "VariablesInfo msg error")

	// cut new line from format
	wrapper.InfoF("Format with new line %d\n", 42)
	match := assertAddNewLine(t, "Format with new line 42")
	require.Equal(t, "Format with new line 42\n", match)

	wrapper.DebugF("VariablesDebug %v %s", 42, "msg")
	assertAddNewLine(t, "VariablesDebug 42 msg")
}
