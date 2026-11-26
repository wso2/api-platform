# syntax=docker/dockerfile:1
# Build from a local binary ZIP or extracted directory; see BINARY-IMAGES.md.
ARG BASE_IMAGE=ubuntu:22.04

# BINARY_PATH is a ZIP file or extracted directory inside the build context.
# It may contain the combined product package or only this component.
FROM --platform=$BUILDPLATFORM ubuntu:22.04 AS binary
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
    if [ -d "$root/gateway-runtime" ]; then \
        component="$root/gateway-runtime"; \
    elif [ -f "$root/policy-engine-amd64" ] || [ -f "$root/policy-engine-arm64" ]; then \
        component="$root"; \
    else \
        set -- "$root"/api-gateway-*/gateway-runtime; \
        if [ "$#" -ne 1 ] || [ ! -d "$1" ]; then \
            echo "Error: BINARY_PATH does not contain a gateway-runtime binary package" >&2; \
            exit 1; \
        fi; \
        component="$1"; \
    fi; \
    mkdir /binary; \
    cp -a "$component"/. /binary/; \
    rm -rf /tmp/input /tmp/unpacked

# Build Python packages for the target platform, including native extensions.
# Build tools, pip, git, and venv are excluded from the final image.
FROM ${BASE_IMAGE} AS python-deps
RUN apt-get update && \
    apt-get upgrade -y && \
    apt-get install -y --no-install-recommends \
        ca-certificates git python3-venv python3-dev build-essential && \
    rm -rf /var/lib/apt/lists/*
COPY --from=binary /binary/python-requirements.txt /tmp/requirements.txt
RUN python3 -m venv /opt/venv && \
    /opt/venv/bin/pip install --no-cache-dir --target /app/python-libs -r /tmp/requirements.txt && \
    /opt/venv/bin/pip freeze --path /app/python-libs > /app/python-libs/requirements-lock.txt

FROM ${BASE_IMAGE} AS production
ARG TARGETARCH
ARG PRODUCT_VERSION
ARG VERSION=${PRODUCT_VERSION:-unknown}
ARG REVISION=unknown
RUN case "$TARGETARCH" in amd64|arm64) ;; *) echo "Unsupported architecture: $TARGETARCH" >&2; exit 1 ;; esac && \
    apt-get update && \
    apt-get upgrade -y && \
    apt-get install -y --no-install-recommends \
        bash coreutils ca-certificates gettext-base tini python3 libstdc++6 && \
    rm -rf /var/lib/apt/lists/* && \
    groupadd --system --gid 10001 wso2 && \
    useradd --system --uid 10001 --gid wso2 --home-dir /home/wso2 --shell /usr/sbin/nologin wso2 && \
    mkdir -p /var/run/api-platform /coverage /home/wso2/conf/llm-pricing && \
    chown -R 10001:0 /var/run/api-platform /coverage /home/wso2 && \
    chmod -R g=u /var/run/api-platform /coverage /home/wso2

WORKDIR /app
COPY --from=binary --chmod=0755 /binary/policy-engine-${TARGETARCH} /app/policy-engine
COPY --from=binary --chmod=0755 /binary/envoy-${TARGETARCH} /usr/local/bin/envoy
COPY --from=python-deps /app/python-libs/ /app/python-libs/
COPY --from=binary /binary/python-executor/ /app/python-executor/
COPY --from=binary /binary/envoy.yaml /binary/config-override.yaml /etc/envoy/
COPY --from=binary /binary/llm-pricing/ /home/wso2/conf/llm-pricing/
COPY --from=binary /binary/build-manifest.yaml /binary/LICENSE /app/
COPY --from=binary --chmod=0755 /binary/docker-entrypoint.sh /binary/health-check.sh /usr/local/bin/
RUN ln -s /app/python-libs/requirements-lock.txt /app/python-requirements-lock.txt

ENV HOME=/home/wso2 \
    PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1 \
    PYTHONPATH="/app/python-libs:/app/python-executor" \
    GATEWAY_CONTROLLER_HOST=gateway-controller \
    LOG_LEVEL=info
USER 10001:0
# Fail the build if the supplied Envoy executable or Python libraries cannot load.
RUN /usr/local/bin/envoy --version && python3 -c 'import grpc, google.protobuf'
EXPOSE 8080 8443 9901 9002 9003
LABEL org.opencontainers.image.title="WSO2 API Platform Gateway Runtime" \
      org.opencontainers.image.vendor="WSO2" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${REVISION}" \
      org.opencontainers.image.licenses="Apache-2.0"
HEALTHCHECK --interval=30s --timeout=5s --start-period=60s --retries=3 \
    CMD ["/usr/local/bin/health-check.sh"]
ENTRYPOINT ["/usr/bin/tini", "--", "/usr/local/bin/docker-entrypoint.sh"]
