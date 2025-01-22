// Copyright 2024 Cosmos Nicolaou. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package main

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/cosnicolaou/automation/cmd/autobot/internal/webapi"
	"github.com/cosnicolaou/automation/devices"
	"github.com/pkg/browser"
)

type ControlFlags struct {
	ConfigFileFlags
}

type ControlScriptFlags struct {
	ControlFlags
}

type ControlTestPageFlags struct {
	ControlFlags
	WebUIFlags
}

type Control struct {
	system devices.System
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
	cc := webapi.NewControlClient(c.system)
	return cc.RunOperation(ctx, os.Stdout, cmd, parameters)
}

func (c *Control) Condition(ctx context.Context, flags any, args []string) error {
	ctx, err := c.setup(ctx, flags.(*ControlFlags))
	if err != nil {
		return err
	}
	cmd := args[0]
	parameters := args[1:]
	cc := webapi.NewControlClient(c.system)
	result, err := cc.RunCondition(ctx, os.Stdout, cmd, parameters)
	if err != nil {
		return err
	}
	fmt.Printf("%v(%v): %v\n", cmd, strings.Join(parameters, ", "), result)
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
	cc := webapi.NewControlClient(c.system)
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
		if err := cc.RunOperation(ctx, os.Stdout, cmd, parameters); err != nil {
			return err
		}
	}
	return nil
}

func (c *Control) ServeTestPage(ctx context.Context, flags any, _ []string) error {
	fv := flags.(*ControlTestPageFlags)
	ctx, err := c.setup(ctx, &fv.ControlFlags)
	if err != nil {
		return err
	}

	mux := http.NewServeMux()
	_, runner, url, err := fv.WebUIFlags.CreateWebServer(ctx, mux)
	if err != nil {
		return err
	}

	tm := tableManager{html: true}
	webapi.AppendTestServerEndpoints(mux,
		fv.ConfigFileFlags.SystemFile,
		tm.RenderHTML(tm.Controllers(c.system)),
		tm.RenderHTML(tm.Devices(c.system)),
		tm.RenderHTML(tm.Conditions(c.system)),
		tm.RenderHTML(tm.ControllerOperations(c.system)),
		tm.RenderHTML(tm.DeviceOperations(c.system)),
		tm.RenderHTML(tm.DeviceConditions(c.system)),
	)

	cc := webapi.NewControlClient(c.system)
	webapi.AppendControlAPIEndpoints(ctx, cc, mux)

	_ = browser.OpenURL(url)
	return runner()
}
