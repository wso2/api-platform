<h1 id="wso2-api-platform-platform-api-agent-proxies">Agent Proxies</h1>

Agent proxy management operations

## Create a new Agent proxy

<a id="opIdcreateAgentProxy"></a>

`POST /agent-proxies`

> Code samples

```shell

curl -X POST https://localhost:9243/api/v0.9/agent-proxies \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -d @payload.json

```

Creates an Agent proxy in the organization resolved from the access token. `id` is the
public handle; when omitted the server derives it from `displayName`. `protocol` is
required and immutable, and the matching named configuration block (`a2a` for
`protocol: a2a`) is required alongside it.

> Payload

```json
{
  "kind": "AgentProxy",
  "id": "weather-agent",
  "displayName": "Weather Agent",
  "description": "Provides forecasts and severe-weather alerts",
  "version": "v1.0",
  "projectId": "default-project",
  "context": "/weather",
  "vhost": "agents.gw.com",
  "upstream": {
    "main": {
      "url": "http://prod-backend:5000/api/v2",
      "auth": {
        "type": "api-key",
        "header": "X-API-Key",
        "value": "my-api-key-value"
      }
    },
    "sandbox": {
      "url": "http://prod-backend:5000/api/v2",
      "auth": {
        "type": "api-key",
        "header": "X-API-Key",
        "value": "my-api-key-value"
      }
    }
  },
  "resilience": {
    "timeout": "30s",
    "idleTimeout": "5s"
  },
  "protocol": "a2a",
  "a2a": {
    "protocolVersion": "1.0",
    "operationConfigs": {
      "transports": [
        {
          "protocolBinding": "JSONRPC",
          "pathPrefix": "/rpc"
        }
      ],
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
      "operations": [
        {
          "name": "SendMessage",
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
          "resilience": {
            "timeout": "30s",
            "idleTimeout": "5s"
          }
        }
      ]
    },
    "agentCard": {
      "public": {
        "mode": "managed",
        "path": "/.well-known/agent-card.json",
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
        "rewriteUrls": true,
        "content": {}
      },
      "protected": {
        "mode": "managed",
        "rewriteUrls": true,
        "content": {}
      }
    }
  },
  "associatedGateways": [
    {
      "id": "ai-gw-prod"
    }
  ]
}
```

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:api:agent:create`, `ap:api:agent:manage`, `ap:api:manage`, `ap:manage`

</aside>

<h3 id="create-a-new-agent-proxy-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|[AgentProxy](schemas.md#schemaagentproxy)|true|Agent proxy to create|

> Example responses
>
> 201 Response

```json
{
  "kind": "AgentProxy",
  "id": "weather-agent",
  "displayName": "Weather Agent",
  "description": "Provides forecasts and severe-weather alerts",
  "version": "v1.0",
  "projectId": "default-project",
  "context": "/weather",
  "vhost": "agents.gw.com",
  "upstream": {
    "main": {
      "url": "http://prod-backend:5000/api/v2",
      "auth": {
        "type": "api-key",
        "header": "X-API-Key",
        "value": "my-api-key-value"
      }
    },
    "sandbox": {
      "url": "http://prod-backend:5000/api/v2",
      "auth": {
        "type": "api-key",
        "header": "X-API-Key",
        "value": "my-api-key-value"
      }
    }
  },
  "resilience": {
    "timeout": "30s",
    "idleTimeout": "5s"
  },
  "protocol": "a2a",
  "a2a": {
    "protocolVersion": "1.0",
    "operationConfigs": {
      "transports": [
        {
          "protocolBinding": "JSONRPC",
          "pathPrefix": "/rpc"
        }
      ],
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
      "operations": [
        {
          "name": "SendMessage",
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
          "resilience": {
            "timeout": "30s",
            "idleTimeout": "5s"
          }
        }
      ]
    },
    "agentCard": {
      "public": {
        "mode": "managed",
        "path": "/.well-known/agent-card.json",
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
        "rewriteUrls": true,
        "content": {}
      },
      "protected": {
        "mode": "managed",
        "rewriteUrls": true,
        "content": {}
      }
    }
  },
  "associatedGateways": [
    {
      "id": "ai-gw-prod"
    }
  ],
  "createdBy": "john.doe",
  "updatedBy": "john.doe",
  "readOnly": false,
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

> 415 Response

```json
{
  "status": "error",
  "code": "UNSUPPORTED_MEDIA_TYPE",
  "message": "This operation accepts application/json only."
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

<h3 id="create-a-new-agent-proxy-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|201|[Created](https://tools.ietf.org/html/rfc7231#section-6.3.2)|Agent proxy created successfully|[AgentProxy](schemas.md#schemaagentproxy)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad Request. Invalid request or validation error.|[Error](schemas.md#schemaerror)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Forbidden. The authenticated user does not have permission to access this resource.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Conflict. The request conflicts with the current state of the resource.|[Error](schemas.md#schemaerror)|
|415|[Unsupported Media Type](https://tools.ietf.org/html/rfc7231#section-6.5.13)|Unsupported Media Type. The request body media type is not supported by this operation.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|

### Response Headers

|Status|Header|Type|Format|Description|
|---|---|---|---|---|
|201|Location|string|uri|URL of the newly created resource.|

## List Agent proxies

<a id="opIdlistAgentProxies"></a>

`GET /agent-proxies`

> Code samples

```shell

curl -X GET https://localhost:9243/api/v0.9/agent-proxies \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Accept: application/json'

```

Lists the Agent proxies in the caller's organization. Items are the lightweight
`AgentProxyListItem` projection; read an individual Agent proxy for the full resource,
including its protocol configuration and any managed Agent Card content. The `protocol`
and `projectId` filters, when supplied, are applied to both the returned page and
`pagination.total`. Filtering by A2A protocol version is not supported.

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:api:agent:read`, `ap:api:agent:manage`, `ap:api:read`, `ap:api:manage`, `ap:read`, `ap:manage`

</aside>

<h3 id="list-agent-proxies-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|protocol|query|string|false|Filter Agent proxies by communication protocol. Omit to list every protocol variant.|
|projectId|query|string|false|**Project ID** consisting of the **handle** (unique slug identifier) of the Project whose|
|limit|query|integer|false|Maximum number of items to return per page.|
|offset|query|integer|false|Zero-based index of the first item to return.|
|sortBy|query|string|false|Field to sort the collection by. An unrecognized value falls back to the default sort (createdAt).|
|sortOrder|query|string|false|Sort direction applied to `sortBy`.|
|query|query|string|false|Case-insensitive substring filter matched against the resource display name and id (handle).|

#### Detailed descriptions

**protocol**: Filter Agent proxies by communication protocol. Omit to list every protocol variant.
An empty or unsupported value is rejected with 400. The filter applies to the returned
page and to `pagination.total` alike, always within the authenticated organization.

**projectId**: **Project ID** consisting of the **handle** (unique slug identifier) of the Project whose
Agent proxies should be returned. Omit to list Agent proxies across every project in the
organization. An empty value is rejected with 400, and a handle that does not resolve to a
project in the authenticated organization is rejected with 404. The filter applies to the
returned page and to `pagination.total` alike.

#### Enumerated Values

|Parameter|Value|
|---|---|
|protocol|a2a|
|sortBy|name|
|sortBy|createdAt|
|sortOrder|asc|
|sortOrder|desc|

> Example responses
>
> 200 Response

```json
{
  "count": 2,
  "list": [
    {
      "id": "weather-agent",
      "displayName": "Weather Agent",
      "description": "Provides forecasts and severe-weather alerts",
      "version": "v1.0",
      "projectId": "default-project",
      "protocol": "a2a",
      "context": "/weather",
      "vhost": "agents.gw.com",
      "createdBy": "john.doe",
      "updatedBy": "john.doe",
      "readOnly": false,
      "createdAt": "2026-01-31T10:30:00Z",
      "updatedAt": "2026-01-31T10:30:00Z"
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

<h3 id="list-agent-proxies-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|List of Agent proxies|[AgentProxyListResponse](schemas.md#schemaagentproxylistresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad Request. Invalid request or validation error.|[Error](schemas.md#schemaerror)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|

## Fetch an Agent Card for preview

<a id="opIdfetchAgentCard"></a>

`POST /agent-proxies/fetch-agent-card`

> Code samples

```shell

curl -X POST https://localhost:9243/api/v0.9/agent-proxies/fetch-agent-card \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -H 'Cache-Control: no-cache' \
  -d @payload.json

```

Provide either `url` with optional `auth`, or `agentProxyId` alone to use the stored
upstream URL and stored credentials. Combining `agentProxyId` with `url` or `auth` is
rejected. The fetched card is returned for preview and is never persisted.

This operation has no side effects, so it is guarded by the read scope. Saving a
fetched card as managed content is a separate Agent proxy update that places it under
`a2a.agentCard.public.content` or `a2a.agentCard.protected.content`.

This is also the display path for a passthrough public Agent Card, which is not
stored, so it runs on ordinary page views and not only during authoring. It issues a
live outbound request: when the upstream is unreachable, times out, rejects the stored
credentials, or returns something that is not a parsable Agent Card, the response is
`503` with `AGENT_PROXY_UPSTREAM_UNREACHABLE` — distinct from `500`, so a client can
tell an unreachable agent from a control-plane failure and render the right state. A
managed public card needs no call here; it is already in the Agent proxy resource.

> Payload

```json
{
  "url": "https://agent.example.com"
}
```

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:api:agent:read`, `ap:api:agent:manage`, `ap:api:read`, `ap:api:manage`, `ap:read`, `ap:manage`

</aside>

<h3 id="fetch-an-agent-card-for-preview-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|Cache-Control|header|string|false|Send `no-cache` to bypass the control plane's cached Agent Card result and force a live|
|body|body|[FetchAgentCardRequest](schemas.md#schemafetchagentcardrequest)|true|Either a direct `url` (with optional `auth`), or an `agentProxyId` to fetch using a stored Agent proxy's endpoint and credentials.|

#### Detailed descriptions

**Cache-Control**: Send `no-cache` to bypass the control plane's cached Agent Card result and force a live
upstream fetch — the explicit "refresh" action in an authoring UI. Any other value, or
omission, allows a cached result. Per RFC 9110 this affects freshness only; it never
changes what is stored.

#### Enumerated Values

|Parameter|Value|
|---|---|
|Cache-Control|no-cache|

> Example responses
>
> 200 Response

```json
{}
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

> 404 Response

```json
{
  "status": "error",
  "code": "NOT_FOUND",
  "message": "The specified resource does not exist."
}
```

> 415 Response

```json
{
  "status": "error",
  "code": "UNSUPPORTED_MEDIA_TYPE",
  "message": "This operation accepts application/json only."
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

> 503 Response

```json
{
  "status": "error",
  "code": "AGENT_PROXY_UPSTREAM_UNREACHABLE",
  "message": "The control plane could not reach the upstream to fetch its Agent Card.",
  "trackingId": "4f1c6f2e-8a4b-4c93-b1de-9f2f6f0c2a11"
}
```

<h3 id="fetch-an-agent-card-for-preview-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Parsed, transient Agent Card preview. Possibly served from the control plane's
short-lived cache — see `Age`. Freshness metadata is carried in headers, never
injected into the body: the card document is free-form and is returned exactly as
the upstream supplied it.|[AgentCardDocument](schemas.md#schemaagentcarddocument)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad Request. Invalid request or validation error.|[Error](schemas.md#schemaerror)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|415|[Unsupported Media Type](https://tools.ietf.org/html/rfc7231#section-6.5.13)|Unsupported Media Type. The request body media type is not supported by this operation.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|
|503|[Service Unavailable](https://tools.ietf.org/html/rfc7231#section-6.6.4)|Service Unavailable. The control plane could not reach the Agent proxy's upstream to
retrieve its Agent Card — unreachable, timed out, credentials rejected, or an
unparsable response. This describes the control plane's own reachability of the
upstream: it is not a statement about whether a deployed gateway can serve the card,
and it is not a deployment status.

Failures are cached for a shorter interval than successes, so a persistently
unreachable upstream is not re-contacted on every page view. A positive `Age` means
this failure was served from that negative cache.|[Error](schemas.md#schemaerror)|

### Response Headers

|Status|Header|Type|Format|Description|
|---|---|---|---|---|
|200|Cache-Control|string||Freshness of an Agent Card fetch result, as `max-age=<seconds>`. The control plane
caches the stored-handle form of the fetch for this long so routine page views do not
each issue an outbound request. `max-age=0` means caching is disabled.
|
|200|Age|integer||Seconds since the control plane actually fetched this result from the upstream. `0` or
absent means it was fetched during this request; a positive value means it was served
from the cache. Present on a cached failure too, so a client can say how long the
upstream has been unreachable.
|
|503|Age|integer||Seconds since the control plane actually fetched this result from the upstream. `0` or
absent means it was fetched during this request; a positive value means it was served
from the cache. Present on a cached failure too, so a client can say how long the
upstream has been unreachable.
|

## Get an Agent proxy by unique identifier

<a id="opIdgetAgentProxy"></a>

`GET /agent-proxies/{agentProxyId}`

> Code samples

```shell

curl -X GET https://localhost:9243/api/v0.9/agent-proxies/{agentProxyId} \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Accept: application/json'

```

Returns the full Agent proxy, including its typed protocol configuration and any managed Agent Card content.

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:api:agent:read`, `ap:api:agent:manage`, `ap:api:read`, `ap:api:manage`, `ap:read`, `ap:manage`

</aside>

<h3 id="get-an-agent-proxy-by-unique-identifier-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|agentProxyId|path|string|true|**Agent Proxy ID** consisting of the **handle** (unique slug identifier) of the Agent proxy.|

#### Detailed descriptions

**agentProxyId**: **Agent Proxy ID** consisting of the **handle** (unique slug identifier) of the Agent proxy.

> Example responses
>
> 200 Response

```json
{
  "kind": "AgentProxy",
  "id": "weather-agent",
  "displayName": "Weather Agent",
  "description": "Provides forecasts and severe-weather alerts",
  "version": "v1.0",
  "projectId": "default-project",
  "context": "/weather",
  "vhost": "agents.gw.com",
  "upstream": {
    "main": {
      "url": "http://prod-backend:5000/api/v2",
      "auth": {
        "type": "api-key",
        "header": "X-API-Key",
        "value": "my-api-key-value"
      }
    },
    "sandbox": {
      "url": "http://prod-backend:5000/api/v2",
      "auth": {
        "type": "api-key",
        "header": "X-API-Key",
        "value": "my-api-key-value"
      }
    }
  },
  "resilience": {
    "timeout": "30s",
    "idleTimeout": "5s"
  },
  "protocol": "a2a",
  "a2a": {
    "protocolVersion": "1.0",
    "operationConfigs": {
      "transports": [
        {
          "protocolBinding": "JSONRPC",
          "pathPrefix": "/rpc"
        }
      ],
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
      "operations": [
        {
          "name": "SendMessage",
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
          "resilience": {
            "timeout": "30s",
            "idleTimeout": "5s"
          }
        }
      ]
    },
    "agentCard": {
      "public": {
        "mode": "managed",
        "path": "/.well-known/agent-card.json",
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
        "rewriteUrls": true,
        "content": {}
      },
      "protected": {
        "mode": "managed",
        "rewriteUrls": true,
        "content": {}
      }
    }
  },
  "associatedGateways": [
    {
      "id": "ai-gw-prod"
    }
  ],
  "createdBy": "john.doe",
  "updatedBy": "john.doe",
  "readOnly": false,
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

<h3 id="get-an-agent-proxy-by-unique-identifier-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Agent proxy details|[AgentProxy](schemas.md#schemaagentproxy)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|

## Replace an existing Agent proxy

<a id="opIdupdateAgentProxy"></a>

`PUT /agent-proxies/{agentProxyId}`

> Code samples

```shell

curl -X PUT https://localhost:9243/api/v0.9/agent-proxies/{agentProxyId} \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -d @payload.json

```

Full replacement of the writable Agent proxy configuration; idempotent. Omitted
optional fields reset to their documented default or absence — omitting
`a2a.agentCard` restores default card behaviour, omitting
`a2a.operationConfigs.policies` or `a2a.operationConfigs.operations` clears those user
additions, and omitting `associatedGateways` empties the association set (subject to
the existing deployment-integrity checks). `a2a.operationConfigs` itself is required,
since it carries the transports.

An omitted body `id` retains the path handle and is never regenerated; a body `id`
that disagrees with the path is a 400. `protocol` is immutable — changing it is a 400.
An omitted write-only credential value retains the stored credential for an unchanged
auth configuration, and `auth.type: none` removes auth; an auth configuration that
changes without complete credentials is rejected. There is no implicit PATCH
behaviour.

For a gateway-originated Agent proxy (`readOnly: true`), only `description` and
`associatedGateways` are replaced from the body; every other field keeps the value
imported from the gateway, and `readOnly` in the body is ignored.

> Payload

```json
{
  "kind": "AgentProxy",
  "id": "weather-agent",
  "displayName": "Weather Agent",
  "description": "Provides forecasts and severe-weather alerts",
  "version": "v1.0",
  "projectId": "default-project",
  "context": "/weather",
  "vhost": "agents.gw.com",
  "upstream": {
    "main": {
      "url": "http://prod-backend:5000/api/v2",
      "auth": {
        "type": "api-key",
        "header": "X-API-Key",
        "value": "my-api-key-value"
      }
    },
    "sandbox": {
      "url": "http://prod-backend:5000/api/v2",
      "auth": {
        "type": "api-key",
        "header": "X-API-Key",
        "value": "my-api-key-value"
      }
    }
  },
  "resilience": {
    "timeout": "30s",
    "idleTimeout": "5s"
  },
  "protocol": "a2a",
  "a2a": {
    "protocolVersion": "1.0",
    "operationConfigs": {
      "transports": [
        {
          "protocolBinding": "JSONRPC",
          "pathPrefix": "/rpc"
        }
      ],
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
      "operations": [
        {
          "name": "SendMessage",
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
          "resilience": {
            "timeout": "30s",
            "idleTimeout": "5s"
          }
        }
      ]
    },
    "agentCard": {
      "public": {
        "mode": "managed",
        "path": "/.well-known/agent-card.json",
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
        "rewriteUrls": true,
        "content": {}
      },
      "protected": {
        "mode": "managed",
        "rewriteUrls": true,
        "content": {}
      }
    }
  },
  "associatedGateways": [
    {
      "id": "ai-gw-prod"
    }
  ]
}
```

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:api:agent:update`, `ap:api:agent:manage`, `ap:api:manage`, `ap:manage`

</aside>

<h3 id="replace-an-existing-agent-proxy-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|agentProxyId|path|string|true|**Agent Proxy ID** consisting of the **handle** (unique slug identifier) of the Agent proxy.|
|body|body|[AgentProxy](schemas.md#schemaagentproxy)|true|Replacement Agent proxy configuration|

#### Detailed descriptions

**agentProxyId**: **Agent Proxy ID** consisting of the **handle** (unique slug identifier) of the Agent proxy.

> Example responses
>
> 200 Response

```json
{
  "kind": "AgentProxy",
  "id": "weather-agent",
  "displayName": "Weather Agent",
  "description": "Provides forecasts and severe-weather alerts",
  "version": "v1.0",
  "projectId": "default-project",
  "context": "/weather",
  "vhost": "agents.gw.com",
  "upstream": {
    "main": {
      "url": "http://prod-backend:5000/api/v2",
      "auth": {
        "type": "api-key",
        "header": "X-API-Key",
        "value": "my-api-key-value"
      }
    },
    "sandbox": {
      "url": "http://prod-backend:5000/api/v2",
      "auth": {
        "type": "api-key",
        "header": "X-API-Key",
        "value": "my-api-key-value"
      }
    }
  },
  "resilience": {
    "timeout": "30s",
    "idleTimeout": "5s"
  },
  "protocol": "a2a",
  "a2a": {
    "protocolVersion": "1.0",
    "operationConfigs": {
      "transports": [
        {
          "protocolBinding": "JSONRPC",
          "pathPrefix": "/rpc"
        }
      ],
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
      "operations": [
        {
          "name": "SendMessage",
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
          "resilience": {
            "timeout": "30s",
            "idleTimeout": "5s"
          }
        }
      ]
    },
    "agentCard": {
      "public": {
        "mode": "managed",
        "path": "/.well-known/agent-card.json",
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
        "rewriteUrls": true,
        "content": {}
      },
      "protected": {
        "mode": "managed",
        "rewriteUrls": true,
        "content": {}
      }
    }
  },
  "associatedGateways": [
    {
      "id": "ai-gw-prod"
    }
  ],
  "createdBy": "john.doe",
  "updatedBy": "john.doe",
  "readOnly": false,
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

> 415 Response

```json
{
  "status": "error",
  "code": "UNSUPPORTED_MEDIA_TYPE",
  "message": "This operation accepts application/json only."
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

<h3 id="replace-an-existing-agent-proxy-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Agent proxy replaced successfully|[AgentProxy](schemas.md#schemaagentproxy)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad Request. Invalid request or validation error.|[Error](schemas.md#schemaerror)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Forbidden. The authenticated user does not have permission to access this resource.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Conflict. The request conflicts with the current state of the resource.|[Error](schemas.md#schemaerror)|
|415|[Unsupported Media Type](https://tools.ietf.org/html/rfc7231#section-6.5.13)|Unsupported Media Type. The request body media type is not supported by this operation.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|

## Delete an Agent proxy

<a id="opIddeleteAgentProxy"></a>

`DELETE /agent-proxies/{agentProxyId}`

> Code samples

```shell

curl -X DELETE https://localhost:9243/api/v0.9/agent-proxies/{agentProxyId} \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Accept: application/json'

```

Deletes the Agent proxy from the control plane and durably records the deletion event.
Gateway cleanup converges asynchronously, so a 204 is not a promise that every gateway
has already removed the routes.

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:api:agent:delete`, `ap:api:agent:manage`, `ap:api:manage`, `ap:manage`

</aside>

<h3 id="delete-an-agent-proxy-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|agentProxyId|path|string|true|**Agent Proxy ID** consisting of the **handle** (unique slug identifier) of the Agent proxy.|

#### Detailed descriptions

**agentProxyId**: **Agent Proxy ID** consisting of the **handle** (unique slug identifier) of the Agent proxy.

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

> 500 Response

```json
{
  "status": "error",
  "code": "INTERNAL_ERROR",
  "message": "An unexpected error occurred.",
  "trackingId": "4f1c6f2e-8a4b-4c93-b1de-9f2f6f0c2a11"
}
```

<h3 id="delete-an-agent-proxy-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|204|[No Content](https://tools.ietf.org/html/rfc7231#section-6.3.5)|Agent proxy deleted successfully|None|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Forbidden. The authenticated user does not have permission to access this resource.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|

## List API keys for an Agent proxy

<a id="opIdlistAgentProxyAPIKeys"></a>

`GET /agent-proxies/{agentProxyId}/api-keys`

> Code samples

```shell

curl -X GET https://localhost:9243/api/v0.9/agent-proxies/{agentProxyId}/api-keys \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Accept: application/json'

```

Returns API-key metadata for the specified Agent proxy. Key material is never returned by
this operation. Only keys the caller created are listed, unless the caller holds
`ap:credential:api_key:all:manage`, which lists every key on the Agent proxy.

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:api:agent:api_key:read`, `ap:api:agent:api_key:manage`, `ap:api:agent:read`, `ap:api:agent:manage`, `ap:api:read`, `ap:api:manage`, `ap:read`, `ap:manage`, `ap:credential:api_key:all:manage`

</aside>

<h3 id="list-api-keys-for-an-agent-proxy-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|agentProxyId|path|string|true|**Agent Proxy ID** consisting of the **handle** (unique slug identifier) of the Agent proxy.|
|limit|query|integer|false|Maximum number of items to return per page.|
|offset|query|integer|false|Zero-based index of the first item to return.|

#### Detailed descriptions

**agentProxyId**: **Agent Proxy ID** consisting of the **handle** (unique slug identifier) of the Agent proxy.

> Example responses
>
> 200 Response

```json
{
  "list": [
    {
      "id": "string",
      "displayName": "string",
      "maskedApiKey": "string",
      "status": "active",
      "createdAt": "2019-08-24T14:15:22Z",
      "createdBy": "john.doe",
      "updatedAt": "2019-08-24T14:15:22Z",
      "expiresAt": "2019-08-24T14:15:22Z",
      "issuer": "api-platform-devportal",
      "allowedTargets": "string"
    }
  ],
  "count": 0,
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

<h3 id="list-api-keys-for-an-agent-proxy-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|API-key metadata list retrieved successfully. Key material is never returned.|[AgentProxyAPIKeyListResponse](schemas.md#schemaagentproxyapikeylistresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad Request. Invalid request or validation error.|[Error](schemas.md#schemaerror)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Forbidden. The authenticated user does not have permission to access this resource.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|

## Create an API key for an Agent proxy

<a id="opIdcreateAgentProxyAPIKey"></a>

`POST /agent-proxies/{agentProxyId}/api-keys`

> Code samples

```shell

curl -X POST https://localhost:9243/api/v0.9/agent-proxies/{agentProxyId}/api-keys \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -d @payload.json

```

Creates an API key for the specified Agent proxy, with the same behavior as REST API
keys. Omit `apiKey` to have the server generate one; the generated value is returned
only in this response and can never be retrieved again. A supplied `apiKey` is
registered as-is and is not echoed back. The key is hashed before storage and
broadcast to the gateways the Agent proxy is deployed to; a gateway the Agent proxy is
deployed to later receives it at deployment time.

`id` is optional and derived from `displayName` when omitted. If the identifier is
already in use on this Agent proxy, a short random suffix is appended. The response's
`keyId` and the `Location` header carry the identifier actually stored.

> Payload

```json
{
  "id": "production-key-01",
  "displayName": "Production API Key",
  "apiKey": "sk_example_1234567890abcdef",
  "externalRefId": "ext-ref-12345",
  "expiresAt": "2026-12-31T23:59:59Z",
  "expiresIn": {
    "duration": 30,
    "unit": "days"
  },
  "issuer": "api-platform-devportal"
}
```

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:api:agent:api_key:create`, `ap:api:agent:api_key:manage`, `ap:api:agent:manage`, `ap:api:manage`, `ap:manage`, `ap:credential:api_key:all:manage`

</aside>

<h3 id="create-an-api-key-for-an-agent-proxy-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|agentProxyId|path|string|true|**Agent Proxy ID** consisting of the **handle** (unique slug identifier) of the Agent proxy.|
|body|body|[CreateAPIKeyRequest](schemas.md#schemacreateapikeyrequest)|true|API key creation request|

#### Detailed descriptions

**agentProxyId**: **Agent Proxy ID** consisting of the **handle** (unique slug identifier) of the Agent proxy.

> Example responses
>
> 201 Response

```json
{
  "status": "success",
  "message": "API key created and broadcasted to gateways successfully",
  "keyId": "production-key-01",
  "apiKey": "sk_example_1234567890abcdef"
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

> 413 Response

```json
{
  "status": "error",
  "code": "PAYLOAD_TOO_LARGE",
  "message": "The uploaded file exceeds the maximum allowed size."
}
```

> 415 Response

```json
{
  "status": "error",
  "code": "UNSUPPORTED_MEDIA_TYPE",
  "message": "This operation accepts application/json only."
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

<h3 id="create-an-api-key-for-an-agent-proxy-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|201|[Created](https://tools.ietf.org/html/rfc7231#section-6.3.2)|API key created successfully. Generated key material is returned only in this response.|[CreateAPIKeyResponse](schemas.md#schemacreateapikeyresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad Request. Invalid request or validation error.|[Error](schemas.md#schemaerror)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Forbidden. The authenticated user does not have permission to access this resource.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|413|[Payload Too Large](https://tools.ietf.org/html/rfc7231#section-6.5.11)|Payload Too Large. The uploaded file exceeds the maximum allowed size.|[Error](schemas.md#schemaerror)|
|415|[Unsupported Media Type](https://tools.ietf.org/html/rfc7231#section-6.5.13)|Unsupported Media Type. The request body media type is not supported by this operation.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|

### Response Headers

|Status|Header|Type|Format|Description|
|---|---|---|---|---|
|201|Location|string|uri|URL of the newly created resource.|

## Update an API key for an Agent proxy

<a id="opIdupdateAgentProxyAPIKey"></a>

`PUT /agent-proxies/{agentProxyId}/api-keys/{apiKeyId}`

> Code samples

```shell

curl -X PUT https://localhost:9243/api/v0.9/agent-proxies/{agentProxyId}/api-keys/{apiKeyId} \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -d @payload.json

```

Updates an existing API key for the specified Agent proxy, with the same behavior as
REST API keys: the supplied `apiKey` value replaces the key material, is hashed before
storage, and is broadcast to the gateways the Agent proxy is deployed to. Only the
key's creator may update it, unless the caller holds `ap:credential:api_key:all:manage`. The
Agent proxy must be associated with at least one gateway; otherwise the update is
refused with 503.

> Payload

```json
{
  "name": "production-key-01",
  "displayName": "Production API Key (Updated)",
  "apiKey": "sk_example_new1234567890abcdef",
  "externalRefId": "ext-ref-12345",
  "expiresAt": "2027-12-31T23:59:59Z",
  "expiresIn": {
    "duration": 30,
    "unit": "days"
  },
  "issuer": "api-platform-devportal"
}
```

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:api:agent:api_key:update`, `ap:api:agent:api_key:manage`, `ap:api:agent:manage`, `ap:api:manage`, `ap:manage`, `ap:credential:api_key:all:manage`

</aside>

<h3 id="update-an-api-key-for-an-agent-proxy-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|agentProxyId|path|string|true|**Agent Proxy ID** consisting of the **handle** (unique slug identifier) of the Agent proxy.|
|apiKeyId|path|string|true|**API Key ID** consisting of the **name** (unique identifier) of the API key.|
|body|body|[UpdateAPIKeyRequest](schemas.md#schemaupdateapikeyrequest)|true|API key update request|

#### Detailed descriptions

**agentProxyId**: **Agent Proxy ID** consisting of the **handle** (unique slug identifier) of the Agent proxy.

**apiKeyId**: **API Key ID** consisting of the **name** (unique identifier) of the API key.

> Example responses
>
> 200 Response

```json
{
  "status": "success",
  "message": "API key updated and broadcasted to gateways successfully",
  "keyId": "production-key-01"
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

> 413 Response

```json
{
  "status": "error",
  "code": "PAYLOAD_TOO_LARGE",
  "message": "The uploaded file exceeds the maximum allowed size."
}
```

> 415 Response

```json
{
  "status": "error",
  "code": "UNSUPPORTED_MEDIA_TYPE",
  "message": "This operation accepts application/json only."
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

> 503 Response

```json
{
  "status": "error",
  "code": "GATEWAY_CONNECTION_UNAVAILABLE",
  "message": "No gateway connections are currently available.",
  "trackingId": "4f1c6f2e-8a4b-4c93-b1de-9f2f6f0c2a11"
}
```

<h3 id="update-an-api-key-for-an-agent-proxy-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|API key updated successfully. Key material is not returned.|[UpdateAPIKeyResponse](schemas.md#schemaupdateapikeyresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad Request. Invalid request or validation error.|[Error](schemas.md#schemaerror)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Forbidden. The authenticated user does not have permission to access this resource.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|413|[Payload Too Large](https://tools.ietf.org/html/rfc7231#section-6.5.11)|Payload Too Large. The uploaded file exceeds the maximum allowed size.|[Error](schemas.md#schemaerror)|
|415|[Unsupported Media Type](https://tools.ietf.org/html/rfc7231#section-6.5.13)|Unsupported Media Type. The request body media type is not supported by this operation.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|
|503|[Service Unavailable](https://tools.ietf.org/html/rfc7231#section-6.6.4)|Service Unavailable. No gateway connections are currently available to service this request.|[Error](schemas.md#schemaerror)|

## Delete an API key for an Agent proxy

<a id="opIdrevokeAgentProxyAPIKey"></a>

`DELETE /agent-proxies/{agentProxyId}/api-keys/{apiKeyId}`

> Code samples

```shell

curl -X DELETE https://localhost:9243/api/v0.9/agent-proxies/{agentProxyId}/api-keys/{apiKeyId} \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Accept: application/json'

```

Deletes the API key from the control plane and records the deletion in the audit log.
A revocation is then propagated to the gateways the
Agent proxy is associated with through the existing gateway event mechanism, so a 204
confirms the control-plane deletion, not that every gateway has already dropped the key.
An Agent proxy associated with no gateway has nothing to notify, and the key is still
deleted. Only the key's creator may delete it, unless the caller holds
`ap:credential:api_key:all:manage`.

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:api:agent:api_key:delete`, `ap:api:agent:api_key:manage`, `ap:api:agent:manage`, `ap:api:manage`, `ap:manage`, `ap:credential:api_key:all:manage`

</aside>

<h3 id="delete-an-api-key-for-an-agent-proxy-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|agentProxyId|path|string|true|**Agent Proxy ID** consisting of the **handle** (unique slug identifier) of the Agent proxy.|
|apiKeyId|path|string|true|**API Key ID** consisting of the **name** (unique identifier) of the API key.|

#### Detailed descriptions

**agentProxyId**: **Agent Proxy ID** consisting of the **handle** (unique slug identifier) of the Agent proxy.

**apiKeyId**: **API Key ID** consisting of the **name** (unique identifier) of the API key.

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

> 500 Response

```json
{
  "status": "error",
  "code": "INTERNAL_ERROR",
  "message": "An unexpected error occurred.",
  "trackingId": "4f1c6f2e-8a4b-4c93-b1de-9f2f6f0c2a11"
}
```

<h3 id="delete-an-api-key-for-an-agent-proxy-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|204|[No Content](https://tools.ietf.org/html/rfc7231#section-6.3.5)|API key deleted successfully (no content)|None|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Forbidden. The authenticated user does not have permission to access this resource.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|
