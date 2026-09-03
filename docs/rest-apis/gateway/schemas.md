# Schemas

## ResourceStatus

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

#### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|id|string|false|none|Unique identifier assigned by the server (equal to metadata.name)|
|state|string|false|none|Desired deployment state reported by the server|
|createdAt|string(date-time)|false|none|Timestamp when the resource was first created (UTC)|
|updatedAt|string(date-time)|false|none|Timestamp when the resource was last updated (UTC)|
|deployedAt|string(date-time)|false|none|Timestamp when the resource was last deployed (omitted when undeployed)|

##### Enumerated Values

|Property|Value|
|---|---|
|state|deployed|
|state|undeployed|

## RestAPIRequest

<a id="schemarestapirequest"></a>
<a id="schema_RestAPIRequest"></a>
<a id="tocSrestapirequest"></a>
<a id="tocsrestapirequest"></a>

```json
{
  "apiVersion": "gateway.api-platform.wso2.com/v1",
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

#### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|apiVersion|string|true|none|API specification version|
|kind|string|true|none|API type|
|metadata|[Metadata](#schemametadata)|true|none|none|
|spec|[APIConfigData](#schemaapiconfigdata)|true|none|none|

##### Enumerated Values

|Property|Value|
|---|---|
|apiVersion|gateway.api-platform.wso2.com/v1|
|kind|RestApi|

## RestAPI

<a id="schemarestapi"></a>
<a id="schema_RestAPI"></a>
<a id="tocSrestapi"></a>
<a id="tocsrestapi"></a>

```json
{
  "apiVersion": "gateway.api-platform.wso2.com/v1",
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

#### Properties

allOf

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|[RestAPIRequest](#schemarestapirequest)|false|none|none|

and

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|object|false|none|none|
|» status|[ResourceStatus](#schemaresourcestatus)|false|read-only|Server-managed lifecycle fields. Populated on responses.|

## Metadata

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

#### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|name|string|true|none|Unique handle for the resource|
|labels|object|false|none|Labels are key-value pairs for organizing and selecting APIs. Keys must not contain spaces.|
|» **additionalProperties**|string|false|none|none|
|annotations|object|false|none|Annotations are arbitrary non-identifying metadata. Use domain-prefixed keys.|
|» **additionalProperties**|string|false|none|none|

## APIConfigData

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
    "main": "api.example.com;docs.example.com;*.example.com",
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
  "resilience": {
    "timeout": "15s",
    "idleTimeout": "0s"
  },
  "operations": [
    {
      "method": "GET",
      "path": "/books/{id}",
      "match": {
        "method": "GET",
        "path": {
          "value": "/books/{id}",
          "type": "Exact"
        },
        "headers": [
          {
            "name": "version",
            "value": "one",
            "type": "Exact"
          }
        ]
      },
      "policies": [
        {
          "name": "cors",
          "version": "v1",
          "executionCondition": "request.metadata[authenticated] != true",
          "params": {}
        }
      ],
      "resilience": {
        "timeout": "15s",
        "idleTimeout": "0s"
      }
    }
  ],
  "deploymentState": "deployed"
}

```

#### Properties

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
|» main|string|true|none|Custom virtual host(s)/domain(s) for production traffic. One or more hostnames<br>separated by ';' — each hostname serves the main upstream (e.g. when a Gateway API<br>HTTPRoute attaches to multiple listener hostnames). The first entry is the primary<br>vhost. Each hostname may be a wildcard such as *.example.com.|
|» sandbox|string|false|none|Custom virtual host/domain for sandbox traffic|
|subscriptionPlans|[string]|false|none|List of subscription plan names available for this API|
|policies|[[Policy](#schemapolicy)]|false|none|List of API-level policies applied to all operations unless overridden|
|resilience|[Resilience](#schemaresilience)|false|none|Backend/route timeout configuration. Maps to Envoy RouteAction timeouts. Can be set at the API level (applies to all routes) and/or the operation level (applies to that operation's route). When set at both levels, the operation-level value takes precedence. When unset, the gateway's global route timeout defaults apply.|
|operations|[[Operation](#schemaoperation)]|true|none|List of HTTP operations/routes|
|deploymentState|string|false|none|Desired deployment state - 'deployed' (default) or 'undeployed'. When set to 'undeployed', the API is removed from router traffic but configuration, API keys, and policies are preserved for potential redeployment.|

##### Enumerated Values

|Property|Value|
|---|---|
|deploymentState|deployed|
|deploymentState|undeployed|

## UpstreamDefinition

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

#### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|name|string|true|none|Unique identifier for this upstream definition|
|basePath|string|false|none|Base path prefix for all endpoints in this upstream (e.g., /api/v2). All requests to this upstream will have this path prepended. Must start with '/' and must not end with '/'; omit for root.|
|timeout|[UpstreamTimeout](#schemaupstreamtimeout)|false|none|Timeout configuration for upstream requests|
|upstreams|[object]|true|none|List of backend targets with optional weights for load balancing|
|» url|string(uri)|true|none|Backend URL (host and port only, path comes from basePath)|
|» weight|integer|false|none|Relative weight for load balancing across multiple upstream targets. Reserved for future multi-target load balancing; not applied yet (only the first target is currently used).|

## UpstreamTimeout

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

#### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|connect|string|false|none|Connection timeout duration (e.g., "5s", "500ms")|

## Resilience

<a id="schemaresilience"></a>
<a id="schema_Resilience"></a>
<a id="tocSresilience"></a>
<a id="tocsresilience"></a>

```json
{
  "timeout": "15s",
  "idleTimeout": "0s"
}

```

Backend/route timeout configuration. Maps to Envoy RouteAction timeouts. Can be set at the API level (applies to all routes) and/or the operation level (applies to that operation's route). When set at both levels, the operation-level value takes precedence. When unset, the gateway's global route timeout defaults apply.

#### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|timeout|string|false|none|Maximum time for the entire route (request to upstream response). "0s" disables the timeout.|
|idleTimeout|string|false|none|Per-route stream idle timeout (overrides the listener stream idle timeout for this route). "0s" disables the timeout.|

## Upstream

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

#### Properties

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

##### Enumerated Values

|Property|Value|
|---|---|
|hostRewrite|auto|
|hostRewrite|manual|

## Operation

<a id="schemaoperation"></a>
<a id="schema_Operation"></a>
<a id="tocSoperation"></a>
<a id="tocsoperation"></a>

```json
{
  "method": "GET",
  "path": "/books/{id}",
  "match": {
    "method": "GET",
    "path": {
      "value": "/books/{id}",
      "type": "Exact"
    },
    "headers": [
      {
        "name": "version",
        "value": "one",
        "type": "Exact"
      }
    ]
  },
  "policies": [
    {
      "name": "cors",
      "version": "v1",
      "executionCondition": "request.metadata[authenticated] != true",
      "params": {}
    }
  ],
  "resilience": {
    "timeout": "15s",
    "idleTimeout": "0s"
  }
}

```

An operation is matched either by the simple top-level method+path form, or by the richer 'match' block (method + path + headers). When 'match' is present it is authoritative and the top-level method/path are ignored. At least one form must be provided.

#### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|method|[OperationMethod](#schemaoperationmethod)|false|none|HTTP method (simple form; ignored when 'match' is set)|
|path|string|false|none|Route path with optional {param} placeholders (simple form; ignored when 'match' is set)|
|match|[OperationMatch](#schemaoperationmatch)|false|none|Request matching criteria for an operation. Extensible with query params, cookies, etc.|
|policies|[[Policy](#schemapolicy)]|false|none|List of policies applied only to this operation (overrides or adds to API-level policies)|
|resilience|[Resilience](#schemaresilience)|false|none|Backend/route timeout configuration. Maps to Envoy RouteAction timeouts. Can be set at the API level (applies to all routes) and/or the operation level (applies to that operation's route). When set at both levels, the operation-level value takes precedence. When unset, the gateway's global route timeout defaults apply.|

## OperationMethod

<a id="schemaoperationmethod"></a>
<a id="schema_OperationMethod"></a>
<a id="tocSoperationmethod"></a>
<a id="tocsoperationmethod"></a>

```json
"GET"

```

HTTP method

#### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|string|false|none|HTTP method|

##### Enumerated Values

|Property|Value|
|---|---|
|*anonymous*|GET|
|*anonymous*|POST|
|*anonymous*|PUT|
|*anonymous*|DELETE|
|*anonymous*|PATCH|
|*anonymous*|HEAD|
|*anonymous*|OPTIONS|

## OperationMatch

<a id="schemaoperationmatch"></a>
<a id="schema_OperationMatch"></a>
<a id="tocSoperationmatch"></a>
<a id="tocsoperationmatch"></a>

```json
{
  "method": "GET",
  "path": {
    "value": "/books/{id}",
    "type": "Exact"
  },
  "headers": [
    {
      "name": "version",
      "value": "one",
      "type": "Exact"
    }
  ]
}

```

Request matching criteria for an operation. Extensible with query params, cookies, etc.

#### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|method|[OperationMethod](#schemaoperationmethod)|true|none|HTTP method|
|path|[OperationPathMatch](#schemaoperationpathmatch)|true|none|none|
|headers|[[OperationHeaderMatch](#schemaoperationheadermatch)]|false|none|Header matchers ANDed with the path match for Envoy route selection|

## OperationPathMatch

<a id="schemaoperationpathmatch"></a>
<a id="schema_OperationPathMatch"></a>
<a id="tocSoperationpathmatch"></a>
<a id="tocsoperationpathmatch"></a>

```json
{
  "value": "/books/{id}",
  "type": "Exact"
}

```

#### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|value|string|true|none|Route path with optional {param} placeholders|
|type|string|false|none|Path matching semantics for the operation route|

##### Enumerated Values

|Property|Value|
|---|---|
|type|Exact|
|type|PathPrefix|

## OperationHeaderMatch

<a id="schemaoperationheadermatch"></a>
<a id="schema_OperationHeaderMatch"></a>
<a id="tocSoperationheadermatch"></a>
<a id="tocsoperationheadermatch"></a>

```json
{
  "name": "version",
  "value": "one",
  "type": "Exact"
}

```

#### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|name|string|true|none|Header name (case-insensitive)|
|value|string|true|none|Header value to match|
|type|string|false|none|Header match type|

##### Enumerated Values

|Property|Value|
|---|---|
|type|Exact|
|type|RegularExpression|

## Policy

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

#### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|name|string|true|none|Name of the policy|
|version|string|true|none|Version of the policy. Only major-only version is allowed (e.g., v0, v1). Full semantic version (e.g., v1.0.0) is not accepted and will be rejected. The Gateway Controller resolves the major version to the single matching full version installed in the gateway image.|
|executionCondition|string|false|none|Expression controlling conditional execution of the policy|
|params|object|false|none|Arbitrary parameters for the policy (free-form key/value structure)|

## Channel

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

#### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|name|string|true|none|Channel name or topic identifier relative to API context.|
|method|string|true|none|Operation method type.|
|policies|[[Policy](#schemapolicy)]|false|none|List of policies applied only to this channel (overrides or adds to API-level policies)|

##### Enumerated Values

|Property|Value|
|---|---|
|method|SUB|

## APIKeyCreationRequest

<a id="schemaapikeycreationrequest"></a>
<a id="schema_APIKeyCreationRequest"></a>
<a id="tocSapikeycreationrequest"></a>
<a id="tocsapikeycreationrequest"></a>

```json
{
  "name": "my-production-key"
}

```

#### Properties

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

##### Enumerated Values

|Property|Value|
|---|---|
|unit|seconds|
|unit|minutes|
|unit|hours|
|unit|days|
|unit|weeks|
|unit|months|

## APIKeyCreationResponse

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

#### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|status|string|true|none|none|
|message|string|true|none|none|
|remainingApiKeyQuota|integer|false|none|Remaining API key quota for the user|
|apiKey|[APIKey](#schemaapikey)|false|none|Details of an API key|

## APIKey

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

#### Properties

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

##### Enumerated Values

|Property|Value|
|---|---|
|status|active|
|status|revoked|
|status|expired|
|source|local|
|source|external|

## APIKeyRegenerationRequest

<a id="schemaapikeyregenerationrequest"></a>
<a id="schema_APIKeyRegenerationRequest"></a>
<a id="tocSapikeyregenerationrequest"></a>
<a id="tocsapikeyregenerationrequest"></a>

```json
{}

```

#### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|expiresIn|object|false|none|Expiration duration for the API key|
|» unit|string|true|none|Time unit for expiration|
|» duration|integer|true|none|Duration value for expiration|
|expiresAt|string(date-time)|false|none|Expiration timestamp|

##### Enumerated Values

|Property|Value|
|---|---|
|unit|seconds|
|unit|minutes|
|unit|hours|
|unit|days|
|unit|weeks|
|unit|months|

## APIKeyUpdateRequest

<a id="schemaapikeyupdaterequest"></a>
<a id="schema_APIKeyUpdateRequest"></a>
<a id="tocSapikeyupdaterequest"></a>
<a id="tocsapikeyupdaterequest"></a>

```json
{
  "name": "my-production-key"
}

```

#### Properties

*None*

## APIKeyRevocationResponse

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

#### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|status|string|true|none|none|
|message|string|true|none|none|

## SubscriptionPlanCreateRequest

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

#### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|planName|string|true|none|none|
|billingPlan|string|false|none|none|
|stopOnQuotaReach|boolean|false|none|none|
|throttleLimitCount|integer|false|none|none|
|throttleLimitUnit|string|false|none|none|
|expiryTime|string(date-time)|false|none|none|
|status|string|false|none|none|

##### Enumerated Values

|Property|Value|
|---|---|
|throttleLimitUnit|Min|
|throttleLimitUnit|Hour|
|throttleLimitUnit|Day|
|throttleLimitUnit|Month|
|status|ACTIVE|
|status|INACTIVE|

## SubscriptionPlanUpdateRequest

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

#### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|planName|string|false|none|none|
|billingPlan|string|false|none|none|
|stopOnQuotaReach|boolean|false|none|none|
|throttleLimitCount|integer|false|none|none|
|throttleLimitUnit|string|false|none|none|
|expiryTime|string(date-time)|false|none|none|
|status|string|false|none|none|

##### Enumerated Values

|Property|Value|
|---|---|
|throttleLimitUnit|Min|
|throttleLimitUnit|Hour|
|throttleLimitUnit|Day|
|throttleLimitUnit|Month|
|status|ACTIVE|
|status|INACTIVE|

## SubscriptionPlanResponse

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

#### Properties

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

##### Enumerated Values

|Property|Value|
|---|---|
|status|ACTIVE|
|status|INACTIVE|

## SubscriptionPlanListResponse

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

#### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|subscriptionPlans|[[SubscriptionPlanResponse](#schemasubscriptionplanresponse)]|false|none|none|
|count|integer|false|none|none|

## SubscriptionCreateRequest

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

#### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|apiId|string|true|none|API identifier (deployment ID or handle)|
|subscriptionToken|string|true|none|Opaque subscription token for API invocation (required; stored as hash only)|
|applicationId|string|false|none|Application identifier (from DevPortal/STS). Optional for token-based subscriptions.|
|subscriptionPlanId|string|false|none|Subscription plan UUID for rate limit and billing configuration.|
|billingCustomerId|string|false|none|Billing customer identifier (optional, for analytics tracking).|
|billingSubscriptionId|string|false|none|Billing subscription identifier (optional, for analytics tracking).|
|status|string|false|none|none|

##### Enumerated Values

|Property|Value|
|---|---|
|status|ACTIVE|
|status|INACTIVE|
|status|REVOKED|

## SubscriptionUpdateRequest

<a id="schemasubscriptionupdaterequest"></a>
<a id="schema_SubscriptionUpdateRequest"></a>
<a id="tocSsubscriptionupdaterequest"></a>
<a id="tocssubscriptionupdaterequest"></a>

```json
{
  "status": "ACTIVE"
}

```

#### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|status|string|false|none|none|

##### Enumerated Values

|Property|Value|
|---|---|
|status|ACTIVE|
|status|INACTIVE|
|status|REVOKED|

## SubscriptionResponse

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

#### Properties

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

##### Enumerated Values

|Property|Value|
|---|---|
|status|ACTIVE|
|status|INACTIVE|
|status|REVOKED|

## SubscriptionListResponse

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

#### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|subscriptions|[[SubscriptionResponse](#schemasubscriptionresponse)]|false|none|none|
|count|integer|false|none|none|

## MCPProxyConfigurationRequest

<a id="schemamcpproxyconfigurationrequest"></a>
<a id="schema_MCPProxyConfigurationRequest"></a>
<a id="tocSmcpproxyconfigurationrequest"></a>
<a id="tocsmcpproxyconfigurationrequest"></a>

```json
{
  "apiVersion": "gateway.api-platform.wso2.com/v1",
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

#### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|apiVersion|string|true|none|MCP Proxy specification version|
|kind|string|true|none|MCP Proxy type|
|metadata|[Metadata](#schemametadata)|true|none|none|
|spec|[MCPProxyConfigData](#schemamcpproxyconfigdata)|true|none|none|

##### Enumerated Values

|Property|Value|
|---|---|
|apiVersion|gateway.api-platform.wso2.com/v1|
|kind|Mcp|

## MCPProxyConfiguration

<a id="schemamcpproxyconfiguration"></a>
<a id="schema_MCPProxyConfiguration"></a>
<a id="tocSmcpproxyconfiguration"></a>
<a id="tocsmcpproxyconfiguration"></a>

```json
{
  "apiVersion": "gateway.api-platform.wso2.com/v1",
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

#### Properties

allOf

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|[MCPProxyConfigurationRequest](#schemamcpproxyconfigurationrequest)|false|none|none|

and

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|object|false|none|none|
|» status|[ResourceStatus](#schemaresourcestatus)|false|read-only|Server-managed lifecycle fields. Populated on responses.|

## MCPProxyConfigData

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
  "deploymentState": "deployed",
  "resilience": {
    "timeout": "15s",
    "idleTimeout": "0s"
  }
}

```

#### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|displayName|string|true|none|Human-readable MCP Proxy display name|
|version|string|true|none|MCP Proxy version|
|context|string|false|none|MCP Proxy context path|
|specVersion|string|false|none|MCP specification version|
|vhost|string|false|none|Virtual host name used for routing. Supports standard domain names, subdomains, or wildcard domains. Must follow RFC-compliant hostname rules. Wildcards are only allowed in the left-most label (e.g., *.example.com).|
|upstreamDefinitions|[[UpstreamDefinition](#schemaupstreamdefinition)]|false|none|List of reusable upstream definitions with optional timeout configurations. Referenced by upstream.ref.|
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
|resilience|[Resilience](#schemaresilience)|false|none|API-level backend/route timeout configuration. Applies to the traffic-forwarding routes generated for this MCP proxy (GET/POST/DELETE on the MCP resource path). Supported at the API level only. Because MCP transports are long-lived streams, the route timeout defaults to disabled ("0s") for MCP unless a timeout is set here (unlike REST/LLM, which fall back to the gateway's global route timeout); the idle timeout remains the liveness guard.|

##### Enumerated Values

|Property|Value|
|---|---|
|deploymentState|deployed|
|deploymentState|undeployed|

## MCPTool

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

#### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|name|string|true|none|Unique identifier for the tool|
|title|string|false|none|Optional human-readable name of the tool for display purposes.|
|description|string|true|none|Human-readable description of functionality|
|inputSchema|string|true|none|JSON Schema defining expected parameters|
|outputSchema|string|false|none|Optional JSON Schema defining expected output structure|

## MCPResource

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

#### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|uri|string|true|none|Unique identifier for the resource|
|name|string|true|none|The name of the resource|
|title|string|false|none|Optional human-readable name of the resource for display purposes|
|description|string|false|none|Optional description|
|mimeType|string|false|none|Optional MIME type|
|size|integer|false|none|Optional size in bytes|

## MCPPrompt

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

#### Properties

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

## AgentConfigurationRequest

<a id="schemaagentconfigurationrequest"></a>
<a id="schema_AgentConfigurationRequest"></a>
<a id="tocSagentconfigurationrequest"></a>
<a id="tocsagentconfigurationrequest"></a>

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

#### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|apiVersion|string|true|none|Agent specification version|
|kind|string|true|none|Agent type|
|metadata|[Metadata](#schemametadata)|true|none|none|
|spec|[AgentConfigData](#schemaagentconfigdata)|true|none|none|

##### Enumerated Values

|Property|Value|
|---|---|
|apiVersion|gateway.api-platform.wso2.com/v1|
|kind|Agent|

## AgentConfiguration

<a id="schemaagentconfiguration"></a>
<a id="schema_AgentConfiguration"></a>
<a id="tocSagentconfiguration"></a>
<a id="tocsagentconfiguration"></a>

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

#### Properties

allOf

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|[AgentConfigurationRequest](#schemaagentconfigurationrequest)|false|none|none|

and

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|object|false|none|none|
|» status|[ResourceStatus](#schemaresourcestatus)|false|read-only|Server-managed lifecycle fields. Populated on responses.|

## AgentConfigData

<a id="schemaagentconfigdata"></a>
<a id="schema_AgentConfigData"></a>
<a id="tocSagentconfigdata"></a>
<a id="tocsagentconfigdata"></a>

```json
{
  "displayName": "Weather Agent",
  "version": "v1.0",
  "context": "/weather",
  "vhost": "agents.example.com",
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
    "url": "http://prod-backend:5000/api/v2",
    "ref": "string",
    "hostRewrite": "auto",
    "auth": {
      "type": "api-key",
      "header": "string",
      "value": "string"
    }
  },
  "deploymentState": "deployed",
  "resilience": {
    "timeout": "15s",
    "idleTimeout": "0s"
  },
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
          "name": "cors",
          "version": "v1",
          "executionCondition": "request.metadata[authenticated] != true",
          "params": {}
        }
      ],
      "operations": [
        {
          "name": "SendMessage",
          "policies": [
            {
              "name": "cors",
              "version": "v1",
              "executionCondition": "request.metadata[authenticated] != true",
              "params": {}
            }
          ],
          "resilience": {
            "timeout": "15s",
            "idleTimeout": "0s"
          }
        }
      ]
    },
    "agentCard": {
      "public": {
        "mode": "managed",
        "content": {
          "name": "Weather Agent",
          "version": "1.0.0",
          "supportedInterfaces": [
            {
              "protocolBinding": "JSONRPC",
              "protocolVersion": "1.0",
              "url": "https://agents.example.com/weather/rpc"
            }
          ],
          "capabilities": {
            "streaming": true,
            "extendedAgentCard": true
          }
        }
      },
      "protected": {
        "mode": "managed",
        "content": {
          "name": "Weather Agent",
          "version": "1.0.0",
          "supportedInterfaces": [
            {
              "protocolBinding": "JSONRPC",
              "protocolVersion": "1.0",
              "url": "https://agents.example.com/weather/rpc"
            }
          ],
          "capabilities": {
            "streaming": true,
            "extendedAgentCard": true
          },
          "skills": [
            {
              "id": "get_forecast_history",
              "name": "Get forecast history"
            }
          ]
        }
      }
    }
  }
}

```

#### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|displayName|string|true|none|Human-readable agent display name|
|version|string|true|none|Agent version|
|context|string|false|none|Gateway context path for the agent (must start with /, no trailing slash). Optional: when omitted the agent is served at the root of its virtual host, which is where an A2A client probes for `/.well-known/agent-card.json` during cold discovery. Every A2A route the gateway generates — the transport base paths and the Agent Card path — is relative to this value.|
|vhost|string|false|none|Virtual host name used for routing. Supports standard domain names, subdomains, or wildcard domains. Must follow RFC-compliant hostname rules. Wildcards are only allowed in the left-most label (e.g., *.example.com).|
|upstreamDefinitions|[[UpstreamDefinition](#schemaupstreamdefinition)]|false|none|List of reusable upstream definitions with optional timeout configurations. Referenced by upstream.ref.|
|upstream|any|true|none|The backend A2A agent url and auth configuration. The URL is the base the gateway forwards A2A operation traffic to, and — in public passthrough card mode — the origin of the standard /.well-known/agent-card.json document.|

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
|deploymentState|string|false|none|Desired deployment state - 'deployed' (default) or 'undeployed'. When set to 'undeployed', the Agent is removed from router traffic but configuration and policies are preserved for potential redeployment.|
|resilience|[Resilience](#schemaresilience)|false|none|Agent-level backend/route timeout configuration. Applies to the traffic-forwarding routes generated for this Agent. Because A2A streaming operations are long-lived streams, the route timeout defaults to disabled ("0s") for the JSON-RPC route and for streaming HTTP+JSON routes unless a timeout is set here (unlike REST/LLM, which fall back to the gateway's global route timeout); the idle timeout remains the liveness guard.|
|a2a|[A2AConfig](#schemaa2aconfig)|true|none|A2A-specific agent configuration.|

##### Enumerated Values

|Property|Value|
|---|---|
|deploymentState|deployed|
|deploymentState|undeployed|

## A2AConfig

<a id="schemaa2aconfig"></a>
<a id="schema_A2AConfig"></a>
<a id="tocSa2aconfig"></a>
<a id="tocsa2aconfig"></a>

```json
{
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
        "name": "cors",
        "version": "v1",
        "executionCondition": "request.metadata[authenticated] != true",
        "params": {}
      }
    ],
    "operations": [
      {
        "name": "SendMessage",
        "policies": [
          {
            "name": "cors",
            "version": "v1",
            "executionCondition": "request.metadata[authenticated] != true",
            "params": {}
          }
        ],
        "resilience": {
          "timeout": "15s",
          "idleTimeout": "0s"
        }
      }
    ]
  },
  "agentCard": {
    "public": {
      "mode": "managed",
      "content": {
        "name": "Weather Agent",
        "version": "1.0.0",
        "supportedInterfaces": [
          {
            "protocolBinding": "JSONRPC",
            "protocolVersion": "1.0",
            "url": "https://agents.example.com/weather/rpc"
          }
        ],
        "capabilities": {
          "streaming": true,
          "extendedAgentCard": true
        }
      }
    },
    "protected": {
      "mode": "managed",
      "content": {
        "name": "Weather Agent",
        "version": "1.0.0",
        "supportedInterfaces": [
          {
            "protocolBinding": "JSONRPC",
            "protocolVersion": "1.0",
            "url": "https://agents.example.com/weather/rpc"
          }
        ],
        "capabilities": {
          "streaming": true,
          "extendedAgentCard": true
        },
        "skills": [
          {
            "id": "get_forecast_history",
            "name": "Get forecast history"
          }
        ]
      }
    }
  }
}

```

A2A-specific agent configuration.

#### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|protocolVersion|string|true|none|A2A protocol version exposed by the gateway. This selects the agent's operation set and HTTP+JSON bindings, the Agent Card model its managed card is validated against, and the field-presence rules used to sign that card — an agent exposes exactly one version and the gateway performs no protocol-version conversion. Managed Agent Card interfaces must advertise this version; in passthrough mode the upstream is responsible for advertising it.|
|operationConfigs|[A2AOperationConfigs](#schemaa2aoperationconfigs)|true|none|Transport exposure and common or operation-specific configuration for A2A operations. These policies and transports do not apply to public Agent Card serving.|
|agentCard|[A2AAgentCard](#schemaa2aagentcard)|true|none|Public Agent Card configuration and optional protected Agent Card configuration for the authenticated A2A GetExtendedAgentCard operation.|

##### Enumerated Values

|Property|Value|
|---|---|
|protocolVersion|1.0|

## A2AOperationConfigs

<a id="schemaa2aoperationconfigs"></a>
<a id="schema_A2AOperationConfigs"></a>
<a id="tocSa2aoperationconfigs"></a>
<a id="tocsa2aoperationconfigs"></a>

```json
{
  "transports": [
    {
      "protocolBinding": "JSONRPC",
      "pathPrefix": "/rpc"
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
  "operations": [
    {
      "name": "SendMessage",
      "policies": [
        {
          "name": "cors",
          "version": "v1",
          "executionCondition": "request.metadata[authenticated] != true",
          "params": {}
        }
      ],
      "resilience": {
        "timeout": "15s",
        "idleTimeout": "0s"
      }
    }
  ]
}

```

Transport exposure and common or operation-specific configuration for A2A operations. These policies and transports do not apply to public Agent Card serving.

#### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|transports|[[A2ATransport](#schemaa2atransport)]|true|none|Ordered A2A protocol bindings and their gateway-facing path prefixes. This is runtime routing configuration, not Agent Card transformation or transport conversion.|
|policies|[[Policy](#schemapolicy)]|false|none|Ordered policies applied to every A2A operation, before operation-level policies. These policies do not apply to the public Agent Card discovery route.|
|operations|[[A2AOperationConfig](#schemaa2aoperationconfig)]|false|none|Optional per-operation configuration keyed by canonical A2A operation name. This array is not an allowlist: unlisted standard operations still receive spec.a2a.operationConfigs.policies. Public Agent Card discovery is configured separately.|

## A2ATransport

<a id="schemaa2atransport"></a>
<a id="schema_A2ATransport"></a>
<a id="tocSa2atransport"></a>
<a id="tocsa2atransport"></a>

```json
{
  "protocolBinding": "JSONRPC",
  "pathPrefix": "/rpc"
}

```

One A2A protocol binding exposed by the gateway and the path prefix at which that binding is served relative to spec.context.

#### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|protocolBinding|[A2AProtocolBinding](#schemaa2aprotocolbinding)|true|none|A2A protocol binding exposed at a transport's path prefix.|
|pathPrefix|string|false|none|Gateway-facing path prefix relative to spec.context. The root value / means that no additional path segment is inserted. For JSONRPC, this is the endpoint path; for HTTP+JSON, canonical operation paths are appended below it. This field does not select or replace the generic upstream.|

## A2AProtocolBinding

<a id="schemaa2aprotocolbinding"></a>
<a id="schema_A2AProtocolBinding"></a>
<a id="tocSa2aprotocolbinding"></a>
<a id="tocsa2aprotocolbinding"></a>

```json
"JSONRPC"

```

A2A protocol binding exposed at a transport's path prefix.

#### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|string|false|none|A2A protocol binding exposed at a transport's path prefix.|

##### Enumerated Values

|Property|Value|
|---|---|
|*anonymous*|JSONRPC|
|*anonymous*|HTTP+JSON|

## A2AOperationName

<a id="schemaa2aoperationname"></a>
<a id="schema_A2AOperationName"></a>
<a id="tocSa2aoperationname"></a>
<a id="tocsa2aoperationname"></a>

```json
"SendMessage"

```

Canonical A2A operation name. These names match the standard JSON-RPC and gRPC method names, but identify the binding-independent A2A operation. The effective set is closed and is the one defined by the agent's spec.a2a.protocolVersion; the values below are A2A 1.0's eleven operations, that being the only protocol version currently supported. A name outside the selected version's set is rejected at deploy time.

#### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|string|false|none|Canonical A2A operation name. These names match the standard JSON-RPC and gRPC method names, but identify the binding-independent A2A operation. The effective set is closed and is the one defined by the agent's spec.a2a.protocolVersion; the values below are A2A 1.0's eleven operations, that being the only protocol version currently supported. A name outside the selected version's set is rejected at deploy time.|

##### Enumerated Values

|Property|Value|
|---|---|
|*anonymous*|SendMessage|
|*anonymous*|SendStreamingMessage|
|*anonymous*|GetTask|
|*anonymous*|ListTasks|
|*anonymous*|CancelTask|
|*anonymous*|SubscribeToTask|
|*anonymous*|CreateTaskPushNotificationConfig|
|*anonymous*|GetTaskPushNotificationConfig|
|*anonymous*|ListTaskPushNotificationConfigs|
|*anonymous*|DeleteTaskPushNotificationConfig|
|*anonymous*|GetExtendedAgentCard|

## A2AOperationConfig

<a id="schemaa2aoperationconfig"></a>
<a id="schema_A2AOperationConfig"></a>
<a id="tocSa2aoperationconfig"></a>
<a id="tocsa2aoperationconfig"></a>

```json
{
  "name": "SendMessage",
  "policies": [
    {
      "name": "cors",
      "version": "v1",
      "executionCondition": "request.metadata[authenticated] != true",
      "params": {}
    }
  ],
  "resilience": {
    "timeout": "15s",
    "idleTimeout": "0s"
  }
}

```

Configuration for one standard A2A 1.0 operation, identified by its canonical operation name. The gateway maps each exposed transport to this name before choosing the policy chain.

#### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|name|[A2AOperationName](#schemaa2aoperationname)|true|none|Canonical A2A operation name. These names match the standard JSON-RPC and gRPC method names, but identify the binding-independent A2A operation. The effective set is closed and is the one defined by the agent's spec.a2a.protocolVersion; the values below are A2A 1.0's eleven operations, that being the only protocol version currently supported. A name outside the selected version's set is rejected at deploy time.|
|policies|[[Policy](#schemapolicy)]|false|none|Ordered policies applied after spec.a2a.operationConfigs.policies when this operation is selected.|
|resilience|[Resilience](#schemaresilience)|false|none|Backend/route timeout configuration. Maps to Envoy RouteAction timeouts. Can be set at the API level (applies to all routes) and/or the operation level (applies to that operation's route). When set at both levels, the operation-level value takes precedence. When unset, the gateway's global route timeout defaults apply.|

## A2AAgentCard

<a id="schemaa2aagentcard"></a>
<a id="schema_A2AAgentCard"></a>
<a id="tocSa2aagentcard"></a>
<a id="tocsa2aagentcard"></a>

```json
{
  "public": {
    "mode": "managed",
    "content": {
      "name": "Weather Agent",
      "version": "1.0.0",
      "supportedInterfaces": [
        {
          "protocolBinding": "JSONRPC",
          "protocolVersion": "1.0",
          "url": "https://agents.example.com/weather/rpc"
        }
      ],
      "capabilities": {
        "streaming": true,
        "extendedAgentCard": true
      }
    }
  },
  "protected": {
    "mode": "managed",
    "content": {
      "name": "Weather Agent",
      "version": "1.0.0",
      "supportedInterfaces": [
        {
          "protocolBinding": "JSONRPC",
          "protocolVersion": "1.0",
          "url": "https://agents.example.com/weather/rpc"
        }
      ],
      "capabilities": {
        "streaming": true,
        "extendedAgentCard": true
      },
      "skills": [
        {
          "id": "get_forecast_history",
          "name": "Get forecast history"
        }
      ]
    }
  }
}

```

Public Agent Card configuration and optional protected Agent Card configuration for the authenticated A2A GetExtendedAgentCard operation.

#### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|public|[A2APublicAgentCard](#schemaa2apublicagentcard)|true|none|Public Agent Card serving. `mode` selects whether the card is proxied unchanged from the upstream (`passthrough`) or validated, stored, and served by the gateway (`managed`). Mode-specific rules are enforced at deploy time, not by this schema: `managed` requires `content`; `passthrough` accepts neither `content` nor `signing`, because the gateway does not parse, transform, or sign a proxied card.|
|protected|[A2AProtectedAgentCard](#schemaa2aprotectedagentcard)|false|none|Authenticated extended Agent Card. It is served through the canonical GetExtendedAgentCard operation and uses that operation's policy chain — the policies in spec.a2a.operationConfigs, then any matching entry in spec.a2a.operationConfigs.operations. Public Agent Card policies never run for it, and it has no custom path or local policy list, because it is an A2A operation rather than a document at a location.<br>This block is optional, and omitting it is not the same as configuring `passthrough`. When it is absent, GetExtendedAgentCard is exposed and proxied to the upstream with no gateway-added authentication guard, which is the behaviour Agents written before protected Agent Cards shipped already have.<br>When it is present, the gateway requires the request to have been authenticated by a policy in the Agent's own chain before the card is returned or proxied, and answers 401 otherwise. That applies in both modes and is not configurable: an Agent that declares a protected card but attaches no authentication policy therefore fails closed instead of publishing its extended card. Where authentication sits among the configured policies is the Agent author's choice.<br>Mode-specific rules are enforced at deploy time, not by this schema: `managed` requires `content`; `passthrough` accepts neither `content` nor `signing`, because the gateway does not parse, transform, or sign a proxied card. When the public Agent Card is `managed`, it must additionally declare `capabilities.extendedAgentCard: true`, since that is what tells a client the operation exists at all.|

## A2APublicAgentCard

<a id="schemaa2apublicagentcard"></a>
<a id="schema_A2APublicAgentCard"></a>
<a id="tocSa2apublicagentcard"></a>
<a id="tocsa2apublicagentcard"></a>

```json
{
  "mode": "managed",
  "path": "/.well-known/agent-card.json",
  "policies": [
    {
      "name": "cors",
      "version": "v1",
      "executionCondition": "request.metadata[authenticated] != true",
      "params": {}
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
  },
  "signing": {
    "enabled": false
  }
}

```

Public Agent Card serving. `mode` selects whether the card is proxied unchanged from the upstream (`passthrough`) or validated, stored, and served by the gateway (`managed`). Mode-specific rules are enforced at deploy time, not by this schema: `managed` requires `content`; `passthrough` accepts neither `content` nor `signing`, because the gateway does not parse, transform, or sign a proxied card.

#### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|mode|string|true|none|How the public Agent Card is produced.|
|path|[A2AAgentCardPath](#schemaa2aagentcardpath)|false|none|Exact gateway-facing Agent Card path relative to spec.context. When omitted, the gateway uses /.well-known/agent-card.json. A custom path replaces that default route rather than creating an additional alias. In passthrough mode this does not change the upstream discovery path.|
|policies|[[Policy](#schemapolicy)]|false|none|Ordered policies applied only to public Agent Card serving.|
|content|[A2AAgentCardDocument](#schemaa2aagentcarddocument)|false|none|Complete A2A 1.0 Agent Card represented as a structured JSON object. JSON can be embedded directly because JSON object syntax is valid YAML. The controller additionally validates this object against the complete A2A Agent Card model for spec.a2a.protocolVersion, taken from the vendored A2A protocol definition (specification/a2a.proto). The document is stored and served as supplied — the gateway never rewrites it — so extension fields are preserved.|
|signing|[A2ACardSigning](#schemaa2acardsigning)|false|none|Optional signing configuration for a managed Agent Card. Passthrough cards cannot configure gateway signing. Agent authors only enable or disable signing: the active key, its key identifier, and the JWS algorithm are selected from administrator-owned gateway system configuration at signing time, so rotating the key — including to a key using a different algorithm — requires no edit to any Agent. A card is re-signed when its Agent is next deployed, not when the key rotates; until then it keeps verifying against the retired key, which stays published while any stored card references it.|

##### Enumerated Values

|Property|Value|
|---|---|
|mode|managed|
|mode|passthrough|

## A2AProtectedAgentCard

<a id="schemaa2aprotectedagentcard"></a>
<a id="schema_A2AProtectedAgentCard"></a>
<a id="tocSa2aprotectedagentcard"></a>
<a id="tocsa2aprotectedagentcard"></a>

```json
{
  "mode": "passthrough",
  "content": {
    "name": "Weather Agent",
    "description": "Provides weather information",
    "version": "1.0.0",
    "supportedInterfaces": [
      {
        "protocolBinding": "JSONRPC",
        "protocolVersion": "1.0",
        "url": "https://agents.example.com/weather/rpc"
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
  },
  "signing": {
    "enabled": false
  }
}

```

Authenticated extended Agent Card. It is served through the canonical GetExtendedAgentCard operation and uses that operation's policy chain — the policies in spec.a2a.operationConfigs, then any matching entry in spec.a2a.operationConfigs.operations. Public Agent Card policies never run for it, and it has no custom path or local policy list, because it is an A2A operation rather than a document at a location.
This block is optional, and omitting it is not the same as configuring `passthrough`. When it is absent, GetExtendedAgentCard is exposed and proxied to the upstream with no gateway-added authentication guard, which is the behaviour Agents written before protected Agent Cards shipped already have.
When it is present, the gateway requires the request to have been authenticated by a policy in the Agent's own chain before the card is returned or proxied, and answers 401 otherwise. That applies in both modes and is not configurable: an Agent that declares a protected card but attaches no authentication policy therefore fails closed instead of publishing its extended card. Where authentication sits among the configured policies is the Agent author's choice.
Mode-specific rules are enforced at deploy time, not by this schema: `managed` requires `content`; `passthrough` accepts neither `content` nor `signing`, because the gateway does not parse, transform, or sign a proxied card. When the public Agent Card is `managed`, it must additionally declare `capabilities.extendedAgentCard: true`, since that is what tells a client the operation exists at all.

#### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|mode|string|true|none|How the protected Agent Card is produced. `managed` serves the supplied `content` from the gateway, and the request never reaches the upstream. `passthrough` forwards the authenticated request and proxies the upstream's own response unchanged.|
|content|[A2AAgentCardDocument](#schemaa2aagentcarddocument)|false|none|Complete A2A 1.0 Agent Card represented as a structured JSON object. JSON can be embedded directly because JSON object syntax is valid YAML. The controller additionally validates this object against the complete A2A Agent Card model for spec.a2a.protocolVersion, taken from the vendored A2A protocol definition (specification/a2a.proto). The document is stored and served as supplied — the gateway never rewrites it — so extension fields are preserved.|
|signing|[A2ACardSigning](#schemaa2acardsigning)|false|none|Optional signing configuration for a managed Agent Card. Passthrough cards cannot configure gateway signing. Agent authors only enable or disable signing: the active key, its key identifier, and the JWS algorithm are selected from administrator-owned gateway system configuration at signing time, so rotating the key — including to a key using a different algorithm — requires no edit to any Agent. A card is re-signed when its Agent is next deployed, not when the key rotates; until then it keeps verifying against the retired key, which stays published while any stored card references it.|

##### Enumerated Values

|Property|Value|
|---|---|
|mode|managed|
|mode|passthrough|

## A2AAgentCardPath

<a id="schemaa2aagentcardpath"></a>
<a id="schema_A2AAgentCardPath"></a>
<a id="tocSa2aagentcardpath"></a>
<a id="tocsa2aagentcardpath"></a>

```json
"/.well-known/agent-card.json"

```

Exact gateway-facing Agent Card path relative to spec.context. When omitted, the gateway uses /.well-known/agent-card.json. A custom path replaces that default route rather than creating an additional alias. In passthrough mode this does not change the upstream discovery path.

#### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|string|false|none|Exact gateway-facing Agent Card path relative to spec.context. When omitted, the gateway uses /.well-known/agent-card.json. A custom path replaces that default route rather than creating an additional alias. In passthrough mode this does not change the upstream discovery path.|

## A2AAgentCardDocument

<a id="schemaa2aagentcarddocument"></a>
<a id="schema_A2AAgentCardDocument"></a>
<a id="tocSa2aagentcarddocument"></a>
<a id="tocsa2aagentcarddocument"></a>

```json
{
  "name": "Weather Agent",
  "description": "Provides weather information",
  "version": "1.0.0",
  "supportedInterfaces": [
    {
      "protocolBinding": "JSONRPC",
      "protocolVersion": "1.0",
      "url": "https://agents.example.com/weather/rpc"
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

```

Complete A2A 1.0 Agent Card represented as a structured JSON object. JSON can be embedded directly because JSON object syntax is valid YAML. The controller additionally validates this object against the complete A2A Agent Card model for spec.a2a.protocolVersion, taken from the vendored A2A protocol definition (specification/a2a.proto). The document is stored and served as supplied — the gateway never rewrites it — so extension fields are preserved.

#### Properties

*None*

## A2ACardSigning

<a id="schemaa2acardsigning"></a>
<a id="schema_A2ACardSigning"></a>
<a id="tocSa2acardsigning"></a>
<a id="tocsa2acardsigning"></a>

```json
{
  "enabled": false
}

```

Optional signing configuration for a managed Agent Card. Passthrough cards cannot configure gateway signing. Agent authors only enable or disable signing: the active key, its key identifier, and the JWS algorithm are selected from administrator-owned gateway system configuration at signing time, so rotating the key — including to a key using a different algorithm — requires no edit to any Agent. A card is re-signed when its Agent is next deployed, not when the key rotates; until then it keeps verifying against the retired key, which stays published while any stored card references it.

#### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|enabled|boolean|true|none|Whether the gateway signs the managed card it serves, using the active Agent Card signing key configured by the gateway administrator.|

## ErrorResponse

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

#### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|status|string|true|none|none|
|message|string|true|none|High-level error description|
|errors|[[ValidationError](#schemavalidationerror)]|false|none|Detailed validation errors|

## ValidationError

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

#### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|field|string|false|none|Field that failed validation|
|message|string|false|none|Human-readable error message|

## LLMProviderTemplateRequest

<a id="schemallmprovidertemplaterequest"></a>
<a id="schema_LLMProviderTemplateRequest"></a>
<a id="tocSllmprovidertemplaterequest"></a>
<a id="tocsllmprovidertemplaterequest"></a>

```json
{
  "apiVersion": "gateway.api-platform.wso2.com/v1",
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

#### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|apiVersion|string|true|none|Template specification version|
|kind|string|true|none|Template kind|
|metadata|[Metadata](#schemametadata)|true|none|none|
|spec|[LLMProviderTemplateData](#schemallmprovidertemplatedata)|true|none|none|

##### Enumerated Values

|Property|Value|
|---|---|
|apiVersion|gateway.api-platform.wso2.com/v1|
|kind|LlmProviderTemplate|

## LLMProviderTemplate

<a id="schemallmprovidertemplate"></a>
<a id="schema_LLMProviderTemplate"></a>
<a id="tocSllmprovidertemplate"></a>
<a id="tocsllmprovidertemplate"></a>

```json
{
  "apiVersion": "gateway.api-platform.wso2.com/v1",
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

#### Properties

allOf

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|[LLMProviderTemplateRequest](#schemallmprovidertemplaterequest)|false|none|none|

and

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|object|false|none|none|
|» status|[ResourceStatus](#schemaresourcestatus)|false|read-only|Server-managed lifecycle fields. Populated on responses.|

## LLMProviderTemplateData

<a id="schemallmprovidertemplatedata"></a>
<a id="schema_LLMProviderTemplateData"></a>
<a id="tocSllmprovidertemplatedata"></a>
<a id="tocsllmprovidertemplatedata"></a>

```json
{
  "displayName": "OpenAI",
  "groupId": "openai",
  "managedBy": "wso2",
  "version": "v1.0",
  "promptTokens": {
    "location": "payload",
    "identifier": "$.usage.inputTokens",
    "fallbackIdentifiers": [
      "$.usage.input_tokens",
      "$.usage.prompt_tokens"
    ],
    "valueMap": {
      "ON_DEMAND_PRIORITY": "priority",
      "ON_DEMAND_FLEX": "flex"
    }
  },
  "completionTokens": {
    "location": "payload",
    "identifier": "$.usage.inputTokens",
    "fallbackIdentifiers": [
      "$.usage.input_tokens",
      "$.usage.prompt_tokens"
    ],
    "valueMap": {
      "ON_DEMAND_PRIORITY": "priority",
      "ON_DEMAND_FLEX": "flex"
    }
  },
  "totalTokens": {
    "location": "payload",
    "identifier": "$.usage.inputTokens",
    "fallbackIdentifiers": [
      "$.usage.input_tokens",
      "$.usage.prompt_tokens"
    ],
    "valueMap": {
      "ON_DEMAND_PRIORITY": "priority",
      "ON_DEMAND_FLEX": "flex"
    }
  },
  "remainingTokens": {
    "location": "payload",
    "identifier": "$.usage.inputTokens",
    "fallbackIdentifiers": [
      "$.usage.input_tokens",
      "$.usage.prompt_tokens"
    ],
    "valueMap": {
      "ON_DEMAND_PRIORITY": "priority",
      "ON_DEMAND_FLEX": "flex"
    }
  },
  "requestModel": {
    "location": "payload",
    "identifier": "$.usage.inputTokens",
    "fallbackIdentifiers": [
      "$.usage.input_tokens",
      "$.usage.prompt_tokens"
    ],
    "valueMap": {
      "ON_DEMAND_PRIORITY": "priority",
      "ON_DEMAND_FLEX": "flex"
    }
  },
  "responseModel": {
    "location": "payload",
    "identifier": "$.usage.inputTokens",
    "fallbackIdentifiers": [
      "$.usage.input_tokens",
      "$.usage.prompt_tokens"
    ],
    "valueMap": {
      "ON_DEMAND_PRIORITY": "priority",
      "ON_DEMAND_FLEX": "flex"
    }
  },
  "cachedTokens": {
    "location": "payload",
    "identifier": "$.usage.inputTokens",
    "fallbackIdentifiers": [
      "$.usage.input_tokens",
      "$.usage.prompt_tokens"
    ],
    "valueMap": {
      "ON_DEMAND_PRIORITY": "priority",
      "ON_DEMAND_FLEX": "flex"
    }
  },
  "cacheWriteTokens": {
    "location": "payload",
    "identifier": "$.usage.inputTokens",
    "fallbackIdentifiers": [
      "$.usage.input_tokens",
      "$.usage.prompt_tokens"
    ],
    "valueMap": {
      "ON_DEMAND_PRIORITY": "priority",
      "ON_DEMAND_FLEX": "flex"
    }
  },
  "cacheWrite1hTokens": {
    "location": "payload",
    "identifier": "$.usage.inputTokens",
    "fallbackIdentifiers": [
      "$.usage.input_tokens",
      "$.usage.prompt_tokens"
    ],
    "valueMap": {
      "ON_DEMAND_PRIORITY": "priority",
      "ON_DEMAND_FLEX": "flex"
    }
  },
  "reasoningTokens": {
    "location": "payload",
    "identifier": "$.usage.inputTokens",
    "fallbackIdentifiers": [
      "$.usage.input_tokens",
      "$.usage.prompt_tokens"
    ],
    "valueMap": {
      "ON_DEMAND_PRIORITY": "priority",
      "ON_DEMAND_FLEX": "flex"
    }
  },
  "audioInputTokens": {
    "location": "payload",
    "identifier": "$.usage.inputTokens",
    "fallbackIdentifiers": [
      "$.usage.input_tokens",
      "$.usage.prompt_tokens"
    ],
    "valueMap": {
      "ON_DEMAND_PRIORITY": "priority",
      "ON_DEMAND_FLEX": "flex"
    }
  },
  "audioOutputTokens": {
    "location": "payload",
    "identifier": "$.usage.inputTokens",
    "fallbackIdentifiers": [
      "$.usage.input_tokens",
      "$.usage.prompt_tokens"
    ],
    "valueMap": {
      "ON_DEMAND_PRIORITY": "priority",
      "ON_DEMAND_FLEX": "flex"
    }
  },
  "serviceTier": {
    "location": "payload",
    "identifier": "$.usage.inputTokens",
    "fallbackIdentifiers": [
      "$.usage.input_tokens",
      "$.usage.prompt_tokens"
    ],
    "valueMap": {
      "ON_DEMAND_PRIORITY": "priority",
      "ON_DEMAND_FLEX": "flex"
    }
  },
  "providerFields": {
    "property1": {
      "location": "payload",
      "identifier": "$.usage.inputTokens",
      "fallbackIdentifiers": [
        "$.usage.input_tokens",
        "$.usage.prompt_tokens"
      ],
      "valueMap": {
        "ON_DEMAND_PRIORITY": "priority",
        "ON_DEMAND_FLEX": "flex"
      }
    },
    "property2": {
      "location": "payload",
      "identifier": "$.usage.inputTokens",
      "fallbackIdentifiers": [
        "$.usage.input_tokens",
        "$.usage.prompt_tokens"
      ],
      "valueMap": {
        "ON_DEMAND_PRIORITY": "priority",
        "ON_DEMAND_FLEX": "flex"
      }
    }
  },
  "cacheAccounting": "inclusive",
  "resourceMappings": {
    "resources": [
      {
        "resource": "/responses",
        "promptTokens": {
          "location": "payload",
          "identifier": "$.usage.inputTokens",
          "fallbackIdentifiers": [
            "$.usage.input_tokens",
            "$.usage.prompt_tokens"
          ],
          "valueMap": {
            "ON_DEMAND_PRIORITY": "priority",
            "ON_DEMAND_FLEX": "flex"
          }
        },
        "completionTokens": {
          "location": "payload",
          "identifier": "$.usage.inputTokens",
          "fallbackIdentifiers": [
            "$.usage.input_tokens",
            "$.usage.prompt_tokens"
          ],
          "valueMap": {
            "ON_DEMAND_PRIORITY": "priority",
            "ON_DEMAND_FLEX": "flex"
          }
        },
        "totalTokens": {
          "location": "payload",
          "identifier": "$.usage.inputTokens",
          "fallbackIdentifiers": [
            "$.usage.input_tokens",
            "$.usage.prompt_tokens"
          ],
          "valueMap": {
            "ON_DEMAND_PRIORITY": "priority",
            "ON_DEMAND_FLEX": "flex"
          }
        },
        "remainingTokens": {
          "location": "payload",
          "identifier": "$.usage.inputTokens",
          "fallbackIdentifiers": [
            "$.usage.input_tokens",
            "$.usage.prompt_tokens"
          ],
          "valueMap": {
            "ON_DEMAND_PRIORITY": "priority",
            "ON_DEMAND_FLEX": "flex"
          }
        },
        "requestModel": {
          "location": "payload",
          "identifier": "$.usage.inputTokens",
          "fallbackIdentifiers": [
            "$.usage.input_tokens",
            "$.usage.prompt_tokens"
          ],
          "valueMap": {
            "ON_DEMAND_PRIORITY": "priority",
            "ON_DEMAND_FLEX": "flex"
          }
        },
        "responseModel": {
          "location": "payload",
          "identifier": "$.usage.inputTokens",
          "fallbackIdentifiers": [
            "$.usage.input_tokens",
            "$.usage.prompt_tokens"
          ],
          "valueMap": {
            "ON_DEMAND_PRIORITY": "priority",
            "ON_DEMAND_FLEX": "flex"
          }
        },
        "cachedTokens": {
          "location": "payload",
          "identifier": "$.usage.inputTokens",
          "fallbackIdentifiers": [
            "$.usage.input_tokens",
            "$.usage.prompt_tokens"
          ],
          "valueMap": {
            "ON_DEMAND_PRIORITY": "priority",
            "ON_DEMAND_FLEX": "flex"
          }
        },
        "cacheWriteTokens": {
          "location": "payload",
          "identifier": "$.usage.inputTokens",
          "fallbackIdentifiers": [
            "$.usage.input_tokens",
            "$.usage.prompt_tokens"
          ],
          "valueMap": {
            "ON_DEMAND_PRIORITY": "priority",
            "ON_DEMAND_FLEX": "flex"
          }
        },
        "cacheWrite1hTokens": {
          "location": "payload",
          "identifier": "$.usage.inputTokens",
          "fallbackIdentifiers": [
            "$.usage.input_tokens",
            "$.usage.prompt_tokens"
          ],
          "valueMap": {
            "ON_DEMAND_PRIORITY": "priority",
            "ON_DEMAND_FLEX": "flex"
          }
        },
        "reasoningTokens": {
          "location": "payload",
          "identifier": "$.usage.inputTokens",
          "fallbackIdentifiers": [
            "$.usage.input_tokens",
            "$.usage.prompt_tokens"
          ],
          "valueMap": {
            "ON_DEMAND_PRIORITY": "priority",
            "ON_DEMAND_FLEX": "flex"
          }
        },
        "audioInputTokens": {
          "location": "payload",
          "identifier": "$.usage.inputTokens",
          "fallbackIdentifiers": [
            "$.usage.input_tokens",
            "$.usage.prompt_tokens"
          ],
          "valueMap": {
            "ON_DEMAND_PRIORITY": "priority",
            "ON_DEMAND_FLEX": "flex"
          }
        },
        "audioOutputTokens": {
          "location": "payload",
          "identifier": "$.usage.inputTokens",
          "fallbackIdentifiers": [
            "$.usage.input_tokens",
            "$.usage.prompt_tokens"
          ],
          "valueMap": {
            "ON_DEMAND_PRIORITY": "priority",
            "ON_DEMAND_FLEX": "flex"
          }
        },
        "serviceTier": {
          "location": "payload",
          "identifier": "$.usage.inputTokens",
          "fallbackIdentifiers": [
            "$.usage.input_tokens",
            "$.usage.prompt_tokens"
          ],
          "valueMap": {
            "ON_DEMAND_PRIORITY": "priority",
            "ON_DEMAND_FLEX": "flex"
          }
        },
        "cacheAccounting": "inclusive"
      }
    ]
  }
}

```

#### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|displayName|string|true|none|Human-readable LLM Template name|
|groupId|string|false|none|Stable family-grouping identifier shared by every version of this<br>template. Multiple versions with the same groupId but different<br>handles are surfaced as versions of one template. Defaults to<br>metadata.name when omitted.|
|managedBy|string|false|none|Origin of the template. Built-in templates use 'wso2'; custom<br>templates default to 'customer' and may be set to any value.|
|version|string|false|none|Template content version (e.g. v1.0). Multiple versions of the same<br>groupId can coexist; defaults to v1.0 when omitted.|
|promptTokens|[ExtractionIdentifier](#schemaextractionidentifier)|false|none|none|
|completionTokens|[ExtractionIdentifier](#schemaextractionidentifier)|false|none|none|
|totalTokens|[ExtractionIdentifier](#schemaextractionidentifier)|false|none|none|
|remainingTokens|[ExtractionIdentifier](#schemaextractionidentifier)|false|none|none|
|requestModel|[ExtractionIdentifier](#schemaextractionidentifier)|false|none|none|
|responseModel|[ExtractionIdentifier](#schemaextractionidentifier)|false|none|none|
|cachedTokens|[ExtractionIdentifier](#schemaextractionidentifier)|false|none|none|
|cacheWriteTokens|[ExtractionIdentifier](#schemaextractionidentifier)|false|none|none|
|cacheWrite1hTokens|[ExtractionIdentifier](#schemaextractionidentifier)|false|none|none|
|reasoningTokens|[ExtractionIdentifier](#schemaextractionidentifier)|false|none|none|
|audioInputTokens|[ExtractionIdentifier](#schemaextractionidentifier)|false|none|none|
|audioOutputTokens|[ExtractionIdentifier](#schemaextractionidentifier)|false|none|none|
|serviceTier|[ExtractionIdentifier](#schemaextractionidentifier)|false|none|none|
|providerFields|object|false|none|Named locations a provider-specific calculator needs but the closed<br>vocabulary does not cover. The value at each location is handed to<br>the calculator as-is; only its position is declared here.|
|» **additionalProperties**|[ExtractionIdentifier](#schemaextractionidentifier)|false|none|none|
|cacheAccounting|string|false|none|Whether the cached token count reported by the provider is already<br>part of the input token total, or additional to it. One of<br>'inclusive' or 'additive'. Defaults to inclusive when omitted.|
|resourceMappings|[LLMProviderTemplateResourceMappings](#schemallmprovidertemplateresourcemappings)|false|none|none|

## LLMProviderTemplateResourceMappings

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
        "identifier": "$.usage.inputTokens",
        "fallbackIdentifiers": [
          "$.usage.input_tokens",
          "$.usage.prompt_tokens"
        ],
        "valueMap": {
          "ON_DEMAND_PRIORITY": "priority",
          "ON_DEMAND_FLEX": "flex"
        }
      },
      "completionTokens": {
        "location": "payload",
        "identifier": "$.usage.inputTokens",
        "fallbackIdentifiers": [
          "$.usage.input_tokens",
          "$.usage.prompt_tokens"
        ],
        "valueMap": {
          "ON_DEMAND_PRIORITY": "priority",
          "ON_DEMAND_FLEX": "flex"
        }
      },
      "totalTokens": {
        "location": "payload",
        "identifier": "$.usage.inputTokens",
        "fallbackIdentifiers": [
          "$.usage.input_tokens",
          "$.usage.prompt_tokens"
        ],
        "valueMap": {
          "ON_DEMAND_PRIORITY": "priority",
          "ON_DEMAND_FLEX": "flex"
        }
      },
      "remainingTokens": {
        "location": "payload",
        "identifier": "$.usage.inputTokens",
        "fallbackIdentifiers": [
          "$.usage.input_tokens",
          "$.usage.prompt_tokens"
        ],
        "valueMap": {
          "ON_DEMAND_PRIORITY": "priority",
          "ON_DEMAND_FLEX": "flex"
        }
      },
      "requestModel": {
        "location": "payload",
        "identifier": "$.usage.inputTokens",
        "fallbackIdentifiers": [
          "$.usage.input_tokens",
          "$.usage.prompt_tokens"
        ],
        "valueMap": {
          "ON_DEMAND_PRIORITY": "priority",
          "ON_DEMAND_FLEX": "flex"
        }
      },
      "responseModel": {
        "location": "payload",
        "identifier": "$.usage.inputTokens",
        "fallbackIdentifiers": [
          "$.usage.input_tokens",
          "$.usage.prompt_tokens"
        ],
        "valueMap": {
          "ON_DEMAND_PRIORITY": "priority",
          "ON_DEMAND_FLEX": "flex"
        }
      },
      "cachedTokens": {
        "location": "payload",
        "identifier": "$.usage.inputTokens",
        "fallbackIdentifiers": [
          "$.usage.input_tokens",
          "$.usage.prompt_tokens"
        ],
        "valueMap": {
          "ON_DEMAND_PRIORITY": "priority",
          "ON_DEMAND_FLEX": "flex"
        }
      },
      "cacheWriteTokens": {
        "location": "payload",
        "identifier": "$.usage.inputTokens",
        "fallbackIdentifiers": [
          "$.usage.input_tokens",
          "$.usage.prompt_tokens"
        ],
        "valueMap": {
          "ON_DEMAND_PRIORITY": "priority",
          "ON_DEMAND_FLEX": "flex"
        }
      },
      "cacheWrite1hTokens": {
        "location": "payload",
        "identifier": "$.usage.inputTokens",
        "fallbackIdentifiers": [
          "$.usage.input_tokens",
          "$.usage.prompt_tokens"
        ],
        "valueMap": {
          "ON_DEMAND_PRIORITY": "priority",
          "ON_DEMAND_FLEX": "flex"
        }
      },
      "reasoningTokens": {
        "location": "payload",
        "identifier": "$.usage.inputTokens",
        "fallbackIdentifiers": [
          "$.usage.input_tokens",
          "$.usage.prompt_tokens"
        ],
        "valueMap": {
          "ON_DEMAND_PRIORITY": "priority",
          "ON_DEMAND_FLEX": "flex"
        }
      },
      "audioInputTokens": {
        "location": "payload",
        "identifier": "$.usage.inputTokens",
        "fallbackIdentifiers": [
          "$.usage.input_tokens",
          "$.usage.prompt_tokens"
        ],
        "valueMap": {
          "ON_DEMAND_PRIORITY": "priority",
          "ON_DEMAND_FLEX": "flex"
        }
      },
      "audioOutputTokens": {
        "location": "payload",
        "identifier": "$.usage.inputTokens",
        "fallbackIdentifiers": [
          "$.usage.input_tokens",
          "$.usage.prompt_tokens"
        ],
        "valueMap": {
          "ON_DEMAND_PRIORITY": "priority",
          "ON_DEMAND_FLEX": "flex"
        }
      },
      "serviceTier": {
        "location": "payload",
        "identifier": "$.usage.inputTokens",
        "fallbackIdentifiers": [
          "$.usage.input_tokens",
          "$.usage.prompt_tokens"
        ],
        "valueMap": {
          "ON_DEMAND_PRIORITY": "priority",
          "ON_DEMAND_FLEX": "flex"
        }
      },
      "cacheAccounting": "inclusive"
    }
  ]
}

```

#### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|resources|[[LLMProviderTemplateResourceMapping](#schemallmprovidertemplateresourcemapping)]|false|none|none|

## LLMProviderTemplateResourceMapping

<a id="schemallmprovidertemplateresourcemapping"></a>
<a id="schema_LLMProviderTemplateResourceMapping"></a>
<a id="tocSllmprovidertemplateresourcemapping"></a>
<a id="tocsllmprovidertemplateresourcemapping"></a>

```json
{
  "resource": "/responses",
  "promptTokens": {
    "location": "payload",
    "identifier": "$.usage.inputTokens",
    "fallbackIdentifiers": [
      "$.usage.input_tokens",
      "$.usage.prompt_tokens"
    ],
    "valueMap": {
      "ON_DEMAND_PRIORITY": "priority",
      "ON_DEMAND_FLEX": "flex"
    }
  },
  "completionTokens": {
    "location": "payload",
    "identifier": "$.usage.inputTokens",
    "fallbackIdentifiers": [
      "$.usage.input_tokens",
      "$.usage.prompt_tokens"
    ],
    "valueMap": {
      "ON_DEMAND_PRIORITY": "priority",
      "ON_DEMAND_FLEX": "flex"
    }
  },
  "totalTokens": {
    "location": "payload",
    "identifier": "$.usage.inputTokens",
    "fallbackIdentifiers": [
      "$.usage.input_tokens",
      "$.usage.prompt_tokens"
    ],
    "valueMap": {
      "ON_DEMAND_PRIORITY": "priority",
      "ON_DEMAND_FLEX": "flex"
    }
  },
  "remainingTokens": {
    "location": "payload",
    "identifier": "$.usage.inputTokens",
    "fallbackIdentifiers": [
      "$.usage.input_tokens",
      "$.usage.prompt_tokens"
    ],
    "valueMap": {
      "ON_DEMAND_PRIORITY": "priority",
      "ON_DEMAND_FLEX": "flex"
    }
  },
  "requestModel": {
    "location": "payload",
    "identifier": "$.usage.inputTokens",
    "fallbackIdentifiers": [
      "$.usage.input_tokens",
      "$.usage.prompt_tokens"
    ],
    "valueMap": {
      "ON_DEMAND_PRIORITY": "priority",
      "ON_DEMAND_FLEX": "flex"
    }
  },
  "responseModel": {
    "location": "payload",
    "identifier": "$.usage.inputTokens",
    "fallbackIdentifiers": [
      "$.usage.input_tokens",
      "$.usage.prompt_tokens"
    ],
    "valueMap": {
      "ON_DEMAND_PRIORITY": "priority",
      "ON_DEMAND_FLEX": "flex"
    }
  },
  "cachedTokens": {
    "location": "payload",
    "identifier": "$.usage.inputTokens",
    "fallbackIdentifiers": [
      "$.usage.input_tokens",
      "$.usage.prompt_tokens"
    ],
    "valueMap": {
      "ON_DEMAND_PRIORITY": "priority",
      "ON_DEMAND_FLEX": "flex"
    }
  },
  "cacheWriteTokens": {
    "location": "payload",
    "identifier": "$.usage.inputTokens",
    "fallbackIdentifiers": [
      "$.usage.input_tokens",
      "$.usage.prompt_tokens"
    ],
    "valueMap": {
      "ON_DEMAND_PRIORITY": "priority",
      "ON_DEMAND_FLEX": "flex"
    }
  },
  "cacheWrite1hTokens": {
    "location": "payload",
    "identifier": "$.usage.inputTokens",
    "fallbackIdentifiers": [
      "$.usage.input_tokens",
      "$.usage.prompt_tokens"
    ],
    "valueMap": {
      "ON_DEMAND_PRIORITY": "priority",
      "ON_DEMAND_FLEX": "flex"
    }
  },
  "reasoningTokens": {
    "location": "payload",
    "identifier": "$.usage.inputTokens",
    "fallbackIdentifiers": [
      "$.usage.input_tokens",
      "$.usage.prompt_tokens"
    ],
    "valueMap": {
      "ON_DEMAND_PRIORITY": "priority",
      "ON_DEMAND_FLEX": "flex"
    }
  },
  "audioInputTokens": {
    "location": "payload",
    "identifier": "$.usage.inputTokens",
    "fallbackIdentifiers": [
      "$.usage.input_tokens",
      "$.usage.prompt_tokens"
    ],
    "valueMap": {
      "ON_DEMAND_PRIORITY": "priority",
      "ON_DEMAND_FLEX": "flex"
    }
  },
  "audioOutputTokens": {
    "location": "payload",
    "identifier": "$.usage.inputTokens",
    "fallbackIdentifiers": [
      "$.usage.input_tokens",
      "$.usage.prompt_tokens"
    ],
    "valueMap": {
      "ON_DEMAND_PRIORITY": "priority",
      "ON_DEMAND_FLEX": "flex"
    }
  },
  "serviceTier": {
    "location": "payload",
    "identifier": "$.usage.inputTokens",
    "fallbackIdentifiers": [
      "$.usage.input_tokens",
      "$.usage.prompt_tokens"
    ],
    "valueMap": {
      "ON_DEMAND_PRIORITY": "priority",
      "ON_DEMAND_FLEX": "flex"
    }
  },
  "cacheAccounting": "inclusive"
}

```

#### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|resource|string|true|none|Resource path pattern for this mapping|
|promptTokens|[ExtractionIdentifier](#schemaextractionidentifier)|false|none|none|
|completionTokens|[ExtractionIdentifier](#schemaextractionidentifier)|false|none|none|
|totalTokens|[ExtractionIdentifier](#schemaextractionidentifier)|false|none|none|
|remainingTokens|[ExtractionIdentifier](#schemaextractionidentifier)|false|none|none|
|requestModel|[ExtractionIdentifier](#schemaextractionidentifier)|false|none|none|
|responseModel|[ExtractionIdentifier](#schemaextractionidentifier)|false|none|none|
|cachedTokens|[ExtractionIdentifier](#schemaextractionidentifier)|false|none|none|
|cacheWriteTokens|[ExtractionIdentifier](#schemaextractionidentifier)|false|none|none|
|cacheWrite1hTokens|[ExtractionIdentifier](#schemaextractionidentifier)|false|none|none|
|reasoningTokens|[ExtractionIdentifier](#schemaextractionidentifier)|false|none|none|
|audioInputTokens|[ExtractionIdentifier](#schemaextractionidentifier)|false|none|none|
|audioOutputTokens|[ExtractionIdentifier](#schemaextractionidentifier)|false|none|none|
|serviceTier|[ExtractionIdentifier](#schemaextractionidentifier)|false|none|none|
|cacheAccounting|string|false|none|Whether the cached token count reported by the provider is already<br>part of the input token total, or additional to it. One of<br>'inclusive' or 'additive'. Overrides the template-level setting for<br>this resource; when omitted, the template-level value is inherited.|

## ExtractionIdentifier

<a id="schemaextractionidentifier"></a>
<a id="schema_ExtractionIdentifier"></a>
<a id="tocSextractionidentifier"></a>
<a id="tocsextractionidentifier"></a>

```json
{
  "location": "payload",
  "identifier": "$.usage.inputTokens",
  "fallbackIdentifiers": [
    "$.usage.input_tokens",
    "$.usage.prompt_tokens"
  ],
  "valueMap": {
    "ON_DEMAND_PRIORITY": "priority",
    "ON_DEMAND_FLEX": "flex"
  }
}

```

#### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|location|string|true|none|Where to find the token information|
|identifier|string|true|none|JSONPath expression or header name to identify the token value|
|fallbackIdentifiers|[string]|false|none|Additional locations to try when the primary identifier yields no<br>value, in order. Used where one provider returns several response<br>shapes on the same path.|
|valueMap|object|false|none|Maps values this provider reports onto the vocabulary the gateway<br>bills on. A reported value that is not a key is used unchanged.<br>Used where providers name the same billing tier differently.|
|» **additionalProperties**|string|false|none|none|

##### Enumerated Values

|Property|Value|
|---|---|
|location|payload|
|location|header|
|location|queryParam|
|location|pathParam|

## LLMProviderConfigurationRequest

<a id="schemallmproviderconfigurationrequest"></a>
<a id="schema_LLMProviderConfigurationRequest"></a>
<a id="tocSllmproviderconfigurationrequest"></a>
<a id="tocsllmproviderconfigurationrequest"></a>

```json
{
  "apiVersion": "gateway.api-platform.wso2.com/v1",
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

#### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|apiVersion|string|true|none|Provider specification version|
|kind|string|true|none|Provider kind|
|metadata|[Metadata](#schemametadata)|true|none|none|
|spec|[LLMProviderConfigData](#schemallmproviderconfigdata)|true|none|none|

##### Enumerated Values

|Property|Value|
|---|---|
|apiVersion|gateway.api-platform.wso2.com/v1|
|kind|LlmProvider|

## LLMProviderConfiguration

<a id="schemallmproviderconfiguration"></a>
<a id="schema_LLMProviderConfiguration"></a>
<a id="tocSllmproviderconfiguration"></a>
<a id="tocsllmproviderconfiguration"></a>

```json
{
  "apiVersion": "gateway.api-platform.wso2.com/v1",
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
        "header": "Authorization"
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

#### Properties

allOf

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|[LLMProviderConfigurationRequest](#schemallmproviderconfigurationrequest)|false|none|none|

and

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|object|false|none|none|
|» status|[ResourceStatus](#schemaresourcestatus)|false|read-only|Server-managed lifecycle fields. Populated on responses.|

## LLMProviderConfigData

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
  "globalPolicies": [
    {
      "name": "cors",
      "version": "v1",
      "executionCondition": "request.metadata[authenticated] != true",
      "params": {}
    }
  ],
  "operationPolicies": [
    {
      "name": "token-based-ratelimit",
      "version": "v1",
      "executionCondition": "string",
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
  "deploymentState": "deployed",
  "resilience": {
    "timeout": "15s",
    "idleTimeout": "0s"
  }
}

```

#### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|displayName|string|true|none|Human-readable LLM Provider name|
|version|string|true|none|Semantic version of the LLM Provider|
|context|string|false|none|Base path for all API routes (must start with /, no trailing slash)|
|vhost|string|false|none|Virtual host name used for routing. Supports standard domain names, subdomains, or wildcard domains. Must follow RFC-compliant hostname rules. Wildcards are only allowed in the left-most label (e.g., *.example.com).|
|template|string|true|none|Template name to use for this LLM Provider|
|upstreamDefinitions|[[UpstreamDefinition](#schemaupstreamdefinition)]|false|none|List of reusable upstream definitions with optional timeout configurations. Referenced by upstream.ref.|
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
|globalPolicies|[[Policy](#schemapolicy)]|false|none|Global (api-level) policies applied across ALL operations as one shared scope, evaluated before operation-level policies.|
|operationPolicies|[[OperationPolicy](#schemaoperationpolicy)]|false|none|Operation-level policies scoped to specific paths/methods, evaluated after global policies.|
|policies|[[LLMPolicy](#schemallmpolicy)]|false|none|DEPRECATED - use operationPolicies. Still honoured (treated identically to operationPolicies).|
|deploymentState|string|false|none|Desired deployment state - 'deployed' (default) or 'undeployed'. When set to 'undeployed', the LLM Provider is removed from router traffic but configuration and policies are preserved for potential redeployment.|
|resilience|[Resilience](#schemaresilience)|false|none|API-level backend/route timeout configuration. Applies to all routes generated for this LLM Provider (the routes that forward traffic upstream). Supported at the API level only - LLM routes are synthesized by the gateway, so there is no operation-level override.|

##### Enumerated Values

|Property|Value|
|---|---|
|deploymentState|deployed|
|deploymentState|undeployed|

## UpstreamAuth

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

#### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|auth|object|false|none|none|
|» type|string|true|none|none|
|» header|string|false|none|none|
|» value|string|false|write-only|Upstream credential. Write-only: accepted on create/update and never returned by the management API on a read, for any role. Supply either a literal value or a secret reference (e.g. a `secret` template expression); either way the field is omitted from management API response bodies. An update that omits it inherits the stored value; set `type: none` to remove auth.|

##### Enumerated Values

|Property|Value|
|---|---|
|type|api-key|
|type|other|
|type|none|

## LLMUpstreamAuth

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

#### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|type|string|true|none|none|
|header|string|false|none|none|
|value|string|false|write-only|Upstream credential. Write-only: accepted on create/update and never returned by the management API on a read, for any role. An update that omits it inherits the stored value; set `type: none` to remove auth.|

##### Enumerated Values

|Property|Value|
|---|---|
|type|api-key|
|type|other|
|type|none|

## LLMProxyProvider

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

#### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|id|string|true|none|Unique id of a deployed llm provider|
|auth|[LLMUpstreamAuth](#schemallmupstreamauth)|false|none|none|

## LLMProxyAdditionalProvider

<a id="schemallmproxyadditionalprovider"></a>
<a id="schema_LLMProxyAdditionalProvider"></a>
<a id="tocSllmproxyadditionalprovider"></a>
<a id="tocsllmproxyadditionalprovider"></a>

```json
{
  "id": "anthropic-provider",
  "as": "anthropic-upstream",
  "auth": {
    "type": "api-key",
    "header": "string",
    "value": "string"
  },
  "transformer": {
    "type": "openai-to-anthropic",
    "version": "v1",
    "params": {}
  }
}

```

Additional LLM provider attached to this proxy as a selectable upstream. Policies route to it by referring to the `as` name (defaults to `id`). Optional auth config is used by the proxy when calling a protected LlmProvider over the internal loopback route.

#### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|id|string|true|none|Unique id of a deployed llm provider|
|as|string|false|none|Logical LLM Provider name used by policies to select this provider. Must be unique within the proxy. Defaults to `id` when omitted.|
|auth|[LLMUpstreamAuth](#schemallmupstreamauth)|false|none|none|
|transformer|[LLMProxyTransformer](#schemallmproxytransformer)|false|none|Request/response translator applied when this provider is the selected upstream. The proxy injects the translator as a conditional policy whose execution condition matches this provider, so it runs only when the provider is selected. The provider's `as` name (defaults to `id`) is passed to the translator as its target upstream.|

## LLMProxyTransformer

<a id="schemallmproxytransformer"></a>
<a id="schema_LLMProxyTransformer"></a>
<a id="tocSllmproxytransformer"></a>
<a id="tocsllmproxytransformer"></a>

```json
{
  "type": "openai-to-anthropic",
  "version": "v1",
  "params": {}
}

```

Request/response translator applied when this provider is the selected upstream. The proxy injects the translator as a conditional policy whose execution condition matches this provider, so it runs only when the provider is selected. The provider's `as` name (defaults to `id`) is passed to the translator as its target upstream.

#### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|type|string|true|none|Translator policy name (for example openai-to-anthropic).|
|version|string|true|none|Major-only translator policy version (for example v1). The Gateway Controller resolves it to the installed full version.|
|params|object|false|none|Translator-specific parameters (for example model, apiVersion).|

## LLMAccessControl

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

#### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|mode|string|true|none|Access control mode|
|exceptions|[[RouteException](#schemarouteexception)]|false|none|Path exceptions to the access control mode|

##### Enumerated Values

|Property|Value|
|---|---|
|mode|allow_all|
|mode|deny_all|

## RouteException

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

#### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|path|string|true|none|Path pattern|
|methods|[string]|true|none|HTTP methods|

## LLMPolicy

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

#### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|name|string|true|none|none|
|version|string|true|none|none|
|paths|[[LLMPolicyPath](#schemallmpolicypath)]|true|none|none|

## LLMPolicyPath

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

#### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|path|string|true|none|none|
|methods|[string]|true|none|none|
|params|object|true|none|JSON Schema describing the parameters accepted by this policy. This itself is a JSON Schema document.|

## OperationPolicy

<a id="schemaoperationpolicy"></a>
<a id="schema_OperationPolicy"></a>
<a id="tocSoperationpolicy"></a>
<a id="tocsoperationpolicy"></a>

```json
{
  "name": "token-based-ratelimit",
  "version": "v1",
  "executionCondition": "string",
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

#### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|name|string|true|none|none|
|version|string|true|none|none|
|executionCondition|string|false|none|Expression controlling conditional execution of the policy|
|paths|[[OperationPolicyPath](#schemaoperationpolicypath)]|true|none|none|

## OperationPolicyPath

<a id="schemaoperationpolicypath"></a>
<a id="schema_OperationPolicyPath"></a>
<a id="tocSoperationpolicypath"></a>
<a id="tocsoperationpolicypath"></a>

```json
{
  "path": "/chat/completions",
  "methods": [
    "GET"
  ],
  "params": {}
}

```

#### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|path|string|true|none|none|
|methods|[string]|true|none|none|
|params|object|true|none|JSON Schema describing the parameters accepted by this policy. This itself is a JSON Schema document.|

## LLMProxyConfigurationRequest

<a id="schemallmproxyconfigurationrequest"></a>
<a id="schema_LLMProxyConfigurationRequest"></a>
<a id="tocSllmproxyconfigurationrequest"></a>
<a id="tocsllmproxyconfigurationrequest"></a>

```json
{
  "apiVersion": "gateway.api-platform.wso2.com/v1",
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

#### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|apiVersion|string|true|none|Proxy specification version|
|kind|string|true|none|Proxy kind|
|metadata|[Metadata](#schemametadata)|true|none|none|
|spec|[LLMProxyConfigData](#schemallmproxyconfigdata)|true|none|none|

##### Enumerated Values

|Property|Value|
|---|---|
|apiVersion|gateway.api-platform.wso2.com/v1|
|kind|LlmProxy|

## LLMProxyConfiguration

<a id="schemallmproxyconfiguration"></a>
<a id="schema_LLMProxyConfiguration"></a>
<a id="tocSllmproxyconfiguration"></a>
<a id="tocsllmproxyconfiguration"></a>

```json
{
  "apiVersion": "gateway.api-platform.wso2.com/v1",
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

#### Properties

allOf

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|[LLMProxyConfigurationRequest](#schemallmproxyconfigurationrequest)|false|none|none|

and

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|object|false|none|none|
|» status|[ResourceStatus](#schemaresourcestatus)|false|read-only|Server-managed lifecycle fields. Populated on responses.|

## LLMProxyConfigData

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
  "globalPolicies": [
    {
      "name": "cors",
      "version": "v1",
      "executionCondition": "request.metadata[authenticated] != true",
      "params": {}
    }
  ],
  "operationPolicies": [
    {
      "name": "token-based-ratelimit",
      "version": "v1",
      "executionCondition": "string",
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
  "additionalProviders": [
    {
      "id": "anthropic-provider",
      "as": "anthropic-upstream",
      "auth": {
        "type": "api-key",
        "header": "string",
        "value": "string"
      },
      "transformer": {
        "type": "openai-to-anthropic",
        "version": "v1",
        "params": {}
      }
    }
  ],
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
  "deploymentState": "deployed",
  "resilience": {
    "timeout": "15s",
    "idleTimeout": "0s"
  }
}

```

#### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|displayName|string|true|none|Human-readable LLM proxy name (must be URL-friendly - only letters, numbers, spaces, hyphens, underscores, and dots allowed)|
|version|string|true|none|Semantic version of the LLM proxy|
|context|string|false|none|Base path for all API routes (must start with /, no trailing slash)|
|vhost|string|false|none|Virtual host name used for routing. Supports standard domain names, subdomains, or wildcard domains. Must follow RFC-compliant hostname rules. Wildcards are only allowed in the left-most label (e.g., *.example.com).|
|provider|[LLMProxyProvider](#schemallmproxyprovider)|true|none|none|
|globalPolicies|[[Policy](#schemapolicy)]|false|none|Global (api-level) policies applied across ALL operations as one shared scope, evaluated before operation-level policies.|
|operationPolicies|[[OperationPolicy](#schemaoperationpolicy)]|false|none|Operation-level policies scoped to specific paths/methods, evaluated after global policies.|
|additionalProviders|[[LLMProxyAdditionalProvider](#schemallmproxyadditionalprovider)]|false|none|Optional list of additional LLM providers attached to this proxy as selectable upstreams. Policies (e.g. an OpenAI translator) can route requests to any of these by setting the upstream name. The primary `provider` field above remains the default upstream and the FK target.|
|policies|[[LLMPolicy](#schemallmpolicy)]|false|none|DEPRECATED - use operationPolicies. Still honoured (treated identically to operationPolicies).|
|deploymentState|string|false|none|Desired deployment state - 'deployed' (default) or 'undeployed'. When set to 'undeployed', the LLM Proxy is removed from router traffic but configuration and policies are preserved for potential redeployment.|
|resilience|[Resilience](#schemaresilience)|false|none|API-level backend/route timeout configuration. Applies to all routes generated for this LLM Proxy (the routes that forward traffic upstream). Supported at the API level only - LLM routes are synthesized by the gateway, so there is no operation-level override.|

##### Enumerated Values

|Property|Value|
|---|---|
|deploymentState|deployed|
|deploymentState|undeployed|

## SecretConfigurationRequest

<a id="schemasecretconfigurationrequest"></a>
<a id="schema_SecretConfigurationRequest"></a>
<a id="tocSsecretconfigurationrequest"></a>
<a id="tocssecretconfigurationrequest"></a>

```json
{
  "apiVersion": "gateway.api-platform.wso2.com/v1",
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

#### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|apiVersion|string|true|none|Secret specification version|
|kind|string|true|none|Secret resource kind|
|metadata|[Metadata](#schemametadata)|true|none|none|
|spec|[SecretConfigData](#schemasecretconfigdata)|true|none|none|

##### Enumerated Values

|Property|Value|
|---|---|
|apiVersion|gateway.api-platform.wso2.com/v1|
|kind|Secret|

## SecretConfiguration

<a id="schemasecretconfiguration"></a>
<a id="schema_SecretConfiguration"></a>
<a id="tocSsecretconfiguration"></a>
<a id="tocssecretconfiguration"></a>

```json
{
  "apiVersion": "gateway.api-platform.wso2.com/v1",
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

#### Properties

allOf

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|[SecretConfigurationRequest](#schemasecretconfigurationrequest)|false|none|none|

and

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|object|false|none|none|
|» status|[ResourceStatus](#schemaresourcestatus)|false|read-only|Server-managed lifecycle fields. Populated on responses.|

## SecretConfigData

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

#### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|displayName|string|true|none|Human-readable secret name (must be URL-friendly - only letters, numbers, spaces, hyphens, underscores, and dots allowed)|
|description|string|false|none|Description of the secret|
|value|string(password)|true|none|Secret value (stored encrypted)|

## SecretConfigListData

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

#### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|displayName|string|true|none|Human-readable secret name (must be URL-friendly - only letters, numbers, spaces, hyphens, underscores, and dots allowed)|
|description|string|false|none|Description of the secret, if the server includes it|

## SecretListItem

<a id="schemasecretlistitem"></a>
<a id="schema_SecretListItem"></a>
<a id="tocSsecretlistitem"></a>
<a id="tocssecretlistitem"></a>

```json
{
  "apiVersion": "gateway.api-platform.wso2.com/v1",
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

#### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|apiVersion|string|true|none|Secret specification version|
|kind|string|true|none|Secret resource kind|
|metadata|[Metadata](#schemametadata)|true|none|none|
|spec|[SecretConfigListData](#schemasecretconfiglistdata)|true|none|none|
|status|[ResourceStatus](#schemaresourcestatus)|false|read-only|Server-managed lifecycle fields. Omitted in list items may vary; the secret value is never included here.|

##### Enumerated Values

|Property|Value|
|---|---|
|apiVersion|gateway.api-platform.wso2.com/v1|
|kind|Secret|

## CertificateUploadRequest

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

#### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|name|string|true|none|Unique name for the certificate. Must be unique across all certificates.|
|certificate|string|true|none|PEM-encoded X.509 certificate(s). Can contain multiple certificates.|

## CertificateResponse

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

#### Properties

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

##### Enumerated Values

|Property|Value|
|---|---|
|status|success|
|status|error|

## CertificateListResponse

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

#### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|certificates|[[CertificateResponse](#schemacertificateresponse)]|false|none|none|
|totalCount|integer|false|none|Total number of certificate files|
|totalBytes|integer|false|none|Total bytes of all certificate files|
|status|string|false|none|none|

## APIKeyListResponse

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

#### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|apiKeys|[[APIKey](#schemaapikey)]|false|none|[Details of an API key]|
|totalCount|integer|false|none|Total number of API keys|
|status|string|false|none|none|

## SecretListResponse

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
      "apiVersion": "gateway.api-platform.wso2.com/v1",
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

#### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|status|string|false|none|none|
|count|integer|false|none|Total number of secrets|
|secrets|[[SecretListItem](#schemasecretlistitem)]|false|none|List of secrets. For security, the spec.value field is omitted for every item in the list; retrieve a single secret by id to obtain the decrypted value.|

## SecretResourceServiceStatus

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

#### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|id|string|true|none|Same as metadata.name / secret handle|
|createdAt|string(date-time)|false|none|none|
|updatedAt|string(date-time)|false|none|none|

## SecretConfigurationResponseCreateUpdate

<a id="schemasecretconfigurationresponsecreateupdate"></a>
<a id="schema_SecretConfigurationResponseCreateUpdate"></a>
<a id="tocSsecretconfigurationresponsecreateupdate"></a>
<a id="tocssecretconfigurationresponsecreateupdate"></a>

```json
{
  "apiVersion": "gateway.api-platform.wso2.com/v1",
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

#### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|apiVersion|string|true|none|none|
|kind|string|true|none|none|
|metadata|[Metadata](#schemametadata)|true|none|none|
|spec|[SecretConfigListData](#schemasecretconfiglistdata)|true|none|none|
|status|[SecretResourceServiceStatus](#schemasecretresourceservicestatus)|true|none|Id and optional timestamps. Not the full ResourceStatus model (no `state` or<br>`deployedAt`).|

##### Enumerated Values

|Property|Value|
|---|---|
|apiVersion|gateway.api-platform.wso2.com/v1|
|kind|Secret|

## SecretConfigurationResponseRetrieved

<a id="schemasecretconfigurationresponseretrieved"></a>
<a id="schema_SecretConfigurationResponseRetrieved"></a>
<a id="tocSsecretconfigurationresponseretrieved"></a>
<a id="tocssecretconfigurationresponseretrieved"></a>

```json
{
  "apiVersion": "gateway.api-platform.wso2.com/v1",
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

#### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|apiVersion|string|true|none|none|
|kind|string|true|none|none|
|metadata|[Metadata](#schemametadata)|true|none|none|
|spec|[SecretConfigData](#schemasecretconfigdata)|true|none|none|
|status|[SecretResourceServiceStatus](#schemasecretresourceservicestatus)|true|none|Id and optional timestamps. Not the full ResourceStatus model (no `state` or<br>`deployedAt`).|

##### Enumerated Values

|Property|Value|
|---|---|
|apiVersion|gateway.api-platform.wso2.com/v1|
|kind|Secret|
