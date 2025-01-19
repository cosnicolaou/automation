// Copyright 2025 Cosmos Nicolaou. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package logging

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

type Time time.Time

func (lt Time) MarshalJSON() ([]byte, error) {
	b := []byte(time.Time(lt).Format(TimeWithTZ))
	fmt.Printf("LT: %s\n", b)
	return b, nil
}

func (lt *Time) UnmarshalJSON(data []byte) error {
	t, err := time.Parse(TimeWithTZ, string(data))
	if err != nil {
		return err
	}
	*lt = Time(t)
	return nil
}

type TimeNano time.Time

func (lt TimeNano) MarshalJSON() ([]byte, error) {
	return []byte(time.Time(lt).Format(TimeWithTZNano)), nil
}

func (lt *TimeNano) UnmarshalJSON(data []byte) error {
	t, err := time.Parse(TimeWithTZNano, string(data))
	if err != nil {
		return err
	}
	*lt = TimeNano(t)
	return nil
}

type Duration time.Duration

func (ld Duration) MarshalJSON() ([]byte, error) {
	return []byte(time.Duration(ld).String()), nil
}

func (ld *Duration) UnmarshalJSON(data []byte) error {
	d, err := time.ParseDuration(string(data))
	if err != nil {
		return err
	}
	*ld = Duration(d)
	return nil
}

type Date datetime.CalendarDate

func (ld Date) MarshalJSON() ([]byte, error) {
	return []byte(datetime.CalendarDate(ld).String()), nil
}

func (ld *Date) UnmarshalJSON(data []byte) error {
	return (*datetime.CalendarDate)(ld).Parse(strings.Trim(string(data), `"`))
}
