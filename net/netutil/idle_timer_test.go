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

func TestIdleWait(t *testing.T) {
	ctx := context.Background()

	timerTick := time.Millisecond * 10
	timer := netutil.NewIdleTimer(timerTick)
	var wg sync.WaitGroup

	iterations := 40
	ticks := make(chan time.Time, iterations)
	startTimes := make([]time.Time, iterations)

	// Run the idle timer iteration times.
	for i := 0; i < iterations; i++ {
		wg.Add(1)
		startTimes[i] = time.Now()
		go func() {
			timer.Wait(ctx, func(context.Context) {
				ticks <- time.Now()
			})
			wg.Done()
		}()
		wg.Wait()
		time.Sleep(timerTick)
		// Calls to stop wait can be interleaved with the timer expiring.
		if i%10 == 0 {
			if err := timer.StopWait(ctx); err != nil {
				t.Fatal(err)
			}
		}
	}

	time.Sleep(time.Second)
	if err := timer.StopWait(ctx); err != nil {
		t.Fatal(err)
	}
	// Multiple calls to stop Wait are safe.
	if err := timer.StopWait(ctx); err != nil {
		t.Fatal(err)
	}
	close(ticks)
	if got, want := len(ticks), iterations; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	first := <-ticks
	last := first
	i := 1
	for tck := range ticks {
		tickTime := tck.Sub(startTimes[i])
		// Expect each tick to be approximately timerTick after
		// the timer was started, allow timerTick's slack.
		if tickTime == 0 || tickTime > timerTick*2 {
			t.Errorf("unexpected tick time: %v", tickTime)
		}
		iterTime := tck.Sub(last)
		// expect approximately 2 * timerTick times between iterations,
		// allow timerTick's slack.
		if iterTime == 0 || iterTime > timerTick*3 {
			t.Errorf("unexpected tick time: %v", iterTime)
		}
		last = tck
		i++
	}
}

func TestIdleReset(t *testing.T) {
	ctx := context.Background()

	timerTick := time.Millisecond * 10
	timer := netutil.NewIdleTimer(timerTick)

	var wg sync.WaitGroup
	var ticks = make(chan time.Time, 1)

	numResets := 500
	resetDelay := time.Millisecond
	go func() {
		for i := 0; i < numResets; i++ {
			time.Sleep(resetDelay)
			timer.Reset()
		}
	}()

	wg.Add(1)
	start := time.Now()
	go func() {
		timer.Wait(ctx, func(context.Context) {
			ticks <- time.Now()
		})
		wg.Done()
	}()
	wg.Wait()

	expireTime := <-ticks
	// The reset should have kept the timer going for at least numResets * resetDelay.
	if got, want := expireTime.Sub(start), time.Duration(numResets)*resetDelay; got < want {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestIdleStopWait(t *testing.T) {
	ctx := context.Background()
	timer := netutil.NewIdleTimer(time.Millisecond)

	readyCh := make(chan struct{})
	waitCh := make(chan time.Time)

	// Make sure StopWait works to stop the timer.
	mytime := time.Now()
	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		timer.Wait(ctx, func(context.Context) {
			<-readyCh
			waitCh <- mytime
			close(waitCh)
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

func TestIdleStopWaitCancel(*testing.T) {
	ctx := context.Background()
	timer := netutil.NewIdleTimer(time.Hour)
	var wg sync.WaitGroup
	wg.Add(1)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		timer.Wait(ctx, func(context.Context) {})
		wg.Done()
	}()

	// Canceling the context should be enough to stop the timer.
	cancel()
	wg.Wait()
}

func TestIdleStopWaitHang(t *testing.T) {
	ctx := context.Background()
	timer := netutil.NewIdleTimer(time.Millisecond)
	readyCh := make(chan struct{})
	go func() {
		timer.Wait(ctx, func(context.Context) {
			// The callback will hang.
			close(readyCh)
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Hour):
			}
		})
	}()
	<-readyCh
	nctx, cancel := context.WithTimeout(ctx, time.Millisecond*10)
	defer cancel()
	if err := timer.StopWait(nctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("unexpected or missing error: %v", err)
	}
}
