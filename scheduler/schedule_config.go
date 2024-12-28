// Copyright 2024 Cosmos Nicolaou. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package scheduler

import (
	"context"
	"fmt"
	"time"

	"cloudeng.io/cmdutil/cmdyaml"
	"cloudeng.io/datetime"
	"cloudeng.io/datetime/schedule"
	"github.com/cosnicolaou/automation/devices"
	"gopkg.in/yaml.v3"
)

type repeatDuration time.Duration

func (rd *repeatDuration) UnmarshalYAML(node *yaml.Node) error {
	d, err := time.ParseDuration(node.Value)
	if err != nil {
		return err
	}
	if d == 0 {
		return fmt.Errorf("repeat duration must be greater than zero")
	}
	*rd = repeatDuration(d)
	return nil
}

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
	Months       monthList         `yaml:"months" cmd:"for the specified months"`
	MirrorMonths bool              `yaml:"mirror_months" cmd:"include the mirror months, ie. those equidistant from the soltices for the set of 'for' months"`
	Ranges       []string          `yaml:"ranges" cmd:"for the specified date ranges"`
	Constraints  constraintsConfig `yaml:",inline" cmd:"constrain the dates"`
}

func (dc *datesConfig) parse() (schedule.Dates, error) {
	d := schedule.Dates{
		Months:       datetime.MonthList(dc.Months),
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

type precondition struct {
	Device string   `yaml:"device" cmd:"name of the device that the pre-condition applies to"`
	Op     string   `yaml:"op" cmd:"name of the pre-condition in device.op format, use \"!op\" for negation"`
	Args   []string `yaml:"args,flow" cmd:"arguments to be passed to the pre-condition"`
}

type actionDetailed struct {
	When         timeOfDay      `yaml:"when" cmd:"time of day when the action is to be taken"`
	Action       string         `yaml:"action" cmd:"action to be taken"`
	Args         []string       `yaml:"args,flow" cmd:"argument to be passed to the action"`
	Precondition precondition   `yaml:"precondition" cmd:"precondition that must be satisfied before the action is taken"`
	Before       string         `yaml:"before" cmd:"action that must be taken before this one if it is scheduled for the same time"`
	After        string         `yaml:"after" cmd:"action that must be taken after this one if it is scheduled for the same time"`
	Repeat       repeatDuration `yaml:"repeat" cmd:"repeat the action every specified duration, starting at 'when'"`
	NumRepeats   int            `yaml:"num_repeats" cmd:"number of times to repeat"`
}

type actionScheduleConfig struct {
	Name            string               `yaml:"name" cmd:"name of the schedule"`
	Device          string               `yaml:"device" cmd:"name of the device that the schedule applies to"`
	Dates           datesConfig          `yaml:",inline" cmd:"dates that the schedule applies to"`
	Actions         map[string]timeOfDay `yaml:"actions" cmd:"actions to be taken and when"`
	ActionsDetailed []actionDetailed     `yaml:"actions_detailed" cmd:"actions that accept arguments"`
}

type schedulesConfig struct {
	Schedules []actionScheduleConfig `yaml:"schedules" cmd:"the schedules"`
}

type Annual struct {
	Name         string
	Dates        schedule.Dates
	DailyActions schedule.ActionSpecs[Action]
}

type Schedules struct {
	System    devices.System
	Schedules []Annual
}

func (s Schedules) Lookup(name string) Annual {
	for _, sched := range s.Schedules {
		if sched.Name == name {
			return sched
		}
	}
	return Annual{}
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

func (cfg schedulesConfig) createActions(sys devices.System, times, scheduleName, deviceName, actionName string, details actionDetailed) (schedule.ActionSpecs[Action], error) {
	var actionTimes ActionTimeList
	if err := actionTimes.Parse(times); err != nil {
		return nil, fmt.Errorf("failed to parse time of day %q for schedule %q, operation: %q: %v", times, scheduleName, actionName, err)
	}
	actions := schedule.ActionSpecs[Action]{}
	for _, actionTime := range actionTimes {
		due, dynDue, delta := actionTime.Literal, actionTime.Dynamic, actionTime.Delta
		if _, _, ok := sys.DeviceConfigs(deviceName); !ok {
			return nil, fmt.Errorf("unknown device: %s for schedule %q", deviceName, scheduleName)
		}
		if _, _, ok := sys.DeviceOp(deviceName, actionName); !ok {
			return nil, fmt.Errorf("unknown operation: %q for device: %q for schedule %q", actionName, deviceName, scheduleName)
		}

		var condition devices.Condition
		if details.Precondition.Op != "" {
			c, _, ok := sys.DeviceCondition(details.Precondition.Device, details.Precondition.Op)
			if !ok {
				return nil, fmt.Errorf("unknown precondition: %q for device: %q for schedule %q", details.Precondition.Op, deviceName, scheduleName)
			}
			condition = c
		}

		actions = append(actions, schedule.ActionSpec[Action]{
			Due:  due,
			Name: actionName,
			Dynamic: schedule.DynamicTimeOfDaySpec{
				Due:    dynDue,
				Offset: delta,
			},
			Repeat: schedule.RepeatSpec{
				Interval: time.Duration(details.Repeat),
				Repeats:  details.NumRepeats,
			},
			T: Action{
				Action: devices.Action{
					DeviceName: deviceName,
					Name:       actionName,
					Args:       details.Args,
				},
				Precondition: Precondition{
					Name:      details.Precondition.Op,
					Condition: condition,
					Args:      details.Precondition.Args,
				}}})
	}
	return actions, nil
}

func (cfg schedulesConfig) createSchedules(sys devices.System) (Schedules, error) {
	var sched Schedules
	names := map[string]struct{}{}
	for _, csched := range cfg.Schedules {
		if _, ok := names[csched.Name]; ok {
			return Schedules{}, fmt.Errorf("duplicate schedule name: %v", csched.Name)
		}
		names[csched.Name] = struct{}{}
		var annual Annual
		annual.Name = csched.Name
		dates, err := csched.Dates.parse()
		if err != nil {
			return Schedules{}, err
		}

		annual.Dates = dates

		for name, when := range csched.Actions {
			actions, err := cfg.createActions(sys, string(when), csched.Name, csched.Device, name, actionDetailed{})
			if err != nil {
				return Schedules{}, err
			}
			annual.DailyActions = append(annual.DailyActions, actions...)
		}
		for _, details := range csched.ActionsDetailed {
			actions, err := cfg.createActions(sys, string(details.When), csched.Name, csched.Device, details.Action, details)
			if err != nil {
				return Schedules{}, err
			}
			annual.DailyActions = append(annual.DailyActions, actions...)
		}
		annual.DailyActions.Sort()
		annual.DailyActions, err = orderActionsStatic(annual.DailyActions, csched.ActionsDetailed)
		if err != nil {
			return Schedules{}, fmt.Errorf("failed to order actions for schedule %q: %v", csched.Name, err)
		}
		if len(annual.DailyActions) == 0 {
			return Schedules{}, fmt.Errorf("no actions defined for schedule %q", csched.Name)
		}
		sched.Schedules = append(sched.Schedules, annual)
	}
	sched.System = sys

	return sched, nil
}
