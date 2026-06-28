# ---- builder ----
FROM golang:1.26.2-alpine AS builder

ARG SERVICE=idgen
ARG BUILD_TAG=0.0.1
ARG BUILD_VERSION=0.0.1

ARG GOPROXY=https://goproxy.cn,direct

RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.ustc.edu.cn/g' /etc/apk/repositories \
    && apk add --no-cache gcc musl-dev git

ENV GOPROXY=${GOPROXY}
ENV CGO_ENABLED=0

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN BUILD_TIME=$(date +%Y-%m-%d_%H:%M:%S) && \
    LAST_COMMIT_HASH=$(git rev-parse HEAD 2>/dev/null || true) && \
    go build -tags netgo -o egoadmin \
    -ldflags "-extldflags '-static' \
    -X github.com/gotomicro/ego/core/eapp.appName=egoadmin-${SERVICE} \
    -X github.com/gotomicro/ego/core/eapp.buildVersion=${BUILD_VERSION} \
    -X github.com/gotomicro/ego/core/eapp.buildAppVersion=${BUILD_VERSION} \
    -X github.com/gotomicro/ego/core/eapp.buildStatus=${BUILD_TAG}-${LAST_COMMIT_HASH} \
    -X github.com/gotomicro/ego/core/eapp.buildTag=${BUILD_TAG} \
    -X github.com/gotomicro/ego/core/eapp.buildTime=${BUILD_TIME}" \
    ./cmd/idgen

# ---- runtime ----
FROM alpine:3.18

RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.ustc.edu.cn/g' /etc/apk/repositories \
    && apk add --no-cache \
    ca-certificates \
    iputils \
    tzdata \
    && rm -f /etc/localtime \
    && ln -sv /usr/share/zoneinfo/Asia/Shanghai /etc/localtime \
    && echo "Asia/Shanghai" > /etc/timezone

ENV LANG=en_US.UTF-8 \
    LC_ALL=en_US.UTF-8 \
    LANGUAGE=en_US.UTF-8

WORKDIR /app/

ENV ATLAS_SERVICE=idgen
ENV EGO_NAME=egoadmin-idgen

COPY --from=arigaio/atlas:1.2.2-community-alpine /atlas /usr/local/bin/atlas
COPY atlas ./atlas
COPY --from=builder /build/egoadmin ./app

RUN chmod +x ./app ./atlas/apply.sh && \
    printf '%s\n' \
    '#!/bin/sh' \
    'set -e' \
    'if [ "${EGOADMIN_ATLAS_MIGRATE:-true}" = "true" ]; then' \
    '  cd /app && ./atlas/apply.sh' \
    '  export EGOADMIN_ATLAS_MIGRATED=true' \
    'fi' \
    'exec ./app "$@"' \
    > /entrypoint.sh && \
    chmod +x /entrypoint.sh

ENTRYPOINT ["/entrypoint.sh"]
