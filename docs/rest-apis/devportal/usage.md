<h1 id="wso2-api-developer-portal-core-devportal-routes-usage">Usage</h1>

## Get subscription usage

<a id="opIdgetSubscriptionUsage"></a>

`GET /organizations/{orgId}/subscriptions/{subId}/usage`

> Code samples

```shell

curl -X GET http://localhost:3000/devportal/organizations/{orgId}/subscriptions/{subId}/usage \
  -u {username}:{password} \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}'

```

Returns usage metrics for a subscription over a date range. When `usageFrom` or `usageTo` are not supplied, the service defaults to the last 30 days ending at the current time.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="get-subscription-usage-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|usageFrom|query|string(date-time)|false|Usage start timestamp. Defaults to 30 days before the request time.|
|usageTo|query|string(date-time)|false|Usage end timestamp. Defaults to the request time.|
|orgId|path|string|true|none|
|subId|path|string|true|none|

> Example responses

> 200 Response

```json
{
  "subscriptionId": "cp-sub-12345",
  "from": "2026-04-07T00:00:00Z",
  "to": "2026-05-07T00:00:00Z",
  "total_requests": 12000,
  "avg_response_time": 145,
  "currency": "USD"
}
```

> 400 Response

```json
{
  "error": "CustomError",
  "message": "Invalid usageFrom date"
}
```

> 404 Response

```json
{
  "error": "CustomError",
  "message": "Invalid usageFrom date"
}
```

> 500 Response

```json
{
  "error": "CustomError",
  "message": "Invalid usageFrom date"
}
```

> 502 Response

```json
{
  "error": "CustomError",
  "message": "Invalid usageFrom date"
}
```

<h3 id="get-subscription-usage-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Usage metrics for a subscription.|[SubscriptionUsageResponse](schemas.md#schemasubscriptionusageresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Usage lookup error.|[ControllerErrorResponse](schemas.md#schemacontrollererrorresponse)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Usage lookup error.|[ControllerErrorResponse](schemas.md#schemacontrollererrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Usage lookup error.|[ControllerErrorResponse](schemas.md#schemacontrollererrorresponse)|
|502|[Bad Gateway](https://tools.ietf.org/html/rfc7231#section-6.6.3)|Usage lookup error.|[ControllerErrorResponse](schemas.md#schemacontrollererrorresponse)|
