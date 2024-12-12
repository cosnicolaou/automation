// Copyright 2024 Cosmos Nicolaou. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package devices

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/cosnicolaou/automation/net/streamconn"
	"gopkg.in/yaml.v3"
)

type Action struct {
	DeviceName string
	Device     Device
	Name       string
	Op         Operation
	Writer     io.Writer
	Args       []string
}

type OperationArgs struct {
	Writer io.Writer
	Logger *slog.Logger
	Args   []string
}

type Controller interface {
	SetConfig(ControllerConfigCommon)
	Config() ControllerConfigCommon
	CustomConfig() any
	UnmarshalYAML(*yaml.Node) error
	Operations() map[string]Operation
	OperationsHelp() map[string]string
	Implementation() any
}

type Operation func(ctx context.Context, opts OperationArgs) error

type Device interface {
	SetConfig(DeviceConfigCommon)
	Config() DeviceConfigCommon
	CustomConfig() any
	SetController(Controller)
	UnmarshalYAML(*yaml.Node) error
	ControlledByName() string
	ControlledBy() Controller
	Operations() map[string]Operation
	OperationsHelp() map[string]string
	Timeout() time.Duration
}

type ZIPCodeLookup interface {
	Lookup(zip string) (float64, float64, error)
}

type Option func(*Options)

type Options struct {
	Logger        *slog.Logger
	Interactive   io.Writer
	Session       streamconn.Session
	Devices       SupportedDevices
	Controllers   SupportedControllers
	tz            *time.Location
	latitude      float64
	longitude     float64
	zipCode       string
	zipCodeLookup ZIPCodeLookup
	Custom        any
}

func WithZIPCodeLookup(l ZIPCodeLookup) Option {
	return func(o *Options) {
		o.zipCodeLookup = l
	}
}

func WithTimeLocation(tz *time.Location) Option {
	return func(o *Options) {
		o.tz = tz
	}
}

func WithLatLong(lat, long float64) Option {
	return func(o *Options) {
		o.latitude = lat
		o.longitude = long
	}
}

func WithZIPCode(zip string) Option {
	return func(o *Options) {
		o.zipCode = zip
	}
}

func WithLogger(l *slog.Logger) Option {
	return func(o *Options) {
		o.Logger = l
	}
}

func WithSession(c streamconn.Session) Option {
	return func(o *Options) {
		o.Session = c
	}
}

func WithCustom(c any) Option {
	return func(o *Options) {
		o.Custom = c
	}
}

func WithDevices(d SupportedDevices) Option {
	return func(o *Options) {
		o.Devices = d
	}
}

func WithControllers(c SupportedControllers) Option {
	return func(o *Options) {
		o.Controllers = c
	}
}

type SupportedControllers map[string]func(typ string, opts Options) (Controller, error)

type SupportedDevices map[string]func(typ string, opts Options) (Device, error)

func BuildDevices(controllerCfg []ControllerConfig, deviceCfg []DeviceConfig, opts ...Option) (map[string]Controller, map[string]Device, error) {
	var options Options
	for _, opt := range opts {
		opt(&options)
	}
	controllers, err := CreateControllers(controllerCfg, options)
	if err != nil {
		return nil, nil, err
	}
	devices, err := CreateDevices(deviceCfg, options)
	if err != nil {
		return nil, nil, err
	}
	for _, dev := range devices {
		if ctrl, ok := controllers[dev.ControlledByName()]; ok {
			dev.SetController(ctrl)
		}
	}
	return controllers, devices, nil
}

func CreateControllers(config []ControllerConfig, options Options) (map[string]Controller, error) {
	controllers := map[string]Controller{}
	availableControllers := options.Controllers
	if availableControllers == nil {
		availableControllers = AvailableControllers
	}
	for _, ctrlcfg := range config {
		f, ok := availableControllers[ctrlcfg.Type]
		if !ok {
			return nil, fmt.Errorf("unsupported controller type: %s", ctrlcfg.Type)
		}
		if f == nil {
			return nil, fmt.Errorf("unsupported controller type, nil new function: %s", ctrlcfg.Type)
		}
		ctrl, err := f(ctrlcfg.Type, options)
		if err != nil {
			return nil, fmt.Errorf("failed to create controller %v: %w", ctrlcfg.Type, err)
		}
		ctrl.SetConfig(ctrlcfg.ControllerConfigCommon)
		if err := ctrl.UnmarshalYAML(&ctrlcfg.Config); err != nil {
			return nil, fmt.Errorf("failed to unmarshal controller %v: %w", ctrlcfg.Type, err)
		}
		controllers[ctrlcfg.Name] = ctrl
	}
	return controllers, nil
}

func CreateDevices(config []DeviceConfig, options Options) (map[string]Device, error) {
	devices := map[string]Device{}
	availableDevices := options.Devices
	if availableDevices == nil {
		availableDevices = AvailableDevices
	}
	for _, devcfg := range config {
		f, ok := availableDevices[devcfg.Type]
		if !ok {
			return nil, fmt.Errorf("unsupported device type: %s", devcfg.Type)
		}
		if f == nil {
			return nil, fmt.Errorf("unsupported device type, nil new function: %s", devcfg.Type)
		}
		dev, err := f(devcfg.Type, options)
		if err != nil {
			return nil, fmt.Errorf("failed to create device %v: %w", devcfg.Type, err)
		}
		dev.SetConfig(devcfg.DeviceConfigCommon)
		if err := dev.UnmarshalYAML(&devcfg.Config); err != nil {
			return nil, fmt.Errorf("failed to unmarshal device %v: %w", devcfg.Type, err)
		}
		devices[devcfg.Name] = dev
	}
	return devices, nil
}
