// Copyright 2024 Cosmos Nicolaou. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package scheduler

import (
	"fmt"
	"strings"
	"time"

	"cloudeng.io/datetime"
	"cloudeng.io/geospatial/astronomy"
)

var (
	AnnualDynamic = map[string]datetime.DynamicDateRange{
		"summer":          astronomy.Summer{},
		"winter":          astronomy.Winter{},
		"spring":          astronomy.Spring{},
		"fall":            astronomy.Autumn{LocalName: "Fall"},
		"autumn":          astronomy.Autumn{},
		"winter-solstice": astronomy.WinterSolstice{},
		"summer-solstice": astronomy.SummerSolstice{},
		"spring-equinox":  astronomy.SpringEquinox{},
		"fall-equinox":    astronomy.AutumnEquinox{},
		"autumn-equinox":  astronomy.AutumnEquinox{},
	}

	DailyDynamic = map[string]datetime.DynamicTimeOfDay{
		"sunrise": astronomy.SunRise{},
		"sunset":  astronomy.SunSet{},
	}
)

type now struct{}

func (now) Evaluate(_ datetime.CalendarDate, loc *time.Location) datetime.TimeOfDay {
	return datetime.TimeOfDayFromTime(time.Now().In(loc))
}

func (now) Name() string {
	return "now"
}

// ParseDateRangesDynamic parses a list of date ranges that may
// contain dynamic date ranges. Valid dynamic date ranges are
// definmed by AnnualDynamic.
func ParseDateRangesDynamic(vals []string) (datetime.DateRangeList, datetime.DynamicDateRangeList, error) {
	var drl datetime.DateRangeList
	var ddl datetime.DynamicDateRangeList
	for _, val := range vals {
		var dr datetime.DateRange
		if err := dr.Parse(val); err == nil {
			drl = append(drl, dr)
			continue
		}
		dyn, ok := AnnualDynamic[val]
		if !ok {
			return nil, nil, fmt.Errorf("invalid date range or unknown dynamic date range: %v", val)
		}
		ddl = append(ddl, dyn)
	}
	return drl, ddl, nil
}

func parseFunctionAndDelta(s string) (datetime.DynamicTimeOfDay, time.Duration, error) {
	s = strings.TrimSpace(s)
	pidx, nidx := strings.Index(s, "+"), strings.Index(s, "-")
	if pidx != -1 && nidx != -1 {
		return nil, 0, fmt.Errorf("dynamic time of day with multiple deltas: %v", s)
	}
	idx := max(pidx, nidx)
	name := s
	delta := ""
	if idx != -1 {
		name = s[:idx]
		delta = s[idx:]
	}
	name = strings.TrimSpace(name)
	dyn, ok := DailyDynamic[name]
	if !ok {
		return nil, 0, fmt.Errorf("unknown dynamic time or invalid time: %v", s)
	}
	if len(delta) == 0 {
		return dyn, 0, nil
	}
	deltaDur, err := time.ParseDuration(delta)
	if err != nil {
		return nil, 0, fmt.Errorf("invalid duration: %v", delta)
	}
	return dyn, deltaDur, nil
}

// ActionTime represents a time of day that may be a literal or a dynamic
// value.
type ActionTime struct {
	Literal datetime.TimeOfDay
	Dynamic datetime.DynamicTimeOfDay
	Delta   time.Duration
}

type ActionTimeList []ActionTime

func (atl *ActionTimeList) Parse(val string) error {
	parts := strings.Split(val, ",")
	for _, p := range parts {
		literal, dyn, delta, err := ParseActionTime(p)
		if err != nil {
			return err
		}
		*atl = append(*atl, ActionTime{Literal: literal, Dynamic: dyn, Delta: delta})
	}
	return nil
}

// ParseAction parses a time of day that may contain
// a dynamic time of day function with a +- delta. Valid dynamic
// time of day functions are defined by DailyDynamic.
func ParseActionTime(v string) (datetime.TimeOfDay, datetime.DynamicTimeOfDay, time.Duration, error) {
	var tod datetime.TimeOfDay
	if err := tod.Parse(v); err == nil {
		return tod, nil, 0, nil
	}
	dyn, delta, err := parseFunctionAndDelta(v)
	if err != nil {
		return datetime.TimeOfDay(0), nil, 0, err
	}
	return datetime.TimeOfDay(0), dyn, delta, err
}
