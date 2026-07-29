<h1 id="wso2-api-platform-platform-api-organizations">Organizations</h1>

Organization management operations

## Register a new organization

<a id="opIdRegisterOrganization"></a>

`POST /organizations`

> Code samples

```shell

curl -X POST https://localhost:9243/api/v0.9/organizations \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -d @payload.json

```

Registers a new organization with a unique UUID and handle.
This endpoint is used during the organization onboarding process.
A valid Bearer JWT token with the `ap:organization:create` or `ap:organization:manage` scope is required.

> Payload

```json
{
  "displayName": "Acme Corporation",
  "region": "us"
}
```

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:organization:create`, `ap:organization:manage`

</aside>

<h3 id="register-a-new-organization-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|[Organization](schemas.md#schemaorganization)|true|Organization that needs to be added|

> Example responses

> 201 Response

```json
{
  "id": "acme",
  "displayName": "Acme Corporation",
  "region": "us",
  "createdBy": "john.doe",
  "updatedBy": "john.doe",
  "createdAt": "2023-10-12T10:30:00Z",
  "updatedAt": "2023-10-12T10:30:00Z"
}
```

> 400 Response

```json
{
  "status": "error",
  "code": "VALIDATION_FAILED",
  "message": "The request failed validation.",
  "errors": [
    {
      "field": "spec.context",
      "message": "must start with /"
    }
  ]
}
```

> 409 Response

```json
{
  "status": "error",
  "code": "CONFLICT",
  "message": "The specified resource already exists."
}
```

> 500 Response

```json
{
  "status": "error",
  "code": "INTERNAL_ERROR",
  "message": "An unexpected error occurred.",
  "trackingId": "4f1c6f2e-8a4b-4c93-b1de-9f2f6f0c2a11"
}
```

<h3 id="register-a-new-organization-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|201|[Created](https://tools.ietf.org/html/rfc7231#section-6.3.2)|Organization registered successfully|[Organization](schemas.md#schemaorganization)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad Request. Invalid request or validation error.|[Error](schemas.md#schemaerror)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Conflict. Specified resource already exists.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|

### Response Headers

|Status|Header|Type|Format|Description|
|---|---|---|---|---|
|201|Location|string|uri|URL of the newly created resource.|

## List organizations

<a id="opIdListOrganizations"></a>

`GET /organizations`

> Code samples

```shell

curl -X GET https://localhost:9243/api/v0.9/organizations \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Accept: application/json'

```

Retrieves the list of organizations. Results are returned in a paginated
collection envelope.

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:organization:read`, `ap:organization:manage`

</aside>

<h3 id="list-organizations-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|limit|query|integer|false|Maximum number of items to return per page.|
|offset|query|integer|false|Zero-based index of the first item to return.|

> Example responses

> 200 Response

```json
{
  "count": 2,
  "list": [
    {
      "id": "acme",
      "displayName": "Acme Corporation",
      "region": "us",
      "createdBy": "john.doe",
      "updatedBy": "john.doe",
      "createdAt": "2023-10-12T10:30:00Z",
      "updatedAt": "2023-10-12T10:30:00Z"
    }
  ],
  "pagination": {
    "total": 10,
    "offset": 0,
    "limit": 10
  }
}
```

> 401 Response

```json
{
  "status": "error",
  "code": "UNAUTHORIZED",
  "message": "Authorization header is required, or the token is invalid or expired."
}
```

> 500 Response

```json
{
  "status": "error",
  "code": "INTERNAL_ERROR",
  "message": "An unexpected error occurred.",
  "trackingId": "4f1c6f2e-8a4b-4c93-b1de-9f2f6f0c2a11"
}
```

<h3 id="list-organizations-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Organizations retrieved successfully|[OrganizationListResponse](schemas.md#schemaorganizationlistresponse)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|

## Get organization by ID

<a id="opIdGetOrganization"></a>

`GET /organizations/{organizationId}`

> Code samples

```shell

curl -X GET https://localhost:9243/api/v0.9/organizations/{organizationId} \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Accept: application/json'

```

Returns the organization with the specified handle (id).

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:organization:read`, `ap:organization:manage`

</aside>

<h3 id="get-organization-by-id-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|organizationId|path|string|true|ID of the organization.|

> Example responses

> 200 Response

```json
{
  "id": "acme",
  "displayName": "Acme Corporation",
  "region": "us",
  "createdBy": "john.doe",
  "updatedBy": "john.doe",
  "createdAt": "2023-10-12T10:30:00Z",
  "updatedAt": "2023-10-12T10:30:00Z"
}
```

> 401 Response

```json
{
  "status": "error",
  "code": "UNAUTHORIZED",
  "message": "Authorization header is required, or the token is invalid or expired."
}
```

> 404 Response

```json
{
  "status": "error",
  "code": "NOT_FOUND",
  "message": "The specified resource does not exist."
}
```

> 500 Response

```json
{
  "status": "error",
  "code": "INTERNAL_ERROR",
  "message": "An unexpected error occurred.",
  "trackingId": "4f1c6f2e-8a4b-4c93-b1de-9f2f6f0c2a11"
}
```

<h3 id="get-organization-by-id-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Organization retrieved successfully|[Organization](schemas.md#schemaorganization)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|

## Check if organization exists

<a id="opIdHeadOrganization"></a>

`HEAD /organizations/{organizationId}`

> Code samples

```shell

curl -X HEAD https://localhost:9243/api/v0.9/organizations/{organizationId} \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Accept: application/json'

```

Checks if an organization with the specified handle (id) exists.
Returns only headers without a response body.

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:organization:read`, `ap:organization:manage`

</aside>

<h3 id="check-if-organization-exists-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|organizationId|path|string|true|ID of the organization.|

> Example responses

> 401 Response

```json
{
  "status": "error",
  "code": "UNAUTHORIZED",
  "message": "Authorization header is required, or the token is invalid or expired."
}
```

> 404 Response

```json
{
  "status": "error",
  "code": "NOT_FOUND",
  "message": "The specified resource does not exist."
}
```

> 500 Response

```json
{
  "status": "error",
  "code": "INTERNAL_ERROR",
  "message": "An unexpected error occurred.",
  "trackingId": "4f1c6f2e-8a4b-4c93-b1de-9f2f6f0c2a11"
}
```

<h3 id="check-if-organization-exists-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Organization exists|None|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|
