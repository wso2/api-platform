<h1 id="wso2-api-developer-portal-core-devportal-routes-subscriptions">Subscriptions</h1>

## Create a subscription

<a id="opIdcreateSubscription"></a>

`POST /organizations/{orgId}/subscriptions`

> Code samples

```shell

curl -X POST http://localhost:3000/devportal/organizations/{orgId}/subscriptions \
  -u {username}:{password} \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}' \
  -d @payload.json

```

Creates a token-based subscription for an API. The Developer Portal API ID is resolved to the control-plane API reference before the subscription is created. The API must exist, have token-based subscriptions enabled, and contain a control-plane reference ID.

> Payload

```json
{
  "apiId": "api-7f4c2a6b"
}
```

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="create-a-subscription-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|[SubscriptionCreateRequest](schemas.md#schemasubscriptioncreaterequest)|true|Subscription creation payload. `apiId` is the Developer Portal API ID; the service resolves it to the control-plane API reference before forwarding the request.|
|orgId|path|string|true|none|

> Example responses

> 201 Response

```json
{
  "subscriptionId": "sub-12345",
  "apiId": "cp-api-12345",
  "apiName": "Weather API",
  "applicationId": "cp-app-98765",
  "subscriptionPlanName": "Gold",
  "status": "ACTIVE",
  "createdTime": "2026-05-07T08:30:00Z"
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

<h3 id="create-a-subscription-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|201|[Created](https://tools.ietf.org/html/rfc7231#section-6.3.2)|Subscription DTO returned by the control plane.|[SubscriptionResponse](schemas.md#schemasubscriptionresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.|Inline|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Resource not found.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="create-a-subscription-responseschema">Response Schema</h3>

## List subscriptions

<a id="opIdlistSubscriptions"></a>

`GET /organizations/{orgId}/subscriptions`

> Code samples

```shell

curl -X GET http://localhost:3000/devportal/organizations/{orgId}/subscriptions \
  -u {username}:{password} \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}'

```

Lists subscriptions from the control plane. When `apiId` is provided, the Developer Portal API ID is validated locally and translated to the control-plane API reference.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="list-subscriptions-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|apiId|query|string|false|Optional Developer Portal API ID used to filter by the resolved control-plane API reference.|
|orgId|path|string|true|none|

> Example responses

> 200 Response

```json
{
  "count": 1,
  "list": [
    {
      "subscriptionId": "sub-12345",
      "apiId": "cp-api-12345",
      "apiName": "Weather API",
      "applicationId": "cp-app-98765",
      "subscriptionPlanName": "Gold",
      "status": "ACTIVE"
    }
  ]
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

<h3 id="list-subscriptions-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|List of subscription DTOs returned by the control plane.|Inline|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.|Inline|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Resource not found.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="list-subscriptions-responseschema">Response Schema</h3>

#### Enumerated Values

|Property|Value|
|---|---|
|status|ACTIVE|
|status|INACTIVE|
|status|ON_HOLD|
|status|REJECTED|
|status|TIER_UPDATE_PENDING|

## Get a subscription

<a id="opIdgetSubscription"></a>

`GET /organizations/{orgId}/subscriptions/{subscriptionId}`

> Code samples

```shell

curl -X GET http://localhost:3000/devportal/organizations/{orgId}/subscriptions/{subscriptionId} \
  -u {username}:{password} \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}'

```

Retrieves a single subscription from the control plane by subscription ID.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="get-a-subscription-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|orgId|path|string|true|none|
|subscriptionId|path|string|true|none|

> Example responses

> 200 Response

```json
{
  "subscriptionId": "sub-12345",
  "apiId": "cp-api-12345",
  "apiName": "Weather API",
  "applicationId": "cp-app-98765",
  "subscriptionPlanName": "Gold",
  "status": "ACTIVE",
  "createdTime": "2026-05-07T08:30:00Z"
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

<h3 id="get-a-subscription-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Subscription DTO returned by the control plane.|[SubscriptionResponse](schemas.md#schemasubscriptionresponse)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Resource not found.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

## Update a subscription

<a id="opIdupdateSubscription"></a>

`PUT /organizations/{orgId}/subscriptions/{subscriptionId}`

> Code samples

```shell

curl -X PUT http://localhost:3000/devportal/organizations/{orgId}/subscriptions/{subscriptionId} \
  -u {username}:{password} \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}' \
  -d @payload.json

```

Updates the subscription status in the control plane. The service accepts only `ACTIVE` or `INACTIVE`.

> Payload

```json
{
  "status": "ACTIVE"
}
```

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="update-a-subscription-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|[SubscriptionUpdateRequest](schemas.md#schemasubscriptionupdaterequest)|true|Subscription status update payload.|
|orgId|path|string|true|none|
|subscriptionId|path|string|true|none|

> Example responses

> 200 Response

```json
{
  "subscriptionId": "sub-12345",
  "apiId": "cp-api-12345",
  "apiName": "Weather API",
  "applicationId": "cp-app-98765",
  "subscriptionPlanName": "Gold",
  "status": "ACTIVE",
  "createdTime": "2026-05-07T08:30:00Z"
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

<h3 id="update-a-subscription-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Subscription DTO returned by the control plane.|[SubscriptionResponse](schemas.md#schemasubscriptionresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.|Inline|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Resource not found.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="update-a-subscription-responseschema">Response Schema</h3>

## Delete a subscription

<a id="opIddeleteSubscription"></a>

`DELETE /organizations/{orgId}/subscriptions/{subscriptionId}`

> Code samples

```shell

curl -X DELETE http://localhost:3000/devportal/organizations/{orgId}/subscriptions/{subscriptionId} \
  -u {username}:{password} \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}'

```

Deletes the subscription in the control plane and returns a success message from the Developer Portal.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="delete-a-subscription-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|orgId|path|string|true|none|
|subscriptionId|path|string|true|none|

> Example responses

> 200 Response

```json
{
  "message": "string"
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

<h3 id="delete-a-subscription-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|JSON message response.|[MessageResponse](schemas.md#schemamessageresponse)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Resource not found.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|
