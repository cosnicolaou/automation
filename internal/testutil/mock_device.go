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
	"gopkg.in/yaml.v3"
)

type DeviceDetail struct {
	Detail string `yaml:"detail"`
}

type MockDevice struct {
	devices.DeviceConfigCommon
	controller devices.Controller
	Detail     DeviceDetail `yaml:",inline"`
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
	return map[string]devices.Operation{
		"on":      d.On,
		"off":     d.Off,
		"another": d.Another,
	}
}

func (d *MockDevice) OperationsHelp() map[string]string {
	return map[string]string{
		"on":  "turn the device on",
		"off": "turn the device off",
	}
}

func (d *MockDevice) Timeout() time.Duration {
	return time.Second
}

func (d *MockDevice) On(ctx context.Context, opts devices.OperationArgs) error {
	fmt.Fprintf(opts.Writer, "device[%s].On: [%d] %v\n", d.Name, len(opts.Args), strings.Join(opts.Args, "--"))
	return nil
}

func (d *MockDevice) Off(ctx context.Context, opts devices.OperationArgs) error {
	fmt.Fprintf(opts.Writer, "device[%s].Off: [%d] %v\n", d.Name, len(opts.Args), strings.Join(opts.Args, "--"))
	return nil
}

func (d *MockDevice) Another(ctx context.Context, opts devices.OperationArgs) error {
	fmt.Fprintf(opts.Writer, "device[%s].Another: [%d] %v\n", d.Name, len(opts.Args), strings.Join(opts.Args, "--"))
	return nil
}
