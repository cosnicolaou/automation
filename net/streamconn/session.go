// Copyright 2024 Cosmos Nicolaou. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package streamconn

import (
	"context"
	"sync"
	"sync/atomic"

	"cloudeng.io/logging/ctxlog"
	"github.com/cosnicolaou/automation/net/netutil"
)

// Transport is the interface for a transport layer connection.
type Transport interface {
	Send(ctx context.Context, buf []byte) (int, error)
	// SendSensitive avoids logging the contents of the buffer, use
	// it for login exchanges, credentials etc.
	SendSensitive(ctx context.Context, buf []byte) (int, error)
	ReadUntil(ctx context.Context, expected []string) ([]byte, error)
	Close(ctx context.Context) error
}

// SessionManager is a manager for creating and releasing sessions
// and ensures that only one session is active at a time.
// Session.Release() must be called to release a session
// and allow the manager to create a new session.
type SessionManager struct {
	mu sync.Mutex
}

var sessionID int64

func (sm *SessionManager) New(t Transport, idle netutil.IdleReset) *Session {
	sm.mu.Lock()
	return &Session{
		conn: t,
		idle: idle,
		id:   atomic.AddInt64(&sessionID, 1),
		mgr:  sm,
	}
}

func (sm *SessionManager) NewWithContext(ctx context.Context, t Transport, idle netutil.IdleReset) (context.Context, *Session) {
	sess := sm.New(t, idle)
	ctx = ctxlog.WithAttributes(ctx, "session", sess.ID())
	return ctx, sess
}

// Release releases the session and allows the manager to create a new session.
// It must be called after the session is no longer needed.

func (sm *SessionManager) release() {
	sm.mu.Unlock()
}

// Session represents exclusive access to a transport layer connection.
// It's usage is for Send/SendSensitive to be called one or more
// times, without the need to check for errors, and then for ReadUntil
// to be called, which will return the error if any occurred during
// the Send/SendSensitive calls or the ReadUntil call.
type Session struct {
	mu   sync.Mutex
	id   int64
	err  error
	conn Transport
	idle netutil.IdleReset
	mgr  *SessionManager
}

// Release releases the session and allows the manager to create a new session.
func (s *Session) Release() {
	s.mgr.release()
}

func (s *Session) ID() int64 {
	return s.id
}

// Err returns the error if any occurred during the session.
func (s *Session) Err() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.err
}

// Send sends a buffer to the transport layer connection.
func (s *Session) Send(ctx context.Context, buf []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return
	}
	s.idle.Reset(ctx)
	_, s.err = s.conn.Send(ctx, buf)
}

// SendSensitive sends a buffer to the transport layer connection
// without logging the contents of the buffer, ie. calls SendSensitive.
func (s *Session) SendSensitive(ctx context.Context, buf []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return
	}
	s.idle.Reset(ctx)
	_, s.err = s.conn.SendSensitive(ctx, buf)
}

// ReadUntil reads from the transport layer connection until one of the
// expected strings is found. It returns the data read and an error if
// any. On error it returns an empty byte slice (not nil) and the error.
func (s *Session) ReadUntil(ctx context.Context, expected ...string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return []byte{}, s.err
	}
	s.idle.Reset(ctx)
	out, err := s.conn.ReadUntil(ctx, expected)
	if err != nil {
		s.err = err
		return []byte{}, err
	}
	return out, nil
}
