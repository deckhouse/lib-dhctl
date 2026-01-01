// Copyright 2026 Flant JSC
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
	"context"
	"fmt"
	"log/slog"
	"strings"
)

type SLogHandler struct {
	loggerProvider LoggerProvider

	attrs       []slog.Attr
	attrsString string
	group       string

	prefix  string
	isDebug bool
}

func NewSLogHandler(provider LoggerProvider) *SLogHandler {
	return NewSLogHandlerWithPrefix(provider, "")
}

func NewSLogHandlerWithPrefix(provider LoggerProvider, prefix string) *SLogHandler {
	return &SLogHandler{
		loggerProvider: provider,
		prefix:         prefix,
	}
}

func NewSLogWithDebug(ctx context.Context, provider LoggerProvider, isDebug bool) *slog.Logger {
	return NewSLogWithPrefixAndDebug(ctx, provider, "", isDebug)
}

func NewSLogWithPrefix(ctx context.Context, provider LoggerProvider, prefix string) *slog.Logger {
	return NewSLogWithPrefixAndDebug(ctx, provider, prefix, false)
}

func NewSLogWithPrefixAndDebug(ctx context.Context, provider LoggerProvider, prefix string, isDebug bool) *slog.Logger {
	handler := NewSLogHandlerWithPrefix(provider, prefix).WithDebug(isDebug)

	logger := slog.New(handler)
	lvl := slog.LevelInfo
	if isDebug {
		lvl = slog.LevelDebug
	}
	logger.Enabled(ctx, lvl)

	return logger
}

func (h *SLogHandler) WithPrefix(p string) *SLogHandler {
	h.prefix = p

	return h
}

func (h *SLogHandler) WithDebug(d bool) *SLogHandler {
	h.isDebug = d

	return h
}

func copyHandler(h *SLogHandler) *SLogHandler {
	return &SLogHandler{
		loggerProvider: h.loggerProvider,
		attrsString:    h.attrsString,
		group:          h.group,
		prefix:         h.prefix,
		isDebug:        h.isDebug,
	}
}

func attrsToString(attrs []slog.Attr) string {
	if len(attrs) == 0 {
		return ""
	}

	builder := strings.Builder{}
	for _, attr := range attrs {
		builder.WriteString(attr.Key)
		builder.WriteString("=")
		builder.WriteString(fmt.Sprintf(`'%s' `, attr.Value.String()))
	}

	return fmt.Sprintf(" | attributes: [%s]", strings.TrimRight(builder.String(), " "))
}

func copyAttrs(attrs []slog.Attr) []slog.Attr {
	if len(attrs) == 0 {
		return nil
	}

	cpy := make([]slog.Attr, len(attrs))
	copy(cpy, attrs)
	return cpy
}

func newHandlerWithAttrs(parent *SLogHandler, attrs []slog.Attr) *SLogHandler {
	a := append(copyAttrs(parent.attrs), attrs...)

	res := copyHandler(parent)
	res.attrsString = attrsToString(a)

	return res
}

func newHandlerWithGroup(parent *SLogHandler, group string) *SLogHandler {
	g := group
	if parent.group != "" {
		g = parent.group + "/" + group
	}

	res := copyHandler(parent)
	res.group = g

	return res
}

func (h *SLogHandler) Enabled(_ context.Context, lvl slog.Level) bool {
	if h.isDebug {
		// handle all
		return true
	}

	return lvl >= slog.LevelInfo
}

func (h *SLogHandler) Handle(_ context.Context, record slog.Record) error {
	logger := SafeProvideLogger(h.loggerProvider)
	write := logger.DebugF
	switch record.Level {
	case slog.LevelDebug:
		write = logger.DebugF
	case slog.LevelInfo:
		write = logger.InfoF
	case slog.LevelWarn:
		write = logger.WarnF
	case slog.LevelError:
		write = logger.ErrorF
	}

	write(h.message(record.Message))

	return nil
}

func (h *SLogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return newHandlerWithAttrs(h, attrs)
}

func (h *SLogHandler) WithGroup(name string) slog.Handler {
	return newHandlerWithGroup(h, name)
}

func (h *SLogHandler) message(msg string) string {
	totalMsg := strings.Builder{}
	if h.prefix != "" {
		totalMsg.WriteString(fmt.Sprintf("%s: %s", h.prefix, msg))
	} else {
		totalMsg.WriteString(msg)
	}

	if h.group != "" {
		totalMsg.WriteString(fmt.Sprintf(" | groups: '%s'", h.group))
	}

	if h.attrsString != "" {
		// h.attrs contains leading space and | before attributes
		totalMsg.WriteString(h.attrsString)
	}

	return totalMsg.String()
}
