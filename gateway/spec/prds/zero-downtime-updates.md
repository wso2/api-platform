# Zero-Downtime Updates

## Overview

Apply API configuration changes to the Router without dropping in-flight HTTP requests or causing connection errors, using Envoy's graceful xDS configuration updates.

## Requirements

### xDS Update Mechanism
- Gateway-Controller generates new xDS snapshot with incremented version number on configuration change
- xDS server pushes complete snapshot (State-of-the-World) to Router via gRPC stream
- Router validates new configuration before applying
- Envoy performs graceful drain of in-flight requests on old listeners/routes

### Update Propagation
- Configuration changes propagate to Router within 5 seconds of API submission
- xDS snapshot version increments monotonically (no version conflicts)
- Router continues serving traffic on current configuration during update
- Failed configuration validation does not disrupt existing traffic

### Connection Handling
- In-flight HTTP requests complete using old route configuration
- New requests use updated route configuration after xDS apply
- No TCP connection resets during configuration updates
- HTTP keep-alive connections remain active across updates

### Rollback Support
- Invalid configurations rejected at validation layer before xDS update
- xDS snapshot cache maintains previous versions for potential rollback
- Manual rollback via API update reverting to previous configuration

## Success Criteria

- 100% of in-flight requests complete successfully during configuration updates
- Zero TCP connection errors (RST packets) during xDS updates
- Configuration updates apply to Router within 5 seconds measured from API submission to active routing
- Invalid configurations rejected with clear error messages without affecting Router state
- Structured logs capture xDS update events with snapshot version numbers for audit trail
