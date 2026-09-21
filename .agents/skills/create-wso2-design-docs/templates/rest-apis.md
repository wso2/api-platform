# **REST APIs**

<!--
The API contract for the feature: every new endpoint and every change to an existing one.
Include enough that a client can be written against this file alone. Repeat the "Endpoint"
block below once per endpoint.
-->

# 1. Summary of Changes

| Endpoint | Method | Change | Breaking? |
| :---- | :---- | :---- | :---- |
| \<path\> | \<GET/POST/PUT/DELETE\> | New / Modified | Yes / No |

# 2. \<Endpoint title\>

\<One sentence on what this endpoint is for and who calls it.\>

```
<METHOD> /api/<component>/v<n>/<resource>
```

**Audience**: Public / Internal (component-to-component) / Admin

**Authorization**

| | |
| :---- | :---- |
| Authentication | \<OAuth2 bearer / API key / mTLS\> |
| Required scope(s) | `\<ap:resource:action\>` |
| Tenant scoping | \<how the organization is resolved — always from verified claims, never from the payload\> |

**Request**

```
Headers:
  Authorization: Bearer <token>
  Content-Type: application/json

Path parameters:
  <name>  <type>  <description>

Query parameters:
  <name>  <type>  <required?>  <default>  <description>
```

```json
{
  "<field>": "<value>"
}
```

| Field | Type | Required | Description |
| :---- | :---- | :---- | :---- |
| \<field\> | \<type\> | Yes / No | \<description, constraints, max length\> |

**Response** — `200 OK`

```json
{
  "<field>": "<value>"
}
```

**Error responses**

<!-- Keep client-facing messages generic; the specific reason belongs in the internal log only. -->

| Status | Condition | Body |
| :---- | :---- | :---- |
| 400 | \<invalid input\> | \<generic error object\> |
| 401 | \<missing/invalid credentials\> | `{"error": "unauthorized", "message": "Invalid or expired credentials."}` |
| 403 | \<scope or ownership check failed\> | \<generic error object\> |
| 404 | \<resource not found\> | \<generic error object\> |
| 409 | \<conflicting state\> | \<generic error object\> |

**Notes**

* \<pagination, sorting, idempotency, rate limits, size caps.\>
* \<why any field is intentionally loosely typed / extensible.\>

# 3. Changes to Existing Endpoints

<!-- For a modified endpoint, show only the delta and state the compatibility impact. -->

\<Endpoint\> — \<what is added or changed.\>

```json
{
  "<newField>": "<example>"
}
```

* **Backward compatibility**: \<is the new field optional; what an old client sees.\>
* **Behaviour when omitted**: \<explicit — omitted vs. empty vs. present.\>

# 4. OpenAPI Spec

* Spec file(s) updated: \<path\>
* Scopes declared in the spec: \<list\>
* CLI sync required: Yes / No \<if yes, which command family\>
