# Coverage architecture

The framework collects runtime coverage from images built from the checked-out source.
Coverage is enabled at build time and is not exposed through a product HTTP endpoint.

## Image modes

| Version source | Build behavior | Coverage behavior |
|---|---|---|
| Suite YAML or `-gateway-version` | Do not build; use the requested image | `-gateway-version` cannot be combined with `-coverage` |
| Product `VERSION` | Build from the checkout through `core/builder` | Images are instrumented only when `-coverage` is selected; image names stay unchanged |

The catalog supplies one build specification per source-built product. Each specification owns
its supported formats, Go package patterns, JavaScript include patterns, build arguments, and
runtime environment. Product Dockerfiles are the canonical Dockerfiles and are never copied
into the framework. An explicit product version selects an existing image and cannot be combined
with `-coverage`.

## Collection flow

1. The framework passes instrumentation arguments to the product's canonical Dockerfile.
2. The framework passes `GOCOVERDIR` and/or `NODE_V8_COVERAGE` only in coverage mode.
3. The suite runs blocks concurrently on isolated networks.
4. Coverage services receive a graceful stop request after the block finishes.
5. The framework copies each service's coverage directory from the stopped container.
6. Data is stored under separate block and service directories.
7. `tools/coverage-report.sh` merges Go counters and converts Node/V8 artifacts independently.
   It writes the complete Go profile, a Go HTML report, and—when Node/V8 artifacts exist—a
   c8 text, HTML, JSON-summary, and `lcov.info` report.

Collection is attempted independently for every service. A collection error is reported with
the block and service name and does not prevent sibling cleanup. Forced termination can lose
counters that were not flushed before the process was killed.

## Product capabilities

- Platform Gateway: Go controller and policy engine; build-time Go coverage is supported.
- Platform API: Go service with atomic coverage counters.
- AI Workspace: Go BFF; the frontend is static output served by that process, so Node/V8
  coverage is not declared for the current runtime image.
- API Portal: Node.js service with V8 coverage artifacts. c8 applies the configured JavaScript
  include pattern (`**/src/**/*.js` by default) when producing the report.

Go HTML generation filters out generated or external source files that are not present in the
checkout. The complete profile and statement totals are retained separately for upload to a
coverage service.

The framework must fail clearly when a requested coverage mode is not supported by a product;
it must never label an ordinary image as instrumented.
