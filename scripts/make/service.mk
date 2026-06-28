.PHONY: _require.service
_require.service:
	@if [ -z "$(SERVICE)" ]; then \
		echo "usage: make $${MAKECMDGOALS:-<target>} SERVICE=<idgen|gateway|user>"; \
		exit 1; \
	fi
	@test -d "cmd/$(SERVICE)" || (echo "unknown service $(SERVICE): missing cmd/$(SERVICE)"; exit 1)
	@test -f "configs/$(SERVICE)/config.toml" || (echo "missing configs/$(SERVICE)/config.toml"; exit 1)

.PHONY: _ensure.web-dist
_ensure.web-dist:
	@if [ "$(SKIP_WEB_BUILD)" = "1" ]; then \
		test -f web/dist/index.html || (echo "web/dist/index.html missing, run make web.build first"; exit 1); \
	elif [ ! -f web/dist/index.html ]; then \
		$(MAKE_CMD) web.build; \
	fi

.PHONY: _run.one
_run.one: _require.service
	@if [ "$(SERVICE)" = "gateway" ]; then \
		$(MAKE_CMD) _ensure.web-dist SKIP_WEB_BUILD="$(SKIP_WEB_BUILD)"; \
	fi
	@$(MAKE_CMD) EGO_DEBUG=true EGO_NAME=$(or $(EGO_NAME),$(PROJECT)-$(SERVICE)) GO_SERVICE=$(SERVICE) go.run

.PHONY: run
run: ## 运行服务。默认 SERVICES="idgen user gateway"，可用 SERVICE=<service> 单独运行
	@set -e; \
	if [ -n "$(SERVICE)" ]; then \
		$(MAKE_CMD) _run.one SERVICE="$(SERVICE)" EGO_NAME="$(EGO_NAME)" SKIP_WEB_BUILD="$(SKIP_WEB_BUILD)"; \
	else \
		if printf '%s\n' "$(SERVICES)" | grep -qw gateway; then \
			$(MAKE_CMD) _ensure.web-dist SKIP_WEB_BUILD="$(SKIP_WEB_BUILD)"; \
		fi; \
		pids=""; \
		trap 'for pid in $$pids; do kill $$pid 2>/dev/null || true; done; wait 2>/dev/null || true' INT TERM EXIT; \
		for service in $(SERVICES); do \
			$(MAKE_CMD) --no-print-directory _run.one SERVICE="$$service" SKIP_WEB_BUILD=1 & \
			pids="$$pids $$!"; \
			sleep $${RUN_START_INTERVAL:-2}; \
		done; \
		wait; \
	fi

.PHONY: build
build: ## 构建服务。可用 SERVICE=<service> 或 SERVICES="idgen user gateway"
	@set -e; \
	if [ -n "$(SERVICE)" ]; then \
		$(MAKE_CMD) _build.one SERVICE="$(SERVICE)" BIN="$(BIN)" SKIP_WEB_BUILD="$(SKIP_WEB_BUILD)"; \
	else \
		for service in $(SERVICES); do \
			$(MAKE_CMD) _build.one SERVICE="$$service" SKIP_WEB_BUILD="$(SKIP_WEB_BUILD)"; \
		done; \
	fi

.PHONY: _build.one
_build.one: _require.service
	@if [ "$(SERVICE)" = "gateway" ]; then \
		$(MAKE_CMD) _ensure.web-dist SKIP_WEB_BUILD="$(SKIP_WEB_BUILD)"; \
	fi
	@$(MAKE_CMD) GO_SERVICE=$(SERVICE) GO_BIN=$(or $(BIN),$(PROJECT)-$(SERVICE)) go.build

.PHONY: build.alpine
build.alpine: _require.service ## 构建指定服务 Alpine Linux 二进制。示例：make build.alpine SERVICE=user
	@if [ "$(SERVICE)" = "gateway" ]; then \
		$(MAKE_CMD) _ensure.web-dist SKIP_WEB_BUILD="$(SKIP_WEB_BUILD)"; \
	fi
	@$(MAKE_CMD) GO_SERVICE=$(SERVICE) GO_BIN=$(or $(BIN),$(PROJECT)-$(SERVICE)) go.build.alpine

.PHONY: build.alpine-arm64
build.alpine-arm64: _require.service ## 构建指定服务 Alpine arm64 二进制。示例：make build.alpine-arm64 SERVICE=gateway
	@if [ "$(SERVICE)" = "gateway" ]; then \
		$(MAKE_CMD) _ensure.web-dist SKIP_WEB_BUILD="$(SKIP_WEB_BUILD)"; \
	fi
	@$(MAKE_CMD) GO_SERVICE=$(SERVICE) GO_BIN=$(or $(BIN),$(PROJECT)-$(SERVICE)) go.build.alpine-arm64

.PHONY: wire
wire: service.check ## 生成指定服务 Wire 代码。示例：make wire SERVICE=user
	@GOFLAGS=-mod=mod wire gen ./internal/app/$(SERVICE)/server

.PHONY: service.config
service.config: _require.service ## 打印指定服务内置默认配置。示例：make service.config SERVICE=user
	@go run ./cmd/$(SERVICE) config print-default

.PHONY: service.check
service.check: _require.service ## 检查指定微服务目录结构和架构约束
	@test -f "cmd/$(SERVICE)/Dockerfile" || (echo "missing cmd/$(SERVICE)/Dockerfile"; exit 1)
	@test -d "internal/app/$(SERVICE)/server" || (echo "missing internal/app/$(SERVICE)/server"; exit 1)
	@test -d "internal/app/$(SERVICE)/controller" || (echo "missing internal/app/$(SERVICE)/controller"; exit 1)
	@test -d "internal/app/$(SERVICE)/application" || (echo "missing internal/app/$(SERVICE)/application"; exit 1)
	@test -d "internal/app/$(SERVICE)/domain" || (echo "missing internal/app/$(SERVICE)/domain"; exit 1)
	@test -d "internal/app/$(SERVICE)/adapter" || (echo "missing internal/app/$(SERVICE)/adapter"; exit 1)
	@test -d "internal/app/$(SERVICE)/schema" || (echo "missing internal/app/$(SERVICE)/schema"; exit 1)
	@test -d "atlas/migrations/$(SERVICE)" || (echo "missing atlas/migrations/$(SERVICE)"; exit 1)
	@test -f "atlas/migrations/$(SERVICE)/atlas.sum" || (echo "missing atlas/migrations/$(SERVICE)/atlas.sum"; exit 1)
	@test -f "internal/app/$(SERVICE)/server/wire.go" || (echo "missing internal/app/$(SERVICE)/server/wire.go"; exit 1)
	@test -f "internal/app/$(SERVICE)/server/http_server.go" || (echo "missing internal/app/$(SERVICE)/server/http_server.go for /healthz and /readyz"; exit 1)
	@grep -q '^\[server\.http\]' "configs/$(SERVICE)/config.toml" || (echo "missing [server.http] in configs/$(SERVICE)/config.toml"; exit 1)
	@if [ "$(SERVICE)" != "gateway" ]; then \
		grep -q '^enableCors = false' "configs/$(SERVICE)/config.toml" || (echo "health-only HTTP server must set enableCors = false in configs/$(SERVICE)/config.toml"; exit 1); \
		grep -q '^enableTraceInterceptor = false' "configs/$(SERVICE)/config.toml" || (echo "health-only HTTP server must set enableTraceInterceptor = false in configs/$(SERVICE)/config.toml"; exit 1); \
	fi
	@grep -R "ProviderSet" "internal/app/$(SERVICE)/server" >/dev/null || (echo "missing server ProviderSet in internal/app/$(SERVICE)/server"; exit 1)
	@grep -R "ProviderSet" "internal/app/$(SERVICE)/controller" >/dev/null || (echo "missing controller ProviderSet in internal/app/$(SERVICE)/controller"; exit 1)
	@grep -R "ProviderSet" "internal/app/$(SERVICE)/application" >/dev/null || (echo "missing application ProviderSet in internal/app/$(SERVICE)/application"; exit 1)
	@grep -R "ProviderSet" "internal/app/$(SERVICE)/adapter" >/dev/null || (echo "missing adapter ProviderSet in internal/app/$(SERVICE)/adapter"; exit 1)
	@test -f "internal/app/$(SERVICE)/server/grpc_server.go" || (echo "missing internal/app/$(SERVICE)/server/grpc_server.go"; exit 1)
	@grep -R "Register.*ServiceServer" "internal/app/$(SERVICE)/server" >/dev/null || (echo "missing gRPC service registration in internal/app/$(SERVICE)/server"; exit 1)
	@grep -R "NewHttpServer" "internal/app/$(SERVICE)/server/wire.go" "internal/app/$(SERVICE)/server/server.go" >/dev/null || (echo "missing NewHttpServer provider in internal/app/$(SERVICE)/server"; exit 1)
	@grep -R "health.Start" "internal/app/$(SERVICE)/server" >/dev/null || (echo "missing health.Start registration for /healthz and /readyz"; exit 1)
	@bash scripts/check_arch.sh "$(SERVICE)"

.PHONY: template.rename
template.rename: ## 基于模板元信息重命名当前项目。默认 dry-run，写入需 APPLY=1
	@if [ -z "$(NEW_NAME)" ] || [ -z "$(NEW_SLUG)" ] || [ -z "$(NEW_MODULE)" ]; then \
		echo "usage: make template.rename NEW_NAME=EgoAdmin NEW_SLUG=egoadmin NEW_MODULE=github.com/egoadmin/egoadmin ENV_PREFIX=EGOADMIN [GO_PACKAGE=egoadmin] [APPLY=1]"; \
		exit 1; \
	fi
	@go run ./tools/egoadminctl rename \
		$(if $(FROM_NAME),--from-name "$(FROM_NAME)") \
		$(if $(FROM_SLUG),--from-slug "$(FROM_SLUG)") \
		$(if $(FROM_MODULE),--from-module "$(FROM_MODULE)") \
		$(if $(FROM_ENV_PREFIX),--from-env-prefix "$(FROM_ENV_PREFIX)") \
		$(if $(FROM_GO_PACKAGE),--from-go-package "$(FROM_GO_PACKAGE)") \
		--name "$(NEW_NAME)" \
		--slug "$(NEW_SLUG)" \
		--module "$(NEW_MODULE)" \
		$(if $(ENV_PREFIX),--env-prefix "$(ENV_PREFIX)") \
		$(if $(GO_PACKAGE),--go-package "$(GO_PACKAGE)") \
		$(if $(TEMPLATE_SERVICES),--services "$(TEMPLATE_SERVICES)") \
		$(if $(INCLUDE_AGENTS),--include-agents) \
		$(if $(APPLY),--write)

.PHONY: template.init
template.init: ## 仓库内维护辅助入口：从模板仓库初始化新项目并完成重命名
	@if [ -z "$(TEMPLATE_DEST)" ] || [ -z "$(NEW_NAME)" ] || [ -z "$(NEW_SLUG)" ] || [ -z "$(NEW_MODULE)" ]; then \
		echo "usage: make template.init TEMPLATE_DEST=<dir> NEW_NAME=DemoAdmin NEW_SLUG=demoadmin NEW_MODULE=github.com/acme/demoadmin [TEMPLATE_REPO=<git-url>] [ENV_PREFIX=DEMOADMIN]"; \
		exit 1; \
	fi
	@go run ./tools/egoadminctl init \
		--dest "$(TEMPLATE_DEST)" \
		$(if $(TEMPLATE_REPO),--repo "$(TEMPLATE_REPO)") \
		$(if $(TEMPLATE_BRANCH),--branch "$(TEMPLATE_BRANCH)") \
		$(if $(TEMPLATE_DEPTH),--depth "$(TEMPLATE_DEPTH)") \
		$(if $(FROM_NAME),--from-name "$(FROM_NAME)") \
		$(if $(FROM_SLUG),--from-slug "$(FROM_SLUG)") \
		$(if $(FROM_MODULE),--from-module "$(FROM_MODULE)") \
		$(if $(FROM_ENV_PREFIX),--from-env-prefix "$(FROM_ENV_PREFIX)") \
		$(if $(FROM_GO_PACKAGE),--from-go-package "$(FROM_GO_PACKAGE)") \
		--name "$(NEW_NAME)" \
		--slug "$(NEW_SLUG)" \
		--module "$(NEW_MODULE)" \
		$(if $(ENV_PREFIX),--env-prefix "$(ENV_PREFIX)") \
		$(if $(GO_PACKAGE),--go-package "$(GO_PACKAGE)") \
		$(if $(TEMPLATE_SERVICES),--services "$(TEMPLATE_SERVICES)") \
		$(if $(KEEP_GIT),--keep-git) \
		$(if $(INCLUDE_AGENTS),--include-agents) \
		$(if $(INIT_DRY_RUN),--dry-run)
