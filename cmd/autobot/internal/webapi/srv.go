// Copyright 2025 Cosmos Nicolaou. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package webapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"

	"github.com/cosnicolaou/automation/cmd/autobot/internal/webassets"
	"github.com/cosnicolaou/automation/devices"
)

func AppendTestServerEndpoints(mux *http.ServeMux,
	cfg string,
	pages *webassets.Pages,
) {

	mux.Handle("/static/",
		http.StripPrefix("/static/", http.FileServer(pages.FS())))

	mux.HandleFunc("/controllers", func(w http.ResponseWriter, _ *http.Request) {
		err := pages.RunOpsPage(w, cfg, "controller operations", webassets.ControllerOperationsPage)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	mux.HandleFunc("/devices", func(w http.ResponseWriter, _ *http.Request) {
		err := pages.RunOpsPage(w, cfg, "device operations", webassets.DeviceOperationsPage)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	mux.HandleFunc("/conditions", func(w http.ResponseWriter, _ *http.Request) {
		err := pages.RunOpsPage(w, cfg, "device conditions", webassets.DeviceConditionsPage)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		err := pages.TestPageHome(w, cfg)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})
}

type Control struct {
	mu       sync.Mutex
	loaded   devices.System
	reloader func(ctx context.Context) (devices.System, error)
	l        *slog.Logger
}

func (c *Control) system() devices.System {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.loaded
}

func (c *Control) reload(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	system, err := c.reloader(ctx)
	if err != nil {
		return err
	}
	c.loaded = system
	return nil
}

func (c *Control) Reload(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	c.l.Log(ctx, slog.LevelInfo, "reload", "request", r.URL.String())
	if err := c.reload(ctx); err != nil {
		c.httpError(ctx, w, r.URL, "reload", err.Error(), http.StatusInternalServerError)
		return
	}
}

func NewControlClient(ctx context.Context, loader func(context.Context) (devices.System, error), l *slog.Logger) (*Control, error) {
	system, err := loader(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load system: %v", err)
	}
	return &Control{
		reloader: loader,
		loaded:   system,
		l:        l.With("component", "webapi"),
	}, nil
}

func (c *Control) RunOperation(ctx context.Context, writer io.Writer, nameAndOp string, args []string) (*OperationResult, error) {
	parts := strings.Split(nameAndOp, ".")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid operation: %v, should be name.operation", nameAndOp)
	}
	name, op := parts[0], parts[1]
	_, cok := c.system().Controllers[name]
	_, dok := c.system().Devices[name]
	if !cok && !dok {
		return nil, fmt.Errorf("unknown controller or device: %v", name)
	}

	or := &OperationResult{
		Device: name,
		Op:     op,
	}

	if fn, pars, ok := c.system().ControllerOp(name, op); ok {
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
		or.Args = args
		or.Data = result
		return or, nil
	}

	if fn, pars, ok := c.system().DeviceOp(name, op); ok {
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
		or.Args = args
		or.Data = result
		return or, nil
	}

	return nil, fmt.Errorf("unknown or not configured operation: %v, %v", name, op)
}

func (c *Control) RunCondition(ctx context.Context, writer io.Writer, nameAndOp string, args []string) (*ConditionResult, error) {
	parts := strings.Split(nameAndOp, ".")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid condition: %v, should be name.condition", nameAndOp)
	}
	name, op := parts[0], parts[1]
	_, cok := c.system().Controllers[name]
	_, dok := c.system().Devices[name]
	if !cok && !dok {
		return nil, fmt.Errorf("unknown controller or device: %v", name)
	}

	cr := &ConditionResult{
		Device: name,
		Cond:   op,
	}
	if fn, pars, ok := c.system().DeviceCondition(name, op); ok {
		if len(args) == 0 {
			args = pars
		}
		opts := devices.OperationArgs{
			Writer: writer,
			Args:   args,
		}
		data, result, err := fn(ctx, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to run condition: %v: %v", op, err)
		}
		cr.Args = args
		cr.Result = result
		cr.Data = data
		return cr, nil
	}

	return nil, fmt.Errorf("unknown or not configured condition: %v, %v", name, op)
}

func decodeArgs(r *http.Request) (string, string, []string) {
	pars := r.URL.Query()
	dev := pars.Get("dev")
	op := pars.Get("op")
	return dev, op, pars["arg"]
}

func (c *Control) httpError(ctx context.Context, w http.ResponseWriter, u *url.URL, msg, err string, statusCode int) {
	c.l.Log(ctx, slog.LevelInfo, msg, "request", u.String(), "code", statusCode, "error", err)
	http.Error(w, err, http.StatusBadRequest)
}

func (c *Control) ServeOperation(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	c.l.Log(ctx, slog.LevelInfo, "op-start", "request", r.URL.String(), "code", http.StatusOK)
	dev, op, args := decodeArgs(r)
	if dev == "" || op == "" {
		c.httpError(ctx, w, r.URL, "op-end", "missing device or operation", http.StatusBadRequest)
		return
	}
	or, err := c.RunOperation(ctx, io.Discard, dev+"."+op, args)
	if err != nil {
		c.httpError(ctx, w, r.URL, "op-end", err.Error(), http.StatusInternalServerError)
		return
	}
	c.serveJSON(ctx, w, r.URL, "op-end", or)
}

func (c *Control) ServeCondition(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	c.l.Log(ctx, slog.LevelInfo, "cond-start", "request", r.URL.String())
	dev, op, args := decodeArgs(r)
	if dev == "" || op == "" {
		c.httpError(ctx, w, r.URL, "cond-end", "missing device or operation", http.StatusBadRequest)
		return
	}
	cr, err := c.RunCondition(ctx, io.Discard, dev+"."+op, args)
	if err != nil {
		c.httpError(ctx, w, r.URL, "cond-end", err.Error(), http.StatusInternalServerError)
		return
	}
	c.serveJSON(ctx, w, r.URL, "cond-end", cr)
}

func (c *Control) serveJSON(ctx context.Context, w http.ResponseWriter, u *url.URL, msg string, result any) {
	c.l.Log(ctx, slog.LevelInfo, msg, "request", u.String(), "code", http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(result); err != nil {
		c.httpError(ctx, w, u, msg, fmt.Sprintf("failed to encode json response: %v", err), http.StatusInternalServerError)
	}
}

func names(c devices.System) (controllers, devices []string) {
	for k := range c.Controllers {
		controllers = append(controllers, k)
	}
	for k := range c.Devices {
		devices = append(devices, k)
	}
	slices.Sort(controllers)
	slices.Sort(devices)
	return
}

func AppendControlAPIEndpoints(ctx context.Context, c *Control, mux *http.ServeMux) {

	mux.HandleFunc("/api/operation", func(w http.ResponseWriter, r *http.Request) {
		c.ServeOperation(ctx, w, r)
	})

	mux.HandleFunc("/api/condition", func(w http.ResponseWriter, r *http.Request) {
		c.ServeCondition(ctx, w, r)
	})

	mux.HandleFunc("/api/reload", func(w http.ResponseWriter, r *http.Request) {
		c.Reload(ctx, w, r)
		sys := c.system()
		cn, dn := names(sys)
		c.serveJSON(ctx, w, r.URL, "reload", struct {
			Controllers []string `json:"controllers"`
			Devices     []string `json:"devices"`
		}{
			Controllers: cn,
			Devices:     dn,
		})
	})

}

type OperationResult struct {
	Device string   `json:"device"`
	Op     string   `json:"operation"`
	Args   []string `json:"args,omitempty"`
	Data   any      `json:"data,omitempty"`
}

type ConditionResult struct {
	Device string   `json:"device"`
	Cond   string   `json:"condition"`
	Args   []string `json:"args,omitempty"`
	Result bool     `json:"status"`
	Data   any      `json:"data,omitempty"`
}
