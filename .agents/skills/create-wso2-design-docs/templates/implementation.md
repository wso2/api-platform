# **\<Feature Name\> - Implementation**

<!--
The build plan. overview.md is the "what and why" for reviewers; this is the "how" for whoever
writes the code. Structure it by component (CLI, control plane, data plane, portal, ...), one
top-level section per work stream, so each can be reviewed and merged independently.
-->

# **1\. Overview**

\<One or two paragraphs: what is being implemented and how the work splits across components.\>

* **\<Component A\> changes**: \<summary\>
* **\<Component B\> changes**: \<summary\>

\<Any precondition an operator must satisfy before the feature can be used — a config that must
be turned off, a version floor, a migration that must run first.\>

# **2\. \<Component A\> Implementation**

## **2.1 \<Structure / layout / model\>**

<!-- Directory layouts, file formats, manifest shapes — anything a developer must match exactly. -->

```
<tree, file layout, or schema>
```

## **2.2 \<Interfaces / command set / entry points\>**

| \<Area\> | \<Command / endpoint / function\> | Purpose |
| :---- | :---- | :---- |
| \<area\> | \<name\> | \<what it does\> |

## **2.3 \<Behaviour\>**

* \<rule the implementation must satisfy\>
* \<rule\>

```
<example invocation or payload>
```

# **3\. \<Component B\> Implementation**

## **3.1 API Changes**

<!-- Summarise here; the full contract lives in rest-apis.md. Link rather than duplicate. -->

\<What changes in the request/response shape, and why.\> The full contract is in the REST APIs
document (name it in prose — never link to it; see the skill's rule 5).

## **3.2 Request Validation**

<!--
One row per input condition, including the omitted/empty/duplicate/unknown cases. This table is
the source for the negative test cases in test-scenarios.md.
-->

| Validation | Expected behavior |
| :---- | :---- |
| \<field omitted\> | \<behaviour\> |
| \<field is empty\> | \<behaviour\> |
| \<duplicate values\> | Reject with 400 Bad Request. |
| \<unknown reference\> | Reject with 404 Not Found. |

## **3.3 Storage Model**

\<How the data is persisted and why it is shaped that way. The full DDL is in the DB schema
document, §2.\>

## **3.4 \<Create / Update / Delete\> Request Handling**

1. \<step\>
2. \<step\>

\<Call out any non-obvious invariant explicitly — what the operation does NOT do, ordering
guarantees, idempotency, concurrency handling.\>

## **3.5 Error Handling**

| Condition | Status | Client-facing behaviour | Logged internally |
| :---- | :---- | :---- | :---- |
| \<condition\> | \<4xx/5xx\> | \<generic message\> | \<specific reason\> |

## **3.6 Concurrency, Retries and Failure Modes**

* \<what happens on a partial failure, a retry, a concurrent update, a restart mid-flow.\>

# **4\. Configuration**

<!--
Every new or changed configuration key the feature introduces. State the default explicitly and
say whether it is safe/fail-closed. A key with no stated default is an open question, not a
blank cell. If the feature adds none, delete this section.
-->

| Key | Type | Default | Component | Description |
| :---- | :---- | :---- | :---- | :---- |
| \<key\> | \<string / int / bool / duration\> | \<default\> | \<component\> | \<what it controls, and whether the default is safe\> |

\<Whether the feature is gated behind an enable flag, and what the out-of-the-box state is.\>

# **5\. Security Considerations**

<!--
Summarise the security-relevant decisions; the full questionnaire lives in
security-impact-review.md. Cover authn/authz, tenant isolation, input validation, and secrets.
-->

* **Authorization**: \<required scope(s) per operation.\>
* **Tenant isolation**: \<how organization scoping is enforced, and where it is enforced.\>
* **Input handling**: \<validation, size limits, untrusted-input paths.\>

# **6\. Observability**

* **Logs**: \<new log lines, at what level, with what fields (no secrets).\>
* **Metrics**: \<new metrics and their labels.\>

# **7\. Testing Plan**

<!-- Coverage by area. The concrete cases go in test-scenarios.md. -->

| Test area | Required coverage |
| :---- | :---- |
| \<area\> | \<what must be covered\> |
| \<area\> | \<...\> |

# **8\. Rollout**

* **Phasing**: \<what ships in which phase / PR.\>
* **Default state**: \<is the feature on or off out of the box.\>
* **Rollback**: \<how to disable or revert safely.\>

# **9\. Future Work**

* \<deferred item and why it is deferred.\>
