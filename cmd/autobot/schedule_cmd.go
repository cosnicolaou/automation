// Copyright 2024 Cosmos Nicolaou. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"time"

	"cloudeng.io/datetime"
	"github.com/cosnicolaou/automation/cmd/autobot/internal/webapi"
	"github.com/cosnicolaou/automation/cmd/autobot/internal/webassets"
	"github.com/cosnicolaou/automation/devices"
	"github.com/cosnicolaou/automation/internal/logging"
	"github.com/cosnicolaou/automation/scheduler"
	"github.com/pkg/browser"
)

type ScheduleFlags struct {
	ConfigFileFlags
	WebUIFlags
	LogFile   string `subcmd:"log-file,,log file"`
	StartDate string `subcmd:"start-date,,start date"`
	DryRun    bool   `subcmd:"dry-run,,dry run"`
}

type SimulateFlags struct {
	ConfigFileFlags
	WebUIFlags
	LogFile   string        `subcmd:"log-file,,log file"`
	DateRange string        `subcmd:"date-range,,date range in <month>/<day>/<year>:<year>/<month>/<day> format"`
	Delay     time.Duration `subcmd:"delay,10ms,delay between each simulated time step and the scheduled time"`
	DryRun    bool          `subcmd:"dry-run,true,dry run"`
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

func (s *Schedule) serveStatusUI(ctx context.Context, systemfile string, fv WebUIFlags, logger *slog.Logger, statusRecorder *logging.StatusRecorder) error {
	mux := http.NewServeMux()
	runner, url, err := fv.CreateWebServer(ctx, mux, logger)
	if err != nil {
		return err
	}
	pages := fv.StatusPages()

	cc := webapi.NewStatusServer(logger, statusRecorder, s.calendar)

	cc.AppendEndpoints(ctx, mux)
	webassets.AppendStatusPages(mux, systemfile, pages)
	go func() {
		_ = browser.OpenURL(url)
		_ = runner()
	}()
	return nil
}

func (s *Schedule) calendar(schedules []string, dr datetime.CalendarDateRange) (webapi.CalendarResponse, error) {
	s.schedules.Schedules = filterSchedules(s.schedules.Schedules, schedules)
	cal, err := scheduler.NewCalendar(s.schedules, s.system)
	if err != nil {
		return webapi.CalendarResponse{}, err
	}
	entries := []webapi.CalendarEntry{}
	for day := range dr.Dates() {
		for _, a := range cal.Scheduled(day) {
			op := formatOperationWithArgs(a.T)
			pre := formatConditionWithArgs(a.T)
			when := datetime.NewTimeOfDay(a.When.Hour(), a.When.Minute(), a.When.Second())
			entries = append(entries, webapi.CalendarEntry{
				Date:      day.String(),
				Time:      when.String(),
				Schedule:  a.Schedule,
				Device:    a.T.DeviceName,
				Operation: op,
				Condition: pre,
			})
		}
	}
	return webapi.CalendarResponse{
		Range:     dr.String(),
		Schedules: schedules,
		Entries:   entries,
	}, nil
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

	if err := s.serveStatusUI(ctx, fv.ConfigFileFlags.SystemFile, fv.WebUIFlags, logger, sr); err != nil {
		return err
	}

	return scheduler.RunSchedulers(ctx, s.schedules, s.system, start, schedulerOpts...)

}

func filterSchedules(schedules []scheduler.Annual, allowed []string) []scheduler.Annual {
	if len(allowed) == 0 || (len(allowed) == 1 && len(allowed[0]) == 0) {
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
		scheduler.WithDryRun(fv.DryRun),
	}

	ctx, err = s.loadFiles(ctx, &fv.ConfigFileFlags, deviceOpts)
	if err != nil {
		return err
	}

	s.schedules.Schedules = filterSchedules(s.schedules.Schedules, args)

	if s.system.Location.Latitude == 0 && s.system.Location.Longitude == 0 {
		return fmt.Errorf("latitude and longitude must be specified either directly or via a zip code")
	}

	logger.Info("starting simulated schedules", "period", period.String(), "loc", s.system.Location.TimeLocation.String(), "zip", s.system.Location.ZIPCode, "latitude", s.system.Location.Latitude, "longitude", s.system.Location.Longitude)

	if err := s.serveStatusUI(ctx, fv.ConfigFileFlags.SystemFile, fv.WebUIFlags, logger, sr); err != nil {
		return err
	}
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

	s.schedules.Schedules = filterSchedules(s.schedules.Schedules, args)
	cal, err := scheduler.NewCalendar(s.schedules, s.system)
	if err != nil {
		return err
	}
	tw := tableManager{}.Calendar(cal, dr)
	fmt.Println(tw.Render())
	return nil
}
