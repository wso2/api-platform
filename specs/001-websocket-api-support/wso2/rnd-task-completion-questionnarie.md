## **R&D Task Completion Questionnaire**

| General Information |  |
| :---- | :---- |
| Submitted by | naduni@wso2.com |
| Submitted date | 2026-10-14 |
| Product and version | WSO2 API Platform (API Platform Gateway / Platform API); exact version NOT ANSWERED |
| Reviewed and approved by | NOT ANSWERED |
| Reviewed date | NOT ANSWERED |
| Short description about the task | Adds WebSocket APIs as a first-class artifact kind, proxied through the standalone API Platform Gateway, with full control-plane, command-line, console, and developer-portal support, mirroring the existing GraphQL API support feature end to end. |
| Github issue | NOT ANSWERED |
| Any email subjects and/or proposal docs related to the task | NOT ANSWERED |
| Architecture review done? Add link to notes. | NOT ANSWERED |
| Code review done? Add link to PR and review notes. | NOT ANSWERED — implementation not yet complete as of this document. |
| Security review done? Add link to notes. | NOT ANSWERED — the Security Impact Review document in this set is the design-time pass; whether a further, independent security review has been completed is not established in the inputs. |
| Behavioural Changes |  |
| Has any APIs or interfaces been changed during this task? Provide details. | Yes — exclusively additive new endpoints (see the REST APIs document). No existing endpoint's request or response shape changes. |
| Has new configurations been introduced or the behaviour of existing configurations changed? Provide details. | Yes — new gateway configuration keys for the WebSocket idle-timeout/max-duration bounds and the accept-over-HTTP/2 flag (see the Implementation document, §4). No existing configuration key's behaviour changes. |
| Database Changes |  |
| Has any changes to database schema been introduced during this task? Provide details. | Yes — one new, additive table (see the DB Schema document). No existing table is modified. |
| If any changes were introduced, mention the types of databases they were tested with. | NOT ANSWERED — not established in the inputs (see the DB Schema document, §5). |
| If any changes were introduced, were all database schemas updated accordingly? Provide PR links.  | NOT ANSWERED — PR not yet created as of this document. The DB Schema document, §3, lists every dialect file the change needs to land in. |
| If any changes were introduced, have they been considered in the migration scripts? If not, why? If yes, provide details and links. | The new table is additive, created with a guarded creation statement that is a no-op against an already-provisioned database — there is no column being added to an existing table and therefore no separate `ALTER TABLE`/migration path is needed for it (see the DB Schema document, §5). Whether this has been exercised against a pre-existing database as part of testing: NOT ANSWERED. |
| Security |  |
| Explain the security aspects of the task. For example how the services etc. are secured. | Every control-plane endpoint is scoped by dedicated, least-privilege scopes checked per operation; the organization used for tenant isolation is always resolved from verified token claims, never request input; credentials accepted at the WebSocket handshake are stripped before reaching the upstream, logs, or analytics; AsyncAPI import by URL goes through the platform's shared SSRF-safe fetch client; an exact-match origin allowlist is available at the handshake as the cross-site WebSocket hijacking defence. Full detail is in the Security Impact Review document. |
| What are the minimum permission needed by a user to use the feature? | To read/discover: the read-level scope for this artifact kind. To create, configure, and deploy: the manage and deployment-manage scopes for this artifact kind. |
| Tests |  |
| Describe the types of tests implemented and provide links to PRs. | Integration tests covering deployment, connection handling, handshake security, connection limits, control-plane/command-line behaviour, console/portal UI, and analytics; component/unit tests for the control-plane and command-line layers (see the Test Scenarios document). PR links: NOT ANSWERED — not yet created as of this document. |
| Were any existing test cases changed during this effort? If yes, please explain why. | NOT ANSWERED — not established in the inputs. This feature is additive and is not expected to require changing an existing test case, but this has not been confirmed. |
| Have you tested this across different operating systems? Provide details. If not, explain why. | NOT ANSWERED. |
| Performance |  |
| Why do you think this doesn't impact the performance of the existing capabilities of the product? If this has an impact, has it been discussed with the leads. Provide details. | The feature adds new routes and a new artifact kind; it does not change the translation, routing, or policy-processing path for any existing API kind. The one capacity-relevant addition is a new, deliberately-bounded connection-counting mechanism specific to this artifact kind's own routes (see the Implementation document, §2.3), which does not run on non-WebSocket traffic. |
| If this is a performance sensitive task, have necessary performance tests been conducted? Provide evidence. | NOT ANSWERED — the performance targets are defined (specification document, SC-006/SC-007), but whether they have been measured against a built implementation is not established in the inputs (see the Test Scenarios document, §8). |
| Documentation |  |
| Has the documentation completed for this task? Provide links to PRs and the respective documentation page. | NOT ANSWERED — not established in the inputs as of this document. |
| Does this have an impact on the existing documentation? If so, have they been updated accordingly? Provide links to PRs. | Yes, in the sense that a new artifact kind is being added to a product whose reference documentation already covers REST, GraphQL, and other kinds side by side — the existing gateway and command-line reference documentation needs a new section for this kind. Whether that documentation update has itself been completed: NOT ANSWERED. |
| Customer Impact |  |
| Explain why this task doesn't impact the existing customers of the product. If this has an impact, has it been discussed with the leads? Provide details. | This feature is purely additive — a new artifact kind, a new database table, and new endpoints. No existing endpoint's request/response shape changes, no existing table is altered, and no existing API kind's routing or policy behaviour changes. A customer who does not use WebSocket APIs is unaffected. |
| Migration Impact |  |
| Explain why this task doesn't impact the smooth upgrade flow of the product. | The new table is additive and created with a guarded creation statement that is a no-op against an already-provisioned database — there is no column being added to an existing table, and therefore no `ALTER TABLE` path to get wrong. The feature is further gated behind a minimum supported gateway version, so an operator who upgrades the control plane without yet upgrading their gateway simply cannot deploy this artifact kind until they do, with no disruption to their existing deployments. |
