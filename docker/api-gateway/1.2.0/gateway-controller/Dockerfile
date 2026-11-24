# syntax=docker/dockerfile:1
# Build from a local binary ZIP or extracted directory; see BINARY-IMAGES.md.
ARG BASE_IMAGE=ubuntu:24.04
# BINARY_PATH is a ZIP file or extracted directory inside the build context.
# It may contain the combined product package or only this component.
FROM --platform=$BUILDPLATFORM ubuntu:24.04 AS binary
RUN apt-get update && \
    apt-get install -y --no-install-recommends unzip && \
    rm -rf /var/lib/apt/lists/*
ARG BINARY_PATH=.
COPY ${BINARY_PATH} /tmp/input/
RUN set -eu; \
    root=/tmp/input; \
    set -- "$root"/*.zip; \
    if [ -f "$1" ]; then \
        if [ "$#" -ne 1 ]; then \
            echo "Error: BINARY_PATH must identify one ZIP or an extracted binary directory" >&2; \
            exit 1; \
        fi; \
        mkdir /tmp/unpacked; \
        unzip -q "$1" -d /tmp/unpacked; \
        root=/tmp/unpacked; \
    fi; \
    if [ -d "$root/gateway-controller" ]; then \
        component="$root/gateway-controller"; \
    elif [ -f "$root/controller-amd64" ] || [ -f "$root/controller-arm64" ]; then \
        component="$root"; \
    else \
        set -- "$root"/api-gateway.*/gateway-controller; \
        if [ "$#" -ne 1 ] || [ ! -d "$1" ]; then \
            echo "Error: BINARY_PATH does not contain a gateway-controller binary package" >&2; \
            exit 1; \
        fi; \
        component="$1"; \
    fi; \
    mkdir /binary; \
    cp -a "$component"/. /binary/; \
    rm -rf /tmp/input /tmp/unpacked

FROM ${BASE_IMAGE} AS production
ARG TARGETARCH
ARG PRODUCT_VERSION
ARG VERSION=${PRODUCT_VERSION:-unknown}
ARG REVISION=unknown

RUN case "$TARGETARCH" in amd64|arm64) ;; *) echo "Unsupported architecture: $TARGETARCH" >&2; exit 1 ;; esac && \
    apt-get update && \
    apt-get upgrade -y && \
    apt-get install -y --no-install-recommends bash coreutils ca-certificates libsqlite3-0 && \
    rm -rf /var/lib/apt/lists/* && \
    groupadd --system --gid 10001 wso2 && \
    useradd --system --uid 10001 --gid wso2 --home-dir /home/wso2 --shell /usr/sbin/nologin wso2 && \
    mkdir -p /app/data /coverage /home/wso2 && \
    chown -R 10001:0 /app/data /coverage /home/wso2 && \
    chmod -R g=u /app/data /coverage /home/wso2

WORKDIR /app
COPY --from=binary --chmod=0755 /binary/controller-${TARGETARCH} /app/controller
COPY --from=binary /binary/default-policies/ /app/default-policies/
COPY --from=binary /binary/default-llm-provider-templates/ /app/default-llm-provider-templates/
COPY --from=binary /binary/lua/ /app/lua/
COPY --from=binary /binary/build-manifest.yaml /binary/LICENSE /app/
COPY --from=binary --chmod=0755 /binary/health-check.sh /usr/local/bin/health-check.sh

ENV HOME=/home/wso2
USER 10001:0
EXPOSE 9090 9092 18000 18001
LABEL org.opencontainers.image.title="WSO2 API Platform Gateway Controller" \
      org.opencontainers.image.vendor="WSO2" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${REVISION}" \
      org.opencontainers.image.licenses="Apache-2.0"
HEALTHCHECK --interval=30s --timeout=5s --start-period=30s --retries=3 \
    CMD ["/usr/local/bin/health-check.sh"]
ENTRYPOINT ["/app/controller"]
