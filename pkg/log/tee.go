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
	"bufio"
	"bytes"
	"fmt"
	"io"
	"sync"
	"time"
)

var (
	_ baseLogger              = &TeeLogger{}
	_ formatWithNewLineLogger = &TeeLogger{}
	_ Logger                  = &TeeLogger{}
	_ io.Writer               = &TeeLogger{}
)

type TeeLogger struct {
	*formatWithNewLineLoggerWrapper

	l      Logger
	closed bool

	bufMutex sync.Mutex
	buf      *bufio.Writer
	out      io.WriteCloser
}

func newTeeLoggerWithParentAndBuf(l Logger, writer io.WriteCloser, buf *bufio.Writer) *TeeLogger {
	res := &TeeLogger{
		l:   l,
		buf: buf,
		out: writer,
	}

	res.formatWithNewLineLoggerWrapper = newFormatWithNewLineLoggerWrapper(res)

	return res
}

func NewTeeLogger(l Logger, writer io.WriteCloser, bufferSize int) (*TeeLogger, error) {
	buf := bufio.NewWriterSize(writer, bufferSize)

	return newTeeLoggerWithParentAndBuf(l, writer, buf), nil
}

func (d *TeeLogger) BufferLogger(buffer *bytes.Buffer) Logger {
	var l Logger
	switch typedLogger := d.l.(type) {
	case *PrettyLogger:
		l = NewPrettyLogger(LoggerOptions{OutStream: buffer, IsDebug: typedLogger.isDebug})
	case *SimpleLogger:
		l = NewJSONLogger(LoggerOptions{OutStream: buffer, IsDebug: typedLogger.isDebug})
	default:
		l = d.l
	}

	buf := bufio.NewWriterSize(d.out, 4096) // 1024 bytes may not be enough when executing in parallel

	return newTeeLoggerWithParentAndBuf(l, d.out, buf)
}

func (d *TeeLogger) FlushAndClose() error {
	if d.closed {
		return nil
	}

	d.bufMutex.Lock()
	defer d.bufMutex.Unlock()

	err := d.buf.Flush()
	if err != nil {
		d.l.WarnF("Cannot flush TeeLogger: %v", err)
		return err
	}

	d.buf = nil

	err = d.out.Close()
	if err != nil {
		d.l.WarnF("Cannot close TeeLogger file: %v", err)
		return err
	}

	d.closed = true
	return nil
}

func (d *TeeLogger) ProcessLogger() ProcessLogger {
	return newWrappedProcessLogger(d)
}

func (d *TeeLogger) SilentLogger() *SilentLogger {
	return newSilentLoggerWithTee(d)
}

func (d *TeeLogger) Process(p Process, t string, run func() error) error {
	d.writeToFile(fmt.Sprintf("Start process %s\n", t))

	err := d.l.Process(p, t, run)

	d.writeToFile(fmt.Sprintf("End process %s\n", t))

	return err
}

func (d *TeeLogger) InfoFWithoutLn(format string, a ...interface{}) {
	d.l.InfoFWithoutLn(format, a...)

	d.writeToFile(fmt.Sprintf(format, a...))
}

// InfoLn
// Deprecated:
// Use InfoF(string) it add \n to end
func (d *TeeLogger) InfoLn(a ...interface{}) {
	d.l.InfoLn(a...)

	d.writeToFile(fmt.Sprintln(a...))
}

func (d *TeeLogger) ErrorFWithoutLn(format string, a ...interface{}) {
	d.l.ErrorFWithoutLn(format, a...)

	d.writeToFile(fmt.Sprintf(format, a...))
}

// ErrorLn
// Deprecated:
// Use ErrorF(string) it add \n to end
func (d *TeeLogger) ErrorLn(a ...interface{}) {
	d.l.ErrorLn(a...)

	d.writeToFile(fmt.Sprintln(a...))
}

func (d *TeeLogger) DebugFWithoutLn(format string, a ...interface{}) {
	d.l.DebugFWithoutLn(format, a...)

	d.writeToFile(fmt.Sprintf(format, a...))
}

// DebugLn
// Deprecated:
// Use DebugF(string) it add \n to end
func (d *TeeLogger) DebugLn(a ...interface{}) {
	d.l.DebugLn(a...)

	d.writeToFile(fmt.Sprintln(a...))
}

func (d *TeeLogger) Success(l string) {
	d.l.Success(l)

	d.writeToFile(l)
}

func (d *TeeLogger) Fail(l string) {
	d.l.Fail(l)

	d.writeToFile(l)
}

func (d *TeeLogger) FailRetry(l string) {
	d.l.FailRetry(l)

	d.writeToFile(l)
}

// WarnLn
// Deprecated:
// Use WarnF(string) it add \n to end
func (d *TeeLogger) WarnLn(a ...interface{}) {
	d.l.WarnLn(a...)

	d.writeToFile(fmt.Sprintln(a...))
}

func (d *TeeLogger) WarnFWithoutLn(format string, a ...interface{}) {
	d.l.WarnFWithoutLn(format, a...)

	d.writeToFile(fmt.Sprintf(format, a...))
}

func (d *TeeLogger) JSON(content []byte) {
	d.l.JSON(content)

	d.writeToFile(string(content))
}

func (d *TeeLogger) Write(content []byte) (int, error) {
	ln, err := d.l.Write(content)
	if err != nil {
		d.l.DebugF("Cannot write to log: %v", err)
	}

	d.writeToFile(string(content))

	return ln, err
}

func (d *TeeLogger) writeToFile(content string) {
	if d.closed {
		return
	}

	d.bufMutex.Lock()
	defer d.bufMutex.Unlock()

	if d.buf == nil {
		return
	}

	timestamp := time.Now().Format(time.DateTime)
	contentWithTimestamp := fmt.Sprintf("%s - %s", timestamp, content)

	if _, err := d.buf.Write([]byte(contentWithTimestamp)); err != nil {
		d.l.DebugF("Cannot write to TeeLog: %v", err)
	}
}
