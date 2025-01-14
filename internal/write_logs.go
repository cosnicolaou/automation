// Copyright 2024 Cosmos Nicolaou. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package internal

import (
	"log/slog"
	"sync/atomic"
	"time"

	"cloudeng.io/datetime"
)

var invocationID int64

// WritePendingLog logs a pending operation and must be called for every new
// action returned by the scheduler for any given day. It returns a unique
// identifier for the operation that must be passed to LogCompletion except
// for overdue operations which are not logged as being completed.
func WritePendingLog(l *slog.Logger, overdue, dryRun bool, device, op string, args []string, precondition string, preArgs []string, now, dueAt time.Time, delay time.Duration) int64 {
	id := atomic.AddInt64(&invocationID, 1)
	msg := LogPending
	if overdue {
		msg = LogTooLate
	}
	l.Info(msg,
		"dry-run", dryRun,
		"id", id,
		"device", device,
		"op", op,
		"args", args,
		"pre", precondition,
		"pre-args", preArgs,
		"loc", dueAt.Location().String(),
		"now", now,
		"due", dueAt,
		"delay", delay)
	return id
}

// WriteCompletionLog logs the completion of all executed operations and must be called for
// every operation non-overdue that was logged as pending. The id must be the value
// returned by LogPending.
func WriteCompletionLog(l *slog.Logger, id int64, err error,
	dryRun bool, device, op, precondition string, preconditionResult bool, started, now, dueAt time.Time, delay time.Duration) {
	msg := LogCompleted
	if err != nil {
		msg = LogFailed
	}
	l.Info(msg,
		"dry-run", dryRun,
		"id", id,
		"device", device,
		"op", op,
		"pre", precondition,
		"pre-result", preconditionResult,
		"started", started,
		"loc", dueAt.Location().String(),
		"now", now,
		"due", dueAt,
		"delay", delay,
		"err", err)
}

const (
	LogPending   = "pending"
	LogCompleted = "completed"
	LogFailed    = "failed"
	LogTooLate   = "too-late"
	LogYearEnd   = "year-end"
	LogNewDay    = "day"
)

// WriteYearEndLog logs the completion of the year-end processing, that is,
// when all scheduled events for the year have been executed and the
// scheduler simply has to wait for the next year to start.
func WriteYearEndLog(l *slog.Logger, year int, delay time.Duration) {
	l.Info(LogYearEnd, "year", year, "year-end-delay", delay)
}

func WriteNewDayLog(l *slog.Logger, date datetime.CalendarDate, nActions int) {
	l.Info(LogNewDay, "date", date.String(), "#actions", nActions)
}
