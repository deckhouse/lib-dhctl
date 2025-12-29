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
	_ Logger    = &DummyLogger{}
	_ io.Writer = &DummyLogger{}
)

type DummyLogger struct {
	isDebug bool
}

func NewDummyLogger(isDebug bool) *DummyLogger {
	return &DummyLogger{
		isDebug: isDebug,
	}
}

func (d *DummyLogger) ProcessLogger() ProcessLogger {
	return newWrappedProcessLogger(d)
}

func (d *DummyLogger) SilentLogger() *SilentLogger {
	return &SilentLogger{}
}

func (d *DummyLogger) BufferLogger(buffer *bytes.Buffer) Logger {
	return NewSimpleLogger(LoggerOptions{OutStream: buffer})
}

func (d *DummyLogger) FlushAndClose() error {
	return nil
}

func (d *DummyLogger) Process(_ Process, t string, run func() error) error {
	fmt.Println(t)
	err := run()
	fmt.Println(t)
	return err
}

func (d *DummyLogger) InfoF(format string, a ...interface{}) {
	fmt.Printf(format, a...)
}

func (d *DummyLogger) InfoLn(a ...interface{}) {
	fmt.Println(a...)
}

func (d *DummyLogger) ErrorF(format string, a ...interface{}) {
	fmt.Printf(format, a...)
}

func (d *DummyLogger) ErrorLn(a ...interface{}) {
	fmt.Println(a...)
}

func (d *DummyLogger) DebugF(format string, a ...interface{}) {
	if d.isDebug {
		fmt.Printf(format, a...)
	}
}

func (d *DummyLogger) DebugLn(a ...interface{}) {
	if d.isDebug {
		fmt.Println(a...)
	}
}

func (d *DummyLogger) Success(l string) {
	fmt.Println(l)
}

func (d *DummyLogger) Fail(l string) {
	fmt.Println(l)
}

func (d *DummyLogger) FailRetry(l string) {
	d.Fail(l)
}

func (d *DummyLogger) WarnLn(a ...interface{}) {
	fmt.Println(a...)
}

func (d *DummyLogger) WarnF(format string, a ...interface{}) {
	fmt.Printf(format, a...)
}

func (d *DummyLogger) JSON(content []byte) {
	fmt.Println(string(content))
}

func (d *DummyLogger) Write(content []byte) (int, error) {
	fmt.Print(string(content))
	return len(content), nil
}
