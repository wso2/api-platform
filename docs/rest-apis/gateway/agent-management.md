<h1 id="gateway-controller-management-api-agent-management">Agent Management</h1>

CRUD operations for A2A Agents

## Create a new Agent

<a id="opIdcreateAgent"></a>

`POST /agents`

> Code samples

```shell

curl -X POST http://localhost:9090/api/management/v1/agents \
  -u {username}:{password} \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -d @payload.json

```

Add a new A2A Agent to the Gateway.

> Payload

```json
{
  "apiVersion": "gateway.api-platform.wso2.com/v1",
  "kind": "Agent",
  "metadata": {
    "name": "weather-agent-v1-0"
  },
  "spec": {
    "displayName": "Weather Agent",
    "version": "v1.0",
    "context": "/weather",
    "vhost": "agents.example.com",
    "upstream": {
      "url": "https://weather.internal"
    },
    "resilience": {
      "timeout": "30s",
      "idleTimeout": "5m"
    },
    "a2a": {
      "protocolVersion": "1.0",
      "operationConfigs": {
        "transports": [
          {
            "protocolBinding": "JSONRPC",
            "pathPrefix": "/rpc"
          },
          {
            "protocolBinding": "HTTP+JSON",
            "pathPrefix": "/rest"
          }
        ],
        "policies": [
          {
            "name": "jwt-auth",
            "version": "v1",
            "params": {
              "issuer": "https://idp.example.com",
              "requiredScopes": [
                "a2a.invoke"
              ]
            }
          }
        ],
        "operations": [
          {
            "name": "SendMessage",
            "policies": [
              {
                "name": "advanced-ratelimit",
                "version": "v1",
                "params": {
                  "quotas": [
                    {
                      "name": "send-message-limit",
                      "limits": [
                        {
                          "limit": 100,
                          "duration": "1m"
                        }
                      ]
                    }
                  ]
                }
              }
            ]
          }
        ]
      },
      "agentCard": {
        "public": {
          "mode": "managed",
          "path": "/.well-known/agent-card.json",
          "policies": [
            {
              "name": "cors",
              "version": "v1"
            }
          ],
          "content": {
            "name": "Weather Agent",
            "description": "Provides weather information",
            "version": "1.0.0",
            "supportedInterfaces": [
              {
                "protocolBinding": "JSONRPC",
                "protocolVersion": "1.0",
                "url": "https://agents.example.com/weather/rpc"
              },
              {
                "protocolBinding": "HTTP+JSON",
                "protocolVersion": "1.0",
                "url": "https://agents.example.com/weather/rest"
              }
            ],
            "capabilities": {
              "streaming": true
            },
            "securitySchemes": {
              "gateway-jwt": {
                "openIdConnectSecurityScheme": {
                  "openIdConnectUrl": "https://idp.example.com/.well-known/openid-configuration"
                }
              }
            },
            "securityRequirements": [
              {
                "schemes": {
                  "gateway-jwt": {
                    "list": [
                      "a2a.invoke"
                    ]
                  }
                }
              }
            ],
            "defaultInputModes": [
              "text/plain"
            ],
            "defaultOutputModes": [
              "text/plain"
            ],
            "skills": [
              {
                "id": "get_weather",
                "name": "Get weather",
                "description": "Gets weather information",
                "tags": [
                  "weather"
                ]
              }
            ]
          }
        }
      }
    }
  }
}
```

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

Required roles: `admin`, `developer`

</aside>

<h3 id="create-a-new-agent-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|[AgentConfigurationRequest](schemas.md#schemaagentconfigurationrequest)|true|none|

> Example responses
>
> 201 Response

```json
{
  "apiVersion": "gateway.api-platform.wso2.com/v1",
  "kind": "Agent",
  "metadata": {
    "name": "weather-agent-v1-0"
  },
  "spec": {
    "displayName": "Weather Agent",
    "version": "v1.0",
    "context": "/weather",
    "vhost": "agents.example.com",
    "upstream": {
      "url": "https://weather.internal"
    },
    "resilience": {
      "timeout": "30s",
      "idleTimeout": "5m"
    },
    "a2a": {
      "protocolVersion": "1.0",
      "operationConfigs": {
        "transports": [
          {
            "protocolBinding": "JSONRPC",
            "pathPrefix": "/rpc"
          },
          {
            "protocolBinding": "HTTP+JSON",
            "pathPrefix": "/rest"
          }
        ],
        "policies": [
          {
            "name": "jwt-auth",
            "version": "v1",
            "params": {
              "issuer": "https://idp.example.com",
              "requiredScopes": [
                "a2a.invoke"
              ]
            }
          }
        ],
        "operations": [
          {
            "name": "SendMessage",
            "policies": [
              {
                "name": "advanced-ratelimit",
                "version": "v1",
                "params": {
                  "quotas": [
                    {}
                  ]
                }
              }
            ]
          }
        ]
      },
      "agentCard": {
        "public": {
          "mode": "managed",
          "path": "/.well-known/agent-card.json",
          "policies": [
            {
              "name": "cors",
              "version": "v1"
            }
          ],
          "content": {
            "name": "Weather Agent",
            "description": "Provides weather information",
            "version": "1.0.0",
            "supportedInterfaces": [
              {
                "protocolBinding": "JSONRPC",
                "protocolVersion": "1.0",
                "url": "https://agents.example.com/weather/rpc"
              },
              {
                "protocolBinding": "HTTP+JSON",
                "protocolVersion": "1.0",
                "url": "https://agents.example.com/weather/rest"
              }
            ],
            "capabilities": {
              "streaming": true
            },
            "securitySchemes": {
              "gateway-jwt": {
                "openIdConnectSecurityScheme": {
                  "openIdConnectUrl": "https://idp.example.com/.well-known/openid-configuration"
                }
              }
            },
            "securityRequirements": [
              {
                "schemes": {
                  "gateway-jwt": {
                    "list": []
                  }
                }
              }
            ],
            "defaultInputModes": [
              "text/plain"
            ],
            "defaultOutputModes": [
              "text/plain"
            ],
            "skills": [
              {
                "id": "get_weather",
                "name": "Get weather",
                "description": "Gets weather information",
                "tags": [
                  "weather"
                ]
              }
            ]
          }
        }
      }
    }
  },
  "status": {
    "id": "reading-list-api-v1.0",
    "state": "deployed",
    "createdAt": "2026-04-24T07:21:13Z",
    "updatedAt": "2026-04-24T07:21:13Z",
    "deployedAt": "2026-04-24T07:21:13Z"
  }
}
```

<h3 id="create-a-new-agent-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|201|[Created](https://tools.ietf.org/html/rfc7231#section-6.3.2)|Agent created successfully|[AgentConfiguration](schemas.md#schemaagentconfiguration)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Invalid configuration (validation failed)|[ErrorResponse](schemas.md#schemaerrorresponse)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Conflict - Agent with same name and version already exists|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

## List all Agents

<a id="opIdlistAgents"></a>

`GET /agents`

> Code samples

```shell

curl -X GET http://localhost:9090/api/management/v1/agents \
  -u {username}:{password} \
  -H 'Accept: application/json'

```

List Agents registered in the Gateway, optionally filtered by name, version, context, or status.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

Required roles: `admin`, `developer`

</aside>

<h3 id="list-all-agents-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|displayName|query|string|false|Filter by agent display name|
|version|query|string|false|Filter by agent version|
|context|query|string|false|Filter by agent context/path|
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
  "count": 5,
  "agents": [
    {
      "apiVersion": "gateway.api-platform.wso2.com/v1",
      "kind": "Agent",
      "metadata": {
        "name": "weather-agent-v1-0"
      },
      "spec": {
        "displayName": "Weather Agent",
        "version": "v1.0",
        "context": "/weather",
        "vhost": "agents.example.com",
        "upstream": {
          "url": "https://weather.internal"
        },
        "resilience": {
          "timeout": "30s",
          "idleTimeout": "5m"
        },
        "a2a": {
          "protocolVersion": "1.0",
          "operationConfigs": {
            "transports": [
              {
                "protocolBinding": "JSONRPC",
                "pathPrefix": "/rpc"
              },
              {
                "protocolBinding": "HTTP+JSON",
                "pathPrefix": "/rest"
              }
            ],
            "policies": [
              {
                "name": "jwt-auth",
                "version": "v1",
                "params": {
                  "issuer": "https://idp.example.com",
                  "requiredScopes": [
                    "a2a.invoke"
                  ]
                }
              }
            ],
            "operations": [
              {
                "name": "SendMessage",
                "policies": [
                  {
                    "name": "advanced-ratelimit",
                    "version": "v1",
                    "params": {}
                  }
                ]
              }
            ]
          },
          "agentCard": {
            "public": {
              "mode": "managed",
              "path": "/.well-known/agent-card.json",
              "policies": [
                {
                  "name": "cors",
                  "version": "v1"
                }
              ],
              "content": {
                "name": "Weather Agent",
                "description": "Provides weather information",
                "version": "1.0.0",
                "supportedInterfaces": [
                  {
                    "protocolBinding": "JSONRPC",
                    "protocolVersion": "1.0",
                    "url": "https://agents.example.com/weather/rpc"
                  },
                  {
                    "protocolBinding": "HTTP+JSON",
                    "protocolVersion": "1.0",
                    "url": "https://agents.example.com/weather/rest"
                  }
                ],
                "capabilities": {
                  "streaming": true
                },
                "securitySchemes": {
                  "gateway-jwt": {
                    "openIdConnectSecurityScheme": {}
                  }
                },
                "securityRequirements": [
                  {
                    "schemes": {}
                  }
                ],
                "defaultInputModes": [
                  "text/plain"
                ],
                "defaultOutputModes": [
                  "text/plain"
                ],
                "skills": [
                  {
                    "id": "get_weather",
                    "name": "Get weather",
                    "description": "Gets weather information",
                    "tags": []
                  }
                ]
              }
            }
          }
        }
      },
      "status": {
        "id": "reading-list-api-v1.0",
        "state": "deployed",
        "createdAt": "2026-04-24T07:21:13Z",
        "updatedAt": "2026-04-24T07:21:13Z",
        "deployedAt": "2026-04-24T07:21:13Z"
      }
    }
  ]
}
```

<h3 id="list-all-agents-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|List of Agents|Inline|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="list-all-agents-responseschema">Response Schema</h3>

Status Code **200**

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|» status|string|false|none|none|
|» count|integer|false|none|none|
|» agents|[allOf]|false|none|none|

*allOf*

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|»» *anonymous*|[AgentConfigurationRequest](schemas.md#schemaagentconfigurationrequest)|false|none|none|
|»»» apiVersion|string|true|none|Agent specification version|
|»»» kind|string|true|none|Agent type|
|»»» metadata|[Metadata](schemas.md#schemametadata)|true|none|none|
|»»»» name|string|true|none|Unique handle for the resource|
|»»»» labels|object|false|none|Labels are key-value pairs for organizing and selecting APIs. Keys must not contain spaces.|
|»»»»» **additionalProperties**|string|false|none|none|
|»»»» annotations|object|false|none|Annotations are arbitrary non-identifying metadata. Use domain-prefixed keys.|
|»»»»» **additionalProperties**|string|false|none|none|
|»»» spec|[AgentConfigData](schemas.md#schemaagentconfigdata)|true|none|none|
|»»»» displayName|string|true|none|Human-readable agent display name|
|»»»» version|string|true|none|Agent version|
|»»»» context|string|false|none|Gateway context path for the agent (must start with /, no trailing slash). Optional: when omitted the agent is served at the root of its virtual host, which is where an A2A client probes for `/.well-known/agent-card.json` during cold discovery. Every A2A route the gateway generates — the transport base paths and the Agent Card path — is relative to this value.|
|»»»» vhost|string|false|none|Virtual host name used for routing. Supports standard domain names, subdomains, or wildcard domains. Must follow RFC-compliant hostname rules. Wildcards are only allowed in the left-most label (e.g., *.example.com).|
|»»»» upstreamDefinitions|[[UpstreamDefinition](schemas.md#schemaupstreamdefinition)]|false|none|List of reusable upstream definitions with optional timeout configurations. Referenced by upstream.ref.|
|»»»»» name|string|true|none|Unique identifier for this upstream definition|
|»»»»» basePath|string|false|none|Base path prefix for all endpoints in this upstream (e.g., /api/v2). All requests to this upstream will have this path prepended. Must start with '/' and must not end with '/'; omit for root.|
|»»»»» timeout|[UpstreamTimeout](schemas.md#schemaupstreamtimeout)|false|none|Timeout configuration for upstream requests|
|»»»»»» connect|string|false|none|Connection timeout duration (e.g., "5s", "500ms")|
|»»»»» upstreams|[object]|true|none|List of backend targets with optional weights for load balancing|
|»»»»»» url|string(uri)|true|none|Backend URL (host and port only, path comes from basePath)|
|»»»»»» weight|integer|false|none|Relative weight for load balancing across multiple upstream targets. Reserved for future multi-target load balancing; not applied yet (only the first target is currently used).|
|»»»» upstream|any|true|none|The backend A2A agent url and auth configuration. The URL is the base the gateway forwards A2A operation traffic to, and — in public passthrough card mode — the origin of the standard /.well-known/agent-card.json document.|

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
|»»»»»»» value|string|false|write-only|Upstream credential. Write-only: accepted on create/update and never returned by the management API on a read, for any role. Supply either a literal value or a secret reference (e.g. a `secret` template expression); either way the field is omitted from management API response bodies. An update that omits it inherits the stored value; set `type: none` to remove auth.|

*continued*

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|»»»» deploymentState|string|false|none|Desired deployment state - 'deployed' (default) or 'undeployed'. When set to 'undeployed', the Agent is removed from router traffic but configuration and policies are preserved for potential redeployment.|
|»»»» resilience|[Resilience](schemas.md#schemaresilience)|false|none|Backend/route timeout configuration. Maps to Envoy RouteAction timeouts. Can be set at the API level (applies to all routes) and/or the operation level (applies to that operation's route). When set at both levels, the operation-level value takes precedence. When unset, the gateway's global route timeout defaults apply.|
|»»»»» timeout|string|false|none|Maximum time for the entire route (request to upstream response). "0s" disables the timeout.|
|»»»»» idleTimeout|string|false|none|Per-route stream idle timeout (overrides the listener stream idle timeout for this route). "0s" disables the timeout.|
|»»»» a2a|[A2AConfig](schemas.md#schemaa2aconfig)|true|none|A2A-specific agent configuration.|
|»»»»» protocolVersion|string|true|none|A2A protocol version exposed by the gateway. This selects the agent's operation set and HTTP+JSON bindings, the Agent Card model its managed card is validated against, and the field-presence rules used to sign that card — an agent exposes exactly one version and the gateway performs no protocol-version conversion. Managed Agent Card interfaces must advertise this version; in passthrough mode the upstream is responsible for advertising it.|
|»»»»» operationConfigs|[A2AOperationConfigs](schemas.md#schemaa2aoperationconfigs)|true|none|Transport exposure and common or operation-specific configuration for A2A operations. These policies and transports do not apply to public Agent Card serving.|
|»»»»»» transports|[[A2ATransport](schemas.md#schemaa2atransport)]|true|none|Ordered A2A protocol bindings and their gateway-facing path prefixes. This is runtime routing configuration, not Agent Card transformation or transport conversion.|
|»»»»»»» protocolBinding|[A2AProtocolBinding](schemas.md#schemaa2aprotocolbinding)|true|none|A2A protocol binding exposed at a transport's path prefix.|
|»»»»»»» pathPrefix|string|false|none|Gateway-facing path prefix relative to spec.context. The root value / means that no additional path segment is inserted. For JSONRPC, this is the endpoint path; for HTTP+JSON, canonical operation paths are appended below it. This field does not select or replace the generic upstream.|
|»»»»»» policies|[[Policy](schemas.md#schemapolicy)]|false|none|Ordered policies applied to every A2A operation, before operation-level policies. These policies do not apply to the public Agent Card discovery route.|
|»»»»»»» name|string|true|none|Name of the policy|
|»»»»»»» version|string|true|none|Version of the policy. Only major-only version is allowed (e.g., v0, v1). Full semantic version (e.g., v1.0.0) is not accepted and will be rejected. The Gateway Controller resolves the major version to the single matching full version installed in the gateway image.|
|»»»»»»» executionCondition|string|false|none|Expression controlling conditional execution of the policy|
|»»»»»»» params|object|false|none|Arbitrary parameters for the policy (free-form key/value structure)|
|»»»»»» operations|[[A2AOperationConfig](schemas.md#schemaa2aoperationconfig)]|false|none|Optional per-operation configuration keyed by canonical A2A operation name. This array is not an allowlist: unlisted standard operations still receive spec.a2a.operationConfigs.policies. Public Agent Card discovery is configured separately.|
|»»»»»»» name|[A2AOperationName](schemas.md#schemaa2aoperationname)|true|none|Canonical A2A operation name. These names match the standard JSON-RPC and gRPC method names, but identify the binding-independent A2A operation. The effective set is closed and is the one defined by the agent's spec.a2a.protocolVersion; the values below are A2A 1.0's eleven operations, that being the only protocol version currently supported. A name outside the selected version's set is rejected at deploy time.|
|»»»»»»» policies|[[Policy](schemas.md#schemapolicy)]|false|none|Ordered policies applied after spec.a2a.operationConfigs.policies when this operation is selected.|
|»»»»»»» resilience|[Resilience](schemas.md#schemaresilience)|false|none|Backend/route timeout configuration. Maps to Envoy RouteAction timeouts. Can be set at the API level (applies to all routes) and/or the operation level (applies to that operation's route). When set at both levels, the operation-level value takes precedence. When unset, the gateway's global route timeout defaults apply.|
|»»»»» agentCard|[A2AAgentCard](schemas.md#schemaa2aagentcard)|true|none|Public Agent Card configuration and optional protected Agent Card configuration for the authenticated A2A GetExtendedAgentCard operation.|
|»»»»»» public|[A2APublicAgentCard](schemas.md#schemaa2apublicagentcard)|true|none|Public Agent Card serving. `mode` selects whether the card is proxied unchanged from the upstream (`passthrough`) or validated, stored, and served by the gateway (`managed`). Mode-specific rules are enforced at deploy time, not by this schema: `managed` requires `content`; `passthrough` accepts neither `content` nor `signing`, because the gateway does not parse, transform, or sign a proxied card.|
|»»»»»»» mode|string|true|none|How the public Agent Card is produced.|
|»»»»»»» path|[A2AAgentCardPath](schemas.md#schemaa2aagentcardpath)|false|none|Exact gateway-facing Agent Card path relative to spec.context. When omitted, the gateway uses /.well-known/agent-card.json. A custom path replaces that default route rather than creating an additional alias. In passthrough mode this does not change the upstream discovery path.|
|»»»»»»» policies|[[Policy](schemas.md#schemapolicy)]|false|none|Ordered policies applied only to public Agent Card serving.|
|»»»»»»» content|[A2AAgentCardDocument](schemas.md#schemaa2aagentcarddocument)|false|none|Complete A2A 1.0 Agent Card represented as a structured JSON object. JSON can be embedded directly because JSON object syntax is valid YAML. The controller additionally validates this object against the complete A2A Agent Card model for spec.a2a.protocolVersion, taken from the vendored A2A protocol definition (specification/a2a.proto). The document is stored and served as supplied — the gateway never rewrites it — so extension fields are preserved.|
|»»»»»»» signing|[A2ACardSigning](schemas.md#schemaa2acardsigning)|false|none|Optional signing configuration for a managed Agent Card. Passthrough cards cannot configure gateway signing. Agent authors only enable or disable signing: the active key, its key identifier, and the JWS algorithm are selected from administrator-owned gateway system configuration at signing time, so rotating the key — including to a key using a different algorithm — requires no edit to any Agent. A card is re-signed when its Agent is next deployed, not when the key rotates; until then it keeps verifying against the retired key, which stays published while any stored card references it.|
|»»»»»»»» enabled|boolean|true|none|Whether the gateway signs the managed card it serves, using the active Agent Card signing key configured by the gateway administrator.|
|»»»»»» protected|[A2AProtectedAgentCard](schemas.md#schemaa2aprotectedagentcard)|false|none|Planned authenticated extended Agent Card support. When present it is served through the canonical GetExtendedAgentCard operation and uses the operation policy chain, not public Agent Card policies. It has no custom path or local policies because it is an A2A operation. Not implemented in this release: an explicitly configured `protected` block is rejected at deploy time. GetExtendedAgentCard is exposed and proxied to the upstream.|
|»»»»»»» mode|string|true|none|How the protected Agent Card is produced.|
|»»»»»»» content|[A2AAgentCardDocument](schemas.md#schemaa2aagentcarddocument)|false|none|Complete A2A 1.0 Agent Card represented as a structured JSON object. JSON can be embedded directly because JSON object syntax is valid YAML. The controller additionally validates this object against the complete A2A Agent Card model for spec.a2a.protocolVersion, taken from the vendored A2A protocol definition (specification/a2a.proto). The document is stored and served as supplied — the gateway never rewrites it — so extension fields are preserved.|
|»»»»»»» signing|[A2ACardSigning](schemas.md#schemaa2acardsigning)|false|none|Optional signing configuration for a managed Agent Card. Passthrough cards cannot configure gateway signing. Agent authors only enable or disable signing: the active key, its key identifier, and the JWS algorithm are selected from administrator-owned gateway system configuration at signing time, so rotating the key — including to a key using a different algorithm — requires no edit to any Agent. A card is re-signed when its Agent is next deployed, not when the key rotates; until then it keeps verifying against the retired key, which stays published while any stored card references it.|

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
|apiVersion|gateway.api-platform.wso2.com/v1|
|kind|Agent|
|hostRewrite|auto|
|hostRewrite|manual|
|type|api-key|
|type|other|
|type|none|
|deploymentState|deployed|
|deploymentState|undeployed|
|protocolVersion|1.0|
|protocolBinding|JSONRPC|
|protocolBinding|HTTP+JSON|
|name|SendMessage|
|name|SendStreamingMessage|
|name|GetTask|
|name|ListTasks|
|name|CancelTask|
|name|SubscribeToTask|
|name|CreateTaskPushNotificationConfig|
|name|GetTaskPushNotificationConfig|
|name|ListTaskPushNotificationConfigs|
|name|DeleteTaskPushNotificationConfig|
|name|GetExtendedAgentCard|
|mode|managed|
|mode|passthrough|
|mode|managed|
|mode|passthrough|
|state|deployed|
|state|undeployed|

## Get Agent by id

<a id="opIdgetAgentById"></a>

`GET /agents/{id}`

> Code samples

```shell

curl -X GET http://localhost:9090/api/management/v1/agents/{id} \
  -u {username}:{password} \
  -H 'Accept: application/json'

```

Get an Agent by its ID.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

Required roles: `admin`, `developer`

</aside>

<h3 id="get-agent-by-id-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|string|true|Unique public identifier of the Agent.|

#### Detailed descriptions

**id**: Unique public identifier of the Agent.

> Example responses
>
> 200 Response

```json
{
  "apiVersion": "gateway.api-platform.wso2.com/v1",
  "kind": "Agent",
  "metadata": {
    "name": "weather-agent-v1-0"
  },
  "spec": {
    "displayName": "Weather Agent",
    "version": "v1.0",
    "context": "/weather",
    "vhost": "agents.example.com",
    "upstream": {
      "url": "https://weather.internal"
    },
    "resilience": {
      "timeout": "30s",
      "idleTimeout": "5m"
    },
    "a2a": {
      "protocolVersion": "1.0",
      "operationConfigs": {
        "transports": [
          {
            "protocolBinding": "JSONRPC",
            "pathPrefix": "/rpc"
          },
          {
            "protocolBinding": "HTTP+JSON",
            "pathPrefix": "/rest"
          }
        ],
        "policies": [
          {
            "name": "jwt-auth",
            "version": "v1",
            "params": {
              "issuer": "https://idp.example.com",
              "requiredScopes": [
                "a2a.invoke"
              ]
            }
          }
        ],
        "operations": [
          {
            "name": "SendMessage",
            "policies": [
              {
                "name": "advanced-ratelimit",
                "version": "v1",
                "params": {
                  "quotas": [
                    {}
                  ]
                }
              }
            ]
          }
        ]
      },
      "agentCard": {
        "public": {
          "mode": "managed",
          "path": "/.well-known/agent-card.json",
          "policies": [
            {
              "name": "cors",
              "version": "v1"
            }
          ],
          "content": {
            "name": "Weather Agent",
            "description": "Provides weather information",
            "version": "1.0.0",
            "supportedInterfaces": [
              {
                "protocolBinding": "JSONRPC",
                "protocolVersion": "1.0",
                "url": "https://agents.example.com/weather/rpc"
              },
              {
                "protocolBinding": "HTTP+JSON",
                "protocolVersion": "1.0",
                "url": "https://agents.example.com/weather/rest"
              }
            ],
            "capabilities": {
              "streaming": true
            },
            "securitySchemes": {
              "gateway-jwt": {
                "openIdConnectSecurityScheme": {
                  "openIdConnectUrl": "https://idp.example.com/.well-known/openid-configuration"
                }
              }
            },
            "securityRequirements": [
              {
                "schemes": {
                  "gateway-jwt": {
                    "list": []
                  }
                }
              }
            ],
            "defaultInputModes": [
              "text/plain"
            ],
            "defaultOutputModes": [
              "text/plain"
            ],
            "skills": [
              {
                "id": "get_weather",
                "name": "Get weather",
                "description": "Gets weather information",
                "tags": [
                  "weather"
                ]
              }
            ]
          }
        }
      }
    }
  },
  "status": {
    "id": "reading-list-api-v1.0",
    "state": "deployed",
    "createdAt": "2026-04-24T07:21:13Z",
    "updatedAt": "2026-04-24T07:21:13Z",
    "deployedAt": "2026-04-24T07:21:13Z"
  }
}
```

<h3 id="get-agent-by-id-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Agent details|[AgentConfiguration](schemas.md#schemaagentconfiguration)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Agent not found|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

## Update an existing Agent

<a id="opIdupdateAgent"></a>

`PUT /agents/{id}`

> Code samples

```shell

curl -X PUT http://localhost:9090/api/management/v1/agents/{id} \
  -u {username}:{password} \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -d @payload.json

```

Update an existing Agent in the Gateway.

> Payload

```json
{
  "apiVersion": "gateway.api-platform.wso2.com/v1",
  "kind": "Agent",
  "metadata": {
    "name": "weather-agent-v1-0"
  },
  "spec": {
    "displayName": "Weather Agent",
    "version": "v1.0",
    "context": "/weather",
    "vhost": "agents.example.com",
    "upstream": {
      "url": "https://weather.internal"
    },
    "resilience": {
      "timeout": "30s",
      "idleTimeout": "5m"
    },
    "a2a": {
      "protocolVersion": "1.0",
      "operationConfigs": {
        "transports": [
          {
            "protocolBinding": "JSONRPC",
            "pathPrefix": "/rpc"
          },
          {
            "protocolBinding": "HTTP+JSON",
            "pathPrefix": "/rest"
          }
        ],
        "policies": [
          {
            "name": "jwt-auth",
            "version": "v1",
            "params": {
              "issuer": "https://idp.example.com",
              "requiredScopes": [
                "a2a.invoke"
              ]
            }
          }
        ],
        "operations": [
          {
            "name": "SendMessage",
            "policies": [
              {
                "name": "advanced-ratelimit",
                "version": "v1",
                "params": {
                  "quotas": [
                    {
                      "name": "send-message-limit",
                      "limits": [
                        {
                          "limit": 100,
                          "duration": "1m"
                        }
                      ]
                    }
                  ]
                }
              }
            ]
          }
        ]
      },
      "agentCard": {
        "public": {
          "mode": "managed",
          "path": "/.well-known/agent-card.json",
          "policies": [
            {
              "name": "cors",
              "version": "v1"
            }
          ],
          "content": {
            "name": "Weather Agent",
            "description": "Provides weather information",
            "version": "1.0.0",
            "supportedInterfaces": [
              {
                "protocolBinding": "JSONRPC",
                "protocolVersion": "1.0",
                "url": "https://agents.example.com/weather/rpc"
              },
              {
                "protocolBinding": "HTTP+JSON",
                "protocolVersion": "1.0",
                "url": "https://agents.example.com/weather/rest"
              }
            ],
            "capabilities": {
              "streaming": true
            },
            "securitySchemes": {
              "gateway-jwt": {
                "openIdConnectSecurityScheme": {
                  "openIdConnectUrl": "https://idp.example.com/.well-known/openid-configuration"
                }
              }
            },
            "securityRequirements": [
              {
                "schemes": {
                  "gateway-jwt": {
                    "list": [
                      "a2a.invoke"
                    ]
                  }
                }
              }
            ],
            "defaultInputModes": [
              "text/plain"
            ],
            "defaultOutputModes": [
              "text/plain"
            ],
            "skills": [
              {
                "id": "get_weather",
                "name": "Get weather",
                "description": "Gets weather information",
                "tags": [
                  "weather"
                ]
              }
            ]
          }
        }
      }
    }
  }
}
```

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

Required roles: `admin`, `developer`

</aside>

<h3 id="update-an-existing-agent-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|string|true|Unique public identifier of the Agent to update.|
|body|body|[AgentConfigurationRequest](schemas.md#schemaagentconfigurationrequest)|true|none|

#### Detailed descriptions

**id**: Unique public identifier of the Agent to update.

> Example responses
>
> 200 Response

```json
{
  "apiVersion": "gateway.api-platform.wso2.com/v1",
  "kind": "Agent",
  "metadata": {
    "name": "weather-agent-v1-0"
  },
  "spec": {
    "displayName": "Weather Agent",
    "version": "v1.0",
    "context": "/weather",
    "vhost": "agents.example.com",
    "upstream": {
      "url": "https://weather.internal"
    },
    "resilience": {
      "timeout": "30s",
      "idleTimeout": "5m"
    },
    "a2a": {
      "protocolVersion": "1.0",
      "operationConfigs": {
        "transports": [
          {
            "protocolBinding": "JSONRPC",
            "pathPrefix": "/rpc"
          },
          {
            "protocolBinding": "HTTP+JSON",
            "pathPrefix": "/rest"
          }
        ],
        "policies": [
          {
            "name": "jwt-auth",
            "version": "v1",
            "params": {
              "issuer": "https://idp.example.com",
              "requiredScopes": [
                "a2a.invoke"
              ]
            }
          }
        ],
        "operations": [
          {
            "name": "SendMessage",
            "policies": [
              {
                "name": "advanced-ratelimit",
                "version": "v1",
                "params": {
                  "quotas": [
                    {}
                  ]
                }
              }
            ]
          }
        ]
      },
      "agentCard": {
        "public": {
          "mode": "managed",
          "path": "/.well-known/agent-card.json",
          "policies": [
            {
              "name": "cors",
              "version": "v1"
            }
          ],
          "content": {
            "name": "Weather Agent",
            "description": "Provides weather information",
            "version": "1.0.0",
            "supportedInterfaces": [
              {
                "protocolBinding": "JSONRPC",
                "protocolVersion": "1.0",
                "url": "https://agents.example.com/weather/rpc"
              },
              {
                "protocolBinding": "HTTP+JSON",
                "protocolVersion": "1.0",
                "url": "https://agents.example.com/weather/rest"
              }
            ],
            "capabilities": {
              "streaming": true
            },
            "securitySchemes": {
              "gateway-jwt": {
                "openIdConnectSecurityScheme": {
                  "openIdConnectUrl": "https://idp.example.com/.well-known/openid-configuration"
                }
              }
            },
            "securityRequirements": [
              {
                "schemes": {
                  "gateway-jwt": {
                    "list": []
                  }
                }
              }
            ],
            "defaultInputModes": [
              "text/plain"
            ],
            "defaultOutputModes": [
              "text/plain"
            ],
            "skills": [
              {
                "id": "get_weather",
                "name": "Get weather",
                "description": "Gets weather information",
                "tags": [
                  "weather"
                ]
              }
            ]
          }
        }
      }
    }
  },
  "status": {
    "id": "reading-list-api-v1.0",
    "state": "deployed",
    "createdAt": "2026-04-24T07:21:13Z",
    "updatedAt": "2026-04-24T07:21:13Z",
    "deployedAt": "2026-04-24T07:21:13Z"
  }
}
```

<h3 id="update-an-existing-agent-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Agent updated successfully|[AgentConfiguration](schemas.md#schemaagentconfiguration)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Invalid configuration (validation failed)|[ErrorResponse](schemas.md#schemaerrorresponse)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Agent not found|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

## Delete an Agent

<a id="opIddeleteAgent"></a>

`DELETE /agents/{id}`

> Code samples

```shell

curl -X DELETE http://localhost:9090/api/management/v1/agents/{id} \
  -u {username}:{password} \
  -H 'Accept: application/json'

```

Delete an Agent from the Gateway.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

Required roles: `admin`, `developer`

</aside>

<h3 id="delete-an-agent-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|string|true|Unique public identifier of the Agent to delete.|

#### Detailed descriptions

**id**: Unique public identifier of the Agent to delete.

> Example responses
>
> 200 Response

```json
{
  "status": "success",
  "message": "Agent deleted successfully",
  "id": "weather-agent-v1.0"
}
```

<h3 id="delete-an-agent-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Agent deleted successfully|Inline|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Agent not found|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="delete-an-agent-responseschema">Response Schema</h3>

Status Code **200**

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|» status|string|false|none|none|
|» message|string|false|none|none|
|» id|string|false|none|none|
