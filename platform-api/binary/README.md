# WSO2 API Platform — Platform API: build your own image

This folder holds the prebuilt Platform API binaries and the Dockerfile the official `platform-api` image is built from. Use it when you need your own Platform API image, for example on a different base image. Your image runs exactly like the official one: same configuration, same mounts, same `setup.sh` in the distribution pack.

Use the build folder with the same version as the Platform API image your pack uses.

## What's here

| Path | What it is |
|---|---|
| `bin/linux-amd64/platform-api`, `bin/linux-arm64/platform-api` | The Platform API binary, one per CPU architecture |
| `internal/database/`, `resources/` | Schema files and resources the binary reads at runtime |
| `Dockerfile` | Builds the image from these files; compiles nothing |

## What you may change

Change only the `Dockerfile`, for example its base image. Don't change any other file in this folder: changes there are unsupported.

## Change the base image

Change the `FROM ubuntu:24.04` line to your base image. Leave the first stage (`FROM scratch AS bin`) as it is: it picks up the binaries from `bin/`. The binary is built with cgo, so the base must:

- be glibc-based (Alpine and other musl-based images won't run it), and
- have a glibc at least as new as the one the binary needs.

The Dockerfile installs `ca-certificates` with `apt-get` and creates the `wso2` user (UID 10001) with `groupadd`/`useradd`. If your base isn't Debian or Ubuntu, replace those lines with your base's equivalents; keep the user's UID 10001.

## Build

From this folder, for your machine's CPU:

```bash
docker build -t <your-registry>/platform-api:<version> .
```

For both architectures at once, with Docker Buildx:

```bash
docker buildx build --platform linux/amd64,linux/arm64 -t <your-registry>/platform-api:<version> --push .
```

No network access is needed beyond pulling the base image and its packages: nothing is compiled or downloaded from source.

## Run your image

In the distribution pack's `docker-compose.yaml`, replace the `image:` line of the `platform-api` service with your image, then start the pack as usual.

Your image's base operating system is yours to patch and scan for vulnerabilities.
