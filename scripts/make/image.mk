.PHONY: image.context
image.context: _require.service ## 准备指定服务 Docker 构建上下文。示例：make image.context SERVICE=user
	@if [ "$(SERVICE)" = "gateway" ]; then \
		$(MAKE_CMD) _ensure.web-dist SKIP_WEB_BUILD="$(SKIP_WEB_BUILD)"; \
	fi

.PHONY: _image.build-one
_image.build-one: image.context
	@image="$(or $(IMAGE),$(DOCKER_REGISTRY)/$(PROJECT)-$(SERVICE):$(DOCKER_TAG))"; \
	docker build \
		--build-arg SERVICE=$(SERVICE) \
		--build-arg BUILD_TAG=$(BUILD_TAG) \
		--build-arg BUILD_VERSION=$(BUILD_VERSION) \
		-f cmd/$(SERVICE)/Dockerfile \
		-t "$$image" .

.PHONY: image.build
image.build: ## 构建一个或多个微服务 Docker 镜像。默认 SERVICES="idgen user gateway"
	@set -e; \
	if [ -n "$(SERVICE)" ]; then \
		$(MAKE_CMD) _image.build-one SERVICE="$(SERVICE)" $(if $(IMAGE),IMAGE="$(IMAGE)"); \
	else \
		for service in $(SERVICES); do \
			$(MAKE_CMD) _image.build-one SERVICE="$$service"; \
		done; \
	fi
