# WSO2 API Platform — API Portal: build your own image

This folder holds the API Portal source code and the Dockerfile the official `api-portal` image is built from. Use it when you need your own API Portal image, for example on a different base image. Your image runs exactly like the official one: same configuration, same mounts, same `setup.sh` in the API Portal distribution pack.

Use the build folder with the same version as your API Portal pack.

## What you may change

Change only the `Dockerfile`, for example its base image. Don't change any other file in this folder: changes there are unsupported.

## Prerequisites

- Docker with BuildKit (Docker 23 or later uses it by default).
- Internet access to the public npm registry while the image builds. The build runs `npm ci` to install the dependencies in `package-lock.json`. Without internet access, point npm at your own registry mirror (see below).

## Change the base image

The Dockerfile has two `FROM node:24-alpine` lines: the build stage and the application stage. Change both to your base image. It must provide:

- Node.js 24 (24.15 or later avoids a startup warning), and
- npm in the build stage.

The other steps work on any Linux base image with a POSIX shell, including images whose default user isn't root (such as Red Hat's Node.js images): each stage switches to root for its setup steps, and the image still runs as UID 10001. Keep those `USER` lines.

To install dependencies through your own npm registry, add `npm config set registry <url>` before `npm ci` in the build stage, or the equivalent for your proxy.

## Build

From this folder:

```bash
docker build -t <your-registry>/api-portal:<version> .
```

No extra build arguments are needed. Push the image to a registry your hosts can pull from, if they are not the machine you built on.

## Run your image

In the API Portal pack's `docker-compose.yaml`, replace the `image:` line of the `api-portal` service with your image:

```yaml
  api-portal:
    image: <your-registry>/api-portal:<version>
```

Then start the pack as usual (see the pack's `README.md`). Nothing else changes.

## After an update

1. Check the `image:` line in the pack's `docker-compose.yaml` still names your image.
2. If the update moves the pack to a new API Portal version, download the build folder of that version, apply your Dockerfile changes to its Dockerfile again, and rebuild.
3. From the pack directory, recreate the API Portal container so it runs your current image (`docker compose restart` would keep the old one):

   ```bash
   docker compose up -d --no-deps api-portal
   ```

   If the stack runs on another host, push the rebuilt image to your registry first, and run `docker compose pull api-portal` on that host before the command above.

Your image's base operating system is yours to patch and scan for vulnerabilities.
