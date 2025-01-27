// Copyright 2024 Cosmos Nicolaou. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package scheduler

import (
	"context"
	"fmt"
	"time"

	"cloudeng.io/datetime"
	"cloudeng.io/datetime/schedule"
	"cloudeng.io/sync/errgroup"
	"github.com/cosnicolaou/automation/devices"
)

func ticksToYearEnd(scheduler *schedule.AnnualScheduler[Action], year int, place datetime.Place, dates schedule.Dates, bound datetime.DateRange, delay time.Duration) []time.Time {
	times := []time.Time{}
	yp := datetime.YearPlace{
		Place: place,
		Year:  year,
	}
	for active := range scheduler.Scheduled(yp, dates, bound) {
		for action := range active.Active(place) {
			times = append(times, action.When.Add(-delay))
		}
	}
	last := time.Date(year, 12, 31, 23, 59, 59, int(time.Second)-1, place.TimeLocation)
	times = append(times, last.Add(-delay))
	return times
}

func ticksForAllYears(scheduler *schedule.AnnualScheduler[Action], place datetime.Place, dates schedule.Dates, period datetime.CalendarDateRange, delay time.Duration) []time.Time {
	times := []time.Time{}
	yearStart := period.From().Date()
	for year := period.From().Year(); year <= period.To().Year(); year++ {
		thisYear := datetime.NewDateRange(yearStart, datetime.NewDate(12, 31))
		times = append(times, ticksToYearEnd(scheduler, year, place, dates, thisYear, delay)...)
		yearStart = datetime.NewDate(1, 1)
	}
	return times
}

type timesource struct {
	ch    chan time.Time
	ticks []time.Time
}

func (t timesource) NowIn(loc *time.Location) time.Time {
	n := <-t.ch
	return n.In(loc)
}

func (t timesource) run(ctx context.Context) error {
	for _, tick := range t.ticks {
		select {
		case t.ch <- tick:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

// RunSimulation runs the specified schedules against the specified system for the
// specified period using a similated time.
func RunSimulation(ctx context.Context, schedules Schedules, system devices.System, period datetime.CalendarDateRange, opts ...Option) error {
	var o options
	for _, opt := range opts {
		opt(&o)
	}
	delay := o.simulatedDelay
	if delay == 0 {
		delay = time.Millisecond * 10
	}
	timeSources := make([]timesource, len(schedules.Schedules))
	for i, s := range schedules.Schedules {
		scheduler := schedule.NewAnnualScheduler(s.DailyActions)
		ticks := ticksForAllYears(scheduler, system.Location.Place, s.Dates, period, delay)
		timeSources[i] = timesource{ch: make(chan time.Time), ticks: ticks}
	}
	schedulers := make([]*Scheduler, len(schedules.Schedules))
	for i, sched := range schedules.Schedules {
		psopts := opts
		psopts = append(psopts, WithTimeSource(timeSources[i]))
		s, err := New(sched, system, psopts...)
		if err != nil {
			return fmt.Errorf("failed to create scheduler for %v: %w", sched.Name, err)
		}
		schedulers[i] = s
	}

	var g errgroup.T
	for i, s := range schedulers {
		g.Go(func() error {
			if err := s.RunYearEnd(ctx, period.From()); err != nil {
				return err
			}
			for year := period.From().Year() + 1; year <= period.To().Year(); year++ {
				cd := datetime.NewCalendarDate(year, 1, 1)
				if err := s.RunYearEnd(ctx, cd); err != nil {
					return err
				}
			}
			return nil
		})
		g.Go(func() error {
			err := timeSources[i].run(ctx)
			return err
		})
	}
	return g.Wait()
}
