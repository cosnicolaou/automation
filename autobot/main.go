// Copyright 2024 Cosmos Nicolaou. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"errors"
	"maps"
	"os"

	"cloudeng.io/cmdutil"
	"cloudeng.io/cmdutil/subcmd"
	"github.com/cosnicolaou/automation/devices"
	"github.com/cosnicolaou/elk/elkm1"
	"github.com/cosnicolaou/lutron/homeworks"
	"github.com/cosnicolaou/pentair/screenlogic"
)

const cmdSpec = `name: autobot
summary: autobot is a command line tool for interacting with home automation systems
commands:
  - name: control
    summary: issue a series of commands to control/interact with a homne automation system
    commands:
      - name: run
        arguments:
          - <name.op> - name of the device or controller and the operation to perform
          - <parameters>...
      - name: test
        arguments:
          - <name.condition> - name of the device and the condition to test
          - <parameters>...
      - name: script
        summary: read commands from a file
        arguments:
          - <filename> - the file to read commands from
  - name: schedule
    summary: schedule a series of commands to be executed at specific times
    commands:
      - name: run
        summary: run the scheduler
        arguments:
          - <schedule>...
      - name: simulate
        summary: |
          run the scheduler using simulated time so that it skips from
          scheduled time to scheduled time with minimal delay
        arguments:
          - <schedule>...
  - name: config
    summary: query/inspect the configuration file
    commands:
      - name: display
      - name: operations
  - name: logs
    summary: query/inspect the log files
    commands:
      - name: status
        arguments:
          - <log-files>...
`

func cli() *subcmd.CommandSetYAML {
	cmd := subcmd.MustFromYAML(cmdSpec)

	control := &Control{}
	cmd.Set("control", "run").MustRunner(control.Run, &ControlFlags{})
	cmd.Set("control", "test").MustRunner(control.Test, &ControlFlags{})
	cmd.Set("control", "script").MustRunner(control.RunScript, &ControlScriptFlags{})

	config := &Config{out: os.Stdout}
	cmd.Set("config", "display").MustRunner(config.Display, &ConfigFlags{})
	cmd.Set("config", "operations").MustRunner(config.Operations, &ConfigFlags{})

	schedule := &Schedule{}
	cmd.Set("schedule", "run").MustRunner(schedule.Run, &ScheduleFlags{})
	cmd.Set("schedule", "simulate").MustRunner(schedule.Simulate, &SimulateFlags{})

	log := &Log{out: os.Stdout}
	cmd.Set("logs", "status").MustRunner(log.Status, &LogStatusFlags{})
	return cmd
}

func init() {
	maps.Insert(devices.AvailableControllers,
		maps.All(homeworks.SupportedControllers()))
	maps.Insert(devices.AvailableControllers,
		maps.All(screenlogic.SupportedControllers()))
	maps.Insert(devices.AvailableControllers,
		maps.All(elkm1.SupportedControllers()))

	maps.Insert(devices.AvailableDevices,
		maps.All(homeworks.SupportedDevices()))
	maps.Insert(devices.AvailableDevices,
		maps.All(screenlogic.SupportedDevices()))
	maps.Insert(devices.AvailableDevices,
		maps.All(elkm1.SupportedDevices()))
}

var errInterrupt = errors.New("interrupt")

func main() {
	ctx := context.Background()
	ctx, cancel := context.WithCancelCause(ctx)
	cmdutil.HandleSignals(func() { cancel(errInterrupt) }, os.Interrupt)
	err := cli().Dispatch(ctx)
	if context.Cause(ctx) == errInterrupt {
		cmdutil.Exit("%v", errInterrupt)
	}
	if err != nil {
		cmdutil.Exit("%v", err)
	}
}
