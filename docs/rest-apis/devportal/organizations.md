<h1 id="wso2-api-developer-portal-core-devportal-routes-organizations">Organizations</h1>

## Create an organization (not supported)

<a id="opIdcreateOrganization"></a>

`POST /organizations`

> Code samples

```shell

curl -X POST https://localhost:9543/api/v0.9/organizations \
  -u {username}:{password} \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}' \
  -d @payload.json

```

NOT SUPPORTED — always returns 405. This Developer Portal serves the single organization named by its `organization.handle` configuration, which is created on startup along with its default portal configuration, label, view, and subscription plans. The operation is retained for forward compatibility.

> Payload

```json
{
  "displayName": "Acme Corporation",
  "businessOwner": "string",
  "businessOwnerContact": "string",
  "businessOwnerEmail": "user@example.com",
  "id": "acme",
  "idpRefId": "string",
  "cpRefId": "string",
  "configuration": {
    "devportalMode": "DEFAULT"
  }
}
```

```yaml
displayName: Acme Corporation
businessOwner: string
businessOwnerContact: string
businessOwnerEmail: user@example.com
id: acme
idpRefId: string
cpRefId: string
configuration:
  devportalMode: DEFAULT

```

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="create-an-organization-(not-supported)-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|[OrganizationCreateRequest](schemas.md#schemaorganizationcreaterequest)|true|Organization creation payload. Send JSON or an organization YAML file in the `organization` multipart field. The JSON example below applies only to the `application/json` content type. When an organization YAML **file** is uploaded instead, its content must use `kind: Organization` with the nested shape `metadata.name` (handle, any top-level `id` is ignored) and `spec.displayName`; all other fields (including `cpRefId`) are read from `spec`. The YAML `spec` block additionally accepts `labels` (array of `{name, displayName}`) and `views` (array of `{id, displayName, labels}` — `id` becomes the view's handle) to bootstrap labels and views at creation time — these are not available via the `application/json` content type.|

> Example responses

> 201 Response

```json
{
  "id": "acme",
  "displayName": "Acme Corporation",
  "businessOwner": "string",
  "businessOwnerContact": "string",
  "businessOwnerEmail": "user@example.com",
  "idpRefId": "string",
  "cpRefId": "string",
  "configuration": {
    "devportalMode": "DEFAULT"
  },
  "createdAt": "2019-08-24T14:15:22Z",
  "updatedAt": "2019-08-24T14:15:22Z"
}
```

> Bad request. Validation and other bad-request errors are returned as a standard error object (field-level details, when present, are carried in its `errors` array); some legacy handlers return a message-only object.

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

> 405 Response

```json
{
  "status": "error",
  "code": "METHOD_NOT_ALLOWED",
  "message": "This Developer Portal serves a single organization, which is configured and provisioned at startup. Organizations cannot be created, listed, or deleted through the API."
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

<h3 id="create-an-organization-(not-supported)-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|201|[Created](https://tools.ietf.org/html/rfc7231#section-6.3.2)|Organization created successfully.|[OrganizationResponse](schemas.md#schemaorganizationresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request. Validation and other bad-request errors are returned as a standard error object (field-level details, when present, are carried in its `errors` array); some legacy handlers return a message-only object.|Inline|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Resource not found.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|405|[Method Not Allowed](https://tools.ietf.org/html/rfc7231#section-6.5.5)|The operation is not offered by this deployment. Returned by the organization lifecycle operations, which a Developer Portal serving a single organization does not expose — that organization is configured and provisioned at startup.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|The request conflicts with an existing resource.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|415|[Unsupported Media Type](https://tools.ietf.org/html/rfc7231#section-6.5.13)|Unsupported request media type.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="create-an-organization-(not-supported)-responseschema">Response Schema</h3>

#### Enumerated Values

|Property|Value|
|---|---|
|status|error|

### Response Headers

|Status|Header|Type|Format|Description|
|---|---|---|---|---|
|201|Location|string|uri|URL of the created organization.|

## List organizations (not supported)

<a id="opIdgetOrganizations"></a>

`GET /organizations`

> Code samples

```shell

curl -X GET https://localhost:9543/api/v0.9/organizations \
  -u {username}:{password} \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}'

```

NOT SUPPORTED — always returns 405. Listing is inherently cross-organization, and this Developer Portal serves exactly one. Use `GET /organizations/{orgId}` with this instance's own handle instead. The operation is retained for forward compatibility.

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
      "id": "acme",
      "displayName": "Acme Corporation",
      "businessOwner": "string",
      "businessOwnerContact": "string",
      "businessOwnerEmail": "user@example.com",
      "idpRefId": "string",
      "cpRefId": "string",
      "configuration": {
        "devportalMode": "DEFAULT"
      },
      "createdAt": "2019-08-24T14:15:22Z",
      "updatedAt": "2019-08-24T14:15:22Z"
    }
  ],
  "count": 1,
  "pagination": {
    "total": 42,
    "limit": 20,
    "offset": 0
  }
}
```

> 405 Response

```json
{
  "status": "error",
  "code": "METHOD_NOT_ALLOWED",
  "message": "This Developer Portal serves a single organization, which is configured and provisioned at startup. Organizations cannot be created, listed, or deleted through the API."
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

<h3 id="list-organizations-(not-supported)-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|List of organization DTOs.|Inline|
|405|[Method Not Allowed](https://tools.ietf.org/html/rfc7231#section-6.5.5)|The operation is not offered by this deployment. Returned by the organization lifecycle operations, which a Developer Portal serving a single organization does not expose — that organization is configured and provisioned at startup.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="list-organizations-(not-supported)-responseschema">Response Schema</h3>

Status Code **200**

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|» list|[[OrganizationResponse](schemas.md#schemaorganizationresponse)]|false|none|none|
|»» id|string|false|none|The organization's handle (unique). Not the internal database uuid.|
|»» displayName|string|false|none|none|
|»» businessOwner|string¦null|false|none|none|
|»» businessOwnerContact|string¦null|false|none|none|
|»» businessOwnerEmail|string(email)¦null|false|none|none|
|»» idpRefId|string|false|none|The organization claim value asserted by the configured Identity Provider at SSO login. On every login, the portal matches the authenticated user's org claim against this value to resolve which organization they belong to — it must exactly match the IDP's claim, or login fails for that org's users. Distinct from `cpRefId`, which is unrelated to authentication.|
|»» cpRefId|string¦null|false|none|Control Plane reference ID. Included in outbound webhook event payloads so subscribers can correlate this organization with its Control Plane (Platform API) counterpart. Not used for authentication or org resolution.|
|»» configuration|object|false|none|Organization portal configuration. Always includes `devportalMode`; may contain additional free-form keys set by the caller.|
|»»» devportalMode|string|false|none|Controls the mode of the developer portal.|
|»» createdAt|string(date-time)¦null|false|none|none|
|»» updatedAt|string(date-time)¦null|false|none|none|
|» count|integer|false|none|Number of items returned in this page.|
|» pagination|[Pagination](schemas.md#schemapagination)|false|none|Standard pagination metadata returned with collection responses.|
|»» total|integer|true|none|Total number of records matching the query.|
|»» limit|integer|true|none|Maximum number of records returned in this response.|
|»» offset|integer|true|none|Number of records skipped before this page.|

#### Enumerated Values

|Property|Value|
|---|---|
|devportalMode|DEFAULT|
|devportalMode|MCP_SERVERS_ONLY|
|devportalMode|APIS_ONLY|

## Update an organization

<a id="opIdupdateOrganization"></a>

`PUT /organizations/{orgId}`

> Code samples

```shell

curl -X PUT https://localhost:9543/api/v0.9/organizations/{orgId} \
  -u {username}:{password} \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}' \
  -d @payload.json

```

Updates organization metadata, claim mappings, role mappings, and portal configuration. `orgId` must name this instance's own organization; any other returns 403. The `id` (handle) and `idpRefId` fields cannot be changed — they are what page URLs and incoming token organization claims are matched against, so a rename would leave the running instance unable to find its own organization. Sending a different value returns 400.

> Payload

```json
{
  "displayName": "Acme Corporation",
  "businessOwner": "string",
  "businessOwnerContact": "string",
  "businessOwnerEmail": "user@example.com",
  "id": "acme",
  "idpRefId": "string",
  "cpRefId": "string",
  "configuration": {
    "devportalMode": "DEFAULT"
  }
}
```

```yaml
displayName: Acme Corporation
businessOwner: string
businessOwnerContact: string
businessOwnerEmail: user@example.com
id: acme
idpRefId: string
cpRefId: string
configuration:
  devportalMode: DEFAULT

```

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="update-an-organization-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|[OrganizationUpdateRequest](schemas.md#schemaorganizationupdaterequest)|true|Organization update payload. Send JSON or an organization YAML file in the `organization` multipart field. The JSON example below applies only to the `application/json` content type. When an organization YAML **file** is uploaded instead, its content must use `kind: Organization` with the nested shape `metadata.name` (handle, any top-level `id` is ignored) and `spec.displayName`; all other fields (including `cpRefId`) are read from `spec`. The YAML `spec` block additionally accepts `labels` (upserted by name) and `views` (upserted by `id`, which becomes the view's handle, with `labels` replacing the view's label set) — these are not available via the `application/json` content type.|
|orgId|path|string|true|The organization's handle (also matches by name or IDP reference ID). Not the internal database uuid.|

> Example responses

> 200 Response

```json
{
  "id": "acme",
  "displayName": "Acme Corporation",
  "businessOwner": "string",
  "businessOwnerContact": "string",
  "businessOwnerEmail": "user@example.com",
  "idpRefId": "string",
  "cpRefId": "string",
  "configuration": {
    "devportalMode": "DEFAULT"
  },
  "createdAt": "2019-08-24T14:15:22Z",
  "updatedAt": "2019-08-24T14:15:22Z"
}
```

> Bad request. Validation and other bad-request errors are returned as a standard error object (field-level details, when present, are carried in its `errors` array); some legacy handlers return a message-only object.

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
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request. Validation and other bad-request errors are returned as a standard error object (field-level details, when present, are carried in its `errors` array); some legacy handlers return a message-only object.|Inline|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Request is forbidden for the current runtime mode or caller permissions.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Resource not found.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|The request conflicts with an existing resource.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="update-an-organization-responseschema">Response Schema</h3>

#### Enumerated Values

|Property|Value|
|---|---|
|status|error|

## Get an organization

<a id="opIdgetOrganization"></a>

`GET /organizations/{orgId}`

> Code samples

```shell

curl -X GET https://localhost:9543/api/v0.9/organizations/{orgId} \
  -u {username}:{password} \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}'

```

Retrieves this instance's organization by organization name, handle, or IDP reference ID. Because the portal serves a single organization, `orgId` must resolve to that one; any other organization returns 403 — and so does an organization that does not exist, so the response cannot be used to discover which organizations the shared database holds.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="get-an-organization-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|orgId|path|string|true|The organization's handle (also matches by name or IDP reference ID). Not the internal database uuid.|

> Example responses

> 200 Response

```json
{
  "id": "acme",
  "displayName": "Acme Corporation",
  "businessOwner": "string",
  "businessOwnerContact": "string",
  "businessOwnerEmail": "user@example.com",
  "idpRefId": "string",
  "cpRefId": "string",
  "configuration": {
    "devportalMode": "DEFAULT"
  },
  "createdAt": "2019-08-24T14:15:22Z",
  "updatedAt": "2019-08-24T14:15:22Z"
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

<h3 id="get-an-organization-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Organization DTO returned by create, update, and lookup operations.|[OrganizationResponse](schemas.md#schemaorganizationresponse)|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Request is forbidden for the current runtime mode or caller permissions.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Resource not found.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

## Delete an organization (not supported)

<a id="opIddeleteOrganization"></a>

`DELETE /organizations/{orgId}`

> Code samples

```shell

curl -X DELETE https://localhost:9543/api/v0.9/organizations/{orgId} \
  -u {username}:{password} \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}'

```

NOT SUPPORTED — always returns 405. This Developer Portal instance is bound to a single organization for its whole lifetime; deleting it would leave the instance serving nothing. The operation is retained for forward compatibility.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="delete-an-organization-(not-supported)-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|orgId|path|string|true|The organization's handle (also matches by name or IDP reference ID). Not the internal database uuid.|

> Example responses

> Bad request. Validation and other bad-request errors are returned as a standard error object (field-level details, when present, are carried in its `errors` array); some legacy handlers return a message-only object.

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

> 405 Response

```json
{
  "status": "error",
  "code": "METHOD_NOT_ALLOWED",
  "message": "This Developer Portal serves a single organization, which is configured and provisioned at startup. Organizations cannot be created, listed, or deleted through the API."
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

<h3 id="delete-an-organization-(not-supported)-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|204|[No Content](https://tools.ietf.org/html/rfc7231#section-6.3.5)|Organization deleted successfully.|None|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request. Validation and other bad-request errors are returned as a standard error object (field-level details, when present, are carried in its `errors` array); some legacy handlers return a message-only object.|Inline|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Resource not found.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|405|[Method Not Allowed](https://tools.ietf.org/html/rfc7231#section-6.5.5)|The operation is not offered by this deployment. Returned by the organization lifecycle operations, which a Developer Portal serving a single organization does not expose — that organization is configured and provisioned at startup.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="delete-an-organization-(not-supported)-responseschema">Response Schema</h3>

#### Enumerated Values

|Property|Value|
|---|---|
|status|error|
