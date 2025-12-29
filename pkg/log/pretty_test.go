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
	"slices"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPrettyDefault(t *testing.T) {
	pretty, inMemory := testNewPretty(LoggerOptions{IsDebug: true})
	testPrettyLoggerDefaultProcesses(t, "default", pretty, inMemory)
}

func TestPrettyAdditionalProcesses(t *testing.T) {
	additionalProcesses := AdditionalProcesses(map[string]StyleEntry{
		"myprocess1": {
			Title:         "😀 My Process %s",
			OptionsSetter: BoldOptions,
		},

		"myprocess2": {
			Title:         "Another My Process %s",
			OptionsSetter: BoldOptions,
		},
	})

	pretty, inMemory := testNewPretty(LoggerOptions{
		IsDebug:             true,
		AdditionalProcesses: additionalProcesses,
	})

	const tstPrefix = "additional processes"

	testPrettyLoggerDefaultProcesses(t, tstPrefix, pretty, inMemory)

	for process, style := range additionalProcesses {
		testPrettyLoggerProcess(t, &testPrettyLogger{
			tstPrefix: tstPrefix,

			process: process,
			style:   style,

			logger: pretty,
			out:    inMemory,
		})
	}
}

func TestPrettyRewriteDefaultProcess(t *testing.T) {
	additionalProcesses := Processes{
		ProcessCommon: {
			Title:         "😀 My Common %s",
			OptionsSetter: BoldOptions,
		},

		ProcessInfrastructure: {
			Title:         "My Infrastructure %s",
			OptionsSetter: BoldOptions,
		},
	}

	pretty, inMemory := testNewPretty(LoggerOptions{
		IsDebug:             true,
		AdditionalProcesses: additionalProcesses,
	})

	const tstPrefix = "rewrite default processes"

	testPrettyLoggerDefaultProcesses(t, tstPrefix, pretty, inMemory, ProcessCommon, ProcessInfrastructure)

	for process, style := range additionalProcesses {
		testPrettyLoggerProcess(t, &testPrettyLogger{
			tstPrefix: tstPrefix,

			process: process,
			style:   style,

			logger: pretty,
			out:    inMemory,
		})
	}
}

func TestPrettyDebugStream(t *testing.T) {
	outBuffer := &bytes.Buffer{}
	debugBuffer := &bytes.Buffer{}

	pretty := NewPrettyLogger(LoggerOptions{
		IsDebug:     false,
		OutStream:   outBuffer,
		DebugStream: debugBuffer,
	})

	const (
		infoMsg    = "Info message"
		infoMsgLn  = "Ln info message"
		debugMsg   = "Debug message"
		debugMsgLn = "Ln debug message"
	)

	pretty.InfoF(infoMsg)
	pretty.InfoLn(infoMsgLn)
	pretty.DebugF(debugMsg)
	pretty.DebugLn(debugMsgLn)

	assertInOut := func(t *testing.T, msg string, inOut bool) {
		assertInBuffer(t, outBuffer, msg, inOut)
	}

	assertInDebug := func(t *testing.T, msg string, inOut bool) {
		assertInBuffer(t, debugBuffer, msg, inOut)
	}

	t.Run("info messages in out", func(t *testing.T) {
		assertInOut(t, infoMsg, true)
		assertInOut(t, infoMsgLn, true)
	})

	t.Run("debug messages is not in out", func(t *testing.T) {
		assertInOut(t, debugMsg, false)
		assertInOut(t, debugMsgLn, false)
	})

	t.Run("debug messages in debug stream", func(t *testing.T) {
		assertInDebug(t, debugMsg, true)
		assertInDebug(t, debugMsgLn, true)
	})

	t.Run("info messages is not in debug stream", func(t *testing.T) {
		assertInDebug(t, infoMsg, false)
		assertInDebug(t, infoMsgLn, false)
	})
}

func TestPrettyFollowInterfaces(t *testing.T) {
	assertFollowAllInterfaces(t, NewPrettyLogger(LoggerOptions{IsDebug: true}))
}

func testNewPretty(opts LoggerOptions) (*PrettyLogger, *InMemoryLogger) {
	inMemoryLogger := NewInMemoryLoggerWithParent(NewDummyLogger(opts.IsDebug))

	opts.OutStream = inMemoryLogger

	return NewPrettyLogger(opts), inMemoryLogger
}

type testPrettyLogger struct {
	tstPrefix string
	logger    *PrettyLogger
	out       *InMemoryLogger

	process Process
	style   StyleEntry
}

func testPrettyLoggerProcess(t *testing.T, tst *testPrettyLogger) {
	require.NotNil(t, tst.logger)
	require.NotNil(t, tst.out)
	require.NotEmpty(t, tst.tstPrefix)

	const processName = "dummy"

	t.Run(fmt.Sprintf("%s: %s", tst.tstPrefix, string(tst.process)), func(t *testing.T) {
		inRunMsg := fmt.Sprintf("run in process: %s", string(tst.process))

		err := tst.logger.Process(tst.process, processName, func() error {
			tst.logger.InfoFLn(inRunMsg)
			return nil
		})

		require.NoError(t, err)

		inRunEscaped := regexp.QuoteMeta(inRunMsg)
		expInRun := regexp.MustCompile(fmt.Sprintf("^.* %s\\n", inRunEscaped))
		matchesInRun, err := tst.out.AllMatches(&Match{
			Regex: []*regexp.Regexp{expInRun},
		})
		require.NoError(t, err)
		require.Len(t, matchesInRun, 1, "should have in process run", tst.process)

		title := fmt.Sprintf(tst.style.Title, processName)
		if title == processName {
			// default process has %s title
			return
		}
		titleEscaped := regexp.QuoteMeta(title)
		expProcess := regexp.MustCompile(fmt.Sprintf("^.*%s[\\w\\d\\.\\s\\(\\)]*", titleEscaped))
		matchesProcessStartEnd, err := tst.out.AllMatches(&Match{
			Regex: []*regexp.Regexp{expProcess},
		})
		require.NoError(t, err)
		require.Len(t, matchesProcessStartEnd, 2, "should have title start and end for", tst.process)
	})
}

func testPrettyLoggerDefaultProcesses(t *testing.T, tstPrefix string, logger *PrettyLogger, out *InMemoryLogger, expectProcesses ...Process) {
	expected := append(make([]Process, 0), expectProcesses...)

	for process, style := range defaultProcesses {
		if slices.Contains(expected, process) {
			t.Log(fmt.Sprintf("Default process '%s' skipped", process))
			continue
		}

		testPrettyLoggerProcess(t, &testPrettyLogger{
			tstPrefix: tstPrefix,
			logger:    logger,
			out:       out,

			process: process,
			style:   style,
		})
	}
}
