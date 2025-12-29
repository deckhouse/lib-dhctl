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
	"flag"
	"fmt"
	"strings"

	"github.com/name212/govalue"
	"k8s.io/klog/v2"
)

var _ klog.LogFilter = &KeywordSanitizer{}

type Sanitizer interface {
	klog.LogFilter
}

type KlogOpt func(opts *KlogOptions)

type KlogOptions struct {
	// verbose
	// 10 by default
	verbose string

	// verbose
	// use defaultKeywords Sanitizer
	sanitizer Sanitizer
}

func WithKlogVerbose(v string) KlogOpt {
	return func(opts *KlogOptions) {
		if v != "" {
			opts.verbose = v
		}
	}
}

func WithKlogSanitizer(sanitizer Sanitizer) KlogOpt {
	return func(opts *KlogOptions) {
		if !govalue.IsNil(sanitizer) {
			opts.sanitizer = sanitizer
		}
	}
}

func InitKlog(logger Logger, opts ...KlogOpt) error {
	if govalue.IsNil(logger) {
		return fmt.Errorf("logger is not provided to init klog")
	}

	optsForSet := &KlogOptions{
		verbose:   "10",
		sanitizer: NewKeywordSanitizer(),
	}

	for _, opt := range opts {
		opt(optsForSet)
	}

	// we always init klog with maximal log level because we use wrapper for klog output which
	// redirects all output to our logger and our logger doing all "perfect"
	// (logs will out in standalone installer and dhctl-server)
	flags := &flag.FlagSet{}
	klog.InitFlags(flags)

	const logStdErrFlag = "logtostderr"
	if err := flags.Set(logStdErrFlag, "false"); err != nil {
		return flagSetError(logStdErrFlag, err)
	}

	if optsForSet.verbose != "" {
		const vFlag = "v"
		if err := flags.Set(vFlag, optsForSet.verbose); err != nil {
			return flagSetError(vFlag, err)
		}
	}

	if !govalue.IsNil(optsForSet.sanitizer) {
		// filter sensitive keywords
		klog.SetLogFilter(optsForSet.sanitizer)
	}

	klog.SetOutput(newKlogWriterWrapper(logger))

	return nil
}

type KeywordSanitizer struct {
	keywords []string
}

func NewDummySanitizer() Sanitizer {
	return &KeywordSanitizer{keywords: make([]string, 0)}
}

func NewKeywordSanitizer() *KeywordSanitizer {
	keywords := make([]string, len(defaultSensitiveKeywords))
	copy(keywords, defaultSensitiveKeywords)

	return &KeywordSanitizer{keywords: keywords}
}

func (l *KeywordSanitizer) WithAdditionalKeywords(keywords []string) *KeywordSanitizer {
	l.keywords = append(l.keywords, keywords...)

	return l
}

func filteredMsg(matchedKeyword string) string {
	return fmt.Sprintf(`[FILTERED - %s]`, matchedKeyword)
}

func (l *KeywordSanitizer) Filter(args []any) []any {
	for i, arg := range args {
		str, ok := arg.(string)
		if !ok {
			continue
		}
		if matchedKeyword := l.isSensitive(str); matchedKeyword != "" {
			args[i] = filteredMsg(matchedKeyword)
		}
	}
	return args
}

func (l *KeywordSanitizer) FilterF(format string, args []any) (string, []any) {
	return format, l.Filter(args)
}

func (l *KeywordSanitizer) FilterS(msg string, keysAndValues []any) (string, []any) {
	return msg, l.Filter(keysAndValues)
}

// isSensitive - returns empty if is not sensitive
func (l *KeywordSanitizer) isSensitive(msg string) string {
	for _, keyword := range l.keywords {
		if strings.Contains(msg, keyword) {
			return keyword
		}
	}
	return ""
}

func flagSetError(key string, err error) error {
	return fmt.Errorf("Failed to set klog falg '%s': %w", key, err)
}

type klogWriterWrapper struct {
	logger Logger
}

func newKlogWriterWrapper(logger Logger) *klogWriterWrapper {
	return &klogWriterWrapper{logger: logger}
}

func (l *klogWriterWrapper) Write(p []byte) (int, error) {
	l.logger.DebugF("klog: %s", string(p))

	return len(p), nil
}

var defaultSensitiveKeywords = []string{
	`"name":"d8-cluster-terraform-state"`,
	`"name":"d8-provider-cluster-configuration"`,
	`"name":"d8-dhctl-converge-state"`,
	`"kind":"DexProvider"`,
	`"kind":"DexProviderList"`,
	`"kind":"ModuleConfig"`,
	`"kind":"ModuleConfigList"`,
	`"kind":"Secret"`,
	`"kind":"SecretList"`,
	`"kind":"SSHCredentials"`,
	`"kind":"SSHCredentialsList"`,
	`"kind":"ClusterLogDestination"`,
	`"kind":"ClusterLogDestinationList"`,
}
