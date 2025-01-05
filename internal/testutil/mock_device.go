// Copyright 2024 Cosmos Nicolaou. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package testutil

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/cosnicolaou/automation/devices"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"gopkg.in/yaml.v3"
)

type DeviceDetail struct {
	Detail string `yaml:"detail"`
}

type MockDevice struct {
	devices.DeviceConfigCommon
	controller     devices.Controller
	Detail         DeviceDetail `yaml:",inline"`
	operations     map[string]devices.Operation
	operationsHelp map[string]string
	conditions     map[string]devices.Condition
	conditionsHelp map[string]string
	useLogger      bool
	useWriter      bool
}

func NewMockDevice(operations ...string) *MockDevice {
	d := &MockDevice{}
	d.operations = map[string]devices.Operation{}
	d.operationsHelp = map[string]string{}
	for _, op := range operations {
		kop := strings.ToLower(op)
		nop := cases.Title(language.English).String(op)
		d.operations[kop] = func(ctx context.Context, opts devices.OperationArgs) error {
			return d.genericOp(ctx, nop, opts)
		}
		d.operationsHelp[kop] = fmt.Sprintf("%s operation", nop)
	}
	d.conditions = map[string]devices.Condition{}
	d.conditionsHelp = map[string]string{}
	return d
}

func (d *MockDevice) SetOutput(logger bool, writer bool) {
	d.useLogger = logger
	d.useWriter = writer
}

func (d *MockDevice) AddCondition(name string, outcome bool) {
	d.conditions[name] = func(ctx context.Context, opts devices.OperationArgs) (bool, error) {
		return outcome, nil
	}
	d.conditionsHelp[name] = fmt.Sprintf("%s condition: outcome %v", name, outcome)
}

func (d *MockDevice) SetConfig(cfg devices.DeviceConfigCommon) {
	d.DeviceConfigCommon = cfg
}

func (d MockDevice) Config() devices.DeviceConfigCommon {
	return d.DeviceConfigCommon
}

func (d *MockDevice) CustomConfig() any {
	return d.Detail
}

func (d *MockDevice) UnmarshalYAML(node *yaml.Node) error {
	return node.Decode(&d.Detail)
}

func (d *MockDevice) Implementation() any {
	return d
}

func (d *MockDevice) SetController(c devices.Controller) {
	d.controller = c
}

func (d *MockDevice) ControlledByName() string {
	return d.Controller
}

func (d *MockDevice) ControlledBy() devices.Controller {
	return d.controller
}

func (d *MockDevice) Operations() map[string]devices.Operation {
	return d.operations
}

func (d *MockDevice) OperationsHelp() map[string]string {
	return d.operationsHelp
}

func (d *MockDevice) Conditions() map[string]devices.Condition {
	return d.conditions
}

func (d *MockDevice) ConditionsHelp() map[string]string {
	return d.conditionsHelp
}

func (d *MockDevice) Timeout() time.Duration {
	return time.Second
}

func (d *MockDevice) genericOp(_ context.Context, opName string, opts devices.OperationArgs) error {
	if d.useWriter {
		fmt.Fprintf(opts.Writer, "device[%s].%s: [%d] %v\n", d.Name, opName, len(opts.Args), strings.Join(opts.Args, "--"))
	}
	if d.useLogger {
		opts.Logger.Info("device", "name", d.Name, "op", opName)
	}
	return nil
}
