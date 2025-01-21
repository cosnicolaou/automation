# Package [github.com/cosnicolaou/automation/internal/testutil](https://pkg.go.dev/github.com/cosnicolaou/automation/internal/testutil?tab=doc)

```go
import github.com/cosnicolaou/automation/internal/testutil
```


## Types
### Type ControllerDetail
```go
type ControllerDetail struct {
	Detail string `yaml:"detail"`
	KeyID  string `yaml:"key_id"`
}
```


### Type DeviceDetail
```go
type DeviceDetail struct {
	Detail string `yaml:"detail"`
}
```


### Type MockController
```go
type MockController struct {
	devices.ControllerBase[ControllerDetail]
}
```

### Methods

```go
func (c *MockController) Disable(_ context.Context, opts devices.OperationArgs) error
```


```go
func (c *MockController) Enable(_ context.Context, opts devices.OperationArgs) error
```


```go
func (c *MockController) Implementation() any
```


```go
func (c *MockController) Operations() map[string]devices.Operation
```


```go
func (c *MockController) OperationsHelp() map[string]string
```




### Type MockDevice
```go
type MockDevice struct {
	devices.DeviceBase[DeviceDetail]
	// contains filtered or unexported fields
}
```

### Functions

```go
func NewMockDevice(operations ...string) *MockDevice
```



### Methods

```go
func (d *MockDevice) AddCondition(name string, outcome bool)
```


```go
func (d *MockDevice) Conditions() map[string]devices.Condition
```


```go
func (d *MockDevice) ConditionsHelp() map[string]string
```


```go
func (d *MockDevice) ControlledBy() devices.Controller
```


```go
func (d *MockDevice) Implementation() any
```


```go
func (d *MockDevice) Operations() map[string]devices.Operation
```


```go
func (d *MockDevice) OperationsHelp() map[string]string
```


```go
func (d *MockDevice) SetController(c devices.Controller)
```


```go
func (d *MockDevice) SetOutput(logger bool, writer bool)
```







