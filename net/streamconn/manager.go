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

type Manager struct {
	mu      sync.Mutex
	idle    *netutil.IdleTimer
	session Session
}

func NewManager() *Manager {
	return &Manager{}
}

func (m *Manager) Session() Session {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.session
}

func (m *Manager) ManageSession(sess Session, idle *netutil.IdleTimer) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.session = sess
	m.idle = idle
}

func (m *Manager) Watch(ctx context.Context) {
	go m.idle.Wait(ctx, m.Expired)
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

func (m *Manager) Expired(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.closeUnderlyingUnlocked(ctx)
}

func (m *Manager) Close(ctx context.Context, timeout time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	err := m.closeUnderlyingUnlocked(ctx)
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if serr := m.idle.StopWait(ctx); serr != nil && err == nil {
		return serr
	}
	return err
}
