.PHONY: tools.install
tools.install: ## 安装 Go 工具、Atlas CLI 和 Git hooks
	@go mod download
	@go install github.com/evilmartians/lefthook@$(LEFTHOOK_VERSION)
	@go install github.com/muandane/goji@$(GOJI_VERSION)
	@go install github.com/google/wire/cmd/wire@$(WIRE_VERSION)
	@ATLAS_VERSION=$(ATLAS_VERSION) sh -c "$$(curl -fsSL https://atlasgo.sh)" -- --yes --no-install --output "$(GO_TOOL_BIN)/atlas"
	@chmod +x "$(GO_TOOL_BIN)/atlas"
	@go install github.com/arnaud-deprez/gsemver@$(GSEMVER_VERSION)
	@go install github.com/llorllale/go-gitlint/cmd/go-gitlint@$(GO_GITLINT_VERSION)
	@go install github.com/git-chglog/git-chglog/cmd/git-chglog@$(GIT_CHGLOG_VERSION)
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@$(LINT_VERSION)
	@go install google.golang.org/protobuf/cmd/protoc-gen-go@$(PROTOC_GEN_GO)
	@go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@$(PROTOC_GEN_GO_GRPC)
	@go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway@$(PROTOC_GEN_GRPC_GATEWAY)
	@go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2@$(PROTOC_GEN_OPENAPIV2_VERSION)
	@go install github.com/srikrsna/protoc-gen-gotag@$(PROTOC_GEN_GOTAG_VERSION)
	@go install github.com/egoadmin/elib/cmd/protoc-gen-go-errors@$(ELIB_VERSION)
	@go install github.com/egoadmin/elib/cmd/protoc-gen-go-http@$(ELIB_VERSION)
	@go install github.com/egoadmin/elib/cmd/protoc-gen-api-catalog@$(ELIB_VERSION)
# go install github.com/gotomicro/ego/cmd/protoc-gen-go-test@$(EGO_VERSION)
	@go install github.com/bufbuild/buf/cmd/buf@$(BUF_VERSION)
	@go install github.com/bufbuild/buf/cmd/protoc-gen-buf-breaking@$(BUF_VERSION)
	@go install github.com/bufbuild/buf/cmd/protoc-gen-buf-lint@$(BUF_VERSION)
	@go install golang.org/x/lint/golint@$(GOLINT_VERSION)
	@go install golang.org/x/tools/cmd/goimports@$(GOIMPORTS_VERSION)
	@go install mvdan.cc/gofumpt@$(GOFUMPT_VERSION)
	@go install github.com/hnlq715/struct2interface/cmd/struct2interface@$(STRUCT2INTEFACE_VERSION)
	@GOBIN="$$(go env GOBIN)"; \
	if [ -z "$$GOBIN" ]; then GOBIN="$$(go env GOPATH)/bin"; fi; \
	PATH="$$GOBIN:$$(go env GOPATH)/bin:$$PATH" lefthook install -f

.PHONY: tools.check
tools.check: ## 检查项目关键工具是否可用
	@command -v go >/dev/null 2>&1 || (echo "missing go"; exit 1)
	@command -v docker >/dev/null 2>&1 || (echo "missing docker"; exit 1)
	@docker compose version >/dev/null 2>&1 || (echo "missing docker compose"; exit 1)
	@command -v buf >/dev/null 2>&1 || (echo "missing buf, run make install"; exit 1)
	@command -v wire >/dev/null 2>&1 || (echo "missing wire, run make install"; exit 1)
	@command -v atlas >/dev/null 2>&1 || (echo "missing atlas, run make install"; exit 1)
	@command -v pnpm >/dev/null 2>&1 || (echo "missing pnpm"; exit 1)
	@command -v golangci-lint >/dev/null 2>&1 || (echo "missing golangci-lint, run make install"; exit 1)
