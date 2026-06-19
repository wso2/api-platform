<h1 id="wso2-api-developer-portal-core-devportal-routes-subscriptions">Subscriptions</h1>

## Create a subscription

<a id="opIdcreateSubscription"></a>

`POST /o/{orgId}/devportal/v1/subscriptions`

> Code samples

```shell

curl -X POST https://devportal.api-platform.io/o/{orgId}/devportal/v1/subscriptions \
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
|body|body|[SubscriptionCreateRequest](schemas.md#schemasubscriptioncreaterequest)|true|Subscription creation payload. `apiId` is the Developer Portal API ID.|
|orgId|path|string|true|none|

> Example responses

> 201 Response

```json
{
  "subscriptionId": "sub-12345",
  "apiId": "api-7f4c2a6b",
  "subscriptionToken": "a3f1e8b2...",
  "subscriptionPlanName": "Gold",
  "gatewayType": "wso2/api-platform",
  "status": "ACTIVE",
  "createdBy": "alice@example.com",
  "createdAt": "2026-05-07T08:30:00Z"
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
|201|[Created](https://tools.ietf.org/html/rfc7231#section-6.3.2)|Subscription DTO.|[SubscriptionResponse](schemas.md#schemasubscriptionresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.|Inline|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Resource not found.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="create-a-subscription-responseschema">Response Schema</h3>

## List subscriptions

<a id="opIdlistSubscriptions"></a>

`GET /o/{orgId}/devportal/v1/subscriptions`

> Code samples

```shell

curl -X GET https://devportal.api-platform.io/o/{orgId}/devportal/v1/subscriptions \
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
|orgId|path|string|true|none|

> Example responses

> 200 Response

```json
{
  "count": 1,
  "list": [
    {
      "subscriptionId": "sub-12345",
      "apiId": "api-7f4c2a6b",
      "subscriptionPlanName": "Gold",
      "gatewayType": "wso2/api-platform",
      "status": "ACTIVE",
      "createdBy": "alice@example.com"
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
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|List of subscription DTOs.|Inline|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.|Inline|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Resource not found.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="list-subscriptions-responseschema">Response Schema</h3>

Status Code **200**

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|» count|integer|false|none|none|
|» list|[[SubscriptionResponse](schemas.md#schemasubscriptionresponse)]|false|none|[Subscription payload.]|
|»» subscriptionId|string|false|none|none|
|»» apiId|string|false|none|Developer Portal API ID.|
|»» subscriptionToken|string|false|none|Plaintext subscription token. Present on create and when the token has not been encrypted at rest.|
|»» subscriptionPlanName|string|false|none|none|
|»» gatewayType|string|false|none|none|
|»» status|string|false|none|none|
|»» createdBy|string|false|none|Identity (sub claim) of the user who created the subscription.|
|»» createdAt|string(date-time)|false|none|none|

#### Enumerated Values

|Property|Value|
|---|---|
|status|ACTIVE|
|status|INACTIVE|

## Get a subscription

<a id="opIdgetSubscription"></a>

`GET /o/{orgId}/devportal/v1/subscriptions/{subId}`

> Code samples

```shell

curl -X GET https://devportal.api-platform.io/o/{orgId}/devportal/v1/subscriptions/{subId} \
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
|orgId|path|string|true|none|
|subId|path|string|true|none|

> Example responses

> 200 Response

```json
{
  "subscriptionId": "sub-12345",
  "apiId": "api-7f4c2a6b",
  "subscriptionToken": "a3f1e8b2...",
  "subscriptionPlanName": "Gold",
  "gatewayType": "wso2/api-platform",
  "status": "ACTIVE",
  "createdBy": "alice@example.com",
  "createdAt": "2026-05-07T08:30:00Z"
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
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Subscription DTO.|[SubscriptionResponse](schemas.md#schemasubscriptionresponse)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Resource not found.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

## Update a subscription

<a id="opIdupdateSubscription"></a>

`PUT /o/{orgId}/devportal/v1/subscriptions/{subId}`

> Code samples

```shell

curl -X PUT https://devportal.api-platform.io/o/{orgId}/devportal/v1/subscriptions/{subId} \
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
|orgId|path|string|true|none|
|subId|path|string|true|none|

> Example responses

> 200 Response

```json
{
  "subscriptionId": "sub-12345",
  "apiId": "api-7f4c2a6b",
  "subscriptionToken": "a3f1e8b2...",
  "subscriptionPlanName": "Gold",
  "gatewayType": "wso2/api-platform",
  "status": "ACTIVE",
  "createdBy": "alice@example.com",
  "createdAt": "2026-05-07T08:30:00Z"
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
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Subscription DTO.|[SubscriptionResponse](schemas.md#schemasubscriptionresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.|Inline|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Resource not found.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="update-a-subscription-responseschema">Response Schema</h3>

## Delete a subscription

<a id="opIddeleteSubscription"></a>

`DELETE /o/{orgId}/devportal/v1/subscriptions/{subId}`

> Code samples

```shell

curl -X DELETE https://devportal.api-platform.io/o/{orgId}/devportal/v1/subscriptions/{subId} \
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
|orgId|path|string|true|none|
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
