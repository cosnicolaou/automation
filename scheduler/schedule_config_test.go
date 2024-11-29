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
    actions_with_args:
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
    actions_with_args:
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
    actions_with_args:
      - action: d
        when: 12:00

  - name: order-3
    <<: *abc
    actions_with_args:
      - action: d
        before: a
        when: 12:00

  - name: order-4
    <<: *abc
    actions_with_args:
      - action: d
        after: a
        when: 12:00

  - name: order-5
    <<: *abc
    actions_with_args:
      - action: d
        after: c
        when: 12:00

  - name: order-6
    <<: *abc
    actions_with_args:
      - action: d
        before: c
        when: 12:00

  - name: dynamic
    for: jan,feb
    ranges:
      - summer
      - winter
    actions:
      on: sunrise
	  off: sunset	
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

	if got, want := len(scheds.Schedules), 15; got != want {
		t.Fatalf("got %d schedules, want %d", got, want)
	}

	simple := scheds.Lookup("simple")
	if got, want := simple.Name, "simple"; got != want {
		t.Fatalf("got %s, want %s", got, want)
	}
	if got, want := len(simple.Actions), 2; got != want {
		t.Fatalf("got %d actions, want %d", got, want)
	}

	if got, want := simple.Actions[0], (schedule.Action[devices.Action]{
		Name: "on",
		Due:  datetime.NewTimeOfDay(0, 0, 1),
		Action: devices.Action{
			DeviceName: "device",
			ActionName: "on",
		},
	}); !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, want %#v", got, want)
	}

	if got, want := simple.Actions[1], (schedule.Action[devices.Action]{
		Name: "off",
		Due:  datetime.NewTimeOfDay(0, 0, 2),
		Action: devices.Action{
			DeviceName: "device",
			ActionName: "off",
		},
	}); !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, want %#v", got, want)
	}

	args := scheds.Lookup("simple_args")
	if got, want := len(args.Actions), 2; got != want {
		t.Fatalf("got %d actions, want %d", got, want)
	}

	if got, want := args.Actions[1], (schedule.Action[devices.Action]{
		Name: "off",
		Due:  datetime.NewTimeOfDay(0, 0, 2),
		Action: devices.Action{
			DeviceName: "device",
			ActionName: "off",
			ActionArgs: []string{"3", "arg"},
		},
	}); !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, want %#v", got, want)
	}

}

func scheduledTimes(t *testing.T, scheds scheduler.Schedules, sys devices.System, year int, name string) []time.Time {
	times := []time.Time{}
	sr, err := scheduler.New(scheds.Lookup(name), sys)
	if err != nil {
		t.Fatal(err)
	}
	yp := datetime.YearAndPlace{
		Year:  year,
		Place: sys.Location,
	}

	for active := range sr.Scheduled(yp) {
		times = append(times, datetime.Time(yp, active.Date, active.Actions[0].Due))
	}
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
	if got, want := len(scheduled), (31+29)*2; got != want {
		t.Errorf("got %d, want %d", got, want)
	}

	scheduled = scheduledTimes(t, scheds, sys, 2023, "months")
	if got, want := len(scheduled), (31+28)*2; got != want {
		t.Errorf("got %d, want %d", got, want)
	}

	// Jan and Feb with two days missing
	scheduled = scheduledTimes(t, scheds, sys, 2024, "exlusions")
	if got, want := len(scheduled), (31+29-2)*2; got != want {
		t.Errorf("got %d, want %d", got, want)
	}

	scheduled = scheduledTimes(t, scheds, sys, 2023, "exlusions")
	if got, want := len(scheduled), (31+29-3)*2; got != want {
		t.Errorf("got %d, want %d", got, want)
	}

	// 01/22:2, 11/22:12/28 translates to:
	// 10+28+9+28 days

	scheduled = scheduledTimes(t, scheds, sys, 2024, "ranges")
	if got, want := len(scheduled), (10+29+9+28)*2; got != want {
		t.Errorf("got %d, want %d", got, want)
	}

	scheduled = scheduledTimes(t, scheds, sys, 2023, "ranges")
	if got, want := len(scheduled), (10+28+9+28)*2; got != want {
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

	for i, tc := range []struct {
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
	} {
		if i != 3 {
			//continue
		}
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

