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
	"strings"

	"cloudeng.io/cmdutil/keystore"
	"github.com/cosnicolaou/automation/cmd/autobot/internal/webapi"
	"github.com/cosnicolaou/automation/cmd/autobot/internal/webassets"
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
	cmd := args[0]
	parameters := args[1:]
	cc, err := webapi.NewControlClient(ctx, loader, c.logger)
	if err != nil {
		return err
	}
	data, err := cc.RunOperation(ctx, os.Stdout, cmd, parameters)
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
	cmd := args[0]
	parameters := args[1:]
	cc, err := webapi.NewControlClient(ctx, loader, c.logger)
	if err != nil {
		return err
	}
	cr, err := cc.RunCondition(ctx, os.Stdout, cmd, parameters)
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
	cc, err := webapi.NewControlClient(ctx, loader, c.logger)
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
		cmd := parts[0]
		parameters := parts[1:]
		or, err := cc.RunOperation(ctx, os.Stdout, cmd, parameters)
		if err != nil {
			return err
		}
		if err := writeJSON(os.Stdout, or); err != nil {
			return err
		}
	}
	return nil
}

func (c *Control) ServeTestPage(ctx context.Context, flags any, _ []string) error {
	fv := flags.(*ControlTestPageFlags)
	ctx, loader, err := c.setup(ctx, &fv.ControlFlags)
	if err != nil {
		return err
	}

	mux := http.NewServeMux()
	_, runner, url, err := fv.WebUIFlags.CreateWebServer(ctx, mux)
	if err != nil {
		return err
	}

	tm := tableManager{html: true, jsapi: true}
	pages := fv.WebUIFlags.Pages()

	rerender := func(ctx context.Context) (devices.System, error) {
		system, err := loader(ctx)
		if err != nil {
			return devices.System{}, err
		}
		pages.SetPages(map[webassets.PageNames]string{
			webassets.ControllersPage:          tm.RenderHTML(tm.Controllers(system)),
			webassets.DevicesPage:              tm.RenderHTML(tm.Devices(system)),
			webassets.ConditionsPage:           tm.RenderHTML(tm.Conditions(system)),
			webassets.ControllerOperationsPage: tm.RenderHTML(tm.ControllerOperations(system)),
			webassets.DeviceOperationsPage:     tm.RenderHTML(tm.DeviceOperations(system)),
			webassets.DeviceConditionsPage:     tm.RenderHTML(tm.DeviceConditions(system)),
		})

		return system, nil
	}

	webapi.AppendTestServerEndpoints(mux,
		fv.ConfigFileFlags.SystemFile,
		pages,
	)

	cc, err := webapi.NewControlClient(ctx, rerender, c.logger)
	if err != nil {
		return err
	}
	webapi.AppendControlAPIEndpoints(ctx, cc, mux)

	_ = browser.OpenURL(url)
	return runner()
}
