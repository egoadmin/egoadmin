.PHONY: test-init
test-init: ## 准备测试所需数据库数据
	@cd test && $(DOCKER_COMPOSE) -p $(DEV_COMPOSE_PROJECT) -f docker-compose.yml exec -T mysql-gateway mysql -u$(MYSQL_ROOT_USER) -p$(MYSQL_ROOT_PASSWORD) egoadmin_gateway < data/db.sql

.PHONY: test
test: _ensure.web-dist ## 运行普通 Go 测试
	@$(MAKE_CMD) go.test

.PHONY: e2e.check
e2e.check: ## 检查 e2e 所需工具
	@command -v go >/dev/null 2>&1 || (echo "missing go"; exit 1)
	@command -v docker >/dev/null 2>&1 || (echo "missing docker"; exit 1)
	@docker compose version >/dev/null 2>&1 || (echo "missing docker compose"; exit 1)
	@command -v atlas >/dev/null 2>&1 || (echo "missing atlas"; exit 1)

.PHONY: e2e
e2e: e2e.check ## 运行 gateway 端到端测试
	@go test -race -tags=e2e ./test/e2e/gateway -count=1 -timeout=$(E2E_TIMEOUT)

.PHONY: cover
cover: ## 生成 Go 覆盖率报告
	@go test -race -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out
