# Gateway Performance Results

This document captures performance characteristics of the API Platform Gateway
under incremental API scale and runtime traffic.

## Test Environment

- Deployment: Docker Compose (GitHub Actions runner)
- Gateway version: `1.0.0`
- Backend: Netty HTTP Echo Service
- Test tool: Apache JMeter
- Traffic pattern: Random API invocation across deployed APIs

## Resource Allocation

- Default resource allocations: 0.2 vCPU, 256MB memory.
- 300 TPS  for 50 APIs, each API with 8 resources.

---

## Router Memory vs API Count

![Router Memory](router_memory_vs_api_count.png)

---

## Controller Memory vs API Count

![Controller Memory](controller_memory_vs_api_count.png)


---

## Observations

- Router memory grows gradually with API count
- Controller and policy-engine memory remain relatively stable
- TPS stabilizes after initial ramp-up
- No errors observed during steady-state traffic

> These results are intended for **trend analysis**, not absolute benchmarking.
