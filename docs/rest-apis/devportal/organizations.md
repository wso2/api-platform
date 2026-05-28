<h1 id="wso2-api-developer-portal-core-devportal-routes-organizations">Organizations</h1>

## Create an organization

<a id="opIdcreateOrganization"></a>

`POST /organizations`

> Code samples

```shell

curl -X POST http://localhost:3000/devportal/organizations \
  -u {username}:{password} \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}' \
  -d @payload.json

```

Creates a Developer Portal organization and initializes its default portal configuration, default label, default view, default WSO2 provider, and default subscription policies when configured.

> Payload

```json
{
  "orgName": "string",
  "businessOwner": "string",
  "businessOwnerContact": "string",
  "businessOwnerEmail": "user@example.com",
  "orgHandle": "string",
  "roleClaimName": "string",
  "groupsClaimName": "string",
  "organizationClaimName": "string",
  "organizationIdentifier": "string",
  "adminRole": "string",
  "subscriberRole": "string",
  "superAdminRole": "string"
}
```

```yaml
orgName: string
businessOwner: string
businessOwnerContact: string
businessOwnerEmail: user@example.com
orgHandle: string
roleClaimName: string
groupsClaimName: string
organizationClaimName: string
organizationIdentifier: string
adminRole: string
subscriberRole: string
superAdminRole: string

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
  "roleClaimName": "string",
  "groupsClaimName": "string",
  "organizationClaimName": "string",
  "organizationIdentifier": "string",
  "adminRole": "string",
  "subscriberRole": "string",
  "superAdminRole": "string",
  "groupClaimName": "string",
  "orgConfiguration": {}
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

> 415 Response

```json
{
  "code": "415",
  "message": "Unsupported Media Type",
  "description": "Content-Type must be application/json"
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

<h3 id="create-an-organization-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|201|[Created](https://tools.ietf.org/html/rfc7231#section-6.3.2)|Organization created successfully.|[OrganizationResponse](schemas.md#schemaorganizationresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.|Inline|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Resource not found.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Duplicate organization data conflicts with an existing record.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|415|[Unsupported Media Type](https://tools.ietf.org/html/rfc7231#section-6.5.13)|Unsupported request media type.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="create-an-organization-responseschema">Response Schema</h3>

## List organizations

<a id="opIdgetOrganizations"></a>

`GET /organizations`

> Code samples

```shell

curl -X GET http://localhost:3000/devportal/organizations \
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
[
  {
    "orgID": "string",
    "orgName": "string",
    "businessOwner": "string",
    "businessOwnerContact": "string",
    "businessOwnerEmail": "user@example.com",
    "orgHandle": "string",
    "roleClaimName": "string",
    "groupsClaimName": "string",
    "organizationClaimName": "string",
    "organizationIdentifier": "string",
    "adminRole": "string",
    "subscriberRole": "string",
    "superAdminRole": "string",
    "orgConfiguration": {}
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

<h3 id="list-organizations-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|List of organization DTOs.|Inline|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="list-organizations-responseschema">Response Schema</h3>

Status Code **200**

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|[[OrganizationListItemResponse](schemas.md#schemaorganizationlistitemresponse)]|false|none|none|
|» orgID|string|false|none|none|
|» orgName|string|false|none|none|
|» businessOwner|string¦null|false|none|none|
|» businessOwnerContact|string¦null|false|none|none|
|» businessOwnerEmail|string(email)¦null|false|none|none|
|» orgHandle|string|false|none|none|
|» roleClaimName|string|false|none|none|
|» groupsClaimName|string|false|none|none|
|» organizationClaimName|string|false|none|none|
|» organizationIdentifier|string|false|none|none|
|» adminRole|string|false|none|none|
|» subscriberRole|string|false|none|none|
|» superAdminRole|string¦null|false|none|none|
|» orgConfiguration|[GenericObject](schemas.md#schemagenericobject)|false|none|none|

## Update an organization

<a id="opIdupdateOrganization"></a>

`PUT /organizations/{orgId}`

> Code samples

```shell

curl -X PUT http://localhost:3000/devportal/organizations/{orgId} \
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
  "roleClaimName": "string",
  "groupsClaimName": "string",
  "organizationClaimName": "string",
  "organizationIdentifier": "string",
  "adminRole": "string",
  "subscriberRole": "string",
  "superAdminRole": "string",
  "orgConfiguration": {}
}
```

```yaml
orgName: string
businessOwner: string
businessOwnerContact: string
businessOwnerEmail: user@example.com
orgHandle: string
roleClaimName: string
groupsClaimName: string
organizationClaimName: string
organizationIdentifier: string
adminRole: string
subscriberRole: string
superAdminRole: string
orgConfiguration: {}

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
  "roleClaimName": "string",
  "groupsClaimName": "string",
  "organizationClaimName": "string",
  "organizationIdentifier": "string",
  "adminRole": "string",
  "subscriberRole": "string",
  "superAdminRole": "string",
  "groupClaimName": "string",
  "orgConfiguration": {}
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

<h3 id="update-an-organization-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Organization DTO returned by create, update, and lookup operations.|[OrganizationResponse](schemas.md#schemaorganizationresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.|Inline|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Resource not found.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Duplicate organization data conflicts with an existing record.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="update-an-organization-responseschema">Response Schema</h3>

## Get an organization

<a id="opIdgetOrganization"></a>

`GET /organizations/{orgId}`

> Code samples

```shell

curl -X GET http://localhost:3000/devportal/organizations/{orgId} \
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
  "roleClaimName": "string",
  "groupsClaimName": "string",
  "organizationClaimName": "string",
  "organizationIdentifier": "string",
  "adminRole": "string",
  "subscriberRole": "string",
  "superAdminRole": "string",
  "groupClaimName": "string",
  "orgConfiguration": {}
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

curl -X DELETE http://localhost:3000/devportal/organizations/{orgId} \
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

<h3 id="delete-an-organization-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|204|[No Content](https://tools.ietf.org/html/rfc7231#section-6.3.5)|Organization deleted successfully.|None|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.|Inline|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Resource not found.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="delete-an-organization-responseschema">Response Schema</h3>
