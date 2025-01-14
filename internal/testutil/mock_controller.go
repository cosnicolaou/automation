// Copyright 2024 Cosmos Nicolaou. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package testutil

import (
	"context"
	"fmt"
	"strings"

	"github.com/cosnicolaou/automation/devices"
)

type ControllerDetail struct {
	Detail string `yaml:"detail"`
	KeyID  string `yaml:"key_id"`
}

type MockController struct {
	devices.ControllerBase[ControllerDetail]
}

func (c *MockController) Enable(_ context.Context, opts devices.OperationArgs) error {
	fmt.Fprintf(opts.Writer, "controller[%s].Enable: [%d] %v\n", c.Name, len(opts.Args), strings.Join(opts.Args, "--"))
	return nil
}

func (c *MockController) Disable(_ context.Context, opts devices.OperationArgs) error {
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
