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
	"regexp"
	"strings"
	"sync"

	"github.com/name212/govalue"
)

var (
	_ baseLogger              = &InMemoryLogger{}
	_ formatWithNewLineLogger = &InMemoryLogger{}
	_ Logger                  = &InMemoryLogger{}
	_ io.Writer               = &InMemoryLogger{}
)

// Match
// if Regex passed Prefix and Suffix will be ignored
type Match struct {
	Prefix []string
	Suffix []string
	Regex  []*regexp.Regexp
}

func (m *Match) IsValid() error {
	if m == nil {
		return fmt.Errorf("Match is nil")
	}

	if len(m.Regex) > 0 {
		return nil
	}

	if len(m.Prefix) == 0 && len(m.Suffix) == 0 {
		return fmt.Errorf("Invalid Match: must pass Regex or Prefix or/and Suffix")
	}

	return nil
}

type InMemoryLogger struct {
	*formatWithNewLineLoggerWrapper

	m       sync.RWMutex
	entries []string
	buffer  *bytes.Buffer

	parent Logger

	errorPrefix string
	debugPrefix string

	notDebug bool
}

func NewInMemoryLogger() *InMemoryLogger {
	return NewInMemoryLoggerWithParent(NewSilentLogger())
}

func NewInMemoryLoggerWithParent(parent Logger) *InMemoryLogger {
	l := &InMemoryLogger{
		entries: make([]string, 0),
	}

	l.formatWithNewLineLoggerWrapper = newFormatWithNewLineLoggerWrapper(l)

	p := parent

	if govalue.IsNil(p) {
		p = NewSilentLogger()
	}

	l.parent = p

	return l
}

func (l *InMemoryLogger) WithNoDebug(f bool) *InMemoryLogger {
	l.notDebug = f
	return l
}

func (l *InMemoryLogger) WithErrorPrefix(prefix string) *InMemoryLogger {
	l.errorPrefix = prefix
	return l
}

func (l *InMemoryLogger) WithDebugPrefix(prefix string) *InMemoryLogger {
	l.debugPrefix = prefix
	return l
}

func (l *InMemoryLogger) WithBuffer(buffer *bytes.Buffer) *InMemoryLogger {
	l.m.Lock()
	defer l.m.Unlock()

	l.buffer = buffer
	return l
}

func (l *InMemoryLogger) Parent() Logger {
	return l.parent
}

func (l *InMemoryLogger) FirstMatch(m *Match) (string, error) {
	if err := m.IsValid(); err != nil {
		return "", err
	}

	l.m.RLock()
	defer l.m.RUnlock()

	for _, entry := range l.entries {
		if l.match(m, entry) {
			return entry, nil
		}
	}

	return "", nil
}

func (l *InMemoryLogger) AllMatches(m *Match) ([]string, error) {
	if err := m.IsValid(); err != nil {
		return nil, err
	}

	l.m.RLock()
	defer l.m.RUnlock()

	result := make([]string, 0)

	for _, entry := range l.entries {
		if l.match(m, entry) {
			result = append(result, entry)
		}
	}

	return result, nil
}

func (l *InMemoryLogger) FlushAndClose() error {
	return nil
}

func (l *InMemoryLogger) Process(p Process, t string, action func() error) error {
	l.writeEntityFormatted("Start process: %s/%s", p, t)
	err := l.parent.Process(p, t, action)
	l.writeEntityFormatted("End process: %s/%s", p, t)
	return err
}

func (l *InMemoryLogger) InfoF(format string, a ...interface{}) {
	l.writeEntityFormatted(format, a...)
	l.parent.InfoF(format, a...)
}
func (l *InMemoryLogger) InfoLn(a ...interface{}) {
	l.writeEntityFormatted("%v\n", a)
	l.parent.InfoLn(a...)
}

func (l *InMemoryLogger) ErrorF(format string, a ...interface{}) {
	l.writeEntityWithPrefix(l.errorPrefix, format, a...)
	l.parent.ErrorF(format, a...)
}
func (l *InMemoryLogger) ErrorLn(a ...interface{}) {
	l.writeEntityWithPrefix(l.errorPrefix, "%v\n", a)
	l.parent.ErrorLn(a...)
}

func (l *InMemoryLogger) DebugF(format string, a ...interface{}) {
	if l.notDebug {
		return
	}

	l.writeEntityWithPrefix(l.debugPrefix, format, a...)
	l.parent.DebugF(format, a...)
}

func (l *InMemoryLogger) DebugLn(a ...interface{}) {
	if l.notDebug {
		return
	}

	l.writeEntityWithPrefix(l.debugPrefix, "%v\n", a)
	l.parent.DebugLn(a...)
}

func (l *InMemoryLogger) WarnF(format string, a ...interface{}) {
	l.writeEntityFormatted(format, a...)
	l.parent.WarnF(format, a...)
}
func (l *InMemoryLogger) WarnLn(a ...interface{}) {
	l.writeEntityFormatted("%v\n", a)
	l.parent.WarnLn(a...)
}

func (l *InMemoryLogger) Success(s string) {
	l.writeEntityFormatted("Success: %s", s)
	l.parent.Success(s)
}
func (l *InMemoryLogger) Fail(s string) {
	l.writeEntityWithPrefix(l.errorPrefix, "Fail: %s", s)
	l.parent.Fail(s)

}
func (l *InMemoryLogger) FailRetry(s string) {
	l.writeEntityWithPrefix(l.errorPrefix, "Fail retry: %s", s)
	l.parent.FailRetry(s)
}

func (l *InMemoryLogger) JSON(s []byte) {
	l.writeEntity(string(s))
	l.parent.JSON(s)
}

func (l *InMemoryLogger) ProcessLogger() ProcessLogger {
	return l
}

func (l *InMemoryLogger) SilentLogger() *SilentLogger {
	return NewSilentLogger()
}

func (l *InMemoryLogger) BufferLogger(buffer *bytes.Buffer) Logger {
	return l.WithBuffer(buffer)
}

func (l *InMemoryLogger) Write(s []byte) (int, error) {
	l.writeEntity(string(s))
	return l.parent.Write(s)
}

func (l *InMemoryLogger) ProcessStart(name string) {
	l.writeEntityFormatted("Start process: %s", name)
}

func (l *InMemoryLogger) ProcessFail() {
	l.writeEntityWithPrefix(l.errorPrefix, "Fail process")
}

func (l *InMemoryLogger) ProcessEnd() {
	l.writeEntity("End process")
}

func (l *InMemoryLogger) match(m *Match, entity string) bool {
	if len(m.Regex) > 0 {
		for _, regex := range m.Regex {
			if regex.MatchString(entity) {
				return true
			}
		}

		return false
	}

	for _, prefix := range m.Prefix {
		if strings.HasPrefix(entity, prefix) {
			return true
		}
	}

	for _, suffix := range m.Suffix {
		if strings.HasSuffix(entity, suffix) {
			return true
		}
	}

	return false
}

func (l *InMemoryLogger) writeEntity(entity string) {
	l.m.Lock()
	defer l.m.Unlock()

	l.entries = append(l.entries, entity)

	if l.buffer != nil {
		l.buffer.WriteString(entity)
	}
}

func (l *InMemoryLogger) formatString(f string, a ...any) string {
	format := f
	if format == "" {
		format = "%v"
	}

	return fmt.Sprintf(format, a...)
}

func (l *InMemoryLogger) writeEntityFormatted(f string, a ...any) {
	l.writeEntity(l.formatString(f, a...))
}

func (l *InMemoryLogger) writeEntityWithPrefix(prefix, f string, a ...any) {
	msg := l.formatString(f, a...)

	if prefix != "" {
		l.writeEntityFormatted("%s: %s", prefix, msg)
		return
	}

	l.writeEntity(msg)
}
