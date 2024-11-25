// Copyright 2024 Cosmos Nicolaou. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package netutil_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/cosnicolaou/automation/net/netutil"
)

type session struct {
	mu  sync.Mutex
	msg string
}

type sessionMgr struct {
	eventCh chan string
	timeCh  chan time.Time
}

func (sm *sessionMgr) Disconnect(_ context.Context, s *session) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.msg = "disconnected"
	sm.eventCh <- "disconnect"
	if sm.timeCh != nil {
		sm.timeCh <- time.Now()
	}
	return nil
}

func (sm *sessionMgr) Connect(ctx context.Context, reset netutil.IdleReset) (*session, error) {
	sm.eventCh <- "connect"
	return &session{}, nil
}
func (sm *sessionMgr) Nil() *session {
	return nil
}

func TestIdleManager(t *testing.T) {
	ctx := context.Background()

	idle := netutil.NewIdleTimer(10 * time.Millisecond)

	eventCh := make(chan string, 1)
	sm := &sessionMgr{eventCh: eventCh}

	mc := netutil.NewIdleManager(ctx, sm, idle)
	_, err := mc.Connection(ctx)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if got, want := <-eventCh, "connect"; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := <-eventCh, "disconnect"; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestIdleManagerReset(t *testing.T) {
	ctx := context.Background()

	timerTick := time.Millisecond * 10
	idle := netutil.NewIdleTimer(timerTick)

	eventCh := make(chan string, 1000)
	timeCh := make(chan time.Time, 1000)
	sm := &sessionMgr{eventCh: eventCh, timeCh: timeCh}

	mc := netutil.NewIdleManager(ctx, sm, idle)

	numResets := 500
	resetDelay := time.Millisecond
	go func() {
		for i := 0; i < numResets; i++ {
			time.Sleep(resetDelay)
			idle.Reset()
			mc.Connection(ctx)
		}
	}()
	start := time.Now()
	_, err := mc.Connection(ctx)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	time.Sleep(time.Second)
	if got, want := len(eventCh), 2; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := <-eventCh, "connect"; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := <-eventCh, "disconnect"; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	select {
	case <-eventCh:
		t.Errorf("expected no more events")
	default:
	}
	stopped := <-timeCh
	if stopped.Sub(start) < time.Duration(numResets)*resetDelay {
		t.Errorf("expected at least %v, got %v", time.Duration(numResets)*resetDelay, stopped.Sub(start))
	}
}
