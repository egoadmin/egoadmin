BUILD_DIR ?=

include $(BUILD_DIR)scripts/make/vars.mk

.DEFAULT_GOAL := help

include $(BUILD_DIR)scripts/make/help.mk
include $(BUILD_DIR)scripts/make/git.mk
include $(BUILD_DIR)scripts/make/proto.mk
include $(BUILD_DIR)scripts/make/buf.mk
include $(BUILD_DIR)scripts/make/db.mk
include $(BUILD_DIR)scripts/make/go.mk
include $(BUILD_DIR)scripts/make/fmt.mk
include $(BUILD_DIR)scripts/make/tools.mk
include $(BUILD_DIR)scripts/make/openapi.mk
include $(BUILD_DIR)scripts/make/web.mk
include $(BUILD_DIR)scripts/make/service.mk
include $(BUILD_DIR)scripts/make/migrate.mk
include $(BUILD_DIR)scripts/make/dev.mk
include $(BUILD_DIR)scripts/make/deploy.mk
include $(BUILD_DIR)scripts/make/image.mk
include $(BUILD_DIR)scripts/make/test.mk

.PHONY: all
all: gen lint test image.build ## 生成、检查、测试并构建默认服务镜像

.PHONY: install
install: tools.install ## 安装项目开发工具

.PHONY: lint
lint: ## 格式化并检查 Go、proto 代码
	@$(MAKE_CMD) fmt.main
	@$(MAKE_CMD) buf.lint
	@$(MAKE_CMD) go.lint

.PHONY: gen
gen: gen.proto gen.proto.internal gen.go gen.wire ## 生成 proto、internal proto、Go 和 Wire 代码

.PHONY: commit
commit: ## 整理依赖、生成 .gitkeep、运行 lint 并提交
	@go mod tidy
	@$(MAKE_CMD) git.keep
	@$(MAKE_CMD) lint git.commit

.PHONY: tag
tag: ## 生成 Git tag 和 changelog
	@$(MAKE_CMD) git.tag
	@$(MAKE_CMD) git.changelog

.PHONY: vendor
vendor:
	$(eval BUILD_HASH := $(shell git rev-parse --short HEAD))
	$(eval VENDOR_FILE := $(shell echo ${APPNAME}_vendor_${BUILD_HASH}.tar.gz))
	@$(MAKE_CMD) VENDOR_FILE=${VENDOR_FILE} go.vendor
	@$(SAVE_VENDOR)

ifeq ($(MAKE_ENV),ci)
SAVE_VENDOR = @mv ${VENDOR_FILE} ${GOVENDOR_PATH}
else
SAVE_VENDOR = @echo "Running in normal environment"
endif
