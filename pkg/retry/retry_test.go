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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	p := testLoopParams()
	loop := NewSilentLoopWithParams(p)
	err := loop.Run(func() error {
		return errors.New("error")
	})
	require.Error(t, err)
}

func TestLoopRunWhitelistStopsOnNonWhitelistedError(t *testing.T) {
	whitelistedErr := errors.New("whitelisted error")
	otherErr := errors.New("other error")

	attempt := 0
	loop := NewLoopWithParams(testLoopParams(WithWhitelist(whitelistedErr)))
	err := loop.Run(func() error {
		attempt++
		return otherErr
	})

	require.ErrorIs(t, err, otherErr)
	require.Equal(t, 1, attempt)
}

func TestLoopRunWhitelistRetriesOnWhitelistedError(t *testing.T) {
	whitelistedErr := errors.New("whitelisted error")

	attempt := 0
	loop := NewLoopWithParams(testLoopParams(WithWhitelist(whitelistedErr)))
	err := loop.Run(func() error {
		attempt++
		if attempt < 3 {
			return whitelistedErr
		}
		return nil
	})

	require.NoError(t, err)
	require.Equal(t, 3, attempt)
}

func TestLoopRunWhitelistEmptyBehavesAsUnset(t *testing.T) {
	attempt := 0
	loop := NewLoopWithParams(testLoopParams(WithWhitelist()))
	err := loop.Run(func() error {
		attempt++
		if attempt < 3 {
			return errors.New("temporary error")
		}
		return nil
	})

	require.NoError(t, err)
	require.Equal(t, 3, attempt)
}

func TestLoopRunInTestEnvironmentCollapsesToSingleAttempt(t *testing.T) {
	InTestEnvironment = true
	defer func() { InTestEnvironment = false }()

	attempt := 0
	start := time.Now()
	loop := NewLoopWithParams(testLoopParams())
	err := loop.Run(func() error {
		attempt++
		return errors.New("error")
	})

	require.Error(t, err)
	require.Equal(t, 1, attempt)
	require.Less(t, time.Since(start), 30*time.Millisecond)
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

func testLoopParams(extraOpts ...ParamsBuilderOpt) Params {
	// nolint:prealloc
	opts := []ParamsBuilderOpt{
		WithName("test loop"),
		WithWait(30 * time.Millisecond),
		WithAttempts(3),
	}
	opts = append(opts, extraOpts...)

	return NewEmptyParams(opts...)
}
