<h1 id="gateway-controller-management-api-backend-management">Backend Management</h1>

CRUD operations for gateway-wide Backends that APIs route to through the dynamic-endpoint policy

## Create a global backend

<a id="opIdcreateBackend"></a>

`POST /backends`

> Code samples

```shell

curl -X POST http://localhost:9090/api/management/v1/backends \
  -u {username}:{password} \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -d @payload.json

```

Add a gateway-wide Backend. The gateway turns it into one Envoy cluster, backend_<name>, that any API using the dynamic-endpoint policy can route to without the API being edited or redeployed.

> Payload

```json
{
  "apiVersion": "gateway.choreolabs.com/v1",
  "kind": "Backend",
  "metadata": {
    "name": "tenant-a",
    "labels": {
      "team": "orders"
    }
  },
  "spec": {
    "displayName": "Orders backend for tenant A",
    "basePath": "/v1",
    "endpoints": [
      {
        "url": "http://orders-tenant-a:9080"
      }
    ],
    "timeout": {
      "connect": "2s"
    }
  }
}
```

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

Required roles: `admin`

</aside>

<h3 id="create-a-global-backend-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|[BackendRequest](schemas.md#schemabackendrequest)|true|none|

> Example responses
>
> 201 Response

```json
{
  "apiVersion": "gateway.choreolabs.com/v1",
  "kind": "Backend",
  "metadata": {
    "name": "tenant-a",
    "labels": {
      "team": "orders"
    }
  },
  "spec": {
    "displayName": "Orders backend for tenant A",
    "basePath": "/v1",
    "endpoints": [
      {
        "url": "http://orders-tenant-a:9080"
      }
    ],
    "timeout": {
      "connect": "2s"
    }
  },
  "status": {
    "id": "tenant-a",
    "state": "deployed",
    "createdAt": "2026-04-24T07:21:13Z",
    "updatedAt": "2026-04-24T07:21:13Z"
  }
}
```

<h3 id="create-a-global-backend-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|201|[Created](https://tools.ietf.org/html/rfc7231#section-6.3.2)|Backend created successfully|[Backend](schemas.md#schemabackend)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Invalid configuration (validation failed)|[ErrorResponse](schemas.md#schemaerrorresponse)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Conflict - a Backend with the same name already exists|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

## List global backends

<a id="opIdlistBackends"></a>

`GET /backends`

> Code samples

```shell

curl -X GET http://localhost:9090/api/management/v1/backends \
  -u {username}:{password} \
  -H 'Accept: application/json'

```

List the gateway-wide Backends. Developers can read them to see the names and labels their APIs can select.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

Required roles: `admin`, `developer`

</aside>

> Example responses
>
> 200 Response

```json
{
  "status": "success",
  "count": 1,
  "backends": [
    {
      "apiVersion": "gateway.choreolabs.com/v1",
      "kind": "Backend",
      "metadata": {
        "name": "tenant-a",
        "labels": {
          "team": "orders"
        }
      },
      "spec": {
        "displayName": "Orders backend for tenant A",
        "basePath": "/v1",
        "endpoints": [
          {
            "url": "http://orders-tenant-a:9080"
          }
        ],
        "timeout": {
          "connect": "2s"
        }
      },
      "status": {
        "id": "tenant-a",
        "state": "deployed",
        "createdAt": "2026-04-24T07:21:13Z",
        "updatedAt": "2026-04-24T07:21:13Z"
      }
    }
  ]
}
```

<h3 id="list-global-backends-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|List of Backends|Inline|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="list-global-backends-responseschema">Response Schema</h3>

Status Code **200**

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|» status|string|false|none|none|
|» count|integer|false|none|none|
|» backends|[allOf]|false|none|none|

*allOf*

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|»» *anonymous*|[BackendRequest](schemas.md#schemabackendrequest)|false|none|none|
|»»» apiVersion|string|true|none|Backend specification version|
|»»» kind|string|true|none|Resource kind|
|»»» metadata|[Metadata](schemas.md#schemametadata)|true|none|none|
|»»»» name|string|true|none|Unique handle for the resource|
|»»»» labels|object|false|none|Labels are key-value pairs for organizing and selecting APIs. Keys must not contain spaces.|
|»»»»» **additionalProperties**|string|false|none|none|
|»»»» annotations|object|false|none|Annotations are arbitrary non-identifying metadata. Use domain-prefixed keys.|
|»»»»» **additionalProperties**|string|false|none|none|
|»»» spec|[BackendConfiguration](schemas.md#schemabackendconfiguration)|true|none|A gateway-wide backend. It carries the fields of an upstreamDefinitions entry, with its target URLs under `endpoints`. metadata.name names it: 3 to 40 characters matching ^[a-z0-9]+(-[a-z0-9]+)*$ (lowercase letters and digits, with single hyphens between them). metadata.labels are what a dynamic-endpoint backendSelector matches.|
|»»»» displayName|string|false|none|Human-readable name|
|»»»» description|string|false|none|Free-form description|
|»»»» basePath|string|false|none|Base path prefixed to every request routed to this Backend, after the API context is stripped. Must start with '/' and must not end with '/'; omit for root.|
|»»»» timeout|[UpstreamTimeout](schemas.md#schemaupstreamtimeout)|false|none|Timeout configuration for upstream requests|
|»»»»» connect|string|false|none|Connection timeout duration (e.g., "5s", "500ms")|
|»»»» endpoints|[object]|true|none|The Backend's endpoint URLs, with optional weights for load balancing. All endpoints of a Backend must use the same scheme, all http:// or all https://, with or without `tls`; a Backend carrying `tls` must use https://.|
|»»»»» url|string(uri)|true|none|Backend URL (host and port only, path comes from basePath)|
|»»»»» weight|integer|false|none|Relative weight of this endpoint; requests are spread across all of the Backend's endpoints in proportion to their weights. Omitted or 0 counts as 1.|
|»»»» tls|object|false|none|Optional mutual-TLS presentation and trust for this backend, with the same keys as an upstreamDefinitions entry: `identity`, `trustedCAs` and `verifyHostName`. Every endpoint of a Backend carrying `tls` must use https://. Unknown keys are rejected.|

*and*

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|»» *anonymous*|object|false|none|none|
|»»» status|[ResourceStatus](schemas.md#schemaresourcestatus)|false|read-only|Server-managed lifecycle fields. Populated on responses. status.id is the Backend's metadata.name. Create and update responses add status.warnings when the Backend's tls settings warrant one.|
|»»»» id|string|false|none|Unique identifier assigned by the server (equal to metadata.name)|
|»»»» state|string|false|none|Desired deployment state reported by the server|
|»»»» createdAt|string(date-time)|false|none|Timestamp when the resource was first created (UTC)|
|»»»» updatedAt|string(date-time)|false|none|Timestamp when the resource was last updated (UTC)|
|»»»» deployedAt|string(date-time)|false|none|Timestamp when the resource was last deployed (omitted when undeployed)|
|»»»» warnings|[[Warning](schemas.md#schemawarning)]|false|read-only|Non-fatal deploy-time findings (e.g. a policy's parameters resolved to something other than what was written, such as an omitted accept list inheriting the whole client-CA pool). Present only when non-empty.|
|»»»»» code|string|false|none|none|
|»»»»» field|string|false|none|none|
|»»»»» message|string|false|none|none|

#### Enumerated Values

|Property|Value|
|---|---|
|apiVersion|gateway.choreolabs.com/v1|
|kind|Backend|
|state|deployed|
|state|undeployed|
|code|MTLS_ACCEPT_INHERITS_POOL|
|code|MTLS_ACCEPT_UNNARROWED|
|code|MTLS_AUTH_NOT_FIRST|
|code|MTLS_ACCEPT_NAMES_RELAY_AUTHORITY|
|code|MTLS_THUMBPRINT_NORMALISED|
|code|HEADER_CERT_BYPASS_ACTIVE|
|code|MTLS_HOSTNAME_NOT_SCOPED|
|code|TLS_VERIFY_HOSTNAME_DISABLED|
|code|TLS_IDENTITY_EXPIRED|
|code|TLS_NO_TRUST_SOURCE|

## Get a global backend

<a id="opIdgetBackendById"></a>

`GET /backends/{id}`

> Code samples

```shell

curl -X GET http://localhost:9090/api/management/v1/backends/{id} \
  -u {username}:{password} \
  -H 'Accept: application/json'

```

Get a gateway-wide Backend by its name.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

Required roles: `admin`, `developer`

</aside>

<h3 id="get-a-global-backend-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|string|true|The Backend name (metadata.name)|

> Example responses
>
> 200 Response

```json
{
  "apiVersion": "gateway.choreolabs.com/v1",
  "kind": "Backend",
  "metadata": {
    "name": "tenant-a",
    "labels": {
      "team": "orders"
    }
  },
  "spec": {
    "displayName": "Orders backend for tenant A",
    "basePath": "/v1",
    "endpoints": [
      {
        "url": "http://orders-tenant-a:9080"
      }
    ],
    "timeout": {
      "connect": "2s"
    }
  },
  "status": {
    "id": "tenant-a",
    "state": "deployed",
    "createdAt": "2026-04-24T07:21:13Z",
    "updatedAt": "2026-04-24T07:21:13Z"
  }
}
```

<h3 id="get-a-global-backend-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Backend details|[Backend](schemas.md#schemabackend)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Backend not found|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

## Update a global backend

<a id="opIdupdateBackend"></a>

`PUT /backends/{id}`

> Code samples

```shell

curl -X PUT http://localhost:9090/api/management/v1/backends/{id} \
  -u {username}:{password} \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -d @payload.json

```

Update a gateway-wide Backend. The gateway rebuilds the Backend's cluster from its endpoints, weights, TLS settings and timeout, and then reconciles the record the policy engine reads (name, base path and labels), pushing it again only when it changed. The APIs that route to the Backend pick up the change without being edited or redeployed.

> Payload

```json
{
  "apiVersion": "gateway.choreolabs.com/v1",
  "kind": "Backend",
  "metadata": {
    "name": "tenant-a",
    "labels": {
      "team": "orders"
    }
  },
  "spec": {
    "displayName": "Orders backend for tenant A",
    "basePath": "/v1",
    "endpoints": [
      {
        "url": "http://orders-tenant-a:9080"
      }
    ],
    "timeout": {
      "connect": "2s"
    }
  }
}
```

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

Required roles: `admin`

</aside>

<h3 id="update-a-global-backend-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|string|true|The Backend name (metadata.name)|
|body|body|[BackendRequest](schemas.md#schemabackendrequest)|true|none|

> Example responses
>
> 200 Response

```json
{
  "apiVersion": "gateway.choreolabs.com/v1",
  "kind": "Backend",
  "metadata": {
    "name": "tenant-a",
    "labels": {
      "team": "orders"
    }
  },
  "spec": {
    "displayName": "Orders backend for tenant A",
    "basePath": "/v1",
    "endpoints": [
      {
        "url": "http://orders-tenant-a:9080"
      }
    ],
    "timeout": {
      "connect": "2s"
    }
  },
  "status": {
    "id": "tenant-a",
    "state": "deployed",
    "createdAt": "2026-04-24T07:21:13Z",
    "updatedAt": "2026-04-24T07:21:13Z"
  }
}
```

<h3 id="update-a-global-backend-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Backend updated successfully|[Backend](schemas.md#schemabackend)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Invalid configuration (validation failed)|[ErrorResponse](schemas.md#schemaerrorresponse)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Backend not found|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

## Delete a global backend

<a id="opIddeleteBackend"></a>

`DELETE /backends/{id}`

> Code samples

```shell

curl -X DELETE http://localhost:9090/api/management/v1/backends/{id} \
  -u {username}:{password} \
  -H 'Accept: application/json'

```

Delete a gateway-wide Backend. Refused with 409 while a deployed API names it as a fixed target (a policy parameter such as dynamic-endpoint's targetUpstream) and has no upstream definition of that name. Requests whose backendExpression selects a deleted Backend get 400 afterwards and are never misrouted.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

Required roles: `admin`

</aside>

<h3 id="delete-a-global-backend-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|string|true|The Backend name (metadata.name)|

> Example responses
>
> 200 Response

```json
{
  "status": "success",
  "message": "Backend deleted successfully",
  "id": "tenant-a"
}
```

<h3 id="delete-a-global-backend-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Backend deleted successfully|Inline|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Backend not found|[ErrorResponse](schemas.md#schemaerrorresponse)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Conflict - the Backend is the fixed target of deployed APIs|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="delete-a-global-backend-responseschema">Response Schema</h3>

Status Code **200**

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|» status|string|false|none|none|
|» message|string|false|none|none|
|» id|string|false|none|none|
