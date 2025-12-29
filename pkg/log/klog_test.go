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
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/klog/v2"
)

func TestInitDefaultKlog(t *testing.T) {
	logger := testInitKlogLogger(t)

	tests := append([]*baseKlogTest{
		newKlogTest(1),
		newKlogTest(2),
		newKlogTest(3),
		newKlogTest(4),
		newKlogTest(5),
		newKlogTest(6),
		newKlogTest(7),
		newKlogTest(8),
		newKlogTest(9),
		newKlogTest(10),
		newKlogTest(10).withMsg("some message").withName("some message"),
	}, testGetDefaultKeywordTests(true)...)

	doKlogTests(t, "default init", tests, logger)
}

func TestInitKlogWithVerbose(t *testing.T) {
	logger := testInitKlogLogger(t, WithKlogVerbose("3"))

	tests := append([]*baseKlogTest{
		newKlogTest(1),
		newKlogTest(2),
		newKlogTest(3),
		newKlogTest(4).withOut(false),
		newKlogTest(5).withOut(false),
		newKlogTest(6).withOut(false),
		newKlogTest(7).withOut(false),
		newKlogTest(8).withOut(false),
		newKlogTest(9).withOut(false),
		newKlogTest(10).withOut(false),
	}, testGetDefaultKeywordTests(true)...)

	tests = append(tests, testCreateSensitive(`"kind":"ConfigMap"`, false))

	doKlogTests(t, "with verbose", tests, logger)
}

func TestInitKlogWithAdditionalSensitive(t *testing.T) {
	additionalKeywords := []string{`"kind":"ConfigMap"`, `"kind":"SuperSecret"`, "secret string"}
	sanitizer := NewKeywordSanitizer().WithAdditionalKeywords(additionalKeywords)
	logger := testInitKlogLogger(t, WithKlogSanitizer(sanitizer))

	tests := append([]*baseKlogTest{
		newKlogTest(1),
		newKlogTest(10),
	}, testGetDefaultKeywordTests(true)...)

	for _, keyword := range additionalKeywords {
		tests = append(tests, testCreateSensitive(keyword, true))
	}

	tests = append(tests, testCreateSensitive(`"kind":"Pod"`, false))

	doKlogTests(t, "additional sensitive", tests, logger)
}

func TestInitKlogWithDummySanitizerAndVerbose(t *testing.T) {
	sanitizer := NewDummySanitizer()
	logger := testInitKlogLogger(
		t,
		WithKlogSanitizer(sanitizer),
		WithKlogVerbose("2"),
	)

	tests := append([]*baseKlogTest{
		newKlogTest(1),
		newKlogTest(3).withOut(false),
		newKlogTest(10).withOut(false),
	}, testGetDefaultKeywordTests(false)...)

	tests = append(tests, testCreateSensitive(`"kind":"Pod"`, false))

	doKlogTests(t, "dummy sanitizer and verbose", tests, logger)
}

func testInitKlogLogger(t *testing.T, opts ...KlogOpt) *InMemoryLogger {
	logger := NewInMemoryLoggerWithParent(NewSimpleLogger(LoggerOptions{IsDebug: true}))
	err := InitKlog(logger, opts...)
	require.NoError(t, err)

	return logger
}

type baseKlogTest struct {
	name      string
	level     klog.Level
	msg       string
	outMsg    string
	shouldOut bool
}

func (t *baseKlogTest) withMsg(msg string) *baseKlogTest {
	t.msg = msg

	return t
}

func (t *baseKlogTest) withNamePrefix(prefix string) *baseKlogTest {
	t.name = fmt.Sprintf("%s: %s", prefix, t.name)

	return t
}

func (t *baseKlogTest) withName(name string) *baseKlogTest {
	t.name = name

	return t
}

func (t *baseKlogTest) withOutMsg(msg string) *baseKlogTest {
	t.outMsg = msg

	return t
}

func (t *baseKlogTest) withOut(shouldOut bool) *baseKlogTest {
	t.shouldOut = shouldOut

	return t
}

func newKlogTest(l klog.Level) *baseKlogTest {
	msg := fmt.Sprintf("klog_test level %d", l)
	return &baseKlogTest{
		name:      fmt.Sprintf("Test level: %d", l),
		level:     l,
		msg:       msg,
		outMsg:    msg,
		shouldOut: true,
	}
}

func testCreateSensitive(keyword string, shouldFiltered bool) *baseKlogTest {
	msg := fmt.Sprintf(`{"Test": "Yes", %s, "Sensitive": true}`, keyword)
	outMsg := msg
	name := "No filter default sensitive"
	if shouldFiltered {
		name = "Should filter default sensitive"
		outMsg = filteredMsg(keyword)
	}

	splitKM := strings.Split(keyword, ":")
	splitKindName := splitKM[0]
	if len(splitKM) > 1 {
		splitKindName = splitKM[1]
	}

	splitKindName = strings.Trim(splitKindName, `"`)
	name = fmt.Sprintf("%s %s", name, splitKindName)

	return &baseKlogTest{
		name:      name,
		level:     1,
		msg:       msg,
		outMsg:    outMsg,
		shouldOut: true,
	}
}

func testGetDefaultKeywordTests(shouldFiltered bool) []*baseKlogTest {
	result := make([]*baseKlogTest, 0, len(defaultSensitiveKeywords))
	for _, keyword := range defaultSensitiveKeywords {
		result = append(result, testCreateSensitive(keyword, shouldFiltered))
	}

	return result
}

func doKlogTests(t *testing.T, tstPrefix string, tests []*baseKlogTest, logger *InMemoryLogger) {
	for _, tst := range tests {
		tst = tst.withNamePrefix(tstPrefix)
		t.Run(tst.name, func(t *testing.T) {
			klog.V(tst.level).Info(tst.msg)
			msgEscaped := regexp.QuoteMeta(tst.outMsg)
			exp := regexp.MustCompile(fmt.Sprintf("^klog\\: .*\\] %s.*", msgEscaped))
			matches, err := logger.AllMatches(&Match{
				Regex: []*regexp.Regexp{exp},
			})

			require.NoError(t, err)
			expectLen := 0
			if tst.shouldOut {
				expectLen = 1
			}

			require.Len(
				t,
				matches,
				expectLen,
				"logger should contains count of message",
				expectLen,
				tst.msg,
				matches,
			)
		})
	}
}
