# See here for image contents: https://github.com/microsoft/vscode-dev-containers/tree/v0.231.6/containers/go/.devcontainer/base.Dockerfile

# [Choice] Go version (use -bullseye variants on local arm64/Apple Silicon): 1, 1.16, 1.17, 1-bullseye, 1.16-bullseye, 1.17-bullseye, 1-buster, 1.16-buster, 1.17-buster
ARG VARIANT="1.22-bullseye"
FROM mcr.microsoft.com/devcontainers/go:${VARIANT}

# [Optional] 设置开源镜像的源
RUN sed -i 's/deb.debian.org/mirrors.ustc.edu.cn/g' /etc/apt/sources.list
RUN apt update && apt install -y mariadb-client wget unzip

# [Optional] 安装依赖
# RUN apt update && apt install clang-format -y

# [Timezone] 设置时区
RUN rm -f /etc/localtime && ln -sv /usr/share/zoneinfo/Asia/Shanghai /etc/localtime  && echo "Asia/Shanghai" > /etc/timezone

# [Optional] Install go
ENV GO111MODULE=on
ENV GOPROXY=https://goproxy.cn,direct

# [Optional] Install go grpc tools
ENV PROTOC_VERSION=3.20.0
RUN wget -q https://github.com/protocolbuffers/protobuf/releases/download/v${PROTOC_VERSION}/protoc-${PROTOC_VERSION}-linux-x86_64.zip -O protoc.zip \
    && unzip protoc.zip -d /usr/local \
    && rm protoc.zip

# RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@latest && \
# 	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest && \
# 	go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway@latest && \
# 	go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2@latest && \
# 	go install github.com/srikrsna/protoc-gen-gotag@latest && \
#     go install github.com/bufbuild/buf/cmd/buf@latest && \
# 	go install github.com/bufbuild/buf/cmd/protoc-gen-buf-breaking@latest && \
# 	go install github.com/bufbuild/buf/cmd/protoc-gen-buf-lint@latest

# ego tools
# RUN go install github.com/gotomicro/ego/cmd/protoc-gen-go-errors@v1.0.0 && \
# 	go install github.com/gotomicro/ego/cmd/protoc-gen-go-http@v1.0.0 && \
# 	go install github.com/gotomicro/ego/cmd/protoc-gen-go-test@v1.0.0

# [Choice] Node.js version: none, lts/*, 16, 14, 12, 10
# ARG NODE_VERSION="none"
# RUN if [ "${NODE_VERSION}" != "none" ]; then su vscode -c "umask 0002 && . /usr/local/share/nvm/nvm.sh && nvm install ${NODE_VERSION} 2>&1"; fi

# [Optional] Uncomment this section to install additional OS packages.
# RUN apt-get update && export DEBIAN_FRONTEND=noninteractive \
#     && apt-get -y install --no-install-recommends <your-package-list-here>

# [Optional] Uncomment the next lines to use go get to install anything else you need
# USER vscode
# RUN go get -x <your-dependency-or-tool>

# [Optional] Uncomment this line to install global node packages.
# RUN su vscode -c "source /usr/local/share/nvm/nvm.sh && npm install -g <your-package-here>" 2>&1
