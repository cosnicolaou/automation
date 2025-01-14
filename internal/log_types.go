// Copyright 2025 Cosmos Nicolaou. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package internal

import (
	"fmt"
	"strings"
	"time"

	"cloudeng.io/datetime"
)

const (
	TimeWithTZ     = "2006-01-02T15:04:05 MST"
	TimeWithTZNano = "2006-01-02T15:04:05.999999999 MST"
)

type LogTime time.Time

func (lt LogTime) MarshalJSON() ([]byte, error) {
	b := []byte(time.Time(lt).Format(TimeWithTZ))
	fmt.Printf("LT: %s\n", b)
	return b, nil
}

func (lt *LogTime) UnmarshalJSON(data []byte) error {
	t, err := time.Parse(TimeWithTZ, string(data))
	if err != nil {
		return err
	}
	*lt = LogTime(t)
	return nil
}

type LogTimeNano time.Time

func (lt LogTimeNano) MarshalJSON() ([]byte, error) {
	return []byte(time.Time(lt).Format(TimeWithTZNano)), nil
}

func (lt *LogTimeNano) UnmarshalJSON(data []byte) error {
	t, err := time.Parse(TimeWithTZNano, string(data))
	if err != nil {
		return err
	}
	*lt = LogTimeNano(t)
	return nil
}

type LogDuration time.Duration

func (ld LogDuration) MarshalJSON() ([]byte, error) {
	return []byte(time.Duration(ld).String()), nil
}

func (ld *LogDuration) UnmarshalJSON(data []byte) error {
	d, err := time.ParseDuration(string(data))
	if err != nil {
		return err
	}
	*ld = LogDuration(d)
	return nil
}

type LogDate datetime.CalendarDate

func (ld LogDate) MarshalJSON() ([]byte, error) {
	return []byte(datetime.CalendarDate(ld).String()), nil
}

func (ld *LogDate) UnmarshalJSON(data []byte) error {
	return (*datetime.CalendarDate)(ld).Parse(strings.Trim(string(data), `"`))
}
