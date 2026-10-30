<h1 id="gateway-controller-management-api-websocket-api-management">WebSocket API Management</h1>

## Create a new WebSocketApi

<a id="opIdcreateWebSocketAPI"></a>

`POST /websocket-apis`

> Code samples

```shell

curl -X POST http://localhost:9090/api/management/v1/websocket-apis \
  -u {username}:{password} \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -d @payload.json

```

Add a new WebSocketApi to the Gateway.

> Payload

```json
{
  "apiVersion": "gateway.api-platform.wso2.com/v1",
  "kind": "WebSocketApi",
  "metadata": {
    "name": "chat-websocket-v1.0"
  },
  "spec": {
    "displayName": "Chat",
    "version": "v1.0",
    "context": "/chat/$version",
    "upstream": {
      "main": {
        "url": "wss://chat-backend:8443/ws"
      }
    },
    "channels": [
      {
        "path": "/rooms/{roomId}"
      }
    ],
    "policies": [
      {
        "name": "jwt-auth",
        "version": "v1"
      }
    ]
  }
}
```

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

Required roles: `admin`, `developer`

</aside>

<h3 id="create-a-new-websocketapi-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|[WebSocketAPIRequest](schemas.md#schemawebsocketapirequest)|true|none|

> Example responses
>
> 201 Response

```json
{
  "apiVersion": "gateway.api-platform.wso2.com/v1",
  "kind": "WebSocketApi",
  "metadata": {
    "name": "chat-websocket-v1.0"
  },
  "spec": {
    "displayName": "Chat",
    "version": "v1.0",
    "context": "/chat/$version",
    "upstream": {
      "main": {
        "url": "wss://chat-backend:8443/ws"
      }
    },
    "channels": [
      {
        "path": "/rooms/{roomId}"
      }
    ]
  },
  "status": {
    "id": "chat-websocket-v1.0",
    "state": "deployed",
    "createdAt": "2026-08-11T10:00:00Z",
    "updatedAt": "2026-08-11T10:00:00Z",
    "deployedAt": "2026-08-11T10:00:00Z"
  }
}
```

<h3 id="create-a-new-websocketapi-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|201|[Created](https://tools.ietf.org/html/rfc7231#section-6.3.2)|WebSocketApi created successfully|[WebSocketAPI](schemas.md#schemawebsocketapi)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Invalid configuration (validation failed)|[ErrorResponse](schemas.md#schemaerrorresponse)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized - authentication required|[ErrorResponse](schemas.md#schemaerrorresponse)|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Forbidden - the caller's role is not allowed to perform this operation|[ErrorResponse](schemas.md#schemaerrorresponse)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Conflict - an API with the same name and version already exists, or the context is already used on one of the API's vhosts|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

## List all WebSocketApis

<a id="opIdlistWebSocketAPIs"></a>

`GET /websocket-apis`

> Code samples

```shell

curl -X GET http://localhost:9090/api/management/v1/websocket-apis \
  -u {username}:{password} \
  -H 'Accept: application/json'

```

List WebSocketApis registered in the Gateway, optionally filtered by name, version, context, or status.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

Required roles: `admin`, `developer`

</aside>

<h3 id="list-all-websocketapis-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|displayName|query|string|false|Filter by API display name|
|version|query|string|false|Filter by API version|
|context|query|string|false|Filter by API context/path|
|status|query|string|false|Filter by deployment status|

#### Enumerated Values

|Parameter|Value|
|---|---|
|status|deployed|
|status|undeployed|

> Example responses
>
> 200 Response

```json
{
  "status": "success",
  "count": 1,
  "websocketApis": [
    {
      "apiVersion": "gateway.api-platform.wso2.com/v1",
      "kind": "WebSocketApi",
      "metadata": {
        "name": "chat-websocket-v1.0"
      },
      "spec": {
        "displayName": "Chat",
        "version": "v1.0",
        "context": "/chat/$version",
        "upstream": {
          "main": {
            "url": "wss://chat-backend:8443/ws"
          }
        },
        "channels": [
          {
            "path": "/rooms/{roomId}"
          }
        ]
      },
      "status": {
        "id": "chat-websocket-v1.0",
        "state": "deployed",
        "createdAt": "2026-08-11T10:00:00Z",
        "updatedAt": "2026-08-11T10:00:00Z",
        "deployedAt": "2026-08-11T10:00:00Z"
      }
    }
  ]
}
```

<h3 id="list-all-websocketapis-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|List of WebSocketApis|Inline|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized - authentication required|[ErrorResponse](schemas.md#schemaerrorresponse)|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Forbidden - the caller's role is not allowed to perform this operation|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="list-all-websocketapis-responseschema">Response Schema</h3>

Status Code **200**

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|» status|string|false|none|none|
|» count|integer|false|none|none|
|» websocketApis|[allOf]|false|none|none|

*allOf*

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|»» *anonymous*|[WebSocketAPIRequest](schemas.md#schemawebsocketapirequest)|false|none|none|
|»»» apiVersion|string|true|none|API specification version|
|»»» kind|string|true|none|API type|
|»»» metadata|[Metadata](schemas.md#schemametadata)|true|none|none|
|»»»» name|string|true|none|Unique handle for the resource|
|»»»» labels|object|false|none|Labels are key-value pairs for organizing and selecting APIs. Keys must not contain spaces.|
|»»»»» **additionalProperties**|string|false|none|none|
|»»»» annotations|object|false|none|Annotations are arbitrary non-identifying metadata. Use domain-prefixed keys.|
|»»»»» **additionalProperties**|string|false|none|none|
|»»» spec|[WebSocketAPIConfigData](schemas.md#schemawebsocketapiconfigdata)|true|none|none|
|»»»» displayName|string|true|none|Human-readable API name (must be URL-friendly - only letters, numbers, spaces, hyphens, underscores, and dots allowed)|
|»»»» version|string|true|none|Semantic version of the API. Both major-only (v1) and major.minor (v1.0) forms are accepted.|
|»»»» context|string|true|none|Base path for the API (must start with /, no trailing slash). Use $version to embed the version in the path (e.g., /chat/$version resolves to /chat/v1.0). Each channel's path is appended to this context to form the route a client connects to.|
|»»»» upstream|object|true|none|API-level upstream configuration. Each slot takes either a direct `ws://`/`wss://` url, which uses the gateway-wide connection defaults, or a `ref` naming an entry in upstreamDefinitions, which carries connect timeout, connection-pool bounds and tls.|
|»»»»» main|[Upstream](schemas.md#schemaupstream)|true|none|Upstream backend configuration (single target or reference)|
|»»»»»» url|string(uri)|false|none|Direct backend URL to route traffic to|
|»»»»»» ref|string|false|none|Reference to a predefined upstreamDefinition|
|»»»»»» hostRewrite|string|false|none|Controls how the Host header is handled when routing to the upstream. `auto` delegates host rewriting to Envoy, which rewrites the Host header using the upstream cluster host. `manual` disables automatic rewriting and expects explicit configuration.|
|»»»»»» tls|object|false|none|Not supported on an inline upstream (a `url`-based main/sandbox entry); a tls block placed here is rejected at deploy time. Put it on the upstreamDefinitions entry referenced via `ref`.|

*oneOf*

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|»»»»»» *anonymous*|object|false|none|none|

*xor*

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|»»»»»» *anonymous*|object|false|none|none|

*continued*

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|»»»»» sandbox|[Upstream](schemas.md#schemaupstream)|false|none|Upstream backend configuration (single target or reference)|
|»»»» upstreamDefinitions|[[WebSocketUpstreamDefinition](schemas.md#schemawebsocketupstreamdefinition)]|false|none|Reusable WebSocket backends referenced from upstream.main / upstream.sandbox via `ref`. Definition names are unique within the API.|
|»»»»» name|string|true|none|Unique identifier for this upstream definition|
|»»»»» basePath|string|false|none|Base path prefix sent to the backend for every channel (e.g., /ws).|
|»»»»» timeout|[UpstreamTimeout](schemas.md#schemaupstreamtimeout)|false|none|Timeout configuration for upstream requests|
|»»»»»» connect|string|false|none|Connection timeout duration (e.g., "5s", "500ms")|
|»»»»» connection|[WebSocketUpstreamConnection](schemas.md#schemawebsocketupstreamconnection)|false|none|Connection-count bounds for the gateway-to-backend connections of one WebSocket upstream definition, enforced per API. maxConnections falls back to the gateway-wide default when omitted, and a value above the gateway-configured upper bound is rejected at deploy time. Connection lifetime is not set here: the gateway's route idle timeout and router.websocket.max_connection_duration bound every WebSocket connection.|
|»»»»»» maxConnections|integer|false|none|Maximum concurrent connections this gateway instance opens to the backend. At the limit, at most maxPendingHandshakes further handshakes wait for a free connection; beyond that a handshake is rejected with 503. Best-effort rather than exact - Envoy guarantees at least one connection per selected host, so a multi-target definition can exceed this by a small amount, and a burst of simultaneous handshakes can briefly be admitted beyond it before the limit takes hold.|
|»»»»»» maxPendingHandshakes|integer|false|none|Maximum handshakes that may wait for a backend connection at once; any further handshake is rejected immediately with 503. Defaults to maxConnections. A waiting handshake is held until a connection frees up or the gateway's route timeout ends it. Every handshake waits briefly while its own backend connection is being opened, so a small value also rejects bursts of simultaneous handshakes below maxConnections; size it to the expected handshake burst.|
|»»»»» upstreams|[object]|true|none|Backend targets. Every target uses the same scheme - all ws:// or all wss:// - and carries host and port only; the path comes from basePath.|
|»»»»»» url|string(uri)|true|none|Backend URL (ws:// or wss://, host and port only)|
|»»»»»» weight|integer|false|none|Relative weight for load balancing across multiple targets.|
|»»»»» tls|object|false|none|Optional mutual-TLS presentation and trust for a wss:// backend, with the same fields as a RestApi upstreamDefinition: `identity` names a gateway identity certificate to present, `trustedCAs` lists usage: upstream certificates to trust in place of the gateway-wide bundle, and `verifyHostName` (default true) controls hostname verification. Every target in a definition carrying `tls` must use wss://.|
|»»»» vhosts|object|false|none|Custom virtual hosts/domains for the API, as on a RestApi. When omitted, the gateway's default main and sandbox virtual hosts apply.|
|»»»»» main|string|true|none|Custom virtual host(s)/domain(s) for production traffic. One or more hostnames<br>separated by ';' — each hostname serves the main upstream. The first entry is the<br>primary vhost. Each hostname may be a wildcard such as *.example.com.|
|»»»»» sandbox|string|false|none|Custom virtual host/domain for sandbox traffic|
|»»»» channels|[[WebSocketChannel](schemas.md#schemawebsocketchannel)]|true|none|The channels this API exposes. At least one is required, and channel paths must be unique within the API.|
|»»»»» path|string|true|none|Channel path relative to the API context, optionally containing {parameter} segments. Each parameter matches exactly one path segment.|
|»»»»» policies|[[Policy](schemas.md#schemapolicy)]|false|none|Policies applied only to this channel, in addition to the API-level policies. Every entry must be handshake-only - a policy declaring request- or response-body processing is rejected at deploy time.|
|»»»»»» name|string|true|none|Name of the policy|
|»»»»»» version|string|true|none|Version of the policy. Only major-only version is allowed (e.g., v0, v1). Full semantic version (e.g., v1.0.0) is not accepted and will be rejected. The Gateway Controller resolves the major version to the single matching full version installed in the gateway image.|
|»»»»»» executionCondition|string|false|none|Expression controlling conditional execution of the policy|
|»»»»»» params|object|false|none|Arbitrary parameters for the policy (free-form key/value structure)|
|»»»»» resilience|[Resilience](schemas.md#schemaresilience)|false|none|Backend/route timeout configuration. Maps to Envoy RouteAction timeouts. Can be set at the API level (applies to all routes) and/or the operation level (applies to that operation's route). When set at both levels, the operation-level value takes precedence. When unset, the gateway's global route timeout defaults apply.|
|»»»»»» timeout|string|false|none|Maximum time for the entire route (request to upstream response). "0s" disables the timeout.|
|»»»»»» idleTimeout|string|false|none|Per-route stream idle timeout (overrides the listener stream idle timeout for this route). "0s" disables the timeout.|
|»»»» subscriptionPlans|[string]|false|none|List of subscription plan names available for this API|
|»»»» policies|[[Policy](schemas.md#schemapolicy)]|false|none|API-level policies, applied to every channel's handshake. Every entry must be handshake-only - a policy declaring request- or response-body processing deadlocks the upgrade and is rejected at deploy time.|
|»»»» resilience|[Resilience](schemas.md#schemaresilience)|false|none|Backend/route timeout configuration. Maps to Envoy RouteAction timeouts. Can be set at the API level (applies to all routes) and/or the operation level (applies to that operation's route). When set at both levels, the operation-level value takes precedence. When unset, the gateway's global route timeout defaults apply.|
|»»»» deploymentState|string|false|none|Desired deployment state - 'deployed' (default) or 'undeployed'. When set to 'undeployed', the API is removed from router traffic but configuration, API keys, and policies are preserved for potential redeployment.|

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
|kind|WebSocketApi|
|hostRewrite|auto|
|hostRewrite|manual|
|deploymentState|deployed|
|deploymentState|undeployed|
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

## Get WebSocketApi by id

<a id="opIdgetWebSocketAPIById"></a>

`GET /websocket-apis/{id}`

> Code samples

```shell

curl -X GET http://localhost:9090/api/management/v1/websocket-apis/{id} \
  -u {username}:{password} \
  -H 'Accept: application/json'

```

Get a WebSocketApi by its ID.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

Required roles: `admin`, `developer`

</aside>

<h3 id="get-websocketapi-by-id-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|string|true|Unique public identifier for the API.|

#### Detailed descriptions

**id**: Unique public identifier for the API.

> Example responses
>
> 200 Response

```json
{
  "apiVersion": "gateway.api-platform.wso2.com/v1",
  "kind": "WebSocketApi",
  "metadata": {
    "name": "chat-websocket-v1.0"
  },
  "spec": {
    "displayName": "Chat",
    "version": "v1.0",
    "context": "/chat/$version",
    "upstream": {
      "main": {
        "url": "wss://chat-backend:8443/ws"
      }
    },
    "channels": [
      {
        "path": "/rooms/{roomId}"
      }
    ]
  },
  "status": {
    "id": "chat-websocket-v1.0",
    "state": "deployed",
    "createdAt": "2026-08-11T10:00:00Z",
    "updatedAt": "2026-08-11T10:00:00Z",
    "deployedAt": "2026-08-11T10:00:00Z"
  }
}
```

<h3 id="get-websocketapi-by-id-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|WebSocketApi details|[WebSocketAPI](schemas.md#schemawebsocketapi)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized - authentication required|[ErrorResponse](schemas.md#schemaerrorresponse)|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Forbidden - the caller's role is not allowed to perform this operation|[ErrorResponse](schemas.md#schemaerrorresponse)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|WebSocketApi not found|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

## Update an existing WebSocketApi

<a id="opIdupdateWebSocketAPI"></a>

`PUT /websocket-apis/{id}`

> Code samples

```shell

curl -X PUT http://localhost:9090/api/management/v1/websocket-apis/{id} \
  -u {username}:{password} \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -d @payload.json

```

Update an existing WebSocketApi in the Gateway.

> Payload

```json
{
  "apiVersion": "gateway.api-platform.wso2.com/v1",
  "kind": "WebSocketApi",
  "metadata": {
    "name": "chat-websocket-v1.0"
  },
  "spec": {
    "displayName": "Chat",
    "version": "v1.0",
    "context": "/chat/$version",
    "upstream": {
      "main": {
        "url": "wss://chat-backend:8443/ws"
      }
    },
    "channels": [
      {
        "path": "/rooms/{roomId}"
      }
    ],
    "policies": [
      {
        "name": "jwt-auth",
        "version": "v1"
      }
    ]
  }
}
```

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

Required roles: `admin`, `developer`

</aside>

<h3 id="update-an-existing-websocketapi-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|string|true|Unique public identifier of the API to update.|
|body|body|[WebSocketAPIRequest](schemas.md#schemawebsocketapirequest)|true|none|

#### Detailed descriptions

**id**: Unique public identifier of the API to update.

> Example responses
>
> 200 Response

```json
{
  "apiVersion": "gateway.api-platform.wso2.com/v1",
  "kind": "WebSocketApi",
  "metadata": {
    "name": "chat-websocket-v1.0"
  },
  "spec": {
    "displayName": "Chat",
    "version": "v1.0",
    "context": "/chat/$version",
    "upstream": {
      "main": {
        "url": "wss://chat-backend:8443/ws"
      }
    },
    "channels": [
      {
        "path": "/rooms/{roomId}"
      }
    ]
  },
  "status": {
    "id": "chat-websocket-v1.0",
    "state": "deployed",
    "createdAt": "2026-08-11T10:00:00Z",
    "updatedAt": "2026-08-11T10:00:00Z",
    "deployedAt": "2026-08-11T10:00:00Z"
  }
}
```

<h3 id="update-an-existing-websocketapi-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|WebSocketApi updated successfully|[WebSocketAPI](schemas.md#schemawebsocketapi)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Invalid configuration (validation failed)|[ErrorResponse](schemas.md#schemaerrorresponse)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized - authentication required|[ErrorResponse](schemas.md#schemaerrorresponse)|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Forbidden - the caller's role is not allowed to perform this operation|[ErrorResponse](schemas.md#schemaerrorresponse)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|WebSocketApi not found|[ErrorResponse](schemas.md#schemaerrorresponse)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Conflict - the updated context is already used on one of the API's vhosts by another API|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

## Delete a WebSocketApi

<a id="opIddeleteWebSocketAPI"></a>

`DELETE /websocket-apis/{id}`

> Code samples

```shell

curl -X DELETE http://localhost:9090/api/management/v1/websocket-apis/{id} \
  -u {username}:{password} \
  -H 'Accept: application/json'

```

Delete a WebSocketApi from the Gateway.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

Required roles: `admin`, `developer`

</aside>

<h3 id="delete-a-websocketapi-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|string|true|Unique public identifier of the API to delete.|

#### Detailed descriptions

**id**: Unique public identifier of the API to delete.

> Example responses
>
> 200 Response

```json
{
  "status": "success",
  "message": "WebSocketApi deleted successfully",
  "id": "chat-websocket-v1.0"
}
```

<h3 id="delete-a-websocketapi-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|WebSocketApi deleted successfully|Inline|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized - authentication required|[ErrorResponse](schemas.md#schemaerrorresponse)|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Forbidden - the caller's role is not allowed to perform this operation|[ErrorResponse](schemas.md#schemaerrorresponse)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|WebSocketApi not found|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="delete-a-websocketapi-responseschema">Response Schema</h3>

Status Code **200**

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|» status|string|false|none|none|
|» message|string|false|none|none|
|» id|string|false|none|none|

## Create a new API key for a WebSocket API

<a id="opIdcreateWebSocketAPIKey"></a>

`POST /websocket-apis/{id}/api-keys`

> Code samples

```shell

curl -X POST http://localhost:9090/api/management/v1/websocket-apis/{id}/api-keys \
  -u {username}:{password} \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -d @payload.json

```

Generate a new API key for a WebSocketApi in the Gateway. The key is a 32-byte random value encoded in hexadecimal, prefixed with `apip_`. Use the API Key policy on the API to validate incoming handshakes with this key.

> Payload

```json
{
  "name": "my-production-key"
}
```

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

Required roles: `admin`, `consumer`

</aside>

<h3 id="create-a-new-api-key-for-a-websocket-api-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|string|true|Unique public identifier of the API to generate the key for|
|body|body|[APIKeyCreationRequest](schemas.md#schemaapikeycreationrequest)|true|none|

#### Detailed descriptions

**id**: Unique public identifier of the API to generate the key for

> Example responses
>
> 201 Response

```json
{
  "status": "success",
  "message": "API key generated successfully",
  "remainingApiKeyQuota": 9,
  "apiKey": {
    "name": "my-production-key",
    "displayName": "My Production Key",
    "apiKey": "apip_1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
    "apiId": "reading-list-api-v1.0",
    "status": "active",
    "createdAt": "2026-04-01T10:30:00Z",
    "createdBy": "admin",
    "expiresAt": null,
    "source": "local"
  }
}
```

<h3 id="create-a-new-api-key-for-a-websocket-api-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|201|[Created](https://tools.ietf.org/html/rfc7231#section-6.3.2)|API key created successfully|[APIKeyCreationResponse](schemas.md#schemaapikeycreationresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Invalid configuration (validation failed)|[ErrorResponse](schemas.md#schemaerrorresponse)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized - authentication required|[ErrorResponse](schemas.md#schemaerrorresponse)|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Forbidden - the caller's role is not allowed to perform this operation|[ErrorResponse](schemas.md#schemaerrorresponse)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|WebSocketApi not found|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

## Get the list of API keys for a WebSocket API

<a id="opIdlistWebSocketAPIKeys"></a>

`GET /websocket-apis/{id}/api-keys`

> Code samples

```shell

curl -X GET http://localhost:9090/api/management/v1/websocket-apis/{id}/api-keys \
  -u {username}:{password} \
  -H 'Accept: application/json'

```

List all API keys for a WebSocketApi in the Gateway.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

Required roles: `admin`, `consumer`

</aside>

<h3 id="get-the-list-of-api-keys-for-a-websocket-api-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|string|true|Unique public identifier of the API to retrieve the keys for|

#### Detailed descriptions

**id**: Unique public identifier of the API to retrieve the keys for

> Example responses
>
> 200 Response

```json
{
  "apiKeys": [
    {
      "name": "my-production-key",
      "displayName": "My Production Key",
      "apiKey": "apip_1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
      "apiId": "reading-list-api-v1.0",
      "status": "active",
      "createdAt": "2026-04-01T10:30:00Z",
      "createdBy": "admin",
      "expiresAt": null,
      "source": "local"
    }
  ],
  "totalCount": 3,
  "status": "success"
}
```

<h3 id="get-the-list-of-api-keys-for-a-websocket-api-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|List of API keys|[APIKeyListResponse](schemas.md#schemaapikeylistresponse)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized - authentication required|[ErrorResponse](schemas.md#schemaerrorresponse)|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Forbidden - the caller's role is not allowed to perform this operation|[ErrorResponse](schemas.md#schemaerrorresponse)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|WebSocketApi not found|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

## Regenerate API key for a WebSocket API

<a id="opIdregenerateWebSocketAPIKey"></a>

`POST /websocket-apis/{id}/api-keys/{apiKeyName}/regenerate`

> Code samples

```shell

curl -X POST http://localhost:9090/api/management/v1/websocket-apis/{id}/api-keys/{apiKeyName}/regenerate \
  -u {username}:{password} \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -d @payload.json

```

Regenerate an existing API key for a WebSocketApi in the Gateway. The previous key is revoked and replaced with a new 32-byte random value encoded in hexadecimal, prefixed with `apip_`.

> Payload

```json
{}
```

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

Required roles: `admin`, `consumer`

</aside>

<h3 id="regenerate-api-key-for-a-websocket-api-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|string|true|Unique public identifier of the API to generate the key for|
|apiKeyName|path|string|true|Name of the API key to regenerate|
|body|body|[APIKeyRegenerationRequest](schemas.md#schemaapikeyregenerationrequest)|true|none|

#### Detailed descriptions

**id**: Unique public identifier of the API to generate the key for

**apiKeyName**: Name of the API key to regenerate

> Example responses
>
> 200 Response

```json
{
  "status": "success",
  "message": "API key generated successfully",
  "remainingApiKeyQuota": 9,
  "apiKey": {
    "name": "my-production-key",
    "displayName": "My Production Key",
    "apiKey": "apip_1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
    "apiId": "reading-list-api-v1.0",
    "status": "active",
    "createdAt": "2026-04-01T10:30:00Z",
    "createdBy": "admin",
    "expiresAt": null,
    "source": "local"
  }
}
```

<h3 id="regenerate-api-key-for-a-websocket-api-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|API key rotated successfully|[APIKeyCreationResponse](schemas.md#schemaapikeycreationresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Invalid configuration (validation failed)|[ErrorResponse](schemas.md#schemaerrorresponse)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized - authentication required|[ErrorResponse](schemas.md#schemaerrorresponse)|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Forbidden - the caller's role is not allowed to perform this operation|[ErrorResponse](schemas.md#schemaerrorresponse)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|WebSocketApi not found|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

## Update an API key with a new regenerated value

<a id="opIdupdateWebSocketAPIKey"></a>

`PUT /websocket-apis/{id}/api-keys/{apiKeyName}`

> Code samples

```shell

curl -X PUT http://localhost:9090/api/management/v1/websocket-apis/{id}/api-keys/{apiKeyName} \
  -u {username}:{password} \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -d @payload.json

```

Update an API key with a custom value instead of auto-generating one.

> Payload

```json
{
  "name": "my-production-key"
}
```

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

Required roles: `admin`, `consumer`

</aside>

<h3 id="update-an-api-key-with-a-new-regenerated-value-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|string|true|Unique public identifier of the API|
|apiKeyName|path|string|true|Name of the API key to update|
|body|body|[APIKeyUpdateRequest](schemas.md#schemaapikeyupdaterequest)|true|none|

#### Detailed descriptions

**id**: Unique public identifier of the API

**apiKeyName**: Name of the API key to update

> Example responses
>
> 200 Response

```json
{
  "status": "success",
  "message": "API key generated successfully",
  "remainingApiKeyQuota": 9,
  "apiKey": {
    "name": "my-production-key",
    "displayName": "My Production Key",
    "apiKey": "apip_1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
    "apiId": "reading-list-api-v1.0",
    "status": "active",
    "createdAt": "2026-04-01T10:30:00Z",
    "createdBy": "admin",
    "expiresAt": null,
    "source": "local"
  }
}
```

<h3 id="update-an-api-key-with-a-new-regenerated-value-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|API key updated successfully|[APIKeyCreationResponse](schemas.md#schemaapikeycreationresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Invalid request (validation failed)|[ErrorResponse](schemas.md#schemaerrorresponse)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized - authentication required|[ErrorResponse](schemas.md#schemaerrorresponse)|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Forbidden - the caller's role is not allowed to perform this operation|[ErrorResponse](schemas.md#schemaerrorresponse)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|API or API key not found|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

## Revoke an API key

<a id="opIdrevokeWebSocketAPIKey"></a>

`DELETE /websocket-apis/{id}/api-keys/{apiKeyName}`

> Code samples

```shell

curl -X DELETE http://localhost:9090/api/management/v1/websocket-apis/{id}/api-keys/{apiKeyName} \
  -u {username}:{password} \
  -H 'Accept: application/json'

```

Revoke an API key. Once revoked, it can no longer be used to authenticate handshakes.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

Required roles: `admin`, `consumer`

</aside>

<h3 id="revoke-an-api-key-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|string|true|Unique public identifier of the API to revoke the key for|
|apiKeyName|path|string|true|Name of the API key to revoke|

#### Detailed descriptions

**id**: Unique public identifier of the API to revoke the key for

**apiKeyName**: Name of the API key to revoke

> Example responses
>
> 200 Response

```json
{
  "status": "success",
  "message": "API key revoked successfully"
}
```

<h3 id="revoke-an-api-key-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|API key revoked successfully|[APIKeyRevocationResponse](schemas.md#schemaapikeyrevocationresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Invalid configuration (validation failed)|[ErrorResponse](schemas.md#schemaerrorresponse)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized - authentication required|[ErrorResponse](schemas.md#schemaerrorresponse)|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Forbidden - the caller's role is not allowed to perform this operation|[ErrorResponse](schemas.md#schemaerrorresponse)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|WebSocketApi not found|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|
