<h1 id="wso2-api-developer-portal-core-devportal-routes-subscription-policies">Subscription Policies</h1>

## List subscription policies

<a id="opIdlistSubscriptionPolicies"></a>

`GET /o/{orgId}/devportal/v1/subscription-policies`

> Code samples

```shell

curl -X GET https://devportal.api-platform.io/o/{orgId}/devportal/v1/subscription-policies \
  -u {username}:{password} \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}'

```

Lists subscription policies for an organization. When `name` is supplied, only the matching policy (if any) is returned. Policy names are unique within an organization.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="list-subscription-policies-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|name|query|string|false|Filter by exact policy name. Returns an array of zero or one items.|
|orgId|path|string|true|none|

> Example responses

> 200 Response

```json
[
  {
    "policyID": "string",
    "policyName": "string",
    "displayName": "string",
    "description": "string",
    "requestCount": 0,
    "orgID": "string"
  }
]
```

> Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.

```json
[
  {
    "code": "400",
    "message": "input validation failed",
    "description": "Invalid value"
  }
]
```

```json
{
  "code": "400",
  "message": "Bad Request",
  "description": "Missing required parameter: 'orgId'"
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
  "code": "500",
  "message": "Internal Server Error",
  "description": "Internal Server Error"
}
```

<h3 id="list-subscription-policies-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|List of subscription policy DTOs. Empty array when no policies match.|Inline|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.|Inline|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="list-subscription-policies-responseschema">Response Schema</h3>

Status Code **200**

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|[[SubscriptionPolicyResponse](schemas.md#schemasubscriptionpolicyresponse)]|false|none|none|
|» policyID|string|false|none|none|
|» policyName|string|false|none|none|
|» displayName|string|false|none|none|
|» description|string|false|none|none|
|» requestCount|any|false|none|none|

*oneOf*

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|»» *anonymous*|integer|false|none|none|

*xor*

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|»» *anonymous*|string|false|none|none|

*continued*

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|» orgID|string|false|none|none|

## Create subscription policies

<a id="opIdaddSubscriptionPolicies"></a>

`POST /o/{orgId}/devportal/v1/subscription-policies`

> Code samples

```shell

curl -X POST https://devportal.api-platform.io/o/{orgId}/devportal/v1/subscription-policies \
  -u {username}:{password} \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}' \
  -d @payload.json

```

Creates one subscription policy when the request body is an object, or multiple subscription policies when the body is an array. Bulk creation returns a message instead of creating policies when `generateDefaultSubPolicies` is enabled.

> Payload

```json
{
  "policyId": "string",
  "policyID": "string",
  "refId": "string",
  "policyName": "string",
  "displayName": "string",
  "description": "string",
  "type": "requestcount",
  "requestCount": 0,
  "eventCount": 0
}
```

```yaml
policyId: string
policyID: string
refId: string
policyName: string
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

<h3 id="create-subscription-policies-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|any|true|Subscription policy payload. Send a single object for single create/upsert, or a non-empty array for bulk create/upsert. The service currently processes policies with `type` set to `requestcount` or `eventcount`. Alternatively, upload a YAML file in the `subscriptionPolicy` multipart field; use `kind: SubscriptionPolicy` for a single policy or `kind: SubscriptionPolicyList` with an `items` array for bulk operations.|
|orgId|path|string|true|none|

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
    "code": "400",
    "message": "input validation failed",
    "description": "Invalid value"
  }
]
```

```json
{
  "code": "400",
  "message": "Bad Request",
  "description": "Missing required parameter: 'orgId'"
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
  "code": "409",
  "message": "Conflict",
  "description": "Organization already exists"
}
```

> 500 Response

```json
{
  "code": "500",
  "message": "Internal Server Error",
  "description": "Internal Server Error"
}
```

<h3 id="create-subscription-policies-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|JSON message response.|[MessageResponse](schemas.md#schemamessageresponse)|
|201|[Created](https://tools.ietf.org/html/rfc7231#section-6.3.2)|Subscription policy create/update response for single or bulk operations.|Inline|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.|Inline|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Duplicate organization data conflicts with an existing record.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="create-subscription-policies-responseschema">Response Schema</h3>

## Upsert subscription policies

<a id="opIdputSubscriptionPolicies"></a>

`PUT /o/{orgId}/devportal/v1/subscription-policies`

> Code samples

```shell

curl -X PUT https://devportal.api-platform.io/o/{orgId}/devportal/v1/subscription-policies \
  -u {username}:{password} \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}' \
  -d @payload.json

```

Upserts one subscription policy when the request body is an object, or multiple policies when the body is an array. A single policy update returns `200` when an existing policy is updated and `201` when a new policy is created. Bulk updates return a message when `generateDefaultSubPolicies` is enabled.

> Payload

```json
{
  "policyId": "string",
  "policyID": "string",
  "refId": "string",
  "policyName": "string",
  "displayName": "string",
  "description": "string",
  "type": "requestcount",
  "requestCount": 0,
  "eventCount": 0
}
```

```yaml
policyId: string
policyID: string
refId: string
policyName: string
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

<h3 id="upsert-subscription-policies-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|any|true|Subscription policy payload. Send a single object for single create/upsert, or a non-empty array for bulk create/upsert. The service currently processes policies with `type` set to `requestcount` or `eventcount`. Alternatively, upload a YAML file in the `subscriptionPolicy` multipart field; use `kind: SubscriptionPolicy` for a single policy or `kind: SubscriptionPolicyList` with an `items` array for bulk operations.|
|orgId|path|string|true|none|

> Example responses

> 200 Response

```json
{
  "policyID": "string",
  "policyName": "string",
  "displayName": "string",
  "description": "string",
  "requestCount": 0,
  "orgID": "string"
}
```

> Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.

```json
[
  {
    "code": "400",
    "message": "input validation failed",
    "description": "Invalid value"
  }
]
```

```json
{
  "code": "400",
  "message": "Bad Request",
  "description": "Missing required parameter: 'orgId'"
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
  "code": "404",
  "message": "Resource Not Found",
  "description": "Organization not found"
}
```

> 409 Response

```json
{
  "code": "409",
  "message": "Conflict",
  "description": "Organization already exists"
}
```

> 500 Response

```json
{
  "code": "500",
  "message": "Internal Server Error",
  "description": "Internal Server Error"
}
```

<h3 id="upsert-subscription-policies-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Subscription policy update response. Bulk updates may return a list, and some configurations return a message.|Inline|
|201|[Created](https://tools.ietf.org/html/rfc7231#section-6.3.2)|Subscription policy create/update response for single or bulk operations.|Inline|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.|Inline|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Resource not found.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Duplicate organization data conflicts with an existing record.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="upsert-subscription-policies-responseschema">Response Schema</h3>

## Get a subscription policy

<a id="opIdgetSubscriptionPolicy"></a>

`GET /o/{orgId}/devportal/v1/subscription-policies/{policyId}`

> Code samples

```shell

curl -X GET https://devportal.api-platform.io/o/{orgId}/devportal/v1/subscription-policies/{policyId} \
  -u {username}:{password} \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}'

```

Retrieves a single subscription policy by `policyId`.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="get-a-subscription-policy-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|orgId|path|string|true|none|
|policyId|path|string|true|none|

> Example responses

> 200 Response

```json
{
  "policyID": "string",
  "policyName": "string",
  "displayName": "string",
  "description": "string",
  "requestCount": 0,
  "orgID": "string"
}
```

> Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.

```json
[
  {
    "code": "400",
    "message": "input validation failed",
    "description": "Invalid value"
  }
]
```

```json
{
  "code": "400",
  "message": "Bad Request",
  "description": "Missing required parameter: 'orgId'"
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
  "code": "404",
  "message": "Resource Not Found",
  "description": "Organization not found"
}
```

> 500 Response

```json
{
  "code": "500",
  "message": "Internal Server Error",
  "description": "Internal Server Error"
}
```

<h3 id="get-a-subscription-policy-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Subscription policy DTO.|[SubscriptionPolicyResponse](schemas.md#schemasubscriptionpolicyresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.|Inline|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Resource not found.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="get-a-subscription-policy-responseschema">Response Schema</h3>

## Delete a subscription policy

<a id="opIddeleteSubscriptionPolicy"></a>

`DELETE /o/{orgId}/devportal/v1/subscription-policies/{policyId}`

> Code samples

```shell

curl -X DELETE https://devportal.api-platform.io/o/{orgId}/devportal/v1/subscription-policies/{policyId} \
  -u {username}:{password} \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}'

```

Deletes a subscription policy by `policyId`.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="delete-a-subscription-policy-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|orgId|path|string|true|none|
|policyId|path|string|true|none|

> Example responses

> Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.

```json
[
  {
    "code": "400",
    "message": "input validation failed",
    "description": "Invalid value"
  }
]
```

```json
{
  "code": "400",
  "message": "Bad Request",
  "description": "Missing required parameter: 'orgId'"
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
  "code": "404",
  "message": "Resource Not Found",
  "description": "Organization not found"
}
```

> 500 Response

```json
{
  "code": "500",
  "message": "Internal Server Error",
  "description": "Internal Server Error"
}
```

<h3 id="delete-a-subscription-policy-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|204|[No Content](https://tools.ietf.org/html/rfc7231#section-6.3.5)|Subscription policy deleted successfully.|None|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.|Inline|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Resource not found.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="delete-a-subscription-policy-responseschema">Response Schema</h3>
