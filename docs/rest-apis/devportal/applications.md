<h1 id="wso2-api-developer-portal-core-devportal-routes-applications">Applications</h1>

## Import an application

<a id="opIdimportApplications"></a>

`POST /organizations/{orgId}/applications/import`

> Code samples

```shell

curl -X POST http://localhost:3000/devportal/organizations/{orgId}/applications/import \
  -u {username}:{password} \
  -H 'Content-Type: multipart/form-data' \
  -H 'Accept: application/json' \
  -H 'pat-token: string' \
  -H 'Authorization: Bearer {access-token}' \
  -d @payload.json

```

Imports an application from an application YAML file. This route is registered only when `config.features.importApplication.enabled` is true. When `withKeys` is true, `keyManager` is required and matching keys from the YAML are imported.

> Payload

```yaml
file: application.yaml
withKeys: false

```

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="import-an-application-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|pat-token|header|string|true|Personal access token used by the application import flow.|
|body|body|object|true|Application import payload. The uploaded YAML describes the application, subscriptions, and optionally application keys. Set `withKeys` to true only when importing keys for a specific `keyManager`.|
|» file|body|string(binary)|true|Imported application YAML file.|
|» withKeys|body|boolean|false|Whether to import application keys from the YAML.|
|» keyManager|body|string|false|Required when `withKeys` is true.|
|orgId|path|string|true|none|

> Example responses

> Application import result.

```json
{
  "status": "Success"
}
```

```json
{
  "status": "Incomplete",
  "failedSubscriptions": [
    {
      "apiName": "Weather API",
      "reason": "Subscription policy not found"
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

> 500 Response

```json
{
  "code": "500",
  "message": "Internal Server Error",
  "description": "Internal Server Error"
}
```

<h3 id="import-an-application-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Application import result.|[ApplicationImportResponse](schemas.md#schemaapplicationimportresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.|Inline|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="import-an-application-responseschema">Response Schema</h3>

## Create an application for the authenticated user's organization

<a id="opIdsaveApplication"></a>

`POST /applications`

> Code samples

```shell

curl -X POST http://localhost:3000/devportal/applications \
  -u {username}:{password} \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}' \
  -d @payload.json

```

Creates a Developer Portal application in the organization resolved from the authenticated user's organization identifier. The request may be JSON, multipart form fields, or an application YAML file in the `application` multipart field.

> Payload

```json
{
  "name": "Weather App",
  "description": "Application used to call Weather APIs.",
  "type": "WEB"
}
```

```yaml
name: Weather App
description: Application used to call Weather APIs.
type: WEB

```

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="create-an-application-for-the-authenticated-user's-organization-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|[ApplicationRequest](schemas.md#schemaapplicationrequest)|true|Application payload. Send JSON, multipart form fields, or an application YAML file in the `application` field. When YAML is used, the service reads `spec.displayName` or `metadata.name` as the application name and `spec.description` as the description.|

> Example responses

> 201 Response

```json
{
  "id": "app-12345",
  "name": "Weather App",
  "description": "Application used to call Weather APIs.",
  "type": "WEB"
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

<h3 id="create-an-application-for-the-authenticated-user's-organization-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|201|[Created](https://tools.ietf.org/html/rfc7231#section-6.3.2)|Application DTO.|[ApplicationResponse](schemas.md#schemaapplicationresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.|Inline|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Duplicate organization data conflicts with an existing record.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="create-an-application-for-the-authenticated-user's-organization-responseschema">Response Schema</h3>

## Update an application for the authenticated user

<a id="opIdupdateApplication"></a>

`PUT /applications/{applicationId}`

> Code samples

```shell

curl -X PUT http://localhost:3000/devportal/applications/{applicationId} \
  -u {username}:{password} \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}' \
  -d @payload.json

```

Updates an application owned by the authenticated user in the organization resolved from the authenticated user's organization identifier. The request may be JSON, multipart form fields, or an application YAML file in the `application` multipart field.

> Payload

```json
{
  "name": "Weather App",
  "description": "Application used to call Weather APIs.",
  "type": "WEB"
}
```

```yaml
name: Weather App
description: Application used to call Weather APIs.
type: WEB

```

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="update-an-application-for-the-authenticated-user-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|[ApplicationRequest](schemas.md#schemaapplicationrequest)|true|Application payload. Send JSON, multipart form fields, or an application YAML file in the `application` field. When YAML is used, the service reads `spec.displayName` or `metadata.name` as the application name and `spec.description` as the description.|
|applicationId|path|string|true|none|

> Example responses

> 200 Response

```json
{
  "id": "app-12345",
  "name": "Weather App",
  "description": "Application used to call Weather APIs.",
  "type": "WEB"
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

<h3 id="update-an-application-for-the-authenticated-user-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Application DTO.|[ApplicationResponse](schemas.md#schemaapplicationresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.|Inline|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Resource not found.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Duplicate organization data conflicts with an existing record.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="update-an-application-for-the-authenticated-user-responseschema">Response Schema</h3>

## Delete an application for the authenticated user

<a id="opIddeleteApplication"></a>

`DELETE /applications/{applicationId}`

> Code samples

```shell

curl -X DELETE http://localhost:3000/devportal/applications/{applicationId} \
  -u {username}:{password} \
  -H 'Accept: text/plain' \
  -H 'Authorization: Bearer {access-token}'

```

Deletes an application owned by the authenticated user. Before removing the application record the service revokes all OAuth clients registered with their respective key managers and deletes all stored key mappings. Key manager revocation failures are logged as warnings and do not abort the deletion.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="delete-an-application-for-the-authenticated-user-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|applicationId|path|string|true|none|

> Example responses

> 200 Response

```
"string"
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

<h3 id="delete-an-application-for-the-authenticated-user-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Plain text success response.|string|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Resource not found.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

## Reset application throttle policy

<a id="opIdresetThrottlingPolicy"></a>

`POST /applications/{applicationId}/reset-throttle-policy`

> Code samples

```shell

curl -X POST http://localhost:3000/devportal/applications/{applicationId}/reset-throttle-policy \
  -u {username}:{password} \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}' \
  -d @payload.json

```

Resets throttling policy state for a control-plane application. The request is proxied to the control plane using the supplied user name.

> Payload

```json
{
  "userName": "alice@example.com"
}
```

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="reset-application-throttle-policy-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|[ResetThrottlePolicyRequest](schemas.md#schemaresetthrottlepolicyrequest)|true|User throttle policy reset payload.|
|applicationId|path|string|true|none|

> Example responses

> 200 Response

```json
{
  "message": "string"
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

<h3 id="reset-application-throttle-policy-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|JSON message response.|[MessageResponse](schemas.md#schemamessageresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.|Inline|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Resource not found.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="reset-application-throttle-policy-responseschema">Response Schema</h3>
