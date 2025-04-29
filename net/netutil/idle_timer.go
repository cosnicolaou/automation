// Copyright 2024 Cosmos Nicolaou. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package netutil

import (
	"context"
	"sync"
	"time"

	"cloudeng.io/logging/ctxlog"
)

// IdleTimer is a timer that expires after a period of inactivity.
type IdleTimer struct {
	mu        sync.Mutex
	ticker    *time.Ticker
	idleTime  time.Duration
	expired   bool
	stopCh    chan struct{}
	stoppedCh chan struct{}
}

// NewIdleTimer creates a new IdleTimer with the specified idle time,
// call Reset to restart the timer. The timer can reused by calling
// Wait again, typically in a goroutine. A negative duration will
// cause a panic.
func NewIdleTimer(d time.Duration) *IdleTimer {
	if d <= 0 {
		panic("idle time duration must be greater than 0")
	}
	return &IdleTimer{
		idleTime:  d,
		stopCh:    make(chan struct{}),
		stoppedCh: make(chan struct{}),
	}
}

// Reset resets the idle timer.
func (d *IdleTimer) Reset(_ context.Context) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.ticker != nil {
		d.ticker.Reset(d.idleTime)
	}
}

// Wait waits for the idle time to expire and then calls expired.
// It returns when the idle timer expires, StopWait is called or the
// context is canceled.
func (d *IdleTimer) Wait(ctx context.Context, expired func(context.Context)) {
	ctxlog.Info(ctx, "idle timer: waiting", "idleTime", d.idleTime.String())
	d.mu.Lock()
	d.expired = false
	d.ticker = time.NewTicker(d.idleTime)
	d.stopCh = make(chan struct{})
	d.stoppedCh = make(chan struct{})
	ch := d.stoppedCh
	d.mu.Unlock()
	defer close(ch)
	for {
		select {
		case <-d.ticker.C:
			expired(ctx)
			d.mu.Lock()
			d.expired = true
			d.ticker.Stop()
			d.stopCh = nil
			d.stoppedCh = nil
			d.mu.Unlock()
			return
		case <-ctx.Done():
			ctxlog.Info(ctx, "idle timer: context done")
			return
		case <-d.stopCh:
			return
		}
	}
}

// StopWait stops the idle timer watcher and waits for it to do so,
// or for the context to be canceled.
func (d *IdleTimer) StopWait(ctx context.Context) error {
	d.mu.Lock()
	if d.expired {
		d.mu.Unlock()
		return nil
	}
	close(d.stopCh)
	stoppedCh := d.stoppedCh
	d.mu.Unlock()
	select {
	case <-stoppedCh:
	case <-ctx.Done():
		return ctx.Err()
	}
	return nil
}
