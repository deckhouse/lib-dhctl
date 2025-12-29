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
	"io"
	"maps"
	"slices"
	"strings"

	"github.com/name212/govalue"
	"github.com/werf/logboek/pkg/types"

	"github.com/deckhouse/deckhouse/pkg/log"
)

type Type string

const (
	Pretty Type = "pretty"
	JSON   Type = "json"
	Empty  Type = "silent"
	Simple Type = "simple"
)

type Process string

const (
	ProcessDefault        Process = "default"
	ProcessCommon         Process = "common"
	ProcessInfrastructure Process = "infrastructure"
	ProcessConverge       Process = "converge"
	ProcessBootstrap      Process = "bootstrap"
)

// Deprecated: add additional processes via opts
const (
	ProcessMirror          Process = "mirror"
	ProcessCommanderAttach Process = "commander/attach"
	ProcessCommanderDetach Process = "commander/detach"
)

var (
	defaultProcesses = Processes{
		ProcessCommon:          {"🎈 ~ Common: %s", commonOptions},
		ProcessInfrastructure:  {"🌱 ~ Infrastructure: %s", InfrastructureOptions},
		ProcessConverge:        {"🛸 ~ Converge: %s", convergeOptions},
		ProcessBootstrap:       {"⛵ ~ Bootstrap: %s", bootstrapOptions},
		ProcessMirror:          {"🪞 ~ Mirror: %s", mirrorOptions},
		ProcessCommanderAttach: {"⚓ ~ Attach to commander: %s", commanderAttachOptions},
		ProcessCommanderDetach: {"🚢 ~ Detach from commander: %s", commanderDetachOptions},
		ProcessDefault:         {"%s", BoldOptions},
	}
)

func AdditionalProcesses(processes map[string]StyleEntry) Processes {
	res := make(Processes, len(processes))
	for k, v := range processes {
		res[Process(k)] = v
	}

	return res
}

type (
	Processes               map[Process]StyleEntry
	StyleEntryOptionsSetter func(opts types.LogProcessOptionsInterface)
)

type StyleEntry struct {
	Title         string
	OptionsSetter StyleEntryOptionsSetter
}

type ProcessLogger interface {
	ProcessStart(name string)
	ProcessFail()
	ProcessEnd()
}

type silentLoggerProvider interface {
	SilentLogger() *SilentLogger
}

type bufferLoggerProvider interface {
	BufferLogger(buffer *bytes.Buffer) Logger
}

type baseLogger interface {
	silentLoggerProvider
	bufferLoggerProvider

	FlushAndClose() error

	Process(Process, string, func() error) error

	InfoF(format string, a ...interface{})
	InfoLn(a ...interface{})

	ErrorF(format string, a ...interface{})
	ErrorLn(a ...interface{})

	DebugF(format string, a ...interface{})
	DebugLn(a ...interface{})

	WarnF(format string, a ...interface{})
	WarnLn(a ...interface{})

	Success(string)
	Fail(string)
	FailRetry(string)

	JSON([]byte)
	Write([]byte) (int, error)

	ProcessLogger() ProcessLogger
}

type formatWithNewLineLogger interface {
	InfoFLn(format string, a ...any)
	ErrorFLn(format string, a ...any)
	DebugFLn(format string, a ...any)
	WarnFLn(format string, a ...any)
}

type Logger interface {
	formatWithNewLineLogger
	baseLogger
}

type LoggerOptions struct {
	OutStream   io.Writer
	Width       int
	IsDebug     bool
	DebugStream io.Writer

	AdditionalProcesses Processes
}

var (
	typesMap = map[string]Type{
		string(Pretty): Pretty,
		string(Simple): Simple,
		string(JSON):   JSON,
		string(Empty):  Empty,
	}
)

func ConvertType(t string) (Type, error) {
	tt, ok := typesMap[t]
	if !ok {
		typesList := strings.Join(slices.Collect(maps.Keys(typesMap)), ", ")
		return Empty, fmt.Errorf("Unknown log type: '%s'. Should be %s", t, typesList)
	}

	return tt, nil
}

// NewLogger
// do not init Klog use InitKlog for initialize Klog wrapper
func NewLogger(loggerType Type, isDebug bool) (Logger, error) {
	return NewLoggerWithOptions(loggerType, LoggerOptions{IsDebug: isDebug})
}

// NewLoggerWithOptions
// do not init Klog use InitKlog for initialize Klog wrapper
func NewLoggerWithOptions(loggerType Type, opts LoggerOptions) (Logger, error) {
	var l Logger
	switch loggerType {
	case Pretty:
		l = NewPrettyLogger(opts)
	case Simple:
		l = NewSimpleLogger(opts)
	case JSON:
		l = NewJSONLogger(opts)
	case Empty:
		l = NewSilentLogger()
	default:
		return nil, fmt.Errorf("Unknown logger type: %s", loggerType)
	}

	if govalue.IsNil(l) {
		return nil, fmt.Errorf("Internal error. Unable to create new logger")
	}

	// Mute Shell-Operator logs
	log.Default().SetLevel(log.LevelFatal)
	if opts.IsDebug {
		// Enable shell-operator log, because it captures klog output
		// todo: capture output of klog with default logger instead
		log.Default().SetLevel(log.LevelDebug)
		// Wrap them with our default logger
		log.Default().SetOutput(l)
	}

	return l, nil
}

func WrapWithTeeLogger(logger Logger, writer io.WriteCloser, bufSize int) (Logger, error) {
	l, err := NewTeeLogger(logger, writer, bufSize)
	if err != nil {
		return nil, err
	}

	return l, nil
}
