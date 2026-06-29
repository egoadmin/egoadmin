# Job 与 Cron 定时任务

本页介绍 EGO 框架的 Job 短时任务和 Cron 定时任务机制，以及 EgoAdmin 中的实际使用场景。

## 概述

EGO 框架提供两种任务执行机制：**Job**（一次性任务）和 **Cron**（定时循环任务）。两者都与 EGO 生命周期深度集成，支持优雅停机、链路追踪和结构化日志。

| 机制 | 触发方式 | 执行次数 | 典型场景 |
|------|----------|----------|----------|
| Job | CLI `--job` 参数或 HTTP 请求 | 一次，执行后进程退出 | 数据迁移、批量导入、一次性修复 |
| Cron | cron 表达式调度 | 周期性执行 | 心跳离线检查、日志清理、租约续期 |

EgoAdmin 中 Cron 的典型应用：

- **用户心跳离线检查**：定时扫描超过心跳超时的在线用户，标记为离线状态
- **上传文件清理**：清理过期的临时上传文件
- **IDGen 机器租约续期**：定时续租 ID 生成器的机器编号
- **IDGen 过期机器清理**：清理已过期的机器租约记录
- **审计日志清理**：清理两年前的审计日志记录

## Job（短时任务）

Job 是一次性的任务执行单元。EGO 支持三种 Job 使用模式：CLI 触发、HTTP 触发和 Cobra 集成。

### 模式一：CLI 触发 Job

最简单的 Job 模式，通过命令行参数 `--job` 指定要执行的任务名称。进程启动后执行指定 Job，完成后退出。

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

**运行方式：**

```bash
# 执行单个 Job
go run main.go --job=job1

# 执行多个 Job（逗号分隔）
go run main.go --job=job1,job2
```

::: tip Job 执行后的行为
Job 执行完成后 EGO 会自动退出进程。如果同时注册了服务器（HTTP/gRPC），Job 执行期间服务器不会启动。`--job` 参数用于选择性执行，未匹配的 Job 不会被运行。
:::

### 模式二：HTTP 触发 Job

通过 Governor HTTP 端点触发 Job 执行，适用于需要外部系统或运维脚本触发的场景。

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
	// 处理请求体数据
	ctx.Writer.Write([]byte("i am ok"))
	return nil
}
```

**HTTP 触发方式：**

```bash
curl -XPOST -d '{"username":"ego"}' \
  -H 'X-Ego-Job-Name:job' \
  -H 'X-Ego-Job-RunID:unique-run-id' \
  http://127.0.0.1:9003/jobs
```

**请求头说明：**

| Header | 必填 | 说明 |
|--------|------|------|
| `X-Ego-Job-Name` | 是 | Job 名称，必须与注册名匹配 |
| `X-Ego-Job-RunID` | 是 | 唯一执行 ID，用于请求去重和追踪 |
| Request Body | 否 | 业务数据，Job 函数通过 `ctx.Request` 读取 |

**Governor 端点：**

```bash
# 查看已注册的 Job 列表
curl http://127.0.0.1:9003/job/list
```

返回已注册的 Job 名称列表，用于运维监控。

::: warning HTTP Job 并发安全
每个 HTTP 触发的 Job 在独立的 goroutine 中执行。如果 Job 执行时间较长，注意并发控制。`X-Ego-Job-RunID` 用于防止重复提交。
:::

### 模式三：Cobra 集成 Job

将 Job 作为 Cobra 子命令集成到 CLI 工具中，适用于需要复杂参数解析的场景。

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
	// 业务逻辑
	return nil
}
```

运行方式：

```bash
go run main.go run --job=doSomething
```

::: tip 与 CLI 模式的区别
Cobra 集成允许自定义命令结构和参数解析。`ego.WithArguments(os.Args[2:])` 确保 EGO 能正确解析 `--job` 参数。
:::

### Job 函数签名

所有 Job 函数的签名统一为 `func(ejob.Context) error`：

```go
type Context struct {
    Ctx     context.Context   // 请求上下文，携带 traceId
    Request *http.Request     // HTTP 请求（仅 HTTP 触发时有值）
    Writer  http.ResponseWriter // HTTP 响应（仅 HTTP 触发时有值）
}
```

- `ctx.Ctx` 始终可用，包含链路追踪信息
- `ctx.Request` 和 `ctx.Writer` 仅在 HTTP 触发模式下有效

## Cron（定时任务）

Cron 是周期性执行的定时任务。EGO 通过 `ecron` 包提供完整的 cron 调度能力，支持秒级精度、分布式锁和延迟执行策略。

### 配置

Cron 任务通过 TOML 配置文件定义，配置节名称即为任务标识：

```toml
[cron.test]
enableDistributedTask = false    # 是否启用分布式任务（需要 Redis 锁）
enableImmediatelyRun = false     # 启动时是否立即执行一次
enableSeconds = false            # 是否启用秒级解析器
spec = "*/5 * * * * *"           # cron 表达式（enableSeconds=true 时每 5 秒）
delayExecType = "skip"           # 延迟执行策略：skip | queue | concurrent
```

### 配置字段说明

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `spec` | string | 必填 | cron 表达式 |
| `enableDistributedTask` | bool | `false` | 启用分布式锁，确保集群中只有一个节点执行 |
| `enableImmediatelyRun` | bool | `false` | 进程启动后立即执行一次，不等待首次调度时间 |
| `enableSeconds` | bool | `false` | 启用秒级 cron 解析器。为 `false` 时标准 5 位表达式（分时日月周）；为 `true` 时 6 位表达式（秒分时日月周） |
| `delayExecType` | string | `"skip"` | 上一次未完成时的处理策略 |
| `waitLockTime` | duration | `4s` | 获取分布式锁的等待时间 |
| `lockTTL` | duration | `16s` | 分布式锁租约时长 |
| `refreshGap` | duration | `4s` | 分布式锁刷新间隔 |

### 延迟执行策略

当上一次任务执行尚未完成，新的调度时间已到时，`delayExecType` 决定如何处理：

| 策略 | 行为 | 适用场景 |
|------|------|----------|
| `skip` | 跳过本次执行 | 绝大多数场景，避免重复执行 |
| `queue` | 排队等待，上一次完成后立即执行 | 任务不能丢失，但执行频率不敏感 |
| `concurrent` | 并发执行 | 任务幂等且无共享状态 |

::: tip EgoAdmin 默认策略
EgoAdmin 的 cron 任务统一使用 `delayExecType = "skip"`，避免重复执行带来的数据一致性问题。
:::

### 基础用法

最简单的 Cron 注册方式，通过 `ecron.Load` 加载配置并绑定执行函数：

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

对应配置：

```toml
[cron.test]
enableDistributedTask = false
enableImmediatelyRun = false
enableSeconds = false
spec = "*/5 * * * *"
delayExecType = "skip"
```

::: warning 配置节名称匹配
`ecron.Load("cron.test")` 中的参数必须与 TOML 配置节名完全匹配。配置节名格式为 `cron.<task_name>`，EGO 框架通过此名称查找对应的 `spec` 和其他配置。
:::

### 分布式 Cron

在多实例部署时，为了避免同一任务被多个节点重复执行，可以启用分布式锁。EGO 通过 `ecronlock` 组件基于 Redis 实现分布式锁。

**配置：**

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

**代码实现：**

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
			// 离线过期用户的业务逻辑
			return nil
		}),
	)
}
```

**分布式锁工作原理：**

1. 调度时间到达时，节点尝试获取 Redis 锁
2. 锁使用 `SET NX` 原子操作，只有获取到锁的节点执行任务
3. 执行期间通过定时刷新锁的 TTL 续租
4. 任务执行完成后释放锁
5. 如果节点崩溃，锁会在 TTL 过期后自动释放

::: warning 锁超时配置
`lockTTL` 应大于任务的最大执行时间，否则锁可能在任务执行中途过期，导致其他节点获取锁并重复执行。建议 `lockTTL` 设置为任务最大执行时间的 2-3 倍。
:::

### Cron 函数签名

Cron 任务函数的签名为 `func(context.Context) error`：

- `context.Context` 包含链路追踪信息，可通过 `etrace.ExtractTraceID(ctx)` 获取 traceId
- 返回 `nil` 表示执行成功，返回 `error` 会记录 ERROR 日志但不影响下次调度

## EgoAdmin 实战示例

### 用户心跳离线检查

用户服务通过 cron 任务定期扫描超过心跳超时的在线用户，将其标记为离线。这是 EgoAdmin 中最核心的定时任务之一。

**配置（`configs/user/config.toml`）：**

```toml
[cron.user.login.offline]
delayExecType = "skip"
enableDistributedTask = false
enableImmediatelyRun = true
enableSeconds = true
spec = "0 0/1 * * * ?"
```

**配置含义：**
- 每分钟执行一次（`0/1` 表示每分钟的第 0 秒）
- `enableSeconds = true`：使用 6 位 cron 表达式，首位为秒
- `enableImmediatelyRun = true`：进程启动后立即执行一次
- `delayExecType = "skip"`：如果上一分钟的任务未完成，跳过本次
- `enableDistributedTask = false`：单实例模式不需要分布式锁

**任务注册（`internal/app/user/internal/job/user.go`）：**

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

**任务聚合（`internal/app/user/internal/job/cron.go`）：**

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

**在服务启动时注册（`internal/app/user/server/server.go`）：**

```go
func newApp(opts Options, _ schemaReady) (*App, error) {
	// ... 启动关键组件 ...

	configureShutdown(opts)

	opts.app.Registry(opts.registry)
	opts.app.Serve(opts.http, opts.grpc, opts.govern)
	opts.app.Cron(opts.cron.Tasks()...)      // 注册业务 cron 任务
	if opts.idm != nil {
		opts.app.Cron(idgen.NewMachineLeaseRenewCron(opts.idm)) // 注册机器租约续期
	}

	// ... 其他初始化 ...
	opts.health.Ready()
	return &App{Ego: opts.app}, nil
}
```

::: tip Wire 依赖注入
`job.Cron` 通过 Wire 自动注入 `UserOfflineService` 接口实现（即 `*service.UserService`）。Wire ProviderSet 定义在 `internal/app/user/internal/job/cron.go`，确保依赖关系在编译时检查。
:::

### 上传文件清理

Gateway 服务通过 cron 任务定期清理过期的临时上传文件，防止存储空间无限增长。

**配置（`configs/gateway/local-live.toml`）：**

```toml
[cron.gateway.upload.cleanup]
delayExecType = "skip"
enableDistributedTask = false
enableImmediatelyRun = false
enableSeconds = true
spec = "0 0/10 * * * ?"
```

每 10 分钟执行一次，启动时不立即执行（因为需要清理的文件通常不会在启动瞬间产生）。

**任务实现（`internal/app/gateway/internal/job/cron.go`）：**

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

### IDGen 机器租约续期

IDGen 服务通过数据库行锁分配机器编号，user 和 gateway 服务需要定时续租以保持机器编号有效。

**任务实现（`internal/component/idgen/machine_cron.go`）：**

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

**在 user 和 gateway 服务中注册：**

```go
// internal/app/user/server/server.go
// internal/app/gateway/server/server.go
opts.app.Cron(idgen.NewMachineLeaseRenewCron(opts.idm))
```

::: warning 条件注册
`NewMachineLeaseRenewCron` 内部对 `manager == nil` 做了安全检查。在不使用 IDGen 的部署拓扑中（`opts.idm == nil`），此 cron 任务不会执行任何操作。
:::

### IDGen 过期机器清理

IDGen 服务通过 cron 任务清理已过期的机器租约记录，释放被占用的机器编号。

**任务实现（`internal/app/idgen/server/cron.go`）：**

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

**在 idgen 服务中注册：**

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

`newMachineLeaseCleanupCron` 通过 Wire ProviderSet 自动注入到 `Options.cron` 字段。

### 审计日志清理

用户服务提供了 `CleanLog` 方法用于清理两年前的审计日志。虽然当前配置中未作为独立 cron 任务注册，但方法已就绪，可通过添加 cron 配置和任务注册来启用：

```go
// internal/app/user/service/user_service.go
func (s *UserService) CleanLog(ctx context.Context) (err error) {
	// 清理两年前的审计日志
	// ...
}
```

若要启用为 cron 任务，可在 `internal/app/user/internal/job/` 中添加：

```go
func auditLogCleanup(opts *Options) ecron.Ecron {
	return ecron.Load("cron.user.audit.log.cleanup").Build(
		ecron.WithJob(func(ctx context.Context) error {
			// 调用 CleanLog 清理日志
			return nil
		}),
	)
}
```

## 工作原理

### Job 执行流程

```
ego.New().Job(ejob.Job("name", fn)).Run()
    |
    v
EGO 解析命令行参数 --job
    |
    ├── 未提供 --job → 正常启动服务器模式（忽略 Job）
    |
    └── 提供了 --job=name
            |
            v
        匹配已注册的 Job
            |
            ├── 未匹配 → 报错退出
            |
            └── 匹配成功
                    |
                    v
                为每个 Job 创建 context（携带 traceId）
                    |
                    v
                顺序执行 Job 函数
                    |
                    v
                所有 Job 完成后进程退出
```

### Cron 调度流程

```
ego.New().Cron(ecron.Load("cron.xxx").Build(...)).Run()
    |
    v
EGO 创建 cron 调度器
    |
    v
注册所有 Cron 任务，解析 spec 表达式
    |
    v
启动调度循环
    |
    v
调度时间到达
    |
    ├── enableDistributedTask = false
    |       → 直接执行任务函数
    |
    └── enableDistributedTask = true
            |
            v
        尝试获取 Redis 分布式锁（SET NX + TTL）
            |
            ├── 获取失败 → 跳过本次执行
            |
            └── 获取成功
                    |
                    v
                创建 context（携带 traceId）
                    |
                    v
                执行任务函数
                    |
                    ├── 刷新锁 TTL（续租）
                    |
                    v
                任务完成 → 释放锁
                    |
                    v
                记录执行结果日志
```

### 分布式锁机制

`ecronlock` 组件基于 Redis 实现分布式锁，核心语义：

1. **加锁**：`SET key value NX PX lockTTL`，原子操作确保只有一个客户端能成功
2. **续租**：执行期间按 `refreshGap` 间隔刷新锁 TTL，防止长任务执行中锁过期
3. **释放**：任务完成后删除锁 key，使用 Lua 脚本确保只释放自己持有的锁
4. **故障恢复**：节点崩溃后锁在 `lockTTL` 后自动过期，其他节点可接管

```
节点 A 获取锁
    |
    v
执行任务 + 定期续租（每 refreshGap 秒）
    |
    ├── 任务完成 → 删除锁 → 节点 B/C 可在下次调度时竞争
    |
    └── 节点崩溃 → 锁在 lockTTL 后自动过期 → 其他节点接管
```

### 延迟执行策略对比

```
场景：上一次任务执行耗时超过调度间隔

skip 策略：
    第 1 次调度 → [===执行中===]
    第 2 次调度 → 跳过（任务仍在执行）
    第 3 次调度 → [===执行中===]

queue 策略：
    第 1 次调度 → [===执行中===]
    第 2 次调度 → 排队等待
                              → 执行（不等待下次调度）
    第 3 次调度 → [===执行中===]

concurrent 策略：
    第 1 次调度 → [===执行中===]
    第 2 次调度 → [===执行中===]  （并行执行）
    第 3 次调度 → [===执行中===]  （并行执行）
```

### 链路追踪集成

每次 Cron 任务执行时，EGO 会自动创建新的 trace context。任务函数可通过 `etrace.ExtractTraceID(ctx)` 获取当前 traceId，用于日志关联：

```go
ecron.WithJob(func(ctx context.Context) error {
    traceId := etrace.ExtractTraceID(ctx)
    elog.Info("cron executing",
        elog.FieldTid(traceId),
        elog.FieldComponentName("cron.user.login.offline"),
    )
    // 业务逻辑
    return nil
})
```

### 优雅停机

Cron 任务与 EGO 的优雅停机机制集成：

1. 进程收到 `SIGTERM`/`SIGINT`
2. Cron 调度器停止接受新的调度
3. 等待当前正在执行的任务完成（受 `stopTimeout` 约束）
4. 如果启用了分布式锁，释放持有的锁
5. 进程退出

::: danger 不要中断正在执行的任务
强制杀进程（`SIGKILL`）会导致正在执行的 cron 任务中断。如果启用了分布式锁，锁会在 `lockTTL` 后自动过期，其他节点才会接管。建议使用 `SIGTERM` 触发优雅停机。
:::

## 常见问题

### Job 找不到怎么办？

**症状**：运行 `go run main.go --job=myjob` 后报错提示 job not found。

**排查步骤**：

1. 检查 `--job=myjob` 的名称是否与 `ejob.Job("myjob", fn)` 注册名完全一致（区分大小写）
2. 检查是否拼写错误
3. 使用 Governor 端点 `GET /job/list` 查看已注册的 Job 列表

```bash
# 查看已注册 Job
curl http://127.0.0.1:9003/job/list
```

### Cron 任务不执行怎么办？

**症状**：配置了 cron 任务但到时间后没有执行。

**排查步骤**：

1. **检查 spec 表达式**：确认 cron 表达式语法正确
   ```bash
   # 使用在线工具验证 cron 表达式
   # enableSeconds=false 时使用 5 位：分 时 日 月 周
   # enableSeconds=true 时使用 6 位：秒 分 时 日 月 周
   ```
2. **检查 enableSeconds 设置**：如果 spec 包含秒字段但 `enableSeconds=false`，表达式解析会失败
3. **检查 enableImmediatelyRun**：如果为 `false`，进程启动后不会立即执行，需等待第一个调度时间
4. **检查日志**：EGO 会在启动时打印 cron 调度器的注册信息

### 多实例部署时任务重复执行怎么办？

**症状**：多个节点同时执行了同一个 cron 任务。

**解决方案**：

1. 启用分布式任务配置
2. 配置 Redis 连接
3. 在代码中使用 `ecron.WithLock` 注册分布式锁

```toml
[cron.user.login.offline]
enableDistributedTask = true    # 关键：启用分布式锁

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

### 任务执行超时怎么办？

**症状**：长耗时任务执行期间锁过期，导致其他节点获取锁并重复执行。

**解决方案**：

1. 增大 `lockTTL`（默认 16s），建议设为任务最大执行时间的 2-3 倍
2. 减小 `refreshGap`（默认 4s），加快续租频率
3. 优化任务执行时间，考虑分批处理

```toml
[cron.my.long.task]
enableDistributedTask = true
lockTTL = "120s"          # 增大锁超时
refreshGap = "10s"        # 保持合理的续租间隔
waitLockTime = "10s"      # 增大锁等待时间
spec = "0 0 2 * * ?"
enableSeconds = true
delayExecType = "skip"
```

### 如何测试 Cron 任务？

```go
// 单元测试：直接调用任务函数
func TestUserOffline(t *testing.T) {
    svc := &mockUserOfflineService{}
    opts := &Options{User: svc}
    cron := userOffline(opts)

    // 获取 job 函数并调用
    // 验证 OfflineUser 被正确调用
}

// 集成测试：启动服务验证调度
// 1. 配置 enableImmediatelyRun = true
// 2. 启动服务后观察日志
// 3. 验证任务执行结果
```

```bash
# 运行相关单元测试
go test ./internal/app/user/internal/job/...
go test ./internal/app/gateway/internal/job/...
go test ./internal/app/idgen/server/...
go test ./internal/component/idgen/...
```

### delayExecType 如何选择？

| 场景 | 推荐策略 | 原因 |
|------|----------|------|
| 用户离线检查 | `skip` | 多次执行结果相同，跳过不影响正确性 |
| 文件清理 | `skip` | 下次调度时会覆盖本次范围 |
| 数据同步 | `queue` | 不能丢失同步批次 |
| 指标收集 | `concurrent` | 采集窗口独立，互不影响 |

### 进程启动后 cron 任务立刻报错？

**可能原因**：

1. 依赖的数据库/Redis 连接尚未就绪
2. 任务函数内部访问了未初始化的组件

**解决方案**：

- 使用 EGO 的 `Invoker` 机制确保组件在 cron 注册前完成初始化
- 在任务函数中对 nil 依赖做安全检查（参考 `NewMachineLeaseRenewCron` 的实现）
- 考虑设置 `enableImmediatelyRun = false`，让任务在首次调度时间才执行

## 参考链接

- EGO Job 包：`github.com/gotomicro/ego/task/ejob`
- EGO Cron 包：`github.com/gotomicro/ego/task/ecron`
- ERedis Cron Lock：`github.com/gotomicro/ego-component/eredis/ecronlock`
- `internal/app/user/internal/job/cron.go` -- user 服务 cron 任务聚合
- `internal/app/user/internal/job/user.go` -- user 服务 cron 任务定义
- `internal/app/gateway/internal/job/cron.go` -- gateway 服务 cron 任务
- `internal/app/idgen/server/cron.go` -- idgen 服务机器清理 cron
- `internal/component/idgen/machine_cron.go` -- 机器租约续期 cron
- `internal/app/user/server/server.go` -- user 服务 cron 注册
- `internal/app/gateway/server/server.go` -- gateway 服务 cron 注册
- `internal/app/idgen/server/server.go` -- idgen 服务 cron 注册
- `configs/user/config.toml` -- user 服务 cron 配置示例
- `configs/gateway/local-live.toml` -- gateway 服务 cron 配置示例
