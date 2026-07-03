<h1 id="wso2-api-developer-portal-core-devportal-routes-webhook-subscribers">Webhook Subscribers</h1>

## Create a webhook subscriber

<a id="opIdcreateWebhookSubscriber"></a>

`POST /webhook-subscribers`

> Code samples

```shell

curl -X POST https://localhost:3000/api/v0.9/webhook-subscribers \
  -u {username}:{password} \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}' \
  -d @payload.json

```

Registers a webhook subscriber for the organization. Event deliveries (apikey.*, subscription.*, etc.) matching the subscriber's events filter are fanned out to its target URL. The `secret`, if provided, is encrypted at rest using AES-256-GCM.

> Payload

```json
{
  "name": "Production Gateway",
  "targetUrl": "https://gateway.example.com/devportal-webhook",
  "secret": "<shared-secret>",
  "publicKey": "string",
  "events": [
    "apikey.*",
    "subscription.*"
  ],
  "enabled": true,
  "timeoutMs": 5000
}
```

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="create-a-webhook-subscriber-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|[WebhookSubscriberRequest](schemas.md#schemawebhooksubscriberrequest)|true|Webhook subscriber configuration payload.|

> Example responses

> 201 Response

```json
{
  "id": "sub-uuid-12345",
  "orgId": "org-12345",
  "name": "Production Gateway",
  "targetUrl": "https://gateway.example.com/devportal-webhook",
  "enabled": true,
  "events": [
    "apikey.*",
    "subscription.*"
  ],
  "timeoutMs": 5000,
  "hasSecret": true,
  "hasPublicKey": false,
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

<h3 id="create-a-webhook-subscriber-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|201|[Created](https://tools.ietf.org/html/rfc7231#section-6.3.2)|Webhook subscriber configuration response.|[WebhookSubscriberResponseSchema](schemas.md#schemawebhooksubscriberresponseschema)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.|Inline|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|The request conflicts with an existing resource.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="create-a-webhook-subscriber-responseschema">Response Schema</h3>

#### Enumerated Values

|Property|Value|
|---|---|
|status|error|
|status|error|

### Response Headers

|Status|Header|Type|Format|Description|
|---|---|---|---|---|
|201|Location|string|uri|URL of the created webhook subscriber.|

## List webhook subscribers

<a id="opIdgetWebhookSubscribers"></a>

`GET /webhook-subscribers`

> Code samples

```shell

curl -X GET https://localhost:3000/api/v0.9/webhook-subscribers \
  -u {username}:{password} \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}'

```

Returns all webhook subscriber configurations for the organization. Secrets are never included in the response.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="list-webhook-subscribers-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|limit|query|integer|false|Maximum number of records to return.|
|offset|query|integer|false|Number of records to skip before returning results.|

> Example responses

> 200 Response

```json
{
  "list": [
    {
      "id": "sub-uuid-12345",
      "orgId": "org-12345",
      "name": "Production Gateway",
      "targetUrl": "https://gateway.example.com/devportal-webhook",
      "enabled": true,
      "events": [
        "apikey.*",
        "subscription.*"
      ],
      "timeoutMs": 5000,
      "hasSecret": true,
      "hasPublicKey": false,
      "createdBy": "alice@example.com",
      "updatedBy": "alice@example.com",
      "createdAt": "2019-08-24T14:15:22Z",
      "updatedAt": "2019-08-24T14:15:22Z"
    }
  ],
  "pagination": {
    "total": 42,
    "limit": 20,
    "offset": 0
  }
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

<h3 id="list-webhook-subscribers-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|List of webhook subscriber configurations.|Inline|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="list-webhook-subscribers-responseschema">Response Schema</h3>

Status Code **200**

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|» list|[[WebhookSubscriberResponseSchema](schemas.md#schemawebhooksubscriberresponseschema)]|false|none|[Webhook subscriber configuration. The secret is never included.]|
|»» id|string|false|none|Webhook subscriber UUID.|
|»» orgId|string|false|none|none|
|»» name|string|false|none|none|
|»» targetUrl|string(uri)|false|none|none|
|»» enabled|boolean|false|none|none|
|»» events|[string]|false|none|none|
|»» timeoutMs|integer|false|none|none|
|»» hasSecret|boolean|false|none|Whether a secret is configured for HMAC-signing outgoing payloads.|
|»» hasPublicKey|boolean|false|none|Whether a public key is configured for envelope-encrypting secret event payloads.|
|»» createdBy|string|false|none|Identity of the user who created this webhook subscriber, or `deleted_user` if that user's IDP reference no longer exists. Present on single-resource GET responses and list items.|
|»» updatedBy|string|false|none|Identity of the user who last updated this webhook subscriber, or `deleted_user` if that user's IDP reference no longer exists. Present on single-resource GET responses only, omitted on list items.|
|»» createdAt|string(date-time)|false|none|none|
|»» updatedAt|string(date-time)|false|none|none|
|» pagination|[Pagination](schemas.md#schemapagination)|false|none|Standard pagination metadata returned with collection responses.|
|»» total|integer|true|none|Total number of records matching the query.|
|»» limit|integer|true|none|Maximum number of records returned in this response.|
|»» offset|integer|true|none|Number of records skipped before this page.|

## Get a webhook subscriber

<a id="opIdgetWebhookSubscriber"></a>

`GET /webhook-subscribers/{subscriberId}`

> Code samples

```shell

curl -X GET https://localhost:3000/api/v0.9/webhook-subscribers/{subscriberId} \
  -u {username}:{password} \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}'

```

Retrieves a single webhook subscriber configuration by ID.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="get-a-webhook-subscriber-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|subscriberId|path|string|true|Webhook subscriber ID (UUID).|

> Example responses

> 200 Response

```json
{
  "id": "sub-uuid-12345",
  "orgId": "org-12345",
  "name": "Production Gateway",
  "targetUrl": "https://gateway.example.com/devportal-webhook",
  "enabled": true,
  "events": [
    "apikey.*",
    "subscription.*"
  ],
  "timeoutMs": 5000,
  "hasSecret": true,
  "hasPublicKey": false,
  "createdBy": "alice@example.com",
  "updatedBy": "alice@example.com",
  "createdAt": "2019-08-24T14:15:22Z",
  "updatedAt": "2019-08-24T14:15:22Z"
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

<h3 id="get-a-webhook-subscriber-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Webhook subscriber configuration response.|[WebhookSubscriberResponseSchema](schemas.md#schemawebhooksubscriberresponseschema)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Resource not found.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

## Update a webhook subscriber

<a id="opIdupdateWebhookSubscriber"></a>

`PUT /webhook-subscribers/{subscriberId}`

> Code samples

```shell

curl -X PUT https://localhost:3000/api/v0.9/webhook-subscribers/{subscriberId} \
  -u {username}:{password} \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}' \
  -d @payload.json

```

Updates an existing webhook subscriber configuration. Only supplied fields are updated; omitted fields retain their stored values.

> Payload

```json
{
  "name": "Production Gateway",
  "targetUrl": "https://gateway.example.com/devportal-webhook",
  "secret": "<shared-secret>",
  "publicKey": "string",
  "events": [
    "apikey.*",
    "subscription.*"
  ],
  "enabled": true,
  "timeoutMs": 5000
}
```

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="update-a-webhook-subscriber-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|[WebhookSubscriberRequest](schemas.md#schemawebhooksubscriberrequest)|false|Webhook subscriber update payload. All fields are optional; only supplied fields are updated.|
|subscriberId|path|string|true|Webhook subscriber ID (UUID).|

> Example responses

> 200 Response

```json
{
  "id": "sub-uuid-12345",
  "orgId": "org-12345",
  "name": "Production Gateway",
  "targetUrl": "https://gateway.example.com/devportal-webhook",
  "enabled": true,
  "events": [
    "apikey.*",
    "subscription.*"
  ],
  "timeoutMs": 5000,
  "hasSecret": true,
  "hasPublicKey": false,
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

<h3 id="update-a-webhook-subscriber-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Webhook subscriber configuration response.|[WebhookSubscriberResponseSchema](schemas.md#schemawebhooksubscriberresponseschema)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.|Inline|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Resource not found.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|The request conflicts with an existing resource.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="update-a-webhook-subscriber-responseschema">Response Schema</h3>

#### Enumerated Values

|Property|Value|
|---|---|
|status|error|
|status|error|

## Delete a webhook subscriber

<a id="opIddeleteWebhookSubscriber"></a>

`DELETE /webhook-subscribers/{subscriberId}`

> Code samples

```shell

curl -X DELETE https://localhost:3000/api/v0.9/webhook-subscribers/{subscriberId} \
  -u {username}:{password} \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}'

```

Deletes a webhook subscriber configuration by ID.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="delete-a-webhook-subscriber-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|subscriberId|path|string|true|Webhook subscriber ID (UUID).|

> Example responses

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

<h3 id="delete-a-webhook-subscriber-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|204|[No Content](https://tools.ietf.org/html/rfc7231#section-6.3.5)|Webhook subscriber deleted successfully.|None|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Resource not found.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

## List recent deliveries for a webhook subscriber

<a id="opIdgetWebhookSubscriberDeliveries"></a>

`GET /webhook-subscribers/{subscriberId}/deliveries`

> Code samples

```shell

curl -X GET https://localhost:3000/api/v0.9/webhook-subscribers/{subscriberId}/deliveries \
  -u {username}:{password} \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}'

```

Returns the most recent webhook delivery attempts for a single subscriber, newest first.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="list-recent-deliveries-for-a-webhook-subscriber-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|subscriberId|path|string|true|Webhook subscriber ID (UUID).|

> Example responses

> 200 Response

```json
{
  "list": [
    {
      "deliveryId": "del-abc123",
      "eventType": "apikey.generated",
      "occurredAt": "2019-08-24T14:15:22Z",
      "status": "DELIVERED",
      "lastHttpStatus": 200,
      "lastError": "string",
      "lastAttemptAt": "2019-08-24T14:15:22Z",
      "deliveredAt": "2019-08-24T14:15:22Z"
    }
  ]
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

<h3 id="list-recent-deliveries-for-a-webhook-subscriber-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Recent delivery attempts for this webhook subscriber.|Inline|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Resource not found.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="list-recent-deliveries-for-a-webhook-subscriber-responseschema">Response Schema</h3>

Status Code **200**

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|» list|[[WebhookSubscriberDeliverySummary](schemas.md#schemawebhooksubscriberdeliverysummary)]|false|none|[A single delivery attempt made to a webhook subscriber.]|
|»» deliveryId|string|false|none|none|
|»» eventType|string¦null|false|none|none|
|»» occurredAt|string(date-time)¦null|false|none|none|
|»» status|string|false|none|none|
|»» lastHttpStatus|integer¦null|false|none|none|
|»» lastError|string¦null|false|none|none|
|»» lastAttemptAt|string(date-time)¦null|false|none|none|
|»» deliveredAt|string(date-time)¦null|false|none|none|

#### Enumerated Values

|Property|Value|
|---|---|
|status|PENDING|
|status|IN_FLIGHT|
|status|DELIVERED|
|status|FAILED|
