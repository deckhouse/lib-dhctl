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
	"regexp"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/lib-dhctl/pkg/log"
)

func TestLoopRunSuccessOnFirstAttempt(t *testing.T) {
	loop := NewLoopWithParams(testLoopParams())
	err := loop.Run(func() error {
		return nil
	})
	assert.NoError(t, err)
}

func TestLoopRunSuccessAfterRetries(t *testing.T) {
	attempt := 0
	loop := NewLoopWithParams(testLoopParams())
	err := loop.Run(func() error {
		attempt++
		if attempt < 3 {
			return errors.New("temporary error")
		}
		return nil
	})
	assert.NoError(t, err)
	assert.Equal(t, 3, attempt)
}

func TestLoopRunBreakIfPredicate(t *testing.T) {
	errorForTest := errors.New("break error")
	loop := NewLoopWithParams(testLoopParams()).BreakIf(IsErr(errorForTest))
	err := loop.Run(func() error {
		return errorForTest
	})
	assert.Error(t, err)
	assert.ErrorIs(t, err, errorForTest)
}

func TestLoopRunContextSuccessOnFirstAttempt(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	loop := NewLoopWithParams(testLoopParams())
	err := loop.RunContext(ctx, func() error {
		return nil
	})
	assert.NoError(t, err)
}

func TestLoopRunContextSuccessAfterRetries(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	attempt := 0
	loop := NewLoopWithParams(testLoopParams())
	err := loop.RunContext(ctx, func() error {
		attempt++
		if attempt < 3 {
			return errors.New("temporary error")
		}
		return nil
	})
	assert.NoError(t, err)
	assert.Equal(t, 3, attempt)
}

func TestLoopRunCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	attempt := 0
	loop := NewLoopWithParams(testLoopParams())
	err := loop.RunContext(ctx, func() error {
		attempt++
		if attempt > 1 {
			cancel()
		}
		return errors.New("error")
	})
	assert.ErrorIs(t, err, context.Canceled)
	assert.Equal(t, 2, attempt)
}

func TestLoopRunDeadlineExceeded(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	attempt := 0
	loop := NewLoopWithParams(testLoopParams())
	err := loop.RunContext(ctx, func() error {
		attempt++
		return errors.New("error")
	})
	assert.ErrorIs(t, err, context.DeadlineExceeded)
	assert.Equal(t, 1, attempt)
}

func TestSilentLoop(t *testing.T) {
	p, logger := testLoopParamsWithLogger()
	loop := NewSilentLoopWithParams(p)
	err := loop.Run(func() error {
		return errors.New("error")
	})
	require.Error(t, err)

	matches, err := logger.AllMatches(stringSubmatch("test loop"))
	require.NoError(t, err)
	require.Len(t, matches, 0)
}

func TestGlobalDefaultLogger(t *testing.T) {
	p, logger := testLoopParamsWithLogger()
	SetGlobalDefaultLogger(logger)

	empty := NewEmptyParams()
	require.Equal(t, logger, empty.Logger())

	const loopName = "some loop"

	loopParamsForCheckLogs := p.Clone().WithName(loopName)
	require.Equal(t, logger, loopParamsForCheckLogs.Logger())

	// check use default logger
	loop := NewLoop(loopParamsForCheckLogs.Name(), loopParamsForCheckLogs.Attempts(), loopParamsForCheckLogs.Wait())
	err := loop.Run(func() error {
		return nil
	})
	require.NoError(t, err)

	matches, err := logger.AllMatches(stringSubmatch(loopName))
	require.NoError(t, err)
	// start and end
	require.Len(t, matches, 2)
}

func TestGlobalGlobalInterruptChecker(t *testing.T) {
	interrupted := false
	checker := func() bool {
		return interrupted
	}

	SetGlobalInterruptChecker(checker)
	attempt := 0
	loop := NewLoopWithParams(testLoopParams())
	err := loop.Run(func() error {
		attempt++
		if attempt > 1 {
			interrupted = true
		}
		return errors.New("error")
	})

	require.Error(t, err)
	require.Equal(t, "Loop was canceled: graceful shutdown", err.Error())
	require.Equal(t, 2, attempt)
}

func testLoopParamsWithLogger() (Params, *log.InMemoryLogger) {
	logger := log.NewInMemoryLoggerWithParent(log.NewDummyLogger(false))
	return NewEmptyParams(
		WithLogger(logger),
		WithName("test loop"),
		WithWait(30*time.Millisecond),
		WithAttempts(3),
	), logger
}

func stringSubmatch(s string) *log.Match {
	escaped := regexp.QuoteMeta(s)
	exp := fmt.Sprintf(".*%s.*", escaped)
	return &log.Match{
		Regex: []*regexp.Regexp{regexp.MustCompile(exp)},
	}
}

func testLoopParams() Params {
	p, _ := testLoopParamsWithLogger()
	return p
}
