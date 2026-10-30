<h1 id="gateway-controller-management-api-audit-log-management">Audit Log Management</h1>

Read-only access to the gateway audit log trail

## List audit events

<a id="opIdlistAuditLogs"></a>

`GET /audit-logs`

> Code samples

```shell

curl -X GET http://localhost:9090/api/management/v1/audit-logs \
  -u {username}:{password} \
  -H 'Accept: application/json'

```

Returns a paginated, filtered slice of this gateway's audit-log trail.
Results are always scoped to the gateway's own ID — no caller-supplied
gateway filter is accepted.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

Required roles: `admin`

</aside>

<h3 id="list-audit-events-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|startTime|query|string(date-time)|false|Inclusive lower bound on performedAt (RFC 3339 UTC).|
|endTime|query|string(date-time)|false|Inclusive upper bound on performedAt (RFC 3339 UTC). Must be after startTime when both are supplied.|
|performedBy|query|string|false|Filter by actor identifier.|
|resourceId|query|string|false|Exact match on the target resource identifier.|
|action|query|[AuditAction](schemas.md#schemaauditaction)|false|Filter by action verb.|
|resourceType|query|[AuditResourceType](schemas.md#schemaauditresourcetype)|false|Filter by target resource type.|
|outcome|query|[AuditOutcome](schemas.md#schemaauditoutcome)|false|Filter by outcome.|
|limit|query|integer(int32)|false|Maximum number of events to return (1-100, default 20).|
|offset|query|integer(int32)|false|Zero-based offset for pagination.|

#### Enumerated Values

|Parameter|Value|
|---|---|
|action|create|
|action|update|
|action|delete|
|action|revoke|
|resourceType|RestApi|
|resourceType|GraphQLApi|
|resourceType|LlmProvider|
|resourceType|LlmProviderTemplate|
|resourceType|LlmProxy|
|resourceType|McpProxy|
|resourceType|Agent|
|resourceType|ApiKey|
|resourceType|SubscriptionPlan|
|resourceType|Subscription|
|resourceType|Certificate|
|resourceType|Secret|
|outcome|succeeded|
|outcome|failed|
|outcome|denied|

> Example responses
>
> 200 Response

```json
{
  "count": 20,
  "events": [
    {
      "id": "01932c70-3f8a-7000-8000-abcdef123456",
      "correlationId": "string",
      "action": "create",
      "resourceType": "RestApi",
      "resourceId": "string",
      "resourceName": "string",
      "performedBy": "string",
      "performedAt": "2026-10-11T18:42:03.419Z",
      "outcome": "succeeded",
      "eventData": {}
    }
  ],
  "pagination": {
    "total": 50,
    "offset": 0,
    "limit": 20
  }
}
```

<h3 id="list-audit-events-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Filtered audit-event slice.|[AuditEventListResponse](schemas.md#schemaauditeventlistresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Invalid filter parameter|[ErrorResponse](schemas.md#schemaerrorresponse)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized - authentication required|[ErrorResponse](schemas.md#schemaerrorresponse)|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Forbidden - insufficient role|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|
