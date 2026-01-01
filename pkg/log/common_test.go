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
	"bytes"
	"fmt"
	"testing"

	"github.com/name212/govalue"
	"github.com/stretchr/testify/require"
)

func assertInBuffer(t *testing.T, buf *bytes.Buffer, msg string, inOut bool) {
	out := buf.String()
	assert := require.NotContains
	if inOut {
		assert = require.Contains
	}
	assert(t, out, msg)
}

func assertFollowAllInterfaces(t *testing.T, logger Logger) {
	t.Run("Silent logger", func(t *testing.T) {
		assertSilentLoggerProviderFollowFormatLnInterface(t, logger)
	})

	t.Run("Format with Ln logger", func(t *testing.T) {
		assertFollowFormatLnInterface(t, logger)
	})

	t.Run("Buffered logger", func(t *testing.T) {
		assertBufferedLoggerProviderFollowFormatLnInterface(t, logger)
	})
}

func assertFollowFormatLnInterface(t *testing.T, logger Logger) {
	runs := []func(){
		func() {
			logger.InfoF("INFO %s", "test_info")
		},
		func() {
			logger.WarnF("WARN %s", "test_warn")
		},

		func() {
			logger.DebugF("DEBUG %s", "test_debug")
		},

		func() {
			logger.ErrorF("ERROR %v", fmt.Errorf("test_error"))
		},
	}

	for i, run := range runs {
		t.Run(fmt.Sprintf("Does not panic FLn func %d", i), func(t *testing.T) {
			require.NotPanics(t, run)
		})
	}
}

func assertSilentLoggerProviderFollowFormatLnInterface(t *testing.T, provider silentLoggerProvider) {
	silentLogger := provider.SilentLogger()

	require.NotNil(t, silentLogger)

	assertFollowFormatLnInterface(t, silentLogger)
}

func assertBufferedLoggerProviderFollowFormatLnInterfaceWithoutCheckWrite(t *testing.T, provider bufferLoggerProvider) *bytes.Buffer {
	buf := bytes.NewBuffer(nil)

	bufferedLogger := provider.BufferLogger(buf)

	require.False(t, govalue.IsNil(bufferedLogger))

	assertFollowFormatLnInterface(t, bufferedLogger)

	return buf
}

func assertBufferedLoggerProviderFollowFormatLnInterface(t *testing.T, provider bufferLoggerProvider) {
	buf := assertBufferedLoggerProviderFollowFormatLnInterfaceWithoutCheckWrite(t, provider)

	messagesInBuffer := []string{
		"INFO test_info",
		"WARN test_warn",
		"DEBUG test_debug",
		"ERROR test_error",
	}

	bufContent := buf.String()

	for _, message := range messagesInBuffer {
		require.Contains(t, bufContent, message, "expected buffer to contain", message)
	}
}
