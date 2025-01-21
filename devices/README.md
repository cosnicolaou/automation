# Package [github.com/cosnicolaou/automation/devices](https://pkg.go.dev/github.com/cosnicolaou/automation/devices?tab=doc)

```go
import github.com/cosnicolaou/automation/devices
```


## Variables
### AvailableControllers, AvailableDevices
```go
AvailableControllers = SupportedControllers{}
AvailableDevices = SupportedDevices{}

```



## Functions
### Func CreateControllers
```go
func CreateControllers(config []ControllerConfig, options Options) (map[string]Controller, error)
```

### Func CreateDevices
```go
func CreateDevices(config []DeviceConfig, options Options) (map[string]Device, error)
```

### Func CreateSystem
```go
func CreateSystem(_ context.Context, controllerCfg []ControllerConfig, deviceCfg []DeviceConfig, opts ...Option) (map[string]Controller, map[string]Device, error)
```



## Types
### Type Action
```go
type Action struct {
	DeviceName string
	Device     Device
	Name       string
	Op         Operation
	Writer     io.Writer
	Args       []string
}
```


### Type Condition
```go
type Condition func(ctx context.Context, opts OperationArgs) (bool, error)
```
Condition represents a condition that can be evaluated to determine if an
operation should be performed.


### Type Controller
```go
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
```


### Type ControllerBase
```go
type ControllerBase[ConfigT any] struct {
	ControllerConfigCommon
	ControllerConfigCustom ConfigT
	// contains filtered or unexported fields
}
```
ControllerBase represents a base implementation of a Controller parametized
by a custom configuration type. Controllers can be created by embedding this
type with the desired custom configuration type and overriding methods as
needed and providing the Implementation method.

### Methods

```go
func (cb *ControllerBase[ConfigT]) Config() ControllerConfigCommon
```


```go
func (cb *ControllerBase[ConfigT]) CustomConfig() any
```


```go
func (cb *ControllerBase[ConfigT]) Operations() map[string]Operation
```


```go
func (cb *ControllerBase[ConfigT]) OperationsHelp() map[string]string
```


```go
func (cb *ControllerBase[ConfigT]) SetConfig(c ControllerConfigCommon)
```


```go
func (cb *ControllerBase[ConfigT]) SetSystem(s System)
```


```go
func (cb *ControllerBase[ConfigT]) System() System
```


```go
func (cb *ControllerBase[ConfigT]) UnmarshalYAML(node *yaml.Node) error
```




### Type ControllerConfig
```go
type ControllerConfig struct {
	ControllerConfigCommon
	Config yaml.Node `yaml:",inline"`
}
```
ControllerConfig represents the configuration for a controller allowing for
delayed unmarshalling of the custom config field.

### Methods

```go
func (lp *ControllerConfig) UnmarshalYAML(node *yaml.Node) error
```




### Type ControllerConfigCommon
```go
type ControllerConfigCommon struct {
	Name        string `yaml:"name"`
	Type        string `yaml:"type"`
	RetryConfig `yaml:",inline"`
	Operations  map[string][]string `yaml:"operations"`
}
```
ControllerConfigCommon represents the common configuration for a controller.


### Type Device
```go
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
```


### Type DeviceBase
```go
type DeviceBase[ConfigT any] struct {
	DeviceConfigCommon
	DeviceConfigCustom ConfigT
}
```
DeviceBase represents a base implementation of a Device parametized by
a custom configuration type. Devices can be created by embedding this
type with the desired custom configuration type and overriding methods
as needed and providing the SetController, ControlledBy, Operations,
and OperationsHelp methods.

### Methods

```go
func (db *DeviceBase[ConfigT]) Conditions() map[string]Condition
```


```go
func (db *DeviceBase[ConfigT]) ConditionsHelp() map[string]string
```


```go
func (db *DeviceBase[ConfigT]) Config() DeviceConfigCommon
```


```go
func (db *DeviceBase[ConfigT]) ControlledByName() string
```


```go
func (db *DeviceBase[ConfigT]) CustomConfig() any
```


```go
func (db *DeviceBase[ConfigT]) Operations() map[string]Operation
```


```go
func (db *DeviceBase[ConfigT]) OperationsHelp() map[string]string
```


```go
func (db *DeviceBase[ConfigT]) SetConfig(c DeviceConfigCommon)
```


```go
func (db *DeviceBase[ConfigT]) UnmarshalYAML(node *yaml.Node) error
```




### Type DeviceConfig
```go
type DeviceConfig struct {
	DeviceConfigCommon
	Config yaml.Node `yaml:",inline"`
}
```
DeviceConfig represents the configuration for a device allowing for delayed
unmarshalling of the custom config field.

### Methods

```go
func (lp *DeviceConfig) UnmarshalYAML(node *yaml.Node) error
```




### Type DeviceConfigCommon
```go
type DeviceConfigCommon struct {
	Name           string              `yaml:"name"`
	Type           string              `yaml:"type"`
	ControllerName string              `yaml:"controller"`
	Operations     map[string][]string `yaml:"operations"`
	Conditions     map[string][]string `yaml:"conditions"`
	RetryConfig    `yaml:",inline"`
}
```
DeviceConfigCommon represents the common configuration for a device.


### Type Location
```go
type Location struct {
	datetime.Place
	ZIPCode string
}
```


### Type LocationConfig
```go
type LocationConfig struct {
	TimeLocation *TimeLocation `yaml:"time_location" cmd:"the system location for time in time.Location format"`
	ZIPCode      string        `yaml:"zip_code" cmd:"the zip/postal for the system used to determine it's latitude and longitude, but not used for time"`
	Latitude     float64       `yaml:"latitude" cmd:"the latitude for the location"`
	Longitude    float64       `yaml:"longitude" cmd:"the longitude for the location"`
}
```


### Type Operation
```go
type Operation func(ctx context.Context, opts OperationArgs) error
```
Operation represents a single operation that can be performed on a device.


### Type OperationArgs
```go
type OperationArgs struct {
	Due    time.Time
	Place  datetime.Place
	Writer io.Writer
	Logger *slog.Logger
	Args   []string
}
```


### Type Option
```go
type Option func(*Options)
```

### Functions

```go
func WithControllers(c SupportedControllers) Option
```


```go
func WithCustom(c any) Option
```


```go
func WithDevices(d SupportedDevices) Option
```


```go
func WithLatLong(lat, long float64) Option
```


```go
func WithLogger(l *slog.Logger) Option
```


```go
func WithSession(c streamconn.Session) Option
```


```go
func WithTimeLocation(loc *time.Location) Option
```


```go
func WithZIPCode(zip string) Option
```


```go
func WithZIPCodeLookup(l ZIPCodeLookup) Option
```




### Type Options
```go
type Options struct {
	Logger      *slog.Logger
	Interactive io.Writer
	Session     streamconn.Session
	Devices     SupportedDevices
	Controllers SupportedControllers

	Custom any
	// contains filtered or unexported fields
}
```


### Type RetryConfig
```go
type RetryConfig struct {
	Timeout time.Duration `yaml:"timeout"` // the initial time to wait for a successful operation
	Retries int           `yaml:"retries"` // the number of exponential backoff steps to take before giving up, zero means try once, one means retry once, etc.
}
```
RetryConfig represents the configuration for retrying an operation. Timeout
is the initial time to wait for a successful operation and Retries is the
number of exponential backoff steps to take before giving up, zero means no
retries, one means retry once, etc.


### Type SupportedControllers
```go
type SupportedControllers map[string]func(typ string, opts Options) (Controller, error)
```


### Type SupportedDevices
```go
type SupportedDevices map[string]func(typ string, opts Options) (Device, error)
```


### Type System
```go
type System struct {
	Config      SystemConfig
	Location    Location
	Controllers map[string]Controller
	Devices     map[string]Device
}
```

### Functions

```go
func ParseSystemConfig(ctx context.Context, cfgData []byte, opts ...Option) (System, error)
```
ParseSystemConfig parses the supplied configuration data and returns a
System using CreateSystem.


```go
func ParseSystemConfigFile(ctx context.Context, cfgFile string, opts ...Option) (System, error)
```
ParseSystemConfigFile parses the supplied configuration file as per
ParseSystemConfig.



### Methods

```go
func (s System) ControllerConfigs(name string) (ControllerConfig, Controller, bool)
```


```go
func (s System) ControllerOp(name, op string) (Operation, []string, bool)
```
ControllerOp returns the operation function (and any configured parameters)
for the specified operation on the named controller. The operation must be
'configured', ie. listed in the operations: list for the controller to be
returned.


```go
func (s System) DeviceCondition(name, op string) (Condition, []string, bool)
```
DeviceCondition returns the condition function (and any configured
parameters) for the specified operation on the named controller. The
condition must be 'configured', ie. listed in the conditions: list for the
device to be returned.


```go
func (s System) DeviceConfigs(name string) (DeviceConfig, Device, bool)
```


```go
func (s System) DeviceOp(name, op string) (Operation, []string, bool)
```
DeviceOp returns the operation function (and any configured parameters)
for the specified operation on the named device. The operation must be
'configured', ie. listed in the operations: list for the device to be
returned.




### Type SystemConfig
```go
type SystemConfig struct {
	Location    LocationConfig     `yaml:",inline"`
	Controllers []ControllerConfig `yaml:"controllers" cmd:"the controllers that are being configured"`
	Devices     []DeviceConfig     `yaml:"devices" cmd:"the devices that are being configured"`
}
```

### Methods

```go
func (cfg SystemConfig) CreateSystem(ctx context.Context, opts ...Option) (System, error)
```
CreateSystem creates a system from the supplied configuration. The place
argument is used to set the location of the system if the location is not
specified in the configuration. Note that if the time_zone: tag is specified
in the configuration without a value then the location is set to the current
time.Location, ie. timezone of 'Local' The WithTimeLocation, WithLatLong and
WithZIPCode options can be used to override the location specified in the
configuration. The WithZIPCodeLookup option must be supplied to enable the
lookup of lat/long from a zip code.




### Type TimeLocation
```go
type TimeLocation struct {
	*time.Location
}
```

### Methods

```go
func (tz *TimeLocation) UnmarshalYAML(node *yaml.Node) error
```




### Type ZIPCodeLookup
```go
type ZIPCodeLookup interface {
	Lookup(zip string) (float64, float64, error)
}
```





