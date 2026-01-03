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

// formatWithNewLineLogger
// we often use *F function, but for pretty log we use "\n" in end of string
// this interface and wrapper help us for get rit of this
type formatWithNewLineLoggerWrapper struct {
	parent baseLogger
}

func newFormatWithNewLineLoggerWrapper(parent baseLogger) *formatWithNewLineLoggerWrapper {
	return &formatWithNewLineLoggerWrapper{parent: parent}
}

func (w *formatWithNewLineLoggerWrapper) InfoF(format string, a ...any) {
	w.parent.InfoFWithoutLn(addLnToFormat(format), a...)
}

func (w *formatWithNewLineLoggerWrapper) ErrorF(format string, a ...any) {
	w.parent.ErrorFWithoutLn(addLnToFormat(format), a...)
}

func (w *formatWithNewLineLoggerWrapper) DebugF(format string, a ...any) {
	w.parent.DebugFWithoutLn(addLnToFormat(format), a...)
}

func (w *formatWithNewLineLoggerWrapper) WarnF(format string, a ...any) {
	w.parent.WarnFWithoutLn(addLnToFormat(format), a...)
}

func addLnToFormat(format string) string {
	return format + "\n"
}
