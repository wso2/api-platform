<h1 id="wso2-api-platform-platform-api-mcp-proxies">MCP Proxies</h1>

MCP proxy management operations

## Create a new MCP proxy

<a id="opIdcreateMCPProxy"></a>

`POST /mcp-proxies`

> Code samples

```shell

curl -X POST https://localhost:9243/api/v0.9/mcp-proxies \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -d @payload.json

```

Deploy a new MCP proxy configuration.

> Payload

```json
{
  "id": "weather-mcp-proxy",
  "displayName": "Weather Server",
  "description": "An MCP server which provides weather information",
  "version": "v1.0",
  "projectId": "550e8400-e29b-41d4-a716-446655440000",
  "context": "/",
  "vhost": "mcp.gw.com",
  "upstream": {
    "main": {
      "url": "http://prod-backend:5000/api/v2",
      "ref": "string",
      "auth": {
        "type": "api-key",
        "header": "X-API-Key",
        "value": "my-api-key-value"
      }
    },
    "sandbox": {
      "url": "http://prod-backend:5000/api/v2",
      "ref": "string",
      "auth": {
        "type": "api-key",
        "header": "X-API-Key",
        "value": "my-api-key-value"
      }
    }
  },
  "mcpSpecVersion": "2025-06-18",
  "policies": [
    {
      "executionCondition": "request.header.x-custom == 'enabled'",
      "name": "SET_HEADER",
      "params": {
        "key": "MyHeader",
        "value": "MyValue"
      },
      "version": "v1"
    }
  ],
  "kind": "Mcp",
  "capabilities": {
    "tools": [
      {}
    ],
    "resources": [
      {}
    ],
    "prompts": [
      {}
    ]
  },
  "associatedGateways": [
    {
      "id": "prod-eu"
    },
    {
      "id": "prod-us"
    }
  ]
}
```

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:mcp_proxy:create`, `ap:mcp_proxy:manage`

</aside>

<h3 id="create-a-new-mcp-proxy-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|[MCPProxy](schemas.md#schemamcpproxy)|true|none|

> Example responses

> 201 Response

```json
{
  "id": "weather-mcp-proxy",
  "displayName": "Weather Server",
  "description": "An MCP server which provides weather information",
  "createdBy": "john.doe",
  "readOnly": false,
  "updatedBy": "john.doe",
  "version": "v1.0",
  "projectId": "550e8400-e29b-41d4-a716-446655440000",
  "context": "/",
  "vhost": "mcp.gw.com",
  "upstream": {
    "main": {
      "url": "http://prod-backend:5000/api/v2",
      "ref": "string",
      "auth": {
        "type": "api-key",
        "header": "X-API-Key"
      }
    },
    "sandbox": {
      "url": "http://prod-backend:5000/api/v2",
      "ref": "string",
      "auth": {
        "type": "api-key",
        "header": "X-API-Key"
      }
    }
  },
  "mcpSpecVersion": "2025-06-18",
  "policies": [
    {
      "executionCondition": "request.header.x-custom == 'enabled'",
      "name": "SET_HEADER",
      "params": {
        "key": "MyHeader",
        "value": "MyValue"
      },
      "version": "v1"
    }
  ],
  "kind": "Mcp",
  "capabilities": {
    "tools": [
      {}
    ],
    "resources": [
      {}
    ],
    "prompts": [
      {}
    ]
  },
  "associatedGateways": [
    {
      "id": "prod-eu"
    },
    {
      "id": "prod-us"
    }
  ],
  "createdAt": "2019-08-24T14:15:22Z",
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
      "field": "spec.context",
      "message": "must start with /"
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
  "message": "The specified resource already exists."
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

<h3 id="create-a-new-mcp-proxy-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|201|[Created](https://tools.ietf.org/html/rfc7231#section-6.3.2)|MCP proxy created successfully|[MCPProxy](schemas.md#schemamcpproxy)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad Request. Invalid request or validation error.|[Error](schemas.md#schemaerror)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Forbidden. The authenticated user does not have permission to access this resource.|[Error](schemas.md#schemaerror)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Conflict. Specified resource already exists.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|

### Response Headers

|Status|Header|Type|Format|Description|
|---|---|---|---|---|
|201|Location|string|uri|URL of the newly created resource.|

## List all MCP proxies

<a id="opIdlistMCPProxies"></a>

`GET /mcp-proxies`

> Code samples

```shell

curl -X GET https://localhost:9243/api/v0.9/mcp-proxies?projectId=default-project \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Accept: application/json'

```

Retrieve a list of all MCP proxies for a project. Requires the projectId query parameter.

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:mcp_proxy:read`, `ap:mcp_proxy:manage`

</aside>

<h3 id="list-all-mcp-proxies-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|projectId|query|string|true|**Project ID** consisting of the **handle** (unique slug identifier) of the Project to filter APIs by.|
|limit|query|integer|false|Maximum number of items to return per page.|
|offset|query|integer|false|Zero-based index of the first item to return.|

#### Detailed descriptions

**projectId**: **Project ID** consisting of the **handle** (unique slug identifier) of the Project to filter APIs by.

> Example responses

> 200 Response

```json
{
  "count": 2,
  "list": [
    {
      "id": "weather-mcp-proxy",
      "displayName": "Weather Server",
      "description": "An MCP server which provides weather information",
      "createdBy": "john.doe",
      "context": "/weather-mcp-proxy",
      "version": "v1.0",
      "projectId": "550e8400-e29b-41d4-a716-446655440000",
      "status": "deployed",
      "mcpSpecVersion": "2025-11-25",
      "createdAt": "2025-11-25T10:30:00Z",
      "updatedAt": "2025-11-25T10:30:00Z",
      "readOnly": false
    }
  ],
  "pagination": {
    "total": 10,
    "offset": 0,
    "limit": 10
  }
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

<h3 id="list-all-mcp-proxies-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|List of MCP proxies|[MCPProxyListResponse](schemas.md#schemamcpproxylistresponse)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|

## Get MCP proxy by unique identifier

<a id="opIdgetMCPProxy"></a>

`GET /mcp-proxies/{mcpProxyId}`

> Code samples

```shell

curl -X GET https://localhost:9243/api/v0.9/mcp-proxies/{mcpProxyId} \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Accept: application/json'

```

Retrieve the complete configuration for a specific MCP proxy.

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:mcp_proxy:read`, `ap:mcp_proxy:manage`

</aside>

<h3 id="get-mcp-proxy-by-unique-identifier-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|mcpProxyId|path|string|true|Unique identifier of the MCP proxy|

> Example responses

> 200 Response

```json
{
  "id": "weather-mcp-proxy",
  "displayName": "Weather Server",
  "description": "An MCP server which provides weather information",
  "createdBy": "john.doe",
  "readOnly": false,
  "updatedBy": "john.doe",
  "version": "v1.0",
  "projectId": "550e8400-e29b-41d4-a716-446655440000",
  "context": "/",
  "vhost": "mcp.gw.com",
  "upstream": {
    "main": {
      "url": "http://prod-backend:5000/api/v2",
      "ref": "string",
      "auth": {
        "type": "api-key",
        "header": "X-API-Key"
      }
    },
    "sandbox": {
      "url": "http://prod-backend:5000/api/v2",
      "ref": "string",
      "auth": {
        "type": "api-key",
        "header": "X-API-Key"
      }
    }
  },
  "mcpSpecVersion": "2025-06-18",
  "policies": [
    {
      "executionCondition": "request.header.x-custom == 'enabled'",
      "name": "SET_HEADER",
      "params": {
        "key": "MyHeader",
        "value": "MyValue"
      },
      "version": "v1"
    }
  ],
  "kind": "Mcp",
  "capabilities": {
    "tools": [
      {}
    ],
    "resources": [
      {}
    ],
    "prompts": [
      {}
    ]
  },
  "associatedGateways": [
    {
      "id": "prod-eu"
    },
    {
      "id": "prod-us"
    }
  ],
  "createdAt": "2019-08-24T14:15:22Z",
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

<h3 id="get-mcp-proxy-by-unique-identifier-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|MCP proxy details|[MCPProxy](schemas.md#schemamcpproxy)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|

## Update an existing MCP proxy

<a id="opIdupdateMCPProxy"></a>

`PUT /mcp-proxies/{mcpProxyId}`

> Code samples

```shell

curl -X PUT https://localhost:9243/api/v0.9/mcp-proxies/{mcpProxyId} \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -d @payload.json

```

Update the configuration of an existing MCP proxy.

> Payload

```json
{
  "id": "weather-mcp-proxy",
  "displayName": "Weather Server",
  "description": "An MCP server which provides weather information",
  "version": "v1.0",
  "projectId": "550e8400-e29b-41d4-a716-446655440000",
  "context": "/",
  "vhost": "mcp.gw.com",
  "upstream": {
    "main": {
      "url": "http://prod-backend:5000/api/v2",
      "ref": "string",
      "auth": {
        "type": "api-key",
        "header": "X-API-Key",
        "value": "my-api-key-value"
      }
    },
    "sandbox": {
      "url": "http://prod-backend:5000/api/v2",
      "ref": "string",
      "auth": {
        "type": "api-key",
        "header": "X-API-Key",
        "value": "my-api-key-value"
      }
    }
  },
  "mcpSpecVersion": "2025-06-18",
  "policies": [
    {
      "executionCondition": "request.header.x-custom == 'enabled'",
      "name": "SET_HEADER",
      "params": {
        "key": "MyHeader",
        "value": "MyValue"
      },
      "version": "v1"
    }
  ],
  "kind": "Mcp",
  "capabilities": {
    "tools": [
      {}
    ],
    "resources": [
      {}
    ],
    "prompts": [
      {}
    ]
  },
  "associatedGateways": [
    {
      "id": "prod-eu"
    },
    {
      "id": "prod-us"
    }
  ]
}
```

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:mcp_proxy:update`, `ap:mcp_proxy:manage`

</aside>

<h3 id="update-an-existing-mcp-proxy-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|mcpProxyId|path|string|true|Unique identifier of the MCP proxy|
|body|body|[MCPProxy](schemas.md#schemamcpproxy)|true|none|

> Example responses

> 200 Response

```json
{
  "id": "weather-mcp-proxy",
  "displayName": "Weather Server",
  "description": "An MCP server which provides weather information",
  "createdBy": "john.doe",
  "readOnly": false,
  "updatedBy": "john.doe",
  "version": "v1.0",
  "projectId": "550e8400-e29b-41d4-a716-446655440000",
  "context": "/",
  "vhost": "mcp.gw.com",
  "upstream": {
    "main": {
      "url": "http://prod-backend:5000/api/v2",
      "ref": "string",
      "auth": {
        "type": "api-key",
        "header": "X-API-Key"
      }
    },
    "sandbox": {
      "url": "http://prod-backend:5000/api/v2",
      "ref": "string",
      "auth": {
        "type": "api-key",
        "header": "X-API-Key"
      }
    }
  },
  "mcpSpecVersion": "2025-06-18",
  "policies": [
    {
      "executionCondition": "request.header.x-custom == 'enabled'",
      "name": "SET_HEADER",
      "params": {
        "key": "MyHeader",
        "value": "MyValue"
      },
      "version": "v1"
    }
  ],
  "kind": "Mcp",
  "capabilities": {
    "tools": [
      {}
    ],
    "resources": [
      {}
    ],
    "prompts": [
      {}
    ]
  },
  "associatedGateways": [
    {
      "id": "prod-eu"
    },
    {
      "id": "prod-us"
    }
  ],
  "createdAt": "2019-08-24T14:15:22Z",
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
      "field": "spec.context",
      "message": "must start with /"
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

> 500 Response

```json
{
  "status": "error",
  "code": "INTERNAL_ERROR",
  "message": "An unexpected error occurred.",
  "trackingId": "4f1c6f2e-8a4b-4c93-b1de-9f2f6f0c2a11"
}
```

<h3 id="update-an-existing-mcp-proxy-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|MCP proxy updated successfully|[MCPProxy](schemas.md#schemamcpproxy)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad Request. Invalid request or validation error.|[Error](schemas.md#schemaerror)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Forbidden. The authenticated user does not have permission to access this resource.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|

## Delete an MCP proxy

<a id="opIddeleteMCPProxy"></a>

`DELETE /mcp-proxies/{mcpProxyId}`

> Code samples

```shell

curl -X DELETE https://localhost:9243/api/v0.9/mcp-proxies/{mcpProxyId} \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Accept: application/json'

```

Remove an MCP proxy.

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:mcp_proxy:delete`, `ap:mcp_proxy:manage`

</aside>

<h3 id="delete-an-mcp-proxy-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|mcpProxyId|path|string|true|Unique identifier of the MCP proxy|

> Example responses

> 400 Response

```json
{
  "status": "error",
  "code": "VALIDATION_FAILED",
  "message": "The request failed validation.",
  "errors": [
    {
      "field": "spec.context",
      "message": "must start with /"
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

> 500 Response

```json
{
  "status": "error",
  "code": "INTERNAL_ERROR",
  "message": "An unexpected error occurred.",
  "trackingId": "4f1c6f2e-8a4b-4c93-b1de-9f2f6f0c2a11"
}
```

<h3 id="delete-an-mcp-proxy-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|204|[No Content](https://tools.ietf.org/html/rfc7231#section-6.3.5)|MCP proxy deleted successfully|None|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad Request. Invalid request or validation error.|[Error](schemas.md#schemaerror)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Forbidden. The authenticated user does not have permission to access this resource.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|

## Fetch server info from MCP proxy backend services

<a id="opIdfetchMCPProxyServerInfo"></a>

`POST /mcp-proxies/fetch-server-info`

> Code samples

```shell

curl -X POST https://localhost:9243/api/v0.9/mcp-proxies/fetch-server-info \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -d @payload.json

```

Fetches server information from the backend services of an MCP proxy. 
This is used to validate connectivity and retrieve metadata about the backend services.

> Payload

```json
{
  "url": "https://mcp.server.com/mcp",
  "proxyId": "my-mcp-proxy",
  "auth": {
    "type": "api-key",
    "header": "X-API-Key",
    "value": "my-api-key-value"
  }
}
```

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:mcp_proxy:read`, `ap:mcp_proxy:manage`

</aside>

<h3 id="fetch-server-info-from-mcp-proxy-backend-services-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|[MCPServerInfoFetchRequest](schemas.md#schemamcpserverinfofetchrequest)|true|Deployment request with gateway ID, base reference, and metadata|

> Example responses

> 200 Response

```json
{
  "serverInfo": {},
  "tools": [
    {}
  ],
  "resources": [
    {}
  ],
  "prompts": [
    {}
  ]
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
      "field": "spec.context",
      "message": "must start with /"
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

<h3 id="fetch-server-info-from-mcp-proxy-backend-services-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Server info retrieved successfully|[MCPServerInfoFetchResponse](schemas.md#schemamcpserverinfofetchresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad Request. Invalid request or validation error.|[Error](schemas.md#schemaerror)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|
