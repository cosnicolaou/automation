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

	"github.com/cosnicolaou/automation/devices"
)

type DeviceControlServer struct {
	mu       sync.Mutex
	loaded   devices.System
	reloader func(ctx context.Context) (devices.System, error)
	l        *slog.Logger
}

func (dc *DeviceControlServer) system() devices.System {
	dc.mu.Lock()
	defer dc.mu.Unlock()
	return dc.loaded
}

func (dc *DeviceControlServer) reload(ctx context.Context) error {
	dc.mu.Lock()
	defer dc.mu.Unlock()
	system, err := dc.reloader(ctx)
	if err != nil {
		return err
	}
	dc.loaded = system
	return nil
}

func (dc *DeviceControlServer) Reload(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	dc.l.Log(ctx, slog.LevelInfo, "reload", "request", r.URL.String())
	if err := dc.reload(ctx); err != nil {
		dc.httpError(ctx, w, r.URL, "reload", err.Error(), http.StatusInternalServerError)
		return
	}
}

func NewDeviceControlServer(ctx context.Context, systemLoader func(context.Context) (devices.System, error), l *slog.Logger) (*DeviceControlServer, error) {
	system, err := systemLoader(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load system: %v", err)
	}
	return &DeviceControlServer{
		reloader: systemLoader,
		loaded:   system,
		l:        l.With("component", "webapi"),
	}, nil
}

func (dc *DeviceControlServer) RunOperation(ctx context.Context, writer io.Writer, nameAndOp string, args []string) (*OperationResult, error) {
	parts := strings.Split(nameAndOp, ".")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid operation: %v, should be name.operation", nameAndOp)
	}
	name, op := parts[0], parts[1]
	_, cok := dc.system().Controllers[name]
	_, dok := dc.system().Devices[name]
	if !cok && !dok {
		return nil, fmt.Errorf("unknown controller or device: %v", name)
	}

	or := &OperationResult{
		Device: name,
		Op:     op,
	}

	if fn, pars, ok := dc.system().ControllerOp(name, op); ok {
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

	if fn, pars, ok := dc.system().DeviceOp(name, op); ok {
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

func (dc *DeviceControlServer) RunCondition(ctx context.Context, writer io.Writer, nameAndOp string, args []string) (*ConditionResult, error) {
	parts := strings.Split(nameAndOp, ".")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid condition: %v, should be name.condition", nameAndOp)
	}
	name, op := parts[0], parts[1]
	_, cok := dc.system().Controllers[name]
	_, dok := dc.system().Devices[name]
	if !cok && !dok {
		return nil, fmt.Errorf("unknown controller or device: %v", name)
	}

	cr := &ConditionResult{
		Device: name,
		Cond:   op,
	}
	if fn, pars, ok := dc.system().DeviceCondition(name, op); ok {
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

func (dc *DeviceControlServer) httpError(ctx context.Context, w http.ResponseWriter, u *url.URL, msg, err string, statusCode int) {
	dc.l.Log(ctx, slog.LevelInfo, msg, "request", u.String(), "code", statusCode, "error", err)
	http.Error(w, err, http.StatusBadRequest)
}

func (dc *DeviceControlServer) ServeOperation(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	dc.l.Log(ctx, slog.LevelInfo, "op-start", "request", r.URL.String(), "code", http.StatusOK)
	dev, op, args := decodeArgs(r)
	if dev == "" || op == "" {
		dc.httpError(ctx, w, r.URL, "op-end", "missing device or operation", http.StatusBadRequest)
		return
	}
	or, err := dc.RunOperation(ctx, io.Discard, dev+"."+op, args)
	if err != nil {
		dc.httpError(ctx, w, r.URL, "op-end", err.Error(), http.StatusInternalServerError)
		return
	}
	dc.serveJSON(ctx, w, r.URL, "op-end", or)
}

func (dc *DeviceControlServer) ServeCondition(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	dc.l.Log(ctx, slog.LevelInfo, "cond-start", "request", r.URL.String())
	dev, op, args := decodeArgs(r)
	if dev == "" || op == "" {
		dc.httpError(ctx, w, r.URL, "cond-end", "missing device or operation", http.StatusBadRequest)
		return
	}
	cr, err := dc.RunCondition(ctx, io.Discard, dev+"."+op, args)
	if err != nil {
		dc.httpError(ctx, w, r.URL, "cond-end", err.Error(), http.StatusInternalServerError)
		return
	}
	dc.serveJSON(ctx, w, r.URL, "cond-end", cr)
}

func (dc *DeviceControlServer) serveJSON(ctx context.Context, w http.ResponseWriter, u *url.URL, msg string, result any) {
	dc.l.Log(ctx, slog.LevelInfo, msg, "request", u.String(), "code", http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(result); err != nil {
		dc.httpError(ctx, w, u, msg, fmt.Sprintf("failed to encode json response: %v", err), http.StatusInternalServerError)
	}
}

func names(sys devices.System) (controllers, devices []string) {
	for k := range sys.Controllers {
		controllers = append(controllers, k)
	}
	for k := range sys.Devices {
		devices = append(devices, k)
	}
	slices.Sort(controllers)
	slices.Sort(devices)
	return
}

func (dc *DeviceControlServer) AppendEndpoints(ctx context.Context, mux *http.ServeMux) {

	mux.HandleFunc("/api/operation", func(w http.ResponseWriter, r *http.Request) {
		dc.ServeOperation(ctx, w, r)
	})

	mux.HandleFunc("/api/condition", func(w http.ResponseWriter, r *http.Request) {
		dc.ServeCondition(ctx, w, r)
	})

	mux.HandleFunc("/api/reload", func(w http.ResponseWriter, r *http.Request) {
		dc.Reload(ctx, w, r)
		sys := dc.system()
		cn, dn := names(sys)
		dc.serveJSON(ctx, w, r.URL, "reload", struct {
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
