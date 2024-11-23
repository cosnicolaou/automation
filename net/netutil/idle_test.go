// Copyright 2024 Cosmos Nicolaou. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package netutil_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/cosnicolaou/automation/net/netutil"
)

func TestWait(t *testing.T) {
	ctx := context.Background()
	timerTick := time.Millisecond * 10
	timer := netutil.NewIdleTimer(timerTick)
	var wg sync.WaitGroup
	wg.Add(1)
	ticks := make(chan time.Time, 10000)

	go func() {
		timer.Wait(ctx, func(ctx context.Context) error {
			ticks <- time.Now()
			return nil
		})
		wg.Done()
	}()

	time.Sleep(time.Second)
	if err := timer.StopWait(ctx); err != nil {
		t.Fatal(err)
	}
	wg.Wait()
	close(ticks)
	if nticks := len(ticks); nticks < 40 {
		t.Errorf("expected at least 40 ticks, got %v", nticks)
	}
	nticks := len(ticks)
	first := <-ticks
	last := first
	for tck := range ticks {
		tickTime := tck.Sub(last)
		if tickTime == 0 || tickTime > timerTick*2 {
			t.Errorf("unexpected tick time: %v", tickTime)
		}
		last = tck
	}

	approx := func(actual, expected time.Duration) bool {
		slack := timerTick * 5
		if actual <= expected-slack || actual >= expected+slack {
			return false
		}
		return true
	}

	if got, want := last.Sub(first), timerTick*time.Duration(nticks); !approx(got, want) {
		t.Errorf("got %v, want < %v", got, want)
	}

}

func TestStopWait(t *testing.T) {
	ctx := context.Background()
	timer := netutil.NewIdleTimer(time.Millisecond)

	readyCh := make(chan struct{})
	waitCh := make(chan time.Time)

	// Make sure StopWait works to stop the timer.
	mytime := time.Now()
	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		timer.Wait(ctx, func(ctx context.Context) error {
			<-readyCh
			waitCh <- mytime
			close(waitCh)
			return nil
		})
		wg.Done()
	}()

	// Allow the expire function to be executed and capture its
	// output (via waitCh).
	readyCh <- struct{}{}
	if got, want := <-waitCh, mytime; !got.Equal(want) {
		t.Errorf("got %v, want %v", got, want)
	}
	if err := timer.StopWait(ctx); err != nil {
		t.Fatal(err)
	}
	// wait will hang unless StopWait has been called.
	wg.Wait()

}

func TestStopWaitCancel(t *testing.T) {
	ctx := context.Background()
	timer := netutil.NewIdleTimer(time.Hour)
	var wg sync.WaitGroup
	wg.Add(1)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		timer.Wait(ctx, func(ctx context.Context) error {
			return nil
		})
		wg.Done()
	}()

	// Canceling the context should be enough to stop the timer.
	cancel()
	wg.Wait()
}

func TestStopWaitHang(t *testing.T) {
	ctx := context.Background()
	timer := netutil.NewIdleTimer(time.Millisecond)
	readyCh := make(chan struct{})
	go func() {
		timer.Wait(ctx, func(ctx context.Context) error {
			// The callback will hang.
			close(readyCh)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Hour):
			}
			return nil
		})
	}()
	<-readyCh
	ctx, cancel := context.WithTimeout(ctx, time.Millisecond*10)
	defer cancel()
	if err := timer.StopWait(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("unexpected or missing error: %v", err)
	}
}
