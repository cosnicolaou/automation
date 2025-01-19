// Copyright 2024 Cosmos Nicolaou. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package scheduler

import (
	"fmt"
	"sort"

	"cloudeng.io/datetime"
	"cloudeng.io/datetime/schedule"
	"github.com/cosnicolaou/automation/devices"
)

type Calendar struct {
	place      datetime.Place
	schedulers []*Scheduler
}

func NewCalendar(schedules Schedules, system devices.System, opts ...Option) (*Calendar, error) {
	c := &Calendar{
		place: system.Location.Place,
	}
	c.schedulers = make([]*Scheduler, len(schedules.Schedules))
	for i, sched := range schedules.Schedules {
		s, err := New(sched, system, opts...)
		if err != nil {
			return nil, fmt.Errorf("failed to create scheduler for %v: %w", sched.Name, err)
		}
		c.schedulers[i] = s
	}
	return c, nil
}

type CalendarEntry struct {
	Schedule string
	schedule.Active[Action]
}

func (c *Calendar) Scheduled(date datetime.CalendarDate) []CalendarEntry {
	yp := datetime.YearPlace{
		Year:  date.Year(),
		Place: c.place,
	}
	month, day := date.Month(), date.Day()
	today := datetime.NewDateRange(
		datetime.NewDate(month, day),
		datetime.NewDate(month, day),
	)
	actions := make([]CalendarEntry, 0, 50)
	for _, schedule := range c.schedulers {
		for perDay := range schedule.scheduler.Scheduled(yp, schedule.schedule.Dates, today) {
			for action := range perDay.Active(c.place) {
				actions = append(actions, CalendarEntry{
					Schedule: schedule.schedule.Name,
					Active:   action,
				})
			}
		}
	}
	sort.Slice(actions, func(i, j int) bool {
		return actions[i].When.Before(actions[j].When)
	})
	return actions
}
