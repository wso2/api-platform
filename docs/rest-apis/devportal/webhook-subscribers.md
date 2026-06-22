<h1 id="wso2-api-developer-portal-core-devportal-routes-webhook-subscribers">Webhook Subscribers</h1>

## Create a webhook subscriber

<a id="opIdcreateWebhookSubscriber"></a>

`POST /o/{orgId}/devportal/v1/webhook-subscribers`

> Code samples

```shell

curl -X POST https://devportal.api-platform.io/o/{orgId}/devportal/v1/webhook-subscribers \
  -u {username}:{password} \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}' \
  -d @payload.json

```

Registers a webhook subscriber for the organization. Gateway event deliveries (apikey.*, subscription.*, etc.) matching the subscriber's gatewayType/events filters are fanned out to its target URL. The `secret` is encrypted at rest using AES-256-GCM.

> Payload

```json
{
  "name": "Production Gateway",
  "url": "https://gateway.example.com/devportal-webhook",
  "secret": "<shared-secret>",
  "publicKey": "string",
  "gatewayType": "*",
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
|body|body|[WebhookSubscriberRequest](schemas.md#schemawebhooksubscriberrequest)|false|Webhook subscriber configuration payload.|
|orgId|path|string|true|none|

> Example responses

> 201 Response

```json
{
  "id": "sub-uuid-12345",
  "orgId": "org-12345",
  "name": "Production Gateway",
  "url": "https://gateway.example.com/devportal-webhook",
  "enabled": true,
  "gatewayType": "*",
  "events": [
    "apikey.*",
    "subscription.*"
  ],
  "timeoutMs": 5000,
  "hasPublicKey": false
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

<h3 id="create-a-webhook-subscriber-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|201|[Created](https://tools.ietf.org/html/rfc7231#section-6.3.2)|Webhook subscriber configuration response.|[WebhookSubscriberResponseSchema](schemas.md#schemawebhooksubscriberresponseschema)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.|Inline|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Duplicate organization data conflicts with an existing record.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="create-a-webhook-subscriber-responseschema">Response Schema</h3>

## List webhook subscribers

<a id="opIdgetWebhookSubscribers"></a>

`GET /o/{orgId}/devportal/v1/webhook-subscribers`

> Code samples

```shell

curl -X GET https://devportal.api-platform.io/o/{orgId}/devportal/v1/webhook-subscribers \
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
|orgId|path|string|true|none|

> Example responses

> 200 Response

```json
[
  {
    "id": "sub-uuid-12345",
    "orgId": "org-12345",
    "name": "Production Gateway",
    "url": "https://gateway.example.com/devportal-webhook",
    "enabled": true,
    "gatewayType": "*",
    "events": [
      "apikey.*",
      "subscription.*"
    ],
    "timeoutMs": 5000,
    "hasPublicKey": false
  }
]
```

> 500 Response

```json
{
  "code": "500",
  "message": "Internal Server Error",
  "description": "Internal Server Error"
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
|*anonymous*|[[WebhookSubscriberResponseSchema](schemas.md#schemawebhooksubscriberresponseschema)]|false|none|[Webhook subscriber configuration. The secret is never included.]|
|» id|string|false|none|Webhook subscriber UUID.|
|» orgId|string|false|none|none|
|» name|string|false|none|none|
|» url|string(uri)|false|none|none|
|» enabled|boolean|false|none|none|
|» gatewayType|string|false|none|none|
|» events|[string]|false|none|none|
|» timeoutMs|integer|false|none|none|
|» hasPublicKey|boolean|false|none|Whether a public key is configured for envelope-encrypting secret event payloads.|

## Get a webhook subscriber

<a id="opIdgetWebhookSubscriber"></a>

`GET /o/{orgId}/devportal/v1/webhook-subscribers/{subscriberId}`

> Code samples

```shell

curl -X GET https://devportal.api-platform.io/o/{orgId}/devportal/v1/webhook-subscribers/{subscriberId} \
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
|orgId|path|string|true|none|
|subscriberId|path|string|true|Webhook subscriber ID (UUID).|

> Example responses

> 200 Response

```json
{
  "id": "sub-uuid-12345",
  "orgId": "org-12345",
  "name": "Production Gateway",
  "url": "https://gateway.example.com/devportal-webhook",
  "enabled": true,
  "gatewayType": "*",
  "events": [
    "apikey.*",
    "subscription.*"
  ],
  "timeoutMs": 5000,
  "hasPublicKey": false
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

<h3 id="get-a-webhook-subscriber-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Webhook subscriber configuration response.|[WebhookSubscriberResponseSchema](schemas.md#schemawebhooksubscriberresponseschema)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Resource not found.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

## Update a webhook subscriber

<a id="opIdupdateWebhookSubscriber"></a>

`PUT /o/{orgId}/devportal/v1/webhook-subscribers/{subscriberId}`

> Code samples

```shell

curl -X PUT https://devportal.api-platform.io/o/{orgId}/devportal/v1/webhook-subscribers/{subscriberId} \
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
  "url": "https://gateway.example.com/devportal-webhook",
  "secret": "<shared-secret>",
  "publicKey": "string",
  "gatewayType": "*",
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
|orgId|path|string|true|none|
|subscriberId|path|string|true|Webhook subscriber ID (UUID).|

> Example responses

> 200 Response

```json
{
  "id": "sub-uuid-12345",
  "orgId": "org-12345",
  "name": "Production Gateway",
  "url": "https://gateway.example.com/devportal-webhook",
  "enabled": true,
  "gatewayType": "*",
  "events": [
    "apikey.*",
    "subscription.*"
  ],
  "timeoutMs": 5000,
  "hasPublicKey": false
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

<h3 id="update-a-webhook-subscriber-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Webhook subscriber configuration response.|[WebhookSubscriberResponseSchema](schemas.md#schemawebhooksubscriberresponseschema)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.|Inline|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Resource not found.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Duplicate organization data conflicts with an existing record.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="update-a-webhook-subscriber-responseschema">Response Schema</h3>

## Delete a webhook subscriber

<a id="opIddeleteWebhookSubscriber"></a>

`DELETE /o/{orgId}/devportal/v1/webhook-subscribers/{subscriberId}`

> Code samples

```shell

curl -X DELETE https://devportal.api-platform.io/o/{orgId}/devportal/v1/webhook-subscribers/{subscriberId} \
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
|orgId|path|string|true|none|
|subscriberId|path|string|true|Webhook subscriber ID (UUID).|

> Example responses

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

<h3 id="delete-a-webhook-subscriber-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|204|[No Content](https://tools.ietf.org/html/rfc7231#section-6.3.5)|Webhook subscriber deleted successfully.|None|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Resource not found.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|
