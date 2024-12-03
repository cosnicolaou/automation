// Copyright 2024 Cosmos Nicolaou. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package scheduler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"slices"
	"sync"
	"testing"
	"time"

	"cloudeng.io/datetime"
	"cloudeng.io/errors"
	"github.com/cosnicolaou/automation/devices"
	"github.com/cosnicolaou/automation/internal/testutil"
	"github.com/cosnicolaou/automation/scheduler"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

type slow_test_device struct {
	testutil.MockDevice
	timeout time.Duration
	delay   time.Duration
}

func (st *slow_test_device) Operations() map[string]devices.Operation {
	return map[string]devices.Operation{
		"on": st.On,
	}
}

func (st *slow_test_device) Timeout() time.Duration {
	return st.timeout
}

func (st *slow_test_device) On(context.Context, devices.OperationArgs) error {
	time.Sleep(st.delay)
	return nil
}

type timesource struct {
	ch chan time.Time
}

func (t *timesource) NowIn(loc *time.Location) time.Time {
	n := <-t.ch
	return n.In(loc)
}

func (t *timesource) tick(nextTick time.Time) {
	t.ch <- nextTick
}

type testAction struct {
	when   time.Time
	action scheduler.Daily
}

func subMilliseconds(yp datetime.YearAndPlace, date datetime.Date, tod datetime.TimeOfDay) time.Time {
	hr := tod.Hour()
	min := tod.Minute()
	sec := tod.Second()
	nano := int(time.Millisecond * 990)
	if sec > 0 {
		return time.Date(yp.Year, time.Month(date.Month()), date.Day(), hr, min, sec-1, nano, yp.Place)
	}
	sec = 59
	if min > 0 {
		return time.Date(yp.Year, time.Month(date.Month()), date.Day(), hr, min-1, sec, nano, yp.Place)
	}
	min = 59
	return time.Date(yp.Year, time.Month(date.Month()), date.Day(), hr-1, min, sec, nano, yp.Place)

}

func allScheduled(s *scheduler.Scheduler, yp datetime.YearAndPlace) ([]testAction, []time.Time) {
	actions := []testAction{}
	times := []time.Time{}
	for active := range s.Scheduled(yp) {
		if len(active.Actions) == 0 {
			continue
		}
		for _, action := range active.Actions {
			// create a time that is a little before the scheduled time
			// to more closely resemble a production setting. Note using
			// time.Date and then subtracting a millisecond is not sufficient
			// to test for DST handling, since time.Add does not handle
			// DST changes and as such is not the same as calling time.Now()
			// 10 milliseconds before the scheduled time.
			dt := subMilliseconds(yp, active.Date, action.Due)
			times = append(times, dt)
			actions = append(actions, testAction{
				action: action.Action,
				when:   datetime.Time(yp, active.Date, action.Due),
			})
		}
	}
	return actions, times
}

type recorder struct {
	sync.Mutex
	out *bytes.Buffer
}

func (r *recorder) Write(p []byte) (n int, err error) {
	r.Lock()
	defer r.Unlock()
	return r.out.Write(p)
}

func (r *recorder) Lines() []string {
	lines := []string{}
	for _, l := range bytes.Split(r.out.Bytes(), []byte("\n")) {
		if len(l) == 0 {
			continue
		}
		lines = append(lines, string(l))
	}
	return lines
}

type logEntry struct {
	Sched      string    `json:"sched"`
	Msg        string    `json:"msg"`
	Op         string    `json:"op"`
	Due        time.Time `json:"due"`
	NumActions int       `json:"#actions"`
	Error      string    `json:"err"`
}

func (r *recorder) Logs(t *testing.T) []logEntry {
	entries := []logEntry{}
	for _, l := range bytes.Split(r.out.Bytes(), []byte("\n")) {
		if len(l) == 0 {
			continue
		}
		var e logEntry
		if err := json.Unmarshal(l, &e); err != nil {
			t.Errorf("failed to unmarshal: %v: %v", string(l), err)
			return nil
		}
		if e.NumActions != 0 || e.Msg == "late" {
			continue
		}
		entries = append(entries, e)
	}
	return entries
}

func containsError(logs []logEntry) error {
	for _, l := range logs {
		if l.Error != "" {
			return errors.New(l.Error)
		}
	}
	return nil
}

func newRecorder() *recorder {
	return &recorder{out: bytes.NewBuffer(nil)}
}

func setupSchedules(t *testing.T, schedule_config string) (devices.System, scheduler.Schedules) {
	ctx := context.Background()
	sys, err := devices.ParseSystemConfig(ctx, "", []byte(devices_config),
		devices.WithDevices(supportedDevices),
		devices.WithControllers(supportedControllers))
	if err != nil {
		t.Fatal(err)
	}
	spec, err := scheduler.ParseConfig(ctx, []byte(schedule_config), sys)
	if err != nil {
		t.Fatal(err)
	}
	return sys, spec
}

func newRecordersAndLogger(ts scheduler.TimeSource) (*recorder, *recorder, []scheduler.Option) {
	deviceRecorder := newRecorder()
	logRecorder := newRecorder()
	logger := slog.New(slog.NewJSONHandler(logRecorder, nil))
	opts := []scheduler.Option{
		scheduler.WithTimeSource(ts),
		scheduler.WithLogger(logger),
		scheduler.WithOperationWriter(deviceRecorder),
	}
	return deviceRecorder, logRecorder, opts
}

func runScheduler(ctx context.Context, t *testing.T, scheduler *scheduler.Scheduler, yp datetime.YearAndPlace, ts *timesource, times []time.Time) {
	var errs errors.M
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		errs.Append(scheduler.RunYear(ctx, yp))
		wg.Done()
	}()
	for _, t := range times {
		ts.tick(t)
		time.Sleep(time.Millisecond * 2)
	}
	wg.Wait()
	if err := errs.Err(); err != nil {
		t.Fatal(err)
	}
}

func TestScheduler(t *testing.T) {
	ctx := context.Background()

	ts := &timesource{ch: make(chan time.Time, 1)}
	deviceRecorder, logRecorder, opts := newRecordersAndLogger(ts)
	sys, spec := setupSchedules(t, schedule_config)

	yp := datetime.YearAndPlace{Year: 2021, Place: sys.Location}

	diningRoom := spec.Lookup("ranges")
	scheduler, err := scheduler.New(diningRoom, sys, opts...)
	if err != nil {
		t.Fatal(err)
	}

	all, times := allScheduled(scheduler, yp)
	runScheduler(ctx, t, scheduler, yp, ts, times)

	logs := logRecorder.Logs(t)
	if err := containsError(logs); err != nil {
		t.Fatal(err)
	}
	// 01/22:2, 11/22:12/28 translates to:
	// 10+28+9+28 days
	days := 10 + 28 + 9 + 28
	if got, want := len(all), days*3; got != want {
		t.Errorf("got %d, want %d", got, want)
	}

	if got, want := len(logs), days*3; got != want {
		t.Errorf("got %d, want %d", got, want)
	}

	for i := range len(logs) / 3 {
		lg1, lg2, lg3 := logs[i*3], logs[i*3+1], logs[i*3+2]
		if got, want := lg1.Due, times[i*3].Add(time.Millisecond*10); !got.Equal(want) {
			t.Errorf("%#v: got %v, want %v", lg1, got, want)
		}
		if got, want := lg2.Due, times[i*3+1].Add(time.Millisecond*10); !got.Equal(want) {
			t.Errorf("%#v: got %v, want %v", lg2, got, want)
		}
		if got, want := lg3.Due, times[i*3+2].Add(time.Millisecond*10); !got.Equal(want) {
			t.Errorf("%#v: got %v, want %v", lg3, got, want)
		}
		if got, want := lg1.Op, "another"; got != want {
			t.Errorf("%#v: got %v, want %v", lg1, got, want)
		}
		if got, want := lg2.Op, "on"; got != want {
			t.Errorf("%#v: got %v, want %v", lg2, got, want)
		}
		if got, want := lg3.Op, "off"; got != want {
			t.Errorf("%#v: got %v, want %v", lg3, got, want)
		}
	}

	lines := deviceRecorder.Lines()
	if got, want := len(lines), days*3; got != want {
		t.Errorf("got %d, want %d", got, want)
	}
	for i := range len(lines) / 3 {
		l1, l2, l3 := lines[i*3], lines[i*3+1], lines[i*3+2]
		// expect to see on, another, off, or another, on, off
		// since on and another are co-scheduled.
		want1 := "device[device].On: [0] "
		want2 := "device[device].Another: [2] arg1--arg2"
		if l1 == "device[device].Another: [2] arg1--arg2" {
			want2, want1 = want1, want2
		}
		if got, want := l1, want1; got != want {
			t.Errorf("got %v, want %v", got, want)
		}
		if got, want := l2, want2; got != want {
			t.Errorf("got %v, want %v", got, want)
		}
		if got, want := l3, "device[device].Off: [0] "; got != want {
			t.Errorf("got %v, want %v", got, want)
		}
	}
}

func TestScheduleRealTime(t *testing.T) {
	ctx := context.Background()
	yp := datetime.YearAndPlace{Year: time.Now().Year()}

	sys, spec := setupSchedules(t, schedule_config)
	yp.Place = sys.Location

	now := time.Now().In(yp.Place)
	today := datetime.DateFromTime(now)
	sched := spec.Lookup("simple")
	sched.Dates.Ranges = []datetime.DateRange{datetime.NewDateRange(today, today)}

	sched.Actions[0].Due = datetime.TimeOfDayFromTime(now.Add(time.Second))
	sched.Actions[1].Due = datetime.TimeOfDayFromTime(now.Add(2 * time.Second))

	deviceRecorder := newRecorder()
	logRecorder := newRecorder()
	logger := slog.New(slog.NewJSONHandler(logRecorder, nil))

	opts := []scheduler.Option{
		scheduler.WithLogger(logger),
		scheduler.WithOperationWriter(deviceRecorder)}

	scheduler, err := scheduler.New(sched, sys, opts...)
	if err != nil {
		t.Fatal(err)
	}

	all, _ := allScheduled(scheduler, yp)

	var errs errors.M
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		errs.Append(scheduler.RunYear(ctx, yp))
		wg.Done()
	}()
	wg.Wait()
	if err := errs.Err(); err != nil {
		t.Fatal(err)
	}

	logs := logRecorder.Logs(t)
	if err := containsError(logs); err != nil {
		t.Fatal(err)
	}
	lines := deviceRecorder.Lines()

	if got, want := len(logs), len(all); got != want {
		t.Errorf("got %d, want %d", got, want)
	}
	if got, want := len(lines), len(all); got != want {
		t.Errorf("got %d, want %d", got, want)
	}
	for i := range all {
		if got, want := logs[i].Due, all[i].when; !got.Equal(want) {
			t.Errorf("got %v, want %v", got, want)
		}
		if got, want := logs[i].Op, all[i].action.Name; got != want {
			t.Errorf("got %v, want %v", got, want)
		}

		deviceOutput := fmt.Sprintf("device[device].%v: [0] ", cases.Title(
			language.English).String(all[i].action.Name))
		if got, want := lines[i], deviceOutput; got != want {
			t.Errorf("got %v, want %v", got, want)
		}
	}
}

func TestTimeout(t *testing.T) {
	ctx := context.Background()
	yp := datetime.YearAndPlace{Year: time.Now().Year()}

	sys, spec := setupSchedules(t, schedule_config)
	yp.Place = sys.Location

	logRecorder := newRecorder()
	logger := slog.New(slog.NewJSONHandler(logRecorder, nil))

	for _, tc := range []struct {
		sched  string
		cancel bool
		errmsg string
	}{
		{"slow", false, "context deadline exceeded"},   // timeout
		{"hanging", true, "context deadline exceeded"}, // hanging, must be canceled
	} {
		ctx, cancel := context.WithCancel(ctx)

		now := time.Now().In(yp.Place)
		today := datetime.DateFromTime(now)
		sched := spec.Lookup(tc.sched) // slow device schedule
		sched.Dates.Ranges = []datetime.DateRange{datetime.NewDateRange(today, today)}

		sched.Actions[0].Due = datetime.TimeOfDayFromTime(now.Add(time.Second))

		opts := []scheduler.Option{scheduler.WithLogger(logger)}
		scheduler, err := scheduler.New(sched, sys, opts...)
		if err != nil {
			t.Fatal(err)
		}
		if tc.cancel {
			go func() {
				time.Sleep(time.Second)
				cancel()
			}()
		}

		if err := scheduler.RunYear(ctx, yp); err != nil {
			t.Fatal(err)
		}

		logs := logRecorder.Logs(t)
		if err := containsError(logs); err == nil || err.Error() != tc.errmsg {
			t.Errorf("unexpected or missing error: %v", err)
		}

		cancel()
	}
}

func TestMultiYear(t *testing.T) {
	ctx := context.Background()
	yp := datetime.YearAndPlace{Year: 2023}

	sys, spec := setupSchedules(t, schedule_config)
	yp.Place = sys.Location

	ts := &timesource{ch: make(chan time.Time, 1)}
	deviceRecorder := newRecorder()
	logRecorder := newRecorder()
	logger := slog.New(slog.NewJSONHandler(logRecorder, nil))
	opts := []scheduler.Option{
		scheduler.WithTimeSource(ts),
		scheduler.WithOperationWriter(deviceRecorder),
		scheduler.WithLogger(logger),
	}

	scheduler, err := scheduler.New(spec.Lookup("multi-year"), sys, opts...)
	if err != nil {
		t.Fatal(err)
	}

	all2023, times2023 := allScheduled(scheduler, yp)
	all2024, times2024 := allScheduled(scheduler, datetime.YearAndPlace{Year: 2024, Place: yp.Place})
	times := append(times2023, times2024...)
	all := append(all2023, all2024...)
	if len(times) != len(all) {
		t.Fatalf("mismatch: %v %v", len(times), len(all))
	}

	var errs errors.M
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		errs.Append(scheduler.RunYears(ctx, yp, 2))
		wg.Done()
	}()
	for _, t := range times {
		ts.tick(t)
		time.Sleep(time.Millisecond * 2)
	}
	wg.Wait()
	if err := errs.Err(); err != nil {
		t.Fatal(err)
	}

	logs := logRecorder.Logs(t)
	if err := containsError(logs); err != nil {
		t.Fatal(err)
	}

	for i, l := range logs {
		if got, want := l.Due, times[i].Add(time.Millisecond*10); !got.Equal(want) {
			t.Errorf("got %v, want %v", got, want)
		}
	}
	lines := deviceRecorder.Lines()
	if got, want := len(lines), 9; got != want {
		t.Errorf("got %d, want %d", got, want)
	}
}

func TestDST(t *testing.T) {
	ctx := context.Background()

	ts := &timesource{ch: make(chan time.Time, 1)}
	deviceRecorder, logRecorder, opts := newRecordersAndLogger(ts)
	sys, spec := setupSchedules(t, schedule_config)

	yp := datetime.YearAndPlace{Year: 2024, Place: sys.Location}

	dst := spec.Lookup("daylight-saving-time")
	scheduler, err := scheduler.New(dst, sys, opts...)
	if err != nil {
		t.Fatal(err)
	}

	all, times := allScheduled(scheduler, yp)
	runScheduler(ctx, t, scheduler, yp, ts, times)

	// Make sure all operations were called despite the DST transitions.
	opsLines := deviceRecorder.Lines()
	ndays := 4 + 3
	if got, want := len(opsLines), ndays*3; got != want {
		t.Errorf("got %d, want %d", got, want)
	}

	for i := 0; i < len(opsLines)/3; i++ {
		on := opsLines[i*3]
		another := opsLines[i*3+1]
		off := opsLines[i*3+2]
		if got, want := on, "device[device].On: [0] "; got != want {
			t.Errorf("%v: got %v, want %v", i, got, want)
		}
		if got, want := another, "device[device].Another: [2] arg1--arg2"; got != want {
			t.Errorf("%v: got %v, want %v", i, got, want)
		}
		if got, want := off, "device[device].Off: [0] "; got != want {
			t.Errorf("%v: got %v, want %v", i, got, want)
		}
	}

	logs := logRecorder.Logs(t)
	if err := containsError(logs); err != nil {
		t.Fatal(err)
	}

	if got, want := len(logs), ndays*3; got != want {
		t.Errorf("got %d, want %d", got, want)
	}

	adjustedTimes := slices.Clone(times)
	for i := 0; i < len(adjustedTimes); i++ {
		adjustedTimes[i] = adjustedTimes[i].Add(time.Millisecond * 10)
	}

	// Subtract 1 hour for standard to dayling saving time transition
	// since 2..3am doesn't exist on the day of the transition and the
	// on operation will be executed at what looks like 1am.
	adjustedTimes[2*3] = adjustedTimes[2*3].Add(-time.Hour)
	// Add 1 hour for dayling saving to standard time transition
	// since 1..2am occurs twice on the day of the transition
	// and the on operation will be executed at what looks like 3am.
	adjustedTimes[6*3] = adjustedTimes[6*3].Add(time.Hour)
	for i := 0; i < len(logs)/3; i++ {
		on := logs[i*3]
		another := logs[i*3+1]
		off := logs[i*3+2]
		if got, want := on.Due, adjustedTimes[i*3]; !got.Equal(want) {
			t.Errorf("%v: got %v, want %v", i, got, want)
		}
		if got, want := another.Due, adjustedTimes[i*3+1]; !got.Equal(want) {
			t.Errorf("%v: got %v, want %v", i, got, want)
		}
		if got, want := off.Due, adjustedTimes[i*3+2]; !got.Equal(want) {
			t.Errorf("%v: got %v, want %v", i, got, want)
		}

		if got, want := on.Due, all[i*3].when; !got.Equal(want) {
			t.Errorf("%v: got %v, want %v", i, got, want)
		}
		if got, want := another.Due, all[i*3+1].when; !got.Equal(want) {
			t.Errorf("%v: got %v, want %v", i, got, want)
		}
		if got, want := off.Due, all[i*3+2].when; !got.Equal(want) {
			t.Errorf("%v: got %v, want %v", i, got, want)
		}
	}

}
