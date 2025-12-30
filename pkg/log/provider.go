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

import "github.com/name212/govalue"

var silentLoggerInstance = NewSilentLogger()

type LoggerProvider func() Logger

func SimpleLoggerProvider(logger Logger) LoggerProvider {
	return func() Logger {
		return logger
	}
}

func SafeProvideLogger(provider LoggerProvider) Logger {
	return ProvideSafe(provider, silentLoggerInstance)
}

func SilentLoggerProvider() LoggerProvider {
	return SimpleLoggerProvider(silentLoggerInstance)
}

func ProvideSafe(provider LoggerProvider, defaultLogger Logger) Logger {
	if provider != nil {
		logger := provider()
		if !govalue.IsNil(logger) {
			return logger
		}
	}

	return defaultLogger
}
