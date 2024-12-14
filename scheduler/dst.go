// Copyright 2024 Cosmos Nicolaou. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package scheduler

/*
import (
	"time"
)

// DSTTransitions provides support for handling Standard to Daylight Saving
// time transitions and vice versa for periodic/repeating actions. The transition
// from Standard to Daylight saving time results in the time interval 1AM to
// 2AM not existing, and the transition from Daylight Saving to Standard time
// results in the time interval 1AM to 2AM occurring twice (in two different
// timezones).
//
// In general, the transition from Standard to Daylight Saving time is
// handled by the time package in that times in the range 1AM to 2AM are
// interpreted as being in Savings time, for example, using time.Add on
// 1:59PST will return 1:59PDT (-800 vs -700 offset).
//
// Periodic or repeating actions that occur within these transitions need to
// be rescheduled or duplicated to maintain the required interval between events
// when possible. If the repeat interval is less than an hour, then it is possible
// to maintain that interval across transitions, but if the interval is
// greater than an hour, then it is reduced or extended by onehour. For the
// transition to DST, the interval will be reduced by one hour
// following the transition, and for the transition to Standard time, the
// interval will be extended by one hour.
//
// The transition from Daylight Saving to Standard time must be handled
// more carefully since the time interval 1AM to 2AM occurs twice and hence
// repeated events must be rescheduled to maintain the same interval between
// events. The Reschedule method provides this functionality and can be
// called for any time.Time values.
type DSTTransitions struct{}

// Reschedule returns the number of times that a repeating action
// should be rescheduled to maintain the same interval (in real-time) between
// events. 'now' represents the end of the last event and 'then' the 'now'
// plus 'interval'.
func (dt DSTTransitions) Reschedule(now, then time.Time, interval time.Duration) int {
	ndst, tdst := now.IsDST(), then.IsDST()
	if interval == 0 || ndst == tdst || interval > time.Hour {
		return 0
	}
	if ndst && !tdst {
		if then.Hour() != 1 { // 2AM is now 1AM in standard time.
			return 0
		}
		return dt.DaylightSavingToStandard(then, interval)
	}
	return 0
}

// DaylightSavingToStandard returns the number of times that an action
// should be rescheduled when transitioning from daylight saving to standard
// to maintain the same interval (in real-time) between events whenever
// possible (see Reschedule). The interval must be less than one hour for
// it to be rescheduled.
func (dt DSTTransitions) DaylightSavingToStandard(then time.Time, interval time.Duration) int {
	if interval == 0 || interval > time.Hour {
		return 0
	}
	secs := 3600 - (then.Minute()*60 + then.Second())
	dsecs := secs * int(time.Second)
	r := dsecs / int(interval)
	if dsecs%int(interval) != 0 {
		r++
	}
	return r
}
*/
