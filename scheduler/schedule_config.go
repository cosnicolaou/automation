// Copyright 2024 Cosmos Nicolaou. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package scheduler

import (
	"context"
	"fmt"
	"slices"

	"cloudeng.io/cmdutil/cmdyaml"
	"cloudeng.io/datetime"
	"cloudeng.io/datetime/schedule"
	"github.com/cosnicolaou/automation/devices"
	"gopkg.in/yaml.v3"
)

type monthList datetime.MonthList
type timeOfDay datetime.TimeOfDay

func (ml *monthList) UnmarshalYAML(node *yaml.Node) error {
	return (*datetime.MonthList)(ml).Parse(node.Value)
}

func (t *timeOfDay) UnmarshalYAML(node *yaml.Node) error {
	return (*datetime.TimeOfDay)(t).Parse(node.Value)
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
	if err := d.Ranges.Parse(dc.Ranges); err != nil {
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
	Schedules []schedule.Annual[devices.Action]
}

func (s Schedules) Lookup(name string) schedule.Annual[devices.Action] {
	for _, sched := range s.Schedules {
		if sched.Name == name {
			return sched
		}
	}
	return schedule.Annual[devices.Action]{}
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
		var annual schedule.Annual[devices.Action]
		annual.Name = csched.Name
		dates, err := csched.Dates.parse()
		if err != nil {
			return Schedules{}, err
		}
		annual.Dates = dates
		for name, when := range csched.Actions {
			if _, ok := sys.Devices[csched.Device]; !ok {
				return Schedules{}, fmt.Errorf("unknown device: %s", csched.Device)
			}
			annual.Actions = append(annual.Actions, schedule.Action[devices.Action]{
				Due:  datetime.TimeOfDay(when),
				Name: name,
				Action: devices.Action{
					DeviceName: csched.Device,
					ActionName: name,
				}})
		}
		for _, withargs := range csched.ActionsWithArgs {
			if _, ok := sys.Devices[csched.Device]; !ok {
				return Schedules{}, fmt.Errorf("unknown device: %s", csched.Device)
			}
			annual.Actions = append(annual.Actions, schedule.Action[devices.Action]{
				Due:  datetime.TimeOfDay(withargs.When),
				Name: withargs.Action,
				Action: devices.Action{
					DeviceName: csched.Device,
					ActionName: withargs.Action,
					ActionArgs: withargs.Args,
				}})
		}

		ordered, err := orderOperations(annual.Actions, csched.ActionsWithArgs)
		if err != nil {
			return Schedules{}, err
		}
		annual.Actions = ordered
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

func orderOperations(actions schedule.Actions[devices.Action], withArgs []actionWithArg) (schedule.Actions[devices.Action], error) {

	actions.Sort()

	order := map[string]int{}
	for i, a := range actions {
		order[a.Name] = i
	}

	for _, wa := range withArgs {
		if len(wa.Before) == 0 && len(wa.After) == 0 {
			continue
		}
		before, target, err := validate(wa)
		if err != nil {
			return nil, err
		}

		targetPos := order[target]
		cPos := order[wa.Action]

		if actions[targetPos].Due != actions[cPos].Due {
			return nil, fmt.Errorf("action %v is not scheduled for the same time as %v", target, wa.Action)
		}

		if before {
			if cPos != targetPos-1 {
				tmp := slices.Insert(actions, targetPos, actions[cPos])
				delPos := posPostInsertion(cPos, targetPos)
				return slices.Delete(tmp, delPos, delPos+1), nil
			}
			continue
		}
		if cPos != targetPos+1 {
			tmp := slices.Insert(actions, targetPos+1, actions[cPos])
			delPos := posPostInsertion(cPos, targetPos)
			return slices.Delete(tmp, delPos, delPos+1), nil
		}
	}
	return actions, nil
}

func posPostInsertion(cPos, targetPos int) int {
	if cPos >= targetPos {
		return cPos + 1
	}
	return cPos
}
