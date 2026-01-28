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
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLnLoggerWrapper(t *testing.T) {
	emptyStringTests := []struct {
		name string
		do   func(w formatWithNewLineLogger)
	}{
		{
			name: "ErrorF",
			do: func(w formatWithNewLineLogger) {
				w.ErrorF("")
			},
		},

		{
			name: "WarnF",
			do: func(w formatWithNewLineLogger) {
				w.WarnF("")
			},
		},

		{
			name: "InfoF",
			do: func(w formatWithNewLineLogger) {
				w.InfoF("")
			},
		},

		{
			name: "DebugF",
			do: func(w formatWithNewLineLogger) {
				w.DebugF("")
			},
		},
	}

	for _, test := range emptyStringTests {
		t.Run(fmt.Sprintf("Log empty line for %s", test.name), func(t *testing.T) {
			logger := NewInMemoryLoggerWithParent(NewPrettyLogger(LoggerOptions{IsDebug: true}))
			wrapper := newFormatWithNewLineLoggerWrapper(logger)

			test.do(wrapper)

			matches, err := logger.AllMatches(&Match{
				Prefix: []string{"\n"},
			})

			require.NoError(t, err)
			require.Len(t, matches, 1, "should one match")
			require.Equal(t, "\n", matches[0], "should produce new line")
		})
	}

	logger := NewInMemoryLoggerWithParent(NewPrettyLogger(LoggerOptions{IsDebug: true}))

	assertAddNewLine := func(t *testing.T, msg string) string {
		matches, err := logger.AllMatches(&Match{
			Prefix: []string{fmt.Sprintf("%s\n", msg)},
		})

		require.NoError(t, err)
		require.Len(t, matches, 1, msg)

		return matches[0]
	}

	assertCountNewLines := func(t *testing.T, msg string, expected int) {
		match := assertAddNewLine(t, msg)
		count := strings.Count(match, "\n")
		require.Equal(t, expected, count, "should contain %d trailing new lines", expected)
	}

	wrapper := newFormatWithNewLineLoggerWrapper(logger)

	wrapper.ErrorF("Error")
	assertAddNewLine(t, "Error")

	wrapper.ErrorF("VariablesError %s %v", "msg", true)
	assertAddNewLine(t, "VariablesError msg true")

	// trim one new line
	wrapper.ErrorF("ErrorOneLn\n")
	assertCountNewLines(t, "ErrorOneLn", 1)
	// save multiple new lines expected one
	wrapper.ErrorF("ErrorMultiLn\n\n\n")
	assertCountNewLines(t, "ErrorMultiLn\n\n", 3)

	wrapper.WarnF("Warn")
	assertAddNewLine(t, "Warn")

	wrapper.WarnF("VariablesWarn %s %v", "msg", true)
	assertAddNewLine(t, "VariablesWarn msg true")

	// trim one new line
	wrapper.WarnF("WarnOneLn\n")
	assertCountNewLines(t, "WarnOneLn", 1)
	// save multiple new lines expected one
	wrapper.WarnF("WarnMultiLn\n\n")
	assertCountNewLines(t, "WarnMultiLn\n", 2)

	wrapper.InfoF("Info")
	assertAddNewLine(t, "Info")

	wrapper.InfoF("VariablesInfo %s %v", "msg", errors.New("error"))
	assertAddNewLine(t, "VariablesInfo msg error")

	// trim one new line
	wrapper.InfoF("InfoOneLn\n")
	assertCountNewLines(t, "InfoOneLn", 1)
	// save multiple new lines expected one
	wrapper.InfoF("InfoMultiLn\n\n\n\n")
	assertCountNewLines(t, "InfoMultiLn\n\n\n", 4)

	wrapper.DebugF("Debug")
	assertAddNewLine(t, "Debug")

	wrapper.DebugF("VariablesDebug %v %s", 42, "msg")
	assertAddNewLine(t, "VariablesDebug 42 msg")

	// trim one new line
	wrapper.DebugF("DebugOneLn\n")
	assertCountNewLines(t, "DebugOneLn", 1)
	// save multiple new lines expected one
	wrapper.DebugF("DebugMultiLn\n\n\n\n\n")
	assertCountNewLines(t, "DebugMultiLn\n\n\n\n", 5)
}
