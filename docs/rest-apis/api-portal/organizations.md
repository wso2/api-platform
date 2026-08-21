<h1 id="wso2-api-portal-and-mcp-hub-organizations">Organizations</h1>

## Update an organization

<a id="opIdupdateOrganization"></a>

`PUT /organizations/{orgId}`

> Code samples

```shell

curl -X PUT https://localhost:9543/api-portal/api/v0.9/organizations/{orgId} \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -d @payload.json

```

Updates organization metadata, claim mappings, role mappings, and portal configuration. `orgId` must name this instance's own organization; any other returns 403. The `id` (handle) and `idpRefId` fields cannot be changed — they are what page URLs and incoming token organization claims are matched against, so a rename would leave the running instance unable to find its own organization. Sending a different value returns 400. `idpRefId` is owned by the portal's `auth.idp_org_id` configuration, which is re-applied on every restart; change it there rather than here.

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
  "configuration": {}
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
configuration: {}

```

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `dp:organization:update`, `dp:organization:manage`

</aside>

<h3 id="update-an-organization-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|[OrganizationUpdateRequest](schemas.md#schemaorganizationupdaterequest)|true|Organization update payload. Send JSON or an organization YAML file in the `organization` multipart field. The JSON example below applies only to the `application/json` content type. When an organization YAML **file** is uploaded instead, its content must use `kind: Organization` with the nested shape `metadata.name` (handle, any top-level `id` is ignored) and `spec.displayName`; all other fields (including `cpRefId`) are read from `spec`. The YAML `spec` block additionally accepts `labels` (upserted by name) and `views` (upserted by `id`, which becomes the view's handle, with `labels` replacing the view's label set) — these are not available via the `application/json` content type.|
|orgId|path|string|true|The organization's handle (also matches by name or IDP reference ID). Not the internal database uuid.|

> Example responses
>
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
  "configuration": {},
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
  "message": "Forbidden"
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
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Request is forbidden — because the caller lacks the required permission, because the current runtime mode disallows the operation (read-only mode), or because an organization was named that is not the single one this instance serves. In that last case an organization that does not exist is answered identically to one belonging to someone else, so the response cannot be used to discover what a shared database holds.|[ErrorResponse](schemas.md#schemaerrorresponse)|
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

curl -X GET https://localhost:9543/api-portal/api/v0.9/organizations/{orgId} \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Accept: application/json'

```

Retrieves this instance's organization by organization name, handle, or IDP reference ID. Because the portal serves a single organization, `orgId` must resolve to that one; any other organization returns 403 — and so does an organization that does not exist, so the response cannot be used to discover which organizations the shared database holds.

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `dp:organization:read`, `dp:organization:manage`

</aside>

<h3 id="get-an-organization-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|orgId|path|string|true|The organization's handle (also matches by name or IDP reference ID). Not the internal database uuid.|

> Example responses
>
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
  "configuration": {},
  "createdAt": "2019-08-24T14:15:22Z",
  "updatedAt": "2019-08-24T14:15:22Z"
}
```

> 403 Response

```json
{
  "status": "error",
  "code": "FORBIDDEN",
  "message": "Forbidden"
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
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Request is forbidden — because the caller lacks the required permission, because the current runtime mode disallows the operation (read-only mode), or because an organization was named that is not the single one this instance serves. In that last case an organization that does not exist is answered identically to one belonging to someone else, so the response cannot be used to discover what a shared database holds.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|
