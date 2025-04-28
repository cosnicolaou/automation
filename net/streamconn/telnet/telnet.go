// Copyright 2024 Cosmos Nicolaou. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package telnet

import (
	"context"
	"time"

	"cloudeng.io/logging/ctxlog"
	"github.com/cosnicolaou/automation/net/streamconn"
	"github.com/ziutek/telnet"
)

type telnetConn struct {
	conn    *telnet.Conn
	addr    string
	timeout time.Duration
}

func Dial(ctx context.Context, addr string, timeout time.Duration) (streamconn.Transport, error) {
	conn, err := telnet.Dial("tcp", addr)
	if err != nil {
		ctxlog.Error(ctx, "telnet: dial failed", "addr", addr, "err", err)
		return nil, err
	}
	ctxlog.Info(ctx, "telnet: dialed", "addr", addr)
	return &telnetConn{conn: conn, addr: addr, timeout: timeout}, nil
}

func (tc *telnetConn) send(ctx context.Context, buf []byte, sensitive bool) (int, error) {
	if err := tc.conn.SetWriteDeadline(time.Now().Add(tc.timeout)); err != nil {
		ctxlog.Error(ctx, "telnet: send failed to set read deadline", "addr", tc.addr, "err", err)
		return -1, err
	}
	n, err := tc.conn.Write(buf)
	if sensitive {
		ctxlog.Info(ctx, "telnet: sent", "addr", tc.addr, "text", "***", "err", err)
	} else {
		ctxlog.Info(ctx, "telnet: sent", "addr", tc.addr, "text", string(buf), "err", err)
	}
	return n, err
}

func (tc *telnetConn) Send(ctx context.Context, buf []byte) (int, error) {
	return tc.send(ctx, buf, false)
}

func (tc *telnetConn) SendSensitive(ctx context.Context, buf []byte) (int, error) {
	return tc.send(ctx, buf, true)
}

func (tc *telnetConn) ReadUntil(ctx context.Context, expected []string) ([]byte, error) {
	if err := tc.conn.SetReadDeadline(time.Now().Add(tc.timeout)); err != nil {
		ctxlog.Error(ctx, "telnet: readUntil failed to set read deadline", "addr", tc.addr, "err", err)
		return nil, err
	}
	buf, err := tc.conn.ReadUntil(expected...)
	if err != nil {
		ctxlog.Error(ctx, "telnet: readUntil failed", "addr", tc.addr, "text", expected, "err", err)
		return nil, err
	}
	ctxlog.Info(ctx, "telnet: readUntil", "addr", tc.addr, "text", expected, "response", string(buf))
	return buf, err
}

func (tc *telnetConn) Close(ctx context.Context) error {
	if err := tc.conn.Close(); err != nil {
		ctxlog.Error(ctx, "telnet: close failed", "addr", tc.addr, "err", err)
		return err
	}
	ctxlog.Info(ctx, "telnet: close", "addr", tc.addr)
	return nil
}
