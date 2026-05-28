<h1 id="wso2-api-developer-portal-core-devportal-routes-key-managers">Key Managers</h1>

## Create a key manager

<a id="opIdcreateKeyManager"></a>

`POST /organizations/{orgId}/key-managers`

> Code samples

```shell

curl -X POST http://localhost:3000/devportal/organizations/{orgId}/key-managers \
  -u {username}:{password} \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}' \
  -d @payload.json

```

Creates a key manager configuration for the organization. Accepts either a `application/json` body or a `multipart/form-data` upload with a `keymanager` field containing the KeyManager YAML file. The `adminClientId` and `adminClientSecret` are encrypted at rest using AES-256-GCM.

> Payload

```json
{
  "name": "Asgardeo",
  "type": "ASGARDEO",
  "enabled": true,
  "tokenEndpoint": "https://api.asgardeo.io/t/myorg/oauth2/token",
  "clientRegistrationEndpoint": "https://api.asgardeo.io/t/myorg/api/identity/oauth2/dcr/v1.1/register",
  "issuer": "https://api.asgardeo.io/t/myorg/oauth2/token",
  "jwksURL": "https://api.asgardeo.io/t/myorg/oauth2/jwks",
  "adminClientId": "<client-id>",
  "adminClientSecret": "<client-secret>",
  "supportedGrantTypes": [
    "client_credentials",
    "authorization_code",
    "refresh_token"
  ],
  "supportedScopes": [
    "openid",
    "profile"
  ],
  "additionalProperties": {
    "authorizeEndpoint": "https://api.asgardeo.io/t/myorg/oauth2/authorize",
    "revokeEndpoint": "https://api.asgardeo.io/t/myorg/oauth2/revoke",
    "logoutEndpoint": "https://api.asgardeo.io/t/myorg/oidc/logout"
  }
}
```

```yaml
name: Asgardeo
type: ASGARDEO
enabled: true
tokenEndpoint: https://api.asgardeo.io/t/myorg/oauth2/token
clientRegistrationEndpoint: https://api.asgardeo.io/t/myorg/api/identity/oauth2/dcr/v1.1/register
issuer: https://api.asgardeo.io/t/myorg/oauth2/token
jwksURL: https://api.asgardeo.io/t/myorg/oauth2/jwks
adminClientId: <client-id>
adminClientSecret: <client-secret>
supportedGrantTypes:
  - client_credentials
  - authorization_code
  - refresh_token
supportedScopes:
  - openid
  - profile
additionalProperties:
  authorizeEndpoint: https://api.asgardeo.io/t/myorg/oauth2/authorize
  revokeEndpoint: https://api.asgardeo.io/t/myorg/oauth2/revoke
  logoutEndpoint: https://api.asgardeo.io/t/myorg/oidc/logout

```

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="create-a-key-manager-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|[KeyManagerRequest](schemas.md#schemakeymanagerrequest)|false|Key manager configuration payload. Submit as `application/json` or as `multipart/form-data` with a `keymanager` field containing a KeyManager YAML file.|
|orgId|path|string|true|none|

> Example responses

> 201 Response

```json
{
  "id": "km-uuid-12345",
  "orgId": "org-12345",
  "name": "Asgardeo",
  "type": "ASGARDEO",
  "enabled": true,
  "tokenEndpoint": "https://api.asgardeo.io/t/myorg/oauth2/token",
  "clientRegistrationEndpoint": "https://api.asgardeo.io/t/myorg/api/identity/oauth2/dcr/v1.1/register",
  "issuer": "https://api.asgardeo.io/t/myorg/oauth2/token",
  "jwksURL": "https://api.asgardeo.io/t/myorg/oauth2/jwks",
  "supportedGrantTypes": [
    "client_credentials",
    "authorization_code",
    "refresh_token"
  ],
  "supportedScopes": [
    "openid",
    "profile"
  ],
  "additionalProperties": {
    "authorizeEndpoint": "https://api.asgardeo.io/t/myorg/oauth2/authorize",
    "revokeEndpoint": "https://api.asgardeo.io/t/myorg/oauth2/revoke",
    "logoutEndpoint": "https://api.asgardeo.io/t/myorg/oidc/logout"
  }
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

<h3 id="create-a-key-manager-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|201|[Created](https://tools.ietf.org/html/rfc7231#section-6.3.2)|Key manager configuration response.|[KeyManagerResponseSchema](schemas.md#schemakeymanagerresponseschema)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.|Inline|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Duplicate organization data conflicts with an existing record.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="create-a-key-manager-responseschema">Response Schema</h3>

## List key managers

<a id="opIdgetKeyManagers"></a>

`GET /organizations/{orgId}/key-managers`

> Code samples

```shell

curl -X GET http://localhost:3000/devportal/organizations/{orgId}/key-managers \
  -u {username}:{password} \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}'

```

Returns all key manager configurations for the organization. Admin credentials are never included in the response.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="list-key-managers-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|orgId|path|string|true|none|

> Example responses

> 200 Response

```json
[
  {
    "id": "km-uuid-12345",
    "orgId": "org-12345",
    "name": "Asgardeo",
    "type": "ASGARDEO",
    "enabled": true,
    "tokenEndpoint": "https://api.asgardeo.io/t/myorg/oauth2/token",
    "clientRegistrationEndpoint": "https://api.asgardeo.io/t/myorg/api/identity/oauth2/dcr/v1.1/register",
    "issuer": "https://api.asgardeo.io/t/myorg/oauth2/token",
    "jwksURL": "https://api.asgardeo.io/t/myorg/oauth2/jwks",
    "supportedGrantTypes": [
      "client_credentials",
      "authorization_code",
      "refresh_token"
    ],
    "supportedScopes": [
      "openid",
      "profile"
    ],
    "additionalProperties": {
      "authorizeEndpoint": "https://api.asgardeo.io/t/myorg/oauth2/authorize",
      "revokeEndpoint": "https://api.asgardeo.io/t/myorg/oauth2/revoke",
      "logoutEndpoint": "https://api.asgardeo.io/t/myorg/oidc/logout"
    }
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

<h3 id="list-key-managers-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|List of key manager configurations.|Inline|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="list-key-managers-responseschema">Response Schema</h3>

Status Code **200**

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|[[KeyManagerResponseSchema](schemas.md#schemakeymanagerresponseschema)]|false|none|[Key manager configuration. Admin credentials are never included.]|
|» id|string|false|none|Key manager UUID.|
|» orgId|string|false|none|none|
|» name|string|false|none|none|
|» type|string|false|none|none|
|» enabled|boolean|false|none|none|
|» tokenEndpoint|string(uri)|false|none|none|
|» clientRegistrationEndpoint|string(uri)|false|none|none|
|» issuer|string(uri)¦null|false|none|none|
|» jwksURL|string(uri)¦null|false|none|none|
|» supportedGrantTypes|[string]|false|none|none|
|» supportedScopes|[string]|false|none|none|
|» additionalProperties|object|false|none|none|

#### Enumerated Values

|Property|Value|
|---|---|
|type|ASGARDEO|
|type|WSO2IS|
|type|KEYCLOAK|
|type|GENERIC_OIDC|

## List available key managers for developers

<a id="opIdgetAvailableKeyManagers"></a>

`GET /organizations/{orgId}/key-managers/discover`

> Code samples

```shell

curl -X GET http://localhost:3000/devportal/organizations/{orgId}/key-managers/discover \
  -u {username}:{password} \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}'

```

Returns the minimal public view of enabled key managers. This is the developer-facing endpoint used when generating application keys — no admin credentials or internal endpoints are exposed.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="list-available-key-managers-for-developers-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|orgId|path|string|true|none|

> Example responses

> 200 Response

```json
[
  {
    "id": "km-uuid-12345",
    "name": "Asgardeo",
    "type": "ASGARDEO",
    "tokenEndpoint": "https://api.asgardeo.io/t/myorg/oauth2/token",
    "supportedGrantTypes": [
      "client_credentials",
      "authorization_code"
    ],
    "supportedScopes": [
      "openid",
      "profile"
    ]
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

<h3 id="list-available-key-managers-for-developers-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|List of enabled key managers (developer-facing, minimal view).|Inline|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="list-available-key-managers-for-developers-responseschema">Response Schema</h3>

Status Code **200**

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|[[KeyManagerPublicResponseSchema](schemas.md#schemakeymanagerpublicresponseschema)]|false|none|[Minimal developer-facing key manager view. No admin credentials or DCR endpoints.]|
|» id|string|false|none|none|
|» name|string|false|none|none|
|» type|string|false|none|none|
|» tokenEndpoint|string(uri)|false|none|none|
|» supportedGrantTypes|[string]|false|none|none|
|» supportedScopes|[string]|false|none|none|

#### Enumerated Values

|Property|Value|
|---|---|
|type|ASGARDEO|
|type|WSO2IS|
|type|KEYCLOAK|
|type|GENERIC_OIDC|

## Get a key manager

<a id="opIdgetKeyManager"></a>

`GET /organizations/{orgId}/key-managers/{kmId}`

> Code samples

```shell

curl -X GET http://localhost:3000/devportal/organizations/{orgId}/key-managers/{kmId} \
  -u {username}:{password} \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}'

```

Retrieves a single key manager configuration by ID.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="get-a-key-manager-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|orgId|path|string|true|none|
|kmId|path|string|true|Key manager ID (UUID).|

> Example responses

> 200 Response

```json
{
  "id": "km-uuid-12345",
  "orgId": "org-12345",
  "name": "Asgardeo",
  "type": "ASGARDEO",
  "enabled": true,
  "tokenEndpoint": "https://api.asgardeo.io/t/myorg/oauth2/token",
  "clientRegistrationEndpoint": "https://api.asgardeo.io/t/myorg/api/identity/oauth2/dcr/v1.1/register",
  "issuer": "https://api.asgardeo.io/t/myorg/oauth2/token",
  "jwksURL": "https://api.asgardeo.io/t/myorg/oauth2/jwks",
  "supportedGrantTypes": [
    "client_credentials",
    "authorization_code",
    "refresh_token"
  ],
  "supportedScopes": [
    "openid",
    "profile"
  ],
  "additionalProperties": {
    "authorizeEndpoint": "https://api.asgardeo.io/t/myorg/oauth2/authorize",
    "revokeEndpoint": "https://api.asgardeo.io/t/myorg/oauth2/revoke",
    "logoutEndpoint": "https://api.asgardeo.io/t/myorg/oidc/logout"
  }
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

<h3 id="get-a-key-manager-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Key manager configuration response.|[KeyManagerResponseSchema](schemas.md#schemakeymanagerresponseschema)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Resource not found.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

## Update a key manager

<a id="opIdupdateKeyManager"></a>

`PUT /organizations/{orgId}/key-managers/{kmId}`

> Code samples

```shell

curl -X PUT http://localhost:3000/devportal/organizations/{orgId}/key-managers/{kmId} \
  -u {username}:{password} \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}' \
  -d @payload.json

```

Updates an existing key manager configuration. Accepts either a `application/json` body or a `multipart/form-data` upload with a `keymanager` YAML file. Only supplied fields are updated; omitted fields retain their stored values.

> Payload

```json
{
  "name": "Asgardeo",
  "type": "ASGARDEO",
  "enabled": true,
  "tokenEndpoint": "https://api.asgardeo.io/t/myorg/oauth2/token",
  "clientRegistrationEndpoint": "https://api.asgardeo.io/t/myorg/api/identity/oauth2/dcr/v1.1/register",
  "issuer": "https://api.asgardeo.io/t/myorg/oauth2/token",
  "jwksURL": "https://api.asgardeo.io/t/myorg/oauth2/jwks",
  "adminClientId": "<client-id>",
  "adminClientSecret": "<client-secret>",
  "supportedGrantTypes": [
    "client_credentials",
    "authorization_code",
    "refresh_token"
  ],
  "supportedScopes": [
    "openid",
    "profile"
  ],
  "additionalProperties": {
    "authorizeEndpoint": "https://api.asgardeo.io/t/myorg/oauth2/authorize",
    "revokeEndpoint": "https://api.asgardeo.io/t/myorg/oauth2/revoke",
    "logoutEndpoint": "https://api.asgardeo.io/t/myorg/oidc/logout"
  }
}
```

```yaml
name: Asgardeo
type: ASGARDEO
enabled: true
tokenEndpoint: https://api.asgardeo.io/t/myorg/oauth2/token
clientRegistrationEndpoint: https://api.asgardeo.io/t/myorg/api/identity/oauth2/dcr/v1.1/register
issuer: https://api.asgardeo.io/t/myorg/oauth2/token
jwksURL: https://api.asgardeo.io/t/myorg/oauth2/jwks
adminClientId: <client-id>
adminClientSecret: <client-secret>
supportedGrantTypes:
  - client_credentials
  - authorization_code
  - refresh_token
supportedScopes:
  - openid
  - profile
additionalProperties:
  authorizeEndpoint: https://api.asgardeo.io/t/myorg/oauth2/authorize
  revokeEndpoint: https://api.asgardeo.io/t/myorg/oauth2/revoke
  logoutEndpoint: https://api.asgardeo.io/t/myorg/oidc/logout

```

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="update-a-key-manager-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|[KeyManagerUpdateRequest](schemas.md#schemakeymanagerupdaterequest)|false|Key manager update payload. All fields are optional; only supplied fields are updated. Submit as `application/json` or as `multipart/form-data` with a `keymanager` field containing a KeyManager YAML file.|
|orgId|path|string|true|none|
|kmId|path|string|true|Key manager ID (UUID).|

> Example responses

> 200 Response

```json
{
  "id": "km-uuid-12345",
  "orgId": "org-12345",
  "name": "Asgardeo",
  "type": "ASGARDEO",
  "enabled": true,
  "tokenEndpoint": "https://api.asgardeo.io/t/myorg/oauth2/token",
  "clientRegistrationEndpoint": "https://api.asgardeo.io/t/myorg/api/identity/oauth2/dcr/v1.1/register",
  "issuer": "https://api.asgardeo.io/t/myorg/oauth2/token",
  "jwksURL": "https://api.asgardeo.io/t/myorg/oauth2/jwks",
  "supportedGrantTypes": [
    "client_credentials",
    "authorization_code",
    "refresh_token"
  ],
  "supportedScopes": [
    "openid",
    "profile"
  ],
  "additionalProperties": {
    "authorizeEndpoint": "https://api.asgardeo.io/t/myorg/oauth2/authorize",
    "revokeEndpoint": "https://api.asgardeo.io/t/myorg/oauth2/revoke",
    "logoutEndpoint": "https://api.asgardeo.io/t/myorg/oidc/logout"
  }
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

<h3 id="update-a-key-manager-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Key manager configuration response.|[KeyManagerResponseSchema](schemas.md#schemakeymanagerresponseschema)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.|Inline|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Resource not found.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Duplicate organization data conflicts with an existing record.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="update-a-key-manager-responseschema">Response Schema</h3>

## Delete a key manager

<a id="opIddeleteKeyManager"></a>

`DELETE /organizations/{orgId}/key-managers/{kmId}`

> Code samples

```shell

curl -X DELETE http://localhost:3000/devportal/organizations/{orgId}/key-managers/{kmId} \
  -u {username}:{password} \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}'

```

Deletes a key manager configuration by ID.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="delete-a-key-manager-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|orgId|path|string|true|none|
|kmId|path|string|true|Key manager ID (UUID).|

> Example responses

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

<h3 id="delete-a-key-manager-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|204|[No Content](https://tools.ietf.org/html/rfc7231#section-6.3.5)|Key manager deleted successfully.|None|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Resource not found.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|
