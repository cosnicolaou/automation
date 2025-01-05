// Copyright 2024 Cosmos Nicolaou. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package netutil

import (
	"context"
	"sync"
	"time"
)

type IdleReset interface {
	Reset()
}

// Managed is the interface used by Manager[T] to manage a connection.
type Managed[T any] interface {
	// Connect is called when a new connection is required.
	Connect(context.Context, IdleReset) (T, error)

	// Disconnect is called when the idle timer has expired.
	Disconnect(context.Context, T) error
}

// IdleManagerManager manages an instance of Managed using the supplied idle timer.
// Connect is called whenever a new managed instance is required and Disconnect
// when the idle time is reached.
type IdleManager[T any, F Managed[T]] struct {
	idle      *IdleTimer
	connector Managed[T]

	mu        sync.Mutex
	connected bool
	conn      T
}

func NewIdleManager[T any, F Managed[T]](ctx context.Context, managed F, idle *IdleTimer) *IdleManager[T, F] {
	m := &IdleManager[T, F]{
		connector: managed,
		idle:      idle,
	}
	return m
}

// Connection returns the current connection, or creates a new one if the idle
// timer has expired.
func (m *IdleManager[T, F]) Connection(ctx context.Context) (T, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.connected {
		return m.conn, nil
	}
	conn, err := m.connector.Connect(ctx, m.idle)
	if err != nil {
		var empty T
		return empty, err
	}
	m.conn = conn
	m.connected = true
	go m.idle.Wait(ctx, m.expired)
	return conn, nil
}

func (m *IdleManager[T, F]) closeUnderlyingUnlocked(ctx context.Context) error {
	if m.connected {
		var empty T
		conn := m.conn
		m.conn = empty
		m.connected = false
		return m.connector.Disconnect(ctx, conn)
	}
	return nil
}

func (m *IdleManager[T, F]) expired(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.closeUnderlyingUnlocked(ctx)
}

// Stop closes the connection and stops the idle timer.
func (m *IdleManager[T, F]) Stop(ctx context.Context, timeout time.Duration) error {
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

// OnDemandConnection wraps an IdleManager to reuse or recreate a connection
// as required.
type OnDemandConnection[T any, F Managed[T]] struct {
	mu              sync.Mutex
	managed         F
	idleManager     *IdleManager[T, F]
	keepAlive       time.Duration
	newErrorSession func(error) T
}

func NewOnDemandConnection[T any, F Managed[T]](managed F, newErrorSession func(error) T) *OnDemandConnection[T, F] {
	return &OnDemandConnection[T, F]{
		managed:         managed,
		newErrorSession: newErrorSession,
	}
}

func (sm *OnDemandConnection[T, F]) SetKeepAlive(keepAlive time.Duration) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.keepAlive = keepAlive
}

func (sm *OnDemandConnection[T, F]) Connection(ctx context.Context) T {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if sm.idleManager == nil {
		sm.idleManager = NewIdleManager(ctx, sm.managed, NewIdleTimer(sm.keepAlive))
	}
	sess, err := sm.idleManager.Connection(ctx)
	if err != nil {
		return sm.newErrorSession(err)
	}
	return sess
}

func (sm *OnDemandConnection[T, F]) Close(ctx context.Context) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if sm.idleManager == nil {
		return nil
	}
	return sm.idleManager.Stop(ctx, time.Minute)
}
