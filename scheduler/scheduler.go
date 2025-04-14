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
	"github.com/cosnicolaou/automation/internal/logging"
)

var ErrOpTimeout = errors.New("op-timeout")

func (s *Scheduler) invokeOp(ctx context.Context, action Action, opts devices.OperationArgs) (bool, error) {
	if pre := action.Precondition; pre.Condition != nil {
		preOpts := devices.OperationArgs{
			Due:    opts.Due,
			Place:  opts.Place,
			Writer: opts.Writer,
			Logger: s.logger,
			Args:   pre.Args,
		}
		_, ok, err := pre.Condition(ctx, preOpts)
		if err != nil {
			return false, fmt.Errorf("failed to evaluate precondition: %v: %v", pre.Name, err)
		}
		s.logger.Info("precondition", "op", action.Name, "passed", ok)
		if !ok {
			return true, nil
		}
	}
	_, err := action.Op(ctx, opts)
	return false, err
}

func (s *Scheduler) runSingleOp(ctx context.Context, due time.Time, action schedule.Active[Action]) (aborted bool, err error) {
	op := action.T.Action
	// TODO(cnicolaou): implement retries.
	ctx, cancel := context.WithTimeoutCause(ctx, op.Device.Config().Timeout, ErrOpTimeout)
	defer cancel()
	opts := devices.OperationArgs{
		Due:    due,
		Place:  s.place,
		Writer: s.opWriter,
		Logger: s.logger,
		Args:   op.Args,
	}
	errCh := make(chan error)
	var preconditionAbort bool
	go func() {
		var err error
		preconditionAbort, err = s.invokeOp(ctx, action.T, opts)
		errCh <- err
	}()
	select {
	case err = <-errCh:
		close(errCh)
	case <-ctx.Done():
		err = ctx.Err()
	}
	return preconditionAbort, err
}

func (s *Scheduler) newStatusRecord(delay time.Duration, a schedule.Active[Action]) *logging.StatusRecord {
	rec := &logging.StatusRecord{
		Schedule: s.schedule.Name,
		Due:      a.When,
		Delay:    delay,
		Device:   a.T.DeviceName,
		Op:       a.T.Name,
	}
	if pc := a.T.Precondition; pc.Condition != nil {
		rec.PreCondition = pc.Name
	}
	return rec
}

func (s *Scheduler) newPending(id int64, delay time.Duration, a schedule.Active[Action]) *logging.StatusRecord {
	if sr := s.statusRecorder; sr != nil {
		rec := s.newStatusRecord(delay, a)
		rec.ID = id
		return sr.NewPending(rec)
	}
	return nil
}

func (s *Scheduler) completed(rec *logging.StatusRecord, precondition bool, err error) {
	if sr := s.statusRecorder; sr != nil {
		sr.PendingDone(rec, precondition, err)
	}
}

func (s *Scheduler) RunDay(ctx context.Context, place datetime.Place, active schedule.Scheduled[Action]) error {
	for active := range active.Active(place) {
		dueAt := active.When
		started := s.timeSource.NowIn(dueAt.Location())
		delay := dueAt.Sub(started)
		overdue := delay < 0 && -delay > time.Minute
		id := logging.WritePending(
			s.logger,
			overdue,
			s.dryRun,
			active.T.DeviceName,
			active.T.Name,
			active.T.Args,
			active.T.Precondition.Name,
			active.T.Precondition.Args,
			started,
			dueAt,
			delay,
		)
		if overdue {
			continue
		}
		rec := s.newPending(id, delay, active)
		if delay > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
		}
		var aborted bool
		var err error
		if !s.dryRun {
			aborted, err = s.runSingleOp(ctx, dueAt, active)
		}
		logging.WriteCompletion(
			s.logger,
			id,
			err,
			s.dryRun,
			active.T.DeviceName,
			active.T.Name,
			active.T.Precondition.Name,
			!aborted,
			started,
			time.Now().In(dueAt.Location()),
			dueAt,
			delay,
		)
		s.completed(rec, !aborted, err)
		if s.dryRun {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
		}
	}
	return nil
}

// Run runs the scheduler from the specified calendar date to the last of the scheduled
// actions for that year.
func (s *Scheduler) RunYear(ctx context.Context, cd datetime.CalendarDate) error {
	yp := datetime.YearPlace{
		Place: s.place,
		Year:  cd.Year(),
	}
	toYearEnd := datetime.NewDateRange(cd.Date(), datetime.NewDate(12, 31))
	for active := range s.scheduler.Scheduled(yp, s.schedule.Dates, toYearEnd) {
		logging.WriteNewDay(s.logger, active.Date, len(active.Specs))
		if len(active.Specs) == 0 {
			continue
		}
		if err := s.RunDay(ctx, yp.Place, active); err != nil {
			return err
		}
	}
	return nil
}

// RunYear runs the scheduler from the specified calendar date to the end of that
// year.
func (s *Scheduler) RunYearEnd(ctx context.Context, cd datetime.CalendarDate) error {
	if err := s.RunYear(ctx, cd); err != nil {
		return err
	}
	year := cd.Year()
	yearEnd := time.Date(year, 12, 31, 23, 59, 59, int(time.Second)-1, s.place.TimeLocation)
	now := s.timeSource.NowIn(s.place.TimeLocation)
	delay := yearEnd.Sub(now)
	logging.WriteYearEnd(s.logger, year, delay)
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(delay):
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

type Scheduler struct {
	options
	schedule  Annual
	scheduler *schedule.AnnualScheduler[Action]
	place     datetime.Place
}

type Option func(o *options)

type options struct {
	timeSource     TimeSource
	logger         *slog.Logger
	opWriter       io.Writer
	dryRun         bool
	statusRecorder *logging.StatusRecorder
	simulatedDelay time.Duration
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

// WithLogger sets the logger to be used by the scheduler and is also
// passed to all device operations/conditions.
func WithLogger(l *slog.Logger) Option {
	return func(o *options) {
		o.logger = l
	}
}

// WithOperationWriter sets the output writer that operations can use
// for interactive output.
func WithOperationWriter(w io.Writer) Option {
	return func(o *options) {
		o.opWriter = w
	}
}

func WithDryRun(v bool) Option {
	return func(o *options) {
		o.dryRun = v
	}
}

func WithStatusRecorder(sr *logging.StatusRecorder) Option {
	return func(o *options) {
		o.statusRecorder = sr
	}
}

func WithSimulationDelay(d time.Duration) Option {
	return func(o *options) {
		o.simulatedDelay = d
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
		sched.DailyActions[i].T.Op = op
	}
	scheduler.logger = scheduler.logger.With("mod", "scheduler", "schedule", sched.Name)
	scheduler.scheduler = schedule.NewAnnualScheduler(sched.DailyActions)
	return scheduler, nil
}

// RunSchedulers runs the supplied schedules against the supplied system starting
// at the specified date until the context is canceled. Note that the WithTimeSource
// option should not be used with this function as it will be used by all of
// the schedulers created which is likely not what is intended. Note that the
// Simulate function can be used to run multiple schedules using simulated
// time appropriate for each schedule.
func RunSchedulers(ctx context.Context, schedules Schedules, system devices.System, start datetime.CalendarDate, opts ...Option) error {
	schedulers := make([]*Scheduler, len(schedules.Schedules))
	for i, sched := range schedules.Schedules {
		s, err := New(sched, system, opts...)
		if err != nil {
			return fmt.Errorf("failed to create scheduler for %v: %w", sched.Name, err)
		}
		schedulers[i] = s
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
				year++
			}
		})
	}
	return g.Wait()
}
