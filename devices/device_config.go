// Copyright 2024 Cosmos Nicolaou. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package devices

import (
	"context"
	"time"

	"cloudeng.io/cmdutil/cmdyaml"
	"cloudeng.io/datetime"
	"gopkg.in/yaml.v3"
)

var (
	AvailableControllers = SupportedControllers{}
	AvailableDevices     = SupportedDevices{}
)

type parameters struct {
	Parameters []string `yaml:",flow"`
}

type ControllerConfigCommon struct {
	Name       string              `yaml:"name"`
	Type       string              `yaml:"type"`
	Operations map[string][]string `yaml:"operations"`
}

type ControllerConfig struct {
	ControllerConfigCommon
	Config yaml.Node `yaml:",inline"`
}

func (lp *ControllerConfig) UnmarshalYAML(node *yaml.Node) error {
	if err := node.Decode(&lp.ControllerConfigCommon); err != nil {
		return err
	}
	return node.Decode(&lp.Config)
}

type DeviceConfigCommon struct {
	Name       string              `yaml:"name"`
	Type       string              `yaml:"type"`
	Controller string              `yaml:"controller"`
	Operations map[string][]string `yaml:"operations"`
	Conditions map[string][]string `yaml:"conditions"`
}

type DeviceConfig struct {
	DeviceConfigCommon
	Config yaml.Node `yaml:",inline"`
}

func (lp *DeviceConfig) UnmarshalYAML(node *yaml.Node) error {
	if err := node.Decode(&lp.DeviceConfigCommon); err != nil {
		return err
	}
	return node.Decode(&lp.Config)
}

func locationFromValue(value string) (*time.Location, error) {
	if len(value) == 0 {
		return time.Now().Location(), nil
	}
	location, err := time.LoadLocation(value)
	if err != nil {
		return nil, err
	}
	return location, nil
}

type TimeZone struct {
	*time.Location
}

func (tz *TimeZone) UnmarshalYAML(node *yaml.Node) error {
	l, err := locationFromValue(node.Value)
	if err != nil {
		return err
	}
	tz.Location = l
	return nil
}

type LocationConfig struct {
	TZ        *TimeZone `yaml:"time_zone" cmd:"the timezone for the location in time.Location format"`
	ZIPCode   string    `yaml:"zip_code" cmd:"the zip/postal code for the location"`
	Latitude  float64   `yaml:"latitude" cmd:"the latitude for the location"`
	Longitude float64   `yaml:"longitude" cmd:"the longitude for the location"`
}

type Location struct {
	datetime.Place
	ZIPCode string
}

type SystemConfig struct {
	Location    LocationConfig     `yaml:",inline"`
	Controllers []ControllerConfig `yaml:"controllers" cmd:"the controllers that are being configured"`
	Devices     []DeviceConfig     `yaml:"devices" cmd:"the devices that are being configured"`
}

type System struct {
	Config      SystemConfig
	Location    Location
	Controllers map[string]Controller
	Devices     map[string]Device
}

func configuredAndExists(name string, configured map[string][]string, operations map[string]Operation) (Operation, bool) {
	if ops, ok := configured[name]; ok {
		for _, op := range ops {
			if fn, ok := operations[op]; ok {
				return fn, true
			}
		}
	}
	return nil, false

}

func (s System) ControllerConfigs(name string) (ControllerConfig, Controller, bool) {
	if ctrl, ok := s.Controllers[name]; ok {
		for _, cfg := range s.Config.Controllers {
			if cfg.Name == name {
				return cfg, ctrl, true
			}
		}
	}
	return ControllerConfig{}, nil, false
}

func (s System) DeviceConfigs(name string) (DeviceConfig, Device, bool) {
	if dev, ok := s.Devices[name]; ok {
		for _, cfg := range s.Config.Devices {
			if cfg.Name == name {
				return cfg, dev, true
			}
		}
	}
	return DeviceConfig{}, nil, false
}

// ControllerOp returns the operation function (and any configured parameters)
// for the specified operation on the named controller. The operation must be
// 'configured', ie. listed in the operations: list for the controller to be
// returned.
func (s System) ControllerOp(name, op string) (Operation, []string, bool) {
	if cfg, ctrl, ok := s.ControllerConfigs(name); ok {
		if fn, ok := ctrl.Operations()[op]; ok {
			if pars, ok := cfg.Operations[op]; ok {
				return fn, pars, true
			}
		}
	}
	return nil, nil, false
}

// DeviceOp returns the operation function (and any configured parameters)
// for the specified operation on the named device. The operation must be
// 'configured', ie. listed in the operations: list for the device to be
// returned.
func (s System) DeviceOp(name, op string) (Operation, []string, bool) {
	if cfg, dev, ok := s.DeviceConfigs(name); ok {
		if fn, ok := dev.Operations()[op]; ok {
			if pars, ok := cfg.Operations[op]; ok {
				return fn, pars, true
			}
		}
	}
	return nil, nil, false
}

// DeviceCondition returns the condition function (and any configured parameters)
// for the specified operation on the named controller. The condition must be
// 'configured', ie. listed in the conditions: list for the device to be
// returned.
func (s System) DeviceCondition(name, op string) (Condition, []string, bool) {
	if cfg, dev, ok := s.DeviceConfigs(name); ok {
		negation := false
		if op[0] == '!' {
			op = op[1:]
			negation = true
		}
		if fn, ok := dev.Conditions()[op]; ok {
			if pars, ok := cfg.Conditions[op]; ok {
				if negation {
					return func(ctx context.Context, opts OperationArgs) (bool, error) {
						ok, err := fn(ctx, opts)
						return !ok, err
					}, pars, true
				}
				return fn, pars, true
			}
		}
	}
	return nil, nil, false
}

// ParseSystemConfigFile parses the supplied configuration file as per ParseSystemConfig.
func ParseSystemConfigFile(ctx context.Context, cfgFile string, opts ...Option) (System, error) {
	var cfg SystemConfig
	if err := cmdyaml.ParseConfigFile(ctx, cfgFile, &cfg); err != nil {
		return System{}, err
	}
	return cfg.CreateSystem(opts...)
}

// ParseSystemConfig parses the supplied configuration data and returns
// a System using CreateSystem.
func ParseSystemConfig(ctx context.Context, cfgData []byte, opts ...Option) (System, error) {
	var cfg SystemConfig
	if err := yaml.Unmarshal(cfgData, &cfg); err != nil {
		return System{}, err
	}
	return cfg.CreateSystem(opts...)
}

func buildLocation(cfg LocationConfig, opts []Option) (Location, error) {
	var o Options
	for _, opt := range opts {
		opt(&o)
	}
	loc := Location{
		Place: datetime.Place{
			Latitude:  cfg.Latitude,
			Longitude: cfg.Longitude,
		},
		ZIPCode: cfg.ZIPCode,
	}
	if cfg.TZ != nil {
		loc.TZ = cfg.TZ.Location
	}
	if o.tz != nil {
		loc.TZ = o.tz
	}
	if loc.TZ == nil {
		tz, err := time.LoadLocation("Local")
		if err != nil {
			return loc, err
		}
		loc.TZ = tz
	}

	if o.latitude != 0 {
		loc.Latitude = o.latitude
	}
	if o.longitude != 0 {
		loc.Longitude = o.longitude
	}
	if o.zipCode != "" {
		loc.ZIPCode = o.zipCode
	}

	if loc.ZIPCode != "" && loc.Latitude == 0 && loc.Longitude == 0 && o.zipCodeLookup != nil {
		lat, long, err := o.zipCodeLookup.Lookup(loc.ZIPCode)
		if err != nil {
			return loc, err
		}
		loc.Latitude = lat
		loc.Longitude = long
	}
	return loc, nil
}

// CreateSystem creates a system from the supplied configuration.
// The place argument is used to set the location of the system if
// the location is not specified in the configuration. Note that if the
// time_zone: tag is specified in the configuration without a value
// then the location is set to the current time.Location, ie. timezone of 'Local'
// The WithTimeLocation, WithLatLong and WithZIPCode options can be used to
// override the location specified in the configuration. The WithZIPCodeLookup
// option must be supplied to enable the lookup of lat/long from a zip code.
func (cfg SystemConfig) CreateSystem(opts ...Option) (System, error) {
	loc, err := buildLocation(cfg.Location, opts)
	if err != nil {
		return System{}, err
	}
	ctrl, dev, err := CreateSystem(cfg.Controllers, cfg.Devices, opts...)
	if err != nil {
		return System{}, err
	}
	return System{
		Config:      cfg,
		Location:    loc,
		Controllers: ctrl,
		Devices:     dev,
	}, nil
}
