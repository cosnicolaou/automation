// Copyright 2024 Cosmos Nicolaou. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package scheduler_test

import (
	"context"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"cloudeng.io/datetime"
	"cloudeng.io/datetime/schedule"
	"cloudeng.io/geospatial/astronomy"
	"github.com/cosnicolaou/automation/devices"
	"github.com/cosnicolaou/automation/internal/testutil"
	"github.com/cosnicolaou/automation/scheduler"
)

const devices_config = `
location: Local
devices:
  - name: device
    type: device
  - name: slow
    type: slow_device
  - name: hanging
    type: hanging_device
`

const schedule_config = `
shared:
  abc: &abc
    device: device
    actions:
      a: 12:00
      b: 12:00
      c: 12:00

schedules:
  - name: simple
    device: device
    actions:
      on: 00:00:01
      off: 00:00:02
  - name: simple_args
    device: device
    actions:
      on: 00:00:01
    actions_detailed:
      - action: off
        when: 00:00:02
        args: ["3", arg]

  - name: slow
    device: slow
    actions:
      on: 00:00:01

  - name: hanging
    device: hanging
    actions:
      on: 00:00:01

  - name: multi-year
    device: device
    actions:
      on: 00:00:01
    ranges:
      - 02/25:02

  - name: months
    for: jan, feb
    actions:
      on: 08:12
      off: 20:01:13
    device: device

  - name: exlusions
    weekdays: true
    for: jan,feb
    exclude_dates: jan-02, feb-02
    actions:
      on: 12:00
      off: 16:00
    device: device

  - name: ranges
    ranges:
       - 01/22:2
       - 11/22:12/28
    actions_detailed:
      - action: another
        when: 12:00
        before: on
        args: ["arg1", "arg2"]
    actions:
      on: 12:00
      off: 16:00
    device: device

  - name: order-1 
    <<: *abc

  - name: order-2
    <<: *abc
    actions_detailed:
      - action: d
        when: 12:00

  - name: order-3
    <<: *abc
    actions_detailed:
      - action: d
        before: a
        when: 12:00

  - name: order-4
    <<: *abc
    actions_detailed:
      - action: d
        after: a
        when: 12:00

  - name: order-5
    <<: *abc
    actions_detailed:
      - action: d
        after: c
        when: 12:00

  - name: order-6
    <<: *abc
    actions_detailed:
      - action: d
        before: c
        when: 12:00

  - name: order-7
    <<: *abc
    actions_detailed:
      - action: d
        when: sunset

  - name: dynamic
    device: device
    for: feb
    ranges:
      - summer
      - winter
    actions:
      on: sunrise-30m
      off: 5:00
   
  - name: daylight-saving-time
    device: device
    ranges: # California DST dates for 2024 are March 10 and November 3.
       - 03/08:03/11
       - 11/01:11/03
    actions:
       on: 2:00
       off: 3:00
    actions_detailed:
      - action: another
        when: 2:30
        args: ["arg1", "arg2"]

  - name: multi-time
    device: device
    actions:
      on: 00:00:01,00:01:00
      off: 00:00:02,00:02:00

  - name: repeating
    device: device
    ranges:
       - 03/09:03/10
       - 11/02:11/03
    actions:
      on: 00:00:01
    actions_detailed:
      - action: off
        when: 01:00:00
        repeat: 1h
      - action: another
        when: 01:13:00
        repeat: 13m

  - name: repeating-bounded
    device: device
    ranges:
       - 03/09:03/10
       - 11/02:11/03
    actions:
      on: 00:00:01
    actions_detailed:
      - action: off
        when: 01:0:00
        repeat: 30m
        num_repeats: 4
`

var supportedDevices = devices.SupportedDevices{
	"device": func(string, devices.Options) (devices.Device, error) {
		return &testutil.MockDevice{}, nil
	},
	"slow_device": func(string, devices.Options) (devices.Device, error) {
		return &slow_test_device{
			timeout: time.Millisecond * 10,
			delay:   time.Minute,
		}, nil
	},
	"hanging_device": func(string, devices.Options) (devices.Device, error) {
		return &slow_test_device{
			timeout: time.Hour,
			delay:   time.Hour,
		}, nil
	},
}

var supportedControllers = devices.SupportedControllers{
	"controller": func(string, devices.Options) (devices.Controller, error) {
		return &testutil.MockController{}, nil
	},
}

func createSystem(t *testing.T) devices.System {
	sys, err := devices.ParseSystemConfig(context.Background(), "", []byte(devices_config),
		devices.WithDevices(supportedDevices),
		devices.WithControllers(supportedControllers))
	if err != nil {
		t.Fatal(err)
	}
	return sys
}

func createSchedules(t *testing.T, sys devices.System) scheduler.Schedules {
	ctx := context.Background()
	scheds, err := scheduler.ParseConfig(ctx, []byte(schedule_config), sys)
	if err != nil {
		t.Fatal(err)
	}
	return scheds
}

func TestParseActions(t *testing.T) {
	sys := createSystem(t)
	scheds := createSchedules(t, sys)

	if got, want := len(scheds.Schedules), 20; got != want {
		t.Fatalf("got %d schedules, want %d", got, want)
	}

	simple := scheds.Lookup("simple")
	if got, want := simple.Name, "simple"; got != want {
		t.Fatalf("got %s, want %s", got, want)
	}
	if got, want := len(simple.Actions), 2; got != want {
		t.Fatalf("got %d actions, want %d", got, want)
	}

	if got, want := simple.Actions[0], (schedule.Action[scheduler.Action]{
		Name: "on",
		Due:  datetime.NewTimeOfDay(0, 0, 1),
		Action: scheduler.Action{
			Action: devices.Action{
				DeviceName: "device",
				Name:       "on",
			},
		},
	}); !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, want %#v", got, want)
	}

	if got, want := simple.Actions[1], (schedule.Action[scheduler.Action]{
		Name: "off",
		Due:  datetime.NewTimeOfDay(0, 0, 2),
		Action: scheduler.Action{
			Action: devices.Action{
				DeviceName: "device",
				Name:       "off",
			},
		},
	}); !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, want %#v", got, want)
	}

	args := scheds.Lookup("simple_args")
	if got, want := len(args.Actions), 2; got != want {
		t.Fatalf("got %d actions, want %d", got, want)
	}

	if got, want := args.Actions[1], (schedule.Action[scheduler.Action]{
		Name: "off",
		Due:  datetime.NewTimeOfDay(0, 0, 2),
		Action: scheduler.Action{
			Action: devices.Action{
				DeviceName: "device",
				Name:       "off",
				Args:       []string{"3", "arg"},
			},
		},
	}); !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, want %#v", got, want)
	}

	multi := scheds.Lookup("multi-time")
	if got, want := len(multi.Actions), 4; got != want {
		t.Fatalf("got %d actions, want %d", got, want)
	}

	if got, want := multi.Actions, (schedule.Actions[scheduler.Action]{
		{Name: "on",
			Due: datetime.NewTimeOfDay(0, 0, 1),
			Action: scheduler.Action{
				Action: devices.Action{
					DeviceName: "device",
					Name:       "on",
				},
			}},
		{Name: "off",
			Due: datetime.NewTimeOfDay(0, 0, 2),
			Action: scheduler.Action{
				Action: devices.Action{
					DeviceName: "device",
					Name:       "off",
				},
			}},
		{Name: "on",
			Due: datetime.NewTimeOfDay(0, 1, 0),
			Action: scheduler.Action{
				Action: devices.Action{
					DeviceName: "device",
					Name:       "on",
				},
			}},
		{Name: "off",
			Due: datetime.NewTimeOfDay(0, 2, 0),
			Action: scheduler.Action{
				Action: devices.Action{
					DeviceName: "device",
					Name:       "off",
				},
			},
		}}); !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, want %#v", got, want)
	}

	repeat := scheds.Lookup("repeating")
	if got, want := len(repeat.Actions), 2; got != want {
		t.Fatalf("got %d actions, want %d", got, want)
	}

	if got, want := repeat.Actions[1], (schedule.Action[scheduler.Action]{Name: "off",
		Due: datetime.NewTimeOfDay(1, 0, 0),
		Action: scheduler.Action{
			Action: devices.Action{
				DeviceName: "device",
				Name:       "off",
			},
			Repeat: time.Hour,
		},
	}); !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func scheduledActions(t *testing.T, scheds scheduler.Schedules, sys devices.System, year int, name string) ([]time.Time, []datetime.Date) {
	s := scheds.Lookup(name)
	sr, err := scheduler.New(s, sys)
	if err != nil {
		t.Fatal(err)
	}
	yp := datetime.YearAndPlace{
		Year:  year,
		Place: sys.Location,
	}
	times := []time.Time{}
	dates := []datetime.Date{}
	for active := range sr.Scheduled(yp) {
		times = append(times, datetime.Time(yp, active.Date, active.Actions[0].Due))
		dates = append(dates, active.Date)
	}
	return times, dates
}

func scheduledTimes(t *testing.T, scheds scheduler.Schedules, sys devices.System, year int, name string) []time.Time {
	times, _ := scheduledActions(t, scheds, sys, year, name)
	return times
}

func TestParseSchedules(t *testing.T) {
	sys := createSystem(t)
	scheds := createSchedules(t, sys)

	scheduled := scheduledTimes(t, scheds, sys, 2024, "simple")
	if got, want := len(scheduled), 0; got != want {
		t.Errorf("got %d, want %d", got, want)
	}

	// Jan and Feb, *2 for two unique times.
	scheduled = scheduledTimes(t, scheds, sys, 2024, "months")
	if got, want := len(scheduled), (31 + 29); got != want {
		t.Errorf("got %d, want %d", got, want)
	}

	scheduled = scheduledTimes(t, scheds, sys, 2023, "months")
	if got, want := len(scheduled), (31 + 28); got != want {
		t.Errorf("got %d, want %d", got, want)
	}

	// Jan and Feb with two days missing
	scheduled = scheduledTimes(t, scheds, sys, 2024, "exlusions")
	if got, want := len(scheduled), (31 + 29 - 2); got != want {
		t.Errorf("got %d, want %d", got, want)
	}

	scheduled = scheduledTimes(t, scheds, sys, 2023, "exlusions")
	if got, want := len(scheduled), (31 + 29 - 3); got != want {
		t.Errorf("got %d, want %d", got, want)
	}

	// 01/22:2, 11/22:12/28 translates to:
	// 10+28+9+28 days

	scheduled = scheduledTimes(t, scheds, sys, 2024, "ranges")
	if got, want := len(scheduled), (10 + 29 + 9 + 28); got != want {
		t.Errorf("got %d, want %d", got, want)
	}

	scheduled = scheduledTimes(t, scheds, sys, 2023, "ranges")
	if got, want := len(scheduled), (10 + 28 + 9 + 28); got != want {
		t.Errorf("got %d, want %d", got, want)
	}

}

func TestParseOperationOrder(t *testing.T) {
	sys := createSystem(t)
	scheds := createSchedules(t, sys)

	for _, tc := range []struct {
		name     string
		expected []string
	}{
		{"ranges", []string{"another", "on", "off"}},
		{"order-1", []string{"a", "b", "c"}},
		{"order-2", []string{"a", "b", "c", "d"}},
		{"order-3", []string{"d", "a", "b", "c"}},
		{"order-4", []string{"a", "d", "b", "c"}},
		{"order-5", []string{"a", "b", "c", "d"}},
		{"order-6", []string{"a", "b", "d", "c"}},
		{"order-7", []string{"d", "a", "b", "c"}},
	} {
		sched := scheds.Lookup(tc.name)
		names := []string{}
		for _, sched := range sched.Actions {
			names = append(names, sched.Name)
		}
		if got, want := names, tc.expected; !slices.Equal(got, want) {
			t.Errorf("%v: got %v, want %v", tc.name, got, want)
		}
	}
}

func datesForRange(year int, dr datetime.DateRangeList) []datetime.Date {
	dates := []datetime.Date{}
	for _, r := range dr {
		for d := range r.Dates(year) {
			dates = append(dates, d.Date())
		}
	}
	return dates
}

func TestDynamic(t *testing.T) {
	sys := createSystem(t)
	scheds := createSchedules(t, sys)

	nd := datetime.NewDate
	ndr := datetime.NewDateRange

	times, dates := scheduledActions(t, scheds, sys, 2024, "dynamic")

	summer := astronomy.Summer{}.Evaluate(2024)
	winter := astronomy.Winter{}.Evaluate(2024)

	expected := datesForRange(2024,
		datetime.DateRangeList{
			ndr(summer.From().Date(), summer.To().Date()),
			ndr(nd(2, 1), nd(2, 29)),
			ndr(winter.From().Date(), nd(12, 31)),
		})
	slices.Sort(expected)
	if got, want := dates, expected; !slices.Equal(got, want) {
		t.Errorf("dates: got %v, want %v", got, want)
	}

	// Dynamic times will be 00:00:00 when returned for each day in the schedule.
	for _, when := range times {
		if when.Hour() != 0 || when.Minute() != 0 || when.Second() != 0 {
			t.Errorf("time hours, mins and seconds %v should be zero", when)
		}
	}
}

const (
	empty = `
schedules:
  - name: simple
    device: device
    actions:`
	bad_times = `
schedules:
  - name: simple
    device: device
    actions:
       on: 00:xx:01
`
	bad_op = `
schedules:
  - name: simple
    device: device
    actions:
    actions_detailed:
      - action: off
        when: 00:00:02
        before: foo
`
	diff_time = `
schedules:
  - name: simple
    device: device
    actions:
      on: 00:00:01
    actions_detailed:
      - action: off
        when: 00:00:02
        before: on
`
	both_before_and_after = `
schedules:
  - name: simple
    device: device
    actions:
      on: 00:00:01
    actions_detailed:
      - action: off
        when: 00:00:02
        before: on
        after: on
`
	refer_to_self = `
schedules:
  - name: simple
    device: device
    actions:
      on: 00:00:01
    actions_detailed:
      - action: off
        when: 00:00:02
        before: off
`

	repeat_zero = `
schedules:
  - name: simple
    device: device
    actions_detailed:
      - action: off
        repeat: 0s
`
)

func TestValidation(t *testing.T) {
	ctx := context.Background()
	sys := createSystem(t)
	for _, tc := range []struct {
		cfg string
		err string
	}{
		{empty, "no actions defined"},
		{bad_times, "invalid time"},
		{bad_op, "foo not found"},
		{diff_time, "not scheduled for the same time"},
		{both_before_and_after, "cannot have both before and after"},
		{refer_to_self, "cannot be before or after itself"},
		{repeat_zero, "repeat duration must be greater than zero"},
	} {
		_, err := scheduler.ParseConfig(ctx, []byte(tc.cfg), sys)
		if err == nil || !strings.Contains(err.Error(), tc.err) {
			t.Errorf("missing or unexpected error: %v, not %v", err, tc.err)
		}
	}

}

func TestMultiAndRepeat(t *testing.T) {

	// impossible repeat values
	// make sure it stops correctly, even on DST transitions
	t.Fail()
}
