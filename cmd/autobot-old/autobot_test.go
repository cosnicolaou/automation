// Copyright 2024 Cosmos Nicolaou. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"maps"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cosnicolaou/automation/devices"
	"github.com/cosnicolaou/automation/internal/testutil"
)

var (
	supportedTestControllers = devices.SupportedControllers{
		"mock-controller": func(string, devices.Options) (devices.Controller, error) {
			return &testutil.MockController{}, nil
		},
	}

	supportedTestDevices = devices.SupportedDevices{
		"mock-device": func(string, devices.Options) (devices.Device, error) {
			md := testutil.NewMockDevice("On", "Off", "Another")
			md.AddCondition("weather", true)
			md.SetOutput(true, false)
			return md, nil
		}}
)

func init() {
	maps.Insert(devices.AvailableControllers,
		maps.All(supportedTestControllers))
	maps.Insert(devices.AvailableDevices,
		maps.All(supportedTestDevices))
}

func TestConfig(t *testing.T) {
	ctx := context.Background()
	var out strings.Builder
	config := &Config{out: &out}
	fl := &ConfigFlags{
		ConfigFileFlags: ConfigFileFlags{
			SystemFile:   filepath.Join("testdata", "system.yaml"),
			KeysFile:     filepath.Join("testdata", "keys.yaml"),
			ScheduleFile: filepath.Join("testdata", "schedule.yaml"),
		},
	}
	if err := config.Display(ctx, fl, []string{}); err != nil {
		t.Fatalf("failed to display config: %v", err)
	}
	o := out.String()
	for _, s := range []string{
		"key1[user1]",
		"Location: {{Local 37.3547 -122.0862} CA 94024}",
		"type: mock-controller",
		"type: mock-device",
		"controller: controller",
		"on 00:01:00",
		"at most 2 times",
		"if !weather [sunny]",
	} {
		if !strings.Contains(o, s) {
			t.Errorf("failed to find %q in output", s)
		}
	}
}
