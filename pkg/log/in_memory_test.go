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

func TestInMemoryLoggerFollowInterfaces(t *testing.T) {
	t.Run("Default constructor", func(t *testing.T) {
		assertFollowAllInterfaces(t, NewInMemoryLogger())
	})

	t.Run("With parent constructor", func(t *testing.T) {
		assertFollowAllInterfaces(t, NewInMemoryLoggerWithParent(NewDummyLogger(true)))
	})
}

func TestInMemoryLoggerProcessLogger(t *testing.T) {
	assertAll := func(t *testing.T, logger *InMemoryLogger, expected []string) {
		for _, prefix := range expected {
			matches, err := logger.AllMatches(&Match{
				Prefix: []string{prefix},
			})

			require.NoError(t, err)
			require.Len(t, matches, 1, "should only have one match for %s", prefix)
			require.Equal(t, prefix, matches[0])
		}
	}

	t.Run("Success", func(t *testing.T) {
		logger := NewInMemoryLoggerWithParent(NewPrettyLogger(LoggerOptions{IsDebug: true}))
		processLogger := logger.ProcessLogger()

		processLogger.ProcessStart("My process")
		logger.InfoF("Do something")
		processLogger.ProcessEnd()

		assertAll(t, logger, []string{
			"Start process: My process",
			"Do something\n",
			"End process",
		})
	})

	t.Run("Failed", func(t *testing.T) {
		logger := NewInMemoryLoggerWithParent(NewPrettyLogger(LoggerOptions{IsDebug: true})).
			WithErrorPrefix("Error")
		processLogger := logger.ProcessLogger()

		processLogger.ProcessStart("My failed process")
		logger.WarnF("Error!")
		processLogger.ProcessFail()

		assertAll(t, logger, []string{
			"Start process: My failed process",
			"Error!\n",
			"Error: Fail process",
		})
	})
}
