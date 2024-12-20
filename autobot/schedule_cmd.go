// Copyright 2024 Cosmos Nicolaou. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"cloudeng.io/datetime"
	"github.com/cosnicolaou/automation/devices"
	"github.com/cosnicolaou/automation/internal"
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
	DateRange string        `subcmd:"date-range,,date range in <year>/<month>/<day>:<year>/<month>/<day> format"`
	Delay     time.Duration `subcmd:"delay,10ms,delay between each simulated time step and the scheduled time"`
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

func (s *Schedule) Run(ctx context.Context, flags any, args []string) error {
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

	sr := internal.NewStatusRecorder()
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

	logger.Info("starting schedules", "start", start.String(), "tz", s.system.Location.TZ.String(), "zip", s.system.Location.ZIPCode, "latitude", s.system.Location.Latitude, "longitude", s.system.Location.Longitude)

	return scheduler.RunSchedulers(ctx, s.schedules, s.system, start, schedulerOpts...)

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

	sr := internal.NewStatusRecorder()
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

	if s.system.Location.Latitude == 0 && s.system.Location.Longitude == 0 {
		return fmt.Errorf("latitude and longitude must be specified either directly or via a zip code")
	}

	logger.Info("starting simulated schedules", "period", period.String(), "tz", s.system.Location.TZ.String(), "zip", s.system.Location.ZIPCode, "latitude", s.system.Location.Latitude, "longitude", s.system.Location.Longitude)

	return scheduler.RunSimulation(ctx, s.schedules, s.system, period, schedulerOpts...)

}
