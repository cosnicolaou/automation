# Package [github.com/cosnicolaou/automation/scheduler](https://pkg.go.dev/github.com/cosnicolaou/automation/scheduler?tab=doc)

```go
import github.com/cosnicolaou/automation/scheduler
```


## Variables
### AnnualDynamic, DailyDynamic
```go
AnnualDynamic = map[string]datetime.DynamicDateRange{
	"summer":          astronomy.Summer{},
	"winter":          astronomy.Winter{},
	"spring":          astronomy.Spring{},
	"fall":            astronomy.Autumn{LocalName: "Fall"},
	"autumn":          astronomy.Autumn{},
	"winter-solstice": astronomy.WinterSolstice{},
	"summer-solstice": astronomy.SummerSolstice{},
	"spring-equinox":  astronomy.SpringEquinox{},
	"fall-equinox":    astronomy.AutumnEquinox{},
	"autumn-equinox":  astronomy.AutumnEquinox{},
}
DailyDynamic = map[string]datetime.DynamicTimeOfDay{
	"sunrise":   astronomy.SunRise{},
	"sunset":    astronomy.SunSet{},
	"solarnoon": astronomy.SolarNoon{},
}

```

### ErrOpTimeout
```go
ErrOpTimeout = errors.New("op-timeout")

```



## Functions
### Func ParseActionTime
```go
func ParseActionTime(v string) (datetime.TimeOfDay, datetime.DynamicTimeOfDay, time.Duration, error)
```
ParseAction parses a time of day that may contain a dynamic time of day
function with a +- delta. Valid dynamic time of day functions are defined by
DailyDynamic.

### Func ParseDateRangesDynamic
```go
func ParseDateRangesDynamic(vals []string) (datetime.DateRangeList, datetime.DynamicDateRangeList, error)
```
ParseDateRangesDynamic parses a list of date ranges that may contain dynamic
date ranges. Valid dynamic date ranges are definmed by AnnualDynamic.

### Func RunSchedulers
```go
func RunSchedulers(ctx context.Context, schedules Schedules, system devices.System, start datetime.CalendarDate, opts ...Option) error
```
RunSchedulers runs the supplied schedules against the supplied system
starting at the specified date until the context is canceled. Note that the
WithTimeSource option should not be used with this function as it will be
used by all of the schedulers created which is likely not what is intended.
Note that the Simulate function can be used to run multiple schedules using
simulated time appropriate for each schedule.

### Func RunSimulation
```go
func RunSimulation(ctx context.Context, schedules Schedules, system devices.System, period datetime.CalendarDateRange, opts ...Option) error
```
RunSimulation runs the specified schedules against the specified system for
the specified period using a similated time.



## Types
### Type Action
```go
type Action struct {
	devices.Action
	Precondition Precondition
}
```
Action represents a single action to be taken on any given day.


### Type ActionTime
```go
type ActionTime struct {
	Literal datetime.TimeOfDay
	Dynamic datetime.DynamicTimeOfDay
	Delta   time.Duration
}
```
ActionTime represents a time of day that may be a literal or a dynamic
value.


### Type ActionTimeList
```go
type ActionTimeList []ActionTime
```

### Methods

```go
func (atl *ActionTimeList) Parse(val string) error
```




### Type Annual
```go
type Annual struct {
	Name         string
	Dates        schedule.Dates
	DailyActions schedule.ActionSpecs[Action]
}
```


### Type Calendar
```go
type Calendar struct {
	// contains filtered or unexported fields
}
```

### Functions

```go
func NewCalendar(schedules Schedules, system devices.System, opts ...Option) (*Calendar, error)
```



### Methods

```go
func (c *Calendar) Scheduled(date datetime.CalendarDate) []CalendarEntry
```




### Type CalendarEntry
```go
type CalendarEntry struct {
	Schedule string
	schedule.Active[Action]
}
```


### Type Option
```go
type Option func(o *options)
```

### Functions

```go
func WithDryRun(v bool) Option
```


```go
func WithLogger(l *slog.Logger) Option
```
WithLogger sets the logger to be used by the scheduler and is also passed to
all device operations/conditions.


```go
func WithOperationWriter(w io.Writer) Option
```
WithOperationWriter sets the output writer that operations can use for
interactive output.


```go
func WithSimulationDelay(d time.Duration) Option
```


```go
func WithStatusRecorder(sr *logging.StatusRecorder) Option
```


```go
func WithTimeSource(ts TimeSource) Option
```
WithTimeSource sets the time source to be used by the scheduler and is
primarily intended for testing purposes.




### Type Precondition
```go
type Precondition struct {
	Device    string
	Name      string
	Condition devices.Condition
	Args      []string
}
```


### Type Scheduler
```go
type Scheduler struct {
	// contains filtered or unexported fields
}
```

### Functions

```go
func New(sched Annual, system devices.System, opts ...Option) (*Scheduler, error)
```
New creates a new scheduler for the supplied schedule and associated
devices.



### Methods

```go
func (s *Scheduler) Place() datetime.Place
```


```go
func (s *Scheduler) RunDay(ctx context.Context, place datetime.Place, active schedule.Scheduled[Action]) error
```


```go
func (s *Scheduler) RunYear(ctx context.Context, cd datetime.CalendarDate) error
```
Run runs the scheduler from the specified calendar date to the last of the
scheduled actions for that year.


```go
func (s *Scheduler) RunYearEnd(ctx context.Context, cd datetime.CalendarDate) error
```
RunYear runs the scheduler from the specified calendar date to the end of
that year.


```go
func (s *Scheduler) ScheduledYearEnd(cd datetime.CalendarDate) iter.Seq[schedule.Scheduled[Action]]
```




### Type Schedules
```go
type Schedules struct {
	System    devices.System
	Schedules []Annual
}
```

### Functions

```go
func ParseConfig(_ context.Context, cfgData []byte, system devices.System) (Schedules, error)
```


```go
func ParseConfigFile(ctx context.Context, cfgFile string, system devices.System) (Schedules, error)
```



### Methods

```go
func (s Schedules) Lookup(name string) Annual
```




### Type SystemTimeSource
```go
type SystemTimeSource struct{}
```

### Methods

```go
func (SystemTimeSource) NowIn(loc *time.Location) time.Time
```




### Type TimeSource
```go
type TimeSource interface {
	NowIn(in *time.Location) time.Time
}
```
TimeSource is an interface that provides the current time in a specific
location and is intended for testing purposes. It will be called once per
iteration of the scheduler to schedule the next action. time.Now().In() will
be used for all other time operations.





### TODO
- cnicolaou: implement retries.




