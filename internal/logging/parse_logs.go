// Copyright 2024 Cosmos Nicolaou. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package logging

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"iter"
	"time"

	"cloudeng.io/datetime"
	"cloudeng.io/errors"
)

type logEntry struct {
	Msg           string    `json:"msg"`
	Mod           string    `json:"mod"`
	DryRun        bool      `json:"dry-run"`
	Schedule      string    `json:"schedule"`
	Device        string    `json:"device"`
	ID            int64     `json:"id"`
	Op            string    `json:"op"`
	Args          []string  `json:"args"`
	PreCond       string    `json:"pre"`
	PreCondArgs   []string  `json:"pre-args"`
	PreCondResult bool      `json:"pre-result"`
	NumActions    int       `json:"#actions"`
	YearEndDelay  int       `json:"year-end-delay"`
	Err           string    `json:"err"`
	Date          Date      `json:"date"`
	Now           time.Time `json:"now"`
	Due           time.Time `json:"due"`
	Started       time.Time `json:"started"`
	Delay         int       `json:"delay"`
	Location      string    `json:"loc"`
	YearEnd       int       `json:"year"`
}

type Entry struct {
	logEntry

	Date         datetime.CalendarDate
	Now          time.Time
	Due          time.Time
	Started      time.Time
	Delay        time.Duration
	YearEndDelay time.Duration
	Err          error
	LogEntry     string // Original log line
}

func ParseLogLine(line string) (Entry, error) {
	var le Entry
	le.LogEntry = line
	if err := json.Unmarshal([]byte(line), &le.logEntry); err != nil {
		return le, fmt.Errorf("error in line: %q: [%02x] %w", line, line, err)
	}
	loc, err := time.LoadLocation(le.Location)
	if err != nil {
		return le, fmt.Errorf("error in location: %q: %w", le.Location, err)
	}
	le.Date = datetime.CalendarDate(le.logEntry.Date)
	le.Now = le.logEntry.Now.In(loc)
	le.Due = le.logEntry.Due.In(loc)
	le.Started = le.logEntry.Started.In(loc)
	le.Delay = time.Duration(le.logEntry.Delay)
	le.YearEndDelay = time.Duration(le.logEntry.YearEndDelay)
	if e := le.logEntry.Err; e != "" {
		le.Err = errors.New(e)
	}
	return le, nil
}

func (le Entry) Aborted() bool {
	return le.PreCond != "" && !le.PreCondResult
}

func (le Entry) Name() string {
	return fmt.Sprintf("%v:%v.%v", le.Schedule, le.Device, le.Op)
}

func (le Entry) StatusRecord() *StatusRecord {
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

type Scanner struct {
	sc   *bufio.Scanner
	errs *errors.M
}

func NewScanner(rd io.Reader) *Scanner {
	return &Scanner{sc: bufio.NewScanner(rd), errs: &errors.M{}}
}

// Entries returns an iterator for over the LogScanner's LogEntry's. Set
// accumulateErrors to true to accumulate errors and continue processing the
// log file, or to false to stop on the first error.
func (ls *Scanner) Entries(accumulateErrors bool) iter.Seq[Entry] {
	return func(yield func(Entry) bool) {
		for {
			if !ls.sc.Scan() {
				if ls.sc.Err() == nil {
					fmt.Printf("scan done\n")
					return
				}
				fmt.Printf("scan error: %v\n", ls.sc.Err())
				ls.errs.Append(ls.sc.Err())
				return
			}
			line := ls.sc.Text()
			le, err := ParseLogLine(line)
			if err != nil {
				ls.errs.Append(err)
				fmt.Printf("parse error: %v\n", ls.sc.Err())
				if !accumulateErrors {
					return
				}
			}
			if !yield(le) {
				return
			}
		}
	}
}

func (ls *Scanner) Err() error {
	fmt.Printf("errs: %v .. %v\n", ls.errs, ls.errs.Err())
	return ls.errs.Err()
}
