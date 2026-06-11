<h1 id="wso2-api-developer-portal-core-devportal-routes-identity-providers">Identity Providers</h1>

## Create an identity provider

<a id="opIdcreateIdentityProvider"></a>

`POST /o/{orgId}/devportal/v1/identity-providers`

> Code samples

```shell

curl -X POST https://devportal.api-platform.io/o/{orgId}/devportal/v1/identity-providers \
  -u {username}:{password} \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}' \
  -d @payload.json

```

Creates the organization-level OIDC identity provider configuration.

> Payload

```json
{
  "issuer": "string",
  "name": "string",
  "authorizationURL": "http://example.com",
  "tokenURL": "http://example.com",
  "userInfoURL": "http://example.com",
  "clientId": "string",
  "callbackURL": "http://example.com",
  "signUpURL": "http://example.com",
  "logoutURL": "http://example.com",
  "logoutRedirectURI": "http://example.com",
  "scope": "string",
  "jwksURL": "http://example.com",
  "certificate": "string"
}
```

```yaml
issuer: string
name: string
authorizationURL: http://example.com
tokenURL: http://example.com
userInfoURL: http://example.com
clientId: string
callbackURL: http://example.com
signUpURL: http://example.com
logoutURL: http://example.com
logoutRedirectURI: http://example.com
scope: string
jwksURL: http://example.com
certificate: string

```

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="create-an-identity-provider-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|[IdentityProviderRequest](schemas.md#schemaidentityproviderrequest)|true|Identity provider payload. Send JSON or an identity provider YAML file in the `identityProvider` multipart field. When YAML is used, the service reads `metadata.name` as the provider name and all other fields from `spec`.|
|orgId|path|string|true|none|

> Example responses

> 201 Response

```json
{
  "name": "string",
  "issuer": "string",
  "authorizationURL": "http://example.com",
  "tokenURL": "http://example.com",
  "clientId": "string",
  "callbackURL": "http://example.com",
  "scope": "string",
  "logoutURL": "http://example.com",
  "logoutRedirectURI": "http://example.com",
  "userInfoURL": "http://example.com",
  "signUpURL": "http://example.com",
  "jwksURL": "http://example.com",
  "certificate": "string"
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

<h3 id="create-an-identity-provider-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|201|[Created](https://tools.ietf.org/html/rfc7231#section-6.3.2)|Identity provider DTO returned by create, update, and lookup operations.|[IdentityProviderResponse](schemas.md#schemaidentityproviderresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.|Inline|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Duplicate organization data conflicts with an existing record.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="create-an-identity-provider-responseschema">Response Schema</h3>

## Update an identity provider

<a id="opIdupdateIdentityProvider"></a>

`PUT /o/{orgId}/devportal/v1/identity-providers`

> Code samples

```shell

curl -X PUT https://devportal.api-platform.io/o/{orgId}/devportal/v1/identity-providers \
  -u {username}:{password} \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}' \
  -d @payload.json

```

Updates the organization-level OIDC identity provider configuration.

> Payload

```json
{
  "issuer": "string",
  "name": "string",
  "authorizationURL": "http://example.com",
  "tokenURL": "http://example.com",
  "userInfoURL": "http://example.com",
  "clientId": "string",
  "callbackURL": "http://example.com",
  "signUpURL": "http://example.com",
  "logoutURL": "http://example.com",
  "logoutRedirectURI": "http://example.com",
  "scope": "string",
  "jwksURL": "http://example.com",
  "certificate": "string"
}
```

```yaml
issuer: string
name: string
authorizationURL: http://example.com
tokenURL: http://example.com
userInfoURL: http://example.com
clientId: string
callbackURL: http://example.com
signUpURL: http://example.com
logoutURL: http://example.com
logoutRedirectURI: http://example.com
scope: string
jwksURL: http://example.com
certificate: string

```

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="update-an-identity-provider-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|[IdentityProviderRequest](schemas.md#schemaidentityproviderrequest)|true|Identity provider payload. Send JSON or an identity provider YAML file in the `identityProvider` multipart field. When YAML is used, the service reads `metadata.name` as the provider name and all other fields from `spec`.|
|orgId|path|string|true|none|

> Example responses

> 200 Response

```json
{
  "name": "string",
  "issuer": "string",
  "authorizationURL": "http://example.com",
  "tokenURL": "http://example.com",
  "clientId": "string",
  "callbackURL": "http://example.com",
  "scope": "string",
  "logoutURL": "http://example.com",
  "logoutRedirectURI": "http://example.com",
  "userInfoURL": "http://example.com",
  "signUpURL": "http://example.com",
  "jwksURL": "http://example.com",
  "certificate": "string"
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

<h3 id="update-an-identity-provider-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Identity provider DTO returned by create, update, and lookup operations.|[IdentityProviderResponse](schemas.md#schemaidentityproviderresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.|Inline|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Resource not found.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Duplicate organization data conflicts with an existing record.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="update-an-identity-provider-responseschema">Response Schema</h3>

## Get an identity provider

<a id="opIdgetIdentityProvider"></a>

`GET /o/{orgId}/devportal/v1/identity-providers`

> Code samples

```shell

curl -X GET https://devportal.api-platform.io/o/{orgId}/devportal/v1/identity-providers \
  -u {username}:{password} \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}'

```

Retrieves the organization-level OIDC identity provider configuration.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="get-an-identity-provider-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|orgId|path|string|true|none|

> Example responses

> 200 Response

```json
{
  "name": "string",
  "issuer": "string",
  "authorizationURL": "http://example.com",
  "tokenURL": "http://example.com",
  "clientId": "string",
  "callbackURL": "http://example.com",
  "scope": "string",
  "logoutURL": "http://example.com",
  "logoutRedirectURI": "http://example.com",
  "userInfoURL": "http://example.com",
  "signUpURL": "http://example.com",
  "jwksURL": "http://example.com",
  "certificate": "string"
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

> 500 Response

```json
{
  "code": "500",
  "message": "Internal Server Error",
  "description": "Internal Server Error"
}
```

<h3 id="get-an-identity-provider-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Identity provider DTO returned by create, update, and lookup operations.|[IdentityProviderResponse](schemas.md#schemaidentityproviderresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.|Inline|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Identity provider configuration not found.|None|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="get-an-identity-provider-responseschema">Response Schema</h3>

## Delete an identity provider

<a id="opIddeleteIdentityProvider"></a>

`DELETE /o/{orgId}/devportal/v1/identity-providers`

> Code samples

```shell

curl -X DELETE https://devportal.api-platform.io/o/{orgId}/devportal/v1/identity-providers \
  -u {username}:{password} \
  -H 'Accept: text/plain' \
  -H 'Authorization: Bearer {access-token}'

```

Deletes the organization-level OIDC identity provider configuration.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="delete-an-identity-provider-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|orgId|path|string|true|none|

> Example responses

> 200 Response

```
"string"
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

> 500 Response

```json
{
  "code": "500",
  "message": "Internal Server Error",
  "description": "Internal Server Error"
}
```

<h3 id="delete-an-identity-provider-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Plain text success response.|string|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.|Inline|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Resource not found.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="delete-an-identity-provider-responseschema">Response Schema</h3>
