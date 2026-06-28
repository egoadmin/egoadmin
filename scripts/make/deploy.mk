DEPLOY_COMPOSE_PROJECT ?= $(PROJECT)-deploy
DEPLOY_COMPOSE_FILE ?= docker-compose.yml
DEPLOY_DIR ?= deploy

.PHONY: deploy.prepare
deploy.prepare: ## 生成部署运行所需的本地配置文件
	@mkdir -p $(DEPLOY_DIR)/data/dtm
	@set -a; \
	if [ -f "$(DEPLOY_DIR)/.env" ]; then . "$(DEPLOY_DIR)/.env"; fi; \
	set +a; \
	{ \
		printf '%s\n' 'Store:'; \
		printf '%s\n' '  Driver: mysql'; \
		printf '  Host: %s\n' "$${DTM_STORE_HOST:-mysql-dtm}"; \
		printf '  User: %s\n' "$${DTM_STORE_USER:-$${MYSQL_USER:-egoadmin}}"; \
		printf '  Password: %s\n' "$${DTM_STORE_PASSWORD:-$${MYSQL_PASSWORD:-egoadmin}}"; \
		printf '  Port: %s\n' "$${DTM_STORE_PORT:-3306}"; \
		printf '  Db: %s\n\n' "$${DTM_STORE_DB:-$${MYSQL_DTM_DATABASE:-egoadmin_dtm}}"; \
		printf '%s\n' 'MicroService:'; \
		printf '%s\n' '  Driver: dtm-driver-ego'; \
		printf '  Target: %s\n' "$${DTM_MICRO_SERVICE_TARGET:-etcd://etcd:2379/egoadmin-dtm}"; \
		printf '  EndPoint: %s\n\n' "$${DTM_MICRO_SERVICE_ENDPOINT:-dtm:36790}"; \
		printf 'HttpPort: %s\n' "$${DTM_INTERNAL_HTTP_PORT:-36789}"; \
		printf 'GrpcPort: %s\n' "$${DTM_INTERNAL_GRPC_PORT:-36790}"; \
	} > $(DEPLOY_DIR)/data/dtm/conf.yml

.PHONY: deploy-config
deploy-config: deploy.prepare ## 校验并输出部署 Docker Compose 配置
	@cd $(DEPLOY_DIR) && $(DOCKER_COMPOSE) -p $(DEPLOY_COMPOSE_PROJECT) -f $(DEPLOY_COMPOSE_FILE) config

.PHONY: deploy-up
deploy-up: deploy.prepare ## 启动完整部署环境 Docker Compose
	@cd $(DEPLOY_DIR) && $(DOCKER_COMPOSE) -p $(DEPLOY_COMPOSE_PROJECT) -f $(DEPLOY_COMPOSE_FILE) up -d

.PHONY: deploy-down
deploy-down: ## 关闭完整部署环境 Docker Compose
	@cd $(DEPLOY_DIR) && $(DOCKER_COMPOSE) -p $(DEPLOY_COMPOSE_PROJECT) -f $(DEPLOY_COMPOSE_FILE) down

.PHONY: deploy-reset
deploy-reset: ## 重置完整部署环境和 deploy/data 本地数据目录
	@cd $(DEPLOY_DIR) && $(DOCKER_COMPOSE) -p $(DEPLOY_COMPOSE_PROJECT) -f $(DEPLOY_COMPOSE_FILE) down -v --remove-orphans
	@find $(DEPLOY_DIR)/data -mindepth 1 ! -name .gitkeep -exec rm -rf {} +

.PHONY: deploy-ps
deploy-ps: ## 查看部署 Docker Compose 服务状态
	@cd $(DEPLOY_DIR) && $(DOCKER_COMPOSE) -p $(DEPLOY_COMPOSE_PROJECT) -f $(DEPLOY_COMPOSE_FILE) ps

.PHONY: deploy-logs
deploy-logs: ## 查看部署 Docker Compose 日志。可用 SERVICE=<service> 指定服务
	@cd $(DEPLOY_DIR) && $(DOCKER_COMPOSE) -p $(DEPLOY_COMPOSE_PROJECT) -f $(DEPLOY_COMPOSE_FILE) logs -f $(SERVICE)
