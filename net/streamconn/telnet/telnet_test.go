// Copyright 2024 Cosmos Nicolaou. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package telnet_test

import (
	"bytes"
	"context"
	"log/slog"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/cosnicolaou/automation/net/streamconn"
	"github.com/cosnicolaou/automation/net/streamconn/telnet"
	telnetserver "github.com/reiver/go-telnet"
)

func runServer(t *testing.T, handler telnetserver.Handler, wg *sync.WaitGroup) net.Listener {
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatal(err)
	}
	server := &telnetserver.Server{
		Handler: handler,
	}
	go func() {
		server.Serve(listener)
		wg.Done()
	}()
	return listener
}

func TestClient(t *testing.T) {
	ctx := context.Background()
	var wg sync.WaitGroup
	wg.Add(1)
	server := runServer(t, telnetserver.EchoHandler, &wg)
	defer func() {
		server.Close()
		wg.Wait()
	}()

	logRecorder := bytes.NewBuffer(nil)
	logger := slog.New(slog.NewJSONHandler(logRecorder, nil))
	addr := server.Addr().String()

	transport, err := telnet.Dial(ctx, addr, time.Minute, logger)
	if err != nil {
		t.Fatal(err)
	}

	idle := streamconn.NewIdleTimer(10 * time.Minute)
	s := streamconn.NewSession(transport, idle)
	s.Send(ctx, []byte("hello\r\n"))
	s.Send(ctx, []byte("world\r\n"))
	read := s.ReadUntil(ctx, "world\r\n")
	if err := s.Err(); err != nil {
		t.Fatal(err)
	}

	if got, want := string(read), "hello\r\nworld\r\n"; got != want {
		t.Fatalf("got %#v, want %#v", got, want)
	}

	s.Send(ctx, []byte("and\r\n"))
	s.Send(ctx, []byte("again\r\n"))
	read = s.ReadUntil(ctx, "again\r\n")
	if err := s.Err(); err != nil {
		t.Fatal(err)
	}

	if got, want := string(read), "and\r\nagain\r\n"; got != want {
		t.Fatalf("got %#v, want %#v", got, want)
	}

}
