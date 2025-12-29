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
	_ Logger    = &SilentLogger{}
	_ io.Writer = &SilentLogger{}
)

type SilentLogger struct {
	t *TeeLogger
}

func NewSilentLogger() *SilentLogger {
	return &SilentLogger{
		t: nil,
	}
}

func (d *SilentLogger) ProcessLogger() ProcessLogger {
	return newWrappedProcessLogger(d)
}

func (d *SilentLogger) SilentLogger() *SilentLogger {
	return &SilentLogger{}
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

func (d *SilentLogger) InfoF(format string, a ...interface{}) {
	if d.t != nil {
		d.t.writeToFile(fmt.Sprintf(format, a...))
	}
}

func (d *SilentLogger) InfoLn(a ...interface{}) {
	if d.t != nil {
		d.t.writeToFile(fmt.Sprintln(a...))
	}
}

func (d *SilentLogger) ErrorF(format string, a ...interface{}) {
	if d.t != nil {
		d.t.writeToFile(fmt.Sprintf(format, a...))
	}
}

func (d *SilentLogger) ErrorLn(a ...interface{}) {
	if d.t != nil {
		d.t.writeToFile(fmt.Sprintln(a...))
	}
}

func (d *SilentLogger) DebugF(format string, a ...interface{}) {
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

func (d *SilentLogger) WarnF(format string, a ...interface{}) {
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
