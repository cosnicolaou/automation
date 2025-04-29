// Copyright 2024 Cosmos Nicolaou. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	"cloudeng.io/datetime/schedule"
	"cloudeng.io/logging/ctxlog"
	"github.com/cosnicolaou/automation/devices"
	"github.com/cosnicolaou/automation/scheduler"
	"gopkg.in/yaml.v3"
)

type ConfigFileFlags struct {
	KeysFile         string  `subcmd:"keys,$HOME/.autobot-keys.yaml,path/URI to a file containing keys"`
	SystemFile       string  `subcmd:"system,$HOME/.autobot-system.yaml,path to a file containing the lutron system configuration"`
	SystemTZLocation string  `subcmd:"tz,,timezone of the system"`
	ZIPCode          string  `subcmd:"zip,,zip code of the system"`
	ZIPDatabase      string  `subcmd:"zip-db-dir,,directory containing zip code database files from geonames.org"`
	Latitude         float64 `subcmd:"lat,,latitude of the system"`
	Longitude        float64 `subcmd:"long,,longitude of the system"`
	ScheduleFile     string  `subcmd:"schedule,$HOME/.lutron-schedule.yaml,path to a file containing the lutron schedule configuration"`
}

type ConfigFlags struct {
	ConfigFileFlags
}

type Config struct {
	out io.Writer
}

func marshalYAML(indent string, v any) string {
	p, _ := yaml.Marshal(v)
	lines := strings.Split(string(p), "\n")
	indented := make([]string, len(lines))
	for i, line := range lines {
		indented[i] = indent + line
	}
	return strings.Join(indented, "\n")
}

func indentBlock(indent, block string) string {
	lines := strings.Split(block, "\n")
	indented := make([]string, 0, len(lines))
	for i, line := range lines {
		if len(line) == 0 && i == len(lines)-1 {
			continue
		}
		indented = append(indented, indent+line)
	}
	return strings.Join(indented, "\n")
}

func formatAction(a schedule.ActionSpec[scheduler.Action]) string {
	var out strings.Builder
	fmt.Fprintf(&out, "%v %v", a.Name, a.Due)
	if a.Repeat.Interval > 0 {
		fmt.Fprintf(&out, " every: %v", a.Repeat.Interval)
		if a.Repeat.Repeats > 0 {
			fmt.Fprintf(&out, ", at most %v times", a.Repeat.Repeats)
		}
	}
	if a.T.Precondition.Condition != nil {
		fmt.Fprintf(&out, " if %v %v", a.T.Precondition.Name, a.T.Precondition.Args)
	}
	return out.String()
}

func (c *Config) Display(ctx context.Context, flags any, _ []string) error {
	fv := flags.(*ConfigFlags)

	ctx = ctxlog.NewJSONLogger(ctx, os.Stderr, nil)
	ctx, system, err := loadSystem(ctx, &fv.ConfigFileFlags)
	if err != nil {
		return err
	}

	// Reread the keys file in order to enumerate all the keys.
	keys, err := ReadKeysFile(ctx, fv.KeysFile)
	if err != nil {
		return fmt.Errorf("failed to read keys file: %q: %w", fv.KeysFile, err)
	}

	fmt.Fprintf(c.out, "Keys:\n")
	for _, key := range keys {
		fmt.Fprintf(c.out, "  %v\n", key)
	}

	fmt.Fprintf(c.out, "\nLocation: %v\n\n", system.Location)

	for _, controller := range system.Controllers {
		fmt.Fprintf(c.out, "Controller:\n%v\n", marshalYAML("  ", controller.Config()))
		fmt.Fprintf(c.out, "%v\n", marshalYAML("  ", controller.CustomConfig()))
	}

	for _, device := range system.Devices {
		fmt.Fprintf(c.out, "Device:\n%v\n", marshalYAML("  ", device.Config()))
		fmt.Fprintf(c.out, "Device Controlled By: %v\n", device.ControlledByName())
		fmt.Fprintf(c.out, "Device Custom Config:\n%v\n", marshalYAML("  ", device.CustomConfig()))
	}

	if fv.ScheduleFile != "" {
		schedules, err := scheduler.ParseConfigFile(ctx, fv.ScheduleFile, system)
		if err != nil {
			return err
		}
		fmt.Fprintf(c.out, "Schedules:\n")
		for _, sched := range schedules.Schedules {
			fmt.Fprintf(c.out, "%s\n", indentBlock("  ", sched.Dates.String()))
			for _, a := range sched.DailyActions {
				fmt.Fprintf(c.out, "    %s\n", formatAction(a))
			}
		}
	}
	return nil
}

func opNames[Map ~map[string]V, V any](m Map) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

func (c *Config) Operations(ctx context.Context, flags any, _ []string) error {
	fv := flags.(*ConfigFlags)
	ctx = ctxlog.NewJSONLogger(ctx, os.Stderr, nil)
	system, err := devices.ParseSystemConfigFile(ctx, fv.SystemFile)
	if err != nil {
		return err
	}
	tm := tableManager{html: false}
	fmt.Println(tm.Controllers(system).Render())
	fmt.Println(tm.Devices(system).Render())
	fmt.Println(tm.Conditions(system).Render())
	return nil
}
