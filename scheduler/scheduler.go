// Copyright 2024 Cosmos Nicolaou. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package scheduler

import (
	"context"
	"errors"
	"fmt"
	"io"
	"iter"
	"log/slog"
	"os"
	"time"

	"cloudeng.io/datetime"
	"cloudeng.io/datetime/schedule"
	"cloudeng.io/sync/errgroup"
	"github.com/cosnicolaou/automation/devices"
)

var ErrOpTimeout = errors.New("op-timeout")

func (s *Scheduler) runSingleOp(ctx context.Context, now, due time.Time, action schedule.Active[Action]) error {
	op := action.T.Action
	timeout := op.Device.Timeout()
	ctx, cancel := context.WithTimeoutCause(ctx, timeout, ErrOpTimeout)
	defer cancel()
	opts := devices.OperationArgs{
		Writer: s.opWriter,
		Logger: s.logger,
		Args:   op.Args,
	}
	errCh := make(chan error)
	go func() {
		errCh <- op.Op(ctx, opts)
	}()
	var err error
	select {
	case err = <-errCh:
	case <-ctx.Done():
		err = ctx.Err()
	}
	if err != nil {
		s.logger.Warn("failed", "op", op.Name, "now", now, "due", due, "err", err)
	} else {
		s.logger.Info("ok", "op", op.Name, "now", now, "due", due)
	}
	return nil
}

func (s *Scheduler) RunDay(ctx context.Context, place datetime.Place, active schedule.Scheduled[Action]) error {
	for active := range active.Active(place) {
		dueAt := active.When
		now := s.timeSource.NowIn(s.place.TZ)
		delay := dueAt.Sub(now)
		if delay > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
		}
		if err := s.runSingleOp(ctx, now, dueAt, active); err != nil {
			return err
		}
	}
	return nil
}

// RunYear runs the scheduler from the specified calendar date to the end of that
// year
func (s *Scheduler) RunYearEnd(ctx context.Context, cd datetime.CalendarDate) error {
	yp := datetime.YearPlace{
		Place: s.place,
		Year:  cd.Year(),
	}
	toYearEnd := datetime.NewDateRange(cd.Date(), datetime.NewDate(12, 31))
	for active := range s.scheduler.Scheduled(yp, s.schedule.Dates, toYearEnd) {
		s.logger.Info("ok", "#actions", len(active.Specs))
		if len(active.Specs) == 0 {
			continue
		}
		if err := s.RunDay(ctx, yp.Place, active); err != nil {
			return err
		}
	}
	return nil
}

func (s *Scheduler) ScheduledYearEnd(cd datetime.CalendarDate) iter.Seq[schedule.Scheduled[Action]] {
	yp := datetime.YearPlace{
		Place: s.place,
		Year:  cd.Year(),
	}
	toYearEnd := datetime.NewDateRange(cd.Date(), datetime.NewDate(12, 31))
	return s.scheduler.Scheduled(yp, s.schedule.Dates, toYearEnd)
}

func (s *Scheduler) Place() datetime.Place {
	return s.place
}

func actionAndDeviceNames(active schedule.Scheduled[Action]) (actionNames, deviceNames []string) {
	for _, a := range active.Specs {
		actionNames = append(actionNames, a.T.Name)
		deviceNames = append(deviceNames, a.T.DeviceName)
	}
	return actionNames, deviceNames
}

type Scheduler struct {
	options
	schedule  Annual
	scheduler *schedule.AnnualScheduler[Action]
	place     datetime.Place
}

type Option func(o *options)

type options struct {
	timeSource TimeSource
	logger     *slog.Logger
	opWriter   io.Writer
}

// TimeSource is an interface that provides the current time in a specific
// location and is intended for testing purposes. It will be called once
// per iteration of the scheduler to schedule the next action. time.Now().In()
// will be used for all other time operations.
type TimeSource interface {
	NowIn(in *time.Location) time.Time
}

type SystemTimeSource struct{}

func (SystemTimeSource) NowIn(loc *time.Location) time.Time {
	return time.Now().In(loc)
}

// WithTimeSource sets the time source to be used by the scheduler and
// is primarily intended for testing purposes.
func WithTimeSource(ts TimeSource) Option {
	return func(o *options) {
		o.timeSource = ts
	}
}

func WithLogger(l *slog.Logger) Option {
	return func(o *options) {
		o.logger = l
	}
}

func WithOperationWriter(w io.Writer) Option {
	return func(o *options) {
		o.opWriter = w
	}
}

// New creates a new scheduler for the supplied schedule and associated devices.
func New(sched Annual, system devices.System, opts ...Option) (*Scheduler, error) {
	scheduler := &Scheduler{
		schedule: sched,
		place:    system.Location.Place,
	}
	for _, opt := range opts {
		opt(&scheduler.options)
	}
	if scheduler.timeSource == nil {
		scheduler.timeSource = SystemTimeSource{}
	}
	if scheduler.logger == nil {
		scheduler.logger = slog.New(slog.NewJSONHandler(os.Stderr, nil))
	}
	if scheduler.opWriter == nil {
		scheduler.opWriter = os.Stdout
	}

	for i, a := range sched.DailyActions {
		dev := system.Devices[a.T.DeviceName]
		if dev == nil {
			return nil, fmt.Errorf("unknown device: %s", a.T.DeviceName)
		}
		op := dev.Operations()[a.T.Name]
		if op == nil {
			return nil, fmt.Errorf("unknown operation: %s for device: %v", a.T.Name, a.T.DeviceName)
		}
		sched.DailyActions[i].T.Device = dev
		sched.DailyActions[i].T.Action.Op = op
	}
	scheduler.logger = scheduler.logger.With("mod", "scheduler", "sched", sched.Name)
	scheduler.scheduler = schedule.NewAnnualScheduler(sched.DailyActions)
	return scheduler, nil
}

func RunSchedulers(ctx context.Context, schedules Schedules, system devices.System, start datetime.CalendarDate, opts ...Option) error {
	schedulers := make([]*Scheduler, 0, len(schedules.Schedules))
	for _, sched := range schedules.Schedules {
		s, err := New(sched, system, opts...)
		if err != nil {
			return fmt.Errorf("failed to create scheduler for %v: %w", sched.Name, err)
		}
		schedulers = append(schedulers, s)
	}
	var g errgroup.T
	for _, s := range schedulers {
		g.Go(func() error {
			if err := s.RunYearEnd(ctx, start); err != nil {
				return err
			}
			year := start.Year() + 1
			for {
				cd := datetime.NewCalendarDate(year, 1, 1)
				if err := s.RunYearEnd(ctx, cd); err != nil {
					return err
				}
			}
		})
	}
	return g.Wait()
}
