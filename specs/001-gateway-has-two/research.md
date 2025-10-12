# Research: Gateway with Controller and Router

**Date**: 2025-10-11
**Phase**: 0 - Outline & Research
**Purpose**: Resolve technical clarifications and establish best practices for implementation

## Overview

This document consolidates research findings for key technical decisions required to implement the Gateway system. All "NEEDS CLARIFICATION" items from the Technical Context have been resolved through industry research and best practices analysis.

---

## Decision 1: go-control-plane Version

**Question**: Which version of go-control-plane should we use for Envoy xDS v3 API?

**Decision**: Use latest stable version from `github.com/envoyproxy/go-control-plane` (February 2025 release)

**Rationale**:
- The library is actively maintained with updates as recent as February 2025
- V2 control-plane code has been removed; the library now focuses exclusively on xDS v3 APIs
- Provides comprehensive v3 packages:
  - `github.com/envoyproxy/go-control-plane/pkg/cache/v3` (snapshot cache)
  - `github.com/envoyproxy/go-control-plane/pkg/server/v3` (xDS server implementation)
  - `github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3` (discovery service APIs)
- Apache-2.0 license with redistributable terms
- Well-documented with examples in the repository
- Matches Envoy 1.35.3 compatibility requirements

**Alternatives Considered**:
- **Writing custom xDS implementation**: Rejected because go-control-plane is the official, battle-tested implementation used by major service mesh projects
- **Using older v2 APIs**: Rejected because v2 is deprecated and removed from the library

**Implementation Notes**:
- Import path: `github.com/envoyproxy/go-control-plane`
- Use v3 packages exclusively
- Leverage the snapshot cache (`cache/v3`) for managing configuration versions
- Use the server implementation (`server/v3`) for gRPC xDS server

**References**:
- https://pkg.go.dev/github.com/envoyproxy/go-control-plane
- https://github.com/envoyproxy/go-control-plane

---

## Decision 2: Storage Library Choice

**Question**: Which embedded database should we use for persisting API configurations (bbolt, Badger, or SQLite)?

**Decision**: Use **bbolt** (etcd-io/bbolt)

**Rationale**:
- **Simplicity**: Single file B+tree database, no complex LSM-tree tuning required
- **ACID Compliance**: Full transactional support with strong consistency guarantees (critical for configuration integrity)
- **Proven Stability**: Originally BoltDB, now maintained by etcd team; development is intentionally "locked" (stable, minimal changes)
- **Bucket Model**: Provides logical separation of data via buckets (comparable to tables), perfect for organizing API configs
- **Low Maintenance**: No compaction, no background goroutines, simple operational model
- **Sufficient Performance**: 339 ops/sec writes and 874K ops/sec reads - more than adequate for configuration management workload (not high-frequency trading)
- **Small Footprint**: Single binary, minimal dependencies
- **Use Case Fit**: Configuration management prioritizes consistency and simplicity over raw write throughput

**Alternatives Considered**:
- **Badger**: Rejected despite superior performance (faster than RocksDB) because:
  - LSM-tree complexity is overkill for configuration management
  - Lacks bucket concept for logical data organization
  - More operational overhead (compaction, background processes)
  - Configuration workload is read-heavy with infrequent writes, doesn't need LSM optimization

- **SQLite**: Rejected because:
  - Adds CGo dependency (complicates cross-compilation and Docker builds)
  - Relational model is overkill for key-value configuration storage
  - Larger binary size and dependency footprint
  - bbolt's simpler model better matches our use case

**Implementation Notes**:
- Import: `go.etcd.io/bbolt`
- Use buckets to separate: API configurations, audit logs, metadata
- Single read-write transaction per configuration operation (atomic updates)
- File location: `/data/gateway-controller.db` (mount as Docker volume)
- Consider implementing storage interface for future flexibility (but start with bbolt)

**References**:
- https://github.com/etcd-io/bbolt
- https://github.com/xeoncross/go-embeddable-stores (benchmark comparison)

---

## Decision 3: Build Tool Selection

**Question**: Is Make the appropriate build tool, or should we consider alternatives?

**Decision**: Use **Make** (Makefile)

**Rationale**:
- **Industry Standard**: Make remains the #1 choice for Go projects in 2025, widely adopted across major projects (Kubernetes, Docker, etc.)
- **Ubiquitous Availability**: Pre-installed on virtually all Unix-like systems (Linux, macOS), including CI/CD environments
- **Simplicity for Use Case**: Our needs are straightforward:
  - `make build` - compile Go binary
  - `make test` - run tests
  - `make docker` - build Docker images
  - `make clean` - cleanup artifacts
- **No Additional Dependencies**: Team members don't need to install extra tools
- **Good Enough**: While Make syntax is archaic, our Makefile will be simple task orchestration, not complex dependency management
- **Consistency**: Using Make for both Gateway-Controller and Router provides uniform build interface

**Alternatives Considered**:
- **Taskfile**: Rejected because:
  - Requires team to install separate binary (`task`)
  - YAML syntax is cleaner but provides minimal benefit for our simple use case
  - Adds unnecessary dependency to development environment

- **Mage**: Rejected because:
  - Requires Go-based build scripts (magefile.go)
  - Overkill for simple task automation
  - Team needs to learn Mage API instead of simple Make targets

- **Just**: Rejected because:
  - Another tool to install
  - Explicitly not a build tool (command runner only)
  - Less familiar to most developers than Make

**Implementation Notes**:
- Create Makefiles for both `gateway-controller/` and `router/` directories
- Standard targets: `build`, `test`, `docker`, `clean`, `run`, `help`
- Use `.PHONY` declarations for non-file targets
- Include helpful comments and `make help` target
- Docker multi-stage builds to keep images minimal

**References**:
- https://www.alexedwards.net/blog/a-time-saving-makefile-for-your-go-projects
- https://vincent.bernat.ch/en/blog/2019-makefile-build-golang

---

## Decision 4: Go Project Structure

**Question**: What project structure should we follow for Gateway-Controller?

**Decision**: Use **standard Go project layout** with `cmd/`, `pkg/`, and `tests/` directories

**Rationale**:
- **Industry Convention**: The golang-standards/project-layout pattern is widely adopted, especially in infrastructure projects like Kubernetes
- **Clear Separation**:
  - `cmd/` - application entry points (executables)
  - `pkg/` - reusable library code organized by domain
  - `tests/` - test code separate from implementation
- **Modularity**: Aligns with Kubernetes design goals (modularity, decoupling, explicit package organization)
- **Future Extensibility**: If we later add CLI tools or additional services, the structure accommodates them easily
- **Not Over-Engineered**: Despite being a "standard layout," we're only using core directories relevant to our needs
- **Avoid `/internal` Initially**: We'll use `/pkg` for now since we may want to reuse xDS translation logic in other platform components

**Alternatives Considered**:
- **Flat Structure (all code in root)**: Rejected because:
  - Works for tiny projects but Gateway-Controller has multiple concerns (API, xDS, storage, config parsing)
  - Harder to navigate and test as codebase grows

- **Heavy DDD Structure**: Rejected because:
  - Domain-driven design with extensive layering is overkill
  - Our domain is straightforward (configuration management and xDS translation)
  - Adds unnecessary directory depth and complexity

**Implementation Notes**:
- Package organization within `pkg/`:
  - `api/` - Gin HTTP handlers and middleware
  - `config/` - YAML/JSON parsing and validation
  - `models/` - Data structures
  - `storage/` - Database abstraction and implementation
  - `xds/` - xDS server and Envoy configuration translation
  - `logger/` - Zap logging setup
- Single executable in `cmd/controller/main.go`
- Tests organized to mirror source structure

**References**:
- https://github.com/golang-standards/project-layout
- https://leapcell.medium.com/learning-large-scale-go-project-architecture-from-k8s-6c8f2c3862d8
- https://www.alexedwards.net/blog/11-tips-for-structuring-your-go-projects

---

## Additional Research: Envoy xDS Protocol

**Context**: Understanding how to implement the xDS server and translate API configs to Envoy configuration

**Key Findings**:
- **xDS Resources**: Need to implement:
  - Listener Discovery Service (LDS) - defines listeners for HTTP traffic
  - Route Discovery Service (RDS) - configures routes for listeners
  - Cluster Discovery Service (CDS) - defines upstream backend clusters
  - Endpoint Discovery Service (EDS) - provides endpoints for clusters (optional for our use case)
- **SotW (State-of-the-World) Protocol**:
  - xDS protocol variant where the control plane sends the complete configuration state in each response
  - Envoy connects via gRPC stream and requests resource types (LDS, RDS, CDS, etc.)
  - Control plane responds with ALL resources of that type (not incremental deltas)
  - Simpler than incremental xDS; suitable for configuration management use cases
  - go-control-plane's snapshot cache implements SotW by default
- **Snapshot Cache**: go-control-plane provides snapshot-based cache that simplifies configuration management
  - Create new snapshot when API config changes
  - Each snapshot contains the complete state of all resources
  - Cache handles versioning and distribution to connected Envoys
  - Envoy polls or streams updates based on resource versions
  - Snapshot version is monotonically increasing integer (converted to string)
- **Translation Pattern**:
  1. Load all API configurations from database to in-memory maps on startup
  2. Generate initial xDS snapshot from in-memory maps
  3. On configuration change:
     - Update in-memory maps + database atomically
     - Translate ALL API configs from in-memory maps to Envoy resources
     - Create new snapshot with incremented version
     - Update cache with new snapshot (SotW approach)
  4. Envoy receives complete configuration state and applies it
- **Graceful Updates**: Envoy's connection draining ensures in-flight requests complete before configuration changes take effect

**Implementation Impact**:
- In-memory maps structure in `pkg/storage/memory.go`
- Database loader on startup in `pkg/storage/[implementation].go`
- xDS translation logic reads from in-memory maps in `pkg/xds/translator.go`
- SotW snapshot management in `pkg/xds/snapshot.go`
- xDS SotW server setup in `pkg/xds/server.go`

**References**:
- https://www.envoyproxy.io/docs/envoy/latest/api-docs/xds_protocol
- https://blog.christianposta.com/envoy/guidance-for-building-a-control-plane-to-manage-envoy-proxy-based-infrastructure/

---

## Additional Research: Docker Multi-Stage Builds

**Context**: Both components need minimal Docker images

**Key Findings**:
- **Gateway-Controller**:
  - Stage 1: Use `golang:1.25.1` to build binary
  - Stage 2: Use `alpine:latest` or `scratch` for runtime
  - Result: Final image <20MB (compared to 1GB+ with full Go image)

- **Router**:
  - Use `envoyproxy/envoy:v1.35.3` as base
  - Copy bootstrap YAML into container
  - No custom build needed (Envoy binary already present)

**Implementation Impact**:
- Gateway-Controller Dockerfile uses multi-stage build
- Router Dockerfile is simple (FROM + COPY + ENTRYPOINT)
- Both images support linux/amd64 and linux/arm64 architectures

---

## Summary of Resolved Clarifications

| Item | Decision | Confidence |
|------|----------|-----------|
| go-control-plane version | Latest stable (Feb 2025), v3 packages | ✅ High |
| Storage library | bbolt (go.etcd.io/bbolt) | ✅ High |
| Build tool | Make (Makefile) | ✅ High |
| Project structure | Standard layout (cmd/pkg/tests) | ✅ High |

**Status**: All technical clarifications resolved. Ready to proceed to Phase 1 (Data Model & Contracts).
