// Copyright 2024 Cosmos Nicolaou. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package devices_test

import (
	"context"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"cloudeng.io/datetime"
	"github.com/cosnicolaou/automation/devices"
	"github.com/cosnicolaou/automation/internal/testutil"
	"gopkg.in/yaml.v3"
)

const controllers_spec = `
  - name: c
    type: controller
    operations:
      enable: [on, command, quoted with space]
      disable: [off, command]
    detail: my-location
    key_id: my-key
`
const devices_spec = `
  - name: d
    controller: c
    type: device
    detail: my-device-d
    operations:
      on: [on, command]

  - name: e
    controller: c
    type: device
    operations:
      off: [off, command]
    detail: my-device-e
`

const simple_spec = `controllers:
` + controllers_spec + `
devices:
` + devices_spec

var supportedControllers = devices.SupportedControllers{
	"controller": func(string, devices.Options) (devices.Controller, error) {
		return &testutil.MockController{}, nil
	},
}

var supportedDevices = devices.SupportedDevices{
	"device": func(string, devices.Options) (devices.Device, error) {
		return testutil.NewMockDevice("on", "off"), nil
	},
}

func init() {
	devices.AvailableControllers = supportedControllers
	devices.AvailableDevices = supportedDevices
}

func compareOperationMaps(got, want map[string][]string) bool {
	if len(got) != len(want) {
		return false
	}
	for k, v := range got {
		if w, ok := want[k]; !ok || !slices.Equal(v, w) {
			return false
		}
	}
	return true
}

func TestParseConfig(t *testing.T) {
	ctx := context.Background()

	system, err := devices.ParseSystemConfig(ctx, []byte(simple_spec))
	if err != nil {
		t.Fatalf("failed to parse system config: %v", err)
	}
	ctrls := system.Controllers
	devs := system.Devices

	ctrl := ctrls["c"]
	dev := devs["d"]

	ccfg := ctrl.Config()
	if got, want := ccfg.Operations, (map[string][]string{
		"enable":  {"on", "command", "quoted with space"},
		"disable": {"off", "command"}}); !compareOperationMaps(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	ccfg.Operations = nil

	if got, want := ccfg, (devices.ControllerConfigCommon{
		Name: "c", Type: "controller"}); !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}

	if got, want := ctrl.(*testutil.MockController).CustomConfig().(testutil.ControllerDetail), (testutil.ControllerDetail{
		Detail: "my-location", KeyID: "my-key"}); !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}

	dcfg := dev.Config()
	if got, want := dcfg.Operations, (map[string][]string{
		"on": {"on", "command"}}); !compareOperationMaps(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	dcfg.Operations = nil
	if got, want := dcfg, (devices.DeviceConfigCommon{
		Name: "d", Controller: "c", Type: "device"}); !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}

	if got, want := dev.(*testutil.MockDevice).CustomConfig().(testutil.DeviceDetail), (testutil.DeviceDetail{Detail: "my-device-d"}); !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}

}

func TestBuildDevices(t *testing.T) {

	var ctrls []devices.ControllerConfig
	var devs []devices.DeviceConfig

	if err := yaml.Unmarshal([]byte(controllers_spec), &ctrls); err != nil {
		t.Fatalf("failed to unmarshal controllers: %v", err)
	}
	if err := yaml.Unmarshal([]byte(devices_spec), &devs); err != nil {
		t.Fatalf("failed to unmarshal devices: %v", err)
	}

	controllers, devices, err := devices.BuildDevices(ctrls, devs)

	if err != nil {
		t.Fatalf("failed to build devices: %v", err)
	}

	if got, want := len(controllers), 1; got != want {
		t.Errorf("got %d, want %d", got, want)
	}
	if got, want := len(devices), 2; got != want {
		t.Errorf("got %d, want %d", got, want)
	}

	for _, dev := range devices {
		if got, want := dev.ControlledByName(), "c"; got != want {
			t.Errorf("got %q, want %q", got, want)
		}
		if got, want := dev.ControlledBy(), controllers["c"]; got != want {
			t.Errorf("got %v, want %v", got, want)
		}
	}

	if got, want := devices["d"].(*testutil.MockDevice).Detail.Detail, "my-device-d"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}

	if got, want := devices["e"].(*testutil.MockDevice).Detail.Detail, "my-device-e"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}

}

func TestParseTZLocation(t *testing.T) {
	ctx := context.Background()
	gl := func(l string) *time.Location {
		if l == "" {
			return nil
		}
		loc, err := time.LoadLocation(l)
		if err != nil {
			t.Fatal(err)
		}
		return loc
	}
	for i, tc := range []struct {
		arg      string
		cfg      string
		expected *time.Location
	}{
		{"", "", gl("Local")},
		{"Local", "", gl("Local")},
		{"UTC", "", gl("UTC")},
		{"UTC", "time_zone: Local", gl("UTC")},
		{"", "time_zone:", gl("Local")},
		{"", "time_zone: America/New_York", gl("America/New_York")},
		{"UTC", "time_zone: America/New_York", gl("UTC")},
		{"America/New_York", "", gl("America/New_York")},
		{"America/Los_Angeles", "time_zone: America/New_York", gl("America/Los_Angeles")},
	} {
		spec := tc.cfg
		loc := gl(tc.arg)
		system, err := devices.ParseSystemConfig(ctx, []byte(spec), devices.WithTimeLocation(loc))
		if err != nil {
			t.Fatalf("failed to parse system config: %v", err)
		}
		if got, want := system.Location.TZ.String(), tc.expected.String(); got != want {
			t.Errorf("%v: got %q, want %q", i, got, want)
		}
	}
}

type ziplookup struct{}

func (ziplookup) Lookup(zip string) (float64, float64, error) {
	if zip == "94102" {
		return 200, -200, nil
	}
	return 100, -100, nil
}

func TestParsePlaceAndZIP(t *testing.T) {
	ctx := context.Background()
	spec := "time_zone:\nlatitude: 37.7749\nlongitude: 122.4194\nzip_code: 94102"

	system, err := devices.ParseSystemConfig(ctx, []byte(spec))
	if err != nil {
		t.Fatalf("failed to parse system config: %v", err)
	}
	if got, want := system.Location, (devices.Location{ZIPCode: "94102", Place: datetime.Place{TZ: time.Local, Latitude: 37.7749, Longitude: 122.4194}}); !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}

	system, err = devices.ParseSystemConfig(ctx, []byte(spec), devices.WithLatLong(23, 43), devices.WithZIPCode("12345"), devices.WithZIPCodeLookup(ziplookup{}))
	if err != nil {
		t.Fatalf("failed to parse system config: %v", err)
	}
	if got, want := system.Location, (devices.Location{ZIPCode: "12345", Place: datetime.Place{TZ: time.Local, Latitude: 23, Longitude: 43}}); !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}

	// Test zip code lookup, the zip that is looked up is the result of the
	// WithZIPCode option if one is given.

	system, err = devices.ParseSystemConfig(ctx, []byte("zip_code: 94102"), devices.WithZIPCode("12345"), devices.WithZIPCodeLookup(ziplookup{}))
	if err != nil {
		t.Fatalf("failed to parse system config: %v", err)
	}
	if got, want := system.Location, (devices.Location{ZIPCode: "12345", Place: datetime.Place{TZ: time.Local, Latitude: 100, Longitude: -100}}); !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}

	system, err = devices.ParseSystemConfig(ctx, []byte("zip_code: 94102"), devices.WithZIPCodeLookup(ziplookup{}))
	if err != nil {
		t.Fatalf("failed to parse system config: %v", err)
	}
	if got, want := system.Location, (devices.Location{ZIPCode: "94102", Place: datetime.Place{TZ: time.Local, Latitude: 200, Longitude: -200}}); !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestOperations(t *testing.T) {

	ctx := context.Background()
	system, err := devices.ParseSystemConfig(ctx, []byte(simple_spec))
	if err != nil {
		t.Fatalf("failed to parse system config: %v", err)
	}

	out := &strings.Builder{}
	args := devices.OperationArgs{
		Writer: out,
	}

	dev := system.Devices["d"]
	args.Args = dev.Config().Operations["on"]
	if err := dev.Operations()["on"](ctx, args); err != nil {
		t.Errorf("failed to perform operation: %v", err)
	}

	if got, want := out.String(), "device[d].On: [2] on--command\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}

	out.Reset()

	ctrl := system.Controllers["c"]
	args.Args = ctrl.Config().Operations["enable"]

	if err := ctrl.Operations()["enable"](ctx, args); err != nil {
		t.Errorf("failed to perform operation: %v", err)
	}
	if got, want := out.String(), "controller[c].Enable: [3] on--command--quoted with space\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}

	out.Reset()

	args.Args = ctrl.Config().Operations["disable"]
	if err := ctrl.Operations()["disable"](ctx, args); err != nil {
		t.Errorf("failed to perform operation: %v", err)
	}
	if got, want := out.String(), "controller[c].Disable: [2] off--command\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
