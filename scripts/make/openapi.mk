.PHONY: openapi.normalize
openapi.normalize:
	@mv api/openapi/openapi.swagger.json api/openapi/openapi.json
	@mv api/openapi/openapi.swagger.yaml api/openapi/openapi.yaml

.PHONY: openapi.merge
openapi.merge:
	@go run ./tools/openapi-merge
