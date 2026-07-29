<h1 id="wso2-api-platform-platform-api-subscriptions">Subscriptions</h1>

Subscription management operations linking applications to APIs under a subscription plan

## Create subscription

<a id="opIdCreateSubscription"></a>

`POST /subscriptions`

> Code samples

```shell

curl -X POST https://localhost:9243/api/v0.9/subscriptions \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -d @payload.json

```

Creates a subscription for the specified artifact.
`subscriberId` identifies the unique subscriber for this artifact, allowing multiple subscribers to create subscriptions for the same artifact.

> Payload

```json
{
  "artifactId": "my-rest-api",
  "kind": "RestApi",
  "subscriberId": "user-123",
  "applicationId": "my-app-handle",
  "subscriptionPlanId": "gold",
  "status": "ACTIVE"
}
```

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:subscription:create`, `ap:subscription:manage`

</aside>

<h3 id="create-subscription-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|[CreateSubscriptionRequest](schemas.md#schemacreatesubscriptionrequest)|true|none|

> Example responses
>
> 201 Response

```json
{
  "id": "497f6eca-6276-4993-bfeb-53cbbbba6f08",
  "artifactId": "my-rest-api",
  "kind": "RestApi",
  "subscriberId": "string",
  "applicationId": "my-app-handle",
  "subscriptionToken": "string",
  "subscriptionPlanId": "gold",
  "subscriptionPlanName": "string",
  "organizationId": "acme",
  "status": "ACTIVE",
  "createdBy": "john.doe",
  "updatedBy": "john.doe",
  "createdAt": "2019-08-24T14:15:22Z",
  "updatedAt": "2019-08-24T14:15:22Z"
}
```

> 400 Response

```json
{
  "status": "error",
  "code": "VALIDATION_FAILED",
  "message": "The request failed validation.",
  "errors": [
    {
      "field": "<name of the offending field>",
      "message": "<reason this field failed validation>"
    }
  ]
}
```

> 401 Response

```json
{
  "status": "error",
  "code": "UNAUTHORIZED",
  "message": "Authorization header is required, or the token is invalid or expired."
}
```

> 403 Response

```json
{
  "status": "error",
  "code": "FORBIDDEN",
  "message": "You do not have permission to perform this action."
}
```

> 404 Response

```json
{
  "status": "error",
  "code": "NOT_FOUND",
  "message": "The specified resource does not exist."
}
```

> 409 Response

```json
{
  "status": "error",
  "code": "CONFLICT",
  "message": "The request conflicts with the current state of the resource."
}
```

> 500 Response

```json
{
  "status": "error",
  "code": "INTERNAL_ERROR",
  "message": "An unexpected error occurred.",
  "trackingId": "4f1c6f2e-8a4b-4c93-b1de-9f2f6f0c2a11"
}
```

<h3 id="create-subscription-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|201|[Created](https://tools.ietf.org/html/rfc7231#section-6.3.2)|Subscription created successfully|[Subscription](schemas.md#schemasubscription)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad Request. Invalid request or validation error.|[Error](schemas.md#schemaerror)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Forbidden. The authenticated user does not have permission to access this resource.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Conflict. The request conflicts with the current state of the resource.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|

### Response Headers

|Status|Header|Type|Format|Description|
|---|---|---|---|---|
|201|Location|string|uri|URL of the newly created resource.|

## List subscriptions

<a id="opIdListSubscriptions"></a>

`GET /subscriptions`

> Code samples

```shell

curl -X GET https://localhost:9243/api/v0.9/subscriptions \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Accept: application/json'

```

Returns subscriptions filtered by artifact and/or application.
Optional query parameters artifactId, subscriberId, applicationId and status filter the list.
Supports pagination via limit and offset.

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:subscription:read`, `ap:subscription:manage`

</aside>

<h3 id="list-subscriptions-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|artifactId|query|string|false|Filter by artifact ID (UUID or handle)|
|subscriberId|query|string|false|Filter by subscriber ID|
|applicationId|query|string|false|Filter by application ID|
|status|query|string|false|Filter by status (ACTIVE, INACTIVE, REVOKED)|
|limit|query|integer|false|Maximum number of items to return per page.|
|offset|query|integer|false|Zero-based index of the first item to return.|

#### Enumerated Values

|Parameter|Value|
|---|---|
|status|ACTIVE|
|status|INACTIVE|
|status|REVOKED|

> Example responses
>
> 200 Response

```json
{
  "list": [
    {
      "id": "497f6eca-6276-4993-bfeb-53cbbbba6f08",
      "artifactId": "my-rest-api",
      "kind": "RestApi",
      "subscriberId": "string",
      "applicationId": "my-app-handle",
      "subscriptionToken": "string",
      "subscriptionPlanId": "gold",
      "subscriptionPlanName": "string",
      "organizationId": "acme",
      "status": "ACTIVE",
      "createdBy": "john.doe",
      "updatedBy": "john.doe",
      "createdAt": "2019-08-24T14:15:22Z",
      "updatedAt": "2019-08-24T14:15:22Z"
    }
  ],
  "count": 0,
  "pagination": {
    "total": 10,
    "offset": 0,
    "limit": 10
  }
}
```

> 400 Response

```json
{
  "status": "error",
  "code": "VALIDATION_FAILED",
  "message": "The request failed validation.",
  "errors": [
    {
      "field": "<name of the offending field>",
      "message": "<reason this field failed validation>"
    }
  ]
}
```

> 401 Response

```json
{
  "status": "error",
  "code": "UNAUTHORIZED",
  "message": "Authorization header is required, or the token is invalid or expired."
}
```

> 404 Response

```json
{
  "status": "error",
  "code": "NOT_FOUND",
  "message": "The specified resource does not exist."
}
```

> 500 Response

```json
{
  "status": "error",
  "code": "INTERNAL_ERROR",
  "message": "An unexpected error occurred.",
  "trackingId": "4f1c6f2e-8a4b-4c93-b1de-9f2f6f0c2a11"
}
```

<h3 id="list-subscriptions-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|List of subscriptions|[SubscriptionListResponse](schemas.md#schemasubscriptionlistresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad Request. Invalid request or validation error.|[Error](schemas.md#schemaerror)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|

## Get subscription by ID

<a id="opIdGetSubscription"></a>

`GET /subscriptions/{subscriptionId}`

> Code samples

```shell

curl -X GET https://localhost:9243/api/v0.9/subscriptions/{subscriptionId} \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Accept: application/json'

```

Returns a single subscription by ID, scoped to the organization in the access token.

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:subscription:read`, `ap:subscription:manage`

</aside>

<h3 id="get-subscription-by-id-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|subscriptionId|path|string(uuid)|true|Subscription UUID|

> Example responses
>
> 200 Response

```json
{
  "id": "497f6eca-6276-4993-bfeb-53cbbbba6f08",
  "artifactId": "my-rest-api",
  "kind": "RestApi",
  "subscriberId": "string",
  "applicationId": "my-app-handle",
  "subscriptionToken": "string",
  "subscriptionPlanId": "gold",
  "subscriptionPlanName": "string",
  "organizationId": "acme",
  "status": "ACTIVE",
  "createdBy": "john.doe",
  "updatedBy": "john.doe",
  "createdAt": "2019-08-24T14:15:22Z",
  "updatedAt": "2019-08-24T14:15:22Z"
}
```

> 400 Response

```json
{
  "status": "error",
  "code": "VALIDATION_FAILED",
  "message": "The request failed validation.",
  "errors": [
    {
      "field": "<name of the offending field>",
      "message": "<reason this field failed validation>"
    }
  ]
}
```

> 401 Response

```json
{
  "status": "error",
  "code": "UNAUTHORIZED",
  "message": "Authorization header is required, or the token is invalid or expired."
}
```

> 403 Response

```json
{
  "status": "error",
  "code": "FORBIDDEN",
  "message": "You do not have permission to perform this action."
}
```

> 404 Response

```json
{
  "status": "error",
  "code": "NOT_FOUND",
  "message": "The specified resource does not exist."
}
```

> 500 Response

```json
{
  "status": "error",
  "code": "INTERNAL_ERROR",
  "message": "An unexpected error occurred.",
  "trackingId": "4f1c6f2e-8a4b-4c93-b1de-9f2f6f0c2a11"
}
```

<h3 id="get-subscription-by-id-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Subscription details|[Subscription](schemas.md#schemasubscription)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad Request. Invalid request or validation error.|[Error](schemas.md#schemaerror)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Forbidden. The authenticated user does not have permission to access this resource.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|

## Update subscription

<a id="opIdUpdateSubscription"></a>

`PUT /subscriptions/{subscriptionId}`

> Code samples

```shell

curl -X PUT https://localhost:9243/api/v0.9/subscriptions/{subscriptionId}?subscriberId=string \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -d @payload.json

```

Updates a subscription (e.g. status).
Query parameter `subscriberId` is required and must match the subscription's subscriber for access control.

> Payload

```json
{
  "id": "497f6eca-6276-4993-bfeb-53cbbbba6f08",
  "artifactId": "my-rest-api",
  "kind": "RestApi",
  "subscriberId": "string",
  "applicationId": "my-app-handle",
  "subscriptionToken": "string",
  "subscriptionPlanId": "gold",
  "subscriptionPlanName": "string",
  "organizationId": "acme",
  "status": "ACTIVE",
  "createdAt": "2019-08-24T14:15:22Z",
  "updatedAt": "2019-08-24T14:15:22Z"
}
```

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:subscription:update`, `ap:subscription:manage`

</aside>

<h3 id="update-subscription-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|subscriptionId|path|string(uuid)|true|Subscription UUID|
|subscriberId|query|string|true|Subscriber ID; must match the subscription's subscriberId.|
|body|body|[Subscription](schemas.md#schemasubscription)|true|none|

> Example responses
>
> 200 Response

```json
{
  "id": "497f6eca-6276-4993-bfeb-53cbbbba6f08",
  "artifactId": "my-rest-api",
  "kind": "RestApi",
  "subscriberId": "string",
  "applicationId": "my-app-handle",
  "subscriptionToken": "string",
  "subscriptionPlanId": "gold",
  "subscriptionPlanName": "string",
  "organizationId": "acme",
  "status": "ACTIVE",
  "createdBy": "john.doe",
  "updatedBy": "john.doe",
  "createdAt": "2019-08-24T14:15:22Z",
  "updatedAt": "2019-08-24T14:15:22Z"
}
```

> 400 Response

```json
{
  "status": "error",
  "code": "VALIDATION_FAILED",
  "message": "The request failed validation.",
  "errors": [
    {
      "field": "<name of the offending field>",
      "message": "<reason this field failed validation>"
    }
  ]
}
```

> 401 Response

```json
{
  "status": "error",
  "code": "UNAUTHORIZED",
  "message": "Authorization header is required, or the token is invalid or expired."
}
```

> 403 Response

```json
{
  "status": "error",
  "code": "FORBIDDEN",
  "message": "You do not have permission to perform this action."
}
```

> 404 Response

```json
{
  "status": "error",
  "code": "NOT_FOUND",
  "message": "The specified resource does not exist."
}
```

> 500 Response

```json
{
  "status": "error",
  "code": "INTERNAL_ERROR",
  "message": "An unexpected error occurred.",
  "trackingId": "4f1c6f2e-8a4b-4c93-b1de-9f2f6f0c2a11"
}
```

<h3 id="update-subscription-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Updated subscription|[Subscription](schemas.md#schemasubscription)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad Request. Invalid request or validation error.|[Error](schemas.md#schemaerror)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Forbidden. The authenticated user does not have permission to access this resource.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|

## Delete subscription

<a id="opIdDeleteSubscription"></a>

`DELETE /subscriptions/{subscriptionId}`

> Code samples

```shell

curl -X DELETE https://localhost:9243/api/v0.9/subscriptions/{subscriptionId}?subscriberId=string \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Accept: application/json'

```

Removes the subscription for the API.
Query parameter `subscriberId` is required and must match the subscription's subscriber for access control.

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:subscription:delete`, `ap:subscription:manage`

</aside>

<h3 id="delete-subscription-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|subscriptionId|path|string(uuid)|true|Subscription UUID|
|subscriberId|query|string|true|Subscriber ID; must match the subscription's subscriberId.|

> Example responses
>
> 400 Response

```json
{
  "status": "error",
  "code": "VALIDATION_FAILED",
  "message": "The request failed validation.",
  "errors": [
    {
      "field": "<name of the offending field>",
      "message": "<reason this field failed validation>"
    }
  ]
}
```

> 401 Response

```json
{
  "status": "error",
  "code": "UNAUTHORIZED",
  "message": "Authorization header is required, or the token is invalid or expired."
}
```

> 403 Response

```json
{
  "status": "error",
  "code": "FORBIDDEN",
  "message": "You do not have permission to perform this action."
}
```

> 404 Response

```json
{
  "status": "error",
  "code": "NOT_FOUND",
  "message": "The specified resource does not exist."
}
```

> 500 Response

```json
{
  "status": "error",
  "code": "INTERNAL_ERROR",
  "message": "An unexpected error occurred.",
  "trackingId": "4f1c6f2e-8a4b-4c93-b1de-9f2f6f0c2a11"
}
```

<h3 id="delete-subscription-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|204|[No Content](https://tools.ietf.org/html/rfc7231#section-6.3.5)|Subscription deleted (no content)|None|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad Request. Invalid request or validation error.|[Error](schemas.md#schemaerror)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Forbidden. The authenticated user does not have permission to access this resource.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|
