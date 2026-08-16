# Coverage architecture

The framework collects server-side coverage from images built from the checked-out source.
Coverage is enabled at build time and is not exposed through a product HTTP endpoint.

## Image modes

| Version source | Build behavior | Coverage behavior |
|---|---|---|
| Suite YAML or `-gateway-version` | Do not build; use the requested image | `-gateway-version` cannot be combined with `-coverage` |
| Product `VERSION` | Build from the checkout through `core/builder` | Gateway images are instrumented and retain their normal image names |

The catalog supplies one build specification per source-built product. Product Dockerfiles are
the canonical Dockerfiles and are never copied into the framework.

## Collection flow

1. The product build sets `GOCOVERDIR` for instrumented Go services.
2. The suite runs blocks concurrently on isolated networks.
3. Coverage services receive a graceful stop request after the block finishes.
4. The framework copies each service's coverage directory from the stopped container.
5. Data is stored under separate block and service directories.
6. `tools/coverage-report.sh` merges the counters and renders the report.

Collection is attempted independently for every service. A collection error is reported with
the block and service name and does not prevent sibling cleanup. Forced termination can lose
counters that were not flushed before the process was killed.

## Product capabilities

- Platform Gateway: Go controller and policy engine; build-time Go coverage is supported.
- Platform API: Go service; source-image build metadata is supported. Coverage is enabled only
  after its Docker build contract exposes instrumentation without product runtime endpoints.
- AI Workspace: Go BFF and JavaScript frontend; source-image build metadata is supported.
  Go and frontend coverage are separate reporting concerns.
- API Portal: Node.js service and frontend; source-image build metadata is supported. It is not
  represented as Go coverage.

The framework must fail clearly when a requested coverage mode is not supported by a product;
it must never label an ordinary image as instrumented.
