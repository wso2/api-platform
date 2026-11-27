# Build the Gateway Runtime image

You need Docker running and the API Gateway binary ZIP downloaded.

## Build command

Replace the placeholders before running:

```bash
docker build \
  --build-arg BINARY_PATH="<binary-zip-path>" \
  --build-arg PRODUCT_VERSION="<product-version>" \
  -f "<dockerfile-path>" \
  -t "<image-name>:<image-tag>" \
  "<build-context-directory>"
```

- `<binary-zip-path>`: ZIP path relative to the build context directory.
- `<product-version>`: version of the binary package.
- `<dockerfile-path>`: Dockerfile path relative to your current terminal directory.
- `<image-name>:<image-tag>`: name and tag for the image you create.
- `<build-context-directory>`: folder containing the ZIP.

The Dockerfile and ZIP do not need to be in the same folder.
Docker selects your machine's architecture automatically.

## Example

If your ZIP is `/tmp/binaries/api-gateway-1.2.0.13.zip`, run this from the
repository root:

```bash
docker build \
  --build-arg BINARY_PATH=api-gateway-1.2.0.13.zip \
  --build-arg PRODUCT_VERSION=1.2.0.13 \
  -f docker/api-gateway/1.2.0/gateway-runtime/Dockerfile \
  -t local/gateway-runtime:1.2.0.13 \
  /tmp/binaries
```

This creates `local/gateway-runtime:1.2.0.13` on your machine.
Check that it exists:

```bash
docker image inspect local/gateway-runtime:1.2.0.13
```

If you have already extracted the ZIP, set `BINARY_PATH` to the extracted
folder path instead. That folder must be inside the build context directory.
