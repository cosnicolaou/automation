// Copyright 2024 Cosmos Nicolaou. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"cloudeng.io/cmdutil/keystore"
	"cloudeng.io/datetime"
	"github.com/cosnicolaou/automation/cmd/autobot/internal/webapi"
	"github.com/cosnicolaou/automation/cmd/autobot/internal/webassets"
	"github.com/cosnicolaou/automation/devices"
	"github.com/cosnicolaou/automation/scheduler"
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
	logger *slog.Logger
}

func (c *Control) setup(ctx context.Context, fv *ControlFlags) (context.Context, func(ctx context.Context) (devices.System, error), error) {
	c.logger = slog.New(slog.NewJSONHandler(os.Stderr, nil))
	opts := []devices.Option{
		devices.WithLogger(c.logger),
	}

	keys, err := ReadKeysFile(ctx, fv.KeysFile)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read keys file: %q: %w", fv.KeysFile, err)
	}

	zdb, err := loadZIPDatabase(fv.ZIPDatabase)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load zip database: %q: %w", fv.ZIPDatabase, err)
	}
	opts = append(opts, devices.WithZIPCodeLookup(zdb))

	ctx = keystore.ContextWithAuth(ctx, keys)

	loader := func(ctx context.Context) (devices.System, error) {
		system, err := devices.ParseSystemConfigFile(ctx, fv.SystemFile, opts...)
		if err != nil {
			return devices.System{}, fmt.Errorf("failed to parse system config file: %q: %w", fv.SystemFile, err)
		}
		return system, nil
	}
	return ctx, loader, nil
}

func (c *Control) Run(ctx context.Context, flags any, args []string) error {
	ctx, loader, err := c.setup(ctx, flags.(*ControlFlags))
	if err != nil {
		return err
	}
	action, err := webapi.NewActionFromArgs(args[0], args[1:]...)
	if err != nil {
		return err
	}
	cc, err := webapi.NewDeviceControlServer(ctx, loader, c.logger)
	if err != nil {
		return err
	}
	data, err := cc.RunOperation(ctx, os.Stdout, action)
	if err != nil {
		return err
	}
	return writeJSON(os.Stdout, data)
}

func writeJSON(w io.Writer, v interface{}) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func (c *Control) Condition(ctx context.Context, flags any, args []string) error {
	ctx, loader, err := c.setup(ctx, flags.(*ControlFlags))
	if err != nil {
		return err
	}
	action, err := webapi.NewActionFromArgs(args[0], args[1:]...)
	if err != nil {
		return err
	}
	cc, err := webapi.NewDeviceControlServer(ctx, loader, c.logger)
	if err != nil {
		return err
	}
	cr, err := cc.RunCondition(ctx, os.Stdout, action)
	if err != nil {
		return err
	}
	return writeJSON(os.Stdout, cr)
}

func (c *Control) RunScript(ctx context.Context, flags any, args []string) error {
	ctx, loader, err := c.setup(ctx, &flags.(*ControlScriptFlags).ControlFlags)
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
	cc, err := webapi.NewDeviceControlServer(ctx, loader, c.logger)
	if err != nil {
		return err
	}
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) == 0 {
			continue
		}
		action, err := webapi.NewActionFromArgs(parts[0], parts[1:]...)
		if err != nil {
			return err
		}
		or, err := cc.RunOperation(ctx, os.Stdout, action)
		if err != nil {
			return err
		}
		if err := writeJSON(os.Stdout, or); err != nil {
			return err
		}
	}
	return nil
}

type conditionalOps struct {
	op   webapi.Action
	cond webapi.Action
}

func findPreconditions(ctx context.Context, system devices.System, fv *ControlTestPageFlags) ([]conditionalOps, error) {
	dedup := map[string]bool{}
	cops := make([]conditionalOps, 0, 10)

	scheds, err := loadSchedules(ctx, &fv.ConfigFileFlags, system)
	if err != nil {
		return nil, err
	}
	cal, err := scheduler.NewCalendar(scheds, system)
	if err != nil {
		return nil, err
	}
	year := time.Now().Year()
	first := datetime.NewCalendarDate(year, 1, 1)
	last := datetime.NewCalendarDate(year, 12, 31)
	for today := first; today <= last; today = today.Tomorrow() {
		for _, ce := range cal.Scheduled(today) {
			if ce.T.Precondition.Name == "" {
				continue
			}
			cond := webapi.Action{
				Device: ce.T.Precondition.Device,
				Op:     ce.T.Precondition.Name,
				Args:   ce.T.Precondition.Args,
			}
			op := webapi.Action{
				Device: ce.T.DeviceName,
				Op:     ce.T.Name,
				Args:   ce.T.Args,
			}
			key := op.String() + "_" + cond.String()
			if dedup[key] {
				continue
			}
			cops = append(cops, conditionalOps{
				op:   op,
				cond: cond,
			})
			dedup[key] = true

		}
	}
	sort.Slice(cops, func(i, j int) bool {
		return cops[i].op.Device < cops[j].op.Device
	})
	return cops, nil
}

func (c *Control) ServeTestPage(ctx context.Context, flags any, _ []string) error {
	fv := flags.(*ControlTestPageFlags)
	ctx, loader, err := c.setup(ctx, &fv.ControlFlags)
	if err != nil {
		return err
	}

	mux := http.NewServeMux()
	runner, url, err := fv.WebUIFlags.CreateWebServer(ctx, mux, c.logger)
	if err != nil {
		return err
	}

	tm := tableManager{html: true, jsapi: true}
	pages := fv.WebUIFlags.TestServerPages()

	rerender := func(ctx context.Context) (devices.System, error) {
		system, err := loader(ctx)
		if err != nil {
			return devices.System{}, err
		}
		cops, err := findPreconditions(ctx, system, fv)
		if err != nil {
			return devices.System{}, err
		}

		pages.SetPages(map[webassets.PageNames]string{
			webassets.ControllersPage:           tm.RenderHTML(tm.Controllers(system)),
			webassets.DevicesPage:               tm.RenderHTML(tm.Devices(system)),
			webassets.ConditionsPage:            tm.RenderHTML(tm.Conditions(system)),
			webassets.ControllerOperationsPage:  tm.RenderHTML(tm.ControllerOperations(system)),
			webassets.DeviceOperationsPage:      tm.RenderHTML(tm.DeviceOperations(system)),
			webassets.DeviceConditionsPage:      tm.RenderHTML(tm.DeviceConditions(system)),
			webassets.ConditionalOperationsPage: tm.RenderHTML(tm.ConditionalOperations(cops)),
		})

		return system, nil
	}

	webassets.AppendTestServerPages(mux,
		fv.ConfigFileFlags.SystemFile,
		pages,
	)

	dc, err := webapi.NewDeviceControlServer(ctx, rerender, c.logger)
	if err != nil {
		return err
	}
	dc.AppendEndpoints(ctx, mux)

	_ = browser.OpenURL(url)
	return runner()
}
