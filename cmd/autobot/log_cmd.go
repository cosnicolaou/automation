// Copyright 2024 Cosmos Nicolaou. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"cloudeng.io/datetime"
	"github.com/cosnicolaou/automation/internal/logging"
)

type LogFlags struct {
	Device   string `subcmd:"device,,display log info for the specific device"`
	Schedule string `subcmd:"schedule,,display log info for the specific schedule"`
}

type LogStatusFlags struct {
	LogFlags
	StreamingSummary bool `subcmd:"streaming-summary,false,print a summary of the status of each log entry as it is completed"`
	FinalSummary     bool `subcmd:"final-summary,true,print a single summary of the entire log"`
	TSV              bool `subcmd:"tsv,false,print the status in tab separated values"`
}

type Log struct {
	out io.Writer
}

type logEntryHandler func(logging.Entry) error

func (l *Log) processLog(rd io.Reader, fv *LogStatusFlags, lh logEntryHandler) error {
	sc := logging.NewScanner(rd)
	for le := range sc.Entries(true) {
		if len(fv.Device) > 0 && le.Device != fv.Device {
			continue
		}
		if len(fv.Schedule) > 0 && le.Schedule != fv.Schedule {
			continue
		}
		if err := lh(le); err != nil {
			return err
		}
	}
	return sc.Err()
}

func (l *Log) Status(_ context.Context, flags any, args []string) error {
	fv := flags.(*LogStatusFlags)
	srh := statusRecoder{
		StatusRecorder: logging.NewStatusRecorder(),
		pending:        make(map[int64]*logging.StatusRecord),
		flags:          fv,
		out:            l.out,
	}
	rd := os.Stdin
	if len(args) == 1 {
		fi, err := os.OpenFile(args[0], os.O_RDONLY, 0)
		if err != nil {
			return err
		}
		defer fi.Close()
		rd = fi
	}
	err := l.processLog(rd, fv, srh.process)
	srh.print(l.out, datetime.CalendarDateFromTime(srh.last))
	return err
}

type statusRecoder struct {
	*logging.StatusRecorder
	pending map[int64]*logging.StatusRecord
	last    time.Time
	flags   *LogStatusFlags
	out     io.Writer
}

func (sr *statusRecoder) print(out io.Writer, when datetime.CalendarDate) {
	tm := tableManager{}
	if sr.flags.TSV {
		_, _ = out.Write([]byte(tm.CompletedAndPending(sr.StatusRecorder, when).RenderTSV()))

	} else {
		_, _ = out.Write([]byte(tm.CompletedAndPending(sr.StatusRecorder, when).Render()))
	}
	fmt.Fprintln(out)
}

func (sr *statusRecoder) process(le logging.Entry) error {
	if le.Mod != "scheduler" {
		return nil
	}
	switch le.Msg {
	case logging.LogPending:
		rec := le.StatusRecord()
		rec.Pending = le.Now
		rec = sr.NewPending(rec)
		sr.pending[le.ID] = rec
		return nil
	case logging.LogCompleted, logging.LogFailed:
		pending, ok := sr.pending[le.ID]
		if !ok {
			return nil
		}
		sr.PendingDone(pending, le.PreCondResult, le.Err)
		if sr.flags.StreamingSummary {
			sr.print(sr.out, datetime.CalendarDateFromTime(le.Due))
			sr.ResetCompleted()
		}
		sr.last = le.Due
	case logging.LogNewDay:
	case logging.LogYearEnd:
	case logging.LogTooLate:
		fmt.Fprintf(sr.out, "% 70v: too late: due at: %v, delay: %v\n", le.Name(), le.Due, le.Delay)
	default: // ignore all other messages.
		return nil
	}
	return nil
}
