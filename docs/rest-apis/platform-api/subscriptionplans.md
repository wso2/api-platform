<h1 id="wso2-api-platform-platform-api-subscriptionplans">SubscriptionPlans</h1>

Subscription plan management operations — rate-limit tiers that subscriptions are created against

## Create subscription plan

<a id="opIdCreateSubscriptionPlan"></a>

`POST /subscription-plans`

> Code samples

```shell

curl -X POST https://localhost:9243/api/v0.9/subscription-plans \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -d @payload.json

```

Creates an organization-scoped subscription plan.

> Payload

```json
{
  "id": "gold",
  "displayName": "Gold",
  "limits": [
    {
      "limitType": "REQUEST_COUNT",
      "timeUnit": "HOUR",
      "timeAmount": 1,
      "limitCount": 10000,
      "limitCountUnit": "string",
      "stopOnQuotaReach": true
    }
  ],
  "expiryTime": "2019-08-24T14:15:22Z",
  "status": "ACTIVE"
}
```

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:subscription_plan:create`, `ap:subscription_plan:manage`

</aside>

<h3 id="create-subscription-plan-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|[CreateSubscriptionPlanRequest](schemas.md#schemacreatesubscriptionplanrequest)|true|none|

> Example responses

> 201 Response

```json
{
  "id": "string",
  "displayName": "string",
  "limits": [
    {
      "limitType": "REQUEST_COUNT",
      "timeUnit": "HOUR",
      "timeAmount": 1,
      "limitCount": 10000,
      "limitCountUnit": "string",
      "stopOnQuotaReach": true
    }
  ],
  "expiryTime": "2019-08-24T14:15:22Z",
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
      "field": "spec.context",
      "message": "must start with /"
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

> 409 Response

```json
{
  "status": "error",
  "code": "CONFLICT",
  "message": "The specified resource already exists."
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

<h3 id="create-subscription-plan-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|201|[Created](https://tools.ietf.org/html/rfc7231#section-6.3.2)|Subscription plan created successfully|[SubscriptionPlan](schemas.md#schemasubscriptionplan)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad Request. Invalid request or validation error.|[Error](schemas.md#schemaerror)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Forbidden. The authenticated user does not have permission to access this resource.|[Error](schemas.md#schemaerror)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Conflict. Specified resource already exists.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|

### Response Headers

|Status|Header|Type|Format|Description|
|---|---|---|---|---|
|201|Location|string|uri|URL of the newly created resource.|

## List subscription plans

<a id="opIdListSubscriptionPlans"></a>

`GET /subscription-plans`

> Code samples

```shell

curl -X GET https://localhost:9243/api/v0.9/subscription-plans \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Accept: application/json'

```

Returns subscription plans for the organization.

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:subscription_plan:read`, `ap:subscription_plan:manage`

</aside>

<h3 id="list-subscription-plans-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|limit|query|integer|false|Maximum number of items to return per page.|
|offset|query|integer|false|Zero-based index of the first item to return.|

> Example responses

> 200 Response

```json
{
  "list": [
    {
      "id": "string",
      "displayName": "string",
      "limits": [
        {
          "limitType": "REQUEST_COUNT",
          "timeUnit": "HOUR",
          "timeAmount": 1,
          "limitCount": 10000,
          "limitCountUnit": "string",
          "stopOnQuotaReach": true
        }
      ],
      "expiryTime": "2019-08-24T14:15:22Z",
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

> 401 Response

```json
{
  "status": "error",
  "code": "UNAUTHORIZED",
  "message": "Authorization header is required, or the token is invalid or expired."
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

<h3 id="list-subscription-plans-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|List of subscription plans|[SubscriptionPlanListResponse](schemas.md#schemasubscriptionplanlistresponse)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|

## Get subscription plan by ID

<a id="opIdGetSubscriptionPlan"></a>

`GET /subscription-plans/{subscriptionPlanId}`

> Code samples

```shell

curl -X GET https://localhost:9243/api/v0.9/subscription-plans/{subscriptionPlanId} \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Accept: application/json'

```

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:subscription_plan:read`, `ap:subscription_plan:manage`

</aside>

<h3 id="get-subscription-plan-by-id-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|subscriptionPlanId|path|string|true|none|

> Example responses

> 200 Response

```json
{
  "id": "string",
  "displayName": "string",
  "limits": [
    {
      "limitType": "REQUEST_COUNT",
      "timeUnit": "HOUR",
      "timeAmount": 1,
      "limitCount": 10000,
      "limitCountUnit": "string",
      "stopOnQuotaReach": true
    }
  ],
  "expiryTime": "2019-08-24T14:15:22Z",
  "organizationId": "acme",
  "status": "ACTIVE",
  "createdBy": "john.doe",
  "updatedBy": "john.doe",
  "createdAt": "2019-08-24T14:15:22Z",
  "updatedAt": "2019-08-24T14:15:22Z"
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

<h3 id="get-subscription-plan-by-id-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Subscription plan details|[SubscriptionPlan](schemas.md#schemasubscriptionplan)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|

## Update subscription plan

<a id="opIdUpdateSubscriptionPlan"></a>

`PUT /subscription-plans/{subscriptionPlanId}`

> Code samples

```shell

curl -X PUT https://localhost:9243/api/v0.9/subscription-plans/{subscriptionPlanId} \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -d @payload.json

```

> Payload

```json
{
  "id": "string",
  "displayName": "string",
  "limits": [
    {
      "limitType": "REQUEST_COUNT",
      "timeUnit": "HOUR",
      "timeAmount": 1,
      "limitCount": 10000,
      "limitCountUnit": "string",
      "stopOnQuotaReach": true
    }
  ],
  "expiryTime": "2019-08-24T14:15:22Z",
  "organizationId": "acme",
  "status": "ACTIVE",
  "createdAt": "2019-08-24T14:15:22Z",
  "updatedAt": "2019-08-24T14:15:22Z"
}
```

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:subscription_plan:update`, `ap:subscription_plan:manage`

</aside>

<h3 id="update-subscription-plan-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|subscriptionPlanId|path|string|true|none|
|body|body|[SubscriptionPlan](schemas.md#schemasubscriptionplan)|true|none|

> Example responses

> 200 Response

```json
{
  "id": "string",
  "displayName": "string",
  "limits": [
    {
      "limitType": "REQUEST_COUNT",
      "timeUnit": "HOUR",
      "timeAmount": 1,
      "limitCount": 10000,
      "limitCountUnit": "string",
      "stopOnQuotaReach": true
    }
  ],
  "expiryTime": "2019-08-24T14:15:22Z",
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
      "field": "spec.context",
      "message": "must start with /"
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
  "message": "The specified resource already exists."
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

<h3 id="update-subscription-plan-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Updated subscription plan|[SubscriptionPlan](schemas.md#schemasubscriptionplan)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad Request. Invalid request or validation error.|[Error](schemas.md#schemaerror)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Forbidden. The authenticated user does not have permission to access this resource.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Conflict. Specified resource already exists.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|

## Delete subscription plan

<a id="opIdDeleteSubscriptionPlan"></a>

`DELETE /subscription-plans/{subscriptionPlanId}`

> Code samples

```shell

curl -X DELETE https://localhost:9243/api/v0.9/subscription-plans/{subscriptionPlanId} \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Accept: application/json'

```

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:subscription_plan:delete`, `ap:subscription_plan:manage`

</aside>

<h3 id="delete-subscription-plan-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|subscriptionPlanId|path|string|true|none|

> Example responses

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

<h3 id="delete-subscription-plan-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|204|[No Content](https://tools.ietf.org/html/rfc7231#section-6.3.5)|Subscription plan deleted (no content)|None|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Forbidden. The authenticated user does not have permission to access this resource.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|
