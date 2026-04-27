<h1 id="gateway-controller-management-api-secrets-management">Secrets Management</h1>

CRUD operations for Secrets

## List all secrets

<a id="opIdlistSecrets"></a>

`GET /secrets`

> Code samples

```shell

curl -X GET http://localhost:9090/api/management/v0.9/secrets \
  -u {username}:{password} \
  -H 'Accept: application/json'

```

Retrieve a list of all stored secrets. Returns secret identifiers without
the actual secret values for security purposes.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

Required roles: `admin`

</aside>

> Example responses

> 200 Response

```json
{
  "status": "success",
  "count": 5,
  "secrets": [
    {
      "apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
      "kind": "Secret",
      "metadata": {
        "name": "database-password"
      },
      "spec": {
        "displayName": "Database Password"
      },
      "status": {
        "id": "database-password",
        "createdAt": "2026-04-24T07:21:13Z",
        "updatedAt": "2026-04-24T07:21:13Z"
      }
    }
  ]
}
```

<h3 id="list-all-secrets-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|List of secrets retrieved successfully|[SecretListResponse](schemas.md#schemasecretlistresponse)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized - authentication required|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

## Create a new secret

<a id="opIdcreateSecret"></a>

`POST /secrets`

> Code samples

```shell

curl -X POST http://localhost:9090/api/management/v0.9/secrets \
  -u {username}:{password} \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -d @payload.json

```

Stores a new secret encrypted at rest. The secret ID must be unique.
The value is encrypted using the primary encryption provider before persistence.

> Payload

```json
{
  "apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
  "kind": "Secret",
  "metadata": {
    "name": "database-password"
  },
  "spec": {
    "displayName": "Database Password",
    "description": "PostgreSQL main database password",
    "value": "sup3rs3cr3t!"
  }
}
```

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

Required roles: `admin`

</aside>

<h3 id="create-a-new-secret-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|[SecretConfigurationRequest](schemas.md#schemasecretconfigurationrequest)|true|none|

> Example responses

> Secret created successfully

```json
{
  "apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
  "kind": "Secret",
  "metadata": {
    "name": "database-password"
  },
  "spec": {
    "displayName": "Database Password",
    "description": "PostgreSQL main database password"
  },
  "status": {
    "id": "database-password",
    "createdAt": "2026-01-05T10:30:00Z",
    "updatedAt": "2026-01-05T10:30:00Z"
  }
}
```

> 400 Response

```json
{
  "status": "error",
  "message": "Configuration validation failed",
  "errors": [
    {
      "field": "spec.context",
      "message": "Context must start with / and cannot end with /"
    }
  ]
}
```

<h3 id="create-a-new-secret-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|201|[Created](https://tools.ietf.org/html/rfc7231#section-6.3.2)|Secret created successfully|[SecretConfigurationResponseCreateUpdate](schemas.md#schemasecretconfigurationresponsecreateupdate)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request - missing or invalid fields|[ErrorResponse](schemas.md#schemaerrorresponse)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized - authentication required|[ErrorResponse](schemas.md#schemaerrorresponse)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Conflict - secret with this ID already exists|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error - encryption failed|[ErrorResponse](schemas.md#schemaerrorresponse)|

## Retrieve a secret

<a id="opIdgetSecret"></a>

`GET /secrets/{id}`

> Code samples

```shell

curl -X GET http://localhost:9090/api/management/v0.9/secrets/{id} \
  -u {username}:{password} \
  -H 'Accept: application/json'

```

Retrieves and decrypts a secret. The secret value is decrypted using the
encryption provider chain before being returned. If all providers fail to
decrypt the secret, a 500 error is returned with a generic message.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

Required roles: `admin`

</aside>

<h3 id="retrieve-a-secret-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|string|true|Unique secret identifier|

> Example responses

> Secret retrieved and decrypted successfully

```json
{
  "apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
  "kind": "Secret",
  "metadata": {
    "name": "database-password"
  },
  "spec": {
    "displayName": "Database Password",
    "description": "PostgreSQL main database password",
    "value": "sup3rs3cr3t!"
  },
  "status": {
    "id": "database-password",
    "createdAt": "2026-01-05T10:30:00Z",
    "updatedAt": "2026-01-05T10:30:00Z"
  }
}
```

> 401 Response

```json
{
  "status": "error",
  "message": "Configuration validation failed",
  "errors": [
    {
      "field": "spec.context",
      "message": "Context must start with / and cannot end with /"
    }
  ]
}
```

<h3 id="retrieve-a-secret-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Secret retrieved and decrypted successfully|[SecretConfigurationResponseRetrieved](schemas.md#schemasecretconfigurationresponseretrieved)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized - authentication required|[ErrorResponse](schemas.md#schemaerrorresponse)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Secret configuration not found|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error - decryption failed|[ErrorResponse](schemas.md#schemaerrorresponse)|

## Update a secret

<a id="opIdupdateSecret"></a>

`PUT /secrets/{id}`

> Code samples

```shell

curl -X PUT http://localhost:9090/api/management/v0.9/secrets/{id} \
  -u {username}:{password} \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -d @payload.json

```

Updates an existing secret with a new value. The new value is encrypted
using the current primary encryption provider, enabling automatic migration
to newer keys during updates. Old secrets remain readable via the provider chain.

> Payload

```json
{
  "apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
  "kind": "Secret",
  "metadata": {
    "name": "database-password"
  },
  "spec": {
    "displayName": "Database Password",
    "description": "PostgreSQL main database password",
    "value": "sup3rs3cr3t!"
  }
}
```

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

Required roles: `admin`

</aside>

<h3 id="update-a-secret-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|[SecretConfigurationRequest](schemas.md#schemasecretconfigurationrequest)|true|none|
|id|path|string|true|Unique secret identifier|

> Example responses

> Secret updated successfully

```json
{
  "apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
  "kind": "Secret",
  "metadata": {
    "name": "database-password"
  },
  "spec": {
    "displayName": "Database Password",
    "description": "PostgreSQL main database password"
  },
  "status": {
    "id": "database-password",
    "createdAt": "2026-01-05T10:30:00Z",
    "updatedAt": "2026-01-05T11:45:00Z"
  }
}
```

> 400 Response

```json
{
  "status": "error",
  "message": "Configuration validation failed",
  "errors": [
    {
      "field": "spec.context",
      "message": "Context must start with / and cannot end with /"
    }
  ]
}
```

<h3 id="update-a-secret-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Secret updated successfully|[SecretConfigurationResponseCreateUpdate](schemas.md#schemasecretconfigurationresponsecreateupdate)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request - missing or invalid value|[ErrorResponse](schemas.md#schemaerrorresponse)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized - authentication required|[ErrorResponse](schemas.md#schemaerrorresponse)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Secret configuration not found|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error - encryption failed|[ErrorResponse](schemas.md#schemaerrorresponse)|

## Delete a secret

<a id="opIddeleteSecret"></a>

`DELETE /secrets/{id}`

> Code samples

```shell

curl -X DELETE http://localhost:9090/api/management/v0.9/secrets/{id} \
  -u {username}:{password} \
  -H 'Accept: application/json'

```

Permanently deletes a secret from the database. This is a hard delete with
no recovery mechanism. The operation is idempotent - deleting a non-existent
secret returns 404.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

Required roles: `admin`

</aside>

<h3 id="delete-a-secret-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|string|true|Unique secret identifier|

> Example responses

> 401 Response

```json
{
  "status": "error",
  "message": "Configuration validation failed",
  "errors": [
    {
      "field": "spec.context",
      "message": "Context must start with / and cannot end with /"
    }
  ]
}
```

> Secret configuration not found

```json
{
  "status": "not_found",
  "message": "secret configuration not found"
}
```

<h3 id="delete-a-secret-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Secret deleted successfully (no content)|None|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized - authentication required|[ErrorResponse](schemas.md#schemaerrorresponse)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Secret configuration not found|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error - database failure|[ErrorResponse](schemas.md#schemaerrorresponse)|
