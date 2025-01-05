// Copyright 2024 Cosmos Nicolaou. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package scheduler_test

import (
	"context"
	"slices"
	"testing"

	"cloudeng.io/datetime"
	"github.com/cosnicolaou/automation/devices"
	"github.com/cosnicolaou/automation/scheduler"
)

func TestParseAnnualDynamic(t *testing.T) {
	ctx := context.Background()
	sys, err := devices.ParseSystemConfig(ctx, []byte(devices_config),
		devices.WithDevices(supportedDevices),
		devices.WithControllers(supportedControllers))
	if err != nil {
		t.Fatal(err)
	}

	scheds, err := scheduler.ParseConfig(ctx, []byte(scheduleConfigSample), sys)
	if err != nil {
		t.Fatal(err)
	}

	sched := scheds.Lookup("dynamic")

	if got, want := len(sched.Dates.Dynamic), 2; got != want {
		t.Fatalf("got %d, want %d", got, want)
	}

	dr := sched.Dates.EvaluateDateRanges(2024, datetime.DateRangeYear())
	if got, want := len(dr), 3; got != want {
		t.Fatalf("got %d, want %d", got, want)
	}

	nd := datetime.NewDate
	ndr := datetime.NewDateRange

	if got, want := dr, (datetime.DateRangeList{
		ndr(nd(2, 1), nd(2, 29)),
		ndr(nd(6, 20), nd(9, 22)),
		ndr(nd(12, 21), nd(12, 31)),
	}); !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}
