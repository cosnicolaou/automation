// Copyright 2024 Cosmos Nicolaou. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"time"

	"cloudeng.io/datetime"
	"github.com/cosnicolaou/automation/devices"
	"github.com/cosnicolaou/automation/internal/logging"
	"github.com/cosnicolaou/automation/scheduler"
)

type ScheduleFlags struct {
	ConfigFileFlags
	LogFile   string `subcmd:"log-file,,log file"`
	StartDate string `subcmd:"start-date,,start date"`
	DryRun    bool   `subcmd:"dry-run,,dry run"`
}

type SimulateFlags struct {
	ConfigFileFlags
	LogFile   string        `subcmd:"log-file,,log file"`
	DateRange string        `subcmd:"date-range,,date range in <month>/<day>/<year>:<year>/<month>/<day> format"`
	Delay     time.Duration `subcmd:"delay,10ms,delay between each simulated time step and the scheduled time"`
}

type SchedulePrintFlags struct {
	ConfigFileFlags
	DateRange string `subcmd:"date-range,,date range in <month>/<day>/<year>:<year>/<month>/<day> 	format"`
	Date      string `subcmd:"date,,date in <month>/<day>/<year> format"`
}

type Schedule struct {
	system    devices.System
	schedules scheduler.Schedules
}

func (s *Schedule) setupLogging(logfile string) (*slog.Logger, func(), error) {
	if len(logfile) == 0 {
		return slog.New(slog.NewJSONHandler(os.Stdout, nil)), func() {}, nil
	}
	var err error
	f, err := newLogfile(logfile)
	if err != nil {
		return nil, func() {}, err
	}
	l := slog.New(slog.NewJSONHandler(f, nil))
	return l, func() { f.Close() }, nil
}

func (s *Schedule) loadFiles(ctx context.Context, fv *ConfigFileFlags, deviceOpts []devices.Option) (context.Context, error) {
	ctx, sys, err := loadSystem(ctx, fv, deviceOpts...)
	if err != nil {
		return nil, err
	}
	scheds, err := loadSchedules(ctx, fv, sys)
	if err != nil {
		return nil, err
	}
	s.system = sys
	s.schedules = scheds
	return ctx, nil
}

func (s *Schedule) Run(ctx context.Context, flags any, _ []string) error {
	fv := flags.(*ScheduleFlags)
	var start datetime.CalendarDate
	if sd := fv.StartDate; sd != "" {
		if err := start.Parse(sd); err != nil {
			return err
		}
	} else {
		start = datetime.CalendarDateFromTime(time.Now())
	}

	logger, cleanup, err := s.setupLogging(fv.LogFile)
	if err != nil {
		return err
	}
	defer cleanup()

	deviceOpts := []devices.Option{
		devices.WithLogger(logger),
	}

	sr := logging.NewStatusRecorder()
	schedulerOpts := []scheduler.Option{
		scheduler.WithLogger(logger),
		scheduler.WithOperationWriter(os.Stdout),
		scheduler.WithDryRun(fv.DryRun),
		scheduler.WithStatusRecorder(sr),
	}

	ctx, err = s.loadFiles(ctx, &fv.ConfigFileFlags, deviceOpts)
	if err != nil {
		return err
	}

	if s.system.Location.Latitude == 0 && s.system.Location.Longitude == 0 {
		return fmt.Errorf("latitude and longitude must be specified either directly or via a zip code")
	}

	logger.Info("starting schedules", "start", start.String(), "loc", s.system.Location.TimeLocation.String(), "zip", s.system.Location.ZIPCode, "latitude", s.system.Location.Latitude, "longitude", s.system.Location.Longitude)

	return scheduler.RunSchedulers(ctx, s.schedules, s.system, start, schedulerOpts...)

}

func (s *Schedule) filterSchedules(schedules []scheduler.Annual, allowed []string) []scheduler.Annual {
	if len(allowed) == 0 {
		return schedules
	}
	filtered := []scheduler.Annual{}
	for _, sched := range schedules {
		for _, name := range allowed {
			if sched.Name == name {
				filtered = append(filtered, sched)
			}
		}
	}
	return filtered
}

func (s *Schedule) Simulate(ctx context.Context, flags any, args []string) error {
	fv := flags.(*SimulateFlags)
	var period datetime.CalendarDateRange
	if err := period.Parse(fv.DateRange); err != nil {
		return err
	}

	logger, cleanup, err := s.setupLogging(fv.LogFile)
	if err != nil {
		return err
	}
	defer cleanup()

	deviceOpts := []devices.Option{
		devices.WithLogger(logger),
	}

	sr := logging.NewStatusRecorder()
	schedulerOpts := []scheduler.Option{
		scheduler.WithLogger(logger),
		scheduler.WithOperationWriter(os.Stdout),
		scheduler.WithStatusRecorder(sr),
		scheduler.WithSimulationDelay(fv.Delay),
	}

	ctx, err = s.loadFiles(ctx, &fv.ConfigFileFlags, deviceOpts)
	if err != nil {
		return err
	}

	s.schedules.Schedules = s.filterSchedules(s.schedules.Schedules, args)

	if s.system.Location.Latitude == 0 && s.system.Location.Longitude == 0 {
		return fmt.Errorf("latitude and longitude must be specified either directly or via a zip code")
	}

	logger.Info("starting simulated schedules", "period", period.String(), "loc", s.system.Location.TimeLocation.String(), "zip", s.system.Location.ZIPCode, "latitude", s.system.Location.Latitude, "longitude", s.system.Location.Longitude)

	return scheduler.RunSimulation(ctx, s.schedules, s.system, period, schedulerOpts...)

}

func (s *Schedule) Print(ctx context.Context, flags any, args []string) error {
	fv := flags.(*SchedulePrintFlags)
	var dr datetime.CalendarDateRange
	if f := fv.DateRange; len(f) > 0 {
		if err := dr.Parse(f); err != nil {
			return err
		}
	} else {
		day := datetime.CalendarDateFromTime(time.Now())
		if f := fv.Date; len(f) > 0 {
			if err := day.Parse(f); err != nil {
				return err
			}
		}
		dr = datetime.NewCalendarDateRange(day, day)
	}

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	deviceOpts := []devices.Option{
		devices.WithLogger(logger),
	}
	_, err := s.loadFiles(ctx, &fv.ConfigFileFlags, deviceOpts)
	if err != nil {
		return err
	}

	s.schedules.Schedules = s.filterSchedules(s.schedules.Schedules, args)

	cal, err := scheduler.NewCalendar(s.schedules, s.system)
	if err != nil {
		return err
	}

	tw := tableManager{}.Calendar(cal, dr)
	fmt.Println(tw.Render())
	return nil
}
