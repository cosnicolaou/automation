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

	// Nil returns a nil value of the connection type and is used to allow
	// the connection to be reset to a nil value when the idle timer expires.
	Nil() T
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
		return m.connector.Nil(), err
	}
	m.conn = conn
	m.connected = true
	go m.idle.Wait(ctx, m.expired)
	return conn, nil
}

func (m *IdleManager[T, F]) closeUnderlyingUnlocked(ctx context.Context) error {
	if m.connected {
		conn := m.conn
		m.conn = m.connector.Nil()
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
