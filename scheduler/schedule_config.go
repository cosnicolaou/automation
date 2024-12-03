// Copyright 2024 Cosmos Nicolaou. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package scheduler

import (
	"context"
	"fmt"

	"cloudeng.io/cmdutil/cmdyaml"
	"cloudeng.io/datetime"
	"cloudeng.io/datetime/schedule"
	"github.com/cosnicolaou/automation/devices"
	"gopkg.in/yaml.v3"
)

type monthList datetime.MonthList

func (ml *monthList) UnmarshalYAML(node *yaml.Node) error {
	return (*datetime.MonthList)(ml).Parse(node.Value)
}

type constraintsConfig struct {
	Weekdays bool   `yaml:"weekdays" cmd:"only on weekdays"`
	Weekends bool   `yaml:"weekends" cmd:"only on weekends"`
	Custom   string `yaml:"exclude_dates" cmd:"exclude the specified dates eg: 01/02,jan-02"`
}

func (cc constraintsConfig) parse() (datetime.Constraints, error) {
	dc := datetime.Constraints{
		Weekdays: cc.Weekdays,
		Weekends: cc.Weekends,
	}
	if err := dc.Custom.Parse(cc.Custom); err != nil {
		return datetime.Constraints{}, err
	}
	return dc, nil
}

type datesConfig struct {
	For          monthList         `yaml:"for" cmd:"for the specified months"`
	MirrorMonths bool              `yaml:"mirror_months" cmd:"include the mirror months, ie. those equidistant from the soltices for the set of 'for' months"`
	Ranges       []string          `yaml:"ranges" cmd:"for the specified date ranges"`
	Constraints  constraintsConfig `yaml:",inline" cmd:"constrain the dates"`
}

func (dc *datesConfig) parse() (schedule.Dates, error) {
	d := schedule.Dates{
		For:          datetime.MonthList(dc.For),
		MirrorMonths: dc.MirrorMonths,
	}
	var err error
	d.Ranges, d.Dynamic, err = ParseDateRangesDynamic(dc.Ranges)
	if err != nil {
		return schedule.Dates{}, err
	}
	cc, err := dc.Constraints.parse()
	if err != nil {
		return schedule.Dates{}, err
	}
	d.Constraints = cc
	return d, nil
}

type actionWithArg struct {
	When   timeOfDay `yaml:"when" cmd:"the time of day when the action is to be taken"`
	Action string    `yaml:"action" cmd:"the action to be taken"`
	Args   []string  `yaml:"args,flow" cmd:"the argument to be passed to the action"`
	Before string    `yaml:"before" cmd:"the action that must be taken before this one if it is scheduled for the same time"`
	After  string    `yaml:"after" cmd:"the action that must be taken after this one if it is scheduled for the same time"`
}

type actionScheduleConfig struct {
	Name            string               `yaml:"name" cmd:"the name of the schedule"`
	Device          string               `yaml:"device" cmd:"the name of the device that the schedule applies to"`
	Dates           datesConfig          `yaml:",inline" cmd:"the dates that the schedule applies to"`
	Actions         map[string]timeOfDay `yaml:"actions" cmd:"the actions to be taken and when"`
	ActionsWithArgs []actionWithArg      `yaml:"actions_with_args" cmd:"actions that accept arguments"`
}

type schedulesConfig struct {
	Schedules []actionScheduleConfig `yaml:"schedules" cmd:"the schedules"`
}

type Schedules struct {
	System    devices.System
	Schedules []schedule.Annual[Daily]
}

func (s Schedules) Lookup(name string) schedule.Annual[Daily] {
	for _, sched := range s.Schedules {
		if sched.Name == name {
			return sched
		}
	}
	return schedule.Annual[Daily]{}
}

func ParseConfigFile(ctx context.Context, cfgFile string, system devices.System) (Schedules, error) {
	var cfg schedulesConfig
	if err := cmdyaml.ParseConfigFile(ctx, cfgFile, &cfg); err != nil {
		return Schedules{}, err
	}
	pcfg, err := cfg.createSchedules(system)
	if err != nil {
		return Schedules{}, err
	}
	return pcfg, nil
}

func ParseConfig(ctx context.Context, cfgData []byte, system devices.System) (Schedules, error) {
	var cfg schedulesConfig
	if err := yaml.Unmarshal(cfgData, &cfg); err != nil {
		return Schedules{}, err
	}
	pcfg, err := cfg.createSchedules(system)
	if err != nil {
		return Schedules{}, err
	}
	return pcfg, err
}

func (cfg schedulesConfig) createSchedules(sys devices.System) (Schedules, error) {
	var sched Schedules
	names := map[string]struct{}{}
	for _, csched := range cfg.Schedules {
		if _, ok := names[csched.Name]; ok {
			return Schedules{}, fmt.Errorf("duplicate schedule name: %v", csched.Name)
		}
		names[csched.Name] = struct{}{}
		var annual schedule.Annual[Daily]
		annual.Name = csched.Name
		dates, err := csched.Dates.parse()
		if err != nil {
			return Schedules{}, err
		}
		annual.Dates = dates
		for name, when := range csched.Actions {
			due, dynDue, delta, err := ParseActionTimeDynamic(string(when))
			if err != nil {
				return Schedules{}, fmt.Errorf("failed to parse time of day %q for schedule %q, operation: %q: %v", when, csched.Name, name, err)
			}
			if _, ok := sys.Devices[csched.Device]; !ok {
				return Schedules{}, fmt.Errorf("unknown device: %s for schedule %q", csched.Device, csched.Name)
			}
			annual.Actions = append(annual.Actions, schedule.Action[Daily]{
				Due:  due,
				Name: name,
				Action: Daily{
					Action: devices.Action{
						DeviceName: csched.Device,
						Name:       name,
					},
					DynamicTimeOfDay: dynDue,
					DynamicDelta:     delta,
				}})
		}
		for _, withargs := range csched.ActionsWithArgs {
			if _, ok := sys.Devices[csched.Device]; !ok {
				return Schedules{}, fmt.Errorf("unknown device: %s for schedule %q", csched.Device, csched.Name)
			}
			due, dynDue, delta, err := ParseActionTimeDynamic(string(withargs.When))
			if err != nil {
				return Schedules{}, fmt.Errorf("failed to parse time of day %q for schedule %q, operation: %q: %v", withargs.When, csched.Name, withargs.Action, err)
			}
			annual.Actions = append(annual.Actions, schedule.Action[Daily]{
				Due:  due,
				Name: withargs.Action,
				Action: Daily{
					Action: devices.Action{
						DeviceName: csched.Device,
						Name:       withargs.Action,
						Args:       withargs.Args,
					},
					DynamicTimeOfDay: dynDue,
					DynamicDelta:     delta,
				}})
		}

		annual.Actions.Sort()
		annual.Actions, err = orderDailyOperationsStatic(annual.Actions, csched.ActionsWithArgs)
		if err != nil {
			return Schedules{}, fmt.Errorf("failed to order actions for schedule %q: %v", csched.Name, err)
		}
		sched.Schedules = append(sched.Schedules, annual)
	}
	sched.System = sys

	return sched, nil
}

func validate(withArgs actionWithArg) (before bool, name string, err error) {
	if len(withArgs.Before) != 0 && len(withArgs.After) != 0 {
		return false, "", fmt.Errorf("action %v cannot have both before and after specified", withArgs.Action)
	}
	name = withArgs.Before
	before = true
	if len(name) == 0 {
		name = withArgs.After
		before = false
	}
	if name == withArgs.Action {
		return false, "", fmt.Errorf("action %v cannot be before or after itself", withArgs.Action)
	}
	return
}
