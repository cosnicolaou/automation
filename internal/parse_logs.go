// Copyright 2024 Cosmos Nicolaou. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package internal

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"iter"
	"time"

	"cloudeng.io/datetime"
)

type LogEntry struct {
	Msg           string   `json:"msg"`
	DateStr       string   `json:"date"`
	Mod           string   `json:"mod"`
	DryRun        bool     `json:"dry-run"`
	Schedule      string   `json:"schedule"`
	Device        string   `json:"device"`
	ID            int64    `json:"id"`
	Op            string   `json:"op"`
	Args          []string `json:"args"`
	PreCond       string   `json:"pre"`
	PreCondArgs   []string `json:"pre-args"`
	PreCondResult bool     `json:"pre-result"`
	NumActions    int      `json:"#actions"`
	NowStr        string   `json:"now"`
	StartedStr    string   `json:"started"`
	DueStr        string   `json:"due"`
	DelayStr      string   `json:"delay"`
	ErrStr        string   `json:"err"`

	Date    datetime.CalendarDate
	Now     time.Time
	Due     time.Time
	Started time.Time
	Delay   time.Duration
	Err     error

	LogEntry string // Original log line
}

func ParseLogLine(line string) (LogEntry, error) {
	var le LogEntry
	le.LogEntry = line
	if err := json.Unmarshal([]byte(line), &le); err != nil {
		return le, err
	}
	var err error
	if len(le.DelayStr) != 0 {
		le.Delay, err = time.ParseDuration(le.DelayStr)
		if err != nil {
			fmt.Printf("failed to parse duration: %v: %v: %v\n", le.DelayStr, err, line)
			return le, err
		}
	}
	if len(le.NowStr) != 0 {
		le.Now, err = time.Parse(time.RFC3339Nano, le.NowStr)
		if err != nil {
			fmt.Printf("failed to parse time: %v: %v: %v\n", le.NowStr, err, line)
			return le, err
		}
	}
	if len(le.StartedStr) != 0 {
		le.Started, err = time.Parse(time.RFC3339Nano, le.StartedStr)
		if err != nil {
			fmt.Printf("failed to parse time: %v: %v: %v\n", le.StartedStr, err, line)
			return le, err
		}
	}
	if len(le.DueStr) != 0 {
		le.Due, err = time.Parse(time.RFC3339, le.DueStr)
		if err != nil {
			fmt.Printf(
				"failed to parse time: %v: %v: %v\n", le.DueStr, err, line)
			return le, err
		}
	}
	if len(le.DateStr) != 0 {
		tmp := new(datetime.CalendarDate)
		if err := tmp.Parse(le.DateStr); err != nil {
			fmt.Printf("failed to parse date: %v: %v: %v\n", le.DateStr, err, line)
			return le, err
		}
		le.Date = *tmp
	}
	if le.ErrStr != "" {
		le.Err = errors.New(le.ErrStr)
	}
	return le, nil
}

func (le LogEntry) Aborted() bool {
	return le.PreCond != "" && !le.PreCondResult
}

func (le LogEntry) Name() string {
	return fmt.Sprintf("%v:%v.%v", le.Schedule, le.Device, le.Op)
}

func (le LogEntry) StatusRecord() *StatusRecord {
	sr := &StatusRecord{
		ID:                 le.ID,
		Schedule:           le.Schedule,
		Device:             le.Device,
		Op:                 le.Op,
		OpArgs:             le.Args,
		PreCondition:       le.PreCond,
		PreConditionArgs:   le.PreCondArgs,
		PreConditionResult: le.PreCondResult,
		Due:                le.Due,
		Delay:              le.Delay,
	}
	return sr
}

type LogScanner struct {
	sc  *bufio.Scanner
	err error
}

func NewLogScanner(rd io.Reader) *LogScanner {
	return &LogScanner{sc: bufio.NewScanner(rd)}
}

// Entries returns an iterator for over the LogScanner's LogEntry's. Note
// that the iterator will stop if an error is encountered and that the
// Scanner's Err method should be checked after the iterator has completed.
func (ls *LogScanner) Entries() iter.Seq[LogEntry] {
	return func(yield func(LogEntry) bool) {
		for {
			if !ls.sc.Scan() {
				ls.err = ls.sc.Err()
				return
			}
			if ls.err != nil {
				return
			}
			line := ls.sc.Text()
			le, err := ParseLogLine(line)
			if err != nil {
				ls.err = err
				continue
			}
			if !yield(le) {
				return
			}
		}
	}
}

func (ls *LogScanner) Err() error {
	return ls.err
}
