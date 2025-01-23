// Copyright 2025 Cosmos Nicolaou. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package webapi

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/cosnicolaou/automation/cmd/autobot/internal/webassets"
	"github.com/cosnicolaou/automation/devices"
)

func AppendTestServerEndpoints(mux *http.ServeMux,
	cfg string,
	pages webassets.Pages,
	controllersTable string,
	devicesTable string,
	conditionsTable string,
	controllers string,
	devices string,
	conditions string,
) {

	mux.Handle("/static/",
		http.StripPrefix("/static/", http.FileServer(pages.FS())))

	mux.HandleFunc("/index.html", func(w http.ResponseWriter, _ *http.Request) {
		err := pages.TestPageIndex(w, cfg, controllersTable, devicesTable, conditionsTable)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	mux.HandleFunc("/controllers", func(w http.ResponseWriter, _ *http.Request) {
		err := pages.RunOpsPage(w, cfg, "controller operations", controllers)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	mux.HandleFunc("/devices", func(w http.ResponseWriter, _ *http.Request) {
		err := pages.RunOpsPage(w, cfg, "device operations", devices)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	mux.HandleFunc("/conditions", func(w http.ResponseWriter, _ *http.Request) {
		err := pages.RunOpsPage(w, cfg, "device conditions", conditions)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/index.html", http.StatusMovedPermanently)
	})
}

type Control struct {
	system devices.System
	l      *slog.Logger
}

func NewControlClient(system devices.System, l *slog.Logger) Control {
	return Control{
		system: system,
		l:      l.With("component", "webapi"),
	}
}

func (c Control) RunOperation(ctx context.Context, writer io.Writer, nameAndOp string, args []string) (any, error) {
	parts := strings.Split(nameAndOp, ".")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid operation: %v, should be name.operation", nameAndOp)
	}
	name, op := parts[0], parts[1]
	_, cok := c.system.Controllers[name]
	_, dok := c.system.Devices[name]
	if !cok && !dok {
		return nil, fmt.Errorf("unknown controller or device: %v", name)
	}

	if fn, pars, ok := c.system.ControllerOp(name, op); ok {
		if len(args) == 0 {
			args = pars
		}
		opts := devices.OperationArgs{
			Writer: writer,
			Args:   args,
		}
		result, err := fn(ctx, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to run operation: %v: %v", op, err)
		}
		return result, nil
	}

	if fn, pars, ok := c.system.DeviceOp(name, op); ok {
		if len(args) == 0 {
			args = pars
		}
		opts := devices.OperationArgs{
			Writer: writer,
			Args:   args,
		}
		result, err := fn(ctx, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to run operation: %v: %v", op, err)
		}
		return result, nil
	}

	return nil, fmt.Errorf("unknown or not configured operation: %v, %v", name, op)
}

func (c Control) RunCondition(ctx context.Context, writer io.Writer, nameAndOp string, args []string) (any, bool, error) {
	parts := strings.Split(nameAndOp, ".")
	if len(parts) != 2 {
		return nil, false, fmt.Errorf("invalid condition: %v, should be name.condition", nameAndOp)
	}
	name, op := parts[0], parts[1]
	_, cok := c.system.Controllers[name]
	_, dok := c.system.Devices[name]
	if !cok && !dok {
		return nil, false, fmt.Errorf("unknown controller or device: %v", name)
	}
	if fn, pars, ok := c.system.DeviceCondition(name, op); ok {
		if len(args) == 0 {
			args = pars
		}
		opts := devices.OperationArgs{
			Writer: writer,
			Args:   args,
		}
		data, result, err := fn(ctx, opts)
		if err != nil {
			return nil, false, fmt.Errorf("failed to run condition: %v: %v", op, err)
		}
		return data, result, nil
	}

	return nil, false, fmt.Errorf("unknown or not configured condition: %v, %v", name, op)
}

func decodeArgs(r *http.Request) (string, string, []string) {
	pars := r.URL.Query()
	dev := pars.Get("dev")
	op := pars.Get("op")
	return dev, op, pars["arg"]
}

func (c Control) httpError(ctx context.Context, w http.ResponseWriter, u *url.URL, msg, err string, statusCode int) {
	c.l.Log(ctx, slog.LevelInfo, msg, "request", u.String(), "code", statusCode, "error", err)
	http.Error(w, "missing device or operation", http.StatusBadRequest)
}

func (c Control) ServeOperation(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	c.l.Log(ctx, slog.LevelInfo, "op-start", "request", r.URL.String(), "code", http.StatusOK)
	dev, op, args := decodeArgs(r)
	if dev == "" || op == "" {
		c.httpError(ctx, w, r.URL, "op-end", "missing device or operation", http.StatusBadRequest)
		return
	}
	if err := c.RunOperation(ctx, w, dev+"."+op, args); err != nil {
		c.httpError(ctx, w, r.URL, "op-end", err.Error(), http.StatusInternalServerError)
		return
	}
	c.serveJSON(ctx, w, r.URL, "op-end", OperationResult{
		Device: dev,

		Op:     op,
		Args:   args,
		Status: true,
		Data:   nil,
	})
	c.l.Log(ctx, slog.LevelInfo, "op-end", "request", r.URL.String(), "code", http.StatusOK)
}

func (c Control) ServeCondition(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	c.l.Log(ctx, slog.LevelInfo, "cond-start", "request", r.URL.String())
	dev, op, args := decodeArgs(r)
	if dev == "" || op == "" {
		c.httpError(ctx, w, r.URL, "cond-end", "missing device or operation", http.StatusBadRequest)
		return
	}
	result, err := c.RunCondition(ctx, w, dev+"."+op, args)
	if err != nil {
		c.httpError(ctx, w, r.URL, "cond-end", err.Error(), http.StatusInternalServerError)
		return
	}
	fmt.Fprintf(w, "%v.%v(%v) => %v", html.EscapeString(dev), html.EscapeString(op), html.EscapeString(strings.Join(args, ", ")), result)
	c.l.Log(ctx, slog.LevelInfo, "cond-end", "request", r.URL.String(), "code", http.StatusOK)
}

func (c Control) serveJSON(ctx context.Context, w http.ResponseWriter, u *url.URL, msg string, result any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(result); err != nil {
		c.httpError(ctx, w, u, msg, fmt.Sprintf("failed to encode json response: %v", err), http.StatusInternalServerError)
	}
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

type OperationResult struct {
	Device string   `json:"device"`
	Op     string   `json:"operation"`
	Args   []string `json:"args"`
	Status bool     `json:"status"`
	Data   any      `json:"data"`
}

type ConditionResult struct {
	Device string   `json:"device"`
	Cond   string   `json:"condition"`
	Args   []string `json:"args"`
	Status bool     `json:"status"`
}
