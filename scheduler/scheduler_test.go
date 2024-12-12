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
	action scheduler.Action
}

// allActive returns all the active actions for the specified year and place along
// with the times at which they are scheduled to run and the times that the
// fake time source should be advanced to in order to trigger the execution
// of the scheduler
func allActive(s *scheduler.Scheduler, year int) (actions []testAction, activeTimes, timeSourceTicks []time.Time) {
	cd := datetime.NewCalendarDate(year, 1, 1)
	for scheduled := range s.ScheduledYearEnd(cd) {
		for active := range scheduled.Active(s.Place()) {
			// create a time that is a little before the scheduled time
			// to more closely resemble a production setting. Note using
			// time.Date and then subtracting a millisecond is not sufficient
			// to test for DST handling, since time.Add does not handle
			// DST changes and as such is not the same as calling time.Now()
			// 10 milliseconds before the scheduled time.
			//dt := subMilliseconds(yp, active.Date.Date(), active.When)
			timeSourceTicks = append(timeSourceTicks, active.When.Add(-time.Millisecond*10))
			activeTimes = append(activeTimes, active.When)
			actions = append(actions, testAction{
				action: active.T,
				when:   active.When,
			})
		}
	}
	return
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
	Now        time.Time `json:"now"`
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

func setupSchedules(t *testing.T) (devices.System, scheduler.Schedules) {
	sys := createSystem(t)
	spec := createSchedules(t, sys)
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

func runScheduler(ctx context.Context, t *testing.T, scheduler *scheduler.Scheduler, year int, ts *timesource, times []time.Time) {
	cd := datetime.NewCalendarDate(year, 1, 1)
	var errs errors.M
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		errs.Append(scheduler.RunYearEnd(ctx, cd))
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

func createScheduler(t *testing.T, sys devices.System, schedule scheduler.Annual, opts ...scheduler.Option) *scheduler.Scheduler {
	scheduler, err := scheduler.New(schedule, sys, opts...)
	if err != nil {
		t.Fatal(err)
	}
	return scheduler
}

func TestScheduler(t *testing.T) {
	ctx := context.Background()

	ts := &timesource{ch: make(chan time.Time, 1)}
	deviceRecorder, logRecorder, opts := newRecordersAndLogger(ts)
	sys, spec := setupSchedules(t)

	scheduler := createScheduler(t, sys, spec.Lookup("ranges"), opts...)

	year := 2021
	all, times, ticks := allActive(scheduler, year)
	runScheduler(ctx, t, scheduler, year, ts, ticks)

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
		if got, want := lg1.Due, times[i*3]; !got.Equal(want) {
			t.Errorf("%#v: got %v, want %v", lg1, got, want)
		}
		if got, want := lg2.Due, times[i*3+1]; !got.Equal(want) {
			t.Errorf("%#v: got %v, want %v", lg2, got, want)
		}
		if got, want := lg3.Due, times[i*3+2]; !got.Equal(want) {
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
	year := time.Now().Year()

	sys, spec := setupSchedules(t)

	now := time.Now().In(sys.Location.TZ)
	today := datetime.DateFromTime(now)
	sched := spec.Lookup("simple")
	sched.Dates.Ranges = []datetime.DateRange{datetime.NewDateRange(today, today)}

	sched.DailyActions[0].Due = datetime.TimeOfDayFromTime(now.Add(time.Second))
	sched.DailyActions[1].Due = datetime.TimeOfDayFromTime(now.Add(2 * time.Second))

	deviceRecorder := newRecorder()
	logRecorder := newRecorder()
	logger := slog.New(slog.NewJSONHandler(logRecorder, nil))

	opts := []scheduler.Option{
		scheduler.WithLogger(logger),
		scheduler.WithOperationWriter(deviceRecorder)}

	scheduler := createScheduler(t, sys, sched, opts...)

	all, _, _ := allActive(scheduler, year)
	cd := datetime.NewCalendarDate(year, 1, 1)

	var errs errors.M
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		errs.Append(scheduler.RunYearEnd(ctx, cd))
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
	year := time.Now().Year()

	sys, spec := setupSchedules(t)

	logRecorder := newRecorder()
	logger := slog.New(slog.NewJSONHandler(logRecorder, nil))

	cd := datetime.NewCalendarDate(year, 1, 1)
	for _, tc := range []struct {
		sched  string
		cancel bool
		errmsg string
	}{
		{"slow", false, "context deadline exceeded"},   // timeout
		{"hanging", true, "context deadline exceeded"}, // hanging, must be canceled
	} {
		ctx, cancel := context.WithCancel(ctx)

		now := time.Now().In(sys.Location.TZ)
		today := datetime.DateFromTime(now)
		sched := spec.Lookup(tc.sched) // slow device schedule
		sched.Dates.Ranges = []datetime.DateRange{datetime.NewDateRange(today, today)}

		sched.DailyActions[0].Due = datetime.TimeOfDayFromTime(now.Add(time.Second))

		opts := []scheduler.Option{scheduler.WithLogger(logger)}
		scheduler := createScheduler(t, sys, sched, opts...)
		if tc.cancel {
			go func() {
				time.Sleep(time.Second)
				cancel()
			}()
		}

		if err := scheduler.RunYearEnd(ctx, cd); err != nil {
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

	sys, spec := setupSchedules(t)

	ts := &timesource{ch: make(chan time.Time, 1)}
	deviceRecorder := newRecorder()
	logRecorder := newRecorder()
	logger := slog.New(slog.NewJSONHandler(logRecorder, nil))
	opts := []scheduler.Option{
		scheduler.WithTimeSource(ts),
		scheduler.WithOperationWriter(deviceRecorder),
		scheduler.WithLogger(logger),
	}

	scheduler := createScheduler(t, sys, spec.Lookup("multi-year"), opts...)

	all2023, times2023, ticks2023 := allActive(scheduler, 2023)
	all2024, times2024, ticks2024 := allActive(scheduler, 2024)
	times := append(times2023, times2024...)
	ticks := append(ticks2023, ticks2024...)

	all := append(all2023, all2024...)
	if len(times) != len(all) {
		t.Fatalf("mismatch: %v %v", len(times), len(all))
	}

	var errs errors.M
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		errs.Append(scheduler.RunYearEnd(ctx, datetime.NewCalendarDate(2023, 1, 1)))
		errs.Append(scheduler.RunYearEnd(ctx, datetime.NewCalendarDate(2024, 1, 1)))
		wg.Done()
	}()
	for _, t := range ticks {
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
		if got, want := l.Due, times[i]; !got.Equal(want) {
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
	sys, spec := setupSchedules(t)

	year := 2024

	scheduler := createScheduler(t, sys, spec.Lookup("daylight-saving-time"), opts...)

	all, times, ticks := allActive(scheduler, year)
	runScheduler(ctx, t, scheduler, year, ts, ticks)

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

	// Check that the due times are as consistent across the logs and the scheduler.
	for i := 0; i < len(logs)/3; i++ {
		on := logs[i*3]
		another := logs[i*3+1]
		off := logs[i*3+2]
		if got, want := on.Due, times[i*3]; !got.Equal(want) {
			t.Errorf("%v: got %v, want %v", i, got, want)
		}
		if got, want := another.Due, times[i*3+1]; !got.Equal(want) {
			t.Errorf("%v: got %v, want %v", i, got, want)
		}
		if got, want := off.Due, times[i*3+2]; !got.Equal(want) {
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

func TestRepeats(t *testing.T) {
	ctx := context.Background()

	ts := &timesource{ch: make(chan time.Time, 1)}
	_, logRecorder, opts := newRecordersAndLogger(ts)
	sys, spec := setupSchedules(t)

	scheduler := createScheduler(t, sys, spec.Lookup("repeating"), opts...)

	year := 2024
	//	ndays := 2 + 2
	all, _, ticks := allActive(scheduler, year)

	runScheduler(ctx, t, scheduler, year, ts, ticks)

	logs := logRecorder.Logs(t)
	if err := containsError(logs); err != nil {
		t.Fatal(err)
	}

	// Look at operations per day
	nops := map[datetime.Date]map[string]int{}
	nowtimes := map[datetime.Date]map[string][]time.Time{}

	for i, l := range logs {
		date := datetime.DateFromTime(l.Due)
		if _, ok := nops[date]; !ok {
			nops[date] = map[string]int{}
			nowtimes[date] = map[string][]time.Time{}
		}
		nops[date][l.Op]++
		nowtimes[date][l.Op] = append(nowtimes[date][l.Op], l.Now)

		if got, want := l.Due, all[i].when; !got.Equal(want) {
			t.Errorf("got %v, want %v", got, want)
		}
		if got, want := l.Now, ticks[i]; !got.Equal(want) {
			t.Errorf("got %v, want %v", got, want)
		}
	}

	days := []datetime.Date{}
	for day := range nops {
		days = append(days, day)
	}
	slices.Sort(days)
	perdayOff := []int{}
	perdayOn := []int{}
	anotherDay := []int{}
	for _, day := range days {
		perdayOn = append(perdayOn, nops[day]["on"])
		perdayOff = append(perdayOff, nops[day]["off"])
		anotherDay = append(anotherDay, nops[day]["another"])
	}

	// add repeats for 'off' operations, per day.
	expectedOff := 23 // once per hour starting at 1am
	// 23 repeast on a normal day, 22 on ST-DST and 24 on ST-DST.
	expectedOffPerday := []int{expectedOff, expectedOff - 1, expectedOff, expectedOff + 1}

	expectedAnother := ((24*60*60)-((60+14)*60))/(13*60) + 1

	// 5 repeats between 1am and 2am on 3/10 are lost
	// 5 repeats between 1am and 2am on 11/3 are gained, but there is one less
	//   repeat at the end of the day.
	expectedAnotherPerday := []int{expectedAnother, expectedAnother - 5, expectedAnother, expectedAnother + 5 - 1}

	// One 'on' operation per day
	if got, want := perdayOn, []int{1, 1, 1, 1}; !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}

	if got, want := perdayOff, expectedOffPerday; !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}

	if got, want := anotherDay, expectedAnotherPerday; !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}

	// The intervals should always be the same.
	for _, day := range days {
		prevNow := nowtimes[day]["off"][0]
		for _, cur := range nowtimes[day]["off"][1:] {
			if got, want := cur.Sub(prevNow), time.Hour; got != want {
				t.Errorf("%v: %v: got %v, want %v", prevNow, cur, got, want)
			}
			prevNow = cur
		}
		prevAnother := nowtimes[day]["another"][0]
		for _, cur := range nowtimes[day]["another"][1:] {
			if got, want := cur.Sub(prevAnother), time.Minute*13; got != want {
				t.Errorf("%v: %v: got %v, want %v", prevAnother, cur, got, want)
			}
			prevAnother = cur
		}
	}

}
