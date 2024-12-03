// Copyright 2024 Cosmos Nicolaou. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package scheduler_test

import (
	"context"
	"fmt"
	"reflect"
	"slices"
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

func TestParseActions(t *testing.T) {
	ctx := context.Background()
	sys, err := devices.ParseSystemConfig(ctx, "", []byte(devices_config),
		devices.WithDevices(supportedDevices),
		devices.WithControllers(supportedControllers))
	if err != nil {
		t.Fatal(err)
	}

	scheds, err := scheduler.ParseConfig(ctx, []byte(schedule_config), sys)
	if err != nil {
		t.Fatal(err)
	}

	if got, want := len(scheds.Schedules), 17; got != want {
		t.Fatalf("got %d schedules, want %d", got, want)
	}

	simple := scheds.Lookup("simple")
	if got, want := simple.Name, "simple"; got != want {
		t.Fatalf("got %s, want %s", got, want)
	}
	if got, want := len(simple.Actions), 2; got != want {
		t.Fatalf("got %d actions, want %d", got, want)
	}

	if got, want := simple.Actions[0], (schedule.Action[scheduler.Daily]{
		Name: "on",
		Due:  datetime.NewTimeOfDay(0, 0, 1),
		Action: scheduler.Daily{
			Action: devices.Action{
				DeviceName: "device",
				Name:       "on",
			},
		},
	}); !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, want %#v", got, want)
	}

	if got, want := simple.Actions[1], (schedule.Action[scheduler.Daily]{
		Name: "off",
		Due:  datetime.NewTimeOfDay(0, 0, 2),
		Action: scheduler.Daily{
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

	if got, want := args.Actions[1], (schedule.Action[scheduler.Daily]{
		Name: "off",
		Due:  datetime.NewTimeOfDay(0, 0, 2),
		Action: scheduler.Daily{
			Action: devices.Action{
				DeviceName: "device",
				Name:       "off",
				Args:       []string{"3", "arg"},
			},
		},
	}); !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, want %#v", got, want)
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
	ctx := context.Background()
	sys, err := devices.ParseSystemConfig(ctx, "", []byte(devices_config),
		devices.WithDevices(supportedDevices),
		devices.WithControllers(supportedControllers))
	if err != nil {
		t.Fatal(err)
	}

	scheds, err := scheduler.ParseConfig(ctx, []byte(schedule_config), sys)
	if err != nil {
		t.Fatal(err)
	}

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
	ctx := context.Background()
	sys, err := devices.ParseSystemConfig(ctx, "", []byte(devices_config),
		devices.WithDevices(supportedDevices),
		devices.WithControllers(supportedControllers))
	if err != nil {
		t.Fatal(err)
	}

	scheds, err := scheduler.ParseConfig(ctx, []byte(schedule_config), sys)
	if err != nil {
		t.Fatal(err)
	}

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
			fmt.Printf("Failed: %v\n", tc.name)
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
	ctx := context.Background()
	sys, err := devices.ParseSystemConfig(ctx, "", []byte(devices_config),
		devices.WithDevices(supportedDevices),
		devices.WithControllers(supportedControllers))
	if err != nil {
		t.Fatal(err)
	}

	scheds, err := scheduler.ParseConfig(ctx, []byte(schedule_config), sys)
	if err != nil {
		t.Fatal(err)
	}

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
