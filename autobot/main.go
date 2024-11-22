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
	"cloudeng.io/cmdutil/cmdyaml"
	"cloudeng.io/cmdutil/subcmd"
	"cloudeng.io/macos/keychainfs"
	"github.com/cosnicolaou/automation/devices"
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
      - name: script
        summary: read commands from a file
        arguments:
          - <filename> - the file to read commands from
  - name: config
    summary: query/inspect the configuration file
    commands:
      - name: display
      - name: operations
`

func cli() *subcmd.CommandSetYAML {
	cmd := subcmd.MustFromYAML(cmdSpec)
	control := &Control{}
	cmd.Set("control", "run").MustRunner(control.Run, &ControlFlags{})
	cmd.Set("control", "script").MustRunner(control.RunScript, &ControlScriptFlags{})
	config := &Config{}
	cmd.Set("config", "display").MustRunner(config.Display, &ConfigFlags{})
	cmd.Set("config", "operations").MustRunner(config.Operations, &ConfigFlags{})

	return cmd
}

var URIHandlers = map[string]cmdyaml.URLHandler{
	"keychain": keychainfs.NewSecureNoteFSFromURL,
}

func init() {
	maps.Insert(devices.AvailableControllers,
		maps.All(homeworks.SupportedControllers()))
	maps.Insert(devices.AvailableControllers,
		maps.All(screenlogic.SupportedControllers()))

	maps.Insert(devices.AvailableDevices,
		maps.All(homeworks.SupportedDevices()))
	maps.Insert(devices.AvailableDevices,
		maps.All(screenlogic.SupportedDevices()))
}

var interrupt = errors.New("interrupt")

func main() {
	ctx := context.Background()
	ctx, cancel := context.WithCancelCause(ctx)
	cmdutil.HandleSignals(func() { cancel(interrupt) }, os.Interrupt)
	err := cli().Dispatch(ctx)
	if context.Cause(ctx) == interrupt {
		cmdutil.Exit("%v", interrupt)
	}
	if err != nil {
		cmdutil.Exit("%v", err)
	}
}
