# Package [github.com/cosnicolaou/automation/net/streamconn](https://pkg.go.dev/github.com/cosnicolaou/automation/net/streamconn?tab=doc)

```go
import github.com/cosnicolaou/automation/net/streamconn
```


## Types
### Type Session
```go
type Session interface {
	Send(ctx context.Context, buf []byte)
	// SendSensitive avoids logging the contents of the buffer, use
	// it for login exchanges, credentials etc.
	SendSensitive(ctx context.Context, buf []byte)
	ReadUntil(ctx context.Context, expected ...string) []byte
	Close(ctx context.Context) error
	Err() error
}
```

### Functions

```go
func NewErrorSession(err error) Session
```
NewErrorSession returns a session that always returns the given error.


```go
func NewSession(t Transport, idle netutil.IdleReset) Session
```




### Type Transport
```go
type Transport interface {
	Send(ctx context.Context, buf []byte) (int, error)
	// SendSensitive avoids logging the contents of the buffer, use
	// it for login exchanges, credentials etc.
	SendSensitive(ctx context.Context, buf []byte) (int, error)
	ReadUntil(ctx context.Context, expected []string) ([]byte, error)
	Close(ctx context.Context) error
}
```





