// Copyright 2024 Cosmos Nicolaou. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package streamconn

import (
	"context"
	"sync"
	"time"

	"github.com/cosnicolaou/automation/net/netutil"
)

// Manager manages a session with an idle timer.
type Manager struct {
	mu      sync.Mutex
	idle    *netutil.IdleTimer
	session Session
}

func NewManager() *Manager {
	return &Manager{}
}

// Session returns the current session, or nil if it the idle
// timer has expired.
func (m *Manager) Session() Session {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.session
}

// ManageSession sets the session and idle timer to be managed and
// calls the idle timer's Wait method in a goroutine.
func (m *Manager) ManageSession(ctx context.Context, sess Session, idle *netutil.IdleTimer) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.session = sess
	m.idle = idle
	go m.idle.Wait(ctx, m.expired)
}

func (m *Manager) closeUnderlyingUnlocked(ctx context.Context) error {
	if m.session == nil {
		return nil
	}
	err := m.session.Close(ctx)
	m.session = nil
	m.idle = nil
	return err
}

func (m *Manager) expired(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.closeUnderlyingUnlocked(ctx)
}

// Stop closes the session and stops the idle timer.
func (m *Manager) Stop(ctx context.Context, timeout time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	err := m.closeUnderlyingUnlocked(ctx)
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if serr := m.idle.StopWait(ctx); serr != nil && err == nil {
		return serr
	}
	m.idle = nil
	m.session = nil
	return err
}
