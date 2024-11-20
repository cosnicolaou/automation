// Copyright 2024 Cosmos Nicolaou. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package testutil

import (
	"context"
	"fmt"
	"strings"

	"github.com/cosnicolaou/automation/devices"
	"gopkg.in/yaml.v3"
)

type ControllerDetail struct {
	Detail string `yaml:"detail"`
	KeyID  string `yaml:"key_id"`
}

type MockController struct {
	devices.ControllerConfigCommon
	Detail ControllerDetail `yaml:",inline"`
}

func (c *MockController) SetConfig(cfg devices.ControllerConfigCommon) {
	c.ControllerConfigCommon = cfg
}

func (c MockController) Config() devices.ControllerConfigCommon {
	return c.ControllerConfigCommon
}

func (c *MockController) CustomConfig() any {
	return c.Detail
}

func (c *MockController) UnmarshalYAML(node *yaml.Node) error {
	return node.Decode(&c.Detail)
}

func (c *MockController) Enable(ctx context.Context, opts devices.OperationArgs) error {
	fmt.Fprintf(opts.Writer, "controller[%s].Enable: [%d] %v\n", c.Name, len(opts.Args), strings.Join(opts.Args, "--"))
	return nil
}

func (c *MockController) Disable(ctx context.Context, opts devices.OperationArgs) error {
	fmt.Fprintf(opts.Writer, "controller[%s].Disable: [%d] %v\n", c.Name, len(opts.Args), strings.Join(opts.Args, "--"))
	return nil
}

func (c *MockController) Operations() map[string]devices.Operation {
	return map[string]devices.Operation{"enable": c.Enable, "disable": c.Disable}
}

func (c *MockController) OperationsHelp() map[string]string {
	return map[string]string{
		"enable":  "enable the controller",
		"disable": "disable the controller",
	}
}

func (c *MockController) Implementation() any {
	return c
}
