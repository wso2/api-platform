<h1 id="wso2-api-developer-portal-core-devportal-routes-organizations">Organizations</h1>

## Create an organization

<a id="opIdcreateOrganization"></a>

`POST /organizations`

> Code samples

```shell

curl -X POST https://devportal.api-platform.io/organizations \
  -u {username}:{password} \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}' \
  -d @payload.json

```

Creates a Developer Portal organization and initializes its default portal configuration, default label, default view, and default subscription plans when configured.

> Payload

```json
{
  "orgName": "string",
  "businessOwner": "string",
  "businessOwnerContact": "string",
  "businessOwnerEmail": "user@example.com",
  "orgHandle": "string",
  "organizationIdentifier": "string"
}
```

```yaml
orgName: string
businessOwner: string
businessOwnerContact: string
businessOwnerEmail: user@example.com
orgHandle: string
organizationIdentifier: string

```

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="create-an-organization-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|[OrganizationCreateRequest](schemas.md#schemaorganizationcreaterequest)|true|Organization creation payload. Send JSON or an organization YAML file in the `organization` multipart field. When YAML is used, the service reads `metadata.name` as `orgHandle` and `spec.displayName` as `orgName`; all other fields are read from `spec`.|

> Example responses

> 201 Response

```json
{
  "orgId": "string",
  "orgName": "string",
  "businessOwner": "string",
  "businessOwnerContact": "string",
  "businessOwnerEmail": "user@example.com",
  "orgHandle": "string",
  "organizationIdentifier": "string",
  "orgConfiguration": {}
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
  "code": "CONFLICT",
  "message": "Conflict"
}
```

> 415 Response

```json
{
  "status": "error",
  "code": "UNSUPPORTED_MEDIA_TYPE",
  "message": "Content-Type must be application/json."
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

<h3 id="create-an-organization-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|201|[Created](https://tools.ietf.org/html/rfc7231#section-6.3.2)|Organization created successfully.|[OrganizationResponse](schemas.md#schemaorganizationresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.|Inline|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Resource not found.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|The request conflicts with an existing resource.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|415|[Unsupported Media Type](https://tools.ietf.org/html/rfc7231#section-6.5.13)|Unsupported request media type.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="create-an-organization-responseschema">Response Schema</h3>

#### Enumerated Values

|Property|Value|
|---|---|
|status|error|
|status|error|

### Response Headers

|Status|Header|Type|Format|Description|
|---|---|---|---|---|
|201|Location|string|uri|URL of the created organization.|

## List organizations

<a id="opIdgetOrganizations"></a>

`GET /organizations`

> Code samples

```shell

curl -X GET https://devportal.api-platform.io/organizations \
  -u {username}:{password} \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}'

```

Returns all Developer Portal organizations visible to the admin context.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

> Example responses

> 200 Response

```json
{
  "list": [
    {
      "orgID": "string",
      "orgName": "string",
      "businessOwner": "string",
      "businessOwnerContact": "string",
      "businessOwnerEmail": "user@example.com",
      "orgHandle": "string",
      "organizationIdentifier": "string",
      "orgConfiguration": {}
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

<h3 id="list-organizations-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|List of organization DTOs.|Inline|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="list-organizations-responseschema">Response Schema</h3>

Status Code **200**

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|» list|[[OrganizationListItemResponse](schemas.md#schemaorganizationlistitemresponse)]|false|none|none|
|»» orgID|string|false|none|none|
|»» orgName|string|false|none|none|
|»» businessOwner|string¦null|false|none|none|
|»» businessOwnerContact|string¦null|false|none|none|
|»» businessOwnerEmail|string(email)¦null|false|none|none|
|»» orgHandle|string|false|none|none|
|»» organizationIdentifier|string|false|none|none|
|»» orgConfiguration|[GenericObject](schemas.md#schemagenericobject)|false|none|none|
|» pagination|[Pagination](schemas.md#schemapagination)|false|none|Standard pagination metadata returned with collection responses.|
|»» total|integer|true|none|Total number of records matching the query.|
|»» limit|integer|true|none|Maximum number of records returned in this response.|
|»» offset|integer|true|none|Number of records skipped before this page.|

## Update an organization

<a id="opIdupdateOrganization"></a>

`PUT /organizations/{orgId}`

> Code samples

```shell

curl -X PUT https://devportal.api-platform.io/organizations/{orgId} \
  -u {username}:{password} \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}' \
  -d @payload.json

```

Updates organization metadata, claim mappings, role mappings, and portal configuration.

> Payload

```json
{
  "orgName": "string",
  "businessOwner": "string",
  "businessOwnerContact": "string",
  "businessOwnerEmail": "user@example.com",
  "orgHandle": "string",
  "organizationIdentifier": "string",
  "orgConfiguration": {
    "devportalMode": "DEFAULT"
  }
}
```

```yaml
orgName: string
businessOwner: string
businessOwnerContact: string
businessOwnerEmail: user@example.com
orgHandle: string
organizationIdentifier: string
orgConfiguration:
  devportalMode: DEFAULT

```

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="update-an-organization-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|[OrganizationUpdateRequest](schemas.md#schemaorganizationupdaterequest)|true|Organization update payload. Send JSON or an organization YAML file in the `organization` multipart field. When YAML is used, the service reads `metadata.name` as `orgHandle` and `spec.displayName` as `orgName`; all other fields are read from `spec`.|
|orgId|path|string|true|none|

> Example responses

> 200 Response

```json
{
  "orgId": "string",
  "orgName": "string",
  "businessOwner": "string",
  "businessOwnerContact": "string",
  "businessOwnerEmail": "user@example.com",
  "orgHandle": "string",
  "organizationIdentifier": "string",
  "orgConfiguration": {}
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

<h3 id="update-an-organization-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Organization DTO returned by create, update, and lookup operations.|[OrganizationResponse](schemas.md#schemaorganizationresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.|Inline|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Resource not found.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|The request conflicts with an existing resource.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="update-an-organization-responseschema">Response Schema</h3>

#### Enumerated Values

|Property|Value|
|---|---|
|status|error|
|status|error|

## Get an organization

<a id="opIdgetOrganization"></a>

`GET /organizations/{orgId}`

> Code samples

```shell

curl -X GET https://devportal.api-platform.io/organizations/{orgId} \
  -u {username}:{password} \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}'

```

Retrieves a single organization by organization ID, organization name, organization handle, or organization identifier.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="get-an-organization-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|orgId|path|string|true|none|

> Example responses

> 200 Response

```json
{
  "orgId": "string",
  "orgName": "string",
  "businessOwner": "string",
  "businessOwnerContact": "string",
  "businessOwnerEmail": "user@example.com",
  "orgHandle": "string",
  "organizationIdentifier": "string",
  "orgConfiguration": {}
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

<h3 id="get-an-organization-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Organization DTO returned by create, update, and lookup operations.|[OrganizationResponse](schemas.md#schemaorganizationresponse)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Resource not found.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

## Delete an organization

<a id="opIddeleteOrganization"></a>

`DELETE /organizations/{orgId}`

> Code samples

```shell

curl -X DELETE https://devportal.api-platform.io/organizations/{orgId} \
  -u {username}:{password} \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}'

```

Deletes an organization and returns no response body when deletion succeeds.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="delete-an-organization-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|orgId|path|string|true|none|

> Example responses

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

> 500 Response

```json
{
  "status": "error",
  "code": "INTERNAL_SERVER_ERROR",
  "message": "An unexpected error occurred."
}
```

<h3 id="delete-an-organization-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|204|[No Content](https://tools.ietf.org/html/rfc7231#section-6.3.5)|Organization deleted successfully.|None|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.|Inline|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Resource not found.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="delete-an-organization-responseschema">Response Schema</h3>

#### Enumerated Values

|Property|Value|
|---|---|
|status|error|
|status|error|
