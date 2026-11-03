<h1 id="gateway-controller-management-api-model-management">Model Management</h1>

CRUD operations for Models served on the gateway's model endpoints

## Create a new Model

<a id="opIdcreateModel"></a>

`POST /models`

> Code samples

```shell

curl -X POST http://localhost:9090/api/management/v1/models \
  -u {username}:{password} \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -d @payload.json

```

Add a Model to the Gateway. A Model is an AI model the gateway serves on the model endpoints it names in `spec.endpoints` (for example the gateway-provisioned `wso2`, by default at `/wso2/v1`): clients send the Model's `modelName` where the endpoint's inbound format carries the model, and the gateway routes the request to one of the Model's targets. The Model is not checked against its endpoints: one that an endpoint cannot serve is left out of that endpoint and the reason is logged. Requires `controller.models.enabled`.

> Payload

```json
{
  "apiVersion": "gateway.api-platform.wso2.com/v1",
  "kind": "Model",
  "metadata": {
    "name": "gpt-4o"
  },
  "spec": {
    "displayName": "GPT-4o",
    "version": "v1.0",
    "modelName": "gpt-4o",
    "endpoints": [
      "wso2"
    ],
    "targets": [
      {
        "name": "openai",
        "provider": {
          "id": "wso2-openai-provider"
        },
        "upstreamModel": "gpt-4o"
      }
    ]
  }
}
```

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

Required roles: `admin`

</aside>

<h3 id="create-a-new-model-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|[ModelConfigurationRequest](schemas.md#schemamodelconfigurationrequest)|true|Model in YAML or JSON format|

> Example responses
>
> 201 Response

```json
{
  "apiVersion": "gateway.api-platform.wso2.com/v1",
  "kind": "Model",
  "metadata": {
    "name": "gpt-4o"
  },
  "spec": {
    "displayName": "GPT-4o",
    "version": "v1.0",
    "modelName": "gpt-4o",
    "endpoints": [
      "wso2"
    ],
    "targets": [
      {
        "name": "openai",
        "provider": {
          "id": "wso2-openai-provider"
        },
        "upstreamModel": "gpt-4o"
      }
    ]
  },
  "status": {
    "id": "gpt-4o",
    "state": "deployed",
    "createdAt": "2026-10-21T07:21:13Z",
    "updatedAt": "2026-10-21T07:21:13Z",
    "deployedAt": "2026-10-21T07:21:13Z"
  }
}
```

<h3 id="create-a-new-model-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|201|[Created](https://tools.ietf.org/html/rfc7231#section-6.3.2)|Model created and deployed successfully|[ModelConfiguration](schemas.md#schemamodelconfiguration)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Invalid configuration (validation failed), or the models feature is disabled|[ErrorResponse](schemas.md#schemaerrorresponse)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Conflict - a Model with the same handle or modelName already exists|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

## List all Models

<a id="opIdlistModels"></a>

`GET /models`

> Code samples

```shell

curl -X GET http://localhost:9090/api/management/v1/models \
  -u {username}:{password} \
  -H 'Accept: application/json'

```

List Models registered in the Gateway, optionally filtered by display name, version, model name or status.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

Required roles: `admin`, `developer`

</aside>

<h3 id="list-all-models-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|displayName|query|string|false|Filter by Model displayName|
|version|query|string|false|Filter by Model version|
|modelName|query|string|false|Filter by the model name clients send|
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
  "count": 2,
  "models": [
    {
      "apiVersion": "gateway.api-platform.wso2.com/v1",
      "kind": "Model",
      "metadata": {
        "name": "gpt-4o"
      },
      "spec": {
        "displayName": "GPT-4o",
        "version": "v1.0",
        "modelName": "gpt-4o",
        "endpoints": [
          "wso2"
        ],
        "targets": [
          {
            "name": "openai",
            "provider": {
              "id": "wso2-openai-provider"
            },
            "upstreamModel": "gpt-4o"
          }
        ]
      },
      "status": {
        "id": "gpt-4o",
        "state": "deployed",
        "createdAt": "2026-10-21T07:21:13Z",
        "updatedAt": "2026-10-21T07:21:13Z",
        "deployedAt": "2026-10-21T07:21:13Z"
      }
    }
  ]
}
```

<h3 id="list-all-models-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|List of Models|Inline|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="list-all-models-responseschema">Response Schema</h3>

Status Code **200**

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|» status|string|false|none|none|
|» count|integer|false|none|none|
|» models|[allOf]|false|none|none|

*allOf*

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|»» *anonymous*|[ModelConfigurationRequest](schemas.md#schemamodelconfigurationrequest)|false|none|none|
|»»» apiVersion|string|true|none|Model specification version|
|»»» kind|string|true|none|Model kind|
|»»» metadata|[Metadata](schemas.md#schemametadata)|true|none|none|
|»»»» name|string|true|none|Unique handle for the resource|
|»»»» labels|object|false|none|Labels are key-value pairs for organizing and selecting APIs. Keys must not contain spaces.|
|»»»»» **additionalProperties**|string|false|none|none|
|»»»» annotations|object|false|none|Annotations are arbitrary non-identifying metadata. Use domain-prefixed keys.|
|»»»»» **additionalProperties**|string|false|none|none|
|»»» spec|[ModelConfigData](schemas.md#schemamodelconfigdata)|true|none|none|
|»»»» displayName|string|true|none|Human-readable Model name|
|»»»» version|string|true|none|Semantic version of the Model|
|»»»» modelName|string|true|none|The name clients send for this Model, wherever the endpoint's inbound format carries the model (the `model` field for OpenAI and Anthropic, the path for Gemini). Unique across Models on this gateway. Matched exactly and case-sensitively.|
|»»»» endpoints|[string]|true|none|Handles of the model endpoints that serve this Model. Required, with no implicit default. Each must name a model endpoint on this gateway, deployed or not; a save naming one that does not exist is refused with 400. A Model saved as deployed is also refused with 400 when a named endpoint cannot serve it, for example when it has no transformer for one of the Model's providers.|
|»»»» targets|[[ModelTarget](schemas.md#schemamodeltarget)]|true|none|The upstream models this Model can be served by. Each target is a pair of a deployed LLM provider and the model name that provider knows it by, and the pair is unique within the Model. With more than one target, `routing` is required to choose between them. The first target is the default: it serves a request the routing policy did not place.|
|»»»»» name|string|true|none|Name of this target, unique within the Model. A routing policy's model fields (`model`, `defaultModel`, `modelName`) may name the target by this name, as well as by its upstream model.|
|»»»»» provider|[ModelTargetProvider](schemas.md#schemamodeltargetprovider)|true|none|none|
|»»»»»» id|string|true|none|Handle of a deployed LLM provider|
|»»»»» upstreamModel|string|true|none|The model name sent to the provider. For an Azure OpenAI provider this is the deployment name.|
|»»»»» auth|[LLMUpstreamAuth](schemas.md#schemallmupstreamauth)|false|none|Credential the gateway presents to a provider reached through its own route (`exposure: route`) — the same field, with the same rules, as `auth` on an LLM proxy's provider. Required when that provider's route authenticates with api-key-auth; rejected for a provider with `exposure: none`, which is called directly with the provider's own upstream credential. Write-only values are never returned and are inherited on an update that omits them.|
|»»»»»» type|string|true|none|"api-key" attaches the built-in set-headers policy by default (overridable via policyName) and accepts either the generic policyParams bucket or its own deprecated header/value fields below. "oauth2" attaches the built-in oauth2-generator policy by default (overridable via policyName) and always requires policyParams - there is no typed-field fallback for it. "other" attaches any policy by name - policyName and policyParams are both required in that case, since there is no built-in default or typed-field fallback for a non-built-in auth scheme. "none": no upstream authentication - the gateway attaches no auth policy of its own; auth (if any) is handled entirely by user-attached policies elsewhere.|
|»»»»»» policyName|string|false|none|Name of the policy that implements this upstream auth. Optional for "api-key"/"oauth2" (defaults to the built-in policy for that type - api-key -> set-headers, oauth2 -> oauth2-generator); set it to point at your own fork or a newer major version's replacement instead. Required when type is "other".|
|»»»»»» policyVersion|string|false|none|Major version of policyName to attach (e.g. "v1"), same format and resolution rules as Policy.version. Optional - defaults to the highest version available in the gateway image when omitted. If set, it must match a version actually loaded in this gateway build, or config validation fails.|
|»»»»»» policyParams|object|false|none|Parameters passed verbatim to policyName (or the built-in default for type). Required when type is "oauth2" or "other" - oauth2 has no typed fields at all, only this bucket (e.g. {tokenEndpoint: ..., clientId: ..., clientSecret: ...} for the token-endpoint path, or {bearerToken: ...} for a directly-supplied credential). For "api-key", optional: replaces the deprecated header/value fields below when set; do not set both at once.|
|»»»»»» header|string|false|none|Deprecated: use policyParams (e.g. {request: {headers: [{name: ..., value: ...}]}} - the set-headers policy's own param shape) instead. HTTP header to set on outbound requests. Applies when type is api-key. Still honored when policyParams is omitted, for backward compatibility.|
|»»»»»» value|string|false|write-only|Deprecated: use policyParams instead. Upstream credential. Applies when type is api-key. Still honored when policyParams is omitted, for backward compatibility. Write-only: accepted on create/update and never returned by the management API on a read, for any role. An update that omits it inherits the stored value; set `type: none` to remove auth.|
|»»»» routing|[ModelRouting](schemas.md#schemamodelrouting)|false|none|The routing policy that chooses a target per request. Any model routing policy can be used (round-robin, weighted round-robin, semantic, cost-based, time-based, intelligent routing, header routing). Its parameters are the policy's own, written as the policy defines them. A parameter that names a model (`model`, `defaultModel`, `modelName`) may give a target's name or its upstream model; one that names a provider gives a provider handle. At request time the gateway matches the policy's choice to one of the Model's targets, and a choice that matches none is served by the first target.|
|»»»»» policy|[ModelRoutingPolicy](schemas.md#schemamodelroutingpolicy)|true|none|none|
|»»»»»» name|string|true|none|Routing policy name|
|»»»»»» version|string|true|none|Major-only routing policy version|
|»»»»» params|object|false|none|Parameters for the routing policy, exactly as the policy defines them, and checked against its parameter schema on save.|
|»»»» policies|[[Policy](schemas.md#schemapolicy)]|false|none|Policies applied to every request for this Model, after the endpoint's own policies (authentication) and before routing. Policies here see the request and response in each endpoint's inbound format.|
|»»»»» name|string|true|none|Name of the policy|
|»»»»» version|string|true|none|Version of the policy. Only major-only version is allowed (e.g., v0, v1). Full semantic version (e.g., v1.0.0) is not accepted and will be rejected. The Gateway Controller resolves the major version to the single matching full version installed in the gateway image.|
|»»»»» executionCondition|string|false|none|Expression controlling conditional execution of the policy|
|»»»»» params|object|false|none|Arbitrary parameters for the policy (free-form key/value structure)|
|»»»» faultPolicies|[[Policy](schemas.md#schemapolicy)]|false|none|Ordered list of policies executed only when a request for this Model fails, over the error response. Run before the endpoint's own fault policies.|
|»»»» deploymentState|string|false|none|Desired deployment state - 'deployed' (default) or 'undeployed'. An undeployed Model stops resolving on every endpoint but its configuration is kept.|

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
|kind|Model|
|type|api-key|
|type|oauth2|
|type|other|
|type|none|
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

## Get Model by unique identifier

<a id="opIdgetModelById"></a>

`GET /models/{id}`

> Code samples

```shell

curl -X GET http://localhost:9090/api/management/v1/models/{id} \
  -u {username}:{password} \
  -H 'Accept: application/json'

```

Get a Model by its ID.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

Required roles: `admin`, `developer`

</aside>

<h3 id="get-model-by-unique-identifier-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|string|true|Unique identifier of the Model|

> Example responses
>
> 200 Response

```json
{
  "apiVersion": "gateway.api-platform.wso2.com/v1",
  "kind": "Model",
  "metadata": {
    "name": "gpt-4o"
  },
  "spec": {
    "displayName": "GPT-4o",
    "version": "v1.0",
    "modelName": "gpt-4o",
    "endpoints": [
      "wso2"
    ],
    "targets": [
      {
        "name": "openai",
        "provider": {
          "id": "wso2-openai-provider"
        },
        "upstreamModel": "gpt-4o"
      }
    ]
  },
  "status": {
    "id": "gpt-4o",
    "state": "deployed",
    "createdAt": "2026-10-21T07:21:13Z",
    "updatedAt": "2026-10-21T07:21:13Z",
    "deployedAt": "2026-10-21T07:21:13Z"
  }
}
```

<h3 id="get-model-by-unique-identifier-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Model details|[ModelConfiguration](schemas.md#schemamodelconfiguration)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Model not found|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

## Update an existing Model

<a id="opIdupdateModel"></a>

`PUT /models/{id}`

> Code samples

```shell

curl -X PUT http://localhost:9090/api/management/v1/models/{id} \
  -u {username}:{password} \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -d @payload.json

```

Update an existing Model in the Gateway.

> Payload

```json
{
  "apiVersion": "gateway.api-platform.wso2.com/v1",
  "kind": "Model",
  "metadata": {
    "name": "gpt-4o"
  },
  "spec": {
    "displayName": "GPT-4o",
    "version": "v1.0",
    "modelName": "gpt-4o",
    "endpoints": [
      "wso2"
    ],
    "targets": [
      {
        "name": "openai",
        "provider": {
          "id": "wso2-openai-provider"
        },
        "upstreamModel": "gpt-4o"
      }
    ]
  }
}
```

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

Required roles: `admin`

</aside>

<h3 id="update-an-existing-model-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|string|true|Unique identifier of the Model|
|body|body|[ModelConfigurationRequest](schemas.md#schemamodelconfigurationrequest)|true|Updated Model|

> Example responses
>
> 200 Response

```json
{
  "apiVersion": "gateway.api-platform.wso2.com/v1",
  "kind": "Model",
  "metadata": {
    "name": "gpt-4o"
  },
  "spec": {
    "displayName": "GPT-4o",
    "version": "v1.0",
    "modelName": "gpt-4o",
    "endpoints": [
      "wso2"
    ],
    "targets": [
      {
        "name": "openai",
        "provider": {
          "id": "wso2-openai-provider"
        },
        "upstreamModel": "gpt-4o"
      }
    ]
  },
  "status": {
    "id": "gpt-4o",
    "state": "deployed",
    "createdAt": "2026-10-21T07:21:13Z",
    "updatedAt": "2026-10-21T07:21:13Z",
    "deployedAt": "2026-10-21T07:21:13Z"
  }
}
```

<h3 id="update-an-existing-model-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Model updated successfully|[ModelConfiguration](schemas.md#schemamodelconfiguration)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Invalid configuration, or the models feature is disabled|[ErrorResponse](schemas.md#schemaerrorresponse)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Model not found|[ErrorResponse](schemas.md#schemaerrorresponse)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Conflict - another Model already uses this modelName|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

## Delete a Model

<a id="opIddeleteModel"></a>

`DELETE /models/{id}`

> Code samples

```shell

curl -X DELETE http://localhost:9090/api/management/v1/models/{id} \
  -u {username}:{password} \
  -H 'Accept: application/json'

```

Delete a Model and its API keys. Its model name stops resolving on every model endpoint.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

Required roles: `admin`

</aside>

<h3 id="delete-a-model-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|string|true|Unique identifier of the Model|

> Example responses
>
> 200 Response

```json
{
  "status": "success",
  "message": "Model deleted successfully",
  "id": "gpt-4o"
}
```

<h3 id="delete-a-model-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Model deleted successfully|Inline|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Model not found|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="delete-a-model-responseschema">Response Schema</h3>

Status Code **200**

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|» status|string|false|none|none|
|» message|string|false|none|none|
|» id|string|false|none|none|

## Create a new API key for a Model

<a id="opIdcreateModelAPIKey"></a>

`POST /models/{id}/api-keys`

> Code samples

```shell

curl -X POST http://localhost:9090/api/management/v1/models/{id}/api-keys \
  -u {username}:{password} \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -d @payload.json

```

Generate a new API key for a Model, or register one with an explicit value.

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

<h3 id="create-a-new-api-key-for-a-model-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|string|true|Unique handle of a Model|
|body|body|[APIKeyCreationRequest](schemas.md#schemaapikeycreationrequest)|true|none|

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

<h3 id="create-a-new-api-key-for-a-model-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|201|[Created](https://tools.ietf.org/html/rfc7231#section-6.3.2)|API key created successfully|[APIKeyCreationResponse](schemas.md#schemaapikeycreationresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Invalid configuration (validation failed)|[ErrorResponse](schemas.md#schemaerrorresponse)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|A Model not found|[ErrorResponse](schemas.md#schemaerrorresponse)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Conflict (duplicate key, conflicting update, or no endpoint that could check the key)|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

## List the API keys of a Model

<a id="opIdlistModelAPIKeys"></a>

`GET /models/{id}/api-keys`

> Code samples

```shell

curl -X GET http://localhost:9090/api/management/v1/models/{id}/api-keys \
  -u {username}:{password} \
  -H 'Accept: application/json'

```

List all API keys issued for a Model.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

Required roles: `admin`, `consumer`

</aside>

<h3 id="list-the-api-keys-of-a-model-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|string|true|Unique handle of a Model|

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

<h3 id="list-the-api-keys-of-a-model-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|List of API keys|[APIKeyListResponse](schemas.md#schemaapikeylistresponse)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|A Model not found|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

## Regenerate an API key of a Model

<a id="opIdregenerateModelAPIKey"></a>

`POST /models/{id}/api-keys/{apiKeyName}/regenerate`

> Code samples

```shell

curl -X POST http://localhost:9090/api/management/v1/models/{id}/api-keys/{apiKeyName}/regenerate \
  -u {username}:{password} \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -d @payload.json

```

Regenerate an existing API key of a Model.

> Payload

```json
{}
```

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

Required roles: `admin`, `consumer`

</aside>

<h3 id="regenerate-an-api-key-of-a-model-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|string|true|Unique handle of a Model|
|apiKeyName|path|string|true|Name of the API key to regenerate|
|body|body|[APIKeyRegenerationRequest](schemas.md#schemaapikeyregenerationrequest)|true|none|

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

<h3 id="regenerate-an-api-key-of-a-model-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|API key rotated successfully|[APIKeyCreationResponse](schemas.md#schemaapikeycreationresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Invalid configuration (validation failed)|[ErrorResponse](schemas.md#schemaerrorresponse)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|A Model or API key not found|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

## Update an API key of a Model

<a id="opIdupdateModelAPIKey"></a>

`PUT /models/{id}/api-keys/{apiKeyName}`

> Code samples

```shell

curl -X PUT http://localhost:9090/api/management/v1/models/{id}/api-keys/{apiKeyName} \
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

<h3 id="update-an-api-key-of-a-model-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|string|true|Unique handle of a Model|
|apiKeyName|path|string|true|Name of the API key to update|
|body|body|[APIKeyUpdateRequest](schemas.md#schemaapikeyupdaterequest)|true|none|

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

<h3 id="update-an-api-key-of-a-model-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|API key updated successfully|[APIKeyCreationResponse](schemas.md#schemaapikeycreationresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Invalid request (validation failed)|[ErrorResponse](schemas.md#schemaerrorresponse)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|A Model or API key not found|[ErrorResponse](schemas.md#schemaerrorresponse)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Conflict (duplicate key or conflicting update)|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

## Revoke an API key of a Model

<a id="opIdrevokeModelAPIKey"></a>

`DELETE /models/{id}/api-keys/{apiKeyName}`

> Code samples

```shell

curl -X DELETE http://localhost:9090/api/management/v1/models/{id}/api-keys/{apiKeyName} \
  -u {username}:{password} \
  -H 'Accept: application/json'

```

Revoke an API key. Once revoked, it can no longer be used to authenticate requests.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

Required roles: `admin`, `consumer`

</aside>

<h3 id="revoke-an-api-key-of-a-model-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|string|true|Unique handle of a Model|
|apiKeyName|path|string|true|Name of the API key to revoke|

> Example responses
>
> 200 Response

```json
{
  "status": "success",
  "message": "API key revoked successfully"
}
```

<h3 id="revoke-an-api-key-of-a-model-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|API key revoked successfully|[APIKeyRevocationResponse](schemas.md#schemaapikeyrevocationresponse)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|A Model or API key not found|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|
