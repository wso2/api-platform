<h1 id="wso2-api-developer-portal-core-devportal-routes-app-key-mapping">App Key Mapping</h1>

## Create application key mapping

<a id="opIdcreateAppKeyMapping"></a>

`POST /organizations/{orgId}/app-key-mapping`

> Code samples

```shell

curl -X POST http://localhost:3000/devportal/organizations/{orgId}/app-key-mapping \
  -u {username}:{password} \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}' \
  -d @payload.json

```

Creates or reuses an application and generates or maps OAuth credentials through the selected key manager. In control-plane mode the portal creates a CP application and synchronizes subscriptions. In decoupled mode the portal calls the Authorization Server DCR endpoint directly using the configured key manager adapter. Supply `clientID` to map an existing AS client instead of creating one; `tokenDetails.keyType` is required in both cases.

> Payload

```json
{
  "applicationName": "Weather App",
  "tokenType": "OAUTH",
  "tokenDetails": {
    "keyManager": "Resident Key Manager",
    "keyType": "PRODUCTION",
    "grantTypesToBeSupported": [
      "client_credentials",
      "refresh_token"
    ],
    "callbackUrl": "https://app.example.com/callback",
    "additionalProperties": {
      "application_access_token_expiry_time": "3600",
      "user_access_token_expiry_time": "3600"
    }
  }
}
```

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="create-application-key-mapping-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|[AppKeyMappingRequest](schemas.md#schemaappkeymappingrequest)|true|Application key mapping payload. Use internal, resident, or STS key managers to generate OAuth keys. For external key managers, provide `clientID` and `tokenDetails.keyType` to map an existing client.|
|orgId|path|string|true|none|

> Example responses

> 200 Response

```json
{
  "keyMappingId": "km-12345",
  "keyManager": "Resident Key Manager",
  "keyType": "PRODUCTION",
  "consumerKey": "consumer-key-123",
  "consumerSecret": "consumer-secret-abc",
  "supportedGrantTypes": [
    "client_credentials",
    "refresh_token"
  ],
  "appRefId": "cp-app-98765",
  "subscriptionScopes": [
    "weather.read",
    "weather.write"
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

<h3 id="create-application-key-mapping-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Control-plane key generation or key mapping response, enriched with the control-plane application reference and subscription scope keys.|[AppKeyMappingCreateResponse](schemas.md#schemaappkeymappingcreateresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.|Inline|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Resource not found.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Duplicate organization data conflicts with an existing record.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="create-application-key-mapping-responseschema">Response Schema</h3>

## Get application key mappings

<a id="opIdretrieveAppKeyMappings"></a>

`GET /organizations/{orgId}/app-key-mapping/{appId}`

> Code samples

```shell

curl -X GET http://localhost:3000/devportal/organizations/{orgId}/app-key-mapping/{appId} \
  -u {username}:{password} \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}'

```

Retrieves the Developer Portal application and its stored control-plane key mapping rows. The request is scoped to the authenticated user, and returns `404` when the application is not visible to that user.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="get-application-key-mappings-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|orgId|path|string|true|none|
|appId|path|string|true|none|

> Example responses

> 200 Response

```json
{
  "APP_ID": "app-12345",
  "ORG_ID": "org-12345",
  "CREATED_BY": "user-12345",
  "NAME": "Weather App",
  "DESCRIPTION": "Application used to call Weather APIs.",
  "TYPE": "WEB",
  "DP_APP_KEY_MAPPINGs": [
    {
      "MAPPING_ID": "map-12345",
      "APP_ID": "app-12345",
      "CP_APP_REF": "cp-app-98765",
      "API_REF_ID": "cp-api-12345",
      "ORG_ID": "org-12345",
      "SUBSCRIPTION_REF_ID": "cp-sub-12345",
      "SHARED_TOKEN": true,
      "TOKEN_TYPE": "OAUTH"
    }
  ]
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

<h3 id="get-application-key-mappings-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Application row with stored control-plane key mapping entries.|[AppKeyMappingListResponse](schemas.md#schemaappkeymappinglistresponse)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Resource not found.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|
