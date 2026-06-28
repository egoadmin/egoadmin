.PHONY: buf.gen
buf.gen:
	@$(MAKE_CMD) buf.lint
	@buf generate api/proto && cd api/gen/go && buf generate --template=../../../buf.gen.tag.yaml ../../proto && cd ../../../
	@$(MAKE_CMD) openapi.normalize
	@$(MAKE_CMD) openapi.merge

.PHONY: buf.lint
buf.lint:
	@buf lint
	@buf build -o - | buf breaking --against -
# 接口不兼容检测 buf breaking --against ../../.git#branch=main,subdir=start/petapis

.PHONY: buf.gen-test
buf.gen-test:
	@cp buf.gentest.tpl && buf.gen.yaml && sed -i 's!@PATH!$(OUT)!g' buf.gen.yaml && buf generate && cp buf.gen.tpl buf.gen.yaml
