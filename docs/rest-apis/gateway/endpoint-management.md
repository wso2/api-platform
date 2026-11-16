<h1 id="gateway-controller-management-api-endpoint-management">Endpoint Management</h1>

CRUD operations for gateway-wide Endpoints

## Create a new endpoint

<a id="opIdcreateEndpoint"></a>

`POST /endpoints`

> Code samples

```shell

curl -X POST http://localhost:9090/api/management/v1/endpoints \
  -u {username}:{password} \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -d @payload.json

```

Add a gateway-wide Endpoint: a named backend, shared by every API on the gateway, that the dynamic-endpoint policy can route requests to. The Endpoint becomes routable without editing or redeploying any API.

> Payload

```json
{
  "apiVersion": "gateway.api-platform.wso2.com/v1",
  "kind": "Endpoint",
  "metadata": {
    "name": "tenant-a",
    "labels": {
      "team": "orders"
    }
  },
  "spec": {
    "displayName": "Orders backend for tenant A",
    "basePath": "/v1",
    "upstreams": [
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

<h3 id="create-a-new-endpoint-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|[EndpointConfigurationRequest](schemas.md#schemaendpointconfigurationrequest)|true|none|

> Example responses
>
> 201 Response

```json
{
  "apiVersion": "gateway.api-platform.wso2.com/v1",
  "kind": "Endpoint",
  "metadata": {
    "name": "tenant-a",
    "labels": {
      "team": "orders"
    }
  },
  "spec": {
    "displayName": "Orders backend for tenant A",
    "basePath": "/v1",
    "upstreams": [
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
    "createdAt": "2026-11-16T07:21:13Z",
    "updatedAt": "2026-11-16T07:21:13Z"
  }
}
```

<h3 id="create-a-new-endpoint-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|201|[Created](https://tools.ietf.org/html/rfc7231#section-6.3.2)|Endpoint created successfully|[EndpointConfiguration](schemas.md#schemaendpointconfiguration)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Invalid configuration (validation failed)|[ErrorResponse](schemas.md#schemaerrorresponse)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Conflict - Endpoint with same name already exists|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

## List all endpoints

<a id="opIdlistEndpoints"></a>

`GET /endpoints`

> Code samples

```shell

curl -X GET http://localhost:9090/api/management/v1/endpoints \
  -u {username}:{password} \
  -H 'Accept: application/json'

```

List the gateway-wide Endpoints, with their names and labels.

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
  "count": 2,
  "endpoints": [
    {
      "apiVersion": "gateway.api-platform.wso2.com/v1",
      "kind": "Endpoint",
      "metadata": {
        "name": "tenant-a",
        "labels": {
          "team": "orders"
        }
      },
      "spec": {
        "displayName": "Orders backend for tenant A",
        "basePath": "/v1",
        "upstreams": [
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
        "createdAt": "2026-11-16T07:21:13Z",
        "updatedAt": "2026-11-16T07:21:13Z"
      }
    }
  ]
}
```

<h3 id="list-all-endpoints-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|List of endpoints|Inline|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="list-all-endpoints-responseschema">Response Schema</h3>

Status Code **200**

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|» status|string|false|none|none|
|» count|integer|false|none|none|
|» endpoints|[allOf]|false|none|none|

*allOf*

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|»» *anonymous*|[EndpointConfigurationRequest](schemas.md#schemaendpointconfigurationrequest)|false|none|none|
|»»» apiVersion|string|true|none|Endpoint specification version|
|»»» kind|string|true|none|Endpoint kind|
|»»» metadata|[Metadata](schemas.md#schemametadata)|true|none|none|
|»»»» name|string|true|none|Unique handle for the resource|
|»»»» labels|object|false|none|Labels are key-value pairs for organizing and selecting APIs. Keys must not contain spaces.|
|»»»»» **additionalProperties**|string|false|none|none|
|»»»» annotations|object|false|none|Annotations are arbitrary non-identifying metadata. Use domain-prefixed keys.|
|»»»»» **additionalProperties**|string|false|none|none|
|»»» spec|[EndpointConfigurationData](schemas.md#schemaendpointconfigurationdata)|true|none|A gateway-wide backend. The fields are those of an upstreamDefinitions entry. metadata.name is the Endpoint's name: lowercase letters and digits in hyphen-separated segments, 3 to 40 characters. metadata.labels is what the dynamic-endpoint policy's endpointSelector matches against.|
|»»»» displayName|string|false|none|Human-readable endpoint name|
|»»»» basePath|string|false|none|Base path prefix for all requests to this endpoint (e.g., /api/v2). Must start with '/' and must not end with '/'; omit for root.|
|»»»» timeout|[UpstreamTimeout](schemas.md#schemaupstreamtimeout)|false|none|Timeout configuration for upstream requests|
|»»»»» connect|string|false|none|Connection timeout duration (e.g., "5s", "500ms")|
|»»»» upstreams|[object]|true|none|List of backend targets with optional weights for load balancing|
|»»»»» url|string(uri)|true|none|Backend URL (host and port only, path comes from basePath)|
|»»»»» weight|integer|false|none|Relative weight for load balancing across the endpoint's targets.|
|»»»» tls|object|false|none|Optional mutual-TLS presentation and trust for this backend. `identity` (string) names a gateway identity — a certificate uploaded to POST /certificates with usage: identity — to present on the connection. `trustedCAs` (array of string) lists usage: upstream certificates to trust for this backend in place of the gateway-wide trust bundle; omit to use the gateway bundle. `verifyHostName` (boolean, default true) controls SAN/hostname verification against the target. Every target of an endpoint carrying `tls` must use https://. Unknown keys are rejected.|

*and*

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|»» *anonymous*|object|false|none|none|
|»»» status|[ResourceStatus](schemas.md#schemaresourcestatus)|false|read-only|Server-managed lifecycle fields. Populated on responses.|
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
|apiVersion|gateway.api-platform.wso2.com/v1|
|kind|Endpoint|
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

## Get endpoint by id

<a id="opIdgetEndpointById"></a>

`GET /endpoints/{id}`

> Code samples

```shell

curl -X GET http://localhost:9090/api/management/v1/endpoints/{id} \
  -u {username}:{password} \
  -H 'Accept: application/json'

```

Get a gateway-wide Endpoint by its name.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

Required roles: `admin`, `developer`

</aside>

<h3 id="get-endpoint-by-id-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|string|true|Name (metadata.name) of the endpoint|

> Example responses
>
> 200 Response

```json
{
  "apiVersion": "gateway.api-platform.wso2.com/v1",
  "kind": "Endpoint",
  "metadata": {
    "name": "tenant-a",
    "labels": {
      "team": "orders"
    }
  },
  "spec": {
    "displayName": "Orders backend for tenant A",
    "basePath": "/v1",
    "upstreams": [
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
    "createdAt": "2026-11-16T07:21:13Z",
    "updatedAt": "2026-11-16T07:21:13Z"
  }
}
```

<h3 id="get-endpoint-by-id-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Endpoint details|[EndpointConfiguration](schemas.md#schemaendpointconfiguration)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Endpoint not found|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

## Update an existing endpoint

<a id="opIdupdateEndpoint"></a>

`PUT /endpoints/{id}`

> Code samples

```shell

curl -X PUT http://localhost:9090/api/management/v1/endpoints/{id} \
  -u {username}:{password} \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -d @payload.json

```

Update a gateway-wide Endpoint. Every API that routes to it follows the change; no API is redeployed. The name cannot be changed.

> Payload

```json
{
  "apiVersion": "gateway.api-platform.wso2.com/v1",
  "kind": "Endpoint",
  "metadata": {
    "name": "tenant-a",
    "labels": {
      "team": "orders"
    }
  },
  "spec": {
    "displayName": "Orders backend for tenant A",
    "basePath": "/v1",
    "upstreams": [
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

<h3 id="update-an-existing-endpoint-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|string|true|Name (metadata.name) of the endpoint to update|
|body|body|[EndpointConfigurationRequest](schemas.md#schemaendpointconfigurationrequest)|true|none|

> Example responses
>
> 200 Response

```json
{
  "apiVersion": "gateway.api-platform.wso2.com/v1",
  "kind": "Endpoint",
  "metadata": {
    "name": "tenant-a",
    "labels": {
      "team": "orders"
    }
  },
  "spec": {
    "displayName": "Orders backend for tenant A",
    "basePath": "/v1",
    "upstreams": [
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
    "createdAt": "2026-11-16T07:21:13Z",
    "updatedAt": "2026-11-16T07:21:13Z"
  }
}
```

<h3 id="update-an-existing-endpoint-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Endpoint updated successfully|[EndpointConfiguration](schemas.md#schemaendpointconfiguration)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Invalid configuration (validation failed)|[ErrorResponse](schemas.md#schemaerrorresponse)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Endpoint not found|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

## Delete an endpoint

<a id="opIddeleteEndpoint"></a>

`DELETE /endpoints/{id}`

> Code samples

```shell

curl -X DELETE http://localhost:9090/api/management/v1/endpoints/{id} \
  -u {username}:{password} \
  -H 'Accept: application/json'

```

Delete a gateway-wide Endpoint. Refused while a deployed API names the Endpoint as the fixed target of a policy. Requests that select a deleted Endpoint by header are rejected and never reach another backend.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

Required roles: `admin`

</aside>

<h3 id="delete-an-endpoint-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|string|true|Name (metadata.name) of the endpoint to delete|

> Example responses
>
> 200 Response

```json
{
  "status": "success",
  "message": "Endpoint deleted successfully",
  "id": "tenant-a"
}
```

<h3 id="delete-an-endpoint-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Endpoint deleted successfully|Inline|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Endpoint not found|[ErrorResponse](schemas.md#schemaerrorresponse)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Conflict - Endpoint is the fixed target of a deployed API|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="delete-an-endpoint-responseschema">Response Schema</h3>

Status Code **200**

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|» status|string|false|none|none|
|» message|string|false|none|none|
|» id|string|false|none|none|
