//go:build e2e

package main

import (
	"fmt"
	"os"
	"path/filepath"
)

func (e *environment) writeComposeFile() error {
	content := fmt.Sprintf(`services:
  mysql-idgen:
    image: mysql:8.4.5
    command:
      - --character-set-server=utf8mb4
      - --collation-server=utf8mb4_unicode_ci
    environment:
      MYSQL_ROOT_PASSWORD: egoadmin
      MYSQL_DATABASE: egoadmin_idgen
      MYSQL_USER: egoadmin
      MYSQL_PASSWORD: egoadmin
    ports:
      - "127.0.0.1:%d:3306"
    volumes:
      - ./data/idgen/mysql:/var/lib/mysql
    healthcheck:
      test: ["CMD-SHELL", "mysqladmin ping -h localhost -uegoadmin -pegoadmin"]
      interval: 5s
      timeout: 5s
      retries: 30
      start_period: 20s

  mysql-gateway:
    image: mysql:8.4.5
    command:
      - --character-set-server=utf8mb4
      - --collation-server=utf8mb4_unicode_ci
    environment:
      MYSQL_ROOT_PASSWORD: egoadmin
      MYSQL_DATABASE: egoadmin_gateway
      MYSQL_USER: egoadmin
      MYSQL_PASSWORD: egoadmin
    ports:
      - "127.0.0.1:%d:3306"
    volumes:
      - ./data/gateway/mysql:/var/lib/mysql
    healthcheck:
      test: ["CMD-SHELL", "mysqladmin ping -h localhost -uegoadmin -pegoadmin"]
      interval: 5s
      timeout: 5s
      retries: 30
      start_period: 20s

  mysql-user:
    image: mysql:8.4.5
    command:
      - --character-set-server=utf8mb4
      - --collation-server=utf8mb4_unicode_ci
    environment:
      MYSQL_ROOT_PASSWORD: egoadmin
      MYSQL_DATABASE: egoadmin_user
      MYSQL_USER: egoadmin
      MYSQL_PASSWORD: egoadmin
    ports:
      - "127.0.0.1:%d:3306"
    volumes:
      - ./data/user/mysql:/var/lib/mysql
    healthcheck:
      test: ["CMD-SHELL", "mysqladmin ping -h localhost -uegoadmin -pegoadmin"]
      interval: 5s
      timeout: 5s
      retries: 30
      start_period: 20s

  redis-gateway:
    image: redis:8.0-alpine
    command: ["redis-server", "--requirepass", "egoadmin", "--appendonly", "no"]
    ports:
      - "127.0.0.1:%d:6379"
    volumes:
      - ./data/gateway/redis:/data
    healthcheck:
      test: ["CMD", "redis-cli", "-a", "egoadmin", "ping"]
      interval: 5s
      timeout: 5s
      retries: 30

  redis-user:
    image: redis:8.0-alpine
    command: ["redis-server", "--requirepass", "egoadmin", "--appendonly", "no"]
    ports:
      - "127.0.0.1:%d:6379"
    volumes:
      - ./data/user/redis:/data
    healthcheck:
      test: ["CMD", "redis-cli", "-a", "egoadmin", "ping"]
      interval: 5s
      timeout: 5s
      retries: 30

  etcd:
    image: quay.io/coreos/etcd:v3.5.15
    command:
      - /usr/local/bin/etcd
      - --name=default
      - --data-dir=/data
      - --listen-client-urls=http://0.0.0.0:2379
      - --advertise-client-urls=http://etcd:2379
      - --listen-peer-urls=http://0.0.0.0:2380
      - --initial-advertise-peer-urls=http://etcd:2380
      - --initial-cluster=default=http://etcd:2380
      - --initial-cluster-token=%s
      - --initial-cluster-state=new
    ports:
      - "127.0.0.1:%d:2379"
      - "127.0.0.1:%d:2380"
    volumes:
      - ./data/etcd:/data
    healthcheck:
      test: ["CMD", "etcdctl", "endpoint", "health", "--endpoints=http://127.0.0.1:2379"]
      interval: 5s
      timeout: 5s
      retries: 30

  minio:
    image: minio/minio:RELEASE.2025-02-18T16-25-55Z-cpuv1
    entrypoint:
      - /bin/sh
      - -c
      - |
        (
          until (/usr/bin/mc alias set localminio http://localhost:9000 egoadmin egoadmin123) do
            sleep 1
          done
          /usr/bin/mc mb --ignore-existing localminio/egoadmin
          /usr/bin/mc anonymous set download localminio/egoadmin
        ) &
        exec minio server /data --console-address ":9001"
    environment:
      MINIO_ROOT_USER: egoadmin
      MINIO_ROOT_PASSWORD: egoadmin123
    ports:
      - "127.0.0.1:%d:9000"
      - "127.0.0.1:%d:9001"
    volumes:
      - ./data/minio:/data
    healthcheck:
      test: ["CMD-SHELL", "/usr/bin/mc alias set health_check http://localhost:9000 egoadmin egoadmin123 && /usr/bin/mc ready health_check"]
      interval: 10s
      timeout: 5s
      retries: 30
      start_period: 20s

  image-processor:
    image: shumc/imagor:1.5.15
    environment:
      PORT: 2853
      IMAGOR_SECRET: e2e-image-processor-secret
      IMAGOR_STORAGE_PATH_STYLE: digest
      IMAGOR_RESULT_STORAGE_PATH_STYLE: suffix
      IMAGOR_AUTO_WEBP: 1
      IMAGOR_AUTO_AVIF: 1
      AWS_ACCESS_KEY_ID: egoadmin
      AWS_SECRET_ACCESS_KEY: egoadmin123
      AWS_REGION: us-east-1
      S3_LOADER_BUCKET: egoadmin
      S3_STORAGE_BUCKET: egoadmin
      S3_RESULT_STORAGE_BUCKET: egoadmin
      S3_ENDPOINT: http://minio:9000
      S3_FORCE_PATH_STYLE: 1
    depends_on:
      minio:
        condition: service_healthy
    ports:
      - "127.0.0.1:%d:2853"

  jaeger:
    image: jaegertracing/all-in-one:1.40
    environment:
      COLLECTOR_ZIPKIN_HOST_PORT: :9411
      COLLECTOR_OTLP_ENABLED: "true"
    ports:
      - "127.0.0.1:%d:4317"
      - "127.0.0.1:%d:16686"
    healthcheck:
      test: ["CMD", "wget", "--spider", "-q", "http://127.0.0.1:16686/"]
      interval: 5s
      timeout: 5s
      retries: 30
`, e.ports.MySQLIDGen, e.ports.MySQLGateway, e.ports.MySQLUser, e.ports.RedisGateway, e.ports.RedisUser, e.runID, e.ports.EtcdClient, e.ports.EtcdPeer, e.ports.MinIOAPI, e.ports.MinIOConsole, e.ports.ImageProcessor, e.ports.JaegerOTLP, e.ports.JaegerHTTP)

	return os.WriteFile(filepath.Join(e.root, "docker-compose.yml"), []byte(content), 0o644)
}

func (e *environment) writeConfigFiles() error {
	configDir := filepath.Join(e.root, "configs")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(configDir, "idgen.toml"), []byte(e.idgenConfig()), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(configDir, "user.toml"), []byte(e.userConfig()), 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(configDir, "gateway.toml"), []byte(e.gatewayConfig()), 0o644)
}

func (e *environment) idgenConfig() string {
	return fmt.Sprintf(`
[app.service]
autoMigrate = false
name = "egoadmin-idgen"
platformName = "核心管理平台"
skipPermissionContractCheck = true
bucketName = "egoadmin"

[app.dbMigration]
enabled = true
driver = "atlas"
url = "%s"
dir = "file://atlas/migrations/idgen"
bin = "atlas"

[server.http]
enableAccessInterceptorReq = false
enableAccessInterceptorRes = false
enableMetricInterceptor = false
enableTraceInterceptor = false
enableCors = false
host = "127.0.0.1"
port = %d
mode = "release"

[server.grpc]
host = "127.0.0.1"
port = %d

[server.governor]
host = "127.0.0.1"
port = %d

[client.mysql]
debug = false
dsn = "egoadmin:egoadmin@tcp(127.0.0.1:%d)/egoadmin_idgen?charset=utf8mb4&parseTime=True&loc=Local&readTimeout=2s&timeout=2s&writeTimeout=3s"

[etcd]
addrs = ["127.0.0.1:%d"]
connectTimeout = "1s"

[registry]
scheme = "etcd"
prefix = "egoadmin"
serviceTTL = "10s"

[trace]
ServiceName = "egoadmin-idgen-e2e"
OtelType = "otlp"
Fraction = 1.00

[trace.otlp]
Endpoint = "127.0.0.1:%d"
`, e.idgenAtlasURL(), e.ports.IDGenHTTP, e.ports.IDGenGRPC, e.ports.IDGenGovern, e.ports.MySQLIDGen, e.ports.EtcdClient, e.ports.JaegerOTLP)
}

func (e *environment) userConfig() string {
	return fmt.Sprintf(`
[app.service]
autoMigrate = false
name = "egoadmin-user"
platformName = "核心管理平台"
skipPermissionContractCheck = true
bucketName = "egoadmin"

[app.dbMigration]
enabled = true
driver = "atlas"
url = "%s"
dir = "file://atlas/migrations/user"
bin = "atlas"

[app.user]
adminPassword = "123456"
jwtExpire = 604800
refreshTokenExpire = 2592000
jwtSignKey = "e2e-egoadmin-jwt-sign-key-%s"
useCaptcha = false
multiLoginEnabled = true
maxLoginClient = 5
heartbeatOfflineEnabled = true
heartbeatOfflineSeconds = 660
revokeSessionOnHeartbeatOffline = false

[server.http]
enableAccessInterceptorReq = false
enableAccessInterceptorRes = false
enableMetricInterceptor = false
enableTraceInterceptor = false
enableCors = false
host = "127.0.0.1"
port = %d
mode = "release"

[server.grpc]
host = "127.0.0.1"
port = %d

[server.governor]
host = "127.0.0.1"
port = %d

[client.mysql]
debug = false
dsn = "egoadmin:egoadmin@tcp(127.0.0.1:%d)/egoadmin_user?charset=utf8mb4&parseTime=True&loc=Local&readTimeout=2s&timeout=2s&writeTimeout=3s"

[client.redis]
addr = "127.0.0.1:%d"
debug = false
mode = "stub"
password = "egoadmin"

[client.grpc.idgen]
addr = "etcd:///egoadmin-idgen"
debug = false
readTimeout = "3s"
dialTimeout = "5s"

[client.jetcache]
name = "default"
remoteExpiry = "1h0m0s"
localSize = 256
localExpiry = "1m0s"
refreshDuration = "2m0s"
stopRefreshAfter = "1h0m0s"
notFoundExpiry = "1m0s"
enableMetrics = true
enableSyncLocal = false
codec = "msgpack"

[component.logincrypto]
challengeTTL = "3m0s"
timestampSkew = "2m0s"
rsaKeyBits = 4096
enableMetrics = true

[component.idgen.default]
namespace = "%s-user"
name = "default"

[component.idgen.machine]
group = "%s-user"

[component.idgen.codec]
secret = "e2e-stable-idcodec-secret-%s"

[client.asyncq]
redisAddr = "127.0.0.1:%d"
redisPassword = "egoadmin"
redisDB = 0
enableClient = true
enableServer = false
concurrency = 10
queues = { critical = 6, default = 3, low = 1 }
maxRetry = 3
retryDelayFunc = "exponential"
taskTimeout = "30s"
enableAccessLog = false
slowLogThreshold = "1s"
enableHealthCheck = false

[client.minio]
endpoint = "127.0.0.1:%d"
accessKeyID = "egoadmin"
secretAccessKey = "egoadmin123"
ssl = false

[etcd]
addrs = ["127.0.0.1:%d"]
connectTimeout = "1s"

[registry]
scheme = "etcd"
prefix = "egoadmin"
serviceTTL = "10s"

[trace]
ServiceName = "egoadmin-user-e2e"
OtelType = "otlp"
Fraction = 1.00

[trace.otlp]
Endpoint = "127.0.0.1:%d"

[cron.user.login.offline]
delayExecType = "skip"
enableDistributedTask = false
enableImmediatelyRun = true
enableSeconds = true
spec = "*/5 * * * * ?"
`, e.userAtlasURL(), e.runID, e.ports.UserHTTP, e.ports.UserGRPC, e.ports.UserGovern, e.ports.MySQLUser, e.ports.RedisUser, e.runID, e.runID, e.runID, e.ports.RedisUser, e.ports.MinIOAPI, e.ports.EtcdClient, e.ports.JaegerOTLP)
}

func (e *environment) gatewayConfig() string {
	return fmt.Sprintf(`
[app.web]
apiBaseUrl = ""
fileBaseUrl = "http://127.0.0.1:%d/egoadmin/"
offlineOnPageLeave = false

[app.service]
autoMigrate = false
name = "egoadmin-gateway"
platformName = "核心管理平台"
webPath = "/tmp/egoadmin/core/frontend/html"
skipPermissionContractCheck = false
bucketName = "egoadmin"

[app.dbMigration]
enabled = true
driver = "atlas"
url = "%s"
dir = "file://atlas/migrations/gateway"
bin = "atlas"

[server.http]
enableAccessInterceptorReq = false
enableAccessInterceptorRes = false
enableMetricInterceptor = false
enableTraceInterceptor = true
enableURLPathTrans = true
host = "127.0.0.1"
port = %d
mode = "release"
accessControlAllowCredentials = true
accessControlAllowOrigin = ["*"]
enableCors = true
ginRelativePath = "/api/*action"
grpcEndpoint = "127.0.0.1:%d"
incomingHeaders = ["Authorization", "X-User-Id"]
stripPrefix = "/api"

[server.grpc]
host = "127.0.0.1"
port = %d

[server.governor]
host = "127.0.0.1"
port = %d

[client.grpc.user]
addr = "etcd:///egoadmin-user"
debug = false
readTimeout = "3s"
dialTimeout = "5s"

[client.grpc.idgen]
addr = "etcd:///egoadmin-idgen"
debug = false
readTimeout = "3s"
dialTimeout = "5s"

[client.mysql]
debug = false
dsn = "egoadmin:egoadmin@tcp(127.0.0.1:%d)/egoadmin_gateway?charset=utf8mb4&parseTime=True&loc=Local&readTimeout=2s&timeout=2s&writeTimeout=3s"

[client.redis]
addr = "127.0.0.1:%d"
debug = false
mode = "stub"
password = "egoadmin"

[client.jetcache]
name = "default"
remoteExpiry = "1h0m0s"
localSize = 256
localExpiry = "1m0s"
refreshDuration = "2m0s"
stopRefreshAfter = "1h0m0s"
notFoundExpiry = "1m0s"
enableMetrics = true
enableSyncLocal = false
codec = "msgpack"

[component.idgen.default]
namespace = "%s-gateway"
name = "default"

[component.idgen.machine]
group = "%s-gateway"

[component.idgen.codec]
secret = "e2e-stable-idcodec-secret-%s"

[client.asyncq]
redisAddr = "127.0.0.1:%d"
redisPassword = "egoadmin"
redisDB = 0
enableClient = true
enableServer = false
concurrency = 10
queues = { critical = 6, default = 3, low = 1 }
maxRetry = 3
retryDelayFunc = "exponential"
taskTimeout = "30s"
enableAccessLog = false
slowLogThreshold = "1s"
enableHealthCheck = false

[client.meili]
host = "http://127.0.0.1:7700"
apiKey = ""
timeout = "5s"
enableHealth = false
ensureOnBuild = false
enableAccessLog = false
slowLog = "1s"

[component.upload.tus]
enabled = true
path = "/tus/upload"
temporaryDirectory = "%s/tus"

[component.cdn]
signSecret = "e2e-cdn-sign-secret"
publicImage = true
allowTemporaryImage = false

[client.imageProcessor]
url = "http://127.0.0.1:%d"
secret = "e2e-image-processor-secret"
timeout = "5s"

[client.minio]
endpoint = "127.0.0.1:%d"
accessKeyID = "egoadmin"
secretAccessKey = "egoadmin123"
ssl = false

[etcd]
addrs = ["127.0.0.1:%d"]
connectTimeout = "1s"

[registry]
scheme = "etcd"
prefix = "egoadmin"
serviceTTL = "10s"

[trace]
ServiceName = "egoadmin-gateway-e2e"
OtelType = "otlp"
Fraction = 1.00

[trace.otlp]
Endpoint = "127.0.0.1:%d"
`, e.ports.MinIOAPI, e.gatewayAtlasURL(), e.ports.GatewayHTTP, e.ports.GatewayGRPC, e.ports.GatewayGRPC, e.ports.GatewayGovern, e.ports.MySQLGateway, e.ports.RedisGateway, e.runID, e.runID, e.runID, e.ports.RedisGateway, filepath.ToSlash(e.root), e.ports.ImageProcessor, e.ports.MinIOAPI, e.ports.EtcdClient, e.ports.JaegerOTLP)
}
