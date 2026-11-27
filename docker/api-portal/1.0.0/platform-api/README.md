# Build the Platform API image

You need Docker running and the API Portal binary ZIP downloaded. The Platform
API binaries are in the same ZIP as the API Portal.

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

If your ZIP is `/tmp/binaries/api-portal-binary-1.0.0.7.zip`, run this from the
repository root:

```bash
docker build \
  --build-arg BINARY_PATH=api-portal-binary-1.0.0.7.zip \
  --build-arg PRODUCT_VERSION=1.0.0.7 \
  -f docker/api-portal/1.0.0/platform-api/Dockerfile \
  -t local/platform-api:1.0.0.7 \
  /tmp/binaries
```

This creates `local/platform-api:1.0.0.7` on your machine.
Check that it exists:

```bash
docker image inspect local/platform-api:1.0.0.7
```

If you have already extracted the ZIP, set `BINARY_PATH` to the extracted
folder path instead. That folder must be inside the build context directory.

To use a different base image, pass `--build-arg BASE_IMAGE=<image>`. The binary
uses cgo, so the base image must be glibc-based (not Alpine).
