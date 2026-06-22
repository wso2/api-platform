<h1 id="wso2-api-developer-portal-core-devportal-routes-applications">Applications</h1>

## List applications for the authenticated user

<a id="opIdlistApplications"></a>

`GET /o/{orgId}/devportal/v1/applications`

> Code samples

```shell

curl -X GET https://devportal.api-platform.io/o/{orgId}/devportal/v1/applications \
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
|orgId|path|string|true|none|

> Example responses

> 200 Response

```json
{
  "list": [
    {
      "id": "app-12345",
      "name": "Weather App",
      "description": "Application used to call Weather APIs.",
      "type": "WEB",
      "appMap": [
        {
          "appRefID": "asgardeo-client-abc123",
          "kmID": "km-uuid-12345",
          "keyType": "PRODUCTION",
          "additionalProperties": {}
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
|»» name|string|false|none|none|
|»» description|string|false|none|none|
|»» type|string|false|none|none|
|»» appMap|[[ApplicationKeyMappingSummary](schemas.md#schemaapplicationkeymappingsummary)]|false|none|[OAuth key mapping entry attached to an application.]|
|»»» appRefID|string|false|none|Authorization Server client ID registered via DCR.|
|»»» kmID|string|false|none|UUID of the key manager that issued credentials for this mapping.|
|»»» keyType|string|false|none|Key type for this mapping.|
|»»» additionalProperties|object|false|none|AS-specific extra properties returned during DCR.|
|» pagination|[Pagination](schemas.md#schemapagination)|false|none|Standard pagination metadata returned with collection responses.|
|»» total|integer|true|none|Total number of records matching the query.|
|»» limit|integer|true|none|Maximum number of records returned in this response.|
|»» offset|integer|true|none|Number of records skipped before this page.|

#### Enumerated Values

|Property|Value|
|---|---|
|keyType|PRODUCTION|
|keyType|SANDBOX|

## Create an application

<a id="opIdsaveApplication"></a>

`POST /o/{orgId}/devportal/v1/applications`

> Code samples

```shell

curl -X POST https://devportal.api-platform.io/o/{orgId}/devportal/v1/applications \
  -u {username}:{password} \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}' \
  -d @payload.json

```

Creates a Developer Portal application in the specified organization. The request may be JSON, multipart form fields, or an application YAML file in the `application` multipart field.

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

<h3 id="create-an-application-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|[ApplicationRequest](schemas.md#schemaapplicationrequest)|true|Application payload. Send JSON, multipart form fields, or an application YAML file in the `application` field. When YAML is used, the service reads `spec.displayName` or `metadata.name` as the application name and `spec.description` as the description.|
|orgId|path|string|true|none|

> Example responses

> 201 Response

```json
{
  "id": "app-12345",
  "name": "Weather App",
  "description": "Application used to call Weather APIs.",
  "type": "WEB",
  "appMap": []
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
        "field": "orgName",
        "message": "orgName is required."
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
  "code": "ORG_ALREADY_EXISTS",
  "message": "Organization already exists."
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
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Duplicate organization data conflicts with an existing record.|[ErrorResponse](schemas.md#schemaerrorresponse)|
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

## Update an application

<a id="opIdupdateApplication"></a>

`PUT /o/{orgId}/devportal/v1/applications/{applicationId}`

> Code samples

```shell

curl -X PUT https://devportal.api-platform.io/o/{orgId}/devportal/v1/applications/{applicationId} \
  -u {username}:{password} \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}' \
  -d @payload.json

```

Updates an application owned by the authenticated user in the specified organization. The request may be JSON, multipart form fields, or an application YAML file in the `application` multipart field.

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

<h3 id="update-an-application-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|[ApplicationRequest](schemas.md#schemaapplicationrequest)|true|Application payload. Send JSON, multipart form fields, or an application YAML file in the `application` field. When YAML is used, the service reads `spec.displayName` or `metadata.name` as the application name and `spec.description` as the description.|
|orgId|path|string|true|none|
|applicationId|path|string|true|none|

> Example responses

> 200 Response

```json
{
  "id": "app-12345",
  "name": "Weather App",
  "description": "Application used to call Weather APIs.",
  "type": "WEB",
  "appMap": []
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
        "field": "orgName",
        "message": "orgName is required."
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
  "code": "ORG_ALREADY_EXISTS",
  "message": "Organization already exists."
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
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Duplicate organization data conflicts with an existing record.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="update-an-application-responseschema">Response Schema</h3>

#### Enumerated Values

|Property|Value|
|---|---|
|status|error|
|status|error|

## Delete an application

<a id="opIddeleteApplication"></a>

`DELETE /o/{orgId}/devportal/v1/applications/{applicationId}`

> Code samples

```shell

curl -X DELETE https://devportal.api-platform.io/o/{orgId}/devportal/v1/applications/{applicationId} \
  -u {username}:{password} \
  -H 'Accept: text/plain' \
  -H 'Authorization: Bearer {access-token}'

```

Deletes an application owned by the authenticated user. Before removing the application record the service will make a best-effort attempt to revoke registered OAuth clients with their respective key managers and deletes all stored key mappings; failures are logged as warnings and do not abort deletion.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="delete-an-application-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|orgId|path|string|true|none|
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
