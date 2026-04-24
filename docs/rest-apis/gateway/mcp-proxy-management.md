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
|body|body|[MCPProxyConfiguration](#schemamcpproxyconfiguration)|true|none|

> Example responses

> 201 Response

```json
{
  "status": "success",
  "message": "MCP proxy created successfully",
  "id": "everything-mcp-v1.0",
  "createdAt": "2025-12-12T10:30:00Z"
}
```

<h3 id="create-a-new-mcpproxy-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|201|[Created](https://tools.ietf.org/html/rfc7231#section-6.3.2)|MCPProxy created successfully|[MCPProxyCreateResponse](#schemamcpproxycreateresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Invalid configuration (validation failed)|[ErrorResponse](#schemaerrorresponse)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Conflict - MCP Proxy with same name and version already exists|[ErrorResponse](#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](#schemaerrorresponse)|

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
      "id": "everything-mcp-v1.0",
      "displayName": "Everything",
      "version": "v1.0",
      "context": "/everything",
      "specVersion": "2025-06-18",
      "status": "deployed",
      "createdAt": "2025-11-24T10:30:00Z",
      "updatedAt": "2025-11-24T10:30:00Z"
    }
  ]
}
```

<h3 id="list-all-mcpproxies-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|List of MCPProxies|Inline|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](#schemaerrorresponse)|

<h3 id="list-all-mcpproxies-responseschema">Response Schema</h3>

Status Code **200**

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|» status|string|false|none|none|
|» count|integer|false|none|none|
|» mcpProxies|[[MCPProxyListItem](#schemamcpproxylistitem)]|false|none|none|
|»» id|string|false|none|none|
|»» displayName|string|false|none|none|
|»» version|string|false|none|none|
|»» context|string|false|none|none|
|»» specVersion|string|false|none|none|
|»» status|string|false|none|none|
|»» createdAt|string(date-time)|false|none|none|
|»» updatedAt|string(date-time)|false|none|none|

#### Enumerated Values

|Property|Value|
|---|---|
|status|deployed|
|status|undeployed|

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
  "status": "success",
  "mcp": {
    "id": "everything-mcp-v1.0",
    "configuration": {
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
    },
    "metadata": {
      "status": "deployed",
      "createdAt": "2025-11-24T10:30:00Z",
      "updatedAt": "2025-11-24T10:30:00Z",
      "deployedAt": "2025-11-24T10:30:05Z"
    }
  }
}
```

<h3 id="get-mcpproxy-by-id-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|MCPProxy details|[MCPDetailResponse](#schemamcpdetailresponse)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|MCPProxy not found|[ErrorResponse](#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](#schemaerrorresponse)|

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
|body|body|[MCPProxyConfiguration](#schemamcpproxyconfiguration)|true|none|

#### Detailed descriptions

**id**: Unique public identifier of the MCP Proxy to update.

> Example responses

> 200 Response

```json
{
  "status": "success",
  "message": "MCP proxy updated successfully",
  "id": "everything-mcp-v1.0",
  "updatedAt": "2025-12-12T11:45:00Z"
}
```

<h3 id="update-an-existing-mcpproxy-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|MCPProxy updated successfully|[MCPProxyUpdateResponse](#schemamcpproxyupdateresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Invalid configuration (validation failed)|[ErrorResponse](#schemaerrorresponse)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|MCPProxy not found|[ErrorResponse](#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](#schemaerrorresponse)|

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
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|MCPProxy not found|[ErrorResponse](#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](#schemaerrorresponse)|

<h3 id="delete-a-mcpproxy-responseschema">Response Schema</h3>

Status Code **200**

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|» status|string|false|none|none|
|» message|string|false|none|none|
|» id|string|false|none|none|
