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
	"github.com/cosnicolaou/automation/internal/logging"
)

func countEvents(t *testing.T, logfile string) (counts map[string]map[string]int, dates map[string][]datetime.CalendarDate) {
	f, err := os.Open(logfile)
	if err != nil {
		t.Fatalf("failed to open log file: %v", err)
	}
	defer f.Close()
	sc := logging.NewScanner(f)
	counts = map[string]map[string]int{}
	counts["aborted"] = map[string]int{}
	dates = map[string][]datetime.CalendarDate{}
	for le := range sc.Entries() {
		if _, ok := counts[le.Msg]; !ok {
			counts[le.Msg] = map[string]int{}
		}
		switch le.Msg {
		case "year-end", "pending":
			counts[le.Msg][le.Schedule]++
		case "completed":
			counts[le.Msg][le.Schedule]++
			if !le.PreCondResult {
				counts["aborted"][le.Schedule]++
			}
		case "day":
			counts[le.Msg][le.Schedule]++
			dates[le.Schedule] = append(dates[le.Schedule], le.Date)
		}
		if le.Msg == "completed" && !le.Aborted() {
			if _, ok := counts[le.Op]; !ok {
				counts[le.Op] = map[string]int{}
			}
			switch le.Op {
			case "on", "off", "another":
				counts[le.Op][le.Schedule]++
			}
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}
	return counts, dates
}

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
		DryRun:    false, // this is safe since the test system has dummy devices
		LogFile:   tmpFile,
		WebUIFlags: WebUIFlags{
			HTTPAddr: "0.0.0.0:0",
		},
	}

	schedule := &Schedule{}
	if err := schedule.Simulate(ctx, fl, []string{}); err != nil {
		t.Fatalf("failed to display config: %v", err)
	}

	counts, dates := countEvents(t, tmpFile)

	if got, want := counts["year-end"], (map[string]int{
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
	if got, want := counts["pending"], allOps; !maps.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}

	if got, want := counts["completed"], allOps; !maps.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}

	if got, want := counts["aborted"], (map[string]int{
		"precondition-not-sunny": daysInSchedule * 3,
	}); !maps.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}

	if got, want := counts["on"], allOnOffOps; !maps.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}

	if got, want := counts["off"], allOnOffOps; !maps.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}

	if got, want := counts["another"], (map[string]int{
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
	testSummaryFlags(ctx, t, tmpFile)
}

func removeHeader(summary string) string {
	// skip the table header
	idx := strings.Index(summary, "ERROR")
	if idx > 0 {
		summary = summary[idx:]
	}
	return summary
}

func testSummaries(ctx context.Context, t *testing.T, logfile string) {
	var out strings.Builder

	daysInSchedule := (31 + daysInSummer(2025))
	opsPerDay := 5 + 5 + 5

	totalOps := daysInSchedule*opsPerDay + (31 * 2) - (daysInSchedule * 3) // subtracted the aborted actions

	// Summary at end, only completed entries will exist.
	lc := Log{out: &out}
	if err := lc.Status(ctx, &LogStatusFlags{FinalSummary: true}, []string{logfile}); err != nil {
		t.Fatalf("failed to display log: %v", err)
	}
	summary := removeHeader(out.String())
	out.Reset()

	if got, want := strings.Count(summary, "| completed"), totalOps; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := strings.Count(summary, "| pending"), 0; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := strings.Count(summary, "| aborted"), daysInSchedule*3; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	// Streaming summary
	if err := lc.Status(ctx, &LogStatusFlags{StreamingSummary: true}, []string{logfile}); err != nil {
		t.Fatalf("failed to display log: %v", err)
	}
	summary = out.String()
	out.Reset()
	// Note the number of entries will vary depending on when the logs
	// are printed across all of the active schedulers, the only constant
	// is the total number of completions and aborted actions.
	if got, want := strings.Count(summary, "| completed"), totalOps; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	if got, want := strings.Count(summary, "| aborted"), daysInSchedule*3; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}

func testSummaryFlags(ctx context.Context, t *testing.T, logfile string) {

	daysInSchedule := (31 + daysInSummer(2025))

	var out strings.Builder
	lc := Log{out: &out}

	// Restrict to one device
	if err := lc.Status(ctx, &LogStatusFlags{
		FinalSummary: true,
		LogFlags:     LogFlags{Device: "other-device"},
	}, []string{logfile}); err != nil {
		t.Fatalf("failed to display log: %v", err)
	}
	summary := removeHeader(out.String())
	out.Reset()

	if got, want := strings.Count(summary, "| completed"), 31*2; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := strings.Count(summary, "| aborted"), 0; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	// Restrict to one schedule
	if err := lc.Status(ctx, &LogStatusFlags{
		FinalSummary: true,
		LogFlags:     LogFlags{Schedule: "precondition-not-sunny"},
	}, []string{logfile}); err != nil {
		t.Fatalf("failed to display log: %v", err)
	}
	summary = removeHeader(out.String())
	out.Reset()

	if got, want := strings.Count(summary, "| completed"), daysInSchedule*2; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := strings.Count(summary, "| aborted"), daysInSchedule*3; got != want {
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
