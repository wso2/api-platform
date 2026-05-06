# Schemas

<h2 id="tocS_ResourceStatus">ResourceStatus</h2>

<a id="schemaresourcestatus"></a>
<a id="schema_ResourceStatus"></a>
<a id="tocSresourcestatus"></a>
<a id="tocsresourcestatus"></a>

```json
{
  "id": "reading-list-api-v1.0",
  "state": "deployed",
  "createdAt": "2026-04-24T07:21:13Z",
  "updatedAt": "2026-04-24T07:21:13Z",
  "deployedAt": "2026-04-24T07:21:13Z"
}

```

Server-managed lifecycle information for a resource

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|id|string|false|none|Unique identifier assigned by the server (equal to metadata.name)|
|state|string|false|none|Desired deployment state reported by the server|
|createdAt|string(date-time)|false|none|Timestamp when the resource was first created (UTC)|
|updatedAt|string(date-time)|false|none|Timestamp when the resource was last updated (UTC)|
|deployedAt|string(date-time)|false|none|Timestamp when the resource was last deployed (omitted when undeployed)|

#### Enumerated Values

|Property|Value|
|---|---|
|state|deployed|
|state|undeployed|

<h2 id="tocS_RestAPIRequest">RestAPIRequest</h2>

<a id="schemarestapirequest"></a>
<a id="schema_RestAPIRequest"></a>
<a id="tocSrestapirequest"></a>
<a id="tocsrestapirequest"></a>

```json
{
  "apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
  "kind": "RestApi",
  "metadata": {
    "name": "reading-list-api-v1.0"
  },
  "spec": {
    "displayName": "Reading-List-API",
    "version": "v1.0",
    "context": "/reading-list/$version",
    "upstream": {
      "main": {
        "url": "https://apis.bijira.dev/samples/reading-list-api-service/v1.0"
      }
    },
    "policies": [
      {
        "name": "set-headers",
        "version": "v1",
        "params": {
          "request": {
            "headers": [
              {
                "name": "x-wso2-apip-gateway-version",
                "value": "v1.0.0"
              }
            ]
          },
          "response": {
            "headers": [
              {
                "name": "x-environment",
                "value": "development"
              }
            ]
          }
        }
      }
    ],
    "operations": [
      {
        "method": "GET",
        "path": "/books"
      },
      {
        "method": "POST",
        "path": "/books"
      },
      {
        "method": "GET",
        "path": "/books/{id}"
      },
      {
        "method": "PUT",
        "path": "/books/{id}"
      },
      {
        "method": "DELETE",
        "path": "/books/{id}"
      }
    ]
  }
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|apiVersion|string|true|none|API specification version|
|kind|string|true|none|API type|
|metadata|[Metadata](#schemametadata)|true|none|none|
|spec|[APIConfigData](#schemaapiconfigdata)|true|none|none|

#### Enumerated Values

|Property|Value|
|---|---|
|apiVersion|gateway.api-platform.wso2.com/v1alpha1|
|kind|RestApi|

<h2 id="tocS_RestAPI">RestAPI</h2>

<a id="schemarestapi"></a>
<a id="schema_RestAPI"></a>
<a id="tocSrestapi"></a>
<a id="tocsrestapi"></a>

```json
{
  "apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
  "kind": "RestApi",
  "metadata": {
    "name": "reading-list-api-v1.0"
  },
  "spec": {
    "displayName": "Reading-List-API",
    "version": "v1.0",
    "context": "/reading-list/$version",
    "upstream": {
      "main": {
        "url": "https://apis.bijira.dev/samples/reading-list-api-service/v1.0"
      }
    },
    "policies": [
      {
        "name": "set-headers",
        "version": "v1",
        "params": {
          "request": {
            "headers": [
              {
                "name": "x-wso2-apip-gateway-version",
                "value": "v1.0.0"
              }
            ]
          },
          "response": {
            "headers": [
              {
                "name": "x-environment",
                "value": "development"
              }
            ]
          }
        }
      }
    ],
    "operations": [
      {
        "method": "GET",
        "path": "/books"
      },
      {
        "method": "POST",
        "path": "/books"
      },
      {
        "method": "GET",
        "path": "/books/{id}"
      },
      {
        "method": "PUT",
        "path": "/books/{id}"
      },
      {
        "method": "DELETE",
        "path": "/books/{id}"
      }
    ]
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

### Properties

allOf

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|[RestAPIRequest](#schemarestapirequest)|false|none|none|

and

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|object|false|none|none|
|» status|[ResourceStatus](#schemaresourcestatus)|false|read-only|Server-managed lifecycle fields. Populated on responses.|

<h2 id="tocS_WebSubAPIRequest">WebSubAPIRequest</h2>

<a id="schemawebsubapirequest"></a>
<a id="schema_WebSubAPIRequest"></a>
<a id="tocSwebsubapirequest"></a>
<a id="tocswebsubapirequest"></a>

```json
{
  "apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
  "kind": "WebSubApi",
  "metadata": {
    "name": "github-events-v1.0"
  },
  "spec": {
    "displayName": "GitHub Events",
    "version": "v1.0",
    "context": "/github-events/$version",
    "channels": [
      {
        "name": "issues",
        "method": "SUB"
      },
      {
        "name": "pull_requests",
        "method": "SUB"
      }
    ]
  }
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|apiVersion|string|true|none|API specification version|
|kind|string|true|none|API type|
|metadata|[Metadata](#schemametadata)|true|none|none|
|spec|[WebhookAPIData](#schemawebhookapidata)|true|none|none|

#### Enumerated Values

|Property|Value|
|---|---|
|apiVersion|gateway.api-platform.wso2.com/v1alpha1|
|kind|WebSubApi|

<h2 id="tocS_WebSubAPI">WebSubAPI</h2>

<a id="schemawebsubapi"></a>
<a id="schema_WebSubAPI"></a>
<a id="tocSwebsubapi"></a>
<a id="tocswebsubapi"></a>

```json
{
  "apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
  "kind": "WebSubApi",
  "metadata": {
    "name": "github-events-v1.0"
  },
  "spec": {
    "displayName": "GitHub Events",
    "version": "v1.0",
    "context": "/github-events/$version",
    "channels": [
      {
        "name": "issues",
        "method": "SUB"
      },
      {
        "name": "pull_requests",
        "method": "SUB"
      }
    ]
  },
  "status": {
    "id": "github-events-v1.0",
    "state": "deployed",
    "createdAt": "2026-04-24T07:21:13Z",
    "updatedAt": "2026-04-24T07:21:13Z",
    "deployedAt": "2026-04-24T07:21:13Z"
  }
}

```

### Properties

allOf

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|[WebSubAPIRequest](#schemawebsubapirequest)|false|none|none|

and

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|object|false|none|none|
|» status|[ResourceStatus](#schemaresourcestatus)|false|read-only|Server-managed lifecycle fields. Populated on responses.|

<h2 id="tocS_Metadata">Metadata</h2>

<a id="schemametadata"></a>
<a id="schema_Metadata"></a>
<a id="tocSmetadata"></a>
<a id="tocsmetadata"></a>

```json
{
  "name": "reading-list-api-v1.0",
  "labels": {
    "environment": "production",
    "team": "backend",
    "version": "v1"
  },
  "annotations": {
    "gateway.api-platform.wso2.com/project-id": "019d953f-d386-7a64-aa92-1869a28292e0"
  }
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|name|string|true|none|Unique handle for the resource|
|labels|object|false|none|Labels are key-value pairs for organizing and selecting APIs. Keys must not contain spaces.|
|» **additionalProperties**|string|false|none|none|
|annotations|object|false|none|Annotations are arbitrary non-identifying metadata. Use domain-prefixed keys.|
|» **additionalProperties**|string|false|none|none|

<h2 id="tocS_APIConfigData">APIConfigData</h2>

<a id="schemaapiconfigdata"></a>
<a id="schema_APIConfigData"></a>
<a id="tocSapiconfigdata"></a>
<a id="tocsapiconfigdata"></a>

```json
{
  "displayName": "Reading List API",
  "version": "v1.0",
  "context": "/reading-list/$version",
  "upstreamDefinitions": [
    {
      "name": "my-upstream-1",
      "basePath": "/api/v2",
      "timeout": {
        "connect": "5s"
      },
      "upstreams": [
        {
          "url": "http://prod-backend-1:5000",
          "weight": 80
        }
      ]
    }
  ],
  "upstream": {
    "main": {
      "url": "http://prod-backend:5000/api/v2",
      "ref": "string",
      "hostRewrite": "auto"
    },
    "sandbox": {
      "url": "http://prod-backend:5000/api/v2",
      "ref": "string",
      "hostRewrite": "auto"
    }
  },
  "vhosts": {
    "main": "api.example.com",
    "sandbox": "sandbox-api.example.com"
  },
  "subscriptionPlans": [
    "Gold",
    "Silver"
  ],
  "policies": [
    {
      "name": "cors",
      "version": "v1",
      "executionCondition": "request.metadata[authenticated] != true",
      "params": {}
    }
  ],
  "operations": [
    {
      "method": "GET",
      "path": "/books/{id}",
      "policies": [
        {
          "name": "cors",
          "version": "v1",
          "executionCondition": "request.metadata[authenticated] != true",
          "params": {}
        }
      ]
    }
  ],
  "deploymentState": "deployed"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|displayName|string|true|none|Human-readable API name (must be URL-friendly - only letters, numbers, spaces, hyphens, underscores, and dots allowed)|
|version|string|true|none|Semantic version of the API|
|context|string|true|none|Base path for all API routes (must start with /, no trailing slash). Use $version to embed the version in the path (e.g., /reading-list/$version resolves to /reading-list/v1.0).|
|upstreamDefinitions|[[UpstreamDefinition](#schemaupstreamdefinition)]|false|none|List of reusable upstream definitions with optional timeout configurations|
|upstream|object|true|none|API-level upstream configuration|
|» main|[Upstream](#schemaupstream)|true|none|Upstream backend configuration (single target or reference)|
|» sandbox|[Upstream](#schemaupstream)|false|none|Upstream backend configuration (single target or reference)|
|vhosts|object|false|none|Custom virtual hosts/domains for the API|
|» main|string|true|none|Custom virtual host/domain for production traffic|
|» sandbox|string|false|none|Custom virtual host/domain for sandbox traffic|
|subscriptionPlans|[string]|false|none|List of subscription plan names available for this API|
|policies|[[Policy](#schemapolicy)]|false|none|List of API-level policies applied to all operations unless overridden|
|operations|[[Operation](#schemaoperation)]|true|none|List of HTTP operations/routes|
|deploymentState|string|false|none|Desired deployment state - 'deployed' (default) or 'undeployed'. When set to 'undeployed', the API is removed from router traffic but configuration, API keys, and policies are preserved for potential redeployment.|

#### Enumerated Values

|Property|Value|
|---|---|
|deploymentState|deployed|
|deploymentState|undeployed|

<h2 id="tocS_UpstreamDefinition">UpstreamDefinition</h2>

<a id="schemaupstreamdefinition"></a>
<a id="schema_UpstreamDefinition"></a>
<a id="tocSupstreamdefinition"></a>
<a id="tocsupstreamdefinition"></a>

```json
{
  "name": "my-upstream-1",
  "basePath": "/api/v2",
  "timeout": {
    "connect": "5s"
  },
  "upstreams": [
    {
      "url": "http://prod-backend-1:5000",
      "weight": 80
    }
  ]
}

```

Reusable upstream configuration with optional timeout and load balancing settings

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|name|string|true|none|Unique identifier for this upstream definition|
|basePath|string|false|none|Base path prefix for all endpoints in this upstream (e.g., /api/v2). All requests to this upstream will have this path prepended.|
|timeout|[UpstreamTimeout](#schemaupstreamtimeout)|false|none|Timeout configuration for upstream requests|
|upstreams|[object]|true|none|List of backend targets with optional weights for load balancing|
|» url|string(uri)|true|none|Backend URL (host and port only, path comes from basePath)|
|» weight|integer|false|none|Weight for load balancing (optional, default 100)|

<h2 id="tocS_UpstreamTimeout">UpstreamTimeout</h2>

<a id="schemaupstreamtimeout"></a>
<a id="schema_UpstreamTimeout"></a>
<a id="tocSupstreamtimeout"></a>
<a id="tocsupstreamtimeout"></a>

```json
{
  "connect": "5s"
}

```

Timeout configuration for upstream requests

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|connect|string|false|none|Connection timeout duration (e.g., "5s", "500ms")|

<h2 id="tocS_Upstream">Upstream</h2>

<a id="schemaupstream"></a>
<a id="schema_Upstream"></a>
<a id="tocSupstream"></a>
<a id="tocsupstream"></a>

```json
{
  "url": "http://prod-backend:5000/api/v2",
  "ref": "string",
  "hostRewrite": "auto"
}

```

Upstream backend configuration (single target or reference)

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|url|string(uri)|false|none|Direct backend URL to route traffic to|
|ref|string|false|none|Reference to a predefined upstreamDefinition|
|hostRewrite|string|false|none|Controls how the Host header is handled when routing to the upstream. `auto` delegates host rewriting to Envoy, which rewrites the Host header using the upstream cluster host. `manual` disables automatic rewriting and expects explicit configuration.|

oneOf

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|object|false|none|none|

xor

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|object|false|none|none|

#### Enumerated Values

|Property|Value|
|---|---|
|hostRewrite|auto|
|hostRewrite|manual|

<h2 id="tocS_Operation">Operation</h2>

<a id="schemaoperation"></a>
<a id="schema_Operation"></a>
<a id="tocSoperation"></a>
<a id="tocsoperation"></a>

```json
{
  "method": "GET",
  "path": "/books/{id}",
  "policies": [
    {
      "name": "cors",
      "version": "v1",
      "executionCondition": "request.metadata[authenticated] != true",
      "params": {}
    }
  ]
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|method|string|true|none|HTTP method|
|path|string|true|none|Route path with optional {param} placeholders|
|policies|[[Policy](#schemapolicy)]|false|none|List of policies applied only to this operation (overrides or adds to API-level policies)|

#### Enumerated Values

|Property|Value|
|---|---|
|method|GET|
|method|POST|
|method|PUT|
|method|DELETE|
|method|PATCH|
|method|HEAD|
|method|OPTIONS|

<h2 id="tocS_Policy">Policy</h2>

<a id="schemapolicy"></a>
<a id="schema_Policy"></a>
<a id="tocSpolicy"></a>
<a id="tocspolicy"></a>

```json
{
  "name": "cors",
  "version": "v1",
  "executionCondition": "request.metadata[authenticated] != true",
  "params": {}
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|name|string|true|none|Name of the policy|
|version|string|true|none|Version of the policy. Only major-only version is allowed (e.g., v0, v1). Full semantic version (e.g., v1.0.0) is not accepted and will be rejected. The Gateway Controller resolves the major version to the single matching full version installed in the gateway image.|
|executionCondition|string|false|none|Expression controlling conditional execution of the policy|
|params|object|false|none|Arbitrary parameters for the policy (free-form key/value structure)|

<h2 id="tocS_WebhookAPIData">WebhookAPIData</h2>

<a id="schemawebhookapidata"></a>
<a id="schema_WebhookAPIData"></a>
<a id="tocSwebhookapidata"></a>
<a id="tocswebhookapidata"></a>

```json
{
  "displayName": "reading-list-api",
  "version": "v1.0",
  "context": "/weather",
  "vhosts": {
    "main": "api.example.com",
    "sandbox": "sandbox-api.example.com"
  },
  "channels": [
    {
      "name": "issues",
      "method": "SUB",
      "policies": [
        {
          "name": "cors",
          "version": "v1",
          "executionCondition": "request.metadata[authenticated] != true",
          "params": {}
        }
      ]
    }
  ],
  "policies": [
    {
      "name": "cors",
      "version": "v1",
      "executionCondition": "request.metadata[authenticated] != true",
      "params": {}
    }
  ],
  "deploymentState": "deployed"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|displayName|string|true|none|Human-readable API name (must be URL-friendly - only letters, numbers, spaces, hyphens, underscores, and dots allowed)|
|version|string|true|none|Semantic version of the API|
|context|string|true|none|Base path for all API routes (must start with /, no trailing slash)|
|vhosts|object|false|none|Custom virtual hosts/domains for the API|
|» main|string|true|none|Custom virtual host/domain for production traffic|
|» sandbox|string|false|none|Custom virtual host/domain for sandbox traffic|
|channels|[[Channel](#schemachannel)]|true|none|List of channels - Async operations(SUB) for WebSub APIs|
|policies|[[Policy](#schemapolicy)]|false|none|List of API-level policies applied to all operations unless overridden|
|deploymentState|string|false|none|Desired deployment state - 'deployed' (default) or 'undeployed'. When set to 'undeployed', the API is removed from router traffic but configuration, API keys, and policies are preserved for potential redeployment.|

#### Enumerated Values

|Property|Value|
|---|---|
|deploymentState|deployed|
|deploymentState|undeployed|

<h2 id="tocS_Channel">Channel</h2>

<a id="schemachannel"></a>
<a id="schema_Channel"></a>
<a id="tocSchannel"></a>
<a id="tocschannel"></a>

```json
{
  "name": "issues",
  "method": "SUB",
  "policies": [
    {
      "name": "cors",
      "version": "v1",
      "executionCondition": "request.metadata[authenticated] != true",
      "params": {}
    }
  ]
}

```

Channel (topic/event stream) definition for async APIs.

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|name|string|true|none|Channel name or topic identifier relative to API context.|
|method|string|true|none|Operation method type.|
|policies|[[Policy](#schemapolicy)]|false|none|List of policies applied only to this channel (overrides or adds to API-level policies)|

#### Enumerated Values

|Property|Value|
|---|---|
|method|SUB|

<h2 id="tocS_APIKeyCreationRequest">APIKeyCreationRequest</h2>

<a id="schemaapikeycreationrequest"></a>
<a id="schema_APIKeyCreationRequest"></a>
<a id="tocSapikeycreationrequest"></a>
<a id="tocsapikeycreationrequest"></a>

```json
{
  "name": "my-production-key"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|name|string|false|none|Identifier of the API key. If not provided, a default identifier will be generated|
|apiKey|string|false|none|Optional plain-text API key value for external key injection.<br>If provided, this key will be used instead of generating a new one.<br>The key will be hashed before storage. The key can be in any format<br>(minimum 36 characters). Use this for injecting externally generated<br>API keys.|
|maskedApiKey|string|false|none|Masked version of the API key for display purposes.<br>Provided by the platform API when injecting pre-hashed keys.|
|expiresIn|object|false|none|Expiration duration for the API key|
|» unit|string|true|none|Time unit for expiration|
|» duration|integer|true|none|Duration value for expiration|
|expiresAt|string(date-time)|false|none|Expiration timestamp. If both expiresIn and expiresAt are provided, expiresAt takes precedence.|
|externalRefId|string|false|none|External reference ID for the API key.<br>This field is optional and used for tracing purposes only.<br>The gateway generates its own internal ID for tracking.|
|issuer|string|false|none|Identifies the portal that created this key. If provided, only api keys generated from<br>the same portal will be accepted. If not provided, there is no portal restriction.|

#### Enumerated Values

|Property|Value|
|---|---|
|unit|seconds|
|unit|minutes|
|unit|hours|
|unit|days|
|unit|weeks|
|unit|months|

<h2 id="tocS_APIKeyCreationResponse">APIKeyCreationResponse</h2>

<a id="schemaapikeycreationresponse"></a>
<a id="schema_APIKeyCreationResponse"></a>
<a id="tocSapikeycreationresponse"></a>
<a id="tocsapikeycreationresponse"></a>

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

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|status|string|true|none|none|
|message|string|true|none|none|
|remainingApiKeyQuota|integer|false|none|Remaining API key quota for the user|
|apiKey|[APIKey](#schemaapikey)|false|none|Details of an API key|

<h2 id="tocS_APIKey">APIKey</h2>

<a id="schemaapikey"></a>
<a id="schema_APIKey"></a>
<a id="tocSapikey"></a>
<a id="tocsapikey"></a>

```json
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

```

Details of an API key

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|name|string|true|none|URL-safe identifier for the API key (auto-generated from displayName, immutable, used as path parameter)|
|displayName|string|false|none|Human-readable name for the API key (user-provided, mutable)|
|apiKey|string|false|none|Generated API key with apip_ prefix|
|apiId|string|true|none|Unique public identifier of the API that the key is associated with|
|status|string|true|none|Status of the API key|
|createdAt|string(date-time)|true|none|Timestamp when the API key was generated|
|createdBy|string|true|none|Identifier of the user who generated the API key|
|expiresAt|string(date-time)¦null|true|none|Expiration timestamp (null if no expiration)|
|source|string|true|none|Source of the API key (local or external)|
|externalRefId|string|false|none|External reference ID for the API key|

#### Enumerated Values

|Property|Value|
|---|---|
|status|active|
|status|revoked|
|status|expired|
|source|local|
|source|external|

<h2 id="tocS_APIKeyRegenerationRequest">APIKeyRegenerationRequest</h2>

<a id="schemaapikeyregenerationrequest"></a>
<a id="schema_APIKeyRegenerationRequest"></a>
<a id="tocSapikeyregenerationrequest"></a>
<a id="tocsapikeyregenerationrequest"></a>

```json
{}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|expiresIn|object|false|none|Expiration duration for the API key|
|» unit|string|true|none|Time unit for expiration|
|» duration|integer|true|none|Duration value for expiration|
|expiresAt|string(date-time)|false|none|Expiration timestamp|

#### Enumerated Values

|Property|Value|
|---|---|
|unit|seconds|
|unit|minutes|
|unit|hours|
|unit|days|
|unit|weeks|
|unit|months|

<h2 id="tocS_APIKeyUpdateRequest">APIKeyUpdateRequest</h2>

<a id="schemaapikeyupdaterequest"></a>
<a id="schema_APIKeyUpdateRequest"></a>
<a id="tocSapikeyupdaterequest"></a>
<a id="tocsapikeyupdaterequest"></a>

```json
{
  "name": "my-production-key"
}

```

### Properties

*None*

<h2 id="tocS_APIKeyRevocationResponse">APIKeyRevocationResponse</h2>

<a id="schemaapikeyrevocationresponse"></a>
<a id="schema_APIKeyRevocationResponse"></a>
<a id="tocSapikeyrevocationresponse"></a>
<a id="tocsapikeyrevocationresponse"></a>

```json
{
  "status": "success",
  "message": "API key revoked successfully"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|status|string|true|none|none|
|message|string|true|none|none|

<h2 id="tocS_SubscriptionPlanCreateRequest">SubscriptionPlanCreateRequest</h2>

<a id="schemasubscriptionplancreaterequest"></a>
<a id="schema_SubscriptionPlanCreateRequest"></a>
<a id="tocSsubscriptionplancreaterequest"></a>
<a id="tocssubscriptionplancreaterequest"></a>

```json
{
  "planName": "Gold",
  "billingPlan": "COMMERCIAL",
  "stopOnQuotaReach": true,
  "throttleLimitCount": 1000,
  "throttleLimitUnit": "Hour",
  "expiryTime": "2026-12-31T23:59:59Z",
  "status": "ACTIVE"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|planName|string|true|none|none|
|billingPlan|string|false|none|none|
|stopOnQuotaReach|boolean|false|none|none|
|throttleLimitCount|integer|false|none|none|
|throttleLimitUnit|string|false|none|none|
|expiryTime|string(date-time)|false|none|none|
|status|string|false|none|none|

#### Enumerated Values

|Property|Value|
|---|---|
|throttleLimitUnit|Min|
|throttleLimitUnit|Hour|
|throttleLimitUnit|Day|
|throttleLimitUnit|Month|
|status|ACTIVE|
|status|INACTIVE|

<h2 id="tocS_SubscriptionPlanUpdateRequest">SubscriptionPlanUpdateRequest</h2>

<a id="schemasubscriptionplanupdaterequest"></a>
<a id="schema_SubscriptionPlanUpdateRequest"></a>
<a id="tocSsubscriptionplanupdaterequest"></a>
<a id="tocssubscriptionplanupdaterequest"></a>

```json
{
  "planName": "string",
  "billingPlan": "string",
  "stopOnQuotaReach": true,
  "throttleLimitCount": 0,
  "throttleLimitUnit": "Min",
  "expiryTime": "2019-08-24T14:15:22Z",
  "status": "ACTIVE"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|planName|string|false|none|none|
|billingPlan|string|false|none|none|
|stopOnQuotaReach|boolean|false|none|none|
|throttleLimitCount|integer|false|none|none|
|throttleLimitUnit|string|false|none|none|
|expiryTime|string(date-time)|false|none|none|
|status|string|false|none|none|

#### Enumerated Values

|Property|Value|
|---|---|
|throttleLimitUnit|Min|
|throttleLimitUnit|Hour|
|throttleLimitUnit|Day|
|throttleLimitUnit|Month|
|status|ACTIVE|
|status|INACTIVE|

<h2 id="tocS_SubscriptionPlanResponse">SubscriptionPlanResponse</h2>

<a id="schemasubscriptionplanresponse"></a>
<a id="schema_SubscriptionPlanResponse"></a>
<a id="tocSsubscriptionplanresponse"></a>
<a id="tocssubscriptionplanresponse"></a>

```json
{
  "id": "string",
  "planName": "string",
  "billingPlan": "string",
  "stopOnQuotaReach": true,
  "throttleLimitCount": 0,
  "throttleLimitUnit": "string",
  "expiryTime": "2019-08-24T14:15:22Z",
  "gatewayId": "string",
  "status": "ACTIVE",
  "createdAt": "2019-08-24T14:15:22Z",
  "updatedAt": "2019-08-24T14:15:22Z"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|id|string|false|none|none|
|planName|string|false|none|none|
|billingPlan|string|false|none|none|
|stopOnQuotaReach|boolean|false|none|none|
|throttleLimitCount|integer|false|none|none|
|throttleLimitUnit|string|false|none|none|
|expiryTime|string(date-time)|false|none|none|
|gatewayId|string|false|none|none|
|status|string|false|none|none|
|createdAt|string(date-time)|false|none|none|
|updatedAt|string(date-time)|false|none|none|

#### Enumerated Values

|Property|Value|
|---|---|
|status|ACTIVE|
|status|INACTIVE|

<h2 id="tocS_SubscriptionPlanListResponse">SubscriptionPlanListResponse</h2>

<a id="schemasubscriptionplanlistresponse"></a>
<a id="schema_SubscriptionPlanListResponse"></a>
<a id="tocSsubscriptionplanlistresponse"></a>
<a id="tocssubscriptionplanlistresponse"></a>

```json
{
  "subscriptionPlans": [
    {
      "id": "string",
      "planName": "string",
      "billingPlan": "string",
      "stopOnQuotaReach": true,
      "throttleLimitCount": 0,
      "throttleLimitUnit": "string",
      "expiryTime": "2019-08-24T14:15:22Z",
      "gatewayId": "string",
      "status": "ACTIVE",
      "createdAt": "2019-08-24T14:15:22Z",
      "updatedAt": "2019-08-24T14:15:22Z"
    }
  ],
  "count": 0
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|subscriptionPlans|[[SubscriptionPlanResponse](#schemasubscriptionplanresponse)]|false|none|none|
|count|integer|false|none|none|

<h2 id="tocS_SubscriptionCreateRequest">SubscriptionCreateRequest</h2>

<a id="schemasubscriptioncreaterequest"></a>
<a id="schema_SubscriptionCreateRequest"></a>
<a id="tocSsubscriptioncreaterequest"></a>
<a id="tocssubscriptioncreaterequest"></a>

```json
{
  "apiId": "c9f2b6ae-1234-5678-9abc-def012345678",
  "subscriptionToken": "sub-token-abc123xyz",
  "applicationId": "string",
  "subscriptionPlanId": "string",
  "billingCustomerId": "string",
  "billingSubscriptionId": "string",
  "status": "ACTIVE"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|apiId|string|true|none|API identifier (deployment ID or handle)|
|subscriptionToken|string|true|none|Opaque subscription token for API invocation (required; stored as hash only)|
|applicationId|string|false|none|Application identifier (from DevPortal/STS). Optional for token-based subscriptions.|
|subscriptionPlanId|string|false|none|Subscription plan UUID for rate limit and billing configuration.|
|billingCustomerId|string|false|none|Billing customer identifier (optional, for analytics tracking).|
|billingSubscriptionId|string|false|none|Billing subscription identifier (optional, for analytics tracking).|
|status|string|false|none|none|

#### Enumerated Values

|Property|Value|
|---|---|
|status|ACTIVE|
|status|INACTIVE|
|status|REVOKED|

<h2 id="tocS_SubscriptionUpdateRequest">SubscriptionUpdateRequest</h2>

<a id="schemasubscriptionupdaterequest"></a>
<a id="schema_SubscriptionUpdateRequest"></a>
<a id="tocSsubscriptionupdaterequest"></a>
<a id="tocssubscriptionupdaterequest"></a>

```json
{
  "status": "ACTIVE"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|status|string|false|none|none|

#### Enumerated Values

|Property|Value|
|---|---|
|status|ACTIVE|
|status|INACTIVE|
|status|REVOKED|

<h2 id="tocS_SubscriptionResponse">SubscriptionResponse</h2>

<a id="schemasubscriptionresponse"></a>
<a id="schema_SubscriptionResponse"></a>
<a id="tocSsubscriptionresponse"></a>
<a id="tocssubscriptionresponse"></a>

```json
{
  "id": "string",
  "apiId": "string",
  "applicationId": "string",
  "subscriptionToken": "string",
  "subscriptionPlanId": "string",
  "billingCustomerId": "string",
  "billingSubscriptionId": "string",
  "gatewayId": "string",
  "status": "ACTIVE",
  "createdAt": "2019-08-24T14:15:22Z",
  "updatedAt": "2019-08-24T14:15:22Z"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|id|string|false|none|none|
|apiId|string|false|none|none|
|applicationId|string|false|none|none|
|subscriptionToken|string|false|none|Opaque subscription token (returned only on create; use Platform-API to retrieve for existing subscriptions)|
|subscriptionPlanId|string|false|none|Subscription plan UUID|
|billingCustomerId|string|false|none|Billing customer identifier|
|billingSubscriptionId|string|false|none|Billing subscription identifier|
|gatewayId|string|false|none|none|
|status|string|false|none|none|
|createdAt|string(date-time)|false|none|none|
|updatedAt|string(date-time)|false|none|none|

#### Enumerated Values

|Property|Value|
|---|---|
|status|ACTIVE|
|status|INACTIVE|
|status|REVOKED|

<h2 id="tocS_SubscriptionListResponse">SubscriptionListResponse</h2>

<a id="schemasubscriptionlistresponse"></a>
<a id="schema_SubscriptionListResponse"></a>
<a id="tocSsubscriptionlistresponse"></a>
<a id="tocssubscriptionlistresponse"></a>

```json
{
  "subscriptions": [
    {
      "id": "string",
      "apiId": "string",
      "applicationId": "string",
      "subscriptionToken": "string",
      "subscriptionPlanId": "string",
      "billingCustomerId": "string",
      "billingSubscriptionId": "string",
      "gatewayId": "string",
      "status": "ACTIVE",
      "createdAt": "2019-08-24T14:15:22Z",
      "updatedAt": "2019-08-24T14:15:22Z"
    }
  ],
  "count": 0
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|subscriptions|[[SubscriptionResponse](#schemasubscriptionresponse)]|false|none|none|
|count|integer|false|none|none|

<h2 id="tocS_MCPProxyConfigurationRequest">MCPProxyConfigurationRequest</h2>

<a id="schemamcpproxyconfigurationrequest"></a>
<a id="schema_MCPProxyConfigurationRequest"></a>
<a id="tocSmcpproxyconfigurationrequest"></a>
<a id="tocsmcpproxyconfigurationrequest"></a>

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

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|apiVersion|string|true|none|MCP Proxy specification version|
|kind|string|true|none|MCP Proxy type|
|metadata|[Metadata](#schemametadata)|true|none|none|
|spec|[MCPProxyConfigData](#schemamcpproxyconfigdata)|true|none|none|

#### Enumerated Values

|Property|Value|
|---|---|
|apiVersion|gateway.api-platform.wso2.com/v1alpha1|
|kind|Mcp|

<h2 id="tocS_MCPProxyConfiguration">MCPProxyConfiguration</h2>

<a id="schemamcpproxyconfiguration"></a>
<a id="schema_MCPProxyConfiguration"></a>
<a id="tocSmcpproxyconfiguration"></a>
<a id="tocsmcpproxyconfiguration"></a>

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

### Properties

allOf

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|[MCPProxyConfigurationRequest](#schemamcpproxyconfigurationrequest)|false|none|none|

and

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|object|false|none|none|
|» status|[ResourceStatus](#schemaresourcestatus)|false|read-only|Server-managed lifecycle fields. Populated on responses.|

<h2 id="tocS_MCPProxyConfigData">MCPProxyConfigData</h2>

<a id="schemamcpproxyconfigdata"></a>
<a id="schema_MCPProxyConfigData"></a>
<a id="tocSmcpproxyconfigdata"></a>
<a id="tocsmcpproxyconfigdata"></a>

```json
{
  "displayName": "Everything",
  "version": "v1.0",
  "context": "/everything",
  "specVersion": "2025-06-18",
  "vhost": "mcp1.example.com",
  "upstream": {
    "url": "http://prod-backend:5000/api/v2",
    "ref": "string",
    "hostRewrite": "auto",
    "auth": {
      "type": "api-key",
      "header": "string",
      "value": "string"
    }
  },
  "policies": [
    {
      "name": "cors",
      "version": "v1",
      "executionCondition": "request.metadata[authenticated] != true",
      "params": {}
    }
  ],
  "tools": [
    {
      "name": "string",
      "title": "string",
      "description": "string",
      "inputSchema": "string",
      "outputSchema": "string"
    }
  ],
  "resources": [
    {
      "uri": "string",
      "name": "string",
      "title": "string",
      "description": "string",
      "mimeType": "string",
      "size": 0
    }
  ],
  "prompts": [
    {
      "name": "string",
      "title": "string",
      "description": "string",
      "arguments": [
        {
          "name": "string",
          "description": "string",
          "required": true,
          "title": "string"
        }
      ]
    }
  ],
  "deploymentState": "deployed"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|displayName|string|true|none|Human-readable MCP Proxy display name|
|version|string|true|none|MCP Proxy version|
|context|string|false|none|MCP Proxy context path|
|specVersion|string|false|none|MCP specification version|
|vhost|string|false|none|Virtual host name used for routing. Supports standard domain names, subdomains, or wildcard domains. Must follow RFC-compliant hostname rules. Wildcards are only allowed in the left-most label (e.g., *.example.com).|
|upstream|any|true|none|The backend MCP server url and auth configurations|

allOf

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|» *anonymous*|[Upstream](#schemaupstream)|false|none|Upstream backend configuration (single target or reference)|

and

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|» *anonymous*|[UpstreamAuth](#schemaupstreamauth)|false|none|none|

continued

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|policies|[[Policy](#schemapolicy)]|false|none|List of MCP Proxy level policies applied|
|tools|[[MCPTool](#schemamcptool)]|false|none|none|
|resources|[[MCPResource](#schemamcpresource)]|false|none|none|
|prompts|[[MCPPrompt](#schemamcpprompt)]|false|none|none|
|deploymentState|string|false|none|Desired deployment state - 'deployed' (default) or 'undeployed'. When set to 'undeployed', the MCP Proxy is removed from router traffic but configuration and policies are preserved for potential redeployment.|

#### Enumerated Values

|Property|Value|
|---|---|
|deploymentState|deployed|
|deploymentState|undeployed|

<h2 id="tocS_MCPTool">MCPTool</h2>

<a id="schemamcptool"></a>
<a id="schema_MCPTool"></a>
<a id="tocSmcptool"></a>
<a id="tocsmcptool"></a>

```json
{
  "name": "string",
  "title": "string",
  "description": "string",
  "inputSchema": "string",
  "outputSchema": "string"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|name|string|true|none|Unique identifier for the tool|
|title|string|false|none|Optional human-readable name of the tool for display purposes.|
|description|string|true|none|Human-readable description of functionality|
|inputSchema|string|true|none|JSON Schema defining expected parameters|
|outputSchema|string|false|none|Optional JSON Schema defining expected output structure|

<h2 id="tocS_MCPResource">MCPResource</h2>

<a id="schemamcpresource"></a>
<a id="schema_MCPResource"></a>
<a id="tocSmcpresource"></a>
<a id="tocsmcpresource"></a>

```json
{
  "uri": "string",
  "name": "string",
  "title": "string",
  "description": "string",
  "mimeType": "string",
  "size": 0
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|uri|string|true|none|Unique identifier for the resource|
|name|string|true|none|The name of the resource|
|title|string|false|none|Optional human-readable name of the resource for display purposes|
|description|string|false|none|Optional description|
|mimeType|string|false|none|Optional MIME type|
|size|integer|false|none|Optional size in bytes|

<h2 id="tocS_MCPPrompt">MCPPrompt</h2>

<a id="schemamcpprompt"></a>
<a id="schema_MCPPrompt"></a>
<a id="tocSmcpprompt"></a>
<a id="tocsmcpprompt"></a>

```json
{
  "name": "string",
  "title": "string",
  "description": "string",
  "arguments": [
    {
      "name": "string",
      "description": "string",
      "required": true,
      "title": "string"
    }
  ]
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|name|string|true|none|Unique identifier for the prompt|
|title|string|false|none|Optional human-readable name of the prompt for display purposes|
|description|string|false|none|Optional human-readable description|
|arguments|[object]|false|none|Optional list of arguments for customization|
|» name|string|true|none|Name of the argument|
|» description|string|false|none|Description of the argument|
|» required|boolean|false|none|Whether the argument is required|
|» title|string|false|none|Optional human-readable title of the argument|

<h2 id="tocS_ErrorResponse">ErrorResponse</h2>

<a id="schemaerrorresponse"></a>
<a id="schema_ErrorResponse"></a>
<a id="tocSerrorresponse"></a>
<a id="tocserrorresponse"></a>

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

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|status|string|true|none|none|
|message|string|true|none|High-level error description|
|errors|[[ValidationError](#schemavalidationerror)]|false|none|Detailed validation errors|

<h2 id="tocS_ValidationError">ValidationError</h2>

<a id="schemavalidationerror"></a>
<a id="schema_ValidationError"></a>
<a id="tocSvalidationerror"></a>
<a id="tocsvalidationerror"></a>

```json
{
  "field": "spec.context",
  "message": "Context must start with / and cannot end with /"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|field|string|false|none|Field that failed validation|
|message|string|false|none|Human-readable error message|

<h2 id="tocS_LLMProviderTemplateRequest">LLMProviderTemplateRequest</h2>

<a id="schemallmprovidertemplaterequest"></a>
<a id="schema_LLMProviderTemplateRequest"></a>
<a id="tocSllmprovidertemplaterequest"></a>
<a id="tocsllmprovidertemplaterequest"></a>

```json
{
  "apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
  "kind": "LlmProviderTemplate",
  "metadata": {
    "name": "openai-template"
  },
  "spec": {
    "displayName": "OpenAI",
    "promptTokens": {
      "location": "payload",
      "identifier": "$.usage.prompt_tokens"
    },
    "completionTokens": {
      "location": "payload",
      "identifier": "$.usage.completion_tokens"
    },
    "totalTokens": {
      "location": "payload",
      "identifier": "$.usage.total_tokens"
    },
    "remainingTokens": {
      "location": "header",
      "identifier": "x-ratelimit-remaining-tokens"
    },
    "requestModel": {
      "location": "payload",
      "identifier": "$.model"
    },
    "responseModel": {
      "location": "payload",
      "identifier": "$.model"
    }
  }
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|apiVersion|string|true|none|Template specification version|
|kind|string|true|none|Template kind|
|metadata|[Metadata](#schemametadata)|true|none|none|
|spec|[LLMProviderTemplateData](#schemallmprovidertemplatedata)|true|none|none|

#### Enumerated Values

|Property|Value|
|---|---|
|apiVersion|gateway.api-platform.wso2.com/v1alpha1|
|kind|LlmProviderTemplate|

<h2 id="tocS_LLMProviderTemplate">LLMProviderTemplate</h2>

<a id="schemallmprovidertemplate"></a>
<a id="schema_LLMProviderTemplate"></a>
<a id="tocSllmprovidertemplate"></a>
<a id="tocsllmprovidertemplate"></a>

```json
{
  "apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
  "kind": "LlmProviderTemplate",
  "metadata": {
    "name": "openai-template"
  },
  "spec": {
    "displayName": "OpenAI",
    "promptTokens": {
      "location": "payload",
      "identifier": "$.usage.prompt_tokens"
    },
    "completionTokens": {
      "location": "payload",
      "identifier": "$.usage.completion_tokens"
    },
    "totalTokens": {
      "location": "payload",
      "identifier": "$.usage.total_tokens"
    },
    "remainingTokens": {
      "location": "header",
      "identifier": "x-ratelimit-remaining-tokens"
    },
    "requestModel": {
      "location": "payload",
      "identifier": "$.model"
    },
    "responseModel": {
      "location": "payload",
      "identifier": "$.model"
    }
  },
  "status": {
    "id": "openai-template",
    "createdAt": "2026-04-24T07:21:13Z",
    "updatedAt": "2026-04-24T07:21:13Z"
  }
}

```

### Properties

allOf

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|[LLMProviderTemplateRequest](#schemallmprovidertemplaterequest)|false|none|none|

and

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|object|false|none|none|
|» status|[ResourceStatus](#schemaresourcestatus)|false|read-only|Server-managed lifecycle fields. Populated on responses.|

<h2 id="tocS_LLMProviderTemplateData">LLMProviderTemplateData</h2>

<a id="schemallmprovidertemplatedata"></a>
<a id="schema_LLMProviderTemplateData"></a>
<a id="tocSllmprovidertemplatedata"></a>
<a id="tocsllmprovidertemplatedata"></a>

```json
{
  "displayName": "OpenAI",
  "promptTokens": {
    "location": "payload",
    "identifier": "$.usage.inputTokens"
  },
  "completionTokens": {
    "location": "payload",
    "identifier": "$.usage.inputTokens"
  },
  "totalTokens": {
    "location": "payload",
    "identifier": "$.usage.inputTokens"
  },
  "remainingTokens": {
    "location": "payload",
    "identifier": "$.usage.inputTokens"
  },
  "requestModel": {
    "location": "payload",
    "identifier": "$.usage.inputTokens"
  },
  "responseModel": {
    "location": "payload",
    "identifier": "$.usage.inputTokens"
  },
  "resourceMappings": {
    "resources": [
      {
        "resource": "/responses",
        "promptTokens": {
          "location": "payload",
          "identifier": "$.usage.inputTokens"
        },
        "completionTokens": {
          "location": "payload",
          "identifier": "$.usage.inputTokens"
        },
        "totalTokens": {
          "location": "payload",
          "identifier": "$.usage.inputTokens"
        },
        "remainingTokens": {
          "location": "payload",
          "identifier": "$.usage.inputTokens"
        },
        "requestModel": {
          "location": "payload",
          "identifier": "$.usage.inputTokens"
        },
        "responseModel": {
          "location": "payload",
          "identifier": "$.usage.inputTokens"
        }
      }
    ]
  }
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|displayName|string|true|none|Human-readable LLM Template name|
|promptTokens|[ExtractionIdentifier](#schemaextractionidentifier)|false|none|none|
|completionTokens|[ExtractionIdentifier](#schemaextractionidentifier)|false|none|none|
|totalTokens|[ExtractionIdentifier](#schemaextractionidentifier)|false|none|none|
|remainingTokens|[ExtractionIdentifier](#schemaextractionidentifier)|false|none|none|
|requestModel|[ExtractionIdentifier](#schemaextractionidentifier)|false|none|none|
|responseModel|[ExtractionIdentifier](#schemaextractionidentifier)|false|none|none|
|resourceMappings|[LLMProviderTemplateResourceMappings](#schemallmprovidertemplateresourcemappings)|false|none|none|

<h2 id="tocS_LLMProviderTemplateResourceMappings">LLMProviderTemplateResourceMappings</h2>

<a id="schemallmprovidertemplateresourcemappings"></a>
<a id="schema_LLMProviderTemplateResourceMappings"></a>
<a id="tocSllmprovidertemplateresourcemappings"></a>
<a id="tocsllmprovidertemplateresourcemappings"></a>

```json
{
  "resources": [
    {
      "resource": "/responses",
      "promptTokens": {
        "location": "payload",
        "identifier": "$.usage.inputTokens"
      },
      "completionTokens": {
        "location": "payload",
        "identifier": "$.usage.inputTokens"
      },
      "totalTokens": {
        "location": "payload",
        "identifier": "$.usage.inputTokens"
      },
      "remainingTokens": {
        "location": "payload",
        "identifier": "$.usage.inputTokens"
      },
      "requestModel": {
        "location": "payload",
        "identifier": "$.usage.inputTokens"
      },
      "responseModel": {
        "location": "payload",
        "identifier": "$.usage.inputTokens"
      }
    }
  ]
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|resources|[[LLMProviderTemplateResourceMapping](#schemallmprovidertemplateresourcemapping)]|false|none|none|

<h2 id="tocS_LLMProviderTemplateResourceMapping">LLMProviderTemplateResourceMapping</h2>

<a id="schemallmprovidertemplateresourcemapping"></a>
<a id="schema_LLMProviderTemplateResourceMapping"></a>
<a id="tocSllmprovidertemplateresourcemapping"></a>
<a id="tocsllmprovidertemplateresourcemapping"></a>

```json
{
  "resource": "/responses",
  "promptTokens": {
    "location": "payload",
    "identifier": "$.usage.inputTokens"
  },
  "completionTokens": {
    "location": "payload",
    "identifier": "$.usage.inputTokens"
  },
  "totalTokens": {
    "location": "payload",
    "identifier": "$.usage.inputTokens"
  },
  "remainingTokens": {
    "location": "payload",
    "identifier": "$.usage.inputTokens"
  },
  "requestModel": {
    "location": "payload",
    "identifier": "$.usage.inputTokens"
  },
  "responseModel": {
    "location": "payload",
    "identifier": "$.usage.inputTokens"
  }
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|resource|string|true|none|Resource path pattern for this mapping|
|promptTokens|[ExtractionIdentifier](#schemaextractionidentifier)|false|none|none|
|completionTokens|[ExtractionIdentifier](#schemaextractionidentifier)|false|none|none|
|totalTokens|[ExtractionIdentifier](#schemaextractionidentifier)|false|none|none|
|remainingTokens|[ExtractionIdentifier](#schemaextractionidentifier)|false|none|none|
|requestModel|[ExtractionIdentifier](#schemaextractionidentifier)|false|none|none|
|responseModel|[ExtractionIdentifier](#schemaextractionidentifier)|false|none|none|

<h2 id="tocS_ExtractionIdentifier">ExtractionIdentifier</h2>

<a id="schemaextractionidentifier"></a>
<a id="schema_ExtractionIdentifier"></a>
<a id="tocSextractionidentifier"></a>
<a id="tocsextractionidentifier"></a>

```json
{
  "location": "payload",
  "identifier": "$.usage.inputTokens"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|location|string|true|none|Where to find the token information|
|identifier|string|true|none|JSONPath expression or header name to identify the token value|

#### Enumerated Values

|Property|Value|
|---|---|
|location|payload|
|location|header|
|location|queryParam|
|location|pathParam|

<h2 id="tocS_LLMProviderConfigurationRequest">LLMProviderConfigurationRequest</h2>

<a id="schemallmproviderconfigurationrequest"></a>
<a id="schema_LLMProviderConfigurationRequest"></a>
<a id="tocSllmproviderconfigurationrequest"></a>
<a id="tocsllmproviderconfigurationrequest"></a>

```json
{
  "apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
  "kind": "LlmProvider",
  "metadata": {
    "name": "wso2-openai-provider"
  },
  "spec": {
    "displayName": "OpenAI Provider",
    "version": "v1.0",
    "template": "openai",
    "context": "/openai/latest",
    "upstream": {
      "url": "https://api.openai.com/v1",
      "auth": {
        "type": "api-key",
        "header": "Authorization",
        "value": "Bearer sk-your-api-key"
      }
    },
    "accessControl": {
      "mode": "deny_all",
      "exceptions": [
        {
          "path": "/chat/completions",
          "methods": [
            "POST"
          ]
        },
        {
          "path": "/models",
          "methods": [
            "GET"
          ]
        },
        {
          "path": "/models/{modelId}",
          "methods": [
            "GET"
          ]
        }
      ]
    }
  }
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|apiVersion|string|true|none|Provider specification version|
|kind|string|true|none|Provider kind|
|metadata|[Metadata](#schemametadata)|true|none|none|
|spec|[LLMProviderConfigData](#schemallmproviderconfigdata)|true|none|none|

#### Enumerated Values

|Property|Value|
|---|---|
|apiVersion|gateway.api-platform.wso2.com/v1alpha1|
|kind|LlmProvider|

<h2 id="tocS_LLMProviderConfiguration">LLMProviderConfiguration</h2>

<a id="schemallmproviderconfiguration"></a>
<a id="schema_LLMProviderConfiguration"></a>
<a id="tocSllmproviderconfiguration"></a>
<a id="tocsllmproviderconfiguration"></a>

```json
{
  "apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
  "kind": "LlmProvider",
  "metadata": {
    "name": "wso2-openai-provider"
  },
  "spec": {
    "displayName": "OpenAI Provider",
    "version": "v1.0",
    "template": "openai",
    "context": "/openai/latest",
    "upstream": {
      "url": "https://api.openai.com/v1",
      "auth": {
        "type": "api-key",
        "header": "Authorization",
        "value": "Bearer sk-your-api-key"
      }
    },
    "accessControl": {
      "mode": "deny_all",
      "exceptions": [
        {
          "path": "/chat/completions",
          "methods": [
            "POST"
          ]
        },
        {
          "path": "/models",
          "methods": [
            "GET"
          ]
        },
        {
          "path": "/models/{modelId}",
          "methods": [
            "GET"
          ]
        }
      ]
    }
  },
  "status": {
    "id": "wso2-openai-provider",
    "state": "deployed",
    "createdAt": "2026-04-24T07:21:13Z",
    "updatedAt": "2026-04-24T07:21:13Z",
    "deployedAt": "2026-04-24T07:21:13Z"
  }
}

```

### Properties

allOf

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|[LLMProviderConfigurationRequest](#schemallmproviderconfigurationrequest)|false|none|none|

and

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|object|false|none|none|
|» status|[ResourceStatus](#schemaresourcestatus)|false|read-only|Server-managed lifecycle fields. Populated on responses.|

<h2 id="tocS_LLMProviderConfigData">LLMProviderConfigData</h2>

<a id="schemallmproviderconfigdata"></a>
<a id="schema_LLMProviderConfigData"></a>
<a id="tocSllmproviderconfigdata"></a>
<a id="tocsllmproviderconfigdata"></a>

```json
{
  "displayName": "WSO2 OpenAI Provider",
  "version": "v1.0",
  "context": "/openai",
  "vhost": "api.openai.com",
  "template": "openai",
  "upstream": {
    "url": "http://prod-backend:5000/api/v2",
    "ref": "string",
    "hostRewrite": "auto",
    "auth": {
      "type": "api-key",
      "header": "string",
      "value": "string"
    }
  },
  "accessControl": {
    "mode": "deny_all",
    "exceptions": [
      {
        "path": "/chat/completions",
        "methods": [
          "GET"
        ]
      }
    ]
  },
  "policies": [
    {
      "name": "llm-cost-based-ratelimit",
      "version": "v1",
      "paths": [
        {
          "path": "/chat/completions",
          "methods": [
            "GET"
          ],
          "params": {}
        }
      ]
    }
  ],
  "deploymentState": "deployed"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|displayName|string|true|none|Human-readable LLM Provider name|
|version|string|true|none|Semantic version of the LLM Provider|
|context|string|false|none|Base path for all API routes (must start with /, no trailing slash)|
|vhost|string|false|none|Virtual host name used for routing. Supports standard domain names, subdomains, or wildcard domains. Must follow RFC-compliant hostname rules. Wildcards are only allowed in the left-most label (e.g., *.example.com).|
|template|string|true|none|Template name to use for this LLM Provider|
|upstream|any|true|none|none|

allOf

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|» *anonymous*|[Upstream](#schemaupstream)|false|none|Upstream backend configuration (single target or reference)|

and

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|» *anonymous*|[UpstreamAuth](#schemaupstreamauth)|false|none|none|

continued

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|accessControl|[LLMAccessControl](#schemallmaccesscontrol)|true|none|none|
|policies|[[LLMPolicy](#schemallmpolicy)]|false|none|List of policies applied only to this operation (overrides or adds to API-level policies)|
|deploymentState|string|false|none|Desired deployment state - 'deployed' (default) or 'undeployed'. When set to 'undeployed', the LLM Provider is removed from router traffic but configuration and policies are preserved for potential redeployment.|

#### Enumerated Values

|Property|Value|
|---|---|
|deploymentState|deployed|
|deploymentState|undeployed|

<h2 id="tocS_UpstreamAuth">UpstreamAuth</h2>

<a id="schemaupstreamauth"></a>
<a id="schema_UpstreamAuth"></a>
<a id="tocSupstreamauth"></a>
<a id="tocsupstreamauth"></a>

```json
{
  "auth": {
    "type": "api-key",
    "header": "string",
    "value": "string"
  }
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|auth|object|false|none|none|
|» type|string|true|none|none|
|» header|string|false|none|none|
|» value|string|false|none|none|

#### Enumerated Values

|Property|Value|
|---|---|
|type|api-key|

<h2 id="tocS_LLMUpstreamAuth">LLMUpstreamAuth</h2>

<a id="schemallmupstreamauth"></a>
<a id="schema_LLMUpstreamAuth"></a>
<a id="tocSllmupstreamauth"></a>
<a id="tocsllmupstreamauth"></a>

```json
{
  "type": "api-key",
  "header": "string",
  "value": "string"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|type|string|true|none|none|
|header|string|false|none|none|
|value|string|false|none|none|

#### Enumerated Values

|Property|Value|
|---|---|
|type|api-key|

<h2 id="tocS_LLMProxyProvider">LLMProxyProvider</h2>

<a id="schemallmproxyprovider"></a>
<a id="schema_LLMProxyProvider"></a>
<a id="tocSllmproxyprovider"></a>
<a id="tocsllmproxyprovider"></a>

```json
{
  "id": "wso2-openai-provider",
  "auth": {
    "type": "api-key",
    "header": "string",
    "value": "string"
  }
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|id|string|true|none|Unique id of a deployed llm provider|
|auth|[LLMUpstreamAuth](#schemallmupstreamauth)|false|none|none|

<h2 id="tocS_LLMAccessControl">LLMAccessControl</h2>

<a id="schemallmaccesscontrol"></a>
<a id="schema_LLMAccessControl"></a>
<a id="tocSllmaccesscontrol"></a>
<a id="tocsllmaccesscontrol"></a>

```json
{
  "mode": "deny_all",
  "exceptions": [
    {
      "path": "/chat/completions",
      "methods": [
        "GET"
      ]
    }
  ]
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|mode|string|true|none|Access control mode|
|exceptions|[[RouteException](#schemarouteexception)]|false|none|Path exceptions to the access control mode|

#### Enumerated Values

|Property|Value|
|---|---|
|mode|allow_all|
|mode|deny_all|

<h2 id="tocS_RouteException">RouteException</h2>

<a id="schemarouteexception"></a>
<a id="schema_RouteException"></a>
<a id="tocSrouteexception"></a>
<a id="tocsrouteexception"></a>

```json
{
  "path": "/chat/completions",
  "methods": [
    "GET"
  ]
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|path|string|true|none|Path pattern|
|methods|[string]|true|none|HTTP methods|

<h2 id="tocS_LLMPolicy">LLMPolicy</h2>

<a id="schemallmpolicy"></a>
<a id="schema_LLMPolicy"></a>
<a id="tocSllmpolicy"></a>
<a id="tocsllmpolicy"></a>

```json
{
  "name": "llm-cost-based-ratelimit",
  "version": "v1",
  "paths": [
    {
      "path": "/chat/completions",
      "methods": [
        "GET"
      ],
      "params": {}
    }
  ]
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|name|string|true|none|none|
|version|string|true|none|none|
|paths|[[LLMPolicyPath](#schemallmpolicypath)]|true|none|none|

<h2 id="tocS_LLMPolicyPath">LLMPolicyPath</h2>

<a id="schemallmpolicypath"></a>
<a id="schema_LLMPolicyPath"></a>
<a id="tocSllmpolicypath"></a>
<a id="tocsllmpolicypath"></a>

```json
{
  "path": "/chat/completions",
  "methods": [
    "GET"
  ],
  "params": {}
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|path|string|true|none|none|
|methods|[string]|true|none|none|
|params|object|true|none|JSON Schema describing the parameters accepted by this policy. This itself is a JSON Schema document.|

<h2 id="tocS_LLMProxyConfigurationRequest">LLMProxyConfigurationRequest</h2>

<a id="schemallmproxyconfigurationrequest"></a>
<a id="schema_LLMProxyConfigurationRequest"></a>
<a id="tocSllmproxyconfigurationrequest"></a>
<a id="tocsllmproxyconfigurationrequest"></a>

```json
{
  "apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
  "kind": "LlmProxy",
  "metadata": {
    "name": "openai-proxy"
  },
  "spec": {
    "displayName": "OpenAI Proxy",
    "version": "v1.0",
    "context": "/openai-proxy",
    "provider": {
      "id": "wso2-openai-provider"
    },
    "policies": []
  }
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|apiVersion|string|true|none|Proxy specification version|
|kind|string|true|none|Proxy kind|
|metadata|[Metadata](#schemametadata)|true|none|none|
|spec|[LLMProxyConfigData](#schemallmproxyconfigdata)|true|none|none|

#### Enumerated Values

|Property|Value|
|---|---|
|apiVersion|gateway.api-platform.wso2.com/v1alpha1|
|kind|LlmProxy|

<h2 id="tocS_LLMProxyConfiguration">LLMProxyConfiguration</h2>

<a id="schemallmproxyconfiguration"></a>
<a id="schema_LLMProxyConfiguration"></a>
<a id="tocSllmproxyconfiguration"></a>
<a id="tocsllmproxyconfiguration"></a>

```json
{
  "apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
  "kind": "LlmProxy",
  "metadata": {
    "name": "openai-proxy"
  },
  "spec": {
    "displayName": "OpenAI Proxy",
    "version": "v1.0",
    "context": "/openai-proxy",
    "provider": {
      "id": "wso2-openai-provider"
    },
    "policies": []
  },
  "status": {
    "id": "openai-proxy",
    "state": "deployed",
    "createdAt": "2026-04-24T07:21:13Z",
    "updatedAt": "2026-04-24T07:21:13Z",
    "deployedAt": "2026-04-24T07:21:13Z"
  }
}

```

### Properties

allOf

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|[LLMProxyConfigurationRequest](#schemallmproxyconfigurationrequest)|false|none|none|

and

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|object|false|none|none|
|» status|[ResourceStatus](#schemaresourcestatus)|false|read-only|Server-managed lifecycle fields. Populated on responses.|

<h2 id="tocS_LLMProxyConfigData">LLMProxyConfigData</h2>

<a id="schemallmproxyconfigdata"></a>
<a id="schema_LLMProxyConfigData"></a>
<a id="tocSllmproxyconfigdata"></a>
<a id="tocsllmproxyconfigdata"></a>

```json
{
  "displayName": "OpenAI Proxy",
  "version": "v1.0",
  "context": "/openai",
  "vhost": "api.openai.com",
  "provider": {
    "id": "wso2-openai-provider",
    "auth": {
      "type": "api-key",
      "header": "string",
      "value": "string"
    }
  },
  "policies": [
    {
      "name": "llm-cost-based-ratelimit",
      "version": "v1",
      "paths": [
        {
          "path": "/chat/completions",
          "methods": [
            "GET"
          ],
          "params": {}
        }
      ]
    }
  ],
  "deploymentState": "deployed"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|displayName|string|true|none|Human-readable LLM proxy name (must be URL-friendly - only letters, numbers, spaces, hyphens, underscores, and dots allowed)|
|version|string|true|none|Semantic version of the LLM proxy|
|context|string|false|none|Base path for all API routes (must start with /, no trailing slash)|
|vhost|string|false|none|Virtual host name used for routing. Supports standard domain names, subdomains, or wildcard domains. Must follow RFC-compliant hostname rules. Wildcards are only allowed in the left-most label (e.g., *.example.com).|
|provider|[LLMProxyProvider](#schemallmproxyprovider)|true|none|none|
|policies|[[LLMPolicy](#schemallmpolicy)]|false|none|List of policies applied only to this operation (overrides or adds to API-level policies)|
|deploymentState|string|false|none|Desired deployment state - 'deployed' (default) or 'undeployed'. When set to 'undeployed', the LLM Proxy is removed from router traffic but configuration and policies are preserved for potential redeployment.|

#### Enumerated Values

|Property|Value|
|---|---|
|deploymentState|deployed|
|deploymentState|undeployed|

<h2 id="tocS_SecretConfigurationRequest">SecretConfigurationRequest</h2>

<a id="schemasecretconfigurationrequest"></a>
<a id="schema_SecretConfigurationRequest"></a>
<a id="tocSsecretconfigurationrequest"></a>
<a id="tocssecretconfigurationrequest"></a>

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

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|apiVersion|string|true|none|Secret specification version|
|kind|string|true|none|Secret resource kind|
|metadata|[Metadata](#schemametadata)|true|none|none|
|spec|[SecretConfigData](#schemasecretconfigdata)|true|none|none|

#### Enumerated Values

|Property|Value|
|---|---|
|apiVersion|gateway.api-platform.wso2.com/v1alpha1|
|kind|Secret|

<h2 id="tocS_SecretConfiguration">SecretConfiguration</h2>

<a id="schemasecretconfiguration"></a>
<a id="schema_SecretConfiguration"></a>
<a id="tocSsecretconfiguration"></a>
<a id="tocssecretconfiguration"></a>

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
    "createdAt": "2026-04-24T07:21:13Z",
    "updatedAt": "2026-04-24T07:21:13Z"
  }
}

```

Composite of request fields plus a generic k8s-style `status` for documentation.
For actual HTTP response shapes, use `SecretConfigurationResponseCreateUpdate`,
`SecretConfigurationResponseRetrieved`, or `SecretListItem` (see
`SecretResourceServiceStatus` for the id/timestamp-only status on secret APIs).

### Properties

allOf

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|[SecretConfigurationRequest](#schemasecretconfigurationrequest)|false|none|none|

and

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|object|false|none|none|
|» status|[ResourceStatus](#schemaresourcestatus)|false|read-only|Server-managed lifecycle fields. Populated on responses.|

<h2 id="tocS_SecretConfigData">SecretConfigData</h2>

<a id="schemasecretconfigdata"></a>
<a id="schema_SecretConfigData"></a>
<a id="tocSsecretconfigdata"></a>
<a id="tocssecretconfigdata"></a>

```json
{
  "displayName": "Database Password",
  "description": "PostgreSQL main database password",
  "value": "sup3rs3cr3t!"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|displayName|string|true|none|Human-readable secret name (must be URL-friendly - only letters, numbers, spaces, hyphens, underscores, and dots allowed)|
|description|string|false|none|Description of the secret|
|value|string(password)|true|none|Secret value (stored encrypted)|

<h2 id="tocS_SecretConfigListData">SecretConfigListData</h2>

<a id="schemasecretconfiglistdata"></a>
<a id="schema_SecretConfigListData"></a>
<a id="tocSsecretconfiglistdata"></a>
<a id="tocssecretconfiglistdata"></a>

```json
{
  "displayName": "Database Password",
  "description": "PostgreSQL main database password"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|displayName|string|true|none|Human-readable secret name (must be URL-friendly - only letters, numbers, spaces, hyphens, underscores, and dots allowed)|
|description|string|false|none|Description of the secret, if the server includes it|

<h2 id="tocS_SecretListItem">SecretListItem</h2>

<a id="schemasecretlistitem"></a>
<a id="schema_SecretListItem"></a>
<a id="tocSsecretlistitem"></a>
<a id="tocssecretlistitem"></a>

```json
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

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|apiVersion|string|true|none|Secret specification version|
|kind|string|true|none|Secret resource kind|
|metadata|[Metadata](#schemametadata)|true|none|none|
|spec|[SecretConfigListData](#schemasecretconfiglistdata)|true|none|none|
|status|[ResourceStatus](#schemaresourcestatus)|false|read-only|Server-managed lifecycle fields. Omitted in list items may vary; the secret value is never included here.|

#### Enumerated Values

|Property|Value|
|---|---|
|apiVersion|gateway.api-platform.wso2.com/v1alpha1|
|kind|Secret|

<h2 id="tocS_CertificateUploadRequest">CertificateUploadRequest</h2>

<a id="schemacertificateuploadrequest"></a>
<a id="schema_CertificateUploadRequest"></a>
<a id="tocScertificateuploadrequest"></a>
<a id="tocscertificateuploadrequest"></a>

```json
{
  "name": "my-custom-ca",
  "certificate": "-----BEGIN CERTIFICATE-----\nMIIDXTCCAkWgAwIBAgIJAKL0UG+mRKtjMA0GCSqGSIb3DQEBCwUAMEUxCzAJBgNV\n...\n-----END CERTIFICATE-----\n"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|name|string|true|none|Unique name for the certificate. Must be unique across all certificates.|
|certificate|string|true|none|PEM-encoded X.509 certificate(s). Can contain multiple certificates.|

<h2 id="tocS_CertificateResponse">CertificateResponse</h2>

<a id="schemacertificateresponse"></a>
<a id="schema_CertificateResponse"></a>
<a id="tocScertificateresponse"></a>
<a id="tocscertificateresponse"></a>

```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "name": "my-custom-ca",
  "subject": "CN=My CA,O=My Organization,C=US",
  "issuer": "CN=My CA,O=My Organization,C=US",
  "notAfter": "2026-11-26 06:07:26",
  "count": 1,
  "message": "Certificate uploaded and SDS updated successfully",
  "status": "success"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|id|string|false|none|Unique identifier (UUID) for the certificate|
|name|string|false|none|Name of the certificate|
|subject|string|false|none|Certificate subject DN (for first cert if bundle)|
|issuer|string|false|none|Certificate issuer DN (for first cert if bundle)|
|notAfter|string(date-time)|false|none|Certificate expiration date (for first cert if bundle)|
|count|integer|false|none|Number of certificates in the file|
|message|string|false|none|Success or informational message|
|status|string|false|none|none|

#### Enumerated Values

|Property|Value|
|---|---|
|status|success|
|status|error|

<h2 id="tocS_CertificateListResponse">CertificateListResponse</h2>

<a id="schemacertificatelistresponse"></a>
<a id="schema_CertificateListResponse"></a>
<a id="tocScertificatelistresponse"></a>
<a id="tocscertificatelistresponse"></a>

```json
{
  "certificates": [
    {
      "id": "550e8400-e29b-41d4-a716-446655440000",
      "name": "my-custom-ca",
      "subject": "CN=My CA,O=My Organization,C=US",
      "issuer": "CN=My CA,O=My Organization,C=US",
      "notAfter": "2026-11-26 06:07:26",
      "count": 1,
      "message": "Certificate uploaded and SDS updated successfully",
      "status": "success"
    }
  ],
  "totalCount": 3,
  "totalBytes": 221599,
  "status": "success"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|certificates|[[CertificateResponse](#schemacertificateresponse)]|false|none|none|
|totalCount|integer|false|none|Total number of certificate files|
|totalBytes|integer|false|none|Total bytes of all certificate files|
|status|string|false|none|none|

<h2 id="tocS_APIKeyListResponse">APIKeyListResponse</h2>

<a id="schemaapikeylistresponse"></a>
<a id="schema_APIKeyListResponse"></a>
<a id="tocSapikeylistresponse"></a>
<a id="tocsapikeylistresponse"></a>

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

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|apiKeys|[[APIKey](#schemaapikey)]|false|none|[Details of an API key]|
|totalCount|integer|false|none|Total number of API keys|
|status|string|false|none|none|

<h2 id="tocS_SecretListResponse">SecretListResponse</h2>

<a id="schemasecretlistresponse"></a>
<a id="schema_SecretListResponse"></a>
<a id="tocSsecretlistresponse"></a>
<a id="tocssecretlistresponse"></a>

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

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|status|string|false|none|none|
|count|integer|false|none|Total number of secrets|
|secrets|[[SecretListItem](#schemasecretlistitem)]|false|none|List of secrets. For security, the spec.value field is omitted for every item in the list; retrieve a single secret by id to obtain the decrypted value.|

<h2 id="tocS_SecretResourceServiceStatus">SecretResourceServiceStatus</h2>

<a id="schemasecretresourceservicestatus"></a>
<a id="schema_SecretResourceServiceStatus"></a>
<a id="tocSsecretresourceservicestatus"></a>
<a id="tocssecretresourceservicestatus"></a>

```json
{
  "id": "database-password",
  "createdAt": "2026-01-05T10:30:00Z",
  "updatedAt": "2026-01-05T10:30:00Z"
}

```

Id and optional timestamps. Not the full ResourceStatus model (no `state` or
`deployedAt`).

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|id|string|true|none|Same as metadata.name / secret handle|
|createdAt|string(date-time)|false|none|none|
|updatedAt|string(date-time)|false|none|none|

<h2 id="tocS_SecretConfigurationResponseCreateUpdate">SecretConfigurationResponseCreateUpdate</h2>

<a id="schemasecretconfigurationresponsecreateupdate"></a>
<a id="schema_SecretConfigurationResponseCreateUpdate"></a>
<a id="tocSsecretconfigurationresponsecreateupdate"></a>
<a id="tocssecretconfigurationresponsecreateupdate"></a>

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

POST/PUT /secrets response. `spec.value` is not returned; see SecretConfigurationRequest for create/update request bodies.

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|apiVersion|string|true|none|none|
|kind|string|true|none|none|
|metadata|[Metadata](#schemametadata)|true|none|none|
|spec|[SecretConfigListData](#schemasecretconfiglistdata)|true|none|none|
|status|[SecretResourceServiceStatus](#schemasecretresourceservicestatus)|true|none|Id and optional timestamps. Not the full ResourceStatus model (no `state` or<br>`deployedAt`).|

#### Enumerated Values

|Property|Value|
|---|---|
|apiVersion|gateway.api-platform.wso2.com/v1alpha1|
|kind|Secret|

<h2 id="tocS_SecretConfigurationResponseRetrieved">SecretConfigurationResponseRetrieved</h2>

<a id="schemasecretconfigurationresponseretrieved"></a>
<a id="schema_SecretConfigurationResponseRetrieved"></a>
<a id="tocSsecretconfigurationresponseretrieved"></a>
<a id="tocssecretconfigurationresponseretrieved"></a>

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

GET /secrets/{id} response including decrypted `spec.value`.

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|apiVersion|string|true|none|none|
|kind|string|true|none|none|
|metadata|[Metadata](#schemametadata)|true|none|none|
|spec|[SecretConfigData](#schemasecretconfigdata)|true|none|none|
|status|[SecretResourceServiceStatus](#schemasecretresourceservicestatus)|true|none|Id and optional timestamps. Not the full ResourceStatus model (no `state` or<br>`deployedAt`).|

#### Enumerated Values

|Property|Value|
|---|---|
|apiVersion|gateway.api-platform.wso2.com/v1alpha1|
|kind|Secret|
