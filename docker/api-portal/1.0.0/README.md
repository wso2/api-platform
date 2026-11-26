# API Portal 1.0.0 Dockerfiles

Use these Dockerfiles to build your own API Portal 1.0.0 images from the API
Portal binary, for example to use a different base image.

You don't need them to run API Portal. Choreo Labs publishes ready-made images for
every update.

## What is here

| Directory | Builds |
|---|---|
| [`api-portal/`](api-portal/) | API Portal image |
| [`platform-api/`](platform-api/) | Platform API image |

API Portal needs both images. Each directory has its own README with the build
command.

## Get the binary

Both images are built from the same API Portal binary ZIP. It is published to
the Choreo Labs registry, tagged with the product version (for example `1.0.0.7`):

```
registry.wso2.com/wso2-api-platform/binaries/api-portal:<product-version>
```

Download it with [ORAS](https://oras.land), using your Choreo Labs subscription
credentials:

```bash
oras login registry.wso2.com
oras pull registry.wso2.com/wso2-api-platform/binaries/api-portal:<product-version>
```

This downloads `api-portal-binary-<product-version>.zip`. The ZIP contains an
`api-portal/` and a `platform-api/` directory, with binaries for `linux/amd64`
and `linux/arm64`.

## Versions

Use the Dockerfiles in this directory only with API Portal 1.0.0 binaries
(`1.0.0.x`). Other product versions have their own directory under
[`docker/api-portal/`](../).
