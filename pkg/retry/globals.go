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

package retry

import (
	"github.com/name212/govalue"

	"github.com/deckhouse/lib-dhctl/pkg/log"
)

type GlobalInterruptChecker func() bool

func SetGlobalInterruptChecker(checker GlobalInterruptChecker) {
	if govalue.IsNil(checker) {
		return
	}

	globalInterruptChecker = checker
}

// SetGlobalDefaultLogger
// Deprecated:
// global logger used for backward compatibility in dhctl with
// deprecated functions NewLoop and NewSilentLoop
// Please use NewLoopWithParams and NewSilentLoopWithParams
func SetGlobalDefaultLogger(logger log.Logger) {
	if govalue.IsNil(logger) {
		return
	}

	defaultLogger = logger
}

var (
	globalInterruptChecker GlobalInterruptChecker = func() bool { return false }
	defaultLogger          log.Logger             = log.NewDummyLogger(false)
	silentLogger                                  = log.NewSilentLogger()
)

func getDefaultSilentLogger() *log.SilentLogger {
	switch defaultLogger.(type) {
	case *log.TeeLogger:
		return defaultLogger.SilentLogger()
	default:
		return silentLogger
	}
}
