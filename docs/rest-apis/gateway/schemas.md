# Schemas

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
    "name": "petstore-api-v1.0"
  },
  "spec": {
    "displayName": "Petstore-API",
    "version": "v1.0",
    "context": "/petstore/$version",
    "upstream": {
      "main": {
        "url": "https://petstore3.swagger.io/api/v3"
      }
    },
    "operations": [
      {
        "method": "PUT",
        "path": "/pet"
      },
      {
        "method": "POST",
        "path": "/pet",
        "policies": [
          {
            "name": "log-message",
            "version": "v1",
            "params": {
              "request": {
                "payload": true,
                "headers": true
              }
            }
          }
        ]
      },
      {
        "method": "GET",
        "path": "/pet/findByStatus"
      },
      {
        "method": "GET",
        "path": "/pet/{petId}"
      },
      {
        "method": "POST",
        "path": "/pet/{petId}"
      },
      {
        "method": "DELETE",
        "path": "/pet/{petId}"
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
    "name": "weather-api-v1.0",
    "labels": {
      "environment": "production",
      "team": "backend",
      "version": "v1"
    }
  },
  "spec": {
    "displayName": "weather-api",
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
            "name": "apiKeyValidation",
            "version": "v1",
            "executionCondition": "request.metadata[authenticated] != true",
            "params": {}
          }
        ]
      }
    ],
    "policies": [
      {
        "name": "apiKeyValidation",
        "version": "v1",
        "executionCondition": "request.metadata[authenticated] != true",
        "params": {}
      }
    ],
    "deploymentState": "deployed"
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

<h2 id="tocS_Metadata">Metadata</h2>

<a id="schemametadata"></a>
<a id="schema_Metadata"></a>
<a id="tocSmetadata"></a>
<a id="tocsmetadata"></a>

```json
{
  "name": "weather-api-v1.0",
  "labels": {
    "environment": "production",
    "team": "backend",
    "version": "v1"
  }
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|name|string|true|none|Unique handle for the resource|
|labels|object|false|none|Labels are key-value pairs for organizing and selecting APIs. Keys must not contain spaces.|
|» **additionalProperties**|string|false|none|none|

<h2 id="tocS_APIConfigData">APIConfigData</h2>

<a id="schemaapiconfigdata"></a>
<a id="schema_APIConfigData"></a>
<a id="tocSapiconfigdata"></a>
<a id="tocsapiconfigdata"></a>

```json
{
  "displayName": "weather-api",
  "version": "v1.0",
  "context": "/weather",
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
      "name": "apiKeyValidation",
      "version": "v1",
      "executionCondition": "request.metadata[authenticated] != true",
      "params": {}
    }
  ],
  "operations": [
    {
      "method": "GET",
      "path": "/{country_code}/{city}",
      "policies": [
        {
          "name": "apiKeyValidation",
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
|context|string|true|none|Base path for all API routes (must start with /, no trailing slash)|
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
  "path": "/{country_code}/{city}",
  "policies": [
    {
      "name": "apiKeyValidation",
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
  "name": "apiKeyValidation",
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
  "displayName": "weather-api",
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
          "name": "apiKeyValidation",
          "version": "v1",
          "executionCondition": "request.metadata[authenticated] != true",
          "params": {}
        }
      ]
    }
  ],
  "policies": [
    {
      "name": "apiKeyValidation",
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
      "name": "apiKeyValidation",
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

<h2 id="tocS_RestAPICreateResponse">RestAPICreateResponse</h2>

<a id="schemarestapicreateresponse"></a>
<a id="schema_RestAPICreateResponse"></a>
<a id="tocSrestapicreateresponse"></a>
<a id="tocsrestapicreateresponse"></a>

```json
{
  "status": "success",
  "message": "RestAPI created successfully",
  "id": "weather-api-v1.0",
  "createdAt": "2025-10-11T10:30:00Z"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|status|string|false|none|none|
|message|string|false|none|none|
|id|string|false|none|Unique id for the created RestAPI|
|createdAt|string(date-time)|false|none|none|

<h2 id="tocS_RestAPIUpdateResponse">RestAPIUpdateResponse</h2>

<a id="schemarestapiupdateresponse"></a>
<a id="schema_RestAPIUpdateResponse"></a>
<a id="tocSrestapiupdateresponse"></a>
<a id="tocsrestapiupdateresponse"></a>

```json
{
  "status": "success",
  "message": "RestAPI updated successfully",
  "id": "weather-api-v1.0",
  "updatedAt": "2025-10-11T11:45:00Z"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|status|string|false|none|none|
|message|string|false|none|none|
|id|string|false|none|none|
|updatedAt|string(date-time)|false|none|none|

<h2 id="tocS_WebSubAPICreateResponse">WebSubAPICreateResponse</h2>

<a id="schemawebsubapicreateresponse"></a>
<a id="schema_WebSubAPICreateResponse"></a>
<a id="tocSwebsubapicreateresponse"></a>
<a id="tocswebsubapicreateresponse"></a>

```json
{
  "status": "success",
  "message": "WebSubAPI created successfully",
  "id": "weather-websub-api",
  "createdAt": "2025-10-11T10:30:00Z"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|status|string|false|none|none|
|message|string|false|none|none|
|id|string|false|none|Unique id for the created WebSubAPI|
|createdAt|string(date-time)|false|none|none|

<h2 id="tocS_WebSubAPIUpdateResponse">WebSubAPIUpdateResponse</h2>

<a id="schemawebsubapiupdateresponse"></a>
<a id="schema_WebSubAPIUpdateResponse"></a>
<a id="tocSwebsubapiupdateresponse"></a>
<a id="tocswebsubapiupdateresponse"></a>

```json
{
  "status": "success",
  "message": "WebSubAPI updated successfully",
  "id": "weather-websub-api",
  "updatedAt": "2025-10-11T11:45:00Z"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|status|string|false|none|none|
|message|string|false|none|none|
|id|string|false|none|none|
|updatedAt|string(date-time)|false|none|none|

<h2 id="tocS_APIKeyCreationRequest">APIKeyCreationRequest</h2>

<a id="schemaapikeycreationrequest"></a>
<a id="schema_APIKeyCreationRequest"></a>
<a id="tocSapikeycreationrequest"></a>
<a id="tocsapikeycreationrequest"></a>

```json
{
  "name": "my-production-key",
  "apiKey": "xxxxxx-wso2-api-platform-key-xxxxxx-xxxxxxx",
  "maskedApiKey": "apip_****xyz789",
  "expiresIn": {
    "unit": "days",
    "duration": 30
  },
  "expiresAt": "2026-12-08T10:30:00Z",
  "externalRefId": "cloud-apim-key-98765",
  "issuer": "api-platform-devportal"
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
    "apiId": "weather-api-v1.0",
    "status": "active",
    "createdAt": "2025-12-08T10:30:00Z",
    "createdBy": "api_consumer",
    "expiresAt": "2025-12-08T10:30:00Z",
    "source": "local",
    "externalRefId": "cloud-apim-key-98765"
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
  "apiId": "weather-api-v1.0",
  "status": "active",
  "createdAt": "2025-12-08T10:30:00Z",
  "createdBy": "api_consumer",
  "expiresAt": "2025-12-08T10:30:00Z",
  "source": "local",
  "externalRefId": "cloud-apim-key-98765"
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
{
  "expiresIn": {
    "unit": "days",
    "duration": 30
  },
  "expiresAt": "2026-12-08T10:30:00Z"
}

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
  "name": "my-production-key",
  "apiKey": "xxxxxx-wso2-api-platform-key-xxxxxx-xxxxxxx",
  "maskedApiKey": "apip_****xyz789",
  "expiresIn": {
    "unit": "days",
    "duration": 30
  },
  "expiresAt": "2026-12-08T10:30:00Z",
  "externalRefId": "cloud-apim-key-98765",
  "issuer": "api-platform-devportal"
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
  "subscriptionToken": "string",
  "applicationId": "string",
  "subscriptionPlanId": "string",
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

<h2 id="tocS_RestAPIListItem">RestAPIListItem</h2>

<a id="schemarestapilistitem"></a>
<a id="schema_RestAPIListItem"></a>
<a id="tocSrestapilistitem"></a>
<a id="tocsrestapilistitem"></a>

```json
{
  "id": "weather-api-v1.0",
  "displayName": "weather-api",
  "version": "v1.0",
  "context": "/weather",
  "status": "deployed",
  "createdAt": "2025-10-11T10:30:00Z",
  "updatedAt": "2025-10-11T10:30:00Z"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|id|string|false|none|none|
|displayName|string|false|none|none|
|version|string|false|none|none|
|context|string|false|none|none|
|status|string|false|none|none|
|createdAt|string(date-time)|false|none|none|
|updatedAt|string(date-time)|false|none|none|

#### Enumerated Values

|Property|Value|
|---|---|
|status|deployed|
|status|undeployed|

<h2 id="tocS_WebSubAPIListItem">WebSubAPIListItem</h2>

<a id="schemawebsubapilistitem"></a>
<a id="schema_WebSubAPIListItem"></a>
<a id="tocSwebsubapilistitem"></a>
<a id="tocswebsubapilistitem"></a>

```json
{
  "id": "weather-websub-api",
  "displayName": "weather-websub",
  "version": "v1.0",
  "context": "/weather-events",
  "status": "deployed",
  "createdAt": "2025-10-11T10:30:00Z",
  "updatedAt": "2025-10-11T10:30:00Z"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|id|string|false|none|none|
|displayName|string|false|none|none|
|version|string|false|none|none|
|context|string|false|none|none|
|status|string|false|none|none|
|createdAt|string(date-time)|false|none|none|
|updatedAt|string(date-time)|false|none|none|

#### Enumerated Values

|Property|Value|
|---|---|
|status|deployed|
|status|undeployed|

<h2 id="tocS_RestAPIDetailResponse">RestAPIDetailResponse</h2>

<a id="schemarestapidetailresponse"></a>
<a id="schema_RestAPIDetailResponse"></a>
<a id="tocSrestapidetailresponse"></a>
<a id="tocsrestapidetailresponse"></a>

```json
{
  "status": "success",
  "api": {
    "id": "weather-api-v1.0",
    "configuration": {
      "apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
      "kind": "RestApi",
      "metadata": {
        "name": "petstore-api-v1.0"
      },
      "spec": {
        "displayName": "Petstore-API",
        "version": "v1.0",
        "context": "/petstore/$version",
        "upstream": {
          "main": {
            "url": "https://petstore3.swagger.io/api/v3"
          }
        },
        "operations": [
          {
            "method": "PUT",
            "path": "/pet"
          },
          {
            "method": "POST",
            "path": "/pet",
            "policies": [
              {
                "name": "log-message",
                "version": "v1",
                "params": {
                  "request": {
                    "payload": true,
                    "headers": true
                  }
                }
              }
            ]
          },
          {
            "method": "GET",
            "path": "/pet/findByStatus"
          },
          {
            "method": "GET",
            "path": "/pet/{petId}"
          },
          {
            "method": "POST",
            "path": "/pet/{petId}"
          },
          {
            "method": "DELETE",
            "path": "/pet/{petId}"
          }
        ]
      }
    },
    "metadata": {
      "status": "deployed",
      "createdAt": "2025-10-11T10:30:00Z",
      "updatedAt": "2025-10-11T10:30:00Z",
      "deployedAt": "2025-10-11T10:30:05Z"
    }
  }
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|status|string|false|none|none|
|api|object|false|none|none|
|» id|string|false|none|Unique id for the RestAPI|
|» configuration|[RestAPI](#schemarestapi)|false|none|none|
|» metadata|object|false|none|none|
|»» status|string|false|none|none|
|»» createdAt|string(date-time)|false|none|none|
|»» updatedAt|string(date-time)|false|none|none|
|»» deployedAt|string(date-time)|false|none|none|

#### Enumerated Values

|Property|Value|
|---|---|
|status|deployed|
|status|undeployed|

<h2 id="tocS_WebSubAPIDetailResponse">WebSubAPIDetailResponse</h2>

<a id="schemawebsubapidetailresponse"></a>
<a id="schema_WebSubAPIDetailResponse"></a>
<a id="tocSwebsubapidetailresponse"></a>
<a id="tocswebsubapidetailresponse"></a>

```json
{
  "status": "success",
  "api": {
    "id": "weather-websub-api",
    "configuration": {
      "apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
      "kind": "WebSubApi",
      "metadata": {
        "name": "weather-api-v1.0",
        "labels": {
          "environment": "production",
          "team": "backend",
          "version": "v1"
        }
      },
      "spec": {
        "displayName": "weather-api",
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
                "name": "apiKeyValidation",
                "version": "v1",
                "executionCondition": "request.metadata[authenticated] != true",
                "params": {}
              }
            ]
          }
        ],
        "policies": [
          {
            "name": "apiKeyValidation",
            "version": "v1",
            "executionCondition": "request.metadata[authenticated] != true",
            "params": {}
          }
        ],
        "deploymentState": "deployed"
      }
    },
    "metadata": {
      "status": "deployed",
      "createdAt": "2025-10-11T10:30:00Z",
      "updatedAt": "2025-10-11T10:30:00Z",
      "deployedAt": "2025-10-11T10:30:05Z"
    }
  }
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|status|string|false|none|none|
|api|object|false|none|none|
|» id|string|false|none|Unique id for the WebSubAPI|
|» configuration|[WebSubAPI](#schemawebsubapi)|false|none|none|
|» metadata|object|false|none|none|
|»» status|string|false|none|none|
|»» createdAt|string(date-time)|false|none|none|
|»» updatedAt|string(date-time)|false|none|none|
|»» deployedAt|string(date-time)|false|none|none|

#### Enumerated Values

|Property|Value|
|---|---|
|status|deployed|
|status|undeployed|

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
    "name": "weather-api-v1.0",
    "labels": {
      "environment": "production",
      "team": "backend",
      "version": "v1"
    }
  },
  "spec": {
    "displayName": "mcp-proxy-1",
    "version": "v1.0",
    "context": "/mcp-proxy",
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
        "name": "apiKeyValidation",
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

<h2 id="tocS_MCPProxyConfigData">MCPProxyConfigData</h2>

<a id="schemamcpproxyconfigdata"></a>
<a id="schema_MCPProxyConfigData"></a>
<a id="tocSmcpproxyconfigdata"></a>
<a id="tocsmcpproxyconfigdata"></a>

```json
{
  "displayName": "mcp-proxy-1",
  "version": "v1.0",
  "context": "/mcp-proxy",
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
      "name": "apiKeyValidation",
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

<h2 id="tocS_MCPProxyCreateResponse">MCPProxyCreateResponse</h2>

<a id="schemamcpproxycreateresponse"></a>
<a id="schema_MCPProxyCreateResponse"></a>
<a id="tocSmcpproxycreateresponse"></a>
<a id="tocsmcpproxycreateresponse"></a>

```json
{
  "status": "success",
  "message": "MCP proxy created successfully",
  "id": "some-mcp-v1.0",
  "createdAt": "2025-12-12T10:30:00Z"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|status|string|false|none|none|
|message|string|false|none|none|
|id|string|false|none|Unique handle (metadata.name) for the created MCPProxy|
|createdAt|string(date-time)|false|none|none|

<h2 id="tocS_MCPProxyUpdateResponse">MCPProxyUpdateResponse</h2>

<a id="schemamcpproxyupdateresponse"></a>
<a id="schema_MCPProxyUpdateResponse"></a>
<a id="tocSmcpproxyupdateresponse"></a>
<a id="tocsmcpproxyupdateresponse"></a>

```json
{
  "status": "success",
  "message": "MCP proxy updated successfully",
  "id": "some-mcp-v1.0",
  "updatedAt": "2025-12-12T11:45:00Z"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|status|string|false|none|none|
|message|string|false|none|none|
|id|string|false|none|Unique handle (metadata.name) for the created MCPProxy|
|updatedAt|string(date-time)|false|none|none|

<h2 id="tocS_MCPProxyListItem">MCPProxyListItem</h2>

<a id="schemamcpproxylistitem"></a>
<a id="schema_MCPProxyListItem"></a>
<a id="tocSmcpproxylistitem"></a>
<a id="tocsmcpproxylistitem"></a>

```json
{
  "id": "mcp-proxy-1-v1.0",
  "displayName": "mcp-proxy-1",
  "version": "v1.0",
  "context": "/mcp-proxy",
  "specVersion": "2025-06-18",
  "status": "deployed",
  "createdAt": "2025-11-24T10:30:00Z",
  "updatedAt": "2025-11-24T10:30:00Z"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|id|string|false|none|none|
|displayName|string|false|none|none|
|version|string|false|none|none|
|context|string|false|none|none|
|specVersion|string|false|none|none|
|status|string|false|none|none|
|createdAt|string(date-time)|false|none|none|
|updatedAt|string(date-time)|false|none|none|

#### Enumerated Values

|Property|Value|
|---|---|
|status|deployed|
|status|undeployed|

<h2 id="tocS_MCPDetailResponse">MCPDetailResponse</h2>

<a id="schemamcpdetailresponse"></a>
<a id="schema_MCPDetailResponse"></a>
<a id="tocSmcpdetailresponse"></a>
<a id="tocsmcpdetailresponse"></a>

```json
{
  "status": "success",
  "mcp": {
    "id": "mcp-proxy-1-v1.0",
    "configuration": {
      "apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
      "kind": "Mcp",
      "metadata": {
        "name": "weather-api-v1.0",
        "labels": {
          "environment": "production",
          "team": "backend",
          "version": "v1"
        }
      },
      "spec": {
        "displayName": "mcp-proxy-1",
        "version": "v1.0",
        "context": "/mcp-proxy",
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
            "name": "apiKeyValidation",
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

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|status|string|false|none|none|
|mcp|object|false|none|none|
|» id|string|false|none|Unique id for the MCPProxy|
|» configuration|[MCPProxyConfiguration](#schemamcpproxyconfiguration)|false|none|none|
|» metadata|object|false|none|none|
|»» status|string|false|none|none|
|»» createdAt|string(date-time)|false|none|none|
|»» updatedAt|string(date-time)|false|none|none|
|»» deployedAt|string(date-time)|false|none|none|

#### Enumerated Values

|Property|Value|
|---|---|
|status|deployed|
|status|undeployed|

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
    "name": "weather-api-v1.0",
    "labels": {
      "environment": "production",
      "team": "backend",
      "version": "v1"
    }
  },
  "spec": {
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

<h2 id="tocS_LLMProviderTemplateCreateResponse">LLMProviderTemplateCreateResponse</h2>

<a id="schemallmprovidertemplatecreateresponse"></a>
<a id="schema_LLMProviderTemplateCreateResponse"></a>
<a id="tocSllmprovidertemplatecreateresponse"></a>
<a id="tocsllmprovidertemplatecreateresponse"></a>

```json
{
  "status": "success",
  "message": "LLM provider template created successfully",
  "id": "openai",
  "createdAt": "2025-10-11T10:30:00Z"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|status|string|false|none|none|
|message|string|false|none|none|
|id|string|false|none|none|
|createdAt|string(date-time)|false|none|none|

<h2 id="tocS_LLMProviderTemplateUpdateResponse">LLMProviderTemplateUpdateResponse</h2>

<a id="schemallmprovidertemplateupdateresponse"></a>
<a id="schema_LLMProviderTemplateUpdateResponse"></a>
<a id="tocSllmprovidertemplateupdateresponse"></a>
<a id="tocsllmprovidertemplateupdateresponse"></a>

```json
{
  "status": "success",
  "message": "LLM provider template updated successfully",
  "id": "openai",
  "updatedAt": "2025-10-11T11:45:00Z"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|status|string|false|none|none|
|message|string|false|none|none|
|id|string|false|none|none|
|updatedAt|string(date-time)|false|none|none|

<h2 id="tocS_LLMProviderTemplateListItem">LLMProviderTemplateListItem</h2>

<a id="schemallmprovidertemplatelistitem"></a>
<a id="schema_LLMProviderTemplateListItem"></a>
<a id="tocSllmprovidertemplatelistitem"></a>
<a id="tocsllmprovidertemplatelistitem"></a>

```json
{
  "id": "openai",
  "displayName": "OpenAI",
  "createdAt": "2025-10-11T10:30:00Z",
  "updatedAt": "2025-10-11T10:30:00Z"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|id|string|false|none|none|
|displayName|string|false|none|none|
|createdAt|string(date-time)|false|none|none|
|updatedAt|string(date-time)|false|none|none|

<h2 id="tocS_LLMProviderTemplateDetailResponse">LLMProviderTemplateDetailResponse</h2>

<a id="schemallmprovidertemplatedetailresponse"></a>
<a id="schema_LLMProviderTemplateDetailResponse"></a>
<a id="tocSllmprovidertemplatedetailresponse"></a>
<a id="tocsllmprovidertemplatedetailresponse"></a>

```json
{
  "status": "success",
  "template": {
    "id": "openai",
    "configuration": {
      "apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
      "kind": "LlmProviderTemplate",
      "metadata": {
        "name": "weather-api-v1.0",
        "labels": {
          "environment": "production",
          "team": "backend",
          "version": "v1"
        }
      },
      "spec": {
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
    },
    "metadata": {
      "createdAt": "2025-10-11T10:30:00Z",
      "updatedAt": "2025-10-11T10:30:00Z"
    }
  }
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|status|string|false|none|none|
|template|object|false|none|none|
|» id|string|false|none|none|
|» configuration|[LLMProviderTemplate](#schemallmprovidertemplate)|false|none|none|
|» metadata|object|false|none|none|
|»» createdAt|string(date-time)|false|none|none|
|»» updatedAt|string(date-time)|false|none|none|

<h2 id="tocS_LLMProviderCreateResponse">LLMProviderCreateResponse</h2>

<a id="schemallmprovidercreateresponse"></a>
<a id="schema_LLMProviderCreateResponse"></a>
<a id="tocSllmprovidercreateresponse"></a>
<a id="tocsllmprovidercreateresponse"></a>

```json
{
  "status": "success",
  "message": "LLM provider created successfully",
  "id": "openai",
  "createdAt": "2025-11-25T10:30:00Z"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|status|string|false|none|none|
|message|string|false|none|none|
|id|string|false|none|none|
|createdAt|string(date-time)|false|none|none|

<h2 id="tocS_LLMProxyCreateResponse">LLMProxyCreateResponse</h2>

<a id="schemallmproxycreateresponse"></a>
<a id="schema_LLMProxyCreateResponse"></a>
<a id="tocSllmproxycreateresponse"></a>
<a id="tocsllmproxycreateresponse"></a>

```json
{
  "status": "success",
  "message": "LLM proxy created successfully",
  "id": "wso2-con-assistant",
  "createdAt": "2025-11-25T10:30:00Z"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|status|string|false|none|none|
|message|string|false|none|none|
|id|string|false|none|none|
|createdAt|string(date-time)|false|none|none|

<h2 id="tocS_LLMProviderUpdateResponse">LLMProviderUpdateResponse</h2>

<a id="schemallmproviderupdateresponse"></a>
<a id="schema_LLMProviderUpdateResponse"></a>
<a id="tocSllmproviderupdateresponse"></a>
<a id="tocsllmproviderupdateresponse"></a>

```json
{
  "status": "success",
  "message": "LLM provider updated successfully",
  "id": "wso2-openai-provider",
  "updatedAt": "2025-11-25T11:45:00Z"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|status|string|false|none|none|
|message|string|false|none|none|
|id|string|false|none|none|
|updatedAt|string(date-time)|false|none|none|

<h2 id="tocS_LLMProxyUpdateResponse">LLMProxyUpdateResponse</h2>

<a id="schemallmproxyupdateresponse"></a>
<a id="schema_LLMProxyUpdateResponse"></a>
<a id="tocSllmproxyupdateresponse"></a>
<a id="tocsllmproxyupdateresponse"></a>

```json
{
  "status": "success",
  "message": "LLM proxy updated successfully",
  "id": "wso2-con-assistant",
  "updatedAt": "2025-11-25T11:45:00Z"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|status|string|false|none|none|
|message|string|false|none|none|
|id|string|false|none|none|
|updatedAt|string(date-time)|false|none|none|

<h2 id="tocS_LLMProviderListItem">LLMProviderListItem</h2>

<a id="schemallmproviderlistitem"></a>
<a id="schema_LLMProviderListItem"></a>
<a id="tocSllmproviderlistitem"></a>
<a id="tocsllmproviderlistitem"></a>

```json
{
  "id": "openai-provider",
  "displayName": "WSO2 OpenAI Provider",
  "version": "v1.0",
  "template": "openai",
  "status": "deployed",
  "createdAt": "2025-11-25T10:30:00Z",
  "updatedAt": "2025-11-25T10:30:00Z"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|id|string|false|none|none|
|displayName|string|false|none|none|
|version|string|false|none|none|
|template|string|false|none|none|
|status|string|false|none|none|
|createdAt|string(date-time)|false|none|none|
|updatedAt|string(date-time)|false|none|none|

#### Enumerated Values

|Property|Value|
|---|---|
|status|deployed|
|status|undeployed|

<h2 id="tocS_LLMProxyListItem">LLMProxyListItem</h2>

<a id="schemallmproxylistitem"></a>
<a id="schema_LLMProxyListItem"></a>
<a id="tocSllmproxylistitem"></a>
<a id="tocsllmproxylistitem"></a>

```json
{
  "id": "wso2-con-assistant",
  "displayName": "WSO2 Con Assistant",
  "version": "v1.0",
  "provider": "wso2-openai-provider",
  "status": "deployed",
  "createdAt": "2025-11-25T10:30:00Z",
  "updatedAt": "2025-11-25T10:30:00Z"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|id|string|false|none|none|
|displayName|string|false|none|none|
|version|string|false|none|none|
|provider|string|false|none|Unique id of a deployed llm provider|
|status|string|false|none|none|
|createdAt|string(date-time)|false|none|none|
|updatedAt|string(date-time)|false|none|none|

#### Enumerated Values

|Property|Value|
|---|---|
|status|deployed|
|status|undeployed|

<h2 id="tocS_LLMProviderDetailResponse">LLMProviderDetailResponse</h2>

<a id="schemallmproviderdetailresponse"></a>
<a id="schema_LLMProviderDetailResponse"></a>
<a id="tocSllmproviderdetailresponse"></a>
<a id="tocsllmproviderdetailresponse"></a>

```json
{
  "status": "success",
  "provider": {
    "id": "wso2-openai",
    "configuration": {
      "apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
      "kind": "LlmProvider",
      "metadata": {
        "name": "weather-api-v1.0",
        "labels": {
          "environment": "production",
          "team": "backend",
          "version": "v1"
        }
      },
      "spec": {
        "displayName": "WSO2 OpenAI Provider",
        "version": "v1.0",
        "context": "/openai",
        "vhost": "api.openai",
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
            "name": "budgetControl",
            "version": "v1.0.0",
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
    },
    "deploymentStatus": "deployed",
    "metadata": {
      "createdAt": "2025-11-25T10:30:00Z",
      "updatedAt": "2025-11-25T10:30:00Z",
      "deployedAt": "2025-11-25T10:35:00Z"
    }
  }
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|status|string|false|none|none|
|provider|object|false|none|none|
|» id|string|false|none|none|
|» configuration|[LLMProviderConfiguration](#schemallmproviderconfiguration)|false|none|none|
|» deploymentStatus|string|false|none|none|
|» metadata|object|false|none|none|
|»» createdAt|string(date-time)|false|none|none|
|»» updatedAt|string(date-time)|false|none|none|
|»» deployedAt|string(date-time)|false|none|none|

#### Enumerated Values

|Property|Value|
|---|---|
|deploymentStatus|deployed|
|deploymentStatus|undeployed|

<h2 id="tocS_LLMProxyDetailResponse">LLMProxyDetailResponse</h2>

<a id="schemallmproxydetailresponse"></a>
<a id="schema_LLMProxyDetailResponse"></a>
<a id="tocSllmproxydetailresponse"></a>
<a id="tocsllmproxydetailresponse"></a>

```json
{
  "status": "success",
  "proxy": {
    "id": "wso2-docs-assistant",
    "configuration": {
      "apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
      "kind": "LlmProxy",
      "metadata": {
        "name": "weather-api-v1.0",
        "labels": {
          "environment": "production",
          "team": "backend",
          "version": "v1"
        }
      },
      "spec": {
        "displayName": "wso2-con-assistant",
        "version": "v1.0",
        "context": "/openai",
        "vhost": "api.openai",
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
            "name": "budgetControl",
            "version": "v1.0.0",
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
    },
    "deploymentStatus": "deployed",
    "metadata": {
      "createdAt": "2025-11-25T10:30:00Z",
      "updatedAt": "2025-11-25T10:30:00Z",
      "deployedAt": "2025-11-25T10:35:00Z"
    }
  }
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|status|string|false|none|none|
|proxy|object|false|none|none|
|» id|string|false|none|none|
|» configuration|[LLMProxyConfiguration](#schemallmproxyconfiguration)|false|none|none|
|» deploymentStatus|string|false|none|none|
|» metadata|object|false|none|none|
|»» createdAt|string(date-time)|false|none|none|
|»» updatedAt|string(date-time)|false|none|none|
|»» deployedAt|string(date-time)|false|none|none|

#### Enumerated Values

|Property|Value|
|---|---|
|deploymentStatus|deployed|
|deploymentStatus|undeployed|

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
    "name": "weather-api-v1.0",
    "labels": {
      "environment": "production",
      "team": "backend",
      "version": "v1"
    }
  },
  "spec": {
    "displayName": "WSO2 OpenAI Provider",
    "version": "v1.0",
    "context": "/openai",
    "vhost": "api.openai",
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
        "name": "budgetControl",
        "version": "v1.0.0",
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
  "vhost": "api.openai",
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
      "name": "budgetControl",
      "version": "v1.0.0",
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
  "name": "budgetControl",
  "version": "v1.0.0",
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
    "name": "weather-api-v1.0",
    "labels": {
      "environment": "production",
      "team": "backend",
      "version": "v1"
    }
  },
  "spec": {
    "displayName": "wso2-con-assistant",
    "version": "v1.0",
    "context": "/openai",
    "vhost": "api.openai",
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
        "name": "budgetControl",
        "version": "v1.0.0",
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

<h2 id="tocS_LLMProxyConfigData">LLMProxyConfigData</h2>

<a id="schemallmproxyconfigdata"></a>
<a id="schema_LLMProxyConfigData"></a>
<a id="tocSllmproxyconfigdata"></a>
<a id="tocsllmproxyconfigdata"></a>

```json
{
  "displayName": "wso2-con-assistant",
  "version": "v1.0",
  "context": "/openai",
  "vhost": "api.openai",
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
      "name": "budgetControl",
      "version": "v1.0.0",
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
    "name": "weather-api-v1.0",
    "labels": {
      "environment": "production",
      "team": "backend",
      "version": "v1"
    }
  },
  "spec": {
    "displayName": "WSO2 OpenAI Key",
    "description": "WSO2 OpenAI provider API Key",
    "value": "sk_xxx"
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

<h2 id="tocS_SecretConfigData">SecretConfigData</h2>

<a id="schemasecretconfigdata"></a>
<a id="schema_SecretConfigData"></a>
<a id="tocSsecretconfigdata"></a>
<a id="tocssecretconfigdata"></a>

```json
{
  "displayName": "WSO2 OpenAI Key",
  "description": "WSO2 OpenAI provider API Key",
  "value": "sk_xxx"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|displayName|string|true|none|Human-readable secret name (must be URL-friendly - only letters, numbers, spaces, hyphens, underscores, and dots allowed)|
|description|string|false|none|Description of the secret|
|value|string(password)|true|none|Secret value (stored encrypted)|

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
      "apiId": "weather-api-v1.0",
      "status": "active",
      "createdAt": "2025-12-08T10:30:00Z",
      "createdBy": "api_consumer",
      "expiresAt": "2025-12-08T10:30:00Z",
      "source": "local",
      "externalRefId": "cloud-apim-key-98765"
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

<h2 id="tocS_SecretResponse">SecretResponse</h2>

<a id="schemasecretresponse"></a>
<a id="schema_SecretResponse"></a>
<a id="tocSsecretresponse"></a>
<a id="tocssecretresponse"></a>

```json
{
  "id": "prod-db-password",
  "createdAt": "2026-01-05T10:00:00Z",
  "updatedAt": "2026-01-05T10:00:00Z"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|id|string|true|none|Unique secret identifier|
|createdAt|string(date-time)|true|none|Timestamp when the secret was created (UTC)|
|updatedAt|string(date-time)|true|none|Timestamp when the secret was last updated (UTC)|

<h2 id="tocS_SecretListResponse">SecretListResponse</h2>

<a id="schemasecretlistresponse"></a>
<a id="schema_SecretListResponse"></a>
<a id="tocSsecretlistresponse"></a>
<a id="tocssecretlistresponse"></a>

```json
{
  "secrets": [
    {
      "id": "openai-api-key",
      "displayName": "OpenAI API Key",
      "createdAt": "2026-01-05T10:00:00Z",
      "updatedAt": "2026-01-05T10:30:00Z"
    }
  ],
  "totalCount": 5,
  "status": "success"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|secrets|[object]|false|none|List of secrets (without values for security)|
|» id|string|true|none|Unique secret identifier (handle)|
|» displayName|string|true|none|Human-readable display name|
|» createdAt|string(date-time)|true|none|Timestamp when the secret was created (UTC)|
|» updatedAt|string(date-time)|true|none|Timestamp when the secret was last updated (UTC)|
|totalCount|integer|false|none|Total number of secrets|
|status|string|false|none|none|

<h2 id="tocS_SecretDetailResponse">SecretDetailResponse</h2>

<a id="schemasecretdetailresponse"></a>
<a id="schema_SecretDetailResponse"></a>
<a id="tocSsecretdetailresponse"></a>
<a id="tocssecretdetailresponse"></a>

```json
{
  "status": "success",
  "secret": {
    "id": "database-password",
    "configuration": {
      "apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
      "kind": "Secret",
      "metadata": {
        "name": "weather-api-v1.0",
        "labels": {
          "environment": "production",
          "team": "backend",
          "version": "v1"
        }
      },
      "spec": {
        "displayName": "WSO2 OpenAI Key",
        "description": "WSO2 OpenAI provider API Key",
        "value": "sk_xxx"
      }
    },
    "metadata": {
      "createdAt": "2026-01-05T10:30:00Z",
      "updatedAt": "2026-01-05T11:45:00Z"
    }
  }
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|status|string|false|none|none|
|secret|object|false|none|none|
|» id|string|false|none|Unique secret identifier|
|» configuration|[SecretConfiguration](#schemasecretconfiguration)|false|none|none|
|» metadata|object|false|none|none|
|»» createdAt|string(date-time)|false|none|none|
|»» updatedAt|string(date-time)|false|none|none|
