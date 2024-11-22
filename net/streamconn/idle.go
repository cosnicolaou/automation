// Copyright 2024 Cosmos Nicolaou. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package streamconn

import (
	"context"
	"sync"
	"time"
)

type IdleTimer struct {
	mu        sync.Mutex
	idleTime  time.Duration
	elapsed   time.Duration
	last      time.Time
	stopCh    chan struct{}
	stoppedCh chan struct{}
}

// NewIdleTimer creates a new IdleTimer with the specified idle time.
func NewIdleTimer(d time.Duration) *IdleTimer {
	return &IdleTimer{
		idleTime:  d,
		last:      time.Now(),
		stopCh:    make(chan struct{}),
		stoppedCh: make(chan struct{}),
	}
}

// Reset resets the idle timer.
func (d *IdleTimer) Reset() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.elapsed = 0
	d.last = time.Now()
}

// Expired returns true if the idle timer has expired.
func (d *IdleTimer) Expired() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return time.Now().After(d.last.Add(d.idleTime))
}

// Remaining returns the remaining time before the idle timer expires.
func (d *IdleTimer) Remaining() time.Duration {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.last.Add(d.idleTime).Sub(time.Now())
}

// Wait waits for the idle to expire, for the channel to be closed or the
// context to be canceled. The close function is called when the idle timer
// expires or the context canceled, but not when the channel is closed.
func (d *IdleTimer) Wait(ctx context.Context, expired func(context.Context) error) {
	defer close(d.stoppedCh)
	for {
		select {
		case <-time.After(d.Remaining()):
			if d.Expired() {
				expired(ctx)
			}
		case <-ctx.Done():
			return
		case <-d.stopCh:
			return
		}
	}
}

// StopWait stops the idle timer watcher and waits for it to do so,
// or for the context to be canceled.
func (d *IdleTimer) StopWait(ctx context.Context) error {
	close(d.stopCh)
	select {
	case <-d.stoppedCh:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
