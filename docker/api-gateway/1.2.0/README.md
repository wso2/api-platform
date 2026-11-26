# API Gateway 1.2.0 Dockerfiles

Use these Dockerfiles to build your own API Gateway 1.2.0 images from the API
Gateway binary, for example to use a different base image.

## What is here

| Directory | Builds |
|---|---|
| [`gateway-controller/`](gateway-controller/) | Gateway Controller image |
| [`gateway-runtime/`](gateway-runtime/) | Gateway Runtime image |

The gateway needs both images. Each directory has its own README with the build
command.

## Get the binary

Both images are built from the same API Gateway binary ZIP. It is published to
the Choreo Labs registry, tagged with the product version (for example `1.2.0.13`):

```
registry.wso2.com/wso2-api-platform/binaries/api-gateway:<product-version>
```

Download it with [ORAS](https://oras.land), using your Choreo Labs subscription
credentials:

```bash
oras login registry.wso2.com
oras pull registry.wso2.com/wso2-api-platform/binaries/api-gateway:<product-version>
```

The ZIP contains a `gateway-controller/` and a `gateway-runtime/` directory,
with binaries for `linux/amd64` and `linux/arm64`.

## Versions

Use the Dockerfiles in this directory only with API Gateway 1.2.0 binaries
(`1.2.0.x`). Other product versions have their own directory under
[`docker/api-gateway/`](../).
