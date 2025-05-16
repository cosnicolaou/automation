// Copyright 2025 Cosmos Nicolaou. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package webapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"

	"cloudeng.io/logging/ctxlog"
	"github.com/cosnicolaou/automation/devices"
)

type DeviceControlServer struct {
	mu       sync.Mutex
	loaded   devices.System
	reloader func(ctx context.Context) (devices.System, error)
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
	ctxlog.Info(ctx, "reload", "request", r.URL.String())
	if err := dc.reload(ctx); err != nil {
		dc.httpError(ctx, w, r.URL, "reload", err.Error(), http.StatusInternalServerError)
		return
	}
}

func NewDeviceControlServer(ctx context.Context, systemLoader func(context.Context) (devices.System, error)) (*DeviceControlServer, error) {
	system, err := systemLoader(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load system: %v", err)
	}
	return &DeviceControlServer{
		reloader: systemLoader,
		loaded:   system,
	}, nil
}

// NewActionFromArgs creates a new Action from a
// device.{operation,condition} string and optional arguments.
func NewActionFromArgs(devOp string, args ...string) (Action, error) {
	parts := strings.Split(devOp, ".")
	if len(parts) != 2 {
		return Action{}, fmt.Errorf("invalid: %q, should be device.operation/condition", devOp)
	}
	return Action{
		Device: parts[0],
		Op:     parts[1],
		Args:   args,
	}, nil
}

// Action is a device.{operation,condition} string and optional
// arguments and is used to represent an operation or condition
type Action struct {
	Device string
	Op     string
	Args   []string
}

func (a Action) String() string {
	return fmt.Sprintf("%v.%v(%v)", a.Device, a.Op, strings.Join(a.Args, ", "))
}

// RunOperationConditionally runs an operation iff the condition
// is true.
func (dc *DeviceControlServer) RunOperationConditionally(ctx context.Context, writer io.Writer, action, condition Action) (*OperationResult, error) {
	ctx = ctxlog.WithAttributes(ctx, "component", "webapi")
	cr, err := dc.RunCondition(ctx, io.Discard, condition)
	if err != nil {
		return nil, fmt.Errorf("failed to run condition: %v: %v", condition.Op, err)
	}
	if !cr.Result {
		return nil, fmt.Errorf("condition not met: %v", condition.Op)
	}
	or, err := dc.RunOperation(ctx, writer, action)
	if err != nil {
		return nil, fmt.Errorf("failed to run operation: %v: %v", action.Op, err)
	}
	return or, nil
}

func (dc *DeviceControlServer) RunOperation(ctx context.Context, writer io.Writer, action Action) (*OperationResult, error) {
	ctx = ctxlog.WithAttributes(ctx, "component", "webapi")
	_, cok := dc.system().Controllers[action.Device]
	_, dok := dc.system().Devices[action.Device]
	if !cok && !dok {
		return nil, fmt.Errorf("unknown controller or device: %v", action.Device)
	}

	or := &OperationResult{
		Device: action.Device,
		Op:     action.Op,
	}

	if fn, pars, ok := dc.system().ControllerOp(action.Device, action.Op); ok {
		if len(action.Args) == 0 {
			action.Args = pars
		}
		opts := devices.OperationArgs{
			Writer: writer,
			Args:   action.Args,
		}
		result, err := fn(ctx, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to run operation: %v: %v", action.Op, err)
		}
		or.Args = action.Args
		or.Data = result
		return or, nil
	}

	if fn, pars, ok := dc.system().DeviceOp(action.Device, action.Op); ok {
		if len(action.Args) == 0 {
			action.Args = pars
		}
		opts := devices.OperationArgs{
			Writer: writer,
			Args:   action.Args,
		}
		result, err := fn(ctx, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to run operation: %v: %v", action.Op, err)
		}
		or.Args = action.Args
		or.Data = result
		return or, nil
	}

	return nil, fmt.Errorf("unknown or not configured operation: %v, %v", action.Device, action.Op)
}

func (dc *DeviceControlServer) RunCondition(ctx context.Context, writer io.Writer, action Action) (*ConditionResult, error) {
	ctx = ctxlog.WithAttributes(ctx, "component", "webapi")
	_, cok := dc.system().Controllers[action.Device]
	_, dok := dc.system().Devices[action.Device]
	if !cok && !dok {
		return nil, fmt.Errorf("unknown controller or device: %v", action.Device)
	}
	cr := &ConditionResult{
		Device: action.Device,
		Cond:   action.Op,
	}
	if fn, pars, ok := dc.system().DeviceCondition(action.Device, action.Op); ok {
		if len(action.Args) == 0 {
			action.Args = pars
		}
		opts := devices.OperationArgs{
			Writer: writer,
			Args:   action.Args,
		}
		data, result, err := fn(ctx, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to run condition: %v: %v", action.Op, err)
		}
		cr.Args = action.Args
		cr.Result = result
		cr.Data = data
		return cr, nil
	}

	return nil, fmt.Errorf("unknown or not configured condition: %v, %v", action.Device, action.Op)
}

func decodeOperationArgs(r *http.Request) (Action, error) {
	pars := r.URL.Query()
	a := Action{
		Device: pars.Get("odev"),
		Op:     pars.Get("op"),
		Args:   pars["oarg"],
	}
	if a.Device == "" || a.Op == "" {
		return Action{}, fmt.Errorf("missing device or operation")
	}
	return a, nil
}

func decodeConditionArgs(r *http.Request) (Action, error) {
	pars := r.URL.Query()
	a := Action{
		Device: pars.Get("cdev"),
		Op:     pars.Get("cond"),
		Args:   pars["carg"],
	}
	if a.Device == "" || a.Op == "" {
		return Action{}, fmt.Errorf("missing device or condition")
	}
	return a, nil
}

func (dc *DeviceControlServer) httpError(ctx context.Context, w http.ResponseWriter, u *url.URL, msg, err string, statusCode int) {
	ctxlog.Info(ctx, msg, "component", "webapi", "request", u.String(), "code", statusCode, "error", err)
	http.Error(w, err, http.StatusBadRequest)
}

func (dc *DeviceControlServer) ServeOperation(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	ctx = ctxlog.WithAttributes(ctx, "component", "webapi", "request", r.URL.String())
	ctxlog.Info(ctx, "op-start")
	action, err := decodeOperationArgs(r)
	if err != nil {
		dc.httpError(ctx, w, r.URL, "op-end", err.Error(), http.StatusBadRequest)
		return
	}

	or, err := dc.RunOperation(ctx, io.Discard, action)
	if err != nil {
		dc.httpError(ctx, w, r.URL, "op-end", err.Error(), http.StatusInternalServerError)
		return
	}
	dc.serveJSON(ctx, w, r.URL, "op-end", or)
}

func (dc *DeviceControlServer) ServeOperationConditionally(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	ctx = ctxlog.WithAttributes(ctx, "component", "webapi", "request", r.URL.String())
	ctxlog.Info(ctx, "op-start")
	opAction, err := decodeOperationArgs(r)
	if err != nil {
		dc.httpError(ctx, w, r.URL, "op-end", err.Error(), http.StatusBadRequest)
		return
	}
	condAction, err := decodeConditionArgs(r)
	if err != nil {
		dc.httpError(ctx, w, r.URL, "op-end", err.Error(), http.StatusBadRequest)
		return
	}
	cr, err := dc.RunCondition(ctx, io.Discard, condAction)
	if err != nil {
		dc.httpError(ctx, w, r.URL, "op-end", err.Error(), http.StatusInternalServerError)
	}
	if !cr.Result {
		dc.serveJSON(ctx, w, r.URL, "op-end", ConditionalOperationResult{Condition: cr})
		return
	}
	or, err := dc.RunOperation(ctx, io.Discard, opAction)
	if err != nil {
		dc.httpError(ctx, w, r.URL, "op-end", err.Error(), http.StatusInternalServerError)
		return
	}
	dc.serveJSON(ctx, w, r.URL, "op-end", ConditionalOperationResult{
		Condition: cr,
		Operation: or,
	})
}

func (dc *DeviceControlServer) ServeCondition(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	ctx = ctxlog.WithAttributes(ctx, "component", "webapi", "request", r.URL.String())
	ctxlog.Info(ctx, "cond-start")
	action, err := decodeConditionArgs(r)
	if err != nil {
		dc.httpError(ctx, w, r.URL, "cond-end", err.Error(), http.StatusBadRequest)
		return
	}
	cr, err := dc.RunCondition(ctx, io.Discard, action)
	if err != nil {
		dc.httpError(ctx, w, r.URL, "cond-end", err.Error(), http.StatusInternalServerError)
		return
	}
	dc.serveJSON(ctx, w, r.URL, "cond-end", cr)
}

func (dc *DeviceControlServer) serveJSON(ctx context.Context, w http.ResponseWriter, u *url.URL, msg string, result any) {
	ctxlog.Info(ctx, msg, "code", http.StatusOK)
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

	mux.HandleFunc("/api/conditionally", func(w http.ResponseWriter, r *http.Request) {
		dc.ServeOperationConditionally(ctx, w, r)
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

type ConditionalOperationResult struct {
	Condition *ConditionResult `json:"condition"`
	Operation *OperationResult `json:"operation,omitempty"`
}
