// Copyright 2024 Cosmos Nicolaou. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package streamconn

import (
	"context"
	"sync"

	"github.com/cosnicolaou/automation/net/netutil"
)

// Transport is the interface for a transport layer.
type Transport interface {
	Send(ctx context.Context, buf []byte) (int, error)
	// SendSensitive avoids logging the contents of the buffer, use
	// it for login exchanges, credentials etc.
	SendSensitive(ctx context.Context, buf []byte) (int, error)
	ReadUntil(ctx context.Context, expected []string) ([]byte, error)
	Close(ctx context.Context) error
}

// Redesign this to support exclusivity.
type Session interface {
	Send(ctx context.Context, buf []byte)
	// SendSensitive avoids logging the contents of the buffer, use
	// it for login exchanges, credentials etc.
	SendSensitive(ctx context.Context, buf []byte)
	ReadUntil(ctx context.Context, expected ...string) []byte
	Close(ctx context.Context) error
	Err() error
}

type session struct {
	session_lock sync.Mutex
	mu           sync.Mutex
	err          error
	conn         Transport
	idle         netutil.IdleReset
}

func NewSession(t Transport, idle netutil.IdleReset) Session {
	return &session{conn: t, idle: idle}
}

func (s *session) Reserve() {
	s.session_lock.Lock()
}

func (s *session) Release() {
	s.session_lock.Unlock()
}

func (s *session) Err() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.err
}

func (s *session) Send(ctx context.Context, buf []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return
	}
	s.idle.Reset(ctx)
	_, s.err = s.conn.Send(ctx, buf)
}

func (s *session) SendSensitive(ctx context.Context, buf []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return
	}
	s.idle.Reset(ctx)
	_, s.err = s.conn.SendSensitive(ctx, buf)
}

func (s *session) ReadUntil(ctx context.Context, expected ...string) []byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return nil
	}
	s.idle.Reset(ctx)
	out, err := s.conn.ReadUntil(ctx, expected)
	if err != nil {
		s.err = err
		return nil
	}
	return out
}

func (s *session) Close(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return s.err
	}
	return s.conn.Close(ctx)
}

type errorSession struct {
	err error
}

// NewErrorSession returns a session that always returns the given error.
func NewErrorSession(err error) Session {
	return &errorSession{err: err}
}

func (s *errorSession) Err() error {
	return s.err
}

func (s *errorSession) Send(context.Context, []byte) {
}

func (s *errorSession) SendSensitive(context.Context, []byte) {
}

func (s *errorSession) ReadUntil(context.Context, ...string) []byte {
	return nil
}

func (s *errorSession) Close(context.Context) error {
	return s.err
}
