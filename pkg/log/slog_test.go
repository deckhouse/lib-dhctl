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
	"context"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDebugLogger(t *testing.T) {
	assertSimpleMessage := func(t *testing.T, logger *InMemoryLogger, msg string, hasInLog bool) {
		matches, err := logger.AllMatches(&Match{
			Prefix: []string{msg},
		})

		expectedLen := 0
		if hasInLog {
			expectedLen = 1
		}

		require.NoError(t, err)
		require.Len(t, matches, expectedLen, "message: '%s'", msg)
	}

	type test struct {
		name     string
		msg      string
		writeLog func(logger *slog.Logger, msg string)
	}

	t.Run("with debug", func(t *testing.T) {
		testsOneShotLogs := []test{
			{
				name: "debug",
				msg:  "debug message",
				writeLog: func(logger *slog.Logger, msg string) {
					logger.Debug(msg)
				},
			},

			{
				name: "info",
				msg:  "info message",
				writeLog: func(logger *slog.Logger, msg string) {
					logger.Info(msg)
				},
			},
			{
				name: "warning",
				msg:  "warn message",
				writeLog: func(logger *slog.Logger, msg string) {
					logger.Warn(msg)
				},
			},
			{
				name: "error",
				msg:  "error message",
				writeLog: func(logger *slog.Logger, msg string) {
					logger.Error(msg)
				},
			},
		}

		for _, tst := range testsOneShotLogs {
			t.Run(tst.name, func(t *testing.T) {
				logger, targetLogger := testCreateSLogLogger("", true)

				tst.writeLog(logger, tst.msg)

				assertSimpleMessage(t, targetLogger, tst.msg, true)
			})
		}
	})

	t.Run("with debug and prefix", func(t *testing.T) {
		testsOneShotLogs := []test{
			{
				name: "debug",
				msg:  "debug message",
				writeLog: func(logger *slog.Logger, msg string) {
					logger.Debug(msg)
				},
			},

			{
				name: "info",
				msg:  "info message",
				writeLog: func(logger *slog.Logger, msg string) {
					logger.Info(msg)
				},
			},
			{
				name: "warning",
				msg:  "warn message",
				writeLog: func(logger *slog.Logger, msg string) {
					logger.Warn(msg)
				},
			},
			{
				name: "error",
				msg:  "error message",
				writeLog: func(logger *slog.Logger, msg string) {
					logger.Error(msg)
				},
			},
		}

		for _, tst := range testsOneShotLogs {
			t.Run(tst.name, func(t *testing.T) {
				const prefix = "ssh"

				logger, targetLogger := testCreateSLogLogger(prefix, true)

				tst.writeLog(logger, tst.msg)

				assertSimpleMessage(t, targetLogger, fmt.Sprintf(`%s: %s`, prefix, tst.msg), true)
			})
		}
	})

	t.Run("without debug but with prefix", func(t *testing.T) {
		testsInLog := []test{
			{
				name: "info",
				msg:  "info message",
				writeLog: func(logger *slog.Logger, msg string) {
					logger.Info(msg)
				},
			},
			{
				name: "warning",
				msg:  "warn message",
				writeLog: func(logger *slog.Logger, msg string) {
					logger.Warn(msg)
				},
			},
			{
				name: "error",
				msg:  "error message",
				writeLog: func(logger *slog.Logger, msg string) {
					logger.Error(msg)
				},
			},
		}

		logger, targetLogger := testCreateSLogLogger("ssh", false)

		for _, tst := range testsInLog {
			t.Run(tst.name, func(t *testing.T) {
				tst.writeLog(logger, tst.msg)

				assertSimpleMessage(t, targetLogger, fmt.Sprintf("ssh: %s", tst.msg), true)
			})
		}

		t.Run("debug should not present", func(t *testing.T) {
			const debugMsg = "debug message"

			logger.Debug(debugMsg)
			assertSimpleMessage(t, targetLogger, debugMsg, false)
		})
	})

	t.Run("with groups", func(t *testing.T) {
		const (
			firstGroup  = "first-group"
			secondGroup = "second-group"
			thirdGroup  = "second-group"
		)

		tests := []struct {
			groups []string
		}{
			{groups: []string{firstGroup}},
			{groups: []string{firstGroup, secondGroup}},
			{groups: []string{firstGroup, secondGroup, thirdGroup}},
		}

		for _, tst := range tests {
			t.Run(strings.Join(tst.groups, "_"), func(t *testing.T) {
				logger, targetLogger := testCreateSLogLogger("", false)
				for _, group := range tst.groups {
					logger = logger.WithGroup(group)
				}

				const msg = "some message"
				logger.Info(msg)

				expectedMsg := fmt.Sprintf(`%s | groups: '%s'`, msg, strings.Join(tst.groups, "/"))

				assertSimpleMessage(t, targetLogger, expectedMsg, true)
			})
		}
	})

	t.Run("with attributes", func(t *testing.T) {
		tests := []struct {
			name        string
			attrs       []any
			attrsSuffix string
		}{
			{
				name:        "with one attribute",
				attrs:       []any{"key", "value"},
				attrsSuffix: "[key='value']",
			},

			{
				name:        "with multiple attributes with different kinds",
				attrs:       []any{"key", "value with space", "err", fmt.Errorf("error"), "int", 42},
				attrsSuffix: "[key='value with space' err='error' int='42']",
			},
		}

		for _, tst := range tests {
			t.Run(tst.name, func(t *testing.T) {
				logger, targetLogger := testCreateSLogLogger("", false)
				logger = logger.With(tst.attrs...)

				const msg = "some message"
				logger.Info(msg)

				expectedMsg := fmt.Sprintf(`%s | attributes: %s`, msg, tst.attrsSuffix)

				assertSimpleMessage(t, targetLogger, expectedMsg, true)
			})
		}
	})

	t.Run("all in", func(t *testing.T) {
		logger, targetLogger := testCreateSLogLogger("ssh", true)

		logger = logger.With("key", "value with spaces").WithGroup("my-group")

		logger.Debug("my message")

		expectedMsg := `ssh: my message | groups: 'my-group' | attributes: [key='value with spaces']`
		assertSimpleMessage(t, targetLogger, expectedMsg, true)
	})
}

func testCreateSLogLogger(prefix string, isDebug bool) (*slog.Logger, *InMemoryLogger) {
	parentLogger := NewInMemoryLoggerWithParent(NewSimpleLogger(LoggerOptions{IsDebug: isDebug}))
	provider := SimpleLoggerProvider(parentLogger)
	return NewSLogWithPrefixAndDebug(context.TODO(), provider, prefix, isDebug), parentLogger
}
