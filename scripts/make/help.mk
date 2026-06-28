.PHONY: help
help: ## 显示可用 make 命令
	@printf "Usage:\n  make <target> [OPTIONS]\n\nTargets:\n"
	@awk 'BEGIN {FS = ":.*## "}; /^[A-Za-z0-9_.-]+:.*## / {printf "  %-24s %s\n", $$1, $$2}' Makefile scripts/make/*.mk | sort
	@printf "\nCommon options:\n"
	@printf "  SERVICE=<name>          单个微服务名，例如 idgen、gateway、user\n"
	@printf "  SERVICES=\"idgen user gateway\"  多服务列表，默认 $(SERVICES)\n"
	@printf "  MIDDLEWARE=<name>       本地中间件 compose service 名\n"
	@printf "  DEPLOY_COMPOSE_PROJECT=<name> 部署 compose project 名，默认 $(DEPLOY_COMPOSE_PROJECT)\n"
	@printf "  DOCKER_TAG=<tag>        微服务镜像 tag，默认 $(DOCKER_TAG)\n"
	@printf "  ATLAS_URL=<url>         Atlas migrate apply 数据库连接\n"
	@printf "  NAME=<change_name>      Atlas migration 名称\n"
	@printf "  SKIP_WEB_BUILD=1        跳过 web.build，要求 web/dist 已存在\n"
