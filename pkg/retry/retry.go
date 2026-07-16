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
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand"
	"time"

	"github.com/name212/govalue"

	"github.com/deckhouse/lib-dhctl/pkg/logger"
)

const (
	attemptMessage = `Attempt #%d of %d |
	%s check attempt, retry in %v"
`
	NotSetName = "Name not set"
)

// InTestEnvironment, when set, collapses every loop to a single, wait-free
// attempt so tests exercising retry-driven code don't pay real wall-clock time.
var InTestEnvironment = false

func setupTests(attemptsQuantity *int, wait *time.Duration) {
	if InTestEnvironment {
		*attemptsQuantity = 1
		*wait = 0 * time.Second
	}
}

type BreakPredicate func(err error) bool

func IsErr(err error) BreakPredicate {
	return func(target error) bool {
		return errors.Is(err, target)
	}
}

func isWhitelistedError(err error, whitelist []error) bool {
	for _, target := range whitelist {
		if errors.Is(err, target) {
			return true
		}
	}

	return false
}

type ParamsBuilderOpt func(Params)

type Params interface {
	Name() string
	Attempts() int
	Wait() time.Duration
	Logger() *slog.Logger
	Whitelisted() bool
	WhitelistedErrors() []error

	Clone(overrides ...ParamsBuilderOpt) Params
}

func WithName(format string, args ...any) ParamsBuilderOpt {
	return func(p Params) {
		if format != "" {
			name := fmt.Sprintf(format, args...)
			p.(*params).name = name
		}
	}
}

func WithAttempts(attempts int) ParamsBuilderOpt {
	return func(p Params) {
		if attempts > 0 {
			p.(*params).attempts = attempts
		}
	}
}

func AttemptsWithWaitOpts(attempts int, wait time.Duration) []ParamsBuilderOpt {
	return []ParamsBuilderOpt{
		WithAttempts(attempts),
		WithWait(wait),
	}
}

func WithLogger(l *slog.Logger) ParamsBuilderOpt {
	return func(p Params) {
		if l != nil {
			p.(*params).logger = l
		}
	}
}

func WithWait(wait time.Duration) ParamsBuilderOpt {
	return func(p Params) {
		if wait > 0 {
			p.(*params).wait = wait
		}
	}
}

// WithWhitelist marks the loop as whitelisted and sets the errors that are allowed to be retried.
// If errs is empty, retry behaves as if whitelist was never set (backward compatible).
// If errs is not empty, a returned error that does not match any whitelisted error (via errors.Is)
// stops the loop immediately instead of retrying.
func WithWhitelist(errs ...error) ParamsBuilderOpt {
	return func(p Params) {
		pp := p.(*params)
		pp.whitelisted = true
		pp.whitelistedErrors = errs
	}
}

type params struct {
	name              string
	attempts          int
	wait              time.Duration
	logger            *slog.Logger
	whitelisted       bool
	whitelistedErrors []error
}

// NewParams
// Deprecated:
// use NewEmptyParams with options will be private
func NewParams(name string, attempts int, wait time.Duration) Params {
	return NewEmptyParams(
		WithName("%s", name),
		WithAttempts(attempts),
		WithWait(wait),
	)
}

func NewEmptyParams(opts ...ParamsBuilderOpt) Params {
	p := &params{
		name:     NotSetName,
		attempts: 1,
		wait:     1 * time.Second,
	}

	for _, opt := range opts {
		opt(p)
	}

	return p
}

func (p *params) Name() string {
	return p.name
}

func (p *params) Attempts() int {
	return p.attempts
}

func (p *params) Wait() time.Duration {
	return p.wait
}

func (p *params) Logger() *slog.Logger {
	return p.logger
}

func (p *params) Whitelisted() bool {
	return p.whitelisted
}

func (p *params) WhitelistedErrors() []error {
	return p.whitelistedErrors
}

func (p *params) Clone(overrides ...ParamsBuilderOpt) Params {
	if govalue.IsNil(p) {
		return nil
	}

	// nolint:prealloc
	cloneOpts := []ParamsBuilderOpt{
		WithName("%s", p.Name()),
		WithAttempts(p.Attempts()),
		WithWait(p.Wait()),
	}

	if p.Whitelisted() {
		cloneOpts = append(cloneOpts, WithWhitelist(p.WhitelistedErrors()...))
	}

	cloneOpts = append(cloneOpts, overrides...)

	return NewEmptyParams(cloneOpts...)
}

func SafeCloneOrNewParams(p Params, opts ...ParamsBuilderOpt) Params {
	if !govalue.IsNil(p) {
		return p.Clone()
	}

	return NewEmptyParams(opts...)
}

// Loop retries a task function until it succeeded with number of attempts and delay between runs are adjustable.
type Loop struct {
	name              string
	attemptsQuantity  int
	waitTime          time.Duration
	breakPredicate    BreakPredicate
	logger            *slog.Logger
	interruptable     bool
	showError         bool
	silent            bool
	prefix            string
	whitelisted       bool
	whitelistedErrors []error
}

// NewLoop create Loop with features:
// - it is "verbose" loop — it prints messages through logboek.
// - this loop is interruptable by the signal watcher in tomb package.
// Deprecated:
// use NewLoopWithParams in futures versions NewLoop will take Params
func NewLoop(name string, attemptsQuantity int, wait time.Duration) *Loop {
	p := NewEmptyParams(
		WithName("%s", name),
		WithAttempts(attemptsQuantity),
		WithWait(wait),
	)

	return NewLoopWithParams(p)
}

func NewLoopWithParams(params Params) *Loop {
	p := params
	if govalue.IsNil(p) {
		p = NewEmptyParams()
	}

	return &Loop{
		name:              p.Name(),
		attemptsQuantity:  p.Attempts(),
		waitTime:          p.Wait(),
		logger:            p.Logger(),
		interruptable:     true,
		showError:         true,
		whitelisted:       p.Whitelisted(),
		whitelistedErrors: p.WhitelistedErrors(),
	}
}

func NewLoopWithParamsOpts(opts ...ParamsBuilderOpt) *Loop {
	return NewLoopWithParams(NewEmptyParams(opts...))
}

// NewSilentLoop create Loop with features:
// - it is "silent" loop — no messages are printed through logboek.
// - this loop is not interruptable by the signal watcher in tomb package.
// Deprecated:
// use NewSilentLoopWithParams in futures versions NewSilentLoop will take Params
func NewSilentLoop(name string, attemptsQuantity int, wait time.Duration) *Loop {
	p := NewEmptyParams(
		WithName("%s", name),
		WithAttempts(attemptsQuantity),
		WithWait(wait),
	)

	return NewSilentLoopWithParams(p)
}

func NewSilentLoopWithParams(params Params) *Loop {
	p := params
	if govalue.IsNil(p) {
		p = NewEmptyParams()
	}

	name := p.Name()
	return &Loop{
		name:             name,
		attemptsQuantity: p.Attempts(),
		waitTime:         p.Wait(),
		logger:           p.Logger(),
		// - this loop is not interruptable by the signal watcher in tomb package.
		interruptable:     false,
		showError:         true,
		silent:            true,
		prefix:            fmt.Sprintf("[%s][%d] ", name, rand.Int()),
		whitelisted:       p.Whitelisted(),
		whitelistedErrors: p.WhitelistedErrors(),
	}
}

func NewSilentLoopWithParamsOpts(opts ...ParamsBuilderOpt) *Loop {
	return NewSilentLoopWithParams(NewEmptyParams(opts...))
}

func (l *Loop) BreakIf(pred BreakPredicate) *Loop {
	l.breakPredicate = pred
	return l
}

func (l *Loop) WithInterruptable(flag bool) *Loop {
	l.interruptable = flag
	return l
}

func (l *Loop) WithShowError(flag bool) *Loop {
	l.showError = flag
	return l
}

func (l *Loop) WithLogger(lg *slog.Logger) *Loop {
	l.logger = lg
	return l
}

func (l *Loop) Run(task func() error) error {
	return l.run(context.Background(), task)
}

// RunContext retries a task like Run but breaks if context done.
func (l *Loop) RunContext(ctx context.Context, task func() error) error {
	return l.run(ctx, task)
}

func (l *Loop) run(ctx context.Context, task func() error) error {
	setupTests(&l.attemptsQuantity, &l.waitTime)

	if l.attemptsQuantity < 1 {
		return fmt.Errorf("Attempts quantity must be greater than zero for loop '%s'", l.name)
	}

	if govalue.IsNil(l.logger) {
		l.logger = logger.FromContext(ctx)
	}

	loopBody := func(ctx context.Context) error {
		var err error
		for i := 1; i <= l.attemptsQuantity; i++ {
			// Check if process is interrupted.
			if l.interruptable && globalInterruptChecker() {
				return fmt.Errorf("Loop was canceled: graceful shutdown")
			}

			// Run task and return if everything is ok.
			err = task()
			if err == nil {
				if !l.silent {
					logger.Success(ctx, l.logger, l.prefix+"Succeeded!")
				}

				return nil
			}

			if l.breakPredicate != nil && l.breakPredicate(err) {
				l.logger.DebugContext(ctx, fmt.Sprintf(l.prefix+"Client break loop with %v", err))
				return err
			}

			if l.whitelisted && len(l.whitelistedErrors) > 0 && !isWhitelistedError(err, l.whitelistedErrors) {
				l.logger.DebugContext(ctx, fmt.Sprintf(l.prefix+"Error is not whitelisted, stop loop with %v", err))
				return err
			}

			// Per-attempt diagnostics are Debug-only (file, not terminal): a loop that's
			// going to succeed after a few retries shouldn't spam the compact view with
			// one line per attempt. Only the final exhaustion error (returned below if
			// every attempt fails) is meant to surface to the caller.
			l.logger.DebugContext(ctx, fmt.Sprintf(l.prefix+attemptMessage, i, l.attemptsQuantity, l.name, l.waitTime))
			errorMsg := "\t%v\n\n"
			if l.showError {
				errorMsg = "\tStatus: %v\n\n"
			}
			l.logger.DebugContext(ctx, fmt.Sprintf(l.prefix+errorMsg, err))

			// Do not waitTime after the last iteration.
			if i < l.attemptsQuantity {
				select {
				case <-time.After(l.waitTime):
				case <-ctx.Done():
					return fmt.Errorf("Loop was canceled: %w", ctx.Err())
				}
			}
		}

		return fmt.Errorf("Timeout while %q: last error: %w", l.name, err)
	}

	if l.silent {
		return loopBody(ctx)
	}

	return logger.RunProcess(ctx, l.logger, l.name, loopBody)
}
