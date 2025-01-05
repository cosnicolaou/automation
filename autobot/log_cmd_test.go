// Copyright 2024 Cosmos Nicolaou. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cloudeng.io/datetime"
	"cloudeng.io/geospatial/astronomy"
	"github.com/cosnicolaou/automation/internal"
)

func TestSimulateAndLogs(t *testing.T) {
	ctx := context.Background()
	tmpFile := filepath.Join(t.TempDir(), "simulate.log")

	fl := &SimulateFlags{
		ConfigFileFlags: ConfigFileFlags{
			SystemFile:   filepath.Join("testdata", "system.yaml"),
			KeysFile:     filepath.Join("testdata", "keys.yaml"),
			ScheduleFile: filepath.Join("testdata", "schedule.yaml"),
		},
		DateRange: "12/01/2024:12/01/2025",
		LogFile:   tmpFile,
	}

	schedule := &Schedule{}
	if err := schedule.Simulate(ctx, fl, []string{}); err != nil {
		t.Fatalf("failed to display config: %v", err)
	}
	f, err := os.Open(tmpFile)
	if err != nil {
		t.Fatalf("failed to open log file: %v", err)
	}
	defer f.Close()
	sc := internal.NewLogScanner(f)
	nYearEnd := map[string]int{}
	nCompleted := map[string]int{}
	nPending := map[string]int{}
	nOn := map[string]int{}
	nOff := map[string]int{}
	nAnother := map[string]int{}
	nDays := map[string]int{}
	nAborted := map[string]int{}
	dates := map[string][]datetime.CalendarDate{}
	for le := range sc.Entries() {
		switch le.Msg {
		case "year-end":
			nYearEnd[le.Schedule]++
		case "pending":
			nPending[le.Schedule]++
		case "completed":
			nCompleted[le.Schedule]++
			if !le.PreCondResult {
				nAborted[le.Schedule]++
			}
		case "day":
			nDays[le.Schedule]++
			dates[le.Schedule] = append(dates[le.Schedule], le.Date)
		}
		if le.Msg == "completed" && !le.Aborted() {
			switch le.Op {
			case "on":
				nOn[le.Schedule]++
			case "off":
				nOff[le.Schedule]++
			case "another":
				nAnother[le.Schedule]++
			}
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	if got, want := nYearEnd, (map[string]int{
		"simple":                 2,
		"precondition-sunny":     2,
		"precondition-not-sunny": 2,
		"other-device":           2,
	}); !maps.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}

	daysInSchedule := (31 + daysInSummer(2025))

	allOps := map[string]int{
		"simple":                 daysInSchedule * (2 + 3),
		"precondition-sunny":     daysInSchedule * (2 + 3),
		"precondition-not-sunny": daysInSchedule * (2 + 3),
		"other-device":           31 * 2,
	}

	allOnOffOps := map[string]int{
		"simple":                 daysInSchedule,
		"precondition-not-sunny": daysInSchedule,
		"precondition-sunny":     daysInSchedule,
		"other-device":           31,
	}
	if got, want := nPending, allOps; !maps.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}

	if got, want := nCompleted, allOps; !maps.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}

	if got, want := nAborted, (map[string]int{
		"precondition-not-sunny": daysInSchedule * 3,
	}); !maps.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}

	if got, want := nOn, allOnOffOps; !maps.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}

	if got, want := nOff, allOnOffOps; !maps.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}

	if got, want := nAnother, (map[string]int{
		"simple":             daysInSchedule * 3,
		"precondition-sunny": daysInSchedule * 3,
	}); !maps.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}

	for _, schedule := range []string{"simple", "precondition-not-sunny"} {
		expected := dates[schedule]
		nextDate := 0
		jan := datetime.NewCalendarDateRange(
			datetime.NewCalendarDate(2025, 1, 1),
			datetime.NewCalendarDate(2025, 1, 31),
		)
		for d := range jan.Dates() {
			if expected[nextDate] != d {
				t.Errorf("got %v, want %v", expected[nextDate], d)
			}
			nextDate++
		}
		for d := range summerDateRange(2025).Dates() {
			if expected[nextDate] != d {
				t.Errorf("got %v, want %v", expected[nextDate], d)
			}
			nextDate++
		}
	}
	testSummaries(ctx, t, tmpFile)
}

func testSummaries(ctx context.Context, t *testing.T, logfile string) {
	var out strings.Builder

	daysInSchedule := (31 + daysInSummer(2025))
	opsPerDay := 5 + 5 + 5

	totalOps := daysInSchedule*opsPerDay + (31 * 2)

	// Summary at end, only completed entries will exist.
	lc := Log{out: &out}
	if err := lc.Status(ctx, &LogStatusFlags{}, []string{logfile}); err != nil {
		t.Fatalf("failed to display log: %v", err)
	}
	summary := out.String()
	out.Reset()

	if got, want := strings.Count(summary, "Completed"), 1; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := strings.Count(summary, "Pending"), 0; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := strings.Count(summary, "completed:"), totalOps; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := strings.Count(summary, "pending:"), 0; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	// Daily summary
	if err := lc.Status(ctx, &LogStatusFlags{DailySummary: true}, []string{logfile}); err != nil {
		t.Fatalf("failed to display log: %v", err)
	}
	summary = out.String()
	out.Reset()

	// Note the number of entries will vary depending on when the logs
	// are printed across all of the active schedulers, the only constant
	// is the total number of completions and aborted actions.

	if got, want := strings.Count(summary, "completed:"), totalOps; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := strings.Count(summary, "aborted due"), daysInSchedule*3; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	// one op per schedule will be pending at the end of the day.
	//if got, want := strings.Count(summary, "pending:"), daysInSchedule*3; got != want {
	//	t.Errorf("got %v, want %v", got, want)
	//}

	// Restrict to one device
	if err := lc.Status(ctx, &LogStatusFlags{
		DailySummary: true,
		LogFlags:     LogFlags{Device: "other-device"},
	}, []string{logfile}); err != nil {
		t.Fatalf("failed to display log: %v", err)
	}
	summary = out.String()
	out.Reset()

	if got, want := strings.Count(summary, "completed:"), 31*2; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := strings.Count(summary, "aborted due"), 0; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	// Restrict to one schedule
	if err := lc.Status(ctx, &LogStatusFlags{
		DailySummary: true,
		LogFlags:     LogFlags{Schedule: "precondition-not-sunny"},
	}, []string{logfile}); err != nil {
		t.Fatalf("failed to display log: %v", err)
	}
	summary = out.String()
	out.Reset()

	if got, want := strings.Count(summary, "Completed"), daysInSchedule; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := strings.Count(summary, "Pending"), 0; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := strings.Count(summary, "completed:"), daysInSchedule*5; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := strings.Count(summary, "pending:"), 0; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

}

func summerDateRange(year int) datetime.CalendarDateRange {
	summer := astronomy.Summer{}
	return summer.Evaluate(year)
}

func daysInSummer(year int) int {
	dr := summerDateRange(year)
	return dr.To().DayOfYear() - dr.From().DayOfYear() + 1
}
