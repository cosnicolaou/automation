// Copyright 2024 Cosmos Nicolaou. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package telnet

import (
	"context"
	"log/slog"
	"time"

	"github.com/cosnicolaou/automation/devices"
	"github.com/cosnicolaou/automation/net/streamconn"
	"github.com/ziutek/telnet"
)

type telnetConn struct {
	conn    *telnet.Conn
	timeout time.Duration
}

func Dial(ctx context.Context, addr string, timeout time.Duration) (streamconn.Transport, error) {
	logger := devices.LoggerFromContext(ctx)
	logger.Log(ctx, slog.LevelInfo, "dialing telnet", "addr", addr)
	conn, err := telnet.Dial("tcp", addr)
	if err != nil {
		logger.Log(ctx, slog.LevelWarn, "dial failed", "addr", addr, "err", err)
		return nil, err
	}
	logger = logger.With("protocol", "telnet", "addr", conn.RemoteAddr().String())
	return &telnetConn{conn: conn, timeout: timeout}, nil
}

func (tc *telnetConn) send(ctx context.Context, buf []byte, sensitive bool) (int, error) {
	if err := tc.conn.SetWriteDeadline(time.Now().Add(tc.timeout)); err != nil {
		devices.LoggerFromContext(ctx).Log(ctx, slog.LevelWarn, "send failed to set read deadline", "err", err)
		return -1, err
	}
	n, err := tc.conn.Write(buf)
	if sensitive {
		devices.LoggerFromContext(ctx).Log(ctx, slog.LevelInfo, "sent", "text", "***", "err", err)
	} else {
		devices.LoggerFromContext(ctx).Log(ctx, slog.LevelInfo, "sent", "text", string(buf), "err", err)
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
		devices.LoggerFromContext(ctx).Log(ctx, slog.LevelWarn, "readUntil failed to set read deadline", "err", err)
		return nil, err
	}
	buf, err := tc.conn.ReadUntil(expected...)
	if err != nil {
		devices.LoggerFromContext(ctx).Log(ctx, slog.LevelWarn, "readUntil failed", "text", expected, "err", err)
		return nil, err
	}
	devices.LoggerFromContext(ctx).Log(ctx, slog.LevelInfo, "readUntil", "text", expected, "response", string(buf))
	return buf, err
}

func (tc *telnetConn) Close(ctx context.Context) error {
	if err := tc.conn.Close(); err != nil {
		devices.LoggerFromContext(ctx).Log(ctx, slog.LevelWarn, "close failed", "err", err)
	}
	devices.LoggerFromContext(ctx).Log(ctx, slog.LevelInfo, "close")
	return nil
}
