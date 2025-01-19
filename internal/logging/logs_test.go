// Copyright 2025 Cosmos Nicolaou. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package logging_test

import (
	"bytes"
	"io"
	"log/slog"
	"testing"
	"time"

	"cloudeng.io/datetime"
	"github.com/cosnicolaou/automation/internal/logging"
)

func TestLogs(t *testing.T) {
	out := &bytes.Buffer{}
	logger := slog.New(slog.NewJSONHandler(out, nil))

	now := time.Now()
	today := datetime.NewCalendarDate(2024, 1, 11)
	logging.WriteNewDay(logger, today, 3)
	id := logging.WritePending(logger, false, false,
		"device", "on", []string{"a"},
		"pre-test", []string{"b"},
		now, now.Add(time.Minute*13), time.Minute)
	logging.WriteCompletion(logger, id, nil, true,
		"device", "on",
		"pre-test", true,
		now, now.Add(time.Minute*13), now.Add(time.Minute*14), time.Minute)
	logging.WriteYearEnd(logger, 2024, time.Hour)
	logging.WriteCompletion(logger, id, io.EOF, true,
		"device", "on",
		"pre-test", true,
		now, now.Add(time.Minute*13), now.Add(time.Minute*14), time.Minute)

	var logs []logging.Entry
	sc := logging.NewScanner(out)
	for le := range sc.Entries() {
		logs = append(logs, le)
	}
	if sc.Err() != nil {
		t.Fatalf("error scanning logs: %v", sc.Err())
	}
	if got, want := len(logs), 5; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	testNewDay(t, logs[0], today)
	testPending(t, logs[1], now, now.Add(time.Minute*13), time.Minute)
	testCompletion(t, logs[2], now, now.Add(time.Minute*13), now.Add(time.Minute*14), time.Minute)
	testYearEnd(t, logs[3], 2024)

	if got, want := logs[4].Err.Error(), "EOF"; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := logs[4].Msg, logging.LogFailed; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

}

func testNewDay(t *testing.T, le logging.Entry, today datetime.CalendarDate) {
	if got, want := le.Msg, logging.LogNewDay; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := le.NumActions, 3; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := le.Date, today; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}

func testPending(t *testing.T, le logging.Entry, now, due time.Time, delay time.Duration) {
	if got, want := le.Msg, logging.LogPending; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := le.Now, now.Round(0); got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := le.Due, due.Round(0); got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := le.Delay, delay; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}

func testCompletion(t *testing.T, le logging.Entry, started, now, due time.Time, delay time.Duration) {
	if got, want := le.Msg, logging.LogCompleted; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := le.Now, now.Round(0); got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := le.Due, due.Round(0); got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := le.Started, started.Round(0); got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := le.Delay, delay; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}

func testYearEnd(t *testing.T, le logging.Entry, year int) {
	if got, want := le.Msg, logging.LogYearEnd; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := le.YearEnd, year; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := le.YearEndDelay, time.Hour; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}
