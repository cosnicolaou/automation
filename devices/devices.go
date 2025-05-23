// Copyright 2024 Cosmos Nicolaou. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package devices

import (
	"context"
	"fmt"
	"io"
	"time"

	"cloudeng.io/datetime"
	"gopkg.in/yaml.v3"
)

// Action represents an action to be performed.
type Action struct {
	Name       string    // Name of the operation
	DeviceName string    // Name of the device that is being controlled
	Device     Device    // The device instance
	Op         Operation // Operation to perform
	Args       []string  // Arguments to the operation
	Writer     io.Writer
}

// OperationArgs are the arguments to an operation.
type OperationArgs struct {
	Due    time.Time
	Place  datetime.Place
	Writer io.Writer
	Args   []string
}

// Controller represents a controller that can control devices.
// All devices are associated with a controller.
type Controller interface {
	SetConfig(ControllerConfigCommon)
	Config() ControllerConfigCommon
	SetSystem(System)
	System() System
	CustomConfig() any
	UnmarshalYAML(*yaml.Node) error
	Operations() map[string]Operation
	OperationsHelp() map[string]string
	Implementation() any
}

// Operation represents a single operation that can be performed on a device.
type Operation func(ctx context.Context, opts OperationArgs) (any, error)

// Condition represents a condition that can be evaluated to determine if an
// operation should be performed.
type Condition func(ctx context.Context, opts OperationArgs) (any, bool, error)

// Device represents a device that can be controlled, eg.
// a light fixturer, alarm zone etc.
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
	Conditions() map[string]Condition
	ConditionsHelp() map[string]string
}

type ZIPCodeLookup interface {
	Lookup(zip string) (float64, float64, error)
}

type Option func(*Options)

type Options struct {
	Interactive   io.Writer
	Devices       SupportedDevices
	Controllers   SupportedControllers
	loc           *time.Location
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

func WithTimeLocation(loc *time.Location) Option {
	return func(o *Options) {
		o.loc = loc
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

func CreateSystem(_ context.Context, controllerCfg []ControllerConfig, deviceCfg []DeviceConfig, opts ...Option) (map[string]Controller, map[string]Device, error) {
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
			return nil, fmt.Errorf("unsupported controller type: %q", ctrlcfg.Type)
		}
		if f == nil {
			return nil, fmt.Errorf("unsupported controller type, nil new function: %s", ctrlcfg.Type)
		}
		ctrl, err := f(ctrlcfg.Type, options)
		if err != nil {
			return nil, fmt.Errorf("failed to create controller %q: %w", ctrlcfg.Type, err)
		}
		ctrl.SetConfig(ctrlcfg.ControllerConfigCommon)
		if err := ctrl.UnmarshalYAML(&ctrlcfg.Config); err != nil {
			return nil, fmt.Errorf("failed to unmarshal controller %q: %w", ctrlcfg.Type, err)
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
			return nil, fmt.Errorf("device %q, unsupported device type: %q", devcfg.Name, devcfg.Type)
		}
		if f == nil {
			return nil, fmt.Errorf("device %q type, device type: %q, has no compiled in support", devcfg.Name, devcfg.Type)
		}
		dev, err := f(devcfg.Type, options)
		if err != nil {
			return nil, fmt.Errorf("device %q type, to create device %v: %w", devcfg.Name, devcfg.Type, err)
		}
		dev.SetConfig(devcfg.DeviceConfigCommon)
		if err := dev.UnmarshalYAML(&devcfg.Config); err != nil {
			return nil, fmt.Errorf("failed to unmarshal device %v: %w", devcfg.Type, err)
		}
		devices[devcfg.Name] = dev
	}
	return devices, nil
}

// ControllerBase represents a base implementation of a Controller parametized
// by a custom configuration type. Controllers can be created by embedding this
// type with the desired custom configuration type and overriding methods
// as needed and providing the Implementation method.
type ControllerBase[ConfigT any] struct {
	ControllerConfigCommon
	ControllerConfigCustom ConfigT
	system                 System
}

func (cb *ControllerBase[ConfigT]) SetConfig(c ControllerConfigCommon) {
	cb.ControllerConfigCommon = c
}

func (cb *ControllerBase[ConfigT]) Config() ControllerConfigCommon {
	return cb.ControllerConfigCommon
}

func (cb *ControllerBase[ConfigT]) SetSystem(s System) {
	cb.system = s
}

func (cb *ControllerBase[ConfigT]) System() System {
	return cb.system
}

func (cb *ControllerBase[ConfigT]) CustomConfig() any {
	return cb.ControllerConfigCustom
}

func (cb *ControllerBase[ConfigT]) UnmarshalYAML(node *yaml.Node) error {
	return node.Decode(&cb.ControllerConfigCustom)
}

func (cb *ControllerBase[ConfigT]) Operations() map[string]Operation {
	return map[string]Operation{}
}

func (cb *ControllerBase[ConfigT]) OperationsHelp() map[string]string {
	return map[string]string{}
}

// DeviceBase represents a base implementation of a Device parametized by a
// custom configuration type. Devices can be created by embedding this type with
// the desired custom configuration type and overriding methods as needed and
// providing the SetController, ControlledBy, Operations, and OperationsHelp methods.
type DeviceBase[ConfigT any] struct {
	DeviceConfigCommon
	DeviceConfigCustom ConfigT
}

func (db *DeviceBase[ConfigT]) SetConfig(c DeviceConfigCommon) {
	db.DeviceConfigCommon = c
}

func (db *DeviceBase[ConfigT]) Config() DeviceConfigCommon {
	return db.DeviceConfigCommon
}

func (db *DeviceBase[ConfigT]) CustomConfig() any {
	return db.DeviceConfigCustom
}

func (db *DeviceBase[ConfigT]) UnmarshalYAML(node *yaml.Node) error {
	return node.Decode(&db.DeviceConfigCustom)
}

func (db *DeviceBase[ConfigT]) ControlledByName() string {
	return db.ControllerName
}

func (db *DeviceBase[ConfigT]) Operations() map[string]Operation {
	return map[string]Operation{}
}

func (db *DeviceBase[ConfigT]) OperationsHelp() map[string]string {
	return map[string]string{}
}

func (db *DeviceBase[ConfigT]) Conditions() map[string]Condition {
	return map[string]Condition{}
}

func (db *DeviceBase[ConfigT]) ConditionsHelp() map[string]string {
	return map[string]string{}
}
