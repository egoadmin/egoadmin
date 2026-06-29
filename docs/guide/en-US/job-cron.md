# Job & Cron Scheduled Tasks

This page covers the EGO framework's Job (one-time) and Cron (scheduled) task mechanisms, along with their real-world usage in EgoAdmin.

## Overview

The EGO framework provides two task execution mechanisms: **Job** (one-time tasks) and **Cron** (periodic scheduled tasks). Both are deeply integrated with the EGO lifecycle, supporting graceful shutdown, distributed tracing, and structured logging.

| Mechanism | Trigger | Execution | Typical Use |
|-----------|---------|-----------|-------------|
| Job | CLI `--job` flag or HTTP request | Runs once, then process exits | Data migration, batch import, one-time fixes |
| Cron | Cron expression scheduling | Periodic | Heartbeat offline check, log cleanup, lease renewal |

EgoAdmin uses Cron for:

- **User heartbeat offline check**: periodically scan online users who exceeded the heartbeat timeout and mark them offline
- **Upload file cleanup**: clean up expired temporary upload files
- **IDGen machine lease renewal**: periodically renew ID generator machine numbers
- **IDGen expired machine cleanup**: clean up expired machine lease records
- **Audit log cleanup**: clean up audit logs older than two years

## Job (One-Time Tasks)

Job is a one-time task execution unit. EGO supports three Job usage patterns: CLI-triggered, HTTP-triggered, and Cobra-integrated.

### Pattern 1: CLI-Triggered Job

The simplest Job pattern. Use the `--job` CLI flag to specify which task to execute. The process starts, runs the specified Job, and exits.

```go
package main

import (
	"errors"
	"fmt"

	"github.com/gotomicro/ego"
	"github.com/gotomicro/ego/core/elog"
	"github.com/gotomicro/ego/core/etrace"
	"github.com/gotomicro/ego/task/ejob"
)

func main() {
	if err := ego.New().Job(
		ejob.Job("job1", job1),
		ejob.Job("job2", job2),
	).Run(); err != nil {
		elog.Error("start up", elog.Any("err", err))
	}
}

func job1(ctx ejob.Context) error {
	fmt.Println("i am job runner, traceId:", etrace.ExtractTraceID(ctx.Ctx))
	return nil
}

func job2(ctx ejob.Context) error {
	fmt.Println("i am error job runner")
	return errors.New("i am error")
}
```

**Running:**

```bash
# Execute a single Job
go run main.go --job=job1

# Execute multiple Jobs (comma-separated)
go run main.go --job=job1,job2
```

::: tip Behavior after Job execution
Once the Job completes, EGO automatically exits the process. If servers (HTTP/gRPC) are also registered, they will not start during Job execution. The `--job` flag is used for selective execution; unmatched Jobs will not run.
:::

### Pattern 2: HTTP-Triggered Job

Trigger Job execution via the Governor HTTP endpoint. This is useful when external systems or operations scripts need to trigger jobs.

```go
package main

import (
	"io"

	"github.com/gotomicro/ego"
	"github.com/gotomicro/ego/core/elog"
	"github.com/gotomicro/ego/server/egovernor"
	"github.com/gotomicro/ego/task/ejob"
)

func main() {
	if err := ego.New().Job(
		ejob.Job("job", job),
	).Serve(
		egovernor.Load("server.governor").Build(),
	).Run(); err != nil {
		elog.Error("start up", elog.Any("err", err))
	}
}

func job(ctx ejob.Context) error {
	bytes, _ := io.ReadAll(ctx.Request.Body)
	// Process request body data
	ctx.Writer.Write([]byte("i am ok"))
	return nil
}
```

**HTTP trigger:**

```bash
curl -XPOST -d '{"username":"ego"}' \
  -H 'X-Ego-Job-Name:job' \
  -H 'X-Ego-Job-RunID:unique-run-id' \
  http://127.0.0.1:9003/jobs
```

**Request headers:**

| Header | Required | Description |
|--------|----------|-------------|
| `X-Ego-Job-Name` | Yes | Job name, must match the registered name |
| `X-Ego-Job-RunID` | Yes | Unique execution ID for deduplication and tracing |
| Request Body | No | Business data, read by the Job function via `ctx.Request` |

**Governor endpoint:**

```bash
# List registered Jobs
curl http://127.0.0.1:9003/job/list
```

Returns a list of registered Job names for operations monitoring.

::: warning HTTP Job concurrency
Each HTTP-triggered Job runs in an independent goroutine. If the Job takes a long time, watch out for concurrency control. The `X-Ego-Job-RunID` header is used to prevent duplicate submissions.
:::

### Pattern 3: Cobra-Integrated Job

Integrate Jobs as Cobra subcommands in your CLI tool, useful when complex argument parsing is needed.

```go
package main

import (
	"os"

	"github.com/gotomicro/ego"
	"github.com/gotomicro/ego/core/elog"
	"github.com/gotomicro/ego/task/ejob"
	"github.com/spf13/cobra"
)

var jobName string

func init() {
	CmdRun.PersistentFlags().StringVar(&jobName, "job", "", "job name")
	rootcmd.RootCommand.AddCommand(CmdRun)
}

var CmdRun = &cobra.Command{
	Use:   "run",
	Short: "Run a job",
	RunE: func(cmd *cobra.Command, args []string) error {
		return ego.New(ego.WithArguments(os.Args[2:])).Job(
			ejob.Job("doSomething", doSomething),
		).Run()
	},
}

func doSomething(ctx ejob.Context) error {
	// Business logic
	return nil
}
```

Running:

```bash
go run main.go run --job=doSomething
```

::: tip Difference from CLI pattern
Cobra integration allows custom command structure and argument parsing. `ego.WithArguments(os.Args[2:])` ensures EGO can correctly parse the `--job` flag.
:::

### Job Function Signature

All Job functions share the signature `func(ejob.Context) error`:

```go
type Context struct {
    Ctx     context.Context      // Request context with traceId
    Request *http.Request        // HTTP request (only valid for HTTP-triggered)
    Writer  http.ResponseWriter  // HTTP response (only valid for HTTP-triggered)
}
```

- `ctx.Ctx` is always available and contains distributed tracing information
- `ctx.Request` and `ctx.Writer` are only valid in HTTP-triggered mode

## Cron (Scheduled Tasks)

Cron is a periodically executing scheduled task. EGO provides complete cron scheduling through the `ecron` package, supporting second-level precision, distributed locking, and delay execution strategies.

### Configuration

Cron tasks are defined in TOML configuration files. The section name serves as the task identifier:

```toml
[cron.test]
enableDistributedTask = false    # Enable distributed task (requires Redis lock)
enableImmediatelyRun = false     # Run immediately on startup
enableSeconds = false            # Enable second-level parser
spec = "*/5 * * * * *"           # Cron expression (every 5 seconds if enableSeconds=true)
delayExecType = "skip"           # Delay strategy: skip | queue | concurrent
```

### Configuration Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `spec` | string | Required | Cron expression |
| `enableDistributedTask` | bool | `false` | Enable distributed lock to ensure only one node executes in a cluster |
| `enableImmediatelyRun` | bool | `false` | Execute once immediately after process startup without waiting for the first scheduled time |
| `enableSeconds` | bool | `false` | Enable second-level cron parser. When `false`, standard 5-field expression (min hour dom mon dow); when `true`, 6-field expression (sec min hour dom mon dow) |
| `delayExecType` | string | `"skip"` | Handling strategy when the previous execution has not completed |
| `waitLockTime` | duration | `4s` | Wait time for acquiring the distributed lock |
| `lockTTL` | duration | `16s` | Distributed lock lease duration |
| `refreshGap` | duration | `4s` | Distributed lock refresh interval |

### Delay Execution Strategies

When the previous task execution has not completed and the next scheduled time arrives, `delayExecType` determines the behavior:

| Strategy | Behavior | Use Case |
|----------|----------|----------|
| `skip` | Skip this execution | Most scenarios; avoids duplicate execution |
| `queue` | Queue and wait, execute immediately after the previous one completes | Tasks that must not be lost, but execution frequency is not critical |
| `concurrent` | Execute concurrently | Tasks that are idempotent with no shared state |

::: tip EgoAdmin default strategy
EgoAdmin's cron tasks uniformly use `delayExecType = "skip"` to avoid data consistency issues from duplicate execution.
:::

### Basic Usage

The simplest way to register a Cron task, using `ecron.Load` to load configuration and bind the execution function:

```go
package main

import (
	"context"
	"errors"

	"github.com/gotomicro/ego"
	"github.com/gotomicro/ego/core/elog"
	"github.com/gotomicro/ego/core/etrace"
	"github.com/gotomicro/ego/task/ecron"
)

func main() {
	err := ego.New().Cron(cronJob1(), cronJob2()).Run()
	if err != nil {
		elog.Panic("startup", elog.FieldErr(err))
	}
}

func cronJob1() ecron.Ecron {
	job := func(ctx context.Context) error {
		elog.Info("info job1", elog.FieldTid(etrace.ExtractTraceID(ctx)))
		return errors.New("exec job1 error")
	}
	return ecron.Load("cron.test").Build(ecron.WithJob(job))
}

func cronJob2() ecron.Ecron {
	job := func(ctx context.Context) error {
		elog.Info("info job2", elog.FieldTid(etrace.ExtractTraceID(ctx)))
		return nil
	}
	return ecron.Load("cron.test").Build(ecron.WithJob(job))
}
```

Corresponding configuration:

```toml
[cron.test]
enableDistributedTask = false
enableImmediatelyRun = false
enableSeconds = false
spec = "*/5 * * * *"
delayExecType = "skip"
```

::: warning Config section name matching
The argument to `ecron.Load("cron.test")` must exactly match the TOML configuration section name. The section name format is `cron.<task_name>`, and the EGO framework uses this name to look up the corresponding `spec` and other settings.
:::

### Distributed Cron

In multi-instance deployments, to prevent the same task from being executed repeatedly by multiple nodes, you can enable distributed locking. EGO uses the `ecronlock` component to implement distributed locking via Redis.

**Configuration:**

```toml
[cron.user.login.offline]
enableDistributedTask = true
enableImmediatelyRun = true
enableSeconds = true
spec = "0 0/1 * * * ?"
delayExecType = "skip"

[client.redis]
addr = "127.0.0.1:6380"
password = "egoadmin"
```

**Code implementation:**

```go
package main

import (
	"context"

	"github.com/gotomicro/ego"
	"github.com/gotomicro/ego-component/eredis"
	"github.com/gotomicro/ego-component/eredis/ecronlock"
	"github.com/gotomicro/ego/core/elog"
	"github.com/gotomicro/ego/task/ecron"
)

var (
	redis  *eredis.Component
	locker *ecronlock.Component
)

func main() {
	err := ego.New().Invoker(initRedis).Cron(cronJob()).Run()
	if err != nil {
		elog.Panic("startup", elog.FieldErr(err))
	}
}

func initRedis() error {
	redis = eredis.Load("client.redis").Build()
	locker = ecronlock.DefaultContainer().Build(ecronlock.WithClient(redis))
	return nil
}

func cronJob() ecron.Ecron {
	return ecron.Load("cron.user.login.offline").Build(
		ecron.WithLock(locker.NewLock("egoadmin:cron:user:offline")),
		ecron.WithJob(func(ctx context.Context) error {
			// Business logic for offline expired users
			return nil
		}),
	)
}
```

**How distributed locking works:**

1. When the scheduled time arrives, the node attempts to acquire the Redis lock
2. The lock uses the `SET NX` atomic operation; only the node that acquires the lock executes the task
3. During execution, the lock's TTL is periodically refreshed (lease renewal)
4. Once the task completes, the lock is released
5. If a node crashes, the lock automatically expires after the TTL

::: warning Lock timeout configuration
`lockTTL` should be greater than the maximum task execution time. Otherwise the lock may expire mid-execution, causing another node to acquire it and execute the task again. Set `lockTTL` to 2-3x the maximum task execution time.
:::

### Cron Function Signature

Cron task functions have the signature `func(context.Context) error`:

- `context.Context` contains distributed tracing information; use `etrace.ExtractTraceID(ctx)` to get the traceId
- Returning `nil` indicates success; returning an `error` logs an ERROR but does not affect the next scheduled execution

## EgoAdmin Real-World Examples

### User Heartbeat Offline Check

The user service uses a cron task to periodically scan online users who exceeded the heartbeat timeout and marks them offline. This is one of the most critical scheduled tasks in EgoAdmin.

**Configuration (`configs/user/config.toml`):**

```toml
[cron.user.login.offline]
delayExecType = "skip"
enableDistributedTask = false
enableImmediatelyRun = true
enableSeconds = true
spec = "0 0/1 * * * ?"
```

**Configuration meaning:**
- Executes once every minute (`0/1` means at second 0 of every minute)
- `enableSeconds = true`: uses 6-field cron expression with seconds as the first field
- `enableImmediatelyRun = true`: executes once immediately after process startup
- `delayExecType = "skip"`: if the previous minute's task is still running, skip this one
- `enableDistributedTask = false`: single-instance mode, no distributed lock needed

**Task registration (`internal/app/user/internal/job/user.go`):**

```go
package job

import (
	"context"

	"github.com/gotomicro/ego/task/ecron"
)

func userCrons(opts *Options) (ecs []ecron.Ecron) {
	ecs = append(ecs, userOffline(opts))
	return
}

func userOffline(opts *Options) ecron.Ecron {
	return ecron.Load("cron.user.login.offline").Build(
		ecron.WithJob(func(ctx context.Context) error {
			return opts.User.OfflineUser(ctx)
		}),
	)
}
```

**Task aggregation (`internal/app/user/internal/job/cron.go`):**

```go
package job

import (
	"github.com/google/wire"
	"github.com/gotomicro/ego/task/ecron"
)

var ProviderSet = wire.NewSet(
	wire.Struct(new(Options), "*"),
	New,
)

type Options struct {
	User UserOfflineService
}

type UserOfflineService interface {
	OfflineUser(context.Context) error
}

type Cron struct {
	tss []ecron.Ecron
}

func New(opts *Options) *Cron {
	cr := &Cron{
		tss: []ecron.Ecron{},
	}
	cr.tss = append(cr.tss, userCrons(opts)...)
	return cr
}

func (c *Cron) Tasks() []ecron.Ecron {
	return c.tss
}
```

**Registering at service startup (`internal/app/user/server/server.go`):**

```go
func newApp(opts Options, _ schemaReady) (*App, error) {
	// ... start critical components ...

	configureShutdown(opts)

	opts.app.Registry(opts.registry)
	opts.app.Serve(opts.http, opts.grpc, opts.govern)
	opts.app.Cron(opts.cron.Tasks()...)      // Register business cron tasks
	if opts.idm != nil {
		opts.app.Cron(idgen.NewMachineLeaseRenewCron(opts.idm)) // Register machine lease renewal
	}

	// ... other initialization ...
	opts.health.Ready()
	return &App{Ego: opts.app}, nil
}
```

::: tip Wire dependency injection
`job.Cron` uses Wire to automatically inject the `UserOfflineService` interface implementation (i.e., `*service.UserService`). The Wire ProviderSet is defined in `internal/app/user/internal/job/cron.go`, ensuring dependency relationships are checked at compile time.
:::

### Upload File Cleanup

The gateway service uses a cron task to periodically clean up expired temporary upload files, preventing unbounded storage growth.

**Configuration (`configs/gateway/local-live.toml`):**

```toml
[cron.gateway.upload.cleanup]
delayExecType = "skip"
enableDistributedTask = false
enableImmediatelyRun = false
enableSeconds = true
spec = "0 0/10 * * * ?"
```

Executes every 10 minutes, does not run immediately on startup (files to clean up typically do not exist at startup time).

**Task implementation (`internal/app/gateway/internal/job/cron.go`):**

```go
package job

import (
	"context"
	"time"

	"github.com/egoadmin/egoadmin/internal/component/upload"
	"github.com/google/wire"
	"github.com/gotomicro/ego/task/ecron"
)

type Options struct {
	Upload *upload.Component
}

type Cron struct {
	tss []ecron.Ecron
}

func New(opts *Options) *Cron {
	cr := &Cron{
		tss: []ecron.Ecron{
			uploadCleanup(opts),
		},
	}
	return cr
}

func (c *Cron) Tasks() []ecron.Ecron {
	return c.tss
}

func uploadCleanup(opts *Options) ecron.Ecron {
	return ecron.Load("cron.gateway.upload.cleanup").Build(
		ecron.WithJob(func(ctx context.Context) error {
			_, err := opts.Upload.CleanupExpired(ctx, time.Now(), 100)
			return err
		}),
	)
}
```

### IDGen Machine Lease Renewal

The IDGen service assigns machine numbers via database row locking. The user and gateway services need to periodically renew their leases to keep machine numbers valid.

**Task implementation (`internal/component/idgen/machine_cron.go`):**

```go
package idgen

import (
	"context"

	"github.com/gotomicro/ego/task/ecron"
)

const machineRenewCronName = "cron.idgen.machine.renew"

func NewMachineLeaseRenewCron(manager MachineLeaseManager) ecron.Ecron {
	return ecron.Load(machineRenewCronName).Build(
		ecron.WithJob(func(ctx context.Context) error {
			if manager == nil {
				return nil
			}
			return manager.Renew(ctx)
		}),
	)
}
```

**Registration in user and gateway services:**

```go
// internal/app/user/server/server.go
// internal/app/gateway/server/server.go
opts.app.Cron(idgen.NewMachineLeaseRenewCron(opts.idm))
```

::: warning Conditional registration
`NewMachineLeaseRenewCron` has a nil-safety check for `manager == nil`. In deployment topologies that do not use IDGen (`opts.idm == nil`), this cron task does nothing.
:::

### IDGen Expired Machine Cleanup

The IDGen service uses a cron task to clean up expired machine lease records, freeing occupied machine numbers.

**Task implementation (`internal/app/idgen/server/cron.go`):**

```go
package server

import (
	"context"
	"fmt"
	"time"

	"github.com/egoadmin/egoadmin/internal/app/idgen/application"
	"github.com/egoadmin/egoadmin/internal/platform/config"
	"github.com/gotomicro/ego/core/elog"
	"github.com/gotomicro/ego/task/ecron"
)

const (
	machineCleanupCronName         = "cron.idgen.machine.cleanup"
	defaultMachineCleanupLimit     = 1000
	defaultMachineCleanupRetention = 7 * 24 * time.Hour
)

func newMachineLeaseCleanupCron(conf *config.Config, usecase *application.MachineLeaseUseCase) ecron.Ecron {
	return ecron.Load(machineCleanupCronName).Build(
		ecron.WithJob(func(ctx context.Context) error {
			if conf == nil || usecase == nil {
				return nil
			}
			retention, limit, err := machineCleanupOptions(conf)
			if err != nil {
				return err
			}
			deleted, err := usecase.CleanupExpired(ctx, time.Now().Add(-retention), limit)
			if err != nil {
				return err
			}
			if deleted > 0 {
				elog.Info("idgen expired machine leases cleaned",
					elog.FieldComponent("task.ecron"),
					elog.FieldComponentName(machineCleanupCronName),
					elog.FieldCustomKeyValue("deleted", fmt.Sprint(deleted)),
				)
			}
			return nil
		}),
	)
}
```

**Registration in the idgen service:**

```go
// internal/app/idgen/server/server.go
func newApp(opts Options, _ schemaReady) (*App, error) {
	configureShutdown(opts)
	opts.app.Registry(opts.registry)
	opts.app.Serve(opts.http, opts.grpc, opts.govern)
	if opts.cron != nil {
		opts.app.Cron(opts.cron)
	}
	opts.health.Ready()
	return &App{Ego: opts.app}, nil
}
```

`newMachineLeaseCleanupCron` is automatically injected into the `Options.cron` field via the Wire ProviderSet.

### Audit Log Cleanup

The user service provides a `CleanLog` method to clean up audit logs older than two years. While not currently registered as a standalone cron task, the method is ready and can be enabled by adding a cron configuration and task registration:

```go
// internal/app/user/service/user_service.go
func (s *UserService) CleanLog(ctx context.Context) (err error) {
	// Clean up audit logs older than two years
	// ...
}
```

To enable it as a cron task, add the following in `internal/app/user/internal/job/`:

```go
func auditLogCleanup(opts *Options) ecron.Ecron {
	return ecron.Load("cron.user.audit.log.cleanup").Build(
		ecron.WithJob(func(ctx context.Context) error {
			// Call CleanLog to clean up logs
			return nil
		}),
	)
}
```

## How It Works

### Job Execution Flow

```
ego.New().Job(ejob.Job("name", fn)).Run()
    |
    v
EGO parses CLI arguments (--job)
    |
    ├── No --job provided → Normal server startup (ignore Jobs)
    |
    └── --job=name provided
            |
            v
        Match against registered Jobs
            |
            ├── No match → Error and exit
            |
            └── Match found
                    |
                    v
                Create context for each Job (with traceId)
                    |
                    v
                Execute Job functions sequentially
                    |
                    v
                All Jobs complete → process exits
```

### Cron Scheduling Flow

```
ego.New().Cron(ecron.Load("cron.xxx").Build(...)).Run()
    |
    v
EGO creates cron scheduler
    |
    v
Register all Cron tasks, parse spec expressions
    |
    v
Start scheduling loop
    |
    v
Scheduled time arrives
    |
    ├── enableDistributedTask = false
    |       → Execute task function directly
    |
    └── enableDistributedTask = true
            |
            v
        Attempt to acquire Redis distributed lock (SET NX + TTL)
            |
            ├── Acquisition failed → Skip this execution
            |
            └── Acquisition succeeded
                    |
                    v
                Create context (with traceId)
                    |
                    v
                Execute task function
                    |
                    ├── Refresh lock TTL (lease renewal)
                    |
                    v
                Task complete → Release lock
                    |
                    v
                Log execution result
```

### Distributed Lock Mechanism

The `ecronlock` component implements distributed locking via Redis with the following semantics:

1. **Lock**: `SET key value NX PX lockTTL` -- atomic operation ensures only one client can succeed
2. **Renewal**: During execution, the lock TTL is refreshed at `refreshGap` intervals to prevent expiration during long-running tasks
3. **Release**: After task completion, the lock key is deleted using a Lua script to ensure only the lock holder can release it
4. **Recovery**: If a node crashes, the lock expires automatically after `lockTTL`, allowing other nodes to take over

```
Node A acquires lock
    |
    v
Execute task + periodic renewal (every refreshGap seconds)
    |
    ├── Task complete → Delete lock → Nodes B/C can compete at next schedule
    |
    └── Node crashes → Lock expires after lockTTL → Another node takes over
```

### Delay Execution Strategy Comparison

```
Scenario: Previous task execution took longer than the scheduling interval

skip strategy:
    Schedule 1 → [===Executing===]
    Schedule 2 → Skipped (task still running)
    Schedule 3 → [===Executing===]

queue strategy:
    Schedule 1 → [===Executing===]
    Schedule 2 → Queued
                           → Execute (no wait for next schedule)
    Schedule 3 → [===Executing===]

concurrent strategy:
    Schedule 1 → [===Executing===]
    Schedule 2 → [===Executing===]  (parallel)
    Schedule 3 → [===Executing===]  (parallel)
```

### Tracing Integration

Each time a Cron task executes, EGO automatically creates a new trace context. The task function can retrieve the current traceId via `etrace.ExtractTraceID(ctx)` for log correlation:

```go
ecron.WithJob(func(ctx context.Context) error {
    traceId := etrace.ExtractTraceID(ctx)
    elog.Info("cron executing",
        elog.FieldTid(traceId),
        elog.FieldComponentName("cron.user.login.offline"),
    )
    // Business logic
    return nil
})
```

### Graceful Shutdown

Cron tasks integrate with EGO's graceful shutdown mechanism:

1. Process receives `SIGTERM`/`SIGINT`
2. Cron scheduler stops accepting new schedules
3. Waits for currently executing tasks to complete (governed by `stopTimeout`)
4. If distributed locking is enabled, releases held locks
5. Process exits

::: danger Do not interrupt running tasks
Forcefully killing a process (`SIGKILL`) will interrupt any executing cron task. If distributed locking is enabled, the lock will not expire until `lockTTL`, at which point other nodes will take over. Use `SIGTERM` to trigger graceful shutdown.
:::

## Common Issues

### Job not found

**Symptom**: Running `go run main.go --job=myjob` reports "job not found".

**Troubleshooting steps:**

1. Verify the `--job=myjob` name exactly matches the `ejob.Job("myjob", fn)` registration name (case-sensitive)
2. Check for typos
3. Use the Governor endpoint `GET /job/list` to see registered Job names

```bash
# List registered Jobs
curl http://127.0.0.1:9003/job/list
```

### Cron task not executing

**Symptom**: Cron task is configured but does not execute at the scheduled time.

**Troubleshooting steps:**

1. **Check the spec expression**: Confirm the cron expression syntax is correct
   ```bash   # Use online tools to verify cron expressions
   # When enableSeconds=false, use 5 fields: min hour dom mon dow
   # When enableSeconds=true, use 6 fields: sec min hour dom mon dow
   ```
2. **Check enableSeconds**: If the spec includes seconds but `enableSeconds=false`, expression parsing will fail
3. **Check enableImmediatelyRun**: If `false`, the task will not execute immediately after startup; wait for the first scheduled time
4. **Check logs**: EGO prints cron scheduler registration info at startup

### Duplicate execution in multi-instance deployment

**Symptom**: Multiple nodes execute the same cron task simultaneously.

**Solution:**

1. Enable distributed task configuration
2. Configure Redis connection
3. Use `ecron.WithLock` to register a distributed lock in code

```toml
[cron.user.login.offline]
enableDistributedTask = true    # Key: enable distributed lock

[client.redis]
addr = "redis-host:6380"
password = "your-password"
```

```go
ecron.Load("cron.user.login.offline").Build(
    ecron.WithLock(locker.NewLock("egoadmin:cron:user:offline")),
    ecron.WithJob(fn),
)
```

### Task execution timeout

**Symptom**: During a long-running task, the lock expires, causing another node to acquire it and execute the task again.

**Solution:**

1. Increase `lockTTL` (default 16s); set it to 2-3x the maximum task execution time
2. Decrease `refreshGap` (default 4s) to speed up lease renewal
3. Optimize task execution time; consider batch processing

```toml
[cron.my.long.task]
enableDistributedTask = true
lockTTL = "120s"          # Increase lock timeout
refreshGap = "10s"        # Maintain reasonable renewal interval
waitLockTime = "10s"      # Increase lock wait time
spec = "0 0 2 * * ?"
enableSeconds = true
delayExecType = "skip"
```

### How to test Cron tasks?

```go
// Unit test: directly call the task function
func TestUserOffline(t *testing.T) {
    svc := &mockUserOfflineService{}
    opts := &Options{User: svc}
    cron := userOffline(opts)

    // Get the job function and invoke it
    // Verify OfflineUser was called correctly
}

// Integration test: start the service and verify scheduling
// 1. Configure enableImmediatelyRun = true
// 2. Start the service and observe logs
// 3. Verify task execution results
```

```bash
# Run related unit tests
go test ./internal/app/user/internal/job/...
go test ./internal/app/gateway/internal/job/...
go test ./internal/app/idgen/server/...
go test ./internal/component/idgen/...
```

### How to choose delayExecType?

| Scenario | Recommended Strategy | Reason |
|----------|---------------------|--------|
| User offline check | `skip` | Multiple executions produce the same result; skipping is safe |
| File cleanup | `skip` | The next schedule covers the previous time range |
| Data synchronization | `queue` | Sync batches must not be lost |
| Metric collection | `concurrent` | Collection windows are independent |

### Cron task errors immediately after startup?

**Possible causes:**

1. The database/Redis connection has not been established yet
2. The task function accesses uninitialized components

**Solution:**

- Use EGO's `Invoker` mechanism to ensure components are initialized before cron registration
- Add nil-safety checks in the task function (refer to the `NewMachineLeaseRenewCron` implementation)
- Set `enableImmediatelyRun = false` so the task only executes at the first scheduled time

## Reference Links

- EGO Job package: `github.com/gotomicro/ego/task/ejob`
- EGO Cron package: `github.com/gotomicro/ego/task/ecron`
- ERedis Cron Lock: `github.com/gotomicro/ego-component/eredis/ecronlock`
- `internal/app/user/internal/job/cron.go` -- user service cron task aggregation
- `internal/app/user/internal/job/user.go` -- user service cron task definitions
- `internal/app/gateway/internal/job/cron.go` -- gateway service cron tasks
- `internal/app/idgen/server/cron.go` -- idgen service machine cleanup cron
- `internal/component/idgen/machine_cron.go` -- machine lease renewal cron
- `internal/app/user/server/server.go` -- user service cron registration
- `internal/app/gateway/server/server.go` -- gateway service cron registration
- `internal/app/idgen/server/server.go` -- idgen service cron registration
- `configs/user/config.toml` -- user service cron configuration example
- `configs/gateway/local-live.toml` -- gateway service cron configuration example
