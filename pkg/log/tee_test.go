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
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTeeLogger(t *testing.T) {
	debugWriter := newTestWriterCloser()
	parentLogger := NewInMemoryLoggerWithParent(NewSilentLogger()).WithNoDebug(true)
	const oneMB = 1024 * 1024
	teeLogger, err := WrapWithTeeLogger(parentLogger, debugWriter, oneMB)
	require.NoError(t, err)

	const (
		infoMsg    = "Info message"
		infoMsgLn  = "Ln info message"
		debugMsg   = "Debug message"
		debugMsgLn = "Ln debug message"
	)

	allLogs := []string{
		infoMsg,
		infoMsgLn,
		debugMsg,
		debugMsgLn,
	}

	teeLogger.InfoF(infoMsg)
	teeLogger.InfoLn(infoMsgLn)
	teeLogger.DebugF(debugMsg)
	teeLogger.DebugLn(debugMsgLn)

	assertInOut := func(t *testing.T, msg string, inOut bool) {
		matches, err := parentLogger.AllMatches(&Match{
			Prefix: []string{msg, fmt.Sprintf("[%s]", msg)},
		})
		require.NoError(t, err)
		expectLen := 0
		if inOut {
			expectLen = 1
		}

		require.Len(t, matches, expectLen)
	}

	assertValidWriter := func(t *testing.T) {
		require.NotNil(t, debugWriter)
		require.NotNil(t, debugWriter.writer)
	}

	assertInTee := func(t *testing.T, msg string, inOut bool) {
		assertValidWriter(t)

		assertInBuffer(t, debugWriter.writer, msg, inOut)
	}

	assertInTeeAll := func(t *testing.T, msg []string, inOut bool) {
		for _, m := range msg {
			assertInTee(t, m, inOut)
		}
	}

	t.Run("info logs in out", func(t *testing.T) {
		assertInOut(t, infoMsg, true)
		assertInOut(t, infoMsgLn, true)
	})

	t.Run("debug logs is not in out", func(t *testing.T) {
		assertInOut(t, debugMsg, false)
		assertInOut(t, debugMsg, false)
	})

	t.Run("no logs in tee because we use long buffer", func(t *testing.T) {
		assertInTeeAll(t, allLogs, false)
	})

	// close tee should flush logger
	err = teeLogger.FlushAndClose()
	require.NoError(t, err)

	t.Run("tee logger close all", func(t *testing.T) {
		tee, ok := teeLogger.(*TeeLogger)
		require.True(t, ok)

		require.True(t, tee.closed)
		require.True(t, debugWriter.closed)
	})

	t.Run("all logs in tee after flush", func(t *testing.T) {
		assertInTeeAll(t, allLogs, true)
	})

	t.Run("all logs in tee has date", func(t *testing.T) {
		assertValidWriter(t)

		teeOut := debugWriter.writer.String()
		for _, m := range allLogs {
			escaped := regexp.QuoteMeta(m)
			// 2006-01-02 15:04:05
			expStr := fmt.Sprintf("\\d{4}\\-\\d{2}\\-\\d{2} \\d{2}\\:\\d{2}\\:\\d{2} - %s", escaped)
			exp := regexp.MustCompile(expStr)
			require.True(t, exp.MatchString(teeOut), "not in buffer with time", m)
		}
	})

	const (
		afterCloseInfoMsg    = "After close Info message"
		afterCloseInfoMsgLn  = "After close Ln info message"
		afterCloseDebugMsg   = "After close Debug message"
		afterCloseDebugMsgLn = "After close Ln debug message"
	)

	teeLogger.InfoF(afterCloseInfoMsg)
	teeLogger.InfoLn(afterCloseInfoMsgLn)
	teeLogger.DebugF(afterCloseDebugMsg)
	teeLogger.DebugLn(afterCloseDebugMsgLn)

	t.Run("info logs in out after close", func(t *testing.T) {
		assertInOut(t, afterCloseInfoMsg, true)
		assertInOut(t, afterCloseInfoMsgLn, true)
	})

	t.Run("debug logs is not in out after close", func(t *testing.T) {
		assertInOut(t, afterCloseDebugMsg, false)
		assertInOut(t, afterCloseDebugMsgLn, false)
	})

	t.Run("no logs in tee after close", func(t *testing.T) {
		allLogsAfterClose := []string{
			afterCloseInfoMsg,
			afterCloseInfoMsgLn,
			afterCloseDebugMsg,
			afterCloseDebugMsgLn,
		}

		assertInTeeAll(t, allLogsAfterClose, false)
	})
}

func TestTeeLoggerFollowInterfaces(t *testing.T) {
	logger, err := NewTeeLogger(
		NewSimpleLogger(LoggerOptions{IsDebug: true}),
		newTestWriterCloser(),
		1024,
	)

	require.NoError(t, err)

	assertFollowAllInterfaces(t, logger)
}

type testWriterCloser struct {
	writer *bytes.Buffer
	closed bool
}

func newTestWriterCloser() *testWriterCloser {
	return &testWriterCloser{
		writer: bytes.NewBuffer(nil),
		closed: false,
	}
}

func (t *testWriterCloser) Close() error {
	t.closed = true
	return nil
}

func (t *testWriterCloser) Write(p []byte) (n int, err error) {
	return t.writer.Write(p)
}
