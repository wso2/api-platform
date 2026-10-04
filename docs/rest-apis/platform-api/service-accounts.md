<h1 id="wso2-api-platform-platform-api-service-accounts">Service Accounts</h1>

Non-human identities that exchange OAuth2 client credentials for a short-lived platform token

## List service accounts

<a id="opIdlistServiceAccounts"></a>

`GET /service-accounts`

> Code samples

```shell

curl -X GET https://localhost:9243/api/v0.9/service-accounts \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Accept: application/json'

```

Returns the organization's service accounts. The client secret is never included, only its masked form.

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:service_account:read`, `ap:service_account:manage`

</aside>

<h3 id="list-service-accounts-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|limit|query|integer|false|Maximum number of items to return per page.|
|offset|query|integer|false|Zero-based index of the first item to return.|

> Example responses
>
> 200 Response

```json
{
  "count": 1,
  "list": [
    {
      "id": "ci-deployer",
      "displayName": "CI deployer",
      "owner": "platform-team@example.com",
      "description": "Deploys REST APIs from the release pipeline",
      "clientId": "sa_acme_ci-deployer_3f9a1c",
      "maskedSecret": "***9f2c1",
      "roles": [
        "ap_sa_reader"
      ],
      "status": "active",
      "lastUsedAt": "2019-08-24T14:15:22Z",
      "lastUsedIp": "203.0.113.7",
      "secretRegeneratedAt": "2019-08-24T14:15:22Z",
      "createdBy": "john.doe",
      "createdAt": "2019-08-24T14:15:22Z",
      "updatedBy": "john.doe",
      "updatedAt": "2019-08-24T14:15:22Z"
    }
  ],
  "pagination": {
    "total": 10,
    "offset": 0,
    "limit": 10
  }
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
      "field": "<name of the offending field>",
      "message": "<reason this field failed validation>"
    }
  ]
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

> 403 Response

```json
{
  "status": "error",
  "code": "FORBIDDEN",
  "message": "You do not have permission to perform this action."
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

<h3 id="list-service-accounts-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|List of service accounts|[ServiceAccountListResponse](schemas.md#schemaserviceaccountlistresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad Request. Invalid request or validation error.|[Error](schemas.md#schemaerror)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Forbidden. The authenticated user does not have permission to access this resource.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|

## Create a service account

<a id="opIdcreateServiceAccount"></a>

`POST /service-accounts`

> Code samples

```shell

curl -X POST https://localhost:9243/api/v0.9/service-accounts \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -d @payload.json

```

Creates a service account and returns its client ID and secret. The secret is
shown only in this response and cannot be recovered later.

> Payload

```json
{
  "id": "ci-deployer",
  "displayName": "CI deployer",
  "owner": "platform-team@example.com",
  "description": "Deploys REST APIs from the release pipeline",
  "roles": [
    "ap_sa_reader"
  ]
}
```

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:service_account:manage`

</aside>

<h3 id="create-a-service-account-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|[ServiceAccountCreateRequest](schemas.md#schemaserviceaccountcreaterequest)|true|none|

> Example responses
>
> 201 Response

```json
{
  "serviceAccount": {
    "id": "ci-deployer",
    "displayName": "CI deployer",
    "owner": "platform-team@example.com",
    "description": "Deploys REST APIs from the release pipeline",
    "clientId": "sa_acme_ci-deployer_3f9a1c",
    "maskedSecret": "***9f2c1",
    "roles": [
      "ap_sa_reader"
    ],
    "status": "active",
    "lastUsedAt": "2019-08-24T14:15:22Z",
    "lastUsedIp": "203.0.113.7",
    "secretRegeneratedAt": "2019-08-24T14:15:22Z",
    "createdBy": "john.doe",
    "createdAt": "2019-08-24T14:15:22Z",
    "updatedBy": "john.doe",
    "updatedAt": "2019-08-24T14:15:22Z"
  },
  "clientId": "sa_acme_ci-deployer_3f9a1c",
  "clientSecret": "string"
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
      "field": "<name of the offending field>",
      "message": "<reason this field failed validation>"
    }
  ]
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

> 403 Response

```json
{
  "status": "error",
  "code": "FORBIDDEN",
  "message": "You do not have permission to perform this action."
}
```

> 409 Response

```json
{
  "status": "error",
  "code": "CONFLICT",
  "message": "The request conflicts with the current state of the resource."
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

<h3 id="create-a-service-account-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|201|[Created](https://tools.ietf.org/html/rfc7231#section-6.3.2)|Service account created. The response carries the only copy of the secret.|[ServiceAccountCredentials](schemas.md#schemaserviceaccountcredentials)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad Request. Invalid request or validation error.|[Error](schemas.md#schemaerror)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Forbidden. The authenticated user does not have permission to access this resource.|[Error](schemas.md#schemaerror)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Conflict. The request conflicts with the current state of the resource.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|

### Response Headers

|Status|Header|Type|Format|Description|
|---|---|---|---|---|
|201|Location|string|uri|URL of the newly created resource.|

## Get a service account

<a id="opIdgetServiceAccount"></a>

`GET /service-accounts/{serviceAccountId}`

> Code samples

```shell

curl -X GET https://localhost:9243/api/v0.9/service-accounts/{serviceAccountId} \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Accept: application/json'

```

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:service_account:read`, `ap:service_account:manage`

</aside>

<h3 id="get-a-service-account-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|serviceAccountId|path|string|true|**Service account ID** consisting of the **handle** of the service account.|

#### Detailed descriptions

**serviceAccountId**: **Service account ID** consisting of the **handle** of the service account.

> Example responses
>
> 200 Response

```json
{
  "id": "ci-deployer",
  "displayName": "CI deployer",
  "owner": "platform-team@example.com",
  "description": "Deploys REST APIs from the release pipeline",
  "clientId": "sa_acme_ci-deployer_3f9a1c",
  "maskedSecret": "***9f2c1",
  "roles": [
    "ap_sa_reader"
  ],
  "status": "active",
  "lastUsedAt": "2019-08-24T14:15:22Z",
  "lastUsedIp": "203.0.113.7",
  "secretRegeneratedAt": "2019-08-24T14:15:22Z",
  "createdBy": "john.doe",
  "createdAt": "2019-08-24T14:15:22Z",
  "updatedBy": "john.doe",
  "updatedAt": "2019-08-24T14:15:22Z"
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

> 403 Response

```json
{
  "status": "error",
  "code": "FORBIDDEN",
  "message": "You do not have permission to perform this action."
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

<h3 id="get-a-service-account-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Service account|[ServiceAccount](schemas.md#schemaserviceaccount)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Forbidden. The authenticated user does not have permission to access this resource.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|

## Update a service account

<a id="opIdupdateServiceAccount"></a>

`PUT /service-accounts/{serviceAccountId}`

> Code samples

```shell

curl -X PUT https://localhost:9243/api/v0.9/service-accounts/{serviceAccountId} \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -d @payload.json

```

Updates metadata, roles or status. Setting `status` to `disabled` stops every
exchange and invalidates tokens already issued. Removing a role also
invalidates tokens already issued; adding one does not. Returns 409 if the
account was changed by another request after this one read it; retry.

> Payload

```json
{
  "displayName": "CI deployer",
  "owner": "platform-team@example.com",
  "description": "Deploys REST APIs from the release pipeline",
  "roles": [
    "ap_sa_reader"
  ],
  "status": "disabled"
}
```

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:service_account:manage`

</aside>

<h3 id="update-a-service-account-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|serviceAccountId|path|string|true|**Service account ID** consisting of the **handle** of the service account.|
|body|body|[ServiceAccountUpdateRequest](schemas.md#schemaserviceaccountupdaterequest)|true|none|

#### Detailed descriptions

**serviceAccountId**: **Service account ID** consisting of the **handle** of the service account.

> Example responses
>
> 200 Response

```json
{
  "id": "ci-deployer",
  "displayName": "CI deployer",
  "owner": "platform-team@example.com",
  "description": "Deploys REST APIs from the release pipeline",
  "clientId": "sa_acme_ci-deployer_3f9a1c",
  "maskedSecret": "***9f2c1",
  "roles": [
    "ap_sa_reader"
  ],
  "status": "active",
  "lastUsedAt": "2019-08-24T14:15:22Z",
  "lastUsedIp": "203.0.113.7",
  "secretRegeneratedAt": "2019-08-24T14:15:22Z",
  "createdBy": "john.doe",
  "createdAt": "2019-08-24T14:15:22Z",
  "updatedBy": "john.doe",
  "updatedAt": "2019-08-24T14:15:22Z"
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
      "field": "<name of the offending field>",
      "message": "<reason this field failed validation>"
    }
  ]
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

> 403 Response

```json
{
  "status": "error",
  "code": "FORBIDDEN",
  "message": "You do not have permission to perform this action."
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

> 409 Response

```json
{
  "status": "error",
  "code": "CONFLICT",
  "message": "The request conflicts with the current state of the resource."
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

<h3 id="update-a-service-account-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Service account updated|[ServiceAccount](schemas.md#schemaserviceaccount)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad Request. Invalid request or validation error.|[Error](schemas.md#schemaerror)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Forbidden. The authenticated user does not have permission to access this resource.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Conflict. The request conflicts with the current state of the resource.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|

## Delete a service account

<a id="opIddeleteServiceAccount"></a>

`DELETE /service-accounts/{serviceAccountId}`

> Code samples

```shell

curl -X DELETE https://localhost:9243/api/v0.9/service-accounts/{serviceAccountId} \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Accept: application/json'

```

Deletes the account and invalidates tokens already issued.

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:service_account:manage`

</aside>

<h3 id="delete-a-service-account-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|serviceAccountId|path|string|true|**Service account ID** consisting of the **handle** of the service account.|

#### Detailed descriptions

**serviceAccountId**: **Service account ID** consisting of the **handle** of the service account.

> Example responses
>
> 401 Response

```json
{
  "status": "error",
  "code": "UNAUTHORIZED",
  "message": "Authorization header is required, or the token is invalid or expired."
}
```

> 403 Response

```json
{
  "status": "error",
  "code": "FORBIDDEN",
  "message": "You do not have permission to perform this action."
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

> 409 Response

```json
{
  "status": "error",
  "code": "CONFLICT",
  "message": "The request conflicts with the current state of the resource."
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

<h3 id="delete-a-service-account-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|204|[No Content](https://tools.ietf.org/html/rfc7231#section-6.3.5)|Service account deleted|None|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Forbidden. The authenticated user does not have permission to access this resource.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Conflict. The request conflicts with the current state of the resource.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|

## Exchange client credentials for an access token

<a id="opIdissueServiceAccountToken"></a>

`POST /service-accounts/token`

> Code samples

```shell

curl -X POST https://localhost:9243/api/v0.9/service-accounts/token \
  -u {username}:{password} \
  -H 'Content-Type: application/x-www-form-urlencoded' \
  -H 'Accept: application/json' \
  -d @payload.json

```

OAuth2 client credentials grant (RFC 6749 section 4.4). Public: the caller has no
token yet. Every authentication failure returns the same 401.

The client authenticates either with HTTP Basic (`Authorization: Basic`, client ID
and secret form-urlencoded first) or with `client_id` and `client_secret` in the
body, not both (RFC 6749 section 2.3.1).

The token carries one authorization claim, chosen by the server's
`auth.authorization.mode`. In `scope` mode, `scope` is required and the token
carries exactly the requested scopes; a missing scope, or one the account's
roles do not grant, is a 400. In `role` mode, `scope` is ignored and the
token carries the account's roles. The response's `scope` is what the token
authorizes in either mode.

> Payload

```yaml
grant_type: client_credentials
client_id: string
client_secret: string
scope: ap:rest_api:read ap:gateway:read

```

<h3 id="exchange-client-credentials-for-an-access-token-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|[ServiceAccountTokenRequest](schemas.md#schemaserviceaccounttokenrequest)|true|none|

> Example responses
>
> 200 Response

```json
{
  "access_token": "string",
  "token_type": "Bearer",
  "expires_in": 900,
  "scope": "ap:rest_api:read ap:rest_api:deployment:manage"
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
      "field": "<name of the offending field>",
      "message": "<reason this field failed validation>"
    }
  ]
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

<h3 id="exchange-client-credentials-for-an-access-token-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Access token issued|[ServiceAccountTokenResponse](schemas.md#schemaserviceaccounttokenresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad Request. Invalid request or validation error.|[Error](schemas.md#schemaerror)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|

## Introspect a service-account token

<a id="opIdintrospectServiceAccountToken"></a>

`POST /service-accounts/introspect`

> Code samples

```shell

curl -X POST https://localhost:9243/api/v0.9/service-accounts/introspect \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Content-Type: application/x-www-form-urlencoded' \
  -H 'Accept: application/json' \
  -d @payload.json

```

RFC 7662 token introspection. Every inactive cause returns exactly
`{"active": false}`.

> Payload

```yaml
token: string
token_type_hint: string

```

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:service_account:token:introspect`

</aside>

<h3 id="introspect-a-service-account-token-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|[IntrospectionRequest](schemas.md#schemaintrospectionrequest)|true|none|

> Example responses
>
> 200 Response

```json
{
  "active": true,
  "scope": "string",
  "client_id": "string",
  "sub": "string",
  "aud": "string",
  "iss": "string",
  "exp": 0,
  "iat": 0,
  "jti": "string",
  "token_type": "string"
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

> 403 Response

```json
{
  "status": "error",
  "code": "FORBIDDEN",
  "message": "You do not have permission to perform this action."
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

<h3 id="introspect-a-service-account-token-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Introspection result|[IntrospectionResponse](schemas.md#schemaintrospectionresponse)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Forbidden. The authenticated user does not have permission to access this resource.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|

## Service-account signing keys

<a id="opIdgetServiceAccountJWKS"></a>

`GET /service-accounts/jwks.json`

> Code samples

```shell

curl -X GET https://localhost:9243/api/v0.9/service-accounts/jwks.json \
  -u {username}:{password} \
  -H 'Accept: application/json'

```

Public keys that verify service-account tokens, as a JSON Web Key Set (RFC 7517).

> Example responses
>
> 200 Response

```json
{
  "keys": [
    {
      "kty": "RSA",
      "kid": "string",
      "use": "sig",
      "alg": "RS256",
      "n": "string",
      "e": "AQAB"
    }
  ]
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

<h3 id="service-account-signing-keys-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|JSON Web Key Set|[JWKSResponse](schemas.md#schemajwksresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|

## Regenerate a service account's secret

<a id="opIdregenerateServiceAccountSecret"></a>

`POST /service-accounts/{serviceAccountId}/regenerate-secret`

> Code samples

```shell

curl -X POST https://localhost:9243/api/v0.9/service-accounts/{serviceAccountId}/regenerate-secret \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Accept: application/json'

```

Replaces the client secret. The old secret stops working at once, with no
overlap, and tokens already issued are invalidated. The new secret is shown
only in this response.

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:service_account:manage`

</aside>

<h3 id="regenerate-a-service-account's-secret-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|serviceAccountId|path|string|true|**Service account ID** consisting of the **handle** of the service account.|

#### Detailed descriptions

**serviceAccountId**: **Service account ID** consisting of the **handle** of the service account.

> Example responses
>
> 200 Response

```json
{
  "serviceAccount": {
    "id": "ci-deployer",
    "displayName": "CI deployer",
    "owner": "platform-team@example.com",
    "description": "Deploys REST APIs from the release pipeline",
    "clientId": "sa_acme_ci-deployer_3f9a1c",
    "maskedSecret": "***9f2c1",
    "roles": [
      "ap_sa_reader"
    ],
    "status": "active",
    "lastUsedAt": "2019-08-24T14:15:22Z",
    "lastUsedIp": "203.0.113.7",
    "secretRegeneratedAt": "2019-08-24T14:15:22Z",
    "createdBy": "john.doe",
    "createdAt": "2019-08-24T14:15:22Z",
    "updatedBy": "john.doe",
    "updatedAt": "2019-08-24T14:15:22Z"
  },
  "clientId": "sa_acme_ci-deployer_3f9a1c",
  "clientSecret": "string"
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

> 403 Response

```json
{
  "status": "error",
  "code": "FORBIDDEN",
  "message": "You do not have permission to perform this action."
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

> 409 Response

```json
{
  "status": "error",
  "code": "CONFLICT",
  "message": "The request conflicts with the current state of the resource."
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

<h3 id="regenerate-a-service-account's-secret-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Secret regenerated|[ServiceAccountCredentials](schemas.md#schemaserviceaccountcredentials)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Forbidden. The authenticated user does not have permission to access this resource.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Conflict. The request conflicts with the current state of the resource.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|
