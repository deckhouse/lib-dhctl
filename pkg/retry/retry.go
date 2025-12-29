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
	"math/rand"
	"time"

	"github.com/name212/govalue"

	"github.com/deckhouse/lib-dhctl/pkg/log"
)

const (
	attemptMessage = `Attempt #%d of %d |
	%s check attempt, retry in %v"
`
	NotSetName = "Name not set"
)

type BreakPredicate func(err error) bool

func IsErr(err error) BreakPredicate {
	return func(target error) bool {
		return errors.Is(err, target)
	}
}

type Params interface {
	WithName(n string) Params
	WithAttempts(attempts int) Params
	WithWait(wait time.Duration) Params
	WithLogger(logger log.Logger) Params

	Name() string
	Attempts() int
	Wait() time.Duration
	Logger() log.Logger

	Clone() Params
	Fill(c Params) Params
}

type params struct {
	name     string
	attempts int
	wait     time.Duration
	logger   log.Logger
}

type ParamsBuilderOpt func(Params)

func WithName(name string) ParamsBuilderOpt {
	return func(p Params) {
		p.WithName(name)
	}
}

func WithAttempts(attempts int) ParamsBuilderOpt {
	return func(p Params) {
		p.WithAttempts(attempts)
	}
}

func AttemptsWithWaitOpts(attempts int, wait time.Duration) []ParamsBuilderOpt {
	return []ParamsBuilderOpt{
		WithAttempts(attempts),
		WithWait(wait),
	}
}

func WithWait(wait time.Duration) ParamsBuilderOpt {
	return func(p Params) {
		p.WithWait(wait)
	}
}

func WithLogger(logger log.Logger) ParamsBuilderOpt {
	return func(p Params) {
		p.WithLogger(logger)
	}
}

// NewParams
// Deprecated:
// use NewEmptyParams with options will be private
func NewParams(name string, attempts int, wait time.Duration) Params {
	return NewEmptyParams().
		WithName(name).
		WithAttempts(attempts).
		WithWait(wait).
		WithLogger(defaultLogger)
}

func NewEmptyParams(opts ...ParamsBuilderOpt) Params {
	p := &params{
		name:     NotSetName,
		attempts: 1,
		wait:     1 * time.Second,
		logger:   defaultLogger,
	}

	for _, opt := range opts {
		opt(p)
	}

	return p
}

func (p *params) WithName(n string) Params {
	if n != "" {
		p.name = n
	}

	return p
}

func (p *params) WithAttempts(attempts int) Params {
	if attempts > 0 {
		p.attempts = attempts
	}

	return p
}

func (p *params) WithWait(wait time.Duration) Params {
	if wait > 0 {
		p.wait = wait
	}
	return p
}

func (p *params) WithLogger(logger log.Logger) Params {
	if !govalue.IsNil(logger) {
		p.logger = logger
	}
	return p
}

func (p *params) Logger() log.Logger {
	return p.logger
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

func (p *params) Clone() Params {
	if govalue.IsNil(p) {
		return nil
	}

	return NewParams(p.Name(), p.Attempts(), p.Wait()).WithLogger(p.Logger())
}

func (p *params) Fill(c Params) Params {
	if govalue.IsNil(p) {
		return nil
	}

	if govalue.IsNil(c) {
		return p
	}

	return p.WithName(c.Name()).
		WithAttempts(c.Attempts()).
		WithWait(c.Wait()).
		WithLogger(c.Logger())
}

func SafeCloneOrNewParams(p Params, opts ...ParamsBuilderOpt) Params {
	if !govalue.IsNil(p) {
		return p.Clone()
	}

	return NewEmptyParams(opts...)
}

// Loop retries a task function until it succeeded with number of attempts and delay between runs are adjustable.
type Loop struct {
	name             string
	attemptsQuantity int
	waitTime         time.Duration
	breakPredicate   BreakPredicate
	logger           log.Logger
	interruptable    bool
	showError        bool
	prefix           string
}

// NewLoop create Loop with features:
// - it is "verbose" loop — it prints messages through logboek.
// - this loop is interruptable by the signal watcher in tomb package.
// Deprecated:
// use NewLoopWithParams in futures versions NewLoop will take Params
func NewLoop(name string, attemptsQuantity int, wait time.Duration) *Loop {
	p := NewParams(name, attemptsQuantity, wait).WithLogger(defaultLogger)
	return NewLoopWithParams(p)
}

func NewLoopWithParams(params Params) *Loop {
	p := params
	if govalue.IsNil(p) {
		p = NewEmptyParams()
	}

	return &Loop{
		name:             p.Name(),
		attemptsQuantity: p.Attempts(),
		waitTime:         p.Wait(),
		logger:           params.Logger(),
		interruptable:    true,
		showError:        true,
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
	p := NewParams(name, attemptsQuantity, wait).WithLogger(getDefaultSilentLogger())
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
		logger:           p.Logger().SilentLogger(),
		// - this loop is not interruptable by the signal watcher in tomb package.
		interruptable: false,
		showError:     true,
		prefix:        fmt.Sprintf("[%s][%d] ", name, rand.Int()),
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

func (l *Loop) WithLogger(logger log.Logger) *Loop {
	l.logger = logger
	return l
}

func (l *Loop) WithShowError(flag bool) *Loop {
	l.showError = flag
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
	if govalue.IsNil(l.logger) {
		return fmt.Errorf("Logger is not provide for loop %s", l.name)
	}

	if l.attemptsQuantity < 1 {
		return fmt.Errorf("Attempts quantity must be greater than zero for loop '%s'", l.name)
	}

	loopBody := func() error {
		var err error
		for i := 1; i <= l.attemptsQuantity; i++ {
			// Check if process is interrupted.
			if l.interruptable && globalInterruptChecker() {
				return fmt.Errorf("Loop was canceled: graceful shutdown")
			}

			// Run task and return if everything is ok.
			err = task()
			if err == nil {
				l.logger.Success(l.prefix + "Succeeded!\n")
				return nil
			}

			if l.breakPredicate != nil && l.breakPredicate(err) {
				l.logger.DebugFLn(l.prefix+"Client break loop with %v", err)
				return err
			}

			l.logger.FailRetry(fmt.Sprintf(l.prefix+attemptMessage, i, l.attemptsQuantity, l.name, l.waitTime))
			errorMsg := "\t%v\n\n"
			if l.showError {
				errorMsg = "\tStatus: %v\n\n"
			}
			l.logger.InfoF(l.prefix+errorMsg, err)

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

	return l.logger.Process(log.ProcessDefault, l.name, loopBody)
}
