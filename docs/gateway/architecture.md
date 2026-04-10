# Gateway Architecture

## Overview

The API Gateway consists of two main components: **Gateway Controller** and **Gateway Runtime**.

- **Gateway Controller** is the control plane that manages API configurations and pushes them to the Gateway Runtime via the xDS protocol.
- **Gateway Runtime** is the data plane that processes API traffic. It contains two sub-components:
  - **Router** (Envoy proxy) — handles traffic routing, load balancing, and TLS termination.
  - **Policy Engine** — an ext_proc filter that executes request/response policies. Policies are compiled into the Policy Engine binary at image build time by the Gateway Builder.

The Gateway Controller configures the Gateway Runtime by pushing API and route configurations through xDS. When a request arrives, the Router forwards it to the Policy Engine for policy evaluation, then routes it to the upstream backend.

## Gateway Architecture

![Gateway Architecture](../images/gateway-architecture.png)

## High Availability Setup

In a production HA deployment:

- **Gateway Controller** instances connect to a shared **PostgreSQL** database for persistent storage of API configurations, subscriptions, and other metadata.
- **Gateway Runtime** instances connect to a shared **Redis** instance used for distributed rate limiting, ensuring rate limit counters are synchronized across all runtime instances.

![Gateway High Availability Setup](../images/gateway-ha-setup.png)

## Configuration

### PostgreSQL (Gateway Controller)

To use PostgreSQL as the storage backend for the Gateway Controller, update the `config.toml`:

```toml
[controller.storage]
type = "postgres"

[controller.storage.postgres]
host = "postgres.example.com"
port = 5432
database = "gateway"
user = "gateway"
password = "your-postgres-password"
```

For the full list of PostgreSQL configuration options, refer to the [config template](../../gateway/configs/config-template.toml).

### Redis (Gateway Runtime — Distributed Rate Limiting)

To enable distributed rate limiting across multiple Gateway Runtime instances, configure the rate limiting policy to use Redis as the backend in `config.toml`:

```toml
[policy_configurations.ratelimit_v1]
algorithm = "fixed-window"
backend = "redis"

[policy_configurations.ratelimit_v1.redis]
host = "redis.example.com"
port = 6379
password = "your-redis-password"
```

For the full list of Redis configuration options, refer to the [Advanced Rate Limiting documentation](https://github.com/wso2/gateway-controllers/blob/main/docs/advanced-ratelimit/v1.0/docs/advanced-ratelimit.md).
