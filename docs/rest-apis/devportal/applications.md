<h1 id="wso2-api-developer-portal-core-devportal-routes-applications">Applications</h1>

## List applications for the authenticated user

<a id="opIdlistApplications"></a>

`GET /devportal/v1/applications`

> Code samples

```shell

curl -X GET https://devportal.api-platform.io/devportal/v1/applications \
  -u {username}:{password} \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}'

```

Returns all applications owned by the authenticated user in the specified organization.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="list-applications-for-the-authenticated-user-parameters">Parameters</h3>

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
      "id": "app-12345",
      "displayName": "Weather App",
      "description": "Application used to call Weather APIs.",
      "appKeyMappings": [
        {
          "asClientId": "asgardeo-client-abc123",
          "kmId": "km-uuid-12345",
          "type": "PRODUCTION"
        }
      ]
    }
  ],
  "pagination": {
    "total": 1,
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

<h3 id="list-applications-for-the-authenticated-user-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|List of application DTOs.|Inline|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="list-applications-for-the-authenticated-user-responseschema">Response Schema</h3>

Status Code **200**

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|» list|[[ApplicationResponse](schemas.md#schemaapplicationresponse)]|false|none|none|
|»» id|string|false|none|none|
|»» displayName|string|false|none|none|
|»» handle|string|false|none|none|
|»» description|string|false|none|none|
|»» appKeyMappings|[[ApplicationKeyMappingSummary](schemas.md#schemaapplicationkeymappingsummary)]|false|none|[OAuth client ID mapping entry attached to an application.]|
|»»» asClientId|string|false|none|OAuth client ID, created directly in the key manager and linked to this application.|
|»»» kmId|string|false|none|UUID of the key manager this client ID is linked to.|
|»»» type|string|false|none|Key type for this mapping.|
|» pagination|[Pagination](schemas.md#schemapagination)|false|none|Standard pagination metadata returned with collection responses.|
|»» total|integer|true|none|Total number of records matching the query.|
|»» limit|integer|true|none|Maximum number of records returned in this response.|
|»» offset|integer|true|none|Number of records skipped before this page.|

#### Enumerated Values

|Property|Value|
|---|---|
|type|PRODUCTION|
|type|SANDBOX|

## Create an application

<a id="opIdsaveApplication"></a>

`POST /devportal/v1/applications`

> Code samples

```shell

curl -X POST https://devportal.api-platform.io/devportal/v1/applications \
  -u {username}:{password} \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}' \
  -d @payload.json

```

Creates a Developer Portal application in the specified organization. The request may be JSON, multipart form fields, or an application YAML file in the `application` multipart field. An `application.created` webhook event is published to the organization's configured webhook subscribers.

> Payload

```json
{
  "displayName": "Weather App",
  "handle": "weather-app",
  "description": "Application used to call Weather APIs."
}
```

```yaml
displayName: Weather App
handle: weather-app
description: Application used to call Weather APIs.

```

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="create-an-application-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|[ApplicationRequest](schemas.md#schemaapplicationrequest)|true|Application payload. Send JSON, multipart form fields, or an application YAML file in the `application` field. When YAML is used, the service reads `spec.displayName` as the application's display name, `spec.description` as the description, and `spec.handle` or `metadata.name` as the handle.|

> Example responses

> 201 Response

```json
{
  "id": "app-12345",
  "displayName": "Weather App",
  "description": "Application used to call Weather APIs.",
  "appKeyMappings": []
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

<h3 id="create-an-application-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|201|[Created](https://tools.ietf.org/html/rfc7231#section-6.3.2)|Application DTO.|[ApplicationResponse](schemas.md#schemaapplicationresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.|Inline|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|The request conflicts with an existing resource.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="create-an-application-responseschema">Response Schema</h3>

#### Enumerated Values

|Property|Value|
|---|---|
|status|error|
|status|error|

### Response Headers

|Status|Header|Type|Format|Description|
|---|---|---|---|---|
|201|Location|string|uri|URL of the created application.|

## Get an application

<a id="opIdgetApplication"></a>

`GET /devportal/v1/applications/{applicationId}`

> Code samples

```shell

curl -X GET https://devportal.api-platform.io/devportal/v1/applications/{applicationId} \
  -u {username}:{password} \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}'

```

Returns the details of a single application owned by the authenticated user.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="get-an-application-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|applicationId|path|string|true|none|

> Example responses

> 200 Response

```json
{
  "id": "app-12345",
  "displayName": "Weather App",
  "handle": "weather-app",
  "description": "Application used to call Weather APIs.",
  "appKeyMappings": []
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

<h3 id="get-an-application-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Application DTO.|[ApplicationResponse](schemas.md#schemaapplicationresponse)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Resource not found.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

## Update an application

<a id="opIdupdateApplication"></a>

`PUT /devportal/v1/applications/{applicationId}`

> Code samples

```shell

curl -X PUT https://devportal.api-platform.io/devportal/v1/applications/{applicationId} \
  -u {username}:{password} \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}' \
  -d @payload.json

```

Updates an application owned by the authenticated user in the specified organization. The request may be JSON, multipart form fields, or an application YAML file in the `application` multipart field. An `application.updated` webhook event is published.

> Payload

```json
{
  "displayName": "Weather App",
  "handle": "weather-app",
  "description": "Application used to call Weather APIs."
}
```

```yaml
displayName: Weather App
handle: weather-app
description: Application used to call Weather APIs.

```

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="update-an-application-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|[ApplicationRequest](schemas.md#schemaapplicationrequest)|true|Application payload. Send JSON, multipart form fields, or an application YAML file in the `application` field. When YAML is used, the service reads `spec.displayName` as the application's display name, `spec.description` as the description, and `spec.handle` or `metadata.name` as the handle.|
|applicationId|path|string|true|none|

> Example responses

> 200 Response

```json
{
  "id": "app-12345",
  "displayName": "Weather App",
  "handle": "weather-app",
  "description": "Application used to call Weather APIs.",
  "appKeyMappings": []
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

<h3 id="update-an-application-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Application DTO.|[ApplicationResponse](schemas.md#schemaapplicationresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.|Inline|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Resource not found.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|The request conflicts with an existing resource.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="update-an-application-responseschema">Response Schema</h3>

#### Enumerated Values

|Property|Value|
|---|---|
|status|error|
|status|error|

## Delete an application

<a id="opIddeleteApplication"></a>

`DELETE /devportal/v1/applications/{applicationId}`

> Code samples

```shell

curl -X DELETE https://devportal.api-platform.io/devportal/v1/applications/{applicationId} \
  -u {username}:{password} \
  -H 'Accept: text/plain' \
  -H 'Authorization: Bearer {access-token}'

```

Deletes an application owned by the authenticated user. Before removing the application record the service will make a best-effort attempt to revoke registered OAuth clients with their respective key managers and deletes all stored key mappings; failures are logged as warnings and do not abort deletion. An `application.deleted` webhook event is published, plus one `apikey.application_updated` event (with a cleared association) per API key that was associated with the application.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="delete-an-application-parameters">Parameters</h3>

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

<h3 id="delete-an-application-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Plain text success response.|string|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Resource not found.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|
