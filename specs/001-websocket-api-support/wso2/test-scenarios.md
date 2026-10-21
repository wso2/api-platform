# **Test Scenarios**

# 1. Coverage Summary

| Area | Unit | Integration | Manual | Notes |
| :---- | :---- | :---- | :---- | :---- |
| Gateway routing and translation | No | Yes | No | Deploy/connect/echo, upgrade opt-in, idle/max-duration, sandbox routing. |
| Handshake security | No | Yes | No | All credential locations, rejection cases, origin allowlist, scope enforcement. |
| Connection lifecycle and limits | No | Yes | No | Idle/max-duration closes, per-API connection ceiling, plus the same bounds applied independently per upstream slot (FR-015–FR-017). |
| Control plane (AsyncAPI import, deployment, bulk sync) | Yes | Yes | No | |
| Command-line tool | Yes | No | No | |
| Console and developer-portal UI | Yes | Yes | No | Component-level tests for the console; end-to-end tests for the developer-portal try-it feature. |
| Analytics | No | Yes | No | |
| Performance and resilience | No | Yes | No | Load/scale harness, run separately from the standard per-change suite. |

# 2. Preconditions and Test Data

* **Environment**: a gateway built with this feature's changes, running a version that satisfies the new minimum-version gate; a WebSocket echo backend reachable from the gateway as the test upstream; the control plane running with the new scopes configured; a registered gateway connection between the two.
* **Seed data**: an organization, a project, a registered gateway, and at least one WebSocket API definition (an AsyncAPI document) to create from.
* **Credentials/roles**: creation, update, and deployment scenarios run as a caller holding `ap:websocket_api:manage` / `ap:websocket_api:deployment:manage`; read scenarios run as a caller holding only the `:read` scopes; negative authorization scenarios run as an unauthenticated caller and as a caller from a different organization.

# 3. Functional Scenarios

| ID | Requirement | Scenario | Steps | Expected result |
| :---- | :---- | :---- | :---- | :---- |
| TS-1 | FR-010, FR-011 | Deploy and connect/echo | 1. Deploy an API with one channel pointing at the echo backend. 2. Connect a client. 3. Send a message. | Handshake accepted; sent frames echoed back unchanged. |
| TS-2 | FR-011 | A non-WebSocket route rejects an upgrade | 1. Deploy a REST API on the same gateway. 2. Send an upgrade request to the REST route. | Upgrade rejected; the REST route keeps serving normal requests. |
| TS-3 | FR-004 | TLS and sandbox upstream routing | 1. Deploy with a TLS-secured main upstream and a plain sandbox upstream. 2. Connect through each vhost. | Each connection reaches the correct backend over the correct transport. |
| TS-4 | FR-030 | Credential accepted from every supported location | 1. Attach an authentication policy. 2. Connect with the credential in a header, a query parameter, and the subprotocol negotiation header, in turn. | All three accepted; the credential is stripped before the upstream request. |
| TS-5 | FR-012, FR-013, FR-031 | Idle timeout and max-duration enforcement | 1. Set a short idle timeout and a short maximum connection duration. 2. Leave one connection idle. 3. Keep another alive with pings past the idle timeout. 4. Wait past the maximum duration. | The idle connection closes at the idle timeout; the pinging connection survives past it; both close by the maximum duration. |
| TS-5a | FR-015, FR-017 | Upstream idle timeout and max-duration enforcement | 1. Set a short upstream idle timeout and a short upstream maximum connection duration on the `main` slot. 2. Leave the backend connection idle past the upstream idle timeout. 3. Keep a connection alive past the upstream maximum duration. | The gateway closes/recycles the pooled backend connection at each configured bound, independently of the downstream connection's own idle-timeout/max-duration settings. |
| TS-6 | FR-040 | Per-API connection ceiling | 1. Set a connection ceiling of N. 2. Open N+1 connections. | The (N+1)th is rejected with a 503 response, within 100 ms, never queued. |
| TS-6a | FR-016 | Per-upstream-slot connection ceiling | 1. Set `main`'s upstream connection ceiling to N. 2. Open N+1 connections routed to `main`, while a `sandbox` connection is also open. | The gateway's native circuit breaker bounds `main`'s backend connections at N; the `sandbox` connection is unaffected, confirming per-slot, not per-API, scope. |
| TS-7 | FR-052 | AsyncAPI import from every source and supported version | 1. Create an API from a file, a URL, and inline content, for each supported AsyncAPI major version. | Channels, upstream, and basic info are derived correctly in every case. |
| TS-8 | FR-053 | Version-gated deployment refusal | 1. Attempt to deploy to a gateway below the minimum supported version. | Deployment refused before translation is attempted. |
| TS-9 | FR-054 | Inclusion in bulk sync | 1. Register or reconnect a gateway that already has WebSocket APIs deployed. | WebSocket APIs and their API keys are included in the synchronization payload. |
| TS-10 | FR-060 | Command-line parity | 1. Run the list/get/delete and API-key subcommands for this kind. | Behaviour matches the equivalent GraphQL command family. |
| TS-11 | FR-070, FR-071 | Console creation and test console | 1. Create via the wizard. 2. Open the test console. 3. Connect, send, and receive a message. | Channels previewed before creation; the message round-trips in the test console. |
| TS-12 | FR-055 | Developer-portal live URL and try-it | 1. Publish the API. 2. Open its documentation page. 3. Use the try-it feature. | The live gateway URL is shown; try-it connects and exchanges a message. |
| TS-13 | FR-080, FR-081 | One analytics event per connection | 1. Open and close a connection normally. | Exactly one event is produced, with the correct close reason, not classified as an error. |

# 4. Negative and Validation Scenarios

| ID | Scenario | Input | Expected result |
| :---- | :---- | :---- | :---- |
| TS-N1 | No channels supplied | An API definition with an empty channel list | 400 Bad Request, naming the missing requirement |
| TS-N2 | Duplicate channel path | Two channels with the same path | 400 Bad Request |
| TS-N3 | Invalid upstream URL | An upstream URL that is not `ws`/`wss` | 400 Bad Request |
| TS-N4 | Body-processing policy in the chain | A policy that declares it needs the request or response body | 400 Bad Request, naming the policy |
| TS-N5 | Connection setting above the configured upper bound | An idle timeout or max duration above the gateway's configured ceiling | 400 Bad Request |
| TS-N6 | AsyncAPI document with no WebSocket server | A document using only a non-WebSocket protocol | 400 Bad Request |
| TS-N7 | Unknown reference | Get/delete a nonexistent API or gateway id | 404 Not Found |

# 5. Authorization and Tenant Isolation Scenarios

| ID | Scenario | Expected result |
| :---- | :---- | :---- |
| TS-A1 | Unauthenticated request to a control-plane endpoint | 401, identical body for every failure cause |
| TS-A2 | Authenticated caller missing the required scope | 403, generic body |
| TS-A3 | Caller from organization A targets an API in organization B | 404/403 — no cross-tenant access, no existence disclosure |
| TS-A4 | Handshake with a missing, invalid, or revoked credential | The platform's identical unified unauthorized body used everywhere else; the backend is never contacted |

# 6. Failure, Retry and Recovery Scenarios

| ID | Scenario | Expected result |
| :---- | :---- | :---- |
| TS-F1 | Policy engine unavailable at handshake time | Handshakes fail closed (rejected); already-open connections are unaffected |
| TS-F2 | Gateway restart or rollout while connections are open | All connections on that instance close at once; clients reconnect. Whether the gateway has been validated to absorb the resulting reconnect burst at a representative scale is an open question — see the specification document, §7 |
| TS-F3 | Undeploy, key revocation, subscription change, or policy change while a client is connected | The live connection is unaffected; it ends no later than its own maximum connection duration |
| TS-F4 | Concurrent updates to the same API | NOT ANSWERED — conflict-handling behaviour is not specified in the inputs |

# 7. Upgrade and Compatibility Scenarios

| ID | Scenario | Expected result |
| :---- | :---- | :---- |
| TS-U1 | Deploy the new table to a fresh install | Table created via the guarded creation statement; no other data affected |
| TS-U2 | Deploy to an already-provisioned database | The guarded creation statement is a no-op if the table already exists; no data migration is needed, since this is a new table, not a change to an existing one |
| TS-U3 | A gateway below the minimum supported version against the updated control plane | Deployment of this artifact kind is refused; deployments of every other kind are unaffected |

# 8. Performance Scenarios

| ID | Scenario | Target | Measured |
| :---- | :---- | :---- | :---- |
| TS-P1 | Handshake latency with a one-policy auth chain | Under 10 ms at p95 (specification document, SC-006) | NOT ANSWERED |
| TS-P2 | Frame-relay overhead | Under 1 ms at p95 (SC-006) | NOT ANSWERED |
| TS-P3 | Concurrent idle connections on one gateway instance | At least 10,000, with no held processing stream per idle connection (SC-007) | NOT ANSWERED |
| TS-P4 | Mass-reconnect burst after a gateway restart | Gateway remains stable | NOT ANSWERED — see the specification document, §7 |

# 9. Automation

| Test suite | Location | PR |
| :---- | :---- | :---- |
| Gateway and control-plane integration suite (behaviour-driven feature scenarios) | The repository's shared integration-test framework | NOT ANSWERED |
| Component unit tests (repository, service, and handler layers) | Colocated with each changed component | NOT ANSWERED |
| Command-line tool unit tests | Colocated with the new command family | NOT ANSWERED |
| Console component tests | The console's existing test suite | NOT ANSWERED |
| Developer-portal end-to-end tests | The portal's existing end-to-end test suite | NOT ANSWERED |
