// Copyright 2024 Cosmos Nicolaou. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package scheduler_test

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"slices"
	"sync"
	"testing"
	"time"

	"cloudeng.io/datetime"
	"cloudeng.io/errors"
	"github.com/cosnicolaou/automation/devices"
	"github.com/cosnicolaou/automation/internal"
	"github.com/cosnicolaou/automation/internal/testutil"
	"github.com/cosnicolaou/automation/scheduler"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

type slowDevice struct {
	testutil.MockDevice
	timeout time.Duration
	delay   time.Duration
}

func (st *slowDevice) Operations() map[string]devices.Operation {
	return map[string]devices.Operation{
		"on": st.On,
	}
}

func (st *slowDevice) SetConfig(cfg devices.DeviceConfigCommon) {
	st.MockDevice.SetConfig(cfg)
	st.DeviceConfigCommon.Timeout = st.timeout
}

func (st *slowDevice) On(context.Context, devices.OperationArgs) error {
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
func allActive(s *scheduler.Scheduler, year int, preDelay time.Duration) (actions []testAction, activeTimes, timeSourceTicks []time.Time) {
	cd := datetime.NewCalendarDate(year, 1, 1)
	for scheduled := range s.ScheduledYearEnd(cd) {
		for active := range scheduled.Active(s.Place()) {
			// create a time that is a little before the scheduled time
			// to more closely resemble a production setting. Note using
			// time.Date and then subtracting a millisecond is not sufficient
			// to test for DST handling, since time.Add does not handle
			// DST changes and as such is not the same as calling time.Now()
			// 10 milliseconds before the scheduled time.
			timeSourceTicks = append(timeSourceTicks, active.When.Add(-preDelay))
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

func (r *recorder) Logs(t *testing.T) []internal.LogEntry {
	entries := []internal.LogEntry{}
	for _, l := range bytes.Split(r.out.Bytes(), []byte("\n")) {
		if len(l) == 0 {
			continue
		}
		e, err := internal.ParseLogLine(string(l))
		if err != nil {
			t.Errorf("failed to parse: %v: %v", string(l), err)
		}
		if e.Msg != "completed" && e.Msg != "year-end" && e.Msg != "failed" {
			continue
		}
		entries = append(entries, e)
	}
	return entries
}

func containsError(logs []internal.LogEntry) error {
	for _, l := range logs {
		if l.Err != nil {
			return l.Err
		}
	}
	return nil
}

func newRecorder() *recorder {
	return &recorder{out: bytes.NewBuffer(nil)}
}

func setupSchedules(t *testing.T, loc string) (devices.System, scheduler.Schedules) {
	sys := createSystem(t, loc)
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

func appendYearEndTimesTicks(year int, loc *time.Location, times, ticks []time.Time) ([]time.Time, []time.Time) {
	last := time.Date(year, 12, 31, 23, 59, 59, int(time.Second)-1, loc)
	times = append(times, last)
	ticks = append(ticks, last.Add(-time.Millisecond*10))
	return times, ticks
}

func TestScheduler(t *testing.T) {
	ctx := context.Background()

	ts := &timesource{ch: make(chan time.Time, 1)}
	deviceRecorder, logRecorder, opts := newRecordersAndLogger(ts)
	sys, spec := setupSchedules(t, "Local")

	scheduler := createScheduler(t, sys, spec.Lookup("ranges"), opts...)

	year := 2021
	preDelay := time.Millisecond * 5
	all, times, ticks := allActive(scheduler, year, preDelay)
	_, ticks = appendYearEndTimesTicks(year, sys.Location.TimeLocation, times, ticks)
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

	if got, want := len(logs), (days*3)+1; got != want {
		t.Errorf("got %d, want %d", got, want)
	}

	for i := 0; i < len(logs)-1; i++ {
		if got, want := logs[i].Due, times[i]; !got.Equal(want) {
			t.Errorf("got %v, want %v", got, want)
		}
		if logs[i].YearEndDelay != 0 {
			t.Errorf("unexpected year end")
		}
		op := []string{"another", "on", "off"}[i%3]
		if got, want := logs[i].Op, op; got != want {
			t.Errorf("got %v, want %v", got, want)
		}
	}
	if logs[len(logs)-1].YearEndDelay == 0 {
		t.Errorf("missing year end delay")
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

	sys, spec := setupSchedules(t, "Local")

	now := time.Now().In(sys.Location.TimeLocation)
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

	preDelay := time.Millisecond * 5
	all, _, _ := allActive(scheduler, year, preDelay)
	cd := datetime.NewCalendarDate(year, 1, 1)

	var errs errors.M
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		errs.Append(scheduler.RunYear(ctx, cd))
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
		t.Fatalf("got %d, want %d", got, want)
	}
	if got, want := len(lines), len(all); got != want {
		t.Fatalf("got %d, want %d", got, want)
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

	sys, spec := setupSchedules(t, "Local")

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

		now := time.Now().In(sys.Location.TimeLocation)
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

		if err := scheduler.RunYear(ctx, cd); err != nil {
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

	sys, spec := setupSchedules(t, "Local")

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

	preDelay := time.Millisecond * 5
	all2023, times2023, ticks2023 := allActive(scheduler, 2023, preDelay)
	times2023, ticks2023 = appendYearEndTimesTicks(2023, sys.Location.TimeLocation, times2023, ticks2023)
	all2024, times2024, ticks2024 := allActive(scheduler, 2024, preDelay)
	times2024, ticks2024 = appendYearEndTimesTicks(2024, sys.Location.TimeLocation, times2024, ticks2024)
	times := append(append([]time.Time(nil), times2023...), times2024...)
	ticks := append(append([]time.Time(nil), ticks2023...), ticks2024...)

	all := append(append([]testAction(nil), all2023...), all2024...)
	if len(times) != len(all)+2 {
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
		if got, want := l.Due, times[i]; l.YearEndDelay == 0 && !got.Equal(want) {
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

	preDelay := time.Millisecond * 5
	for _, loc := range []string{"America/Los_Angeles", "Europe/London"} {
		ts := &timesource{ch: make(chan time.Time, 1)}
		deviceRecorder, logRecorder, opts := newRecordersAndLogger(ts)
		sys, spec := setupSchedules(t, loc)

		year := 2024

		scheduler := createScheduler(t, sys, spec.Lookup("daylight-saving-time"), opts...)

		all, times, ticks := allActive(scheduler, year, preDelay)
		times, ticks = appendYearEndTimesTicks(year, sys.Location.TimeLocation, times, ticks)
		runScheduler(ctx, t, scheduler, year, ts, ticks)

		// Make sure all operations were called despite the DST transitions.
		opsLines := deviceRecorder.Lines()
		ndays := 2 + 2 + 2 + 2
		if got, want := len(opsLines), (ndays * 3); got != want {
			t.Errorf("%v: got %d, want %d", loc, got, want)
		}

		for i := 0; i < len(opsLines); i++ {
			expected := []string{"device[device].On: [0] ",
				"device[device].Another: [2] arg1--arg2",
				"device[device].Off: [0] "}[i%3]
			if got, want := opsLines[i], expected; got != want {
				t.Errorf("%v: %v: got %v, want %v", loc, i, got, want)
			}
		}

		logs := logRecorder.Logs(t)
		if err := containsError(logs); err != nil {
			t.Fatal(err)
		}

		if got, want := len(logs), (ndays*3)+1; got != want {
			t.Errorf("%v: got %d, want %d", loc, got, want)
		}

		// Check that the due times are as consistent across the logs and the scheduler.
		for i := 0; i < len(logs)/3; i++ {
			on := logs[i*3]
			another := logs[i*3+1]
			off := logs[i*3+2]
			if got, want := on.Due, times[i*3]; !got.Equal(want) {
				t.Errorf("%v: %v: got %v, want %v", loc, i, got, want)
			}
			if got, want := another.Due, times[i*3+1]; !got.Equal(want) {
				t.Errorf("%v: %v: got %v, want %v", loc, i, got, want)
			}
			if got, want := off.Due, times[i*3+2]; !got.Equal(want) {
				t.Errorf("%v: %v: got %v, want %v", loc, i, got, want)
			}

			if got, want := on.Due, all[i*3].when; !got.Equal(want) {
				t.Errorf("%v: %v: got %v, want %v", loc, i, got, want)
			}
			if got, want := another.Due, all[i*3+1].when; !got.Equal(want) {
				t.Errorf("%v: %v: got %v, want %v", loc, i, got, want)
			}
			if got, want := off.Due, all[i*3+2].when; !got.Equal(want) {
				t.Errorf("%v: %v: got %v, want %v", loc, i, got, want)
			}
		}
	}
}

func operationsByDate(logs []internal.LogEntry) (
	days []datetime.Date,
	timesByDate map[datetime.Date]map[string][]time.Time,
	opsByName map[string][]int,
) {
	opsByDate := map[datetime.Date]map[string]int{}
	timesByDate = map[datetime.Date]map[string][]time.Time{}
	opsByName = map[string][]int{}
	for _, l := range logs {
		if l.YearEndDelay != 0 {
			break
		}
		date := datetime.DateFromTime(l.Due)
		if _, ok := opsByDate[date]; !ok {
			opsByDate[date] = map[string]int{}
			timesByDate[date] = map[string][]time.Time{}
		}
		opsByDate[date][l.Op]++
		timesByDate[date][l.Op] = append(timesByDate[date][l.Op], l.Started)
	}
	for day := range opsByDate {
		days = append(days, day)
	}
	slices.Sort(days)

	for _, day := range days {
		for op := range opsByDate[day] {
			opsByName[op] = append(opsByName[op], opsByDate[day][op])
		}
	}

	return
}

func applyDelta(base, delta []int) []int {
	result := make([]int, len(base))
	for i := range base {
		result[i] = base[i] + delta[i]
	}
	return result
}

func TestRepeats(t *testing.T) {
	ctx := context.Background()

	preDelay := time.Millisecond * 5

	// On is called once per day for both repeating and repeating-illdefined schedules
	baseOn := []int{1, 1, 1, 1, 1, 1, 1, 1}

	// Off is called 24 times per day for the repeating schedule.
	baseOff := []int{24, 24, 24, 24, 24, 24, 24, 24}
	// Another is called 68 times per day for the repeating schedule.
	anotherDuration := 21
	nAnother := (((24 * 60) - 13) / anotherDuration) + 1
	baseAnother := []int{nAnother, nAnother, nAnother, nAnother, nAnother, nAnother, nAnother, nAnother}

	// Off is called 23 times per day for the repeating-illdefined schedule
	baseOffIllDefined := []int{23, 23, 23, 23, 23, 23, 23, 23}
	// Another is called 66 times per day for the repeating-illdefined schedule.
	nAnotherIllDefined := (((24 * 60) - (60 + 13)) / anotherDuration) + 1
	baseAnotherIllDefined := []int{nAnotherIllDefined, nAnotherIllDefined, nAnotherIllDefined, nAnotherIllDefined, nAnotherIllDefined, nAnotherIllDefined, nAnotherIllDefined, nAnotherIllDefined}

	// The repeating-illdefined schedule has the repeats starting during
	// a DST transition (ie. 1AM to 2AM) whose treatment is not well defined by
	// the time.Date function. time.Date behaves differently
	// for America/Los_Angeles and Europe/London for example, 1:30AM
	// on the DST to ST transition is returned as 1:30AM PDT and 1:30AM GMT
	// respectively, ie. the timezone is set differently in each location
	// leading to a different number of invocations of an action depending
	// on the location. However, the interval between repeats is always correct
	// and the number of operations will be the same if the repeat starts
	// before, and traverses the transition. This latter point explains the
	// difference in behaviour between Los Angeles and London in that for
	// LA a time in the transition is returned as being before it, whereas
	// for London it is returned as being after the transition.
	// The different handling for each case is shown below
	//
	// time.Date(2024, 3, 10, 2, 59, 0, 0, America/Los_Angeles) -> 2024-03-10 01:59:00 -0800 PST (isdst: false)
	// time.Date(2024, 3, 10, 3, 0, 0, 0, America/Los_Angeles) -> 2024-03-10 03:00:00 -0700 PDT (isdst: true)
	//
	// time.Date(2024, 3, 31, 0, 59, 0, 0, Europe/London) -> 2024-03-31 00:59:00 +0000 GMT (isdst: false)
	// time.Date(2024, 3, 31, 1, 0, 0, 0, Europe/London) -> 2024-03-31 02:00:00 +0100 BST (isdst: true)
	//
	// time.Date(2024, 11, 3, 1, 59, 0, 0, America/Los_Angeles) -> 2024-11-03 01:59:00 -0700 PDT (isdst: true)
	// time.Date(2024, 11, 3, 2, 0, 0, 0, America/Los_Angeles) -> 2024-11-03 02:00:00 -0800 PST (isdst: false)
	//
	// time.Date(2024, 10, 27, 0, 59, 0, 0, Europe/London) -> 2024-10-27 00:59:00 +0100 BST (isdst: true)
	// time.Date(2024, 10, 27, 1, 0, 0, 0, Europe/London) -> 2024-10-27 01:00:00 +0000 GMT (isdst: false)

	// CA/UK spring 'repeating' schedules are as follows:
	// CA/UK: no-transition:     ... 01:37 01:58 02:19 02:40 03:01 03:22 03:43 ... 23:40
	// CA/UK: spring-transition: ... 01:37 01:58 ----------- 03:19 03:40 04:01 ... 23:48
	// CA/UK: fall-transition:   ... 01:37 01:58 01:19 01:40 02:01 02:22 02:43 03:04 ... 23:43
	//                                                    +++++++++++++++++
	// The transition loses the 2 repeats between 2am and 1am on 3/10 in the spring
	// gains 3 in the fall.

	// CA 'repeating-illdefined' schedules are as follows:
	// CA: no-transitions:    01:13 01:34 01:55 02:16 02:37 02:58 03:19 03:40 ... 23:58
	// CA: spring-transition: 01:13 01:34 01:55 ----------------- 03:16 03:37 ... 23:55
	// CA: fall-transition:   01:13 01:34 01:55 02:16 02:37 02:58 03:19 03:40 ... 23:58
	//                                                      +++++
	// The transition loses 3 in the spring and gains 1 in the fall.
	//
	// UK 'repeating-illdefined' schedules are as follows:
	// UK: no-transitions: same as CA, repeated for clarity
	// UK: no-transitions:    01:13 01:34 01:55 02:16 02:37 02:58 03:19 03:40 ... 23:58
	// UK: spring-transition: ----------------- 02:13 02:34 02:55 03:16 03:37 ... 23:55
	// UK: fall-transition:   01:13 01:34 01:55 02:16 02:37 02:58 03:19 03:40 ... 23:58
	// The transition loses 3 in the spring, but none in the fall since all of the repeats
	// fall past the transition as determined by time.Date.
	// The differences between the CA and UK are entirely due to how the time.Date function
	// handles the transition times. In all cases the internal is maintained correctly.
	// Similarly the 'off' operation for the UK loses 1 item in the spring, but gains
	// none in the fall.
	//
	// UTC does not have DST so the deltas are zero.

	springOnDelta, fallOnDelta := -1, 1
	springAnotherDelta, fallAnotherDelta := -2, 3
	springCADelta, fallCADelta := -3, 1
	springUKDelta, fallUKDelta := -3, 0

	offDeltaCA := []int{0, springOnDelta, 0, 0, 0, 0, 0, fallOnDelta}
	anotherDeltaCA := []int{0, springAnotherDelta, 0, 0, 0, 0, 0, fallAnotherDelta}
	// CA and UK have the same values, just in different positions corresponding
	// to the different dates.
	offDeltaUK := []int{0, 0, 0, springOnDelta, 0, fallOnDelta, 0, 0}
	anotherDeltaUK := []int{0, 0, 0, springAnotherDelta, 0, fallAnotherDelta, 0, 0}

	for i, tc := range []struct {
		loc                    string
		schedule               string
		baseOff, baseAnother   []int
		offDelta, anotherDelta []int
	}{
		{loc: "America/Los_Angeles", schedule: "repeating",
			baseOff: baseOff, baseAnother: baseAnother,
			offDelta:     offDeltaCA,
			anotherDelta: anotherDeltaCA},
		{loc: "Europe/London", schedule: "repeating",
			baseOff: baseOff, baseAnother: baseAnother,
			offDelta:     offDeltaUK,
			anotherDelta: anotherDeltaUK},
		{loc: "America/Los_Angeles", schedule: "repeating-illdefined",
			baseOff: baseOffIllDefined, baseAnother: baseAnotherIllDefined,
			offDelta:     offDeltaCA,
			anotherDelta: []int{0, springCADelta, 0, 0, 0, 0, 0, fallCADelta}},
		{loc: "Europe/London", schedule: "repeating-illdefined",
			baseOff: baseOffIllDefined, baseAnother: baseAnotherIllDefined,
			offDelta:     []int{0, 0, 0, -1, 0, 0, 0, 0},
			anotherDelta: []int{0, 0, 0, springUKDelta, 0, fallUKDelta, 0, 0}},
		{loc: "UTC", schedule: "repeating",
			baseOff: baseOff, baseAnother: baseAnother,
			offDelta:     []int{0, 0, 0, 0, 0, 0, 0, 0},
			anotherDelta: []int{0, 0, 0, 0, 0, 0, 0, 0}},
	} {
		ts := &timesource{ch: make(chan time.Time, 1)}
		_, logRecorder, opts := newRecordersAndLogger(ts)
		sys, spec := setupSchedules(t, tc.loc)

		if got, want := sys.Location.TimeLocation.String(), tc.loc; got != want {
			t.Fatalf("got %v, want %v", got, want)
		}

		scheduler := createScheduler(t, sys, spec.Lookup(tc.schedule), opts...)

		year := 2024
		all, _, ticks := allActive(scheduler, year, preDelay)
		_, ticks = appendYearEndTimesTicks(year, sys.Location.TimeLocation, nil, ticks)

		runScheduler(ctx, t, scheduler, year, ts, ticks)

		logs := logRecorder.Logs(t)
		if err := containsError(logs); err != nil {
			t.Fatal(err)
		}

		for i, l := range logs {
			if l.YearEndDelay != 0 {
				break
			}
			if got, want := l.Due, all[i].when; !got.Equal(want) {
				t.Errorf("%v: %v: got %v, want %v", tc.loc, tc.schedule, got, want)
			}
			if got, want := l.Started, ticks[i]; !got.Equal(want) {
				t.Errorf("%v: %v: got %v, want %v", tc.loc, tc.schedule, got, want)
			}
		}

		// Look at operations per day
		days, startedTimes, opsPerDay := operationsByDate(logs)

		// On is not affected by DST
		if got, want := opsPerDay["on"], baseOn; !slices.Equal(got, want) {
			t.Errorf("%v: %v: 'on': got %v, want %v", tc.loc, tc.schedule, got, want)
		}

		expectedOff := applyDelta(tc.baseOff, tc.offDelta)
		if got, want := opsPerDay["off"], expectedOff; !slices.Equal(got, want) {
			t.Errorf("%v: %v: 'off': got %v, want %v", tc.loc, tc.schedule, got, want)
		}

		expectedAnother := applyDelta(tc.baseAnother, tc.anotherDelta)
		if got, want := opsPerDay["another"], expectedAnother; !slices.Equal(got, want) {
			t.Errorf("%v: %v: 'another': got %v, want %v", tc.loc, tc.schedule, got, want)
		}

		// The intervals should always be the same.
		for _, day := range days {
			if got, want, ok := compareIntervals(startedTimes[day]["off"], time.Hour); !ok {
				t.Errorf("%v: %v: %v: got %v, want %v", tc.loc, tc.schedule, day, got, want)
			}
			if got, want, ok := compareIntervals(startedTimes[day]["another"], time.Minute*time.Duration(anotherDuration)); !ok {
				t.Errorf("%v: %v: %v: got %v, want %v", tc.loc, tc.schedule, day, got, want)
			}
		}
	}
}

func compareIntervals(times []time.Time, repeat time.Duration) (got, want time.Duration, ok bool) {
	p := times[0]
	for _, c := range times[1:] {
		if got, want := c.Sub(p), repeat; got != want {
			return got, want, false
		}
		p = c
	}
	return 0, 0, true
}

func TestRepeatsBounded(t *testing.T) {
	ctx := context.Background()

	preDelay := time.Millisecond * 5
	ts := &timesource{ch: make(chan time.Time, 1)}
	_, logRecorder, opts := newRecordersAndLogger(ts)
	sys, spec := setupSchedules(t, "Local")

	scheduler := createScheduler(t, sys, spec.Lookup("repeating-bounded"), opts...)

	year := 2024
	all, _, ticks := allActive(scheduler, year, preDelay)
	_, ticks = appendYearEndTimesTicks(year, sys.Location.TimeLocation, nil, ticks)
	runScheduler(ctx, t, scheduler, year, ts, ticks)

	logs := logRecorder.Logs(t)
	if err := containsError(logs); err != nil {
		t.Fatal(err)
	}

	for i, l := range logs {
		if l.YearEndDelay != 0 {
			break
		}
		if got, want := l.Due, all[i].when; !got.Equal(want) {
			t.Errorf("got %v, want %v", got, want)
		}
		if got, want := l.Started, ticks[i]; !got.Equal(want) {
			t.Errorf("got %v, want %v", got, want)
		}
	}

	// Look at operations per day
	days, startedTimes, opsPerDay := operationsByDate(logs)

	// One 'on' operation per day
	if got, want := opsPerDay["on"], []int{1, 1, 1, 1, 1, 1, 1, 1}; !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}

	// Five 'off' operations per day, since num_repeats is set to 4.
	if got, want := opsPerDay["off"], []int{5, 5, 5, 5, 5, 5, 5, 5}; !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}

	// The intervals should always be the same.
	for _, day := range days {
		prevNow := startedTimes[day]["off"][0]
		for _, cur := range startedTimes[day]["off"][1:] {
			if got, want := cur.Sub(prevNow), time.Minute*30; got != want {
				t.Errorf("%v: %v: got %v, want %v", prevNow, cur, got, want)
			}
			prevNow = cur
		}
	}
}
