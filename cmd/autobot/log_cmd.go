// Copyright 2024 Cosmos Nicolaou. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/cosnicolaou/automation/internal/logging"
)

type LogFlags struct {
	Device   string `subcmd:"device,,display log info for the specific device"`
	Schedule string `subcmd:"schedule,,display log info for the specific schedule"`
}

type LogStatusFlags struct {
	LogFlags
	StreamingSummary bool `subcmd:"streaming-summary,true,print a summary of the status of each log entry as it is completed"`
	DailySummary     bool `subcmd:"daily-summary,true,print a summary of the status at the end of each day"`
	FinalSummary     bool `subcmd:"final-summary,false,print a single summary of the entire log"`
}

type Log struct {
	out io.Writer
}

type logEntryHandler func(logging.Entry) error

func (l *Log) processLog(rd io.Reader, fv *LogStatusFlags, lh logEntryHandler) error {
	sc := logging.NewScanner(rd)
	for le := range sc.Entries() {
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
	if err := l.processLog(rd, fv, srh.process); err != nil {
		return err
	}
	srh.print(l.out)
	return nil
}

type statusRecoder struct {
	*logging.StatusRecorder
	pending map[int64]*logging.StatusRecord
	flags   *LogStatusFlags
	out     io.Writer
}

func (sr *statusRecoder) print(out io.Writer) {
	/*
		banner := false
		for rec := range sr.Completed() {
			if !banner {
				fmt.Fprint(out, "Completed:\n")
				banner = true
			}
			var o strings.Builder
			fmt.Fprintf(&o, "% 70v: completed: %v, pending since: %v, due at: %v, delay: %v", rec.Name(), rec.Completed, rec.Pending.Truncate(time.Minute), rec.Due, rec.Delay)
			if rec.PreCondition != "" {
				pa := strings.Join(rec.PreConditionArgs, " ")
				if rec.Aborted() {
					o.WriteString(fmt.Sprintf(" (aborted due to %v %v)", rec.PreCondition, pa))
				} else {
					o.WriteString(fmt.Sprintf(" (completed after %v %v)", rec.PreCondition, pa))
				}
			}
			o.WriteRune('\n')
			_, _ = out.Write([]byte(o.String()))
		}
		banner = false
		for rec := range sr.Pending() {
			if !banner {
				fmt.Fprint(out, "Pending:\n")
				banner = true
			}
			fmt.Fprintf(out, "% 70v: pending: due: %v, in %v\n", rec.Name(), rec.Due, rec.Delay.Round(time.Second))
		}*/

	tm := tableManager{}
	_, _ = out.Write([]byte(tm.Completed(sr.StatusRecorder).Render()))
	_, _ = out.Write([]byte("\n"))
	_, _ = out.Write([]byte(tm.Pending(sr.StatusRecorder).Render()))
	return
}

func (sr *statusRecoder) process(le logging.Entry) error {
	if le.Mod != "scheduler" {
		return nil
	}
	printSummary := sr.flags.StreamingSummary
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
	case logging.LogNewDay, logging.LogYearEnd:
		if sr.flags.DailySummary {
			printSummary = true
		}
	case logging.LogTooLate:
		fmt.Fprintf(sr.out, "% 70v: too late: due at: %v, delay: %v", le.Name(), le.Due, le.Delay)
	default: // ignore all other messages.
		return nil
	}
	if printSummary && !sr.flags.FinalSummary {
		sr.print(sr.out)
		sr.ResetCompleted()
	}
	return nil
}
