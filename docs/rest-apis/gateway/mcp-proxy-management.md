<h1 id="gateway-controller-management-api-mcp-proxy-management">MCP Proxy Management</h1>

CRUD operations for MCPProxies

## Create a new MCPProxy

<a id="opIdcreateMCPProxy"></a>

`POST /mcp-proxies`

> Code samples

```shell

curl -X POST http://localhost:9090/api/management/v0.9/mcp-proxies \
  -u {username}:{password} \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -d @payload.json

```

Add a new MCPProxy to the Gateway.

> Payload

```json
{
  "apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
  "kind": "Mcp",
  "metadata": {
    "name": "everything-mcp-v1.0"
  },
  "spec": {
    "displayName": "Everything",
    "version": "v1.0",
    "context": "/everything",
    "specVersion": "2025-06-18",
    "upstream": {
      "url": "http://everything:3001"
    },
    "tools": [],
    "resources": [],
    "prompts": []
  }
}
```

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

Required roles: `admin`, `developer`

</aside>

<h3 id="create-a-new-mcpproxy-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|[MCPProxyConfigurationRequest](schemas.md#schemamcpproxyconfigurationrequest)|true|none|

> Example responses

> 201 Response

```json
{
  "apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
  "kind": "Mcp",
  "metadata": {
    "name": "everything-mcp-v1.0"
  },
  "spec": {
    "displayName": "Everything",
    "version": "v1.0",
    "context": "/everything",
    "specVersion": "2025-06-18",
    "upstream": {
      "url": "http://everything:3001"
    },
    "tools": [],
    "resources": [],
    "prompts": []
  },
  "status": {
    "id": "everything-mcp-v1.0",
    "state": "deployed",
    "createdAt": "2026-04-24T07:21:13Z",
    "updatedAt": "2026-04-24T07:21:13Z",
    "deployedAt": "2026-04-24T07:21:13Z"
  }
}
```

<h3 id="create-a-new-mcpproxy-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|201|[Created](https://tools.ietf.org/html/rfc7231#section-6.3.2)|MCPProxy created successfully|[MCPProxyConfiguration](schemas.md#schemamcpproxyconfiguration)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Invalid configuration (validation failed)|[ErrorResponse](schemas.md#schemaerrorresponse)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Conflict - MCP Proxy with same name and version already exists|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

## List all MCPProxies

<a id="opIdlistMCPProxies"></a>

`GET /mcp-proxies`

> Code samples

```shell

curl -X GET http://localhost:9090/api/management/v0.9/mcp-proxies \
  -u {username}:{password} \
  -H 'Accept: application/json'

```

List MCPProxies registered in the Gateway, optionally filtered by name, version, context, or status.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

Required roles: `admin`, `developer`

</aside>

<h3 id="list-all-mcpproxies-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|displayName|query|string|false|Filter by MCP proxy display name|
|version|query|string|false|Filter by MCP proxy version|
|context|query|string|false|Filter by MCP proxy context/path|
|status|query|string|false|Filter by deployment status|

#### Enumerated Values

|Parameter|Value|
|---|---|
|status|deployed|
|status|undeployed|

> Example responses

> 200 Response

```json
{
  "status": "success",
  "count": 5,
  "mcpProxies": [
    {
      "apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
      "kind": "Mcp",
      "metadata": {
        "name": "everything-mcp-v1.0"
      },
      "spec": {
        "displayName": "Everything",
        "version": "v1.0",
        "context": "/everything",
        "specVersion": "2025-06-18",
        "upstream": {
          "url": "http://everything:3001"
        },
        "tools": [],
        "resources": [],
        "prompts": []
      },
      "status": {
        "id": "everything-mcp-v1.0",
        "state": "deployed",
        "createdAt": "2026-04-24T07:21:13Z",
        "updatedAt": "2026-04-24T07:21:13Z",
        "deployedAt": "2026-04-24T07:21:13Z"
      }
    }
  ]
}
```

<h3 id="list-all-mcpproxies-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|List of MCPProxies|Inline|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="list-all-mcpproxies-responseschema">Response Schema</h3>

Status Code **200**

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|» status|string|false|none|none|
|» count|integer|false|none|none|
|» mcpProxies|[allOf]|false|none|none|

*allOf*

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|»» *anonymous*|[MCPProxyConfigurationRequest](schemas.md#schemamcpproxyconfigurationrequest)|false|none|none|
|»»» apiVersion|string|true|none|MCP Proxy specification version|
|»»» kind|string|true|none|MCP Proxy type|
|»»» metadata|[Metadata](schemas.md#schemametadata)|true|none|none|
|»»»» name|string|true|none|Unique handle for the resource|
|»»»» labels|object|false|none|Labels are key-value pairs for organizing and selecting APIs. Keys must not contain spaces.|
|»»»»» **additionalProperties**|string|false|none|none|
|»»»» annotations|object|false|none|Annotations are arbitrary non-identifying metadata. Use domain-prefixed keys.|
|»»»»» **additionalProperties**|string|false|none|none|
|»»» spec|[MCPProxyConfigData](schemas.md#schemamcpproxyconfigdata)|true|none|none|
|»»»» displayName|string|true|none|Human-readable MCP Proxy display name|
|»»»» version|string|true|none|MCP Proxy version|
|»»»» context|string|false|none|MCP Proxy context path|
|»»»» specVersion|string|false|none|MCP specification version|
|»»»» vhost|string|false|none|Virtual host name used for routing. Supports standard domain names, subdomains, or wildcard domains. Must follow RFC-compliant hostname rules. Wildcards are only allowed in the left-most label (e.g., *.example.com).|
|»»»» upstream|any|true|none|The backend MCP server url and auth configurations|

*allOf*

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|»»»»» *anonymous*|[Upstream](schemas.md#schemaupstream)|false|none|Upstream backend configuration (single target or reference)|
|»»»»»» url|string(uri)|false|none|Direct backend URL to route traffic to|
|»»»»»» ref|string|false|none|Reference to a predefined upstreamDefinition|
|»»»»»» hostRewrite|string|false|none|Controls how the Host header is handled when routing to the upstream. `auto` delegates host rewriting to Envoy, which rewrites the Host header using the upstream cluster host. `manual` disables automatic rewriting and expects explicit configuration.|

*oneOf*

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|»»»»»» *anonymous*|object|false|none|none|

*xor*

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|»»»»»» *anonymous*|object|false|none|none|

*and*

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|»»»»» *anonymous*|[UpstreamAuth](schemas.md#schemaupstreamauth)|false|none|none|
|»»»»»» auth|object|false|none|none|
|»»»»»»» type|string|true|none|none|
|»»»»»»» header|string|false|none|none|
|»»»»»»» value|string|false|none|none|

*continued*

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|»»»» policies|[[Policy](schemas.md#schemapolicy)]|false|none|List of MCP Proxy level policies applied|
|»»»»» name|string|true|none|Name of the policy|
|»»»»» version|string|true|none|Version of the policy. Only major-only version is allowed (e.g., v0, v1). Full semantic version (e.g., v1.0.0) is not accepted and will be rejected. The Gateway Controller resolves the major version to the single matching full version installed in the gateway image.|
|»»»»» executionCondition|string|false|none|Expression controlling conditional execution of the policy|
|»»»»» params|object|false|none|Arbitrary parameters for the policy (free-form key/value structure)|
|»»»» tools|[[MCPTool](schemas.md#schemamcptool)]|false|none|none|
|»»»»» name|string|true|none|Unique identifier for the tool|
|»»»»» title|string|false|none|Optional human-readable name of the tool for display purposes.|
|»»»»» description|string|true|none|Human-readable description of functionality|
|»»»»» inputSchema|string|true|none|JSON Schema defining expected parameters|
|»»»»» outputSchema|string|false|none|Optional JSON Schema defining expected output structure|
|»»»» resources|[[MCPResource](schemas.md#schemamcpresource)]|false|none|none|
|»»»»» uri|string|true|none|Unique identifier for the resource|
|»»»»» name|string|true|none|The name of the resource|
|»»»»» title|string|false|none|Optional human-readable name of the resource for display purposes|
|»»»»» description|string|false|none|Optional description|
|»»»»» mimeType|string|false|none|Optional MIME type|
|»»»»» size|integer|false|none|Optional size in bytes|
|»»»» prompts|[[MCPPrompt](schemas.md#schemamcpprompt)]|false|none|none|
|»»»»» name|string|true|none|Unique identifier for the prompt|
|»»»»» title|string|false|none|Optional human-readable name of the prompt for display purposes|
|»»»»» description|string|false|none|Optional human-readable description|
|»»»»» arguments|[object]|false|none|Optional list of arguments for customization|
|»»»»»» name|string|true|none|Name of the argument|
|»»»»»» description|string|false|none|Description of the argument|
|»»»»»» required|boolean|false|none|Whether the argument is required|
|»»»»»» title|string|false|none|Optional human-readable title of the argument|
|»»»» deploymentState|string|false|none|Desired deployment state - 'deployed' (default) or 'undeployed'. When set to 'undeployed', the MCP Proxy is removed from router traffic but configuration and policies are preserved for potential redeployment.|

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

#### Enumerated Values

|Property|Value|
|---|---|
|apiVersion|gateway.api-platform.wso2.com/v1alpha1|
|kind|Mcp|
|hostRewrite|auto|
|hostRewrite|manual|
|type|api-key|
|deploymentState|deployed|
|deploymentState|undeployed|
|state|deployed|
|state|undeployed|

## Get MCPProxy by id

<a id="opIdgetMCPProxyById"></a>

`GET /mcp-proxies/{id}`

> Code samples

```shell

curl -X GET http://localhost:9090/api/management/v0.9/mcp-proxies/{id} \
  -u {username}:{password} \
  -H 'Accept: application/json'

```

Get an MCPProxy by its ID.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

Required roles: `admin`, `developer`

</aside>

<h3 id="get-mcpproxy-by-id-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|string|true|Unique public identifier of the MCP Proxy.|

#### Detailed descriptions

**id**: Unique public identifier of the MCP Proxy.

> Example responses

> 200 Response

```json
{
  "apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
  "kind": "Mcp",
  "metadata": {
    "name": "everything-mcp-v1.0"
  },
  "spec": {
    "displayName": "Everything",
    "version": "v1.0",
    "context": "/everything",
    "specVersion": "2025-06-18",
    "upstream": {
      "url": "http://everything:3001"
    },
    "tools": [],
    "resources": [],
    "prompts": []
  },
  "status": {
    "id": "everything-mcp-v1.0",
    "state": "deployed",
    "createdAt": "2026-04-24T07:21:13Z",
    "updatedAt": "2026-04-24T07:21:13Z",
    "deployedAt": "2026-04-24T07:21:13Z"
  }
}
```

<h3 id="get-mcpproxy-by-id-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|MCPProxy details|[MCPProxyConfiguration](schemas.md#schemamcpproxyconfiguration)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|MCPProxy not found|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

## Update an existing MCPProxy

<a id="opIdupdateMCPProxy"></a>

`PUT /mcp-proxies/{id}`

> Code samples

```shell

curl -X PUT http://localhost:9090/api/management/v0.9/mcp-proxies/{id} \
  -u {username}:{password} \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -d @payload.json

```

Update an existing MCPProxy in the Gateway.

> Payload

```json
{
  "apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
  "kind": "Mcp",
  "metadata": {
    "name": "everything-mcp-v1.0"
  },
  "spec": {
    "displayName": "Everything",
    "version": "v1.0",
    "context": "/everything",
    "specVersion": "2025-06-18",
    "upstream": {
      "url": "http://everything:3001"
    },
    "tools": [],
    "resources": [],
    "prompts": []
  }
}
```

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

Required roles: `admin`, `developer`

</aside>

<h3 id="update-an-existing-mcpproxy-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|string|true|Unique public identifier of the MCP Proxy to update.|
|body|body|[MCPProxyConfigurationRequest](schemas.md#schemamcpproxyconfigurationrequest)|true|none|

#### Detailed descriptions

**id**: Unique public identifier of the MCP Proxy to update.

> Example responses

> 200 Response

```json
{
  "apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
  "kind": "Mcp",
  "metadata": {
    "name": "everything-mcp-v1.0"
  },
  "spec": {
    "displayName": "Everything",
    "version": "v1.0",
    "context": "/everything",
    "specVersion": "2025-06-18",
    "upstream": {
      "url": "http://everything:3001"
    },
    "tools": [],
    "resources": [],
    "prompts": []
  },
  "status": {
    "id": "everything-mcp-v1.0",
    "state": "deployed",
    "createdAt": "2026-04-24T07:21:13Z",
    "updatedAt": "2026-04-24T07:21:13Z",
    "deployedAt": "2026-04-24T07:21:13Z"
  }
}
```

<h3 id="update-an-existing-mcpproxy-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|MCPProxy updated successfully|[MCPProxyConfiguration](schemas.md#schemamcpproxyconfiguration)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Invalid configuration (validation failed)|[ErrorResponse](schemas.md#schemaerrorresponse)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|MCPProxy not found|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

## Delete a MCPProxy

<a id="opIddeleteMCPProxy"></a>

`DELETE /mcp-proxies/{id}`

> Code samples

```shell

curl -X DELETE http://localhost:9090/api/management/v0.9/mcp-proxies/{id} \
  -u {username}:{password} \
  -H 'Accept: application/json'

```

Delete an MCPProxy from the Gateway.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

Required roles: `admin`, `developer`

</aside>

<h3 id="delete-a-mcpproxy-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|string|true|Unique public identifier of the MCP Proxy to delete.|

#### Detailed descriptions

**id**: Unique public identifier of the MCP Proxy to delete.

> Example responses

> 200 Response

```json
{
  "status": "success",
  "message": "MCPProxy deleted successfully",
  "id": "everything-mcp-v1.0"
}
```

<h3 id="delete-a-mcpproxy-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|MCPProxy deleted successfully|Inline|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|MCPProxy not found|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="delete-a-mcpproxy-responseschema">Response Schema</h3>

Status Code **200**

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|» status|string|false|none|none|
|» message|string|false|none|none|
|» id|string|false|none|none|
