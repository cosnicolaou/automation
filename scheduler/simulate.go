// Copyright 2024 Cosmos Nicolaou. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package scheduler

import (
	"time"

	"cloudeng.io/datetime"
	"cloudeng.io/datetime/schedule"
	"github.com/cosnicolaou/automation/devices"
)

func yearEndTimes(scheduler schedule.AnnualScheduler[Action], year int, place datetime.Place, bound datetime.DateRange) []time.Time {
	times := []time.Time{}
	yp := datetime.YearPlace{
		Place: place,
		Year:  year,
	}
	for active := range s.scheduler.Scheduled(yp, s.schedule.Dates, bound) {
		for action := range active.Active(s.place) {
			times = append(times, action.When)
		}
	}
	return nil
}

func (s *Scheduler) allTimes(bound datetime.CalendarDateRange) []time.Time {
	times := []time.Time{}
	yearStart := bound.From().Date()
	for year := bound.From().Year(); year <= bound.To().Year(); year++ {
		nd := datetime.NewDateRange(yearStart, datetime.NewDate(12, 31))
		times = append(times, s.yearEndTimes(year, nd)...)
		yearStart = datetime.NewDate(1, 1)
	}
	return times
}

func Simulate(schedules Schedules, system devices.System, bound datetime.CalendarDateRange) error {
	times := []time.Time{}
	for _, s := range schedules.Schedules {
		scheduler := schedule.NewAnnualScheduler(s.DailyActions)
		times = append(times, scheduler.allTimes(bound)...)
	}
	return nil
}
