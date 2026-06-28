.PHONY: _require.name
_require.name:
	@if [ -z "$(or $(NAME),$(MIGRATION_NAME))" ]; then \
		echo "usage: make migrate.new SERVICE=<idgen|gateway|user> NAME=<change_name>"; \
		exit 1; \
	fi

.PHONY: _require.atlas-url
_require.atlas-url:
	@if [ -z "$(ATLAS_URL)" ]; then \
		echo "usage: make migrate.apply SERVICE=<idgen|gateway|user> ATLAS_URL=<database-url>"; \
		exit 1; \
	fi

MIGRATION_DIALECT = $(or $(DIALECT),mysql)
MIGRATION_DIR = $(if $(filter mysql,$(MIGRATION_DIALECT)),atlas/migrations/$(SERVICE),atlas/migrations/$(MIGRATION_DIALECT)/$(SERVICE))
SCHEMA_DIR = atlas/schema/$(MIGRATION_DIALECT)

.PHONY: _require.migration-service
_require.migration-service: _require.service
	@test -d "atlas/migrations/$(SERVICE)" || (echo "missing atlas/migrations/$(SERVICE)"; exit 1)

.PHONY: _require.migration-dir
_require.migration-dir: _require.service
	@if [ "$(MIGRATION_DIALECT)" = "mysql" ]; then \
		test -d "$(MIGRATION_DIR)" || (echo "missing $(MIGRATION_DIR)"; exit 1); \
	else \
		mkdir -p "$(MIGRATION_DIR)"; \
	fi

.PHONY: _require.migration-dir-existing
_require.migration-dir-existing: _require.service
	@test -d "$(MIGRATION_DIR)" || (echo "missing $(MIGRATION_DIR)"; exit 1)

.PHONY: _require.schema-dialect
_require.schema-dialect:
	@case "$(MIGRATION_DIALECT)" in \
		mysql|postgres|sqlite|sqlserver) ;; \
		*) echo "unsupported DIALECT=$(DIALECT), available: mysql, postgres, sqlite, sqlserver"; exit 1 ;; \
	esac

.PHONY: migrate.new
migrate.new: _require.schema-dialect _require.migration-dir _require.name ## 生成 Atlas migration。示例：make migrate.new SERVICE=user NAME=add_field [DIALECT=mysql]
	$(eval MIGRATION_NAME := $(or $(MIGRATION_NAME),$(NAME)))
	@go run ./tools/atlasloader --service "$(SERVICE)" --dialect "$(MIGRATION_DIALECT)" >/dev/null
	@atlas migrate diff "$(MIGRATION_NAME)" --env gorm --var service="$(SERVICE)" --var dialect="$(MIGRATION_DIALECT)" $(if $(ATLAS_DEV_URL),--var dev_url="$(ATLAS_DEV_URL)") --config file://atlas/atlas.hcl

.PHONY: migrate.hash
migrate.hash: _require.schema-dialect _require.migration-dir-existing ## 重新计算指定服务 Atlas migration hash
	@atlas migrate hash --dir "file://$(MIGRATION_DIR)"

.PHONY: migrate.apply
migrate.apply: _require.schema-dialect _require.migration-dir-existing _require.atlas-url ## 应用指定服务 Atlas migration。需要 ATLAS_URL=<database-url>
	@ATLAS_URL="$(ATLAS_URL)" atlas migrate apply --env local --var service="$(SERVICE)" --var dialect="$(MIGRATION_DIALECT)" $(if $(ATLAS_DEV_URL),--var dev_url="$(ATLAS_DEV_URL)") --config file://atlas/atlas.hcl

.PHONY: migrate.validate
migrate.validate: _require.schema-dialect _require.migration-dir-existing ## 校验指定服务 Atlas migration 目录
	@atlas migrate validate --dir "file://$(MIGRATION_DIR)"

.PHONY: migrate.schema
migrate.schema: _require.schema-dialect _require.service ## 生成审计用 Atlas schema HCL。示例：make migrate.schema SERVICE=user [DIALECT=mysql]
	@mkdir -p "$(SCHEMA_DIR)"
	@tmp="$$(mktemp "$(SCHEMA_DIR)/$(SERVICE).hcl.XXXXXX")"; \
	if atlas schema inspect --env schema --url env://src --var service="$(SERVICE)" --var dialect="$(MIGRATION_DIALECT)" $(if $(ATLAS_DEV_URL),--var dev_url="$(ATLAS_DEV_URL)") --config file://atlas/atlas.hcl > "$$tmp"; then \
		mv "$$tmp" "$(SCHEMA_DIR)/$(SERVICE).hcl"; \
	else \
		rm -f "$$tmp"; \
		exit 1; \
	fi
