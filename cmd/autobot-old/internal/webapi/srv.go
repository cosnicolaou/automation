// Copyright 2025 Cosmos Nicolaou. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package webapi

import (
	"context"
	"fmt"
	"html"
	"io"
	"net/http"
	"strings"

	"github.com/cosnicolaou/automation/cmd/autobot/internal/webassets"
	"github.com/cosnicolaou/automation/devices"
)

func AppendTestServerEndpoints(mux *http.ServeMux,
	cfg string,
	controllersTable string,
	devicesTable string,
	conditionsTable string,
	controllers string,
	devices string,
	conditions string,
) {
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/index.html", http.StatusMovedPermanently)
	})
	mux.HandleFunc("/index.html", func(w http.ResponseWriter, _ *http.Request) {
		err := webassets.TestPageIndex(w, cfg, controllersTable, devicesTable, conditionsTable)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})
	mux.HandleFunc("/controllers", func(w http.ResponseWriter, _ *http.Request) {
		err := webassets.RunOpsPage(w, cfg, "controller operations", controllers)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})
	mux.HandleFunc("/devices", func(w http.ResponseWriter, _ *http.Request) {
		err := webassets.RunOpsPage(w, cfg, "device operations", devices)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})
	mux.HandleFunc("/conditions", func(w http.ResponseWriter, _ *http.Request) {
		err := webassets.RunOpsPage(w, cfg, "device conditions", conditions)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})
}

type Control struct {
	system devices.System
}

func NewControlClient(system devices.System) Control {
	return Control{
		system: system,
	}
}

func (c Control) RunOperation(ctx context.Context, writer io.Writer, nameAndOp string, args []string) error {
	parts := strings.Split(nameAndOp, ".")
	if len(parts) != 2 {
		return fmt.Errorf("invalid operation: %v, should be name.operation", nameAndOp)
	}
	name, op := parts[0], parts[1]
	_, cok := c.system.Controllers[name]
	_, dok := c.system.Devices[name]
	if !cok && !dok {
		return fmt.Errorf("unknown controller or device: %v", name)
	}

	if fn, pars, ok := c.system.ControllerOp(name, op); ok {
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

	if fn, pars, ok := c.system.DeviceOp(name, op); ok {
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

func (c Control) RunCondition(ctx context.Context, writer io.Writer, nameAndOp string, args []string) (bool, error) {
	parts := strings.Split(nameAndOp, ".")
	if len(parts) != 2 {
		return false, fmt.Errorf("invalid condition: %v, should be name.condition", nameAndOp)
	}
	name, op := parts[0], parts[1]
	_, cok := c.system.Controllers[name]
	_, dok := c.system.Devices[name]
	if !cok && !dok {
		return false, fmt.Errorf("unknown controller or device: %v", name)
	}
	if fn, pars, ok := c.system.DeviceCondition(name, op); ok {
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

func decodeArgs(r *http.Request) (string, string, []string) {
	pars := r.URL.Query()
	dev := pars.Get("device")
	op := pars.Get("op")
	return dev, op, pars["arg"]
}

func (c Control) ServeOperation(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	dev, op, args := decodeArgs(r)
	if dev == "" || op == "" {
		http.Error(w, "missing device or operation", http.StatusBadRequest)
		return
	}
	if err := c.RunOperation(ctx, w, dev+"."+op, args); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (c Control) ServeCondition(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	dev, op, args := decodeArgs(r)
	if dev == "" || op == "" {
		http.Error(w, "missing device or operation", http.StatusBadRequest)
		return
	}
	result, err := c.RunCondition(ctx, w, dev+"."+op, args)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	fmt.Fprintf(w, "%v.%v(%v) => %v", html.EscapeString(dev), html.EscapeString(op), html.EscapeString(strings.Join(args, ", ")), result)
}

func AppendControlAPIEndpoints(ctx context.Context, c Control, mux *http.ServeMux) {

	mux.HandleFunc("/api/operation", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		c.ServeOperation(ctx, w, r)
	})

	mux.HandleFunc("/api/condition", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		c.ServeCondition(ctx, w, r)
	})

}
