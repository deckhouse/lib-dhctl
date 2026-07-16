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

package logger

import (
	"io"
	"log/slog"
	"sync"
)

// Options configures the root logger.
//   - FileWriter receives every record (the always-on debug-file sink). Required.
//   - TTYWriter, when non-nil, receives terminal or plain non-TTY output.
//   - IsTTY determines whether TTYWriter supports interactive terminal rendering.
type Options struct {
	FileWriter io.Writer // required; always-on sink
	TTYWriter  io.Writer // optional; terminal or plain non-TTY output sink
	IsTTY      bool      // whether TTYWriter is connected to an interactive terminal
	// Interactive enables the pinned pterm progress bar when TTYWriter is connected
	// to a real terminal. Otherwise, the sink renders plain linear output.
	Interactive bool
	// Verbose (-v) shows every Info+ record in the output stream,
	// not just curated compact output.
	Verbose bool
}

// rootHandler is the *TerminalUIHandler created by the most recent NewRoot call, stored so the
// no-arg RestoreTerminal can leave the alternate screen on any exit path (SIGINT/SIGTERM, panic,
// normal) without threading the handle through the call stack. A CLI owns exactly one terminal, so
// a single guarded slot is the right model. rootMu makes the slot safe against a signal-fired
// RestoreTerminal racing a concurrent NewRoot rebind (action.go installs a fallback root, then
// rebinds the real one). Nil until the first NewRoot.
var (
	rootMu      sync.Mutex
	rootHandler *TerminalUIHandler
)

// setRootHandler records h as the current terminal owner for RestoreTerminal.
func setRootHandler(h *TerminalUIHandler) {
	rootMu.Lock()
	rootHandler = h
	rootMu.Unlock()
}

// RestoreTerminal leaves the alternate screen if the current root handler is using an interactive
// Block. Safe to call before NewRoot, multiple times, and after the Block has already been
// finished. Designed as a no-arg shutdown hook / defer (see cmd/dhctl/main.go).
func RestoreTerminal() {
	rootMu.Lock()
	h := rootHandler
	rootMu.Unlock()
	if h != nil {
		h.RestoreTerminal()
	}
}

// NewRoot builds the application root logger. Replaces InitLogger / InitLoggerWithOptions /
// WrapWithTeeLogger / NewLogToFile from the old package.
func NewRoot(opts Options) *slog.Logger {
	// The file sink always captures every record, including DEBUG.
	// The external output floor is fixed at Info, so DEBUG records remain file-only.
	lv := new(slog.LevelVar)
	lv.Set(slog.LevelDebug)

	// TTYWriter enables external output whenever it is configured.
	// IsTTY selects the rendering mode: interactive terminal output for a real TTY,
	// or plain linear output for non-TTY consumers such as pipes, CI, and the installer.
	// The pinned pterm bar is used only when Interactive is true and the writer is a real terminal.
	// Verbose (-v) shows every Info+ record; otherwise only curated compact output,
	// process markers, and Warn+ records are shown. DEBUG records remain in the file sink only.
	h := newTerminalUIHandler(handlerConfig{
		fileW:       opts.FileWriter,
		ttyW:        opts.TTYWriter,
		isTTY:       opts.IsTTY,
		interactive: opts.Interactive,
		level:       lv,
		verbose:     opts.Verbose,
	})
	setRootHandler(h)
	return slog.New(h)
}
