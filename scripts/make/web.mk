.PHONY: web.install
web.install: ## 安装同仓前端依赖
	@cd web && pnpm install

.PHONY: web.build
web.build: ## 构建前端并生成权限契约
ifeq ($(SKIP_WEB_BUILD),1)
	@test -f web/dist/index.html || (echo "web/dist/index.html missing, run make web.build first"; exit 1)
else
	@$(MAKE_CMD) web.install
	@cd web && pnpm run build
endif

.PHONY: web.contract
web.contract: ## 只生成前端权限契约
	@cd web && pnpm run contract:gen

.PHONY: web.type-check
web.type-check: ## 只运行前端 TypeScript 类型检查
	@cd web && pnpm run type-check
