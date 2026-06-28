.PHONY: gen.proto
gen.proto: ## 生成外部 proto、OpenAPI 和 API catalog
	@$(MAKE_CMD) buf.gen

.PHONY: gen.proto.internal
gen.proto.internal: ## 生成内部 gRPC proto Go 代码
	@buf lint api/proto-internal
	@buf generate api/proto-internal --template buf.gen.internal.yaml

.PHONY: gen.go
gen.go: ## 执行 go generate
	@$(MAKE_CMD) GOFLAGS=-mod=mod go.gen

.PHONY: gen.wire
gen.wire: ## 生成 Wire 代码。可选 SERVICE=<service> 只生成单服务
	@if [ -n "$(SERVICE)" ]; then \
		$(MAKE_CMD) wire SERVICE="$(SERVICE)"; \
	else \
		GOFLAGS=-mod=mod wire gen ./...; \
	fi
