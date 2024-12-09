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
	"github.com/cosnicolaou/automation/devices"
)

var OpTimeout = errors.New("op-timeout")

func (s *Scheduler) runSingleOp(ctx context.Context, now, due time.Time, action schedule.Action[Action]) error {
	op := action.Action
	timeout := op.Device.Timeout()
	ctx, cancel := context.WithTimeoutCause(ctx, timeout, OpTimeout)
	defer cancel()
	opts := devices.OperationArgs{
		Writer: s.opWriter,
		Logger: s.logger,
		Args:   op.Args,
	}
	errCh := make(chan error)
	go func() {
		errCh <- op.Action.Op(ctx, opts)
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

func (s *Scheduler) RunDay(ctx context.Context, yp datetime.YearAndPlace, active schedule.Active[Action]) error {
	cd := active.Date.CalendarDate(yp.Year)
	for action := range Actions(active.Actions).Daily(cd, s.place) {
		dueAt := datetime.Time(yp, active.Date, action.Due)
		now := s.timeSource.NowIn(s.place)
		delay := dueAt.Sub(now)
		if delay > 0 {
			if delay > time.Millisecond*10 {
				fmt.Printf("% 8v: delay %v: now: %v, due: %v\n", action.Name, delay, now, dueAt)
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
		}
		fmt.Printf("% 8v: executing: now: %v, (%v) due: %v\n", action.Name, now, delay, dueAt)
		fmt.Printf("clock: % 8v: %v\n", action.Name, now)
		if err := s.runSingleOp(ctx, now, dueAt, action); err != nil {
			return err
		}
	}
	return nil
}

func (s *Scheduler) RunYear(ctx context.Context, yp datetime.YearAndPlace) error {
	for active := range s.scheduler.Scheduled(yp) {
		s.logger.Info("ok", "#actions", len(active.Actions))
		if len(active.Actions) == 0 {
			continue
		}
		if err := s.RunDay(ctx, yp, active); err != nil {
			return err
		}
	}
	return nil
}

func (s *Scheduler) RunYears(ctx context.Context, yp datetime.YearAndPlace, nYears int) error {
	for y := 0; y < nYears; y++ {
		if err := s.RunYear(ctx, yp); err != nil {
			return err
		}
		yp.Year++
	}
	return nil
}

func (s *Scheduler) Scheduled(yp datetime.YearAndPlace) iter.Seq[schedule.Active[Action]] {
	return s.scheduler.Scheduled(yp)
}

func actionAndDeviceNames(active schedule.Active[Action]) (actionNames, deviceNames []string) {
	for _, a := range active.Actions {
		actionNames = append(actionNames, a.Action.Name)
		deviceNames = append(deviceNames, a.Action.DeviceName)
	}
	return actionNames, deviceNames
}

type Scheduler struct {
	options
	schedule  schedule.Annual[Action]
	scheduler *schedule.AnnualScheduler[Action]
	place     *time.Location
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
func New(sched schedule.Annual[Action], system devices.System, opts ...Option) (*Scheduler, error) {
	scheduler := &Scheduler{
		schedule: sched,
		place:    system.Location,
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

	for i, a := range sched.Actions {
		dev := system.Devices[a.Action.DeviceName]
		if dev == nil {
			return nil, fmt.Errorf("unknown device: %s", a.Action.DeviceName)
		}
		op := dev.Operations()[a.Action.Name]
		if op == nil {
			return nil, fmt.Errorf("unknown operation: %s for device: %v", a.Action.Name, a.Action.DeviceName)
		}
		sched.Actions[i].Action.Device = dev
		sched.Actions[i].Action.Action.Op = op
	}
	scheduler.logger = scheduler.logger.With("mod", "scheduler", "sched", sched.Name)
	scheduler.scheduler = schedule.NewAnnualScheduler(sched)
	return scheduler, nil
}

type MasterScheduler struct {
	schedulers []*Scheduler
}

func CreateMasterScheduler(schedules []Schedules, devices map[string]devices.Device, opts ...Option) (*MasterScheduler, error) {
	schedulers := make([]*Scheduler, 0, len(schedules))
	for _, sched := range schedules {
		_ = sched
		//schedulers = append(schedulers, NewScheduler(sched, dev, opts...))
	}
	ms := &MasterScheduler{schedulers: schedulers}
	return ms, nil
}
