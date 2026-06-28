.PHONY: fmt.main
fmt.main:
	@$(MAKE_CMD) fmt.go fmt.gofumpt

.PHONY: fmt.go
fmt.go:
	@go fmt ./...

.PHONY: fmt.gofumpt
fmt.gofumpt:
	@git ls-files '*.go' \
		':!*.pb.go' \
		':!.agents/**' \
		':!.claude/**' \
		':!opensrc/**' \
		':!web/**' | xargs -r gofumpt -l -w
