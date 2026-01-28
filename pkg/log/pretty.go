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
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"os"

	"github.com/gookit/color"
	"github.com/name212/govalue"
	"github.com/werf/logboek"
	"github.com/werf/logboek/pkg/level"
	"github.com/werf/logboek/pkg/types"
)

var (
	_ baseLogger              = &PrettyLogger{}
	_ formatWithNewLineLogger = &PrettyLogger{}
	_ Logger                  = &PrettyLogger{}
	_ io.Writer               = &PrettyLogger{}
)

type debugLogWriter struct {
	DebugStream io.Writer
}

type PrettyLogger struct {
	*formatWithNewLineLoggerWrapper

	processTitles  Processes
	isDebug        bool
	logboekLogger  types.LoggerInterface
	debugLogWriter *debugLogWriter
}

func NewPrettyLogger(opts LoggerOptions) *PrettyLogger {
	processes := make(Processes, len(defaultProcesses))
	maps.Copy(processes, defaultProcesses)

	if len(opts.AdditionalProcesses) > 0 {
		for process, style := range opts.AdditionalProcesses {
			processes[process] = style
		}
	}

	res := &PrettyLogger{
		processTitles: processes,
		isDebug:       opts.IsDebug,
	}

	res.formatWithNewLineLoggerWrapper = newFormatWithNewLineLoggerWrapper(res)

	if opts.OutStream != nil {
		res.logboekLogger = logboek.DefaultLogger().NewSubLogger(opts.OutStream, opts.OutStream)
	} else {
		res.logboekLogger = logboek.DefaultLogger()
	}

	if !govalue.IsNil(opts.DebugStream) {
		res.debugLogWriter = &debugLogWriter{DebugStream: opts.DebugStream}
	}

	res.logboekLogger.SetAcceptedLevel(level.Info)

	if opts.Width != 0 {
		res.logboekLogger.Streams().SetWidth(opts.Width)
	} else {
		res.logboekLogger.Streams().SetWidth(140)
	}

	if opts.IsDebug {
		res.logboekLogger.Streams().DisableProxyStreamDataFormatting()
	} else {
		res.logboekLogger.Streams().EnableProxyStreamDataFormatting()
	}

	return res
}

func (d *PrettyLogger) FlushAndClose() error {
	return nil
}

func (d *PrettyLogger) ProcessLogger() ProcessLogger {
	return newPrettyProcessLogger(d.logboekLogger)
}

func (d *PrettyLogger) SilentLogger() *SilentLogger {
	return NewSilentLogger()
}

func (d *PrettyLogger) BufferLogger(buffer *bytes.Buffer) Logger {
	return NewPrettyLogger(LoggerOptions{OutStream: buffer, IsDebug: d.isDebug})
}

func (d *PrettyLogger) Process(p Process, t string, run func() error) error {
	format, ok := d.processTitles[p]
	if !ok {
		format = d.processTitles["default"]
	}
	return d.logboekLogger.LogProcess(format.Title, t).Options(format.OptionsSetter).DoError(run)
}

func (d *PrettyLogger) InfoFWithoutLn(format string, a ...interface{}) {
	d.logboekLogger.Info().LogF(format, a...)
}

// InfoLn
// Deprecated:
// Use InfoF(string) it add \n to end
func (d *PrettyLogger) InfoLn(a ...interface{}) {
	d.logboekLogger.Info().LogLn(a...)
}

func (d *PrettyLogger) ErrorFWithoutLn(format string, a ...interface{}) {
	d.logboekLogger.Error().LogF(format, a...)
}

// ErrorLn
// Deprecated:
// Use ErrorF(string) it add \n to end
func (d *PrettyLogger) ErrorLn(a ...interface{}) {
	d.logboekLogger.Error().LogLn(a...)
}

func (d *PrettyLogger) DebugFWithoutLn(format string, a ...interface{}) {
	if d.debugLogWriter != nil {
		o := fmt.Sprintf(format, a...)
		_, err := d.debugLogWriter.DebugStream.Write([]byte(o))
		if err != nil {
			fmt.Fprintf(os.Stderr, "cannot write debug log (%s): %v", o, err)
		}
	}

	if d.isDebug {
		d.logboekLogger.Info().LogF(format, a...)
	}
}

// DebugLn
// Deprecated:
// Use DebugF(string) it add \n to end
func (d *PrettyLogger) DebugLn(a ...interface{}) {
	if d.debugLogWriter != nil {
		o := fmt.Sprintln(a...)
		_, err := d.debugLogWriter.DebugStream.Write([]byte(o))
		if err != nil {
			d.logboekLogger.Info().LogF("cannot write debug log (%s): %v", o, err)
		}
	}

	if d.isDebug {
		d.logboekLogger.Info().LogLn(a...)
	}
}

func (d *PrettyLogger) Success(l string) {
	d.InfoFWithoutLn("🎉 %s", l)
}

func (d *PrettyLogger) Fail(l string) {
	d.InfoFWithoutLn("️⛱️️ %s", l)
}

func (d *PrettyLogger) FailRetry(l string) {
	d.Fail(l)
}

// WarnLn
// Deprecated:
// Use WarnF(string) it add \n to end
func (d *PrettyLogger) WarnLn(a ...interface{}) {
	a = append([]interface{}{"❗ ~ "}, a...)
	d.InfoLn(color.New(color.Bold).Sprint(a...))
}

func (d *PrettyLogger) WarnFWithoutLn(format string, a ...interface{}) {
	line := color.New(color.Bold).Sprintf("❗ ~ "+format, a...)
	d.InfoFWithoutLn(line)
}

func (d *PrettyLogger) JSON(content []byte) {
	d.InfoF(prettyJSON(content))
}

func (d *PrettyLogger) Write(content []byte) (int, error) {
	d.InfoFWithoutLn(string(content))
	return len(content), nil
}

func prettyJSON(content []byte) string {
	result := &bytes.Buffer{}
	if err := json.Indent(result, content, "", "  "); err != nil {
		panic(err)
	}

	return result.String()
}

func bootstrapOptions(opts types.LogProcessOptionsInterface) {
	opts.Style(color.New(color.FgYellow, color.Bold))
}

func mirrorOptions(opts types.LogProcessOptionsInterface) {
	opts.Style(color.New(color.FgGreen, color.Bold))
}

func commanderAttachOptions(opts types.LogProcessOptionsInterface) {
	opts.Style(color.New(color.FgLightCyan, color.Bold))
}

func commanderDetachOptions(opts types.LogProcessOptionsInterface) {
	opts.Style(color.New(color.FgLightCyan, color.Bold))
}

func commonOptions(opts types.LogProcessOptionsInterface) {
	opts.Style(color.New(color.FgBlue, color.Bold))
}

func BoldOptions(opts types.LogProcessOptionsInterface) {
	opts.Style(boldStyle())
}

func BoldStartOptions(opts types.LogProcessOptionsInterface) {
	opts.Style(boldStyle())
}

func BoldEndOptions(opts types.LogProcessOptionsInterface) {
	opts.Style(boldStyle())
}

func BoldFailOptions(opts types.LogProcessOptionsInterface) {
	opts.Style(boldStyle())
}

func InfrastructureOptions(opts types.LogProcessOptionsInterface) {
	opts.Style(color.New(color.FgGreen, color.Bold))
}

func convergeOptions(opts types.LogProcessOptionsInterface) {
	opts.Style(color.New(color.FgLightCyan, color.Bold))
}

func boldStyle() color.Style {
	return color.New(color.Bold)
}
