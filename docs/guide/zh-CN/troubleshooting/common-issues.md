# 常见问题诊断

本页汇总 EgoAdmin 开发和部署过程中最常遇到的问题，按照故障场景分类，提供诊断步骤和解决方案。

## 概述

EgoAdmin 是由 gateway、user、idgen 三个微服务组成的系统，配合 MySQL、Redis、etcd、MinIO、DTM、Jaeger 等中间件运行。问题通常发生在服务启动、服务间通信、认证鉴权和前端构建四个阶段。

排查时建议按以下顺序进行：

1. 确认中间件已全部就绪（`make dev-up` 或 `make deploy-up` 输出无报错）。
2. 确认服务健康检查通过（`curl http://localhost:<port>/healthz`）。
3. 查看服务日志 `logs/ego.sys` 定位具体错误。
4. 根据本页对应章节进行针对性排查。

::: tip
运行 `make service.config SERVICE=<name>` 可以查看当前生效的服务完整配置，确认是否有误。
:::

## 服务启动失败

### 端口被占用

**症状**：服务启动后立即退出，日志中出现 `bind: address already in use`。

**诊断**：

```bash
netstat -tlnp | grep 9001
netstat -tlnp | grep 9101
netstat -tlnp | grep 9201
```

**解决**：

修改 `configs/<service>/config.toml` 中对应端口，或停止占用端口的进程。

```toml
# gateway/config.toml
[server.http]
port = 9001

# user/config.toml
[server.http]
port = 9101
[server.grpc]
port = 9102
```

::: warning
gateway 的 gRPC 端口由 `grpcEndpoint` 指定（默认 `127.0.0.1:9002`），user 和 idgen 各自使用独立的 HTTP 和 gRPC 端口。
:::

### 中间件未就绪

**症状**：服务启动后日志出现 `connection refused` 或 `context deadline exceeded`，连接 MySQL / Redis / etcd 失败。

**诊断**：

```bash
# 确认中间件容器运行状态
docker ps

# 检查 etcd
curl http://127.0.0.1:2379/health

# 检查 MySQL
mysql -h 127.0.0.1 -P 3307 -u egoadmin -pegoadmin -e "SELECT 1"
```

**解决**：

```bash
# 启动全部中间件
make dev-up

# 如果某个中间件有问题，单独重启
make dev.reset-one MIDDLEWARE=mysql-user
```

::: tip
`make dev-up` 必须在启动任何服务之前完成。中间件的容器名和端口定义在 `test/compose/docker-compose.dev.yml`。
:::

### 配置文件错误

**症状**：服务启动后 panic 或报 `unmarshal` 错误。

**诊断**：

```bash
# 查看默认配置
go run ./cmd/user config print-default
make service.config SERVICE=gateway
```

**解决**：

对照默认输出检查 `configs/<service>/config.toml` 中的 key 名和类型。常见错误：

- 端口写成字符串而非整数。
- 缺少必填的 `[app.service]` 段。
- TOML 数组语法错误（`[]` 内缺少引号）。

### Atlas 迁移失败

**症状**：服务启动日志出现 `migration` 或 `atlas` 相关错误。

**诊断**：

```bash
# 检查 atlas 是否安装
atlas version

# 检查迁移目录
ls atlas/migrations/user/

# 验证迁移文件
make migrate.validate SERVICE=user
```

**解决**：

1. 确认 atlas 二进制已安装：`make install`。
2. 确认 `configs/<service>/config.toml` 中 `[app.dbMigration].url` 指向正确的数据库。
3. 确认迁移目录与服务对应（user 用 `atlas/migrations/user`，gateway 用 `atlas/migrations/gateway`）。

```toml
[app.dbMigration]
enabled = true
driver = "atlas"
url = "mysql://egoadmin:egoadmin@127.0.0.1:3307/egoadmin_user?charset=utf8mb4&parseTime=True&loc=Local"
dir = "file://atlas/migrations/user"
```

::: warning
迁移 hash 校验失败时，执行 `make migrate.hash SERVICE=user` 重新生成 `atlas.sum`。
:::

## 数据库连接问题

### 连接超时

**症状**：日志出现 `dial tcp: i/o timeout` 或 `connection timed out`。

**诊断**：

```bash
# 确认 MySQL 容器运行
docker ps | grep mysql

# 测试端口可达
nc -zv 127.0.0.1 3307

# 检查 DSN 格式
grep "dsn" configs/user/config.toml
```

**解决**：

本地开发环境 DSN 示例：

```toml
[client.mysql]
dsn = "egoadmin:egoadmin@tcp(127.0.0.1:3307)/egoadmin_user?charset=utf8mb4&parseTime=True&loc=Local"
```

容器环境必须使用 Docker Compose 服务名：

```toml
[client.mysql]
dsn = "egoadmin:egoadmin@tcp(mysql-user:3306)/egoadmin_user?charset=utf8mb4&parseTime=True&loc=Local"
```

### 认证失败

**症状**：日志出现 `Access denied for user`。

**解决**：

- 确认 DSN 中用户名和密码与 MySQL 容器初始化参数一致。
- 默认开发凭据为 `egoadmin:egoadmin`，定义在 `test/compose/docker-compose.dev.yml`。
- 容器环境下通过 `ATLAS_URL` 环境变量覆盖：

```bash
export ATLAS_URL='mysql://user:password@mysql-user:3306/egoadmin_user?charset=utf8mb4&parseTime=True&loc=Local'
```

### Schema 不匹配

**症状**：SQL 执行报 `Table 'xxx' doesn't exist` 或 `Unknown column`。

**解决**：

```bash
# 执行迁移
make migrate.apply SERVICE=user ATLAS_URL='mysql://egoadmin:egoadmin@127.0.0.1:3307/egoadmin_user?charset=utf8mb4&parseTime=True&loc=Local'

# 或者确认 autoMigrate 配置
grep autoMigrate configs/user/config.toml
```

## 认证与权限错误

### JWT Token 过期

**症状**：API 返回 `401 Unauthorized`，前端跳转到登录页。

**诊断**：

检查 `jwtExpire` 配置（单位：秒）：

```toml
[app.user]
jwtExpire = 604800        # access token 有效期，7 天
refreshTokenExpire = 2592000  # refresh token 有效期，30 天
```

**解决**：

前端应实现 refresh token 自动续期。如果开发时频繁遇到 token 过期，可临时增大 `jwtExpire`。

::: warning
`jwtSignKey` 在生产和开发环境必须不同。生产环境通过环境变量覆盖，不要提交到仓库。
:::

### 权限校验失败 (403)

**症状**：API 返回 `403 Forbidden`，前端按钮显示但无法操作。

**诊断**：

```text
权限链路排查顺序：
1. 用户是否有对应角色
2. 角色是否分配了对应菜单
3. 菜单是否绑定了该 API
4. Casbin 策略是否加载正确
```

**解决**：

- 进入系统管理 -> 角色管理，确认角色已分配目标菜单权限。
- 进入菜单管理，确认菜单的 API 标识与后端 OpenAPI 注解一致。
- 检查 `web/dist/permission-contract.json` 是否存在且与当前菜单同步：

```bash
test -f web/dist/permission-contract.json && echo ok || echo missing
```

::: tip
保存角色权限时，后端会使用 `permission-contract.json` 校验可授予的 API ID。如果前端未构建，该文件不存在，权限保存会失败。
:::

### 会话意外失效

**症状**：用户操作过程中突然被踢出，需要重新登录。

**诊断**：

```toml
[app.user]
heartbeatOfflineEnabled = true
heartbeatOfflineSeconds = 660   # 心跳超时，单位秒
multiLoginEnabled = true
maxLoginClient = 2
```

**解决**：

- 如果是开发环境频繁切换浏览器，可临时设置 `heartbeatOfflineEnabled = false`。
- 检查 Redis 是否正常运行，会话数据存储在 Redis 中：

```bash
redis-cli -h 127.0.0.1 -p 6380 -a egoadmin ping
```

### 登录加密挑战过期

**症状**：登录时前端控制台出现 challenge expired 或 timestamp skew 错误。

**解决**：

```toml
[component.logincrypto]
challengeTTL = "3m0s"     # 挑战有效期
timestampSkew = "2m0s"    # 时间偏差容忍
rsaKeyBits = 4096
```

前端获取 challenge 后需在 `challengeTTL` 内完成登录。如果开发调试较慢，可临时增大 `challengeTTL`。

## 微服务通信失败

### 服务发现异常

**症状**：gateway 调用 user/idgen 超时，日志中出现 `etcd` 相关错误。

**诊断**：

```bash
# 确认 etcd 健康
curl http://127.0.0.1:2379/health

# 确认服务已注册（查看 etcd 中的 key）
etcdctl get --prefix /egoadmin --keys-only

# 确认 EGO_NAME 配置
grep "name" configs/user/config.toml
```

**解决**：

每个服务的 `EGO_NAME` 必须与 gateway 客户端配置中引用的地址一致：

```toml
# user/config.toml
[app.service]
name = "egoadmin-user"  # EGO_NAME

# gateway/config.toml - 对应的客户端配置
[client.grpc.user]
addr = "etcd:///egoadmin-user"  # 必须匹配
```

::: warning
如果 etcd 地址配置不正确，服务注册会静默失败。检查所有服务的 etcd 配置指向同一个 etcd 实例。
:::

### gRPC 客户端超时

**症状**：gateway 日志出现 `context deadline exceeded`，请求 user 或 idgen 超时。

**解决**：

调整 gateway 中对应客户端的超时配置：

```toml
[client.grpc.user]
addr = "etcd:///egoadmin-user"
readTimeout = "3s"    # 单次请求超时
dialTimeout = "3s"    # 建连超时

[client.grpc.idgen]
addr = "etcd:///egoadmin-idgen"
readTimeout = "3s"
dialTimeout = "5s"    # idgen 建连超时更长，因为涉及 ID 分配初始化
```

### 容器 DNS 解析失败

**症状**：容器环境下服务间调用失败，日志中出现 `no such host`。

**解决**：

容器内必须使用 Docker Compose 服务名，不能使用 `127.0.0.1`：

```toml
# 正确 - 容器环境
[client.grpc.user]
addr = "etcd:///egoadmin-user"

[etcd]
addrs = ["etcd:2379"]

[client.mysql]
dsn = "egoadmin:egoadmin@tcp(mysql-user:3306)/egoadmin_user?..."

# 错误 - 不要在容器中使用 127.0.0.1
[etcd]
addrs = ["127.0.0.1:2379"]
```

## API 调用失败

### 参数校验错误

**症状**：API 返回 `400 Bad Request`，body 中包含 `invalid` 字样。

**诊断**：

检查 proto 文件中的 validation 标签：

```protobuf
string name = 1 [(buf.validate.field).string.min_len = 1];
```

**解决**：

- 确认请求字段类型正确（如 `uint64` 在 JSON 中是字符串，不是数字）。
- 确认必填字段已传入。
- 前端提交数据时使用 `JSON.stringify` 检查实际 payload。

### Proto 字段访问错误

**症状**：Go 代码中读取 proto 字段得到零值。

**解决**：

proto 生成的字段是私有的，必须使用 `GetXxx()` 访问器：

```go
// 错误
name := req.Name

// 正确
name := req.GetName()
```

### Copier 映射失败

**症状**：proto 对象到 domain 对象的字段复制不正确，字段为空。

**解决**：

检查 `copier` struct tag 是否匹配：

```go
type User struct {
    Name string `copier:"Name"`
}
```

proto 生成的字段名通常是 PascalCase，确保 copier tag 与之一致。如果字段类型不同（如 `uint64` vs `int64`），copier 可能跳过复制，需要手动赋值。

## 前端问题

### 构建失败

**症状**：`pnpm run build` 报错退出。

**诊断**：

```bash
cd web
node -v     # 需要 18+
pnpm -v     # 需要 11.5.x

# 清理重装
rm -rf node_modules pnpm-lock.yaml
pnpm install
```

**解决**：

- 确认 Node.js 和 pnpm 版本符合要求（见 `web/package.json` 中 `engines` 字段）。
- 如果类型检查失败，运行 `pnpm run type-check` 查看具体错误。
- 确认没有修改自动生成的文件（如 `*.gen.ts`）。

### 代理请求失败

**症状**：前端开发时 API 请求返回 502 或连接被拒。

**解决**：

确认后端服务已启动，并检查 `web/vite.config.ts` 中的代理目标地址：

```text
开发服务器默认代理 /api/* 到后端 gateway
后端默认地址为 http://127.0.0.1:9001
```

::: tip
前端开发服务器（`pnpm run dev`）和后端 gateway 必须同时运行，代理才能正常工作。
:::

### 路由 404

**症状**：登录后访问页面显示 404 或空白。

**解决**：

EgoAdmin 前端使用动态路由，菜单数据驱动路由生成：

1. 检查登录后是否成功获取菜单数据。
2. 检查 `permission` store 中的路由是否正确生成。
3. 检查菜单管理中该菜单的路由路径配置是否正确。

### 白屏

**症状**：页面打开后白屏，控制台有报错。

**诊断**：

打开浏览器开发者工具 -> Console，查看具体错误信息。

**常见原因**：

- 组件未注册：检查是否在 `main.ts` 中正确引入。
- 权限数据加载失败：检查网络请求是否返回 401。
- Vue Router 配置错误：检查路由 `component` 引用是否正确。

## 数据迁移问题

### 迁移 hash 不匹配

**症状**：`make migrate.apply` 报 hash mismatch 错误。

**解决**：

```bash
# 重新计算 hash
make migrate.hash SERVICE=user

# 提交更新后的 atlas.sum
git add atlas/migrations/user/atlas.sum
```

### 迁移文件冲突

**症状**：多人同时创建迁移导致 `atlas.sum` 冲突。

**解决**：

```bash
# 拉取最新代码后重新校验
git pull
make migrate.validate SERVICE=user

# 如果冲突，手动解决后重新 hash
make migrate.hash SERVICE=user
```

## 参考链接

- `configs/gateway/config.toml` — gateway 配置文件
- `configs/user/config.toml` — user 配置文件
- `configs/idgen/config.toml` — idgen 配置文件
- `test/compose/docker-compose.dev.yml` — 开发中间件编排
- `deploy/configs/` — 部署配置目录
- `atlas/migrations/` — 数据库迁移文件
- `web/vite.config.ts` — 前端代理配置
- `internal/component/authsession/` — 认证会话组件
- `internal/component/logincrypto/` — 登录加密组件
