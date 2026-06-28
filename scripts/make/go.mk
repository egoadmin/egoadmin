.PHONY: go.lint
go.lint:
	@$(MAKE_CMD) go.check

GO_SERVICE ?= $(if $(SERVICE),$(SERVICE),gateway)
GO_CMD ?= ./cmd/$(GO_SERVICE)
GO_CONFIG ?= ./configs/$(GO_SERVICE)/config.toml
GO_BIN ?= $(PROJECT)

.PHONY: go.gen
go.gen:
	@go generate ./...

.PHONY: go.run
go.run:
	@go run -race $(GO_CMD) --config=$(GO_CONFIG)

.PHONY: go.build
go.build:
	@CGO_ENABLED=0 go build -o $(GO_BIN) \
	-ldflags "-extldflags '-static' \
	-X github.com/gotomicro/ego/core/eapp.appName=$(APPNAME)-$(GO_SERVICE) \
	-X github.com/gotomicro/ego/core/eapp.buildVersion=$(BUILD_VERSION) \
	-X github.com/gotomicro/ego/core/eapp.buildAppVersion=$(BUILD_VERSION) \
	-X github.com/gotomicro/ego/core/eapp.buildStatus=$(BUILD_TAG)-$(LAST_COMMIT_HASH) \
	-X github.com/gotomicro/ego/core/eapp.buildTag=$(BUILD_TAG) \
	-X github.com/gotomicro/ego/core/eapp.buildUser=$(BUILD_USER) \
	-X github.com/gotomicro/ego/core/eapp.buildHost=$(BUILD_HOST) \
	-X github.com/gotomicro/ego/core/eapp.buildTime=$(BUILD_TIME) \
	" \
	$(GO_CMD)

.PHONY: go.build.alpine
go.build.alpine:
	@CGO_ENABLED=0 go build -tags netgo -o $(GO_BIN) \
	-ldflags "-extldflags '-static' \
	-X github.com/gotomicro/ego/core/eapp.appName=$(APPNAME)-$(GO_SERVICE) \
	-X github.com/gotomicro/ego/core/eapp.buildVersion=$(BUILD_VERSION) \
	-X github.com/gotomicro/ego/core/eapp.buildAppVersion=$(BUILD_VERSION) \
	-X github.com/gotomicro/ego/core/eapp.buildStatus=$(BUILD_TAG)-$(LAST_COMMIT_HASH) \
	-X github.com/gotomicro/ego/core/eapp.buildTag=$(BUILD_TAG) \
	-X github.com/gotomicro/ego/core/eapp.buildUser=$(BUILD_USER) \
	-X github.com/gotomicro/ego/core/eapp.buildHost=$(BUILD_HOST) \
	-X github.com/gotomicro/ego/core/eapp.buildTime=$(BUILD_TIME) \
	" \
	$(GO_CMD)

.PHONY: go.build.alpine-arm64
go.build.alpine-arm64:
	@CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -tags netgo -o $(GO_BIN).arm64 \
	-ldflags "-extldflags '-static' \
	-X github.com/gotomicro/ego/core/eapp.appName=$(APPNAME)-$(GO_SERVICE) \
	-X github.com/gotomicro/ego/core/eapp.buildVersion=$(BUILD_VERSION) \
	-X github.com/gotomicro/ego/core/eapp.buildAppVersion=$(BUILD_VERSION) \
	-X github.com/gotomicro/ego/core/eapp.buildStatus=$(BUILD_TAG)-$(LAST_COMMIT_HASH) \
	-X github.com/gotomicro/ego/core/eapp.buildTag=$(BUILD_TAG)-$(LAST_COMMIT_HASH) \
	-X github.com/gotomicro/ego/core/eapp.buildUser=$(BUILD_USER) \
	-X github.com/gotomicro/ego/core/eapp.buildHost=$(BUILD_HOST) \
	-X github.com/gotomicro/ego/core/eapp.buildTime=$(BUILD_TIME) \
	" \
	$(GO_CMD)

.PHONY: go.test
go.test:
	@go test -race ./...

.PHONY: go.mod-tidy
go.mod-tidy:
	@go mod tidy

.PHONY: go.check
go.check:
	@golangci-lint run ./...

.PHONY: go.vendor
go.vendor:
	@go mod vendor
	@tar -czvf ${VENDOR_FILE} vendor
