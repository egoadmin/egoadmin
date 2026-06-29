# Log Analysis

This page explains EgoAdmin's logging architecture and analysis methods, helping quickly locate service issues, trace request chains, and diagnose business anomalies through logs.

## Overview

EgoAdmin uses the EGO framework's logging system to produce structured JSON logs. The logging system has three layers:

- **Runtime logs**: `logs/ego.sys` file, recording service startup, request processing, component interactions, and other runtime information.
- **Access logs**: Enabled via configuration, recording request parameters and response values for each HTTP/gRPC request.
- **Audit logs**: Business operation logs in the `sys_log` table, recording users' key operations.

Log analysis workflow for troubleshooting:

1. Check ERROR / FATAL level logs in `logs/ego.sys`.
2. Correlate request logs across services using `trace_id`.
3. Enable access logs to get full request parameters and return values.
4. Query the `sys_log` table for business operation audits.

::: tip
EgoAdmin logs default to output in both `logs/ego.sys` file and stdout. In container environments, stdout logs can be viewed via `docker logs` or log aggregation platforms.
:::

## Core Usage

### EGO Structured Logs

The EGO framework uses JSON-formatted structured logs. Each log entry contains standardized fields:

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

Core log field descriptions:

| Field | Description |
|-------|-------------|
| `level` | Log level: debug, info, warn, error, fatal |
| `ts` | Unix timestamp (seconds, with millisecond precision) |
| `caller` | Call location (filename:line) |
| `msg` | Log message |
| `service` | Service name |
| `method` | Request method path |
| `duration` | Request duration |
| `trace_id` | Distributed trace ID for cross-service correlation |
| `code` | Business status code |

### Access Logs

Access logs record detailed information for each request, controlled by configuration flags:

```toml
# configs/user/config.toml
[server.http]
enableAccessInterceptorReq = true   # Log request parameters
enableAccessInterceptorRes = false  # Log response body (use cautiously in production)

[server.grpc]
enableAccessInterceptorReq = true
enableAccessInterceptorRes = false
```

Sample output with request parameter logging enabled:

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
Enabling `enableAccessInterceptorRes` logs complete response bodies, which may contain sensitive data. In production, consider enabling only request parameter logging, or selectively enable as needed.
:::

### Slow Logs

The EGO framework supports component-level slow log threshold configuration. When an operation exceeds the threshold, it is automatically logged at warn level.

Redis slow log example:

```toml
# configs/user/config.toml
[client.redis]
debug = true   # Enable Redis command logging (includes execution time)
```

Output when enabled:

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

### Error Pattern Recognition

Here are common error patterns in EgoAdmin logs and their corresponding troubleshooting directions:

**codes.Unavailable - Downstream Service Unreachable**

```json
{
  "level": "error",
  "msg": "rpc error: code = Unavailable desc = connection error",
  "method": "/api/user.Create"
}
```

Troubleshooting:
- Confirm the target service is running.
- Check etcd registry is healthy.
- Check network connectivity and firewall rules.

**AuthMissingToken - Missing Authentication Token**

```json
{
  "level": "warn",
  "msg": "AuthMissingToken",
  "method": "/api/dept.List",
  "peer": "127.0.0.1:54321"
}
```

Troubleshooting:
- Confirm request Header contains `Authorization: Bearer <token>`.
- Check if the frontend token has expired.
- Confirm gateway auth interceptor is configured correctly.

**DataPermissionOutOfScope - Data Permission Out of Scope**

```json
{
  "level": "warn",
  "msg": "DataPermissionOutOfScope",
  "userId": 1234,
  "targetDeptId": 5678
}
```

Troubleshooting:
- Check user's data permission scope configuration (self/department/department and subordinates/all).
- Confirm `sys_user.dept_id` and data permission rules match.
- Check Casbin policy loading is correct.

**LoginFailed - Login Failure**

```json
{
  "level": "warn",
  "msg": "LoginFailed",
  "username": "admin",
  "reason": "invalid credentials"
}
```

Troubleshooting:
- Confirm username and password are correct.
- Check if user account is disabled.
- Confirm login encryption parameters match (loginCrypto).

## Configuration Examples

### Full Access Log Configuration

```toml
# configs/user/config.toml

# HTTP server access logs
[server.http]
enableAccessInterceptorReq = true   # Log request parameters
enableAccessInterceptorRes = true   # Log response body (for debugging)

# gRPC server access logs
[server.grpc]
enableAccessInterceptorReq = true
enableAccessInterceptorRes = true

# Redis command logging
[client.redis]
debug = true
```

### Log Output to stdout (Container Environments)

```toml
[app.log]
# EGO logs default to output to both file and stdout
# View stdout via docker logs or log aggregation systems in container environments
dir = "logs"
name = "ego.sys"
```

### Audit Log Queries

EgoAdmin records business operation logs in the `sys_log` table:

```sql
-- Query recent operation logs
SELECT * FROM sys_log ORDER BY created_at DESC LIMIT 20;

-- Query by operation type
SELECT * FROM sys_log WHERE type = 'login' ORDER BY created_at DESC LIMIT 20;

-- Query by user
SELECT * FROM sys_log WHERE user_id = 1234 ORDER BY created_at DESC LIMIT 20;

-- Query by time range
SELECT * FROM sys_log
WHERE created_at BETWEEN '2024-01-01' AND '2024-01-31'
ORDER BY created_at DESC;
```

## Real-World Examples

### Example 1: Cross-Service Request Tracing

**Symptom**: An API occasionally times out, and it's unclear whether the gateway or user service is the cause.

**Diagnosis**:

1. Find the `trace_id` of the timed-out request in gateway logs:

```bash
grep "duration" logs/ego.sys | grep -v "duration\":\"[0-9]" | tail -20
# Find the log entry with abnormal duration, note the trace_id
```

2. Search the full call chain in Jaeger using the `trace_id`:

```bash
# Open Jaeger UI
open http://localhost:16686

# Search for the trace_id, view each span's duration
```

3. Search for the same `trace_id` across service logs:

```bash
grep "abc123def456" logs/ego.sys
```

**Solution**:

Jaeger tracing revealed that the gateway -> user gRPC call took too long because the user service's MySQL connection pool was exhausted, causing requests to queue for connections. Adjusting `maxOpenConns` resolved the issue.

### Example 2: Batch Error Log Analysis

**Symptom**: A large number of ERROR level logs appear in service logs, requiring quick error distribution statistics.

**Diagnosis**:

```bash
# Count error log entries
grep '"level":"error"' logs/ego.sys | wc -l

# Categorize by error message
grep '"level":"error"' logs/ego.sys | jq -r '.msg' | sort | uniq -c | sort -rn | head -20

# Error distribution by time period
grep '"level":"error"' logs/ego.sys | jq -r '.ts' | cut -d. -f1 | awk '{print strftime("%Y-%m-%d %H:%M", $1)}' | uniq -c
```

**Solution**:

Prioritize based on error distribution. Address high-frequency errors first, then analyze root causes of low-frequency errors individually.

### Example 3: Request Parameter Audit

**Symptom**: Need to audit a user's API call records within a specific time period.

**Diagnosis**:

Query via log aggregation after enabling access logs:

```bash
# Filter request logs by user IP
grep '"peer":"192.168.1.100"' logs/ego.sys | jq '.method, .req, .ts'

# Filter by API path
grep '/api/dept.Delete' logs/ego.sys | jq '{ts, peer, req}'
```

For audit logs (`sys_log` table), query via API or directly:

```sql
SELECT * FROM sys_log
WHERE user_id = 1234
  AND created_at BETWEEN '2024-06-01' AND '2024-06-30'
ORDER BY created_at DESC;
```

### Example 4: Log Aggregation Export

**Symptom**: Need to export EgoAdmin logs to a centralized log platform (such as Loki or ELK).

**Approach**:

The EGO framework is based on `log/slog`, and logs can be sent to external systems via slog handlers:

```text
Log export path:
EGO structured logs → slog handler → Loki / Elasticsearch / stdout
```

Common log aggregation approaches:

1. **File collection**: Use Promtail (Loki) or Filebeat (ELK) to collect the `logs/ego.sys` file.
2. **Sidecar container**: Add a log collection sidecar to the Kubernetes Pod.
3. **slog backend**: Send logs directly to the target platform via `samber/slog-*` libraries.

::: tip
File collection is recommended as it is non-intrusive to EgoAdmin code. For more granular log routing, consider a custom slog handler.
:::

### Example 5: Error Code to User Feedback Mapping

**Symptom**: Frontend shows generic error messages, unable to map to specific backend error reasons.

**Diagnosis**:

EgoAdmin's error codes are defined in the `codes` package within each service's `internal/` directory. Correlate via the `code` field in logs:

```bash
# Search for specific error codes
grep '"code":401' logs/ego.sys | tail -10

# Search for data permission errors
grep 'DataPermissionOutOfScope' logs/ego.sys | jq '{ts, userId, targetDeptId}'
```

**Solution**:

The frontend should parse error codes returned by the backend (not just HTTP status codes) and map user-friendly error messages to corresponding error codes.

## How It Works

EgoAdmin's logging system is based on the EGO framework's `elog` component, using Go's standard library `log/slog` underneath.

```text
Log data flow:
Application code → ego.ILogger → slog.Handler → Output (file/stdout)
                                ↓
                          Formatting (JSON)
                                ↓
                          Field injection (trace_id, service, method)
```

Access logs are implemented through EGO's interceptor mechanism:

```text
HTTP/gRPC request → EGO interceptor (request interception)
                    ↓ Log request params (enableAccessInterceptorReq)
               Business Handler processing
                    ↓ Log response (enableAccessInterceptorRes)
               EGO interceptor (response interception)
                    ↓ Calculate duration, inject trace_id
               Log output
```

## Common Issues

### Empty Log File

**Symptom**: `logs/ego.sys` file is empty or does not exist.

**Solution**:

- Confirm the service has started successfully.
- Check write permissions for the `logs/` directory.
- Confirm the `dir` path in log configuration is correct.

### Excessive Log Volume

**Symptom**: Log file grows too quickly, disk space runs out.

**Solution**:

```bash
# Check log file size
ls -lh logs/ego.sys

# Count log entries
wc -l logs/ego.sys
```

- Disable unnecessary access logs (`enableAccessInterceptorRes = false`).
- Disable Redis debug logging (`debug = false`).
- Set up log rotation (via external tools like logrotate).

::: warning
In production, do not enable both `enableAccessInterceptorReq` and `enableAccessInterceptorRes` simultaneously unless actively troubleshooting a specific issue. Full access logs significantly increase disk I/O and storage consumption.
:::

### Empty trace_id

**Symptom**: `trace_id` field is empty or missing in logs.

**Troubleshooting**:

- Confirm Jaeger agent is running (`make dev-up` includes Jaeger).
- Confirm tracing is enabled in service configuration.
- If calling directly rather than through the gateway, trace context must be manually injected.

## Reference Links

- [EGO Framework Logging](https://github.com/gotomicro/ego)
- [Jaeger Distributed Tracing](https://www.jaegertracing.io/)
- [Grafana Loki Log Aggregation](https://grafana.com/oss/loki/)
- [Go slog Standard Library](https://pkg.go.dev/log/slog)
