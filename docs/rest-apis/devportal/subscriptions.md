<h1 id="wso2-api-developer-portal-core-devportal-routes-subscriptions">Subscriptions</h1>

## Create a subscription

<a id="opIdcreateSubscription"></a>

`POST /subscriptions`

> Code samples

```shell

curl -X POST https://localhost:3000/api/v0.9/subscriptions \
  -u {username}:{password} \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}' \
  -d @payload.json

```

Creates a subscription for an API. The API must exist in the Developer Portal and have subscription plans enabled. The subscription is owned by the authenticated user.

> Payload

```json
{
  "apiId": "weather-api-v1",
  "subscriptionPlanId": "Gold"
}
```

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="create-a-subscription-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|[SubscriptionCreateRequest](schemas.md#schemasubscriptioncreaterequest)|true|Subscription creation payload. `apiId` is the Developer Portal API ID.|

> Example responses

> 201 Response

```json
{
  "subscriptionId": "sub-12345",
  "apiId": "weather-api-v1",
  "subscriptionToken": "a3f1e8b2c4d6e8f0a1b3c5d7e9f10b2c4d6e8f0a1b3c5d7e9f10b2c4d6e8f0a1",
  "subscriptionPlanName": "Gold",
  "status": "ACTIVE",
  "createdBy": "alice@example.com",
  "createdAt": "2026-05-07T08:30:00Z"
}
```

> 400 Response

```json
{
  "code": "400",
  "message": "Bad Request",
  "description": "Invalid request parameters"
}
```

> 404 Response

```json
{
  "code": "404",
  "message": "Not Found",
  "description": "Subscription not found"
}
```

> 409 Response

```json
{
  "code": "409",
  "message": "Conflict",
  "description": "A subscription for this API already exists"
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

<h3 id="create-a-subscription-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|201|[Created](https://tools.ietf.org/html/rfc7231#section-6.3.2)|Subscription DTO.|[SubscriptionResponse](schemas.md#schemasubscriptionresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request.|[SimpleErrorResponse](schemas.md#schemasimpleerrorresponse)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Resource not found.|[SimpleErrorResponse](schemas.md#schemasimpleerrorresponse)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|The request conflicts with an existing resource.|[SimpleErrorResponse](schemas.md#schemasimpleerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

### Response Headers

|Status|Header|Type|Format|Description|
|---|---|---|---|---|
|201|Location|string|uri|URL of the created subscription.|

## List subscriptions

<a id="opIdlistSubscriptions"></a>

`GET /subscriptions`

> Code samples

```shell

curl -X GET https://localhost:3000/api/v0.9/subscriptions \
  -u {username}:{password} \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}'

```

Lists subscriptions owned by the authenticated user. When `apiId` is provided, results are additionally filtered by the Developer Portal API ID.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="list-subscriptions-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|apiId|query|string|false|Optional Developer Portal API ID used to filter results.|
|limit|query|integer|false|Maximum number of records to return.|
|offset|query|integer|false|Number of records to skip before returning results.|

> Example responses

> 200 Response

```json
{
  "list": [
    {
      "subscriptionId": "sub-12345",
      "apiId": "weather-api-v1",
      "subscriptionToken": "a3f1e8b2c4d6e8f0a1b3c5d7e9f10b2c4d6e8f0a1b3c5d7e9f10b2c4d6e8f0a1",
      "subscriptionPlanName": "Gold",
      "status": "ACTIVE",
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

> 404 Response

```json
{
  "code": "404",
  "message": "Not Found",
  "description": "API not found"
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

<h3 id="list-subscriptions-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|List of subscription DTOs.|Inline|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Returned when `apiId` is provided but does not match an existing API.|[SimpleErrorResponse](schemas.md#schemasimpleerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="list-subscriptions-responseschema">Response Schema</h3>

Status Code **200**

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|» list|[[SubscriptionResponse](schemas.md#schemasubscriptionresponse)]|false|none|[Subscription payload.]|
|»» subscriptionId|string|false|none|none|
|»» apiId|string|false|none|Developer Portal API ID.|
|»» subscriptionToken|string¦null|false|none|Plaintext subscription token, decrypted on every read (not just on create). Null if decryption fails (e.g. the encryption key changed since the token was stored).|
|»» subscriptionPlanName|string|false|none|none|
|»» status|string|false|none|none|
|»» createdBy|string|false|none|Identity of the user who created the subscription, or `deleted_user` if that user's IDP reference no longer exists. Present on single-resource GET responses and list items.|
|»» updatedBy|string|false|none|Identity of the user who last updated the subscription, or `deleted_user` if that user's IDP reference no longer exists. Present on single-resource GET responses only, omitted on list items.|
|»» createdAt|string(date-time)|false|none|none|
|»» updatedAt|string(date-time)|false|none|none|
|» pagination|[Pagination](schemas.md#schemapagination)|false|none|Standard pagination metadata returned with collection responses.|
|»» total|integer|true|none|Total number of records matching the query.|
|»» limit|integer|true|none|Maximum number of records returned in this response.|
|»» offset|integer|true|none|Number of records skipped before this page.|

#### Enumerated Values

|Property|Value|
|---|---|
|status|ACTIVE|
|status|INACTIVE|

## Get a subscription

<a id="opIdgetSubscription"></a>

`GET /subscriptions/{subId}`

> Code samples

```shell

curl -X GET https://localhost:3000/api/v0.9/subscriptions/{subId} \
  -u {username}:{password} \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}'

```

Retrieves a single subscription by subscription ID.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="get-a-subscription-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|subId|path|string|true|none|

> Example responses

> 200 Response

```json
{
  "subscriptionId": "sub-12345",
  "apiId": "weather-api-v1",
  "subscriptionToken": "a3f1e8b2c4d6e8f0a1b3c5d7e9f10b2c4d6e8f0a1b3c5d7e9f10b2c4d6e8f0a1",
  "subscriptionPlanName": "Gold",
  "status": "ACTIVE",
  "createdBy": "alice@example.com",
  "updatedBy": "alice@example.com",
  "createdAt": "2026-05-07T08:30:00Z",
  "updatedAt": "2026-05-07T08:30:00Z"
}
```

> 404 Response

```json
{
  "code": "404",
  "message": "Not Found",
  "description": "Subscription not found"
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

<h3 id="get-a-subscription-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Subscription DTO.|[SubscriptionResponse](schemas.md#schemasubscriptionresponse)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Resource not found.|[SimpleErrorResponse](schemas.md#schemasimpleerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

## Update a subscription

<a id="opIdupdateSubscription"></a>

`PUT /subscriptions/{subId}`

> Code samples

```shell

curl -X PUT https://localhost:3000/api/v0.9/subscriptions/{subId} \
  -u {username}:{password} \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}' \
  -d @payload.json

```

Updates the subscription status. Accepts only `ACTIVE` or `INACTIVE`.

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
|subId|path|string|true|none|

> Example responses

> 200 Response

```json
{
  "subscriptionId": "sub-12345",
  "apiId": "weather-api-v1",
  "subscriptionToken": "a3f1e8b2c4d6e8f0a1b3c5d7e9f10b2c4d6e8f0a1b3c5d7e9f10b2c4d6e8f0a1",
  "subscriptionPlanName": "Gold",
  "status": "ACTIVE",
  "createdBy": "alice@example.com",
  "updatedBy": "alice@example.com",
  "createdAt": "2026-05-07T08:30:00Z",
  "updatedAt": "2026-05-07T08:30:00Z"
}
```

> 400 Response

```json
{
  "code": "400",
  "message": "Bad Request",
  "description": "Invalid request parameters"
}
```

> 404 Response

```json
{
  "code": "404",
  "message": "Not Found",
  "description": "Subscription not found"
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

<h3 id="update-a-subscription-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Subscription DTO.|[SubscriptionResponse](schemas.md#schemasubscriptionresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request.|[SimpleErrorResponse](schemas.md#schemasimpleerrorresponse)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Resource not found.|[SimpleErrorResponse](schemas.md#schemasimpleerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

## Delete a subscription

<a id="opIddeleteSubscription"></a>

`DELETE /subscriptions/{subId}`

> Code samples

```shell

curl -X DELETE https://localhost:3000/api/v0.9/subscriptions/{subId} \
  -u {username}:{password} \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}'

```

Deletes the subscription and returns a success message.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="delete-a-subscription-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|subId|path|string|true|none|

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
  "message": "Not Found",
  "description": "Subscription not found"
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

<h3 id="delete-a-subscription-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|JSON message response.|[MessageResponse](schemas.md#schemamessageresponse)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Resource not found.|[SimpleErrorResponse](schemas.md#schemasimpleerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

## Change subscription plan

<a id="opIdchangePlan"></a>

`POST /subscriptions/{subId}/change-plan`

> Code samples

```shell

curl -X POST https://localhost:3000/api/v0.9/subscriptions/{subId}/change-plan \
  -u {username}:{password} \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}' \
  -d @payload.json

```

Changes the subscription plan in-place. The subscription UUID and token remain unchanged; only the plan is updated. A `subscription.plan_changed` webhook event is published to the organization's configured webhook subscribers.

> Payload

```json
{
  "apiId": "weather-api-v1",
  "planId": "Gold"
}
```

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="change-subscription-plan-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|[SubscriptionChangePlanRequest](schemas.md#schemasubscriptionchangeplanrequest)|true|Subscription plan change payload. `planId` is the Developer Portal subscription plan ID.|
|subId|path|string|true|none|

> Example responses

> 200 Response

```json
{
  "subscriptionId": "sub-12345",
  "apiId": "weather-api-v1",
  "subscriptionToken": "a3f1e8b2c4d6e8f0a1b3c5d7e9f10b2c4d6e8f0a1b3c5d7e9f10b2c4d6e8f0a1",
  "subscriptionPlanName": "Gold",
  "status": "ACTIVE",
  "createdBy": "alice@example.com",
  "updatedBy": "alice@example.com",
  "createdAt": "2026-05-07T08:30:00Z",
  "updatedAt": "2026-05-07T08:30:00Z"
}
```

> 400 Response

```json
{
  "code": "400",
  "message": "Bad Request",
  "description": "Invalid request parameters"
}
```

> 404 Response

```json
{
  "code": "404",
  "message": "Not Found",
  "description": "Subscription not found"
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

<h3 id="change-subscription-plan-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Subscription DTO.|[SubscriptionResponse](schemas.md#schemasubscriptionresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request.|[SimpleErrorResponse](schemas.md#schemasimpleerrorresponse)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Resource not found.|[SimpleErrorResponse](schemas.md#schemasimpleerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

## Regenerate subscription token

<a id="opIdregenerateSubscriptionToken"></a>

`POST /subscriptions/{subId}/regenerate-token`

> Code samples

```shell

curl -X POST https://localhost:3000/api/v0.9/subscriptions/{subId}/regenerate-token \
  -u {username}:{password} \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}'

```

Regenerates the subscription token, immediately invalidating the old one. A `subscription.token_regenerated` webhook event is published to the organization's configured webhook subscribers so they can update the token at the gateway. The new plaintext token is returned in the response.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="regenerate-subscription-token-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|subId|path|string|true|none|

> Example responses

> 200 Response

```json
{
  "subscriptionId": "sub-12345",
  "apiId": "weather-api-v1",
  "subscriptionToken": "a3f1e8b2c4d6e8f0a1b3c5d7e9f10b2c4d6e8f0a1b3c5d7e9f10b2c4d6e8f0a1",
  "subscriptionPlanName": "Gold",
  "status": "ACTIVE",
  "createdBy": "alice@example.com",
  "updatedBy": "alice@example.com",
  "createdAt": "2026-05-07T08:30:00Z",
  "updatedAt": "2026-05-07T08:30:00Z"
}
```

> 404 Response

```json
{
  "code": "404",
  "message": "Not Found",
  "description": "Subscription not found"
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

<h3 id="regenerate-subscription-token-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Subscription DTO.|[SubscriptionResponse](schemas.md#schemasubscriptionresponse)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Resource not found.|[SimpleErrorResponse](schemas.md#schemasimpleerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|
