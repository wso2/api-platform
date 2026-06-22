<h1 id="wso2-api-developer-portal-core-devportal-routes-webhook-events">Webhook Events</h1>

## List webhook events

<a id="opIdlistWebhookEvents"></a>

`GET /o/{orgId}/devportal/v1/webhook-events`

> Code samples

```shell

curl -X GET https://devportal.api-platform.io/o/{orgId}/devportal/v1/webhook-events \
  -u {username}:{password} \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}'

```

Returns a paginated list of webhook events for the organization. Each event includes a summary of its delivery rows. Requires dp:event_read scope.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="list-webhook-events-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|status|query|string|false|Filter events by status.|
|limit|query|integer|false|Maximum number of records to return.|
|offset|query|integer|false|Number of records to skip before returning results.|
|orgId|path|string|true|none|

#### Enumerated Values

|Parameter|Value|
|---|---|
|status|PENDING|
|status|DISPATCHED|
|status|ALL_DELIVERED|
|status|FAILED|

> Example responses

> 200 Response

```json
{
  "list": [
    {
      "eventId": "evt-abc123",
      "eventType": "apikey.generated",
      "orgId": "org-default",
      "gatewayType": "default",
      "aggregateType": "apikey",
      "aggregateId": "key-12345",
      "status": "ALL_DELIVERED",
      "occurredAt": "2019-08-24T14:15:22Z",
      "deliveries": [
        {
          "deliveryId": "del-abc123",
          "subscriberId": "sub-xyz789",
          "targetUrl": "https://example.com/webhook",
          "status": "DELIVERED",
          "attemptCount": 1,
          "lastHttpStatus": 200,
          "lastError": "string",
          "lastAttemptAt": "2019-08-24T14:15:22Z",
          "deliveredAt": "2019-08-24T14:15:22Z"
        }
      ]
    }
  ],
  "pagination": {
    "total": 42,
    "limit": 20,
    "offset": 0
  }
}
```

> 403 Response

```json
{
  "status": "error",
  "code": "FORBIDDEN",
  "message": "Write operations are disabled in read-only mode."
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

<h3 id="list-webhook-events-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Paginated list of webhook events.|Inline|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Request is forbidden for the current runtime mode or caller permissions.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="list-webhook-events-responseschema">Response Schema</h3>

Status Code **200**

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|» list|[[WebhookEvent](schemas.md#schemawebhookevent)]|false|none|[A webhook event with its delivery rows.]|
|»» eventId|string|false|none|none|
|»» eventType|string|false|none|none|
|»» orgId|string|false|none|none|
|»» gatewayType|string¦null|false|none|none|
|»» aggregateType|string|false|none|none|
|»» aggregateId|string|false|none|none|
|»» status|string|false|none|none|
|»» occurredAt|string(date-time)|false|none|none|
|»» deliveries|[[WebhookEventDelivery](schemas.md#schemawebhookeventdelivery)]|false|none|[A single webhook delivery attempt.]|
|»»» deliveryId|string|false|none|none|
|»»» subscriberId|string|false|none|none|
|»»» targetUrl|string¦null|false|none|none|
|»»» status|string|false|none|none|
|»»» attemptCount|integer|false|none|none|
|»»» lastHttpStatus|integer¦null|false|none|none|
|»»» lastError|string¦null|false|none|none|
|»»» lastAttemptAt|string(date-time)¦null|false|none|none|
|»»» deliveredAt|string(date-time)¦null|false|none|none|
|» pagination|[Pagination](schemas.md#schemapagination)|false|none|Standard pagination metadata returned with collection responses.|
|»» total|integer|true|none|Total number of records matching the query.|
|»» limit|integer|true|none|Maximum number of records returned in this response.|
|»» offset|integer|true|none|Number of records skipped before this page.|

#### Enumerated Values

|Property|Value|
|---|---|
|status|PENDING|
|status|DISPATCHED|
|status|ALL_DELIVERED|
|status|FAILED|
|status|PENDING|
|status|IN_FLIGHT|
|status|DELIVERED|
|status|FAILED|
|status|DEAD_LETTERED|

## Get a webhook event

<a id="opIdgetWebhookEvent"></a>

`GET /o/{orgId}/devportal/v1/webhook-events/{eventId}`

> Code samples

```shell

curl -X GET https://devportal.api-platform.io/o/{orgId}/devportal/v1/webhook-events/{eventId} \
  -u {username}:{password} \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}'

```

Returns a single webhook event with the full details of all its delivery rows. Requires dp:event_read scope.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="get-a-webhook-event-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|orgId|path|string|true|none|
|eventId|path|string|true|Webhook event identifier.|

> Example responses

> 200 Response

```json
{
  "eventId": "evt-abc123",
  "eventType": "apikey.generated",
  "orgId": "org-default",
  "gatewayType": "default",
  "aggregateType": "apikey",
  "aggregateId": "key-12345",
  "status": "ALL_DELIVERED",
  "occurredAt": "2019-08-24T14:15:22Z",
  "deliveries": [
    {
      "deliveryId": "del-abc123",
      "subscriberId": "sub-xyz789",
      "targetUrl": "https://example.com/webhook",
      "status": "DELIVERED",
      "attemptCount": 1,
      "lastHttpStatus": 200,
      "lastError": "string",
      "lastAttemptAt": "2019-08-24T14:15:22Z",
      "deliveredAt": "2019-08-24T14:15:22Z"
    }
  ]
}
```

> 403 Response

```json
{
  "status": "error",
  "code": "FORBIDDEN",
  "message": "Write operations are disabled in read-only mode."
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

<h3 id="get-a-webhook-event-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Single webhook event with full delivery details.|[WebhookEvent](schemas.md#schemawebhookevent)|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Request is forbidden for the current runtime mode or caller permissions.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Resource not found.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

## Retry a failed webhook delivery

<a id="opIdretryWebhookDelivery"></a>

`POST /o/{orgId}/devportal/v1/webhook-deliveries/{deliveryId}/retry`

> Code samples

```shell

curl -X POST https://devportal.api-platform.io/o/{orgId}/devportal/v1/webhook-deliveries/{deliveryId}/retry \
  -u {username}:{password} \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}'

```

Resets a `DEAD_LETTERED` or `FAILED` delivery back to `PENDING` so the delivery worker retries it immediately. Requires dp:delivery_manage scope.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="retry-a-failed-webhook-delivery-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|orgId|path|string|true|none|
|deliveryId|path|string|true|Webhook delivery identifier.|

> Example responses

> 200 Response

```json
{
  "message": "Delivery queued for retry"
}
```

> 403 Response

```json
{
  "status": "error",
  "code": "FORBIDDEN",
  "message": "Write operations are disabled in read-only mode."
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

<h3 id="retry-a-failed-webhook-delivery-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Delivery queued for retry.|Inline|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Request is forbidden for the current runtime mode or caller permissions.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Resource not found.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="retry-a-failed-webhook-delivery-responseschema">Response Schema</h3>

Status Code **200**

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|» message|string|false|none|none|
