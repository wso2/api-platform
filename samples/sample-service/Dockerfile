# Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
#
# WSO2 LLC. licenses this file to you under the Apache License,
# Version 2.0 (the "License"); you may not use this file except
# in compliance with the License.
# You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing,
# software distributed under the License is distributed on an
# "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
# KIND, either express or implied. See the License for the
# specific language governing permissions and limitations
# under the License.

# Stage 1: Build
FROM --platform=$BUILDPLATFORM golang:1.26.2-bookworm AS builder
ARG TARGETARCH
ARG VERSION=0.0.1-SNAPSHOT

WORKDIR /build

COPY go.mod ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . ./

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=${TARGETARCH} \
    go build -trimpath \
    -ldflags "-s -w -X main.version=${VERSION}" \
    -o sample-service cmd/server/main.go

# Stage 2: Runtime
FROM gcr.io/distroless/static-debian12

COPY --from=builder /build/sample-service /sample-service

EXPOSE 8080

ENTRYPOINT ["/sample-service"]
