// Copyright 2024 Cosmos Nicolaou. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"cloudeng.io/cmdutil"
	"github.com/cosnicolaou/automation/devices"
	"github.com/cosnicolaou/automation/internal/webapi"
	"github.com/jedib0t/go-pretty/v6/table"
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
	Port string `subcmd:"port,8080,port to listen on"`
}

type Control struct {
	system devices.System
}

func (c *Control) runOp(ctx context.Context, system devices.System, writer io.Writer, nameAndOp string, args []string) error {
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
			Writer: writer,
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
			Writer: writer,
			Args:   args,
		}
		if err := fn(ctx, opts); err != nil {
			return fmt.Errorf("failed to run operation: %v: %v", op, err)
		}
		return nil
	}

	return fmt.Errorf("unknown or not configured operation: %v, %v", name, op)
}

func (c *Control) runCondition(ctx context.Context, system devices.System, writer io.Writer, nameAndOp string, args []string) (bool, error) {
	parts := strings.Split(nameAndOp, ".")
	if len(parts) != 2 {
		return false, fmt.Errorf("invalid condition: %v, should be name.condition", nameAndOp)
	}
	name, op := parts[0], parts[1]
	_, cok := system.Controllers[name]
	_, dok := system.Devices[name]
	if !cok && !dok {
		return false, fmt.Errorf("unknown controller or device: %v", name)
	}
	if fn, pars, ok := system.DeviceCondition(name, op); ok {
		if len(args) == 0 {
			args = pars
		}
		opts := devices.OperationArgs{
			Writer: writer,
			Args:   args,
		}
		result, err := fn(ctx, opts)
		if err != nil {
			return false, fmt.Errorf("failed to run condition: %v: %v", op, err)
		}
		return result, nil
	}

	return false, fmt.Errorf("unknown or not configured condition: %v, %v", name, op)
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
	if err := c.runOp(ctx, c.system, os.Stdout, cmd, parameters); err != nil {
		return err
	}
	return nil
}

func (c *Control) Condition(ctx context.Context, flags any, args []string) error {
	ctx, err := c.setup(ctx, flags.(*ControlFlags))
	if err != nil {
		return err
	}
	cmd := args[0]
	parameters := args[1:]
	result, err := c.runCondition(ctx, c.system, os.Stdout, cmd, parameters)
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
		if err := c.runOp(ctx, c.system, os.Stdout, cmd, parameters); err != nil {
			return err
		}
	}
	return nil
}

func renderHTML(t table.Writer) string {
	t.SetStyle(table.Style{
		HTML: table.HTMLOptions{
			CSSClass:    "table",
			EmptyColumn: "&nbsp;",
			EscapeText:  false,
			Newline:     "<br/>",
		}})
	return t.RenderHTML()
}

func decodeArgs(r *http.Request) (string, string, []string) {
	pars := r.URL.Query()
	dev := pars.Get("device")
	op := pars.Get("op")
	return dev, op, pars["arg"]
}

func (c *Control) serveOperation(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	dev, op, args := decodeArgs(r)
	if dev == "" || op == "" {
		http.Error(w, "missing device or operation", http.StatusBadRequest)
		return
	}
	if err := c.runOp(ctx, c.system, w, dev+"."+op, args); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (c *Control) serveCondition(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	_, _, _ = ctx, w, r
}

func (c *Control) ServeTestPage(ctx context.Context, flags any, _ []string) error {
	fv := flags.(*ControlTestPageFlags)
	ctx, err := c.setup(ctx, &fv.ControlFlags)
	if err != nil {
		return err
	}

	addr := fmt.Sprintf("127.0.0.1:%v", fv.Port)

	ctrl, dev, conds := newOperationsTables(c.system, addr)
	ctrlList, devList, devWithCondList := newDevicesTables(c.system)

	mux := http.NewServeMux()
	webapi.AppendTestServerEndpoints(mux,
		fv.ConfigFileFlags.SystemFile,
		renderHTML(ctrlList),
		renderHTML(devList),
		renderHTML(devWithCondList),
		renderHTML(ctrl),
		renderHTML(dev),
		renderHTML(conds),
	)

	mux.HandleFunc("/api/operation", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		c.serveOperation(ctx, w, r)
	})

	mux.HandleFunc("/api/condition", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		c.serveCondition(ctx, w, r)
	})

	server := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	fmt.Printf("running server at http://%v\n", addr)
	cmdutil.HandleSignals(func() {
		_ = server.Shutdown(ctx)
	}, os.Interrupt)
	_ = browser.OpenURL("http://" + addr)
	return server.ListenAndServe()
}
