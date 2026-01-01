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
)

var (
	_ baseLogger              = &SilentLogger{}
	_ formatWithNewLineLogger = &SilentLogger{}
	_ Logger                  = &SilentLogger{}
	_ io.Writer               = &SilentLogger{}
)

type SilentLogger struct {
	*formatWithNewLineLoggerWrapper

	t *TeeLogger
}

func NewSilentLogger() *SilentLogger {
	return newSilentLoggerWithTee(nil)
}

func newSilentLoggerWithTee(t *TeeLogger) *SilentLogger {
	l := &SilentLogger{
		t: t,
	}

	l.formatWithNewLineLoggerWrapper = newFormatWithNewLineLoggerWrapper(l)

	return l
}

func (d *SilentLogger) ProcessLogger() ProcessLogger {
	return newWrappedProcessLogger(d)
}

func (d *SilentLogger) SilentLogger() *SilentLogger {
	return NewSilentLogger()
}

func (d *SilentLogger) BufferLogger(buffer *bytes.Buffer) Logger {
	return d
}

func (d *SilentLogger) Process(_ Process, t string, run func() error) error {
	err := run()
	return err
}

func (d *SilentLogger) FlushAndClose() error {
	return nil
}

func (d *SilentLogger) InfoFWithoutLn(format string, a ...interface{}) {
	if d.t != nil {
		d.t.writeToFile(fmt.Sprintf(format, a...))
	}
}

func (d *SilentLogger) InfoLn(a ...interface{}) {
	if d.t != nil {
		d.t.writeToFile(fmt.Sprintln(a...))
	}
}

func (d *SilentLogger) ErrorFWithoutLn(format string, a ...interface{}) {
	if d.t != nil {
		d.t.writeToFile(fmt.Sprintf(format, a...))
	}
}

func (d *SilentLogger) ErrorLn(a ...interface{}) {
	if d.t != nil {
		d.t.writeToFile(fmt.Sprintln(a...))
	}
}

func (d *SilentLogger) DebugFWithoutLn(format string, a ...interface{}) {
	if d.t != nil {
		d.t.writeToFile(fmt.Sprintf(format, a...))
	}
}

func (d *SilentLogger) DebugLn(a ...interface{}) {
	if d.t != nil {
		d.t.writeToFile(fmt.Sprintln(a...))
	}
}

func (d *SilentLogger) Success(l string) {
	if d.t != nil {
		d.t.writeToFile(l)
	}
}

func (d *SilentLogger) Fail(l string) {
	if d.t != nil {
		d.t.writeToFile(l)
	}
}

func (d *SilentLogger) FailRetry(l string) {
	if d.t != nil {
		d.t.writeToFile(l)
	}
}

func (d *SilentLogger) WarnLn(a ...interface{}) {
	if d.t != nil {
		d.t.writeToFile(fmt.Sprintln(a...))
	}
}

func (d *SilentLogger) WarnFWithoutLn(format string, a ...interface{}) {
	if d.t != nil {
		d.t.writeToFile(fmt.Sprintf(format, a...))
	}
}

func (d *SilentLogger) JSON(content []byte) {
	if d.t != nil {
		d.t.writeToFile(string(content))
	}
}

func (d *SilentLogger) Write(content []byte) (int, error) {
	if d.t != nil {
		d.t.writeToFile(string(content))
	}
	return len(content), nil
}
