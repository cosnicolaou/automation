// Copyright 2024 Cosmos Nicolaou. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package tls

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"slices"
	"time"

	"github.com/cosnicolaou/automation/net/streamconn"
)

type tlsConn struct {
	conn    *tls.Conn
	rd      *bufio.Reader
	timeout time.Duration
	logger  *slog.Logger
}

func Dial(ctx context.Context, addr string, version string, timeout time.Duration, logger *slog.Logger) (streamconn.Transport, error) {
	ids := []uint16{}
	for _, cs := range tls.CipherSuites() {
		ids = append(ids, cs.ID)
	}
	for _, cs := range tls.InsecureCipherSuites() {
		ids = append(ids, cs.ID)
	}
	cfg := tls.Config{
		InsecureSkipVerify: true, //nolint:gosec
		CipherSuites:       ids,
	}
	switch version {
	case "1.0":
		cfg.MinVersion = tls.VersionTLS10
		cfg.MaxVersion = tls.VersionTLS10
	case "1.2":
		cfg.MinVersion = tls.VersionTLS12
		cfg.MaxVersion = tls.VersionTLS12
	default:
		return nil, fmt.Errorf("unsupported tls version: %v", version)
	}
	logger.Log(ctx, slog.LevelInfo, "dialing tls", "addr", addr, "version", version)
	conn, err := tls.Dial("tcp", addr, &cfg)
	if err != nil {
		logger.Log(ctx, slog.LevelWarn, "dial tls failed", "addr", addr, "err", err)
		return nil, err
	}
	logger = logger.With("protocol", "tls", "addr", conn.RemoteAddr().String())
	rd := bufio.NewReader(conn)
	return &tlsConn{conn: conn, rd: rd, timeout: timeout, logger: logger}, nil
}

func (tc *tlsConn) send(ctx context.Context, buf []byte, sensitive bool) (int, error) {
	if err := tc.conn.SetWriteDeadline(time.Now().Add(tc.timeout)); err != nil {
		tc.logger.Log(ctx, slog.LevelWarn, "send failed to set read deadline", "err", err)
		return -1, err
	}
	n, err := tc.conn.Write(buf)
	if sensitive {
		tc.logger.Log(ctx, slog.LevelInfo, "sent", "text", "***", "err", err)
	} else {
		tc.logger.Log(ctx, slog.LevelInfo, "sent", "text", string(buf), "err", err)
	}
	return n, err
}

func (tc *tlsConn) Send(ctx context.Context, buf []byte) (int, error) {
	return tc.send(ctx, buf, false)
}

func (tc *tlsConn) SendSensitive(ctx context.Context, buf []byte) (int, error) {
	return tc.send(ctx, buf, true)
}

func (tc *tlsConn) readUntil(ctx context.Context, expected []string) ([]byte, error) {
	for _, e := range expected {
		if len(e) == 0 {
			return nil, nil
		}
	}
	exp := slices.Clone(expected)
	buf := make([]byte, 0, 1024)
	for {
		select {
		case <-ctx.Done():
			return buf, ctx.Err()
		default:
		}
		nb, err := tc.rd.ReadByte()
		if err != nil {
			return buf, err
		}
		buf = append(buf, nb)
		for i, e := range exp {
			if e[0] == nb {
				if len(e) == 1 {
					return buf, nil
				}
				exp[i] = e[1:]
				continue
			}
			exp[i] = expected[i]
		}

	}
}

func (tc *tlsConn) ReadUntil(ctx context.Context, expected []string) ([]byte, error) {
	if err := tc.conn.SetReadDeadline(time.Now().Add(tc.timeout)); err != nil {
		tc.logger.Log(ctx, slog.LevelWarn, "readUntil failed to set read deadline", "err", err)
		return nil, err
	}
	buf, err := tc.readUntil(ctx, expected)
	if err != nil {
		tc.logger.Log(ctx, slog.LevelWarn, "readUntil failed", "text", expected, "err", err)
		return nil, err
	}
	tc.logger.Log(ctx, slog.LevelInfo, "readUntil", "text", expected)
	return buf, err
}

func (tc *tlsConn) Close(ctx context.Context) error {
	if err := tc.conn.Close(); err != nil {
		tc.logger.Log(ctx, slog.LevelWarn, "close failed", "err", err)
	}
	tc.logger.Log(ctx, slog.LevelInfo, "close")
	return nil
}
