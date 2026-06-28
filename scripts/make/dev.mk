.PHONY: _require.middleware
_require.middleware:
	@if [ -z "$(MIDDLEWARE)" ]; then \
		echo "usage: make dev.up-one MIDDLEWARE=<mysql-idgen|mysql-gateway|mysql-user|mysql-dtm|redis-gateway|redis-user|minio|image-processor|etcd|dtm|meilisearch|jaeger>"; \
		exit 1; \
	fi
	@cd test && $(DOCKER_COMPOSE) -p $(DEV_COMPOSE_PROJECT) -f docker-compose.yml config --services | grep -qx "$(MIDDLEWARE)" || (echo "unknown middleware $(MIDDLEWARE)"; exit 1)

.PHONY: dev-up
dev-up: ## 启动本地开发中间件 Docker Compose
	@cd test && $(DOCKER_COMPOSE) -p $(DEV_COMPOSE_PROJECT) -f docker-compose.yml up -d

.PHONY: dev-down
dev-down: ## 关闭本地开发中间件 Docker Compose
	@cd test && $(DOCKER_COMPOSE) -p $(DEV_COMPOSE_PROJECT) -f docker-compose.yml down

.PHONY: dev-reset
dev-reset: ## 重置本地开发中间件 Docker Compose 和本地数据目录
	@cd test && $(DOCKER_COMPOSE) -p $(DEV_COMPOSE_PROJECT) -f docker-compose.yml down -v --remove-orphans
	@rm -rf test/data/idgen test/data/gateway test/data/user test/data/dtm test/data/minio test/data/etcd test/data/meilisearch

.PHONY: dev.up-one
dev.up-one: _require.middleware ## 启动指定本地中间件。示例：make dev.up-one MIDDLEWARE=mysql-user
	@cd test && $(DOCKER_COMPOSE) -p $(DEV_COMPOSE_PROJECT) -f docker-compose.yml up -d $(MIDDLEWARE)

.PHONY: dev.down-one
dev.down-one: _require.middleware ## 关闭指定本地中间件。示例：make dev.down-one MIDDLEWARE=mysql-user
	@cd test && $(DOCKER_COMPOSE) -p $(DEV_COMPOSE_PROJECT) -f docker-compose.yml stop $(MIDDLEWARE)

.PHONY: dev.reset-one
dev.reset-one: _require.middleware ## 重置指定本地中间件。示例：make dev.reset-one MIDDLEWARE=mysql-user
	@cd test && $(DOCKER_COMPOSE) -p $(DEV_COMPOSE_PROJECT) -f docker-compose.yml stop $(MIDDLEWARE)
	@cd test && $(DOCKER_COMPOSE) -p $(DEV_COMPOSE_PROJECT) -f docker-compose.yml rm -fsv $(MIDDLEWARE)
	@case "$(MIDDLEWARE)" in \
		mysql-gateway) rm -rf test/data/gateway/mysql ;; \
		mysql-user) rm -rf test/data/user/mysql ;; \
		mysql-idgen) rm -rf test/data/idgen/mysql ;; \
		mysql-dtm) rm -rf test/data/dtm/mysql ;; \
		redis-gateway) rm -rf test/data/gateway/redis ;; \
		redis-user) rm -rf test/data/user/redis ;; \
		*) rm -rf "test/data/$(MIDDLEWARE)" ;; \
	esac
