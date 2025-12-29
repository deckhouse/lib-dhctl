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
	"io"

	"github.com/deckhouse/deckhouse/pkg/log"
)

var (
	_ Logger    = &SimpleLogger{}
	_ io.Writer = &SimpleLogger{}
)

type SimpleLogger struct {
	logger  *log.Logger
	isDebug bool
}

func NewSimpleLogger(opts LoggerOptions) *SimpleLogger {
	//todo: now unused, need change formatter to text when our slog implementation will support it
	l := log.NewLogger()

	if opts.OutStream != nil {
		l.SetOutput(opts.OutStream)
	}

	return &SimpleLogger{
		logger:  l,
		isDebug: opts.IsDebug,
	}

}

func (d *SimpleLogger) BufferLogger(buffer *bytes.Buffer) Logger {
	return NewJSONLogger(LoggerOptions{OutStream: buffer})
}

func (d *SimpleLogger) ProcessLogger() ProcessLogger {
	return newWrappedProcessLogger(d)
}

func (d *SimpleLogger) SilentLogger() *SilentLogger {
	return &SilentLogger{}
}

func (d *SimpleLogger) FlushAndClose() error {
	return nil
}

func (d *SimpleLogger) Process(p Process, t string, run func() error) error {
	d.logger.With("action", "start").With("process", string(p)).Info(t)
	err := run()
	d.logger.With("action", "end").With("process", string(p)).Info(t)
	return err
}

func (d *SimpleLogger) InfoF(format string, a ...interface{}) {
	d.logger.Infof(format, a...)
}

func (d *SimpleLogger) InfoLn(a ...interface{}) {
	d.logger.Infof("%v", a)
}

func (d *SimpleLogger) ErrorF(format string, a ...interface{}) {
	d.logger.Errorf(format, a...)
}

func (d *SimpleLogger) ErrorLn(a ...interface{}) {
	d.logger.Errorf("%v", a)
}

func (d *SimpleLogger) DebugF(format string, a ...interface{}) {
	if d.isDebug {
		d.logger.Debugf(format, a...)
	}
}

func (d *SimpleLogger) DebugLn(a ...interface{}) {
	if d.isDebug {
		d.logger.Debugf("%v", a)
	}
}

func (d *SimpleLogger) Success(l string) {
	d.logger.With("status", "SUCCESS").Info(l)
}

func (d *SimpleLogger) Fail(l string) {
	d.logger.With("status", "FAIL").Error(l)
}

func (d *SimpleLogger) FailRetry(l string) {
	// there used warn log level because in retry cycle we don't want to catch stacktraces which exist as default in Error and Fatal log level of slog logger
	d.logger.With("status", "FAIL").Warn(l)
}

func (d *SimpleLogger) WarnF(format string, a ...interface{}) {
	d.logger.Warnf(format, a...)
}

func (d *SimpleLogger) WarnLn(a ...interface{}) {
	d.logger.Warnf("%v", a)
}

func (d *SimpleLogger) JSON(content []byte) {
	d.logger.Info(string(content))
}

func (d *SimpleLogger) Write(content []byte) (int, error) {
	d.logger.Infof("%s", string(content))
	return len(content), nil
}
