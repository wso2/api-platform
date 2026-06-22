<h1 id="wso2-api-developer-portal-core-devportal-routes-key-managers">Key Managers</h1>

## Create a key manager

<a id="opIdcreateKeyManager"></a>

`POST /o/{orgId}/devportal/v1/key-managers`

> Code samples

```shell

curl -X POST https://devportal.api-platform.io/o/{orgId}/devportal/v1/key-managers \
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

<h3 id="create-a-key-manager-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|201|[Created](https://tools.ietf.org/html/rfc7231#section-6.3.2)|Key manager configuration response.|[KeyManagerResponseSchema](schemas.md#schemakeymanagerresponseschema)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.|Inline|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Duplicate organization data conflicts with an existing record.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="create-a-key-manager-responseschema">Response Schema</h3>

#### Enumerated Values

|Property|Value|
|---|---|
|status|error|
|status|error|

### Response Headers

|Status|Header|Type|Format|Description|
|---|---|---|---|---|
|201|Location|string|uri|URL of the created key manager.|

## List key managers

<a id="opIdgetKeyManagers"></a>

`GET /o/{orgId}/devportal/v1/key-managers`

> Code samples

```shell

curl -X GET https://devportal.api-platform.io/o/{orgId}/devportal/v1/key-managers \
  -u {username}:{password} \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}'

```

Returns key manager configurations for the organization. When `role=developer`, returns only the minimal public view of enabled key managers (no admin credentials or internal endpoints). Without `role`, returns full configurations. Admin credentials are never included in any response.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="list-key-managers-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|role|query|string|false|When `role=developer`, returns the minimal public view of enabled key managers.|
|limit|query|integer|false|Maximum number of records to return.|
|offset|query|integer|false|Number of records to skip before returning results.|
|orgId|path|string|true|none|

#### Enumerated Values

|Parameter|Value|
|---|---|
|role|developer|

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
  "status": "error",
  "code": "INTERNAL_SERVER_ERROR",
  "message": "An unexpected error occurred."
}
```

<h3 id="list-key-managers-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|List of key manager configurations.|Inline|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="list-key-managers-responseschema">Response Schema</h3>

#### Enumerated Values

|Property|Value|
|---|---|
|type|ASGARDEO|
|type|WSO2IS|
|type|KEYCLOAK|
|type|GENERIC_OIDC|
|type|ASGARDEO|
|type|WSO2IS|
|type|KEYCLOAK|
|type|GENERIC_OIDC|

## Get a key manager

<a id="opIdgetKeyManager"></a>

`GET /o/{orgId}/devportal/v1/key-managers/{kmId}`

> Code samples

```shell

curl -X GET https://devportal.api-platform.io/o/{orgId}/devportal/v1/key-managers/{kmId} \
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

<h3 id="get-a-key-manager-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Key manager configuration response.|[KeyManagerResponseSchema](schemas.md#schemakeymanagerresponseschema)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Resource not found.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

## Update a key manager

<a id="opIdupdateKeyManager"></a>

`PUT /o/{orgId}/devportal/v1/key-managers/{kmId}`

> Code samples

```shell

curl -X PUT https://devportal.api-platform.io/o/{orgId}/devportal/v1/key-managers/{kmId} \
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

<h3 id="update-a-key-manager-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Key manager configuration response.|[KeyManagerResponseSchema](schemas.md#schemakeymanagerresponseschema)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.|Inline|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Resource not found.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Duplicate organization data conflicts with an existing record.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="update-a-key-manager-responseschema">Response Schema</h3>

#### Enumerated Values

|Property|Value|
|---|---|
|status|error|
|status|error|

## Delete a key manager

<a id="opIddeleteKeyManager"></a>

`DELETE /o/{orgId}/devportal/v1/key-managers/{kmId}`

> Code samples

```shell

curl -X DELETE https://devportal.api-platform.io/o/{orgId}/devportal/v1/key-managers/{kmId} \
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

<h3 id="delete-a-key-manager-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|204|[No Content](https://tools.ietf.org/html/rfc7231#section-6.3.5)|Key manager deleted successfully.|None|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Resource not found.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|
