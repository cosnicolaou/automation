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

func (sm *sessionMgr) Connect(context.Context, netutil.IdleReset) (*session, error) {
	sm.eventCh <- "connect"
	return &session{}, nil
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

func TestIdleManager(t *testing.T) {
	ctx := context.Background()

	idle := netutil.NewIdleTimer(10 * time.Millisecond)

	eventCh := make(chan string, 1)
	sm := &sessionMgr{eventCh: eventCh}

	mc := netutil.NewIdleManager(sm, idle)
	_, _, err := mc.Connection(ctx)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if got, want := <-eventCh, "connect"; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := <-eventCh, "disconnect"; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	if err := mc.Stop(ctx, time.Second); err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestIdleManagerReset(t *testing.T) {
	ctx := context.Background()

	timerTick := time.Millisecond * 15
	idle := netutil.NewIdleTimer(timerTick)

	eventCh := make(chan string, 1000)
	timeCh := make(chan time.Time, 1000)
	sm := &sessionMgr{eventCh: eventCh, timeCh: timeCh}

	mc := netutil.NewIdleManager(sm, idle)
	start := time.Now()

	numResets := 500
	resetDelay := time.Millisecond
	go func() {
		idle.Reset(ctx)
		for i := 0; i < numResets; i++ {
			time.Sleep(resetDelay)
			idle.Reset(ctx)
			if _, _, err := mc.Connection(ctx); err != nil {
				panic(err)
			}
		}
	}()

	_, _, err := mc.Connection(ctx)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	time.Sleep(time.Second)
	if got := len(eventCh); got < 2 || got > 10 {
		for e := range eventCh {
			t.Logf("event: %v", e)
		}
		t.Fatalf("got %v, want 2..10", got)
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

func TestOnDemand(t *testing.T) {
	ctx := context.Background()
	eventCh := make(chan string, 1000)
	timeCh := make(chan time.Time, 1000)

	sm := &sessionMgr{eventCh: eventCh, timeCh: timeCh}
	odm := netutil.NewOnDemandConnection(sm)
	odm.SetKeepAlive(time.Millisecond)
	s, _, err := odm.Connection(ctx)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	time.Sleep(5 * time.Millisecond)
	if got, want := <-eventCh, "connect"; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := <-eventCh, "disconnect"; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := s.msg, "disconnected"; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	eventCh = make(chan string, 1000)
	timeCh = make(chan time.Time, 1000)

	sm = &sessionMgr{eventCh: eventCh, timeCh: timeCh}
	odm = netutil.NewOnDemandConnection(sm)
	odm.SetKeepAlive(time.Minute * 10)
	s1, _, _ := odm.Connection(ctx)
	time.Sleep(5 * time.Millisecond)
	s2, _, _ := odm.Connection(ctx)
	if got, want := s1, s2; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := <-eventCh, "connect"; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := len(eventCh), 0; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	odm.Close(ctx)
	if got, want := <-eventCh, "disconnect"; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}
