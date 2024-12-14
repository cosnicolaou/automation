// Copyright 2024 Cosmos Nicolaou. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package scheduler

import (
	"testing"
	"time"

	"cloudeng.io/datetime"
)

func TestDSTCalculations(t *testing.T) {
	loc, _ := time.LoadLocation("America/Los_Angeles")
	nd := func(m, d int) datetime.CalendarDate {
		return datetime.NewCalendarDate(2024, datetime.Month(m), d)
	}
	nt := func(h, m int) datetime.TimeOfDay {
		return datetime.NewTimeOfDay(h, m, 0)
	}

	for i, tc := range []struct {
		day     datetime.CalendarDate
		tod     datetime.TimeOfDay
		repeat  time.Duration
		retries int
	}{
		{nd(3, 10), nt(1, 0), time.Hour, 0},         // not affected by transition
		{nd(3, 10), nt(2, 0), time.Hour, 0},         // no need to reschedule
		{nd(11, 3), nt(1, 0), time.Hour, 1},         // reschedule once
		{nd(11, 3), nt(2, 0), time.Hour, 0},         // not affected by transition
		{nd(11, 3), nt(1, 52), 13 * time.Minute, 5}, // reschedule 5 times
		{nd(11, 3), nt(2, 52), 13 * time.Minute, 0}, // not affected by transition
		{nd(11, 3), nt(1, 59), time.Minute, 60},     // reschedule 60 times
		{nd(11, 3), nt(2, 59), time.Minute, 0},      // not affected by transition
		{nd(3, 10), nt(0, 0), time.Hour * 2, 0},     // always zero
		{nd(11, 3), nt(0, 0), time.Hour * 2, 0},     // always zero
		{nd(11, 3), nt(2, 0), time.Hour * 2, 0},     // always zero
	} {
		trh := DSTTransitions{}
		now := tc.day.Time(tc.tod, loc)
		then := now.Add(tc.repeat)
		if tc.repeat <= time.Hour {
			continue
		}
		nreschedules := trh.Reschedule(now, then, tc.repeat)
		if got, want := nreschedules, tc.retries; got != want {
			t.Errorf("%v: got %v, want %v", i, got, want)
		}

	}
}
