# CI/CD Pipeline

EgoAdmin uses GitHub Actions to implement continuous integration and continuous deployment, covering the full workflow from code checks and testing to building and image publishing.

## Overview

The CI/CD pipeline triggers automatically on every push and PR, running through three sequential/parallel stages: Lint, Test, and Build. The final output is verified Docker images.

| Stage | Trigger | Dependencies |
|-------|---------|--------------|
| lint | push / PR to main | none |
| test | push / PR to main | none |
| build | after lint + test pass | lint, test |

## Core Usage

### Make Commands

Core commands used in the CI pipeline:

```bash
make install         # Install development tools (Buf, Wire, Atlas, protoc plugins, etc.)
make gen             # Generate proto, Go, Wire code
make lint            # Format check and lint
make test            # Run Go tests
make build           # Compile all service binaries
make image.build     # Build Docker images
```

### Simulating CI Locally

Replicate the CI pipeline on your machine:

```bash
# Install tools
make install

# Generate code (must run before build)
make gen

# Lint
make lint

# Run tests
go test -race ./...

# Build frontend (gateway build requires frontend artifacts)
cd web && pnpm install && pnpm run build

# Compile all services
make build

# Build images
make image.build
```

::: warning Code Generation
`make gen` must run before `make build` or `make lint`. It generates proto definitions, Go code, and Wire dependency injection code. Forgetting this step will cause compilation or lint failures.
:::

## Configuration Examples

### Workflow File

The complete workflow definition is at `.github/workflows/ci.yml`:

```yaml
# .github/workflows/ci.yml
name: CI

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

permissions:
  contents: read

jobs:
  lint:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version: "1.26"

      - uses: pnpm/action-setup@v4
        with:
          version: 11

      - uses: actions/setup-node@v4
        with:
          node-version: 22
          cache: pnpm
          cache-dependency-path: web/pnpm-lock.yaml

      - name: Build frontend
        working-directory: web
        run: pnpm install && pnpm run build

      - name: Install protoc
        run: |
          wget -q https://github.com/protocolbuffers/protobuf/releases/download/v3.20.0/protoc-3.20.0-linux-x86_64.zip -O protoc.zip
          sudo unzip protoc.zip -d /usr/local && rm protoc.zip

      - name: Install tools
        run: make install

      - name: Generate code
        run: make gen

      - name: Lint
        run: make lint

  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version: "1.26"

      - uses: pnpm/action-setup@v4
        with:
          version: 11

      - uses: actions/setup-node@v4
        with:
          node-version: 22
          cache: pnpm
          cache-dependency-path: web/pnpm-lock.yaml

      - name: Build frontend
        working-directory: web
        run: pnpm install && pnpm run build

      - name: Install protoc
        run: |
          wget -q https://github.com/protocolbuffers/protobuf/releases/download/v3.20.0/protoc-3.20.0-linux-x86_64.zip -O protoc.zip
          sudo unzip protoc.zip -d /usr/local && rm protoc.zip

      - name: Install tools
        run: make install

      - name: Generate code
        run: make gen

      - name: Test
        run: go test -race ./...

  build:
    runs-on: ubuntu-latest
    needs: [lint, test]
    strategy:
      matrix:
        service: [gateway, user, idgen]
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version: "1.26"

      - uses: pnpm/action-setup@v4
        with:
          version: 11

      - uses: actions/setup-node@v4
        with:
          node-version: 22
          cache: pnpm
          cache-dependency-path: web/pnpm-lock.yaml

      - name: Install protoc
        run: |
          wget -q https://github.com/protocolbuffers/protobuf/releases/download/v3.20.0/protoc-3.20.0-linux-x86_64.zip -O protoc.zip
          sudo unzip protoc.zip -d /usr/local && rm protoc.zip

      - name: Build frontend
        working-directory: web
        run: pnpm install && pnpm run build

      - name: Generate code
        run: make install && make gen

      - name: Build Docker image
        run: |
          docker build \
            --build-arg SERVICE=${{ matrix.service }} \
            -f cmd/${{ matrix.service }}/Dockerfile \
            -t egoadmin-test/${{ matrix.service }}:ci .
```

### Image Publishing Workflow

Production image publishing is defined in `.github/workflows/docker.yml`, triggered by Git tags:

```yaml
# Trigger (example)
on:
  push:
    tags:
      - "v*"
```

Published image tag formats:

| Tag | Description |
|-----|-------------|
| `latest` | Latest main branch build |
| `v1.2.3` | Semantic version tag |
| `sha-abc1234` | Short commit SHA |

## Real-World Examples

### PR Submission Flow

```text
Developer pushes PR
  ├── GitHub Actions triggers CI
  ├── lint job
  │   ├── checkout → setup-go → setup-node/pnpm
  │   ├── pnpm install && pnpm run build     (frontend)
  │   ├── make install → make gen             (code generation)
  │   └── make lint                           (format check)
  ├── test job (parallel with lint)
  │   ├── checkout → setup-go → setup-node/pnpm
  │   ├── make install → make gen
  │   └── go test -race ./...                 (race condition tests)
  └── build job (after lint + test pass)
      ├── matrix: [gateway, user, idgen]      (parallel build for 3 services)
      └── docker build -f cmd/$service/Dockerfile
```

### Branch Protection

The main branch has branch protection rules:

- All PRs must pass CI (lint + test + build).
- At least one reviewer approval required before merging.
- CI and image build trigger automatically on merge to main.

### Release Flow

```text
Maintainer creates Git tag
  ├── git tag v1.2.3 && git push origin v1.2.3
  ├── docker.yml workflow triggers
  ├── matrix builds images for 3 services
  ├── Push to ghcr.io/egoadmin/
  └── Image tags: v1.2.3, latest, sha-xxxxxxx
```

## How It Works

### Pipeline Architecture

```text
         ┌─────────┐     ┌─────────┐
         │  lint    │     │  test   │
         │ (parallel)│    │ (parallel)│
         └────┬────┘     └────┬────┘
              │               │
              └───────┬───────┘
                      │
              ┌───────┴───────┐
              │    build      │
              │  matrix:      │
              │  gateway      │
              │  user         │
              │  idgen        │
              └───────────────┘
```

lint and test run in parallel. build runs after both pass, using a matrix strategy to build all three services in parallel.

### Toolchain Installation

Each job installs the following toolchain:

| Tool | Version | Purpose |
|------|---------|---------|
| Go | 1.26 | Compilation, testing |
| Node.js | 22 | Frontend build |
| pnpm | 11 | Frontend package management |
| protoc | 3.20.0 | Protobuf compilation |
| Buf | Latest | Protobuf lint and generation |
| Wire | Latest | Dependency injection code generation |
| Atlas | Latest | Database migration |

### Frontend Build Dependency

Gateway build requires frontend artifacts (`web/dist/index.html`), so all jobs include a frontend build step:

```bash
cd web && pnpm install && pnpm run build
```

## Common Issues

### Code Generation Failure

If `make gen` fails in CI, check the following:

```bash
# Confirm generated files are up to date locally
make gen
git diff --stat    # Should show no changes
```

::: tip
Generated code (`wire_gen.go`, proto outputs) should be committed to the repository. If CI and local generation results differ, it is usually due to tool version mismatches.
:::

### Test Timeout

CI uses `go test -race ./...` to run all tests. If tests time out:

```bash
# Use a longer timeout locally
go test -race -timeout 30m ./...

# Or run a specific package only
go test -race ./internal/app/gateway/...
```

### Frontend Build Failure

Make sure `web/pnpm-lock.yaml` is committed:

```bash
cd web
pnpm install
git add pnpm-lock.yaml
```

### Image Build Failure

Docker build depends on frontend artifacts and code generation:

```bash
# Complete local image build flow
cd web && pnpm install && pnpm run build
cd ..
make install
make gen
make image.build
```

### Cache Optimization

CI uses the following caching strategies:

| Cached Content | Mechanism | Notes |
|----------------|-----------|-------|
| Go modules | Built into `actions/setup-go` | Reads `go.sum` |
| pnpm store | `actions/setup-node` cache | Specifies `web/pnpm-lock.yaml` |
| Docker layers | BuildKit cache | No explicit cache config |

::: info Improving Build Speed
For further optimization, consider adding Docker layer caching or `actions/cache` for Go build cache in the build job.
:::

## Reference Links

- [GitHub Actions Documentation](https://docs.github.com/en/actions)
- [EgoAdmin CI Workflow](https://github.com/egoadmin/egoadmin/blob/main/.github/workflows/ci.yml)
- [Testing & Deployment](/guide/en-US/testing-deployment)
- [Docker Containerization](/guide/en-US/deployment/docker)
