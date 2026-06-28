#!/usr/bin/env bash
set -euo pipefail

service="${1:-}"
if [[ -z "$service" ]]; then
  echo "usage: scripts/check_arch.sh <service>" >&2
  exit 1
fi

module="$(go list -m)"

platform_imports="$(
  go list -f '{{.ImportPath}} {{range .Imports}}{{.}} {{end}}' ./internal/platform/...
)"
if grep -q "${module}/internal/app/" <<<"$platform_imports"; then
  echo "internal/platform must not import internal/app packages" >&2
  grep "${module}/internal/app/" <<<"$platform_imports" >&2
  exit 1
fi

service_imports="$(
  go list -f '{{.ImportPath}} {{range .Imports}}{{.}} {{end}}' "./internal/app/${service}/..."
)"
other_services="$(
  find internal/app -mindepth 1 -maxdepth 1 -type d -printf '%f\n' | grep -v "^${service}$" || true
)"
for other in $other_services; do
  if grep -Eq "${module}/internal/app/${other}/(internal/store|adapter|domain)(/| |$)" <<<"$service_imports"; then
    echo "internal/app/${service} must not import ${other} store/adapter/domain packages" >&2
    grep -E "${module}/internal/app/${other}/(internal/store|adapter|domain)(/| |$)" <<<"$service_imports" >&2
    exit 1
  fi
done

if [[ -d "internal/app/${service}/domain" ]]; then
  domain_imports="$(go list -f '{{.ImportPath}} {{range .Imports}}{{.}} {{end}}' "./internal/app/${service}/domain/..." 2>/dev/null || true)"
  if grep -Eq "(${module}/api/gen/|gorm.io/|github.com/gotomicro/ego|github.com/gin-gonic/gin|github.com/redis/|github.com/minio/|github.com/casbin/)" <<<"$domain_imports"; then
    echo "internal/app/${service}/domain must stay free of proto, GORM, EGO/Gin, Redis, MinIO, and Casbin infrastructure imports" >&2
    grep -E "(${module}/api/gen/|gorm.io/|github.com/gotomicro/ego|github.com/gin-gonic/gin|github.com/redis/|github.com/minio/|github.com/casbin/)" <<<"$domain_imports" >&2
    exit 1
  fi
fi

if [[ -d "internal/app/${service}/application" ]]; then
  application_imports="$(
    go list -f '{{.ImportPath}} {{range .Imports}}{{.}} {{end}}' "./internal/app/${service}/application/..." 2>/dev/null || true
  )"
  if grep -Eq "(${module}/api/gen/|gorm.io/|github.com/gotomicro/ego|github.com/gin-gonic/gin)" <<<"$application_imports"; then
    echo "internal/app/${service}/application must stay free of proto, GORM, EGO/Gin infrastructure imports" >&2
    grep -E "(${module}/api/gen/|gorm.io/|github.com/gotomicro/ego|github.com/gin-gonic/gin)" <<<"$application_imports" >&2
    exit 1
  fi
fi

if [[ -d "internal/app/${service}/controller" ]]; then
  controller_imports="$(
    go list -f '{{.ImportPath}} {{range .Imports}}{{.}} {{end}}' "./internal/app/${service}/controller/..." 2>/dev/null || true
  )"
  if grep -Eq "${module}/internal/app/${service}/adapter/persistence/mysql(/| |$)" <<<"$controller_imports"; then
    echo "internal/app/${service}/controller must not import persistence adapters directly" >&2
    grep -E "${module}/internal/app/${service}/adapter/persistence/mysql(/| |$)" <<<"$controller_imports" >&2
    exit 1
  fi
fi
