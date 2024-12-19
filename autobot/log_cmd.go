// Copyright 2024 Cosmos Nicolaou. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/cosnicolaou/automation/internal"
)

type LogFlags struct {
	Device   string `subcmd:"device,,display log info for the specific device"`
	Schedule string `subcmd:"schedule,,display log info for the specific schedule"`
}

type LogStatusFlags struct {
	LogFlags
	StreamingSummary bool `subcmd:"streaming-summary,true,print a summary of the status of each log entry as it is completed"`
}

type Log struct{}

type logEntry struct {
	Msg          string   `json:"msg"`
	Mod          string   `json:"mod"`
	DryRun       bool     `json:"dry-run"`
	Schedule     string   `json:"sched"`
	Device       string   `json:"device"`
	ID           int64    `json:"id"`
	Op           string   `json:"op"`
	Args         []string `json:"args"`
	PreCond      string   `json:"pre"`
	PreCondAbort bool     `json:"pre-abort"`

	NowStr   string `json:"now"`
	DueStr   string `json:"due"`
	DelayStr string `json:"delay"`
	ErrStr   string `json:"err"`
	Now      time.Time
	Due      time.Time
	Delay    time.Duration
	Err      error
}

func parseLogLine(line string) (logEntry, error) {
	var le logEntry
	if err := json.Unmarshal([]byte(line), &le); err != nil {
		return le, err
	}
	var err error
	if len(le.DelayStr) != 0 {
		le.Delay, err = time.ParseDuration(le.DelayStr)
		if err != nil {
			fmt.Printf("failed to parse duration: %v: %v: %v\n", le.DelayStr, err, line)
		}
	}
	if len(le.NowStr) != 0 {
		le.Now, err = time.Parse(time.RFC3339, le.NowStr)
		if err != nil {
			fmt.Printf("failed to parse time: %v: %v: %v\n", le.NowStr, err, line)
		}
	}
	if len(le.DueStr) != 0 {
		le.Due, err = time.Parse(time.RFC3339, le.DueStr)
		if err != nil {
			fmt.Printf("failed to parse time: %v: %v: %v\n", le.DueStr, err, line)
		}
	}
	if le.ErrStr != "" {
		le.Err = errors.New(le.ErrStr)
	}
	return le, nil
}

func (le logEntry) statusRecord() *internal.StatusRecord {
	sr := &internal.StatusRecord{
		ID:                 le.ID,
		Schedule:           le.Schedule,
		Device:             le.Device,
		Op:                 le.Op,
		PreCondition:       le.PreCond,
		PreConditionResult: le.PreCondAbort,
		Due:                le.Due,
		Delay:              le.Delay,
	}

	return sr
}

type logEntryHandler func(logEntry) error

func (l *Log) processLog(filename string, fv *LogStatusFlags, lh logEntryHandler) error {
	fi, err := os.OpenFile(filename, os.O_RDONLY, 0)
	if err != nil {
		return err
	}
	sc := bufio.NewScanner(fi)
	filterDevice, filterSchedule := len(fv.Device) > 0, len(fv.Schedule) > 0
	for sc.Scan() {
		line := sc.Text()
		le, err := parseLogLine(line)
		if err != nil {
			return err
		}
		if filterDevice && le.Device != fv.Device {
			continue
		}
		if filterSchedule && le.Schedule != fv.Schedule {
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
		StatusRecorder:   internal.NewStatusRecorder(),
		pending:          make(map[int64]*internal.StatusRecord),
		streamingSummary: fv.StreamingSummary,
	}
	for _, arg := range args {
		if err := l.processLog(arg, fv, srh.process); err != nil {
			return err
		}
	}
	return nil
}

type statusRecoder struct {
	*internal.StatusRecorder
	pending          map[int64]*internal.StatusRecord
	streamingSummary bool
}

func fmtOp(rec *internal.StatusRecord) string {
	return fmt.Sprintf("%v:%v.%v", rec.Schedule, rec.Device, rec.Op)
}

func (sr *statusRecoder) print(out io.Writer) {
	fmt.Fprint(out, "Completed:\n")
	for rec := range sr.Completed() {
		var o strings.Builder
		fmt.Fprintf(&o, "% 70v: at %v, pending at: %v, due at: %v, delay: %v", fmtOp(rec), rec.Completed, rec.Pending.Truncate(time.Minute), rec.Due, rec.Delay)
		if rec.PreCondition != "" {
			if !rec.PreConditionResult {
				o.WriteString(fmt.Sprintf("(aborted due to %v)", rec.PreCondition))
			} else {
				o.WriteString(fmt.Sprintf("(completed after %v)", rec.PreCondition))
			}
		}
		o.WriteRune('\n')
		out.Write([]byte(o.String()))
	}
	fmt.Fprint(out, "Pending:\n")
	for rec := range sr.Pending() {
		fmt.Fprintf(out, "% 70v: due: %v, in %v\n", fmtOp(rec), rec.Due, rec.Delay.Round(time.Second))
	}
}

func (sr *statusRecoder) process(le logEntry) error {
	if le.Mod != "scheduler" {
		return nil
	}
	switch le.Msg {
	case "pending":
		rec := le.statusRecord()
		rec.Pending = le.Now
		rec = sr.NewPending(rec)
		sr.pending[le.ID] = rec
		return nil
	case "completed", "failed":
	case "starting schedules", "year-end", "ok", "too-late":
		return nil
	default:
		return fmt.Errorf("unknown message: %q", le.Msg)
	}
	pending, ok := sr.pending[le.ID]
	if !ok {
		return nil
	}
	if sr.streamingSummary {
		sr.print(os.Stdout)
	}
	sr.PendingDone(pending, le.PreCondAbort, le.Err)
	sr.print(os.Stdout)
	return nil
}
