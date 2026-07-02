<h1 id="wso2-api-developer-portal-core-devportal-routes-subscription-plans">Subscription Plans</h1>

## List subscription plans

<a id="opIdlistSubscriptionPlans"></a>

`GET /subscription-plans`

> Code samples

```shell

curl -X GET https://localhost:3000/api/v0.9/subscription-plans \
  -u {username}:{password} \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}'

```

Lists subscription plans for an organization. When `name` is supplied, only the matching plan (if any) is returned. Plan names are unique within an organization.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="list-subscription-plans-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|name|query|string|false|Filter by exact plan name. Returns an array of zero or one items.|

> Example responses

> 200 Response

```json
{
  "list": [
    {
      "id": "string",
      "displayName": "string",
      "description": "string",
      "requestCount": "string",
      "refId": "string",
      "orgId": "string",
      "createdBy": "alice@example.com",
      "createdAt": "2026-05-07T08:30:00Z",
      "updatedAt": "2026-05-07T08:30:00Z"
    }
  ],
  "pagination": {
    "total": 42,
    "limit": 20,
    "offset": 0
  }
}
```

> Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.

```json
[
  {
    "status": "error",
    "code": "COMMON_VALIDATION_ERROR",
    "message": "Input validation failed.",
    "errors": [
      {
        "field": "name",
        "message": "name is required."
      }
    ]
  }
]
```

```json
{
  "status": "error",
  "code": "MISSING_REQUIRED_PARAMETER",
  "message": "Missing required parameter."
}
```

```json
{
  "message": "Missing or invalid fields in the request payload"
}
```

> 500 Response

```json
{
  "status": "error",
  "code": "INTERNAL_SERVER_ERROR",
  "message": "An unexpected error occurred."
}
```

<h3 id="list-subscription-plans-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|List of subscription plan DTOs. Empty array when no plans match.|Inline|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.|Inline|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="list-subscription-plans-responseschema">Response Schema</h3>

Status Code **200**

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|» list|[[SubscriptionPlanResponse](schemas.md#schemasubscriptionplanresponse)]|false|none|none|
|»» id|string|false|none|The plan's handle (unique per org). Not the internal database uuid.|
|»» displayName|string|false|none|none|
|»» description|string|false|none|none|
|»» requestCount|string¦null|false|none|Always stored and returned as a string ("Unlimited" or a numeric string), regardless of the type (request-count or event-count) used to create the plan. Null if not set.|
|»» refId|string¦null|false|none|Platform API subscription plan UUID associated with this plan.|
|»» orgId|string|false|none|none|
|»» createdBy|string|false|none|Identity of the user who created this subscription plan, or `deleted_user` if that user's IDP reference no longer exists. Present on single-resource GET responses and list items.|
|»» updatedBy|string|false|none|Identity of the user who last updated this subscription plan, or `deleted_user` if that user's IDP reference no longer exists. Present on single-resource GET responses only, omitted on list items.|
|»» createdAt|string(date-time)|false|none|none|
|»» updatedAt|string(date-time)|false|none|none|
|» pagination|[Pagination](schemas.md#schemapagination)|false|none|Standard pagination metadata returned with collection responses.|
|»» total|integer|true|none|Total number of records matching the query.|
|»» limit|integer|true|none|Maximum number of records returned in this response.|
|»» offset|integer|true|none|Number of records skipped before this page.|

#### Enumerated Values

|Property|Value|
|---|---|
|status|error|
|status|error|

## Create subscription plans

<a id="opIdaddSubscriptionPlans"></a>

`POST /subscription-plans`

> Code samples

```shell

curl -X POST https://localhost:3000/api/v0.9/subscription-plans \
  -u {username}:{password} \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}' \
  -d @payload.json

```

Creates one subscription plan when the request body is an object, or multiple subscription plans when the body is an array. Bulk creation returns a message instead of creating plans when `generateDefaultSubPlans` is enabled.

> Payload

```json
{
  "id": "Gold",
  "refId": "string",
  "displayName": "string",
  "description": "string",
  "type": "requestcount",
  "requestCount": 0,
  "eventCount": 0
}
```

```yaml
id: Gold
refId: string
displayName: string
description: string
type: requestcount
requestCount: 0
eventCount: 0

```

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="create-subscription-plans-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|any|true|Subscription plan payload. Send a single object for single create/upsert, or a non-empty array for bulk create/upsert. The service currently processes plans with `type` set to `requestcount` or `eventcount`. Alternatively, upload a YAML file in the `subscriptionPlan` multipart field; use `kind: SubscriptionPlan` for a single plan or `kind: SubscriptionPlanList` with an `items` array for bulk operations.|

> Example responses

> 200 Response

```json
{
  "message": "string"
}
```

> Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.

```json
[
  {
    "status": "error",
    "code": "COMMON_VALIDATION_ERROR",
    "message": "Input validation failed.",
    "errors": [
      {
        "field": "name",
        "message": "name is required."
      }
    ]
  }
]
```

```json
{
  "status": "error",
  "code": "MISSING_REQUIRED_PARAMETER",
  "message": "Missing required parameter."
}
```

```json
{
  "message": "Missing or invalid fields in the request payload"
}
```

> 409 Response

```json
{
  "status": "error",
  "code": "CONFLICT",
  "message": "Conflict"
}
```

> 500 Response

```json
{
  "status": "error",
  "code": "INTERNAL_SERVER_ERROR",
  "message": "An unexpected error occurred."
}
```

<h3 id="create-subscription-plans-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|JSON message response.|[MessageResponse](schemas.md#schemamessageresponse)|
|201|[Created](https://tools.ietf.org/html/rfc7231#section-6.3.2)|Subscription plan create/update response for single or bulk operations.|Inline|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.|Inline|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|The request conflicts with an existing resource.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="create-subscription-plans-responseschema">Response Schema</h3>

#### Enumerated Values

|Property|Value|
|---|---|
|status|error|
|status|error|

## Upsert subscription plans

<a id="opIdputSubscriptionPlans"></a>

`PUT /subscription-plans`

> Code samples

```shell

curl -X PUT https://localhost:3000/api/v0.9/subscription-plans \
  -u {username}:{password} \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}' \
  -d @payload.json

```

Upserts one subscription plan when the request body is an object, or multiple plans when the body is an array. A single plan update returns `200` when an existing plan is updated and `201` when a new plan is created. Bulk updates return a message when `generateDefaultSubPlans` is enabled.

> Payload

```json
{
  "id": "Gold",
  "refId": "string",
  "displayName": "string",
  "description": "string",
  "type": "requestcount",
  "requestCount": 0,
  "eventCount": 0
}
```

```yaml
id: Gold
refId: string
displayName: string
description: string
type: requestcount
requestCount: 0
eventCount: 0

```

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="upsert-subscription-plans-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|any|true|Subscription plan payload. Send a single object for single create/upsert, or a non-empty array for bulk create/upsert. The service currently processes plans with `type` set to `requestcount` or `eventcount`. Alternatively, upload a YAML file in the `subscriptionPlan` multipart field; use `kind: SubscriptionPlan` for a single plan or `kind: SubscriptionPlanList` with an `items` array for bulk operations.|

> Example responses

> 200 Response

```json
{
  "id": "string",
  "displayName": "string",
  "description": "string",
  "requestCount": "string",
  "refId": "string",
  "orgId": "string",
  "createdBy": "alice@example.com",
  "updatedBy": "alice@example.com",
  "createdAt": "2019-08-24T14:15:22Z",
  "updatedAt": "2019-08-24T14:15:22Z"
}
```

> Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.

```json
[
  {
    "status": "error",
    "code": "COMMON_VALIDATION_ERROR",
    "message": "Input validation failed.",
    "errors": [
      {
        "field": "name",
        "message": "name is required."
      }
    ]
  }
]
```

```json
{
  "status": "error",
  "code": "MISSING_REQUIRED_PARAMETER",
  "message": "Missing required parameter."
}
```

```json
{
  "message": "Missing or invalid fields in the request payload"
}
```

> 404 Response

```json
{
  "status": "error",
  "code": "ORG_NOT_FOUND",
  "message": "Organization not found."
}
```

> 409 Response

```json
{
  "status": "error",
  "code": "CONFLICT",
  "message": "Conflict"
}
```

> 500 Response

```json
{
  "status": "error",
  "code": "INTERNAL_SERVER_ERROR",
  "message": "An unexpected error occurred."
}
```

<h3 id="upsert-subscription-plans-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Subscription plan update response. Bulk updates may return a list, and some configurations return a message.|Inline|
|201|[Created](https://tools.ietf.org/html/rfc7231#section-6.3.2)|Subscription plan create/update response for single or bulk operations.|Inline|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.|Inline|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Resource not found.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|The request conflicts with an existing resource.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="upsert-subscription-plans-responseschema">Response Schema</h3>

#### Enumerated Values

|Property|Value|
|---|---|
|status|error|
|status|error|

## Get a subscription plan

<a id="opIdgetSubscriptionPlan"></a>

`GET /subscription-plans/{planId}`

> Code samples

```shell

curl -X GET https://localhost:3000/api/v0.9/subscription-plans/{planId} \
  -u {username}:{password} \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}'

```

Retrieves a single subscription plan by `planId`.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="get-a-subscription-plan-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|planId|path|string|true|The subscription plan's handle (unique per org).|

> Example responses

> 200 Response

```json
{
  "id": "string",
  "displayName": "string",
  "description": "string",
  "requestCount": "string",
  "refId": "string",
  "orgId": "string",
  "createdBy": "alice@example.com",
  "updatedBy": "alice@example.com",
  "createdAt": "2019-08-24T14:15:22Z",
  "updatedAt": "2019-08-24T14:15:22Z"
}
```

> Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.

```json
[
  {
    "status": "error",
    "code": "COMMON_VALIDATION_ERROR",
    "message": "Input validation failed.",
    "errors": [
      {
        "field": "name",
        "message": "name is required."
      }
    ]
  }
]
```

```json
{
  "status": "error",
  "code": "MISSING_REQUIRED_PARAMETER",
  "message": "Missing required parameter."
}
```

```json
{
  "message": "Missing or invalid fields in the request payload"
}
```

> 404 Response

```json
{
  "status": "error",
  "code": "ORG_NOT_FOUND",
  "message": "Organization not found."
}
```

> 500 Response

```json
{
  "status": "error",
  "code": "INTERNAL_SERVER_ERROR",
  "message": "An unexpected error occurred."
}
```

<h3 id="get-a-subscription-plan-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Subscription plan DTO.|[SubscriptionPlanResponse](schemas.md#schemasubscriptionplanresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.|Inline|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Resource not found.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="get-a-subscription-plan-responseschema">Response Schema</h3>

#### Enumerated Values

|Property|Value|
|---|---|
|status|error|
|status|error|

## Delete a subscription plan

<a id="opIddeleteSubscriptionPlan"></a>

`DELETE /subscription-plans/{planId}`

> Code samples

```shell

curl -X DELETE https://localhost:3000/api/v0.9/subscription-plans/{planId} \
  -u {username}:{password} \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}'

```

Deletes a subscription plan by `planId`.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="delete-a-subscription-plan-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|planId|path|string|true|The subscription plan's handle (unique per org).|

> Example responses

> Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.

```json
[
  {
    "status": "error",
    "code": "COMMON_VALIDATION_ERROR",
    "message": "Input validation failed.",
    "errors": [
      {
        "field": "name",
        "message": "name is required."
      }
    ]
  }
]
```

```json
{
  "status": "error",
  "code": "MISSING_REQUIRED_PARAMETER",
  "message": "Missing required parameter."
}
```

```json
{
  "message": "Missing or invalid fields in the request payload"
}
```

> 404 Response

```json
{
  "status": "error",
  "code": "ORG_NOT_FOUND",
  "message": "Organization not found."
}
```

> 500 Response

```json
{
  "status": "error",
  "code": "INTERNAL_SERVER_ERROR",
  "message": "An unexpected error occurred."
}
```

<h3 id="delete-a-subscription-plan-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|204|[No Content](https://tools.ietf.org/html/rfc7231#section-6.3.5)|Subscription plan deleted successfully.|None|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.|Inline|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Resource not found.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="delete-a-subscription-plan-responseschema">Response Schema</h3>

#### Enumerated Values

|Property|Value|
|---|---|
|status|error|
|status|error|
