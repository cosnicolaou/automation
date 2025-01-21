# Package [github.com/cosnicolaou/automation/net/netutil](https://pkg.go.dev/github.com/cosnicolaou/automation/net/netutil?tab=doc)

```go
import github.com/cosnicolaou/automation/net/netutil
```


## Types
### Type IdleManager
```go
type IdleManager[T any, F Managed[T]] struct {
	// contains filtered or unexported fields
}
```
IdleManagerManager manages an instance of Managed using the supplied idle
timer. Connect is called whenever a new managed instance is required and
Disconnect when the idle time is reached.

### Functions

```go
func NewIdleManager[T any, F Managed[T]](managed F, idle *IdleTimer) *IdleManager[T, F]
```



### Methods

```go
func (m *IdleManager[T, F]) Connection(ctx context.Context) (T, error)
```
Connection returns the current connection, or creates a new one if the idle
timer has expired.


```go
func (m *IdleManager[T, F]) Stop(ctx context.Context, timeout time.Duration) error
```
Stop closes the connection and stops the idle timer.




### Type IdleReset
```go
type IdleReset interface {
	Reset()
}
```


### Type IdleTimer
```go
type IdleTimer struct {
	// contains filtered or unexported fields
}
```
IdleTimer is a timer that expires after a period of inactivity.

### Functions

```go
func NewIdleTimer(d time.Duration) *IdleTimer
```
NewIdleTimer creates a new IdleTimer with the specified idle time,
call Reset to restart the timer. The timer can reused by calling Wait again,
typically in a goroutine. A negative duration will cause a panic.



### Methods

```go
func (d *IdleTimer) Reset()
```
Reset resets the idle timer.


```go
func (d *IdleTimer) StopWait(ctx context.Context) error
```
StopWait stops the idle timer watcher and waits for it to do so, or for the
context to be canceled.


```go
func (d *IdleTimer) Wait(ctx context.Context, expired func(context.Context))
```
Wait waits for the idle to expire, for the channel to be closed or the
context to be canceled. The close function is called when the idle timer
expires or the context canceled, but not when the channel is closed.




### Type Managed
```go
type Managed[T any] interface {
	// Connect is called when a new connection is required.
	Connect(context.Context, IdleReset) (T, error)

	// Disconnect is called when the idle timer has expired.
	Disconnect(context.Context, T) error
}
```
Managed is the interface used by Manager[T] to manage a connection.


### Type OnDemandConnection
```go
type OnDemandConnection[T any, F Managed[T]] struct {
	// contains filtered or unexported fields
}
```
OnDemandConnection wraps an IdleManager to reuse or recreate a connection as
required.

### Functions

```go
func NewOnDemandConnection[T any, F Managed[T]](managed F, newErrorSession func(error) T) *OnDemandConnection[T, F]
```



### Methods

```go
func (sm *OnDemandConnection[T, F]) Close(ctx context.Context) error
```


```go
func (sm *OnDemandConnection[T, F]) Connection(ctx context.Context) T
```


```go
func (sm *OnDemandConnection[T, F]) SetKeepAlive(keepAlive time.Duration)
```







