# 日志分析

本页介绍 EgoAdmin 的日志体系结构和分析方法，帮助通过日志快速定位服务问题、追踪请求链路和排查业务异常。

## 概述

EgoAdmin 基于 EGO 框架的日志系统，产生结构化 JSON 日志。日志体系包含三个层次：

- **运行日志**：`logs/ego.sys` 文件，记录服务启动、请求处理、组件交互等运行时信息。
- **访问日志**：通过配置开启，记录每个 HTTP/gRPC 请求的入参和出参。
- **审计日志**：`sys_log` 表中的业务操作日志，记录用户的关键操作。

排查问题时的日志分析流程：

1. 检查 `logs/ego.sys` 中的错误级别日志（ERROR / FATAL）。
2. 通过 `trace_id` 跨服务关联请求日志。
3. 开启访问日志获取完整的请求入参和返回值。
4. 查询 `sys_log` 表审计业务操作。

::: tip
EgoAdmin 日志默认输出到 `logs/ego.sys` 文件和 stdout。容器环境下 stdout 日志可通过 `docker logs` 或日志聚合平台查看。
:::

## 核心用法

### EGO 结构化日志

EGO 框架使用 JSON 格式的结构化日志，每个日志条目包含标准化字段：

```json
{
  "level": "info",
  "ts": 1719500000.123,
  "caller": "controller/user.go:42",
  "msg": "request completed",
  "service": "egoadmin-user",
  "method": "/api/user.List",
  "duration": "23.5ms",
  "trace_id": "abc123def456",
  "code": 0
}
```

核心日志字段说明：

| 字段 | 说明 |
|------|------|
| `level` | 日志级别：debug、info、warn、error、fatal |
| `ts` | Unix 时间戳（秒，含毫秒精度） |
| `caller` | 调用位置（文件名:行号） |
| `msg` | 日志消息 |
| `service` | 服务名称 |
| `method` | 请求方法路径 |
| `duration` | 请求耗时 |
| `trace_id` | 链路追踪 ID，用于跨服务关联 |
| `code` | 业务状态码 |

### 访问日志

访问日志记录每个请求的详细信息，通过配置项控制开关：

```toml
# configs/user/config.toml
[server.http]
enableAccessInterceptorReq = true   # 记录请求入参
enableAccessInterceptorRes = false  # 记录响应出参（生产环境谨慎开启）

[server.grpc]
enableAccessInterceptorReq = true
enableAccessInterceptorRes = false
```

开启请求入参日志后的输出示例：

```json
{
  "level": "info",
  "ts": 1719500000.123,
  "msg": "access",
  "method": "POST /api/user.Create",
  "peer": "127.0.0.1:54321",
  "req": "{\"username\":\"admin\",\"deptId\":1}",
  "duration": "15.2ms",
  "trace_id": "abc123"
}
```

::: warning
开启 `enableAccessInterceptorRes` 会在日志中记录完整的响应体，可能包含敏感数据。生产环境建议仅开启请求入参日志，或根据需要选择性开启。
:::

### 慢日志

EGO 框架支持组件级别的慢日志阈值配置。当某个操作耗时超过阈值时，自动记录为 warn 级别日志。

Redis 慢日志示例：

```toml
# configs/user/config.toml
[client.redis]
debug = true   # 开启 Redis 命令日志（包含执行时间）
```

开启后的日志输出：

```json
{
  "level": "warn",
  "ts": 1719500000.456,
  "msg": "redis slow log",
  "cmd": "HGETALL user:1234",
  "cost": "156ms",
  "trace_id": "abc123"
}
```

### 错误模式识别

以下是 EgoAdmin 日志中常见的错误模式和对应的排查方向：

**codes.Unavailable - 下游服务不可达**

```json
{
  "level": "error",
  "msg": "rpc error: code = Unavailable desc = connection error",
  "method": "/api/user.Create"
}
```

排查方向：
- 确认目标服务是否运行。
- 检查 etcd 注册中心是否正常。
- 检查网络连通性和防火墙规则。

**AuthMissingToken - 缺少认证 Token**

```json
{
  "level": "warn",
  "msg": "AuthMissingToken",
  "method": "/api/dept.List",
  "peer": "127.0.0.1:54321"
}
```

排查方向：
- 确认请求 Header 中包含 `Authorization: Bearer <token>`。
- 检查前端 Token 是否过期。
- 确认网关认证拦截器配置正确。

**DataPermissionOutOfScope - 数据权限越界**

```json
{
  "level": "warn",
  "msg": "DataPermissionOutOfScope",
  "userId": 1234,
  "targetDeptId": 5678
}
```

排查方向：
- 检查用户的数据权限范围配置（本人/本部门/本部门及下级/全部）。
- 确认 `sys_user.dept_id` 和数据权限规则是否匹配。
- 检查 Casbin 策略加载是否正确。

**LoginFailed - 登录失败**

```json
{
  "level": "warn",
  "msg": "LoginFailed",
  "username": "admin",
  "reason": "invalid credentials"
}
```

排查方向：
- 确认用户名和密码是否正确。
- 检查用户状态是否被禁用。
- 确认登录加密参数是否匹配（loginCrypto）。

## 配置示例

### 完整访问日志配置

```toml
# configs/user/config.toml

# HTTP 服务访问日志
[server.http]
enableAccessInterceptorReq = true   # 记录请求入参
enableAccessInterceptorRes = true   # 记录响应出参（调试用）

# gRPC 服务访问日志
[server.grpc]
enableAccessInterceptorReq = true
enableAccessInterceptorRes = true

# Redis 命令日志
[client.redis]
debug = true
```

### 日志输出到 stdout（容器环境）

```toml
[app.log]
# EGO 日志默认同时输出到文件和 stdout
# 容器环境通过 docker logs 或日志聚合系统查看 stdout
dir = "logs"
name = "ego.sys"
```

### 审计日志查询

EgoAdmin 在 `sys_log` 表中记录用户的业务操作日志：

```sql
-- 查询最近的操作日志
SELECT * FROM sys_log ORDER BY created_at DESC LIMIT 20;

-- 按操作类型查询
SELECT * FROM sys_log WHERE type = 'login' ORDER BY created_at DESC LIMIT 20;

-- 按用户查询
SELECT * FROM sys_log WHERE user_id = 1234 ORDER BY created_at DESC LIMIT 20;

-- 按时间范围查询
SELECT * FROM sys_log
WHERE created_at BETWEEN '2024-01-01' AND '2024-01-31'
ORDER BY created_at DESC;
```

## 实际案例

### 案例一：跨服务请求追踪

**症状**：某个 API 偶尔超时，无法确定是 gateway 还是 user 服务导致。

**诊断**：

1. 在 gateway 日志中找到超时请求的 `trace_id`：

```bash
grep "duration" logs/ego.sys | grep -v "duration\":\"[0-9]" | tail -20
# 找到耗时异常的日志条目，记录 trace_id
```

2. 使用 `trace_id` 在 Jaeger 中搜索完整调用链：

```bash
# 打开 Jaeger UI
open http://localhost:16686

# 搜索 trace_id，查看各 span 耗时
```

3. 在各服务日志中搜索同一 `trace_id`：

```bash
grep "abc123def456" logs/ego.sys
```

**解决**：

通过 Jaeger 追踪发现 gateway -> user 的 gRPC 调用耗时过长，原因是 user 服务的 MySQL 连接池满，请求排队等待连接。调整 `maxOpenConns` 后问题解决。

### 案例二：批量错误日志分析

**症状**：服务日志中出现大量 ERROR 级别日志，需要快速统计错误分布。

**诊断**：

```bash
# 统计错误日志数量
grep '"level":"error"' logs/ego.sys | wc -l

# 按错误消息分类统计
grep '"level":"error"' logs/ego.sys | jq -r '.msg' | sort | uniq -c | sort -rn | head -20

# 按时间段统计错误分布
grep '"level":"error"' logs/ego.sys | jq -r '.ts' | cut -d. -f1 | awk '{print strftime("%Y-%m-%d %H:%M", $1)}' | uniq -c
```

**解决**：

根据错误分布确定优先级。高频错误优先排查，低频错误逐个分析根因。

### 案例三：请求入参审计

**症状**：需要审计某用户在特定时间段的 API 调用记录。

**诊断**：

开启访问日志后通过日志聚合查询：

```bash
# 按用户 IP 过滤请求日志
grep '"peer":"192.168.1.100"' logs/ego.sys | jq '.method, .req, .ts'

# 按 API 路径过滤
grep '/api/dept.Delete' logs/ego.sys | jq '{ts, peer, req}'
```

对于审计日志（`sys_log` 表），通过 API 或直接查询：

```sql
SELECT * FROM sys_log
WHERE user_id = 1234
  AND created_at BETWEEN '2024-06-01' AND '2024-06-30'
ORDER BY created_at DESC;
```

### 案例四：日志聚合导出

**症状**：需要将 EgoAdmin 日志导出到集中式日志平台（如 Loki 或 ELK）。

**方案**：

EGO 框架基于 `log/slog`，可以通过 slog handler 将日志发送到外部系统：

```text
日志导出路径：
EGO 结构化日志 → slog handler → Loki / Elasticsearch / stdout
```

常见的日志聚合方案：

1. **文件采集**：使用 Promtail（Loki）或 Filebeat（ELK）采集 `logs/ego.sys` 文件。
2. **Sidecar 容器**：在 Kubernetes Pod 中添加日志采集 Sidecar。
3. **slog 后端**：通过 `samber/slog-*` 系列库直接将日志发送到目标平台。

::: tip
推荐使用文件采集方案，对 EgoAdmin 代码无侵入性。如果需要更细粒度的日志路由，可以考虑自定义 slog handler。
:::

### 案例五：错误码与用户反馈关联

**症状**：前端显示通用错误提示，无法对应到后端具体错误原因。

**诊断**：

EgoAdmin 的错误码定义在 `internal/` 各服务的 `codes` 包中。通过日志中的 `code` 字段关联：

```bash
# 搜索特定错误码
grep '"code":401' logs/ego.sys | tail -10

# 搜索数据权限错误
grep 'DataPermissionOutOfScope' logs/ego.sys | jq '{ts, userId, targetDeptId}'
```

**解决**：

前端应解析后端返回的错误码（而非仅依赖 HTTP 状态码），将用户友好的错误信息映射到对应错误码。

## 工作原理

EgoAdmin 的日志体系基于 EGO 框架的 `elog` 组件，底层使用 Go 标准库 `log/slog`。

```text
日志数据流：
应用代码 → ego.ILogger → slog.Handler → 输出（文件/stdout）
                                ↓
                          格式化（JSON）
                                ↓
                          字段注入（trace_id, service, method）
```

访问日志通过 EGO 的拦截器（interceptor）机制实现：

```text
HTTP/gRPC 请求 → EGO interceptor（请求拦截）
                    ↓ 记录入参（enableAccessInterceptorReq）
               业务 Handler 处理
                    ↓ 记录出参（enableAccessInterceptorRes）
               EGO interceptor（响应拦截）
                    ↓ 计算 duration，注入 trace_id
               日志输出
```

## 常见问题

### 日志文件为空

**症状**：`logs/ego.sys` 文件为空或不存在。

**解决**：

- 确认服务已正常启动。
- 检查 `logs/` 目录的写入权限。
- 确认日志配置中 `dir` 路径正确。

### 日志量过大

**症状**：日志文件增长速度过快，磁盘空间不足。

**解决**：

```bash
# 统计日志文件大小
ls -lh logs/ego.sys

# 统计日志条目数
wc -l logs/ego.sys
```

- 关闭不必要的访问日志（`enableAccessInterceptorRes = false`）。
- 关闭 Redis debug 日志（`debug = false`）。
- 设置日志轮转（通过外部工具如 logrotate）。

::: warning
生产环境不要同时开启 `enableAccessInterceptorReq` 和 `enableAccessInterceptorRes`，除非正在排查特定问题。全量访问日志会显著增加磁盘 I/O 和存储消耗。
:::

### trace_id 为空

**症状**：日志中 `trace_id` 字段为空或不存在。

**排查**：

- 确认 Jaeger agent 已启动（`make dev-up` 包含 Jaeger）。
- 确认服务配置中启用了 tracing。
- 如果是直接调用而非经过 gateway，需要手动注入 trace context。

## 参考链接

- [EGO 框架日志文档](https://github.com/gotomicro/ego)
- [Jaeger 分布式追踪](https://www.jaegertracing.io/)
- [Grafana Loki 日志聚合](https://grafana.com/oss/loki/)
- [Go slog 标准库](https://pkg.go.dev/log/slog)
