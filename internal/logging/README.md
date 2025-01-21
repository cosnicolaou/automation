# Package [github.com/cosnicolaou/automation/internal/logging](https://pkg.go.dev/github.com/cosnicolaou/automation/internal/logging?tab=doc)

```go
import github.com/cosnicolaou/automation/internal/logging
```


## Constants
### TimeWithTZ, TimeWithTZNano
```go
TimeWithTZ = "2006-01-02T15:04:05 MST"
TimeWithTZNano = "2006-01-02T15:04:05.999999999 MST"

```

### LogPending, LogCompleted, LogFailed, LogTooLate, LogYearEnd, LogNewDay
```go
LogPending = "pending"
LogCompleted = "completed"
LogFailed = "failed"
LogTooLate = "too-late"
LogYearEnd = "year-end"
LogNewDay = "day"

```



## Functions
### Func WriteCompletion
```go
func WriteCompletion(l *slog.Logger, id int64, err error,
	dryRun bool, device, op, precondition string, preconditionResult bool, started, now, dueAt time.Time, delay time.Duration)
```
WriteCompletion logs the completion of all executed operations and must
be called for every operation non-overdue that was logged as pending.
The id must be the value returned by LogPending.

### Func WriteNewDay
```go
func WriteNewDay(l *slog.Logger, date datetime.CalendarDate, nActions int)
```

### Func WritePending
```go
func WritePending(l *slog.Logger, overdue, dryRun bool, device, op string, args []string, precondition string, preArgs []string, now, dueAt time.Time, delay time.Duration) int64
```
WritePending logs a pending operation and must be called for every new
action returned by the scheduler for any given day. It returns a unique
identifier for the operation that must be passed to LogCompletion except for
overdue operations which are not logged as being completed.

### Func WriteYearEnd
```go
func WriteYearEnd(l *slog.Logger, year int, delay time.Duration)
```
WriteYearEndLog logs the completion of the year-end processing, that is,
when all scheduled events for the year have been executed and the scheduler
simply has to wait for the next year to start.



## Types
### Type Date
```go
type Date datetime.CalendarDate
```

### Methods

```go
func (ld Date) MarshalJSON() ([]byte, error)
```


```go
func (ld *Date) UnmarshalJSON(data []byte) error
```




### Type Duration
```go
type Duration time.Duration
```

### Methods

```go
func (ld Duration) MarshalJSON() ([]byte, error)
```


```go
func (ld *Duration) UnmarshalJSON(data []byte) error
```




### Type Entry
```go
type Entry struct {
	Date         datetime.CalendarDate
	Now          time.Time
	Due          time.Time
	Started      time.Time
	Delay        time.Duration
	YearEndDelay time.Duration
	Err          error
	LogEntry     string // Original log line
	// contains filtered or unexported fields
}
```

### Functions

```go
func ParseLogLine(line string) (Entry, error)
```



### Methods

```go
func (le Entry) Aborted() bool
```


```go
func (le Entry) Name() string
```


```go
func (le Entry) StatusRecord() *StatusRecord
```




### Type Scanner
```go
type Scanner struct {
	// contains filtered or unexported fields
}
```

### Functions

```go
func NewScanner(rd io.Reader) *Scanner
```



### Methods

```go
func (ls *Scanner) Entries() iter.Seq[Entry]
```
Entries returns an iterator for over the LogScanner's LogEntry's. Note that
the iterator will stop if an error is encountered and that the Scanner's Err
method should be checked after the iterator has completed.


```go
func (ls *Scanner) Err() error
```




### Type StatusRecord
```go
type StatusRecord struct {
	Schedule         string
	Device           string
	ID               int64 // Unique identifier for this invocation
	Op               string
	OpArgs           []string
	Due              time.Time
	Delay            time.Duration
	PreCondition     string // Name of the precondition, if any
	PreConditionArgs []string

	// The following fields are filled in by the status recorder.
	Pending            time.Time // Time the operation was added to the pending list, set by NewPending
	Completed          time.Time // Time the operation was completed set by Finalize
	PreConditionResult bool      // Set using the argument to Finalize
	Error              error     // Set using the argument to Finalize
	// contains filtered or unexported fields
}
```

### Methods

```go
func (sr *StatusRecord) Aborted() bool
```


```go
func (sr *StatusRecord) Name() string
```




### Type StatusRecorder
```go
type StatusRecorder struct {
	// contains filtered or unexported fields
}
```

### Functions

```go
func NewStatusRecorder() *StatusRecorder
```



### Methods

```go
func (s *StatusRecorder) Completed() iter.Seq[*StatusRecord]
```


```go
func (s *StatusRecorder) NewPending(sr *StatusRecord) *StatusRecord
```


```go
func (s *StatusRecorder) Pending() iter.Seq[*StatusRecord]
```


```go
func (s *StatusRecorder) PendingDone(sr *StatusRecord, precondition bool, err error)
```


```go
func (s *StatusRecorder) ResetCompleted()
```




### Type Time
```go
type Time time.Time
```

### Methods

```go
func (lt Time) MarshalJSON() ([]byte, error)
```


```go
func (lt *Time) UnmarshalJSON(data []byte) error
```




### Type TimeNano
```go
type TimeNano time.Time
```

### Methods

```go
func (lt TimeNano) MarshalJSON() ([]byte, error)
```


```go
func (lt *TimeNano) UnmarshalJSON(data []byte) error
```







