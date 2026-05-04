FROM --platform=$BUILDPLATFORM golang:1.26.2-alpine AS builder

ARG TARGETOS=linux
ARG TARGETARCH

WORKDIR /src

COPY go.mod ./
COPY main.go ./

RUN --mount=type=cache,target=/root/.cache/go-build \
    target_os="${TARGETOS:-$(go env GOOS)}" && \
    target_arch="${TARGETARCH:-$(go env GOARCH)}" && \
    CGO_ENABLED=0 GOOS="${target_os}" GOARCH="${target_arch}" \
    go build -trimpath -buildvcs=false -ldflags="-s -w" -o /out/webhook-listener .

FROM scratch

COPY --from=builder /out/webhook-listener /webhook-listener

USER 65532:65532

EXPOSE 8090

ENTRYPOINT ["/webhook-listener"]
