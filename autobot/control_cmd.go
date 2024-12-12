// Copyright 2024 Cosmos Nicolaou. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package main

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/cosnicolaou/automation/devices"
)

type ControlFlags struct {
	ConfigFileFlags
}

type ControlScriptFlags struct {
	ControlFlags
}

type Control struct {
	system devices.System
}

func (c *Control) runOp(ctx context.Context, system devices.System, nameAndOp string, args []string) error {
	parts := strings.Split(nameAndOp, ".")
	if len(parts) != 2 {
		return fmt.Errorf("invalid operation: %v, should be name.operation", nameAndOp)
	}
	name, op := parts[0], parts[1]
	_, cok := system.Controllers[name]
	_, dok := system.Devices[name]
	if !cok && !dok {
		return fmt.Errorf("unknown controller or device: %v", name)
	}

	if fn, pars, ok := system.ControllerOp(name, op); ok {
		if len(args) == 0 {
			args = pars
		}
		opts := devices.OperationArgs{
			Writer: os.Stdout,
			Args:   args,
		}
		if err := fn(ctx, opts); err != nil {
			return fmt.Errorf("failed to run operation: %v: %v", op, err)
		}
		return nil
	}
	if fn, pars, ok := system.DeviceOp(name, op); ok {
		if len(args) == 0 {
			args = pars
		}
		opts := devices.OperationArgs{
			Writer: os.Stdout,
			Args:   args,
		}
		if err := fn(ctx, opts); err != nil {
			return fmt.Errorf("failed to run operation: %v: %v", op, err)
		}
		return nil
	}
	return fmt.Errorf("unknown or not configured operation: %v, %v", name, op)
}

func (c *Control) setup(ctx context.Context, fv *ControlFlags) (context.Context, error) {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	opts := []devices.Option{
		devices.WithLogger(logger),
	}
	ctx, sys, err := loadSystem(ctx, &fv.ConfigFileFlags, opts...)
	if err != nil {
		return nil, err
	}
	c.system = sys
	return ctx, nil
}

func (c *Control) Run(ctx context.Context, flags any, args []string) error {
	ctx, err := c.setup(ctx, flags.(*ControlFlags))
	if err != nil {
		return err
	}
	cmd := args[0]
	parameters := args[1:]
	if err := c.runOp(ctx, c.system, cmd, parameters); err != nil {
		return err
	}
	return nil
}

func (c *Control) RunScript(ctx context.Context, flags any, args []string) error {
	ctx, err := c.setup(ctx, &flags.(*ControlScriptFlags).ControlFlags)
	if err != nil {
		return err
	}

	scriptFile := args[0]

	f, err := os.Open(scriptFile)
	if err != nil {
		return fmt.Errorf("failed to open script file: %v: %v", scriptFile, err)
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) == 0 {
			continue
		}
		cmd := parts[0]
		parameters := parts[1:]
		if err := c.runOp(ctx, c.system, cmd, parameters); err != nil {
			return err
		}
	}
	return nil
}
