<h1 id="gateway-controller-management-api-model-endpoint-management">Model Endpoint Management</h1>

CRUD operations for model endpoints, which serve Models to clients

## Create a new model endpoint

<a id="opIdcreateModelEndpoint"></a>

`POST /model-endpoints`

> Code samples

```shell

curl -X POST http://localhost:9090/api/management/v1/model-endpoints \
  -u {username}:{password} \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -d @payload.json

```

Add a model endpoint to the Gateway. A model endpoint is the client-facing API that serves Models: a request names a Model (in the body or the path, as the endpoint's inbound template says) and is dispatched to that Model's own policy chain. The handle `wso2` is reserved for the endpoint the gateway provisions when `controller.models.endpoint.provision` is set. Requires `controller.models.enabled`.

> Payload

```json
{
  "apiVersion": "gateway.api-platform.wso2.com/v1",
  "kind": "ModelEndpoint",
  "metadata": {
    "name": "partners"
  },
  "spec": {
    "displayName": "Partner Model Endpoint",
    "version": "v1.0",
    "context": "/partners/v1",
    "policies": [
      {
        "name": "api-key-auth",
        "version": "v1",
        "params": {
          "key": "Authorization",
          "in": "header",
          "valuePrefix": "Bearer"
        }
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

<h3 id="create-a-new-model-endpoint-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|[ModelEndpointConfigurationRequest](schemas.md#schemamodelendpointconfigurationrequest)|true|Model endpoint in YAML or JSON format|

> Example responses
>
> 201 Response

```json
{
  "apiVersion": "gateway.api-platform.wso2.com/v1",
  "kind": "ModelEndpoint",
  "metadata": {
    "name": "partners"
  },
  "spec": {
    "displayName": "Partner Model Endpoint",
    "version": "v1.0",
    "context": "/partners/v1",
    "policies": [
      {
        "name": "api-key-auth",
        "version": "v1",
        "params": {
          "key": "Authorization",
          "in": "header",
          "valuePrefix": "Bearer"
        }
      }
    ]
  },
  "status": {
    "id": "partners",
    "state": "deployed",
    "createdAt": "2026-10-28T09:12:44Z",
    "updatedAt": "2026-10-28T09:40:02Z",
    "deployedAt": "2026-10-28T09:40:02Z"
  }
}
```

<h3 id="create-a-new-model-endpoint-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|201|[Created](https://tools.ietf.org/html/rfc7231#section-6.3.2)|Model endpoint created and deployed successfully|[ModelEndpointConfiguration](schemas.md#schemamodelendpointconfiguration)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Invalid configuration (validation failed), or the models feature is disabled|[ErrorResponse](schemas.md#schemaerrorresponse)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Conflict - an endpoint with the same handle or display name exists, the context and vhost are already used, or the handle is the reserved `wso2`|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

## List all model endpoints

<a id="opIdlistModelEndpoints"></a>

`GET /model-endpoints`

> Code samples

```shell

curl -X GET http://localhost:9090/api/management/v1/model-endpoints \
  -u {username}:{password} \
  -H 'Accept: application/json'

```

List the model endpoints on the Gateway, optionally filtered by display name, version, context, status or vhost.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

Required roles: `admin`, `developer`

</aside>

<h3 id="list-all-model-endpoints-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|displayName|query|string|false|Filter by display name|
|version|query|string|false|Filter by version|
|context|query|string|false|Filter by context|
|status|query|string|false|Filter by deployment status|
|vhost|query|string|false|Filter by vhost|

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
  "modelEndpoints": [
    {
      "apiVersion": "gateway.api-platform.wso2.com/v1",
      "kind": "ModelEndpoint",
      "metadata": {
        "name": "partners"
      },
      "spec": {
        "displayName": "Partner Model Endpoint",
        "version": "v1.0",
        "context": "/partners/v1",
        "policies": [
          {
            "name": "api-key-auth",
            "version": "v1",
            "params": {
              "key": "Authorization",
              "in": "header",
              "valuePrefix": "Bearer"
            }
          }
        ]
      },
      "status": {
        "id": "partners",
        "state": "deployed",
        "createdAt": "2026-10-28T09:12:44Z",
        "updatedAt": "2026-10-28T09:40:02Z",
        "deployedAt": "2026-10-28T09:40:02Z"
      }
    }
  ]
}
```

<h3 id="list-all-model-endpoints-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|List of model endpoints|Inline|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="list-all-model-endpoints-responseschema">Response Schema</h3>

Status Code **200**

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|» status|string|false|none|none|
|» count|integer|false|none|none|
|» modelEndpoints|[allOf]|false|none|none|

*allOf*

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|»» *anonymous*|[ModelEndpointConfigurationRequest](schemas.md#schemamodelendpointconfigurationrequest)|false|none|none|
|»»» apiVersion|string|true|none|Model endpoint specification version|
|»»» kind|string|true|none|Model endpoint kind|
|»»» metadata|[Metadata](schemas.md#schemametadata)|true|none|none|
|»»»» name|string|true|none|Unique handle for the resource|
|»»»» labels|object|false|none|Labels are key-value pairs for organizing and selecting APIs. Keys must not contain spaces.|
|»»»»» **additionalProperties**|string|false|none|none|
|»»»» annotations|object|false|none|Annotations are arbitrary non-identifying metadata. Use domain-prefixed keys.|
|»»»»» **additionalProperties**|string|false|none|none|
|»»» spec|[ModelEndpointConfigData](schemas.md#schemamodelendpointconfigdata)|true|none|none|
|»»»» displayName|string|true|none|Human-readable endpoint name, unique among the gateway's model endpoints (API-level rate-limit counters key on it). Only letters, numbers, spaces, hyphens, underscores and dots.|
|»»»» version|string|true|none|Semantic version of the endpoint|
|»»»» context|string|false|none|Base path the endpoint is served at (must start with /, no trailing slash). Required for every endpoint but the provisioned `wso2`, whose context is set by `controller.models.endpoint.context`. The context and vhost must not clash with any other artifact on the gateway.|
|»»»» vhost|string|false|none|Virtual host the endpoint is served on; the gateway's default vhost when absent. Wildcards are only allowed in the left-most label.|
|»»»» inboundTemplate|string|false|none|Handle of the provider template describing the wire format clients send, as on an LLM proxy. The model is read where the template's `requestModel` says. `openai` (the default), `anthropic` and `gemini` are accepted: the formats the gateway has an inbound profile for, which fixes the endpoint's operations, catalog and error bodies.|
|»»»» policies|[[Policy](schemas.md#schemapolicy)]|false|none|Endpoint-wide policies, run first in every Model chain and in the catalog chain. Attached at API level, so an endpoint rate limit is shared by every Model. When omitted, defaults to api-key-auth on `Authorization: Bearer <key>`. An explicit list is used as given, so an empty list serves the endpoint without authentication.|
|»»»»» name|string|true|none|Name of the policy|
|»»»»» version|string|true|none|Version of the policy. Only major-only version is allowed (e.g., v0, v1). Full semantic version (e.g., v1.0.0) is not accepted and will be rejected. The Gateway Controller resolves the major version to the single matching full version installed in the gateway image.|
|»»»»» executionCondition|string|false|none|Expression controlling conditional execution of the policy|
|»»»»» params|object|false|none|Arbitrary parameters for the policy (free-form key/value structure)|
|»»»» faultPolicies|[[Policy](schemas.md#schemapolicy)]|false|none|Policies executed when any request on the endpoint fails, over the error response; run after a Model's own fault policies.|
|»»»» transformers|[[ModelEndpointTransformer](schemas.md#schemamodelendpointtransformer)]|false|none|The transformers that convert this endpoint's inbound format to a provider's. There are no built-in defaults: a provider whose template is the endpoint's inbound template needs none, and any other provider needs an entry here, matched by provider handle first and template second. A Model with a provider no entry matches is left out of this endpoint.|
|»»»»» provider|string|false|none|LLM provider handle this entry applies to. Wins over a `template` entry.|
|»»»»» template|string|false|none|Provider template handle or template group this entry applies to.|
|»»»»» type|string|true|none|An installed transformer policy name, or `none` for no transformer (a provider that already speaks the endpoint's format).|
|»»»»» version|string|false|none|Major-only policy version, required unless `type` is `none`.|
|»»»»» params|object|false|none|Parameters passed to the transformer. `providerId` is injected by the gateway and cannot be set. String values may use `{apiVersion}` or `{apiVersion:<default>}`, filled from the provider's `spec.upstream.apiVersion`.|
|»»»»» upstreamPaths|[string]|false|none|The paths the transformer writes upstream, with `{upstreamModel}` and `{apiVersion}` placeholders. Used only to check that a provider reached through its own route allows them (accessControl).|
|»»»» resilience|[Resilience](schemas.md#schemaresilience)|false|none|Backend/route timeout configuration. Maps to Envoy RouteAction timeouts. Can be set at the API level (applies to all routes) and/or the operation level (applies to that operation's route). When set at both levels, the operation-level value takes precedence. When unset, the gateway's global route timeout defaults apply.|
|»»»»» timeout|string|false|none|Maximum time for the entire route (request to upstream response). "0s" disables the timeout.|
|»»»»» idleTimeout|string|false|none|Per-route stream idle timeout (overrides the listener stream idle timeout for this route). "0s" disables the timeout.|
|»»»» deploymentState|string|false|none|Desired deployment state - 'deployed' (default) or 'undeployed'. An undeployed endpoint serves nothing; its Models and keys are kept, and Models keep serving on their other endpoints.|

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
|kind|ModelEndpoint|
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

## Get a model endpoint by handle

<a id="opIdgetModelEndpointById"></a>

`GET /model-endpoints/{id}`

> Code samples

```shell

curl -X GET http://localhost:9090/api/management/v1/model-endpoints/{id} \
  -u {username}:{password} \
  -H 'Accept: application/json'

```

Get a model endpoint by its handle.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

Required roles: `admin`, `developer`

</aside>

<h3 id="get-a-model-endpoint-by-handle-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|string|true|Unique handle of the model endpoint|

> Example responses
>
> 200 Response

```json
{
  "apiVersion": "gateway.api-platform.wso2.com/v1",
  "kind": "ModelEndpoint",
  "metadata": {
    "name": "partners"
  },
  "spec": {
    "displayName": "Partner Model Endpoint",
    "version": "v1.0",
    "context": "/partners/v1",
    "policies": [
      {
        "name": "api-key-auth",
        "version": "v1",
        "params": {
          "key": "Authorization",
          "in": "header",
          "valuePrefix": "Bearer"
        }
      }
    ]
  },
  "status": {
    "id": "partners",
    "state": "deployed",
    "createdAt": "2026-10-28T09:12:44Z",
    "updatedAt": "2026-10-28T09:40:02Z",
    "deployedAt": "2026-10-28T09:40:02Z"
  }
}
```

<h3 id="get-a-model-endpoint-by-handle-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Model endpoint details|[ModelEndpointConfiguration](schemas.md#schemamodelendpointconfiguration)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Model endpoint not found|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

## Update an existing model endpoint

<a id="opIdupdateModelEndpoint"></a>

`PUT /model-endpoints/{id}`

> Code samples

```shell

curl -X PUT http://localhost:9090/api/management/v1/model-endpoints/{id} \
  -u {username}:{password} \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -d @payload.json

```

Update a model endpoint. Every Model the endpoint serves is recompiled; an update that would leave a Model it serves today unable to compile (for example by removing the transformer that Model's provider needs) is refused, naming those Models. The context of the provisioned `wso2` endpoint is set by `controller.models.endpoint.context` and cannot be changed here.

> Payload

```json
{
  "apiVersion": "gateway.api-platform.wso2.com/v1",
  "kind": "ModelEndpoint",
  "metadata": {
    "name": "partners"
  },
  "spec": {
    "displayName": "Partner Model Endpoint",
    "version": "v1.0",
    "context": "/partners/v1",
    "policies": [
      {
        "name": "api-key-auth",
        "version": "v1",
        "params": {
          "key": "Authorization",
          "in": "header",
          "valuePrefix": "Bearer"
        }
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

<h3 id="update-an-existing-model-endpoint-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|string|true|Unique handle of the model endpoint|
|body|body|[ModelEndpointConfigurationRequest](schemas.md#schemamodelendpointconfigurationrequest)|true|Updated model endpoint|

> Example responses
>
> 200 Response

```json
{
  "apiVersion": "gateway.api-platform.wso2.com/v1",
  "kind": "ModelEndpoint",
  "metadata": {
    "name": "partners"
  },
  "spec": {
    "displayName": "Partner Model Endpoint",
    "version": "v1.0",
    "context": "/partners/v1",
    "policies": [
      {
        "name": "api-key-auth",
        "version": "v1",
        "params": {
          "key": "Authorization",
          "in": "header",
          "valuePrefix": "Bearer"
        }
      }
    ]
  },
  "status": {
    "id": "partners",
    "state": "deployed",
    "createdAt": "2026-10-28T09:12:44Z",
    "updatedAt": "2026-10-28T09:40:02Z",
    "deployedAt": "2026-10-28T09:40:02Z"
  }
}
```

<h3 id="update-an-existing-model-endpoint-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Model endpoint updated successfully|[ModelEndpointConfiguration](schemas.md#schemamodelendpointconfiguration)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Invalid configuration, or the models feature is disabled|[ErrorResponse](schemas.md#schemaerrorresponse)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Model endpoint not found|[ErrorResponse](schemas.md#schemaerrorresponse)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Conflict - the update would break a Model the endpoint serves, the context and vhost are already used, or it changes the provisioned endpoint's context|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

## Delete a model endpoint

<a id="opIddeleteModelEndpoint"></a>

`DELETE /model-endpoints/{id}`

> Code samples

```shell

curl -X DELETE http://localhost:9090/api/management/v1/model-endpoints/{id} \
  -u {username}:{password} \
  -H 'Accept: application/json'

```

Delete a model endpoint and its API keys. Refused while a deployed Model names the endpoint in `spec.endpoints`, and for the provisioned `wso2` endpoint while `controller.models.endpoint.provision` is set.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

Required roles: `admin`

</aside>

<h3 id="delete-a-model-endpoint-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|string|true|Unique handle of the model endpoint|

> Example responses
>
> 200 Response

```json
{
  "status": "success",
  "message": "Model endpoint deleted successfully",
  "id": "partners"
}
```

<h3 id="delete-a-model-endpoint-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Model endpoint deleted successfully|Inline|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Model endpoint not found|[ErrorResponse](schemas.md#schemaerrorresponse)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|A deployed Model names the endpoint, or it is the provisioned `wso2` endpoint|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="delete-a-model-endpoint-responseschema">Response Schema</h3>

Status Code **200**

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|» status|string|false|none|none|
|» message|string|false|none|none|
|» id|string|false|none|none|

## Create a new API key for a model endpoint

<a id="opIdcreateModelEndpointAPIKey"></a>

`POST /model-endpoints/{id}/api-keys`

> Code samples

```shell

curl -X POST http://localhost:9090/api/management/v1/model-endpoints/{id}/api-keys \
  -u {username}:{password} \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -d @payload.json

```

Generate a new API key for a model endpoint, or register one with an explicit value.

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

<h3 id="create-a-new-api-key-for-a-model-endpoint-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|string|true|Unique handle of a model endpoint|
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

<h3 id="create-a-new-api-key-for-a-model-endpoint-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|201|[Created](https://tools.ietf.org/html/rfc7231#section-6.3.2)|API key created successfully|[APIKeyCreationResponse](schemas.md#schemaapikeycreationresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Invalid configuration (validation failed)|[ErrorResponse](schemas.md#schemaerrorresponse)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|A model endpoint not found|[ErrorResponse](schemas.md#schemaerrorresponse)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Conflict (duplicate key, conflicting update, or no endpoint that could check the key)|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

## List the API keys of a model endpoint

<a id="opIdlistModelEndpointAPIKeys"></a>

`GET /model-endpoints/{id}/api-keys`

> Code samples

```shell

curl -X GET http://localhost:9090/api/management/v1/model-endpoints/{id}/api-keys \
  -u {username}:{password} \
  -H 'Accept: application/json'

```

List all API keys issued for a model endpoint.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

Required roles: `admin`, `consumer`

</aside>

<h3 id="list-the-api-keys-of-a-model-endpoint-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|string|true|Unique handle of a model endpoint|

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

<h3 id="list-the-api-keys-of-a-model-endpoint-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|List of API keys|[APIKeyListResponse](schemas.md#schemaapikeylistresponse)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|A model endpoint not found|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

## Regenerate an API key of a model endpoint

<a id="opIdregenerateModelEndpointAPIKey"></a>

`POST /model-endpoints/{id}/api-keys/{apiKeyName}/regenerate`

> Code samples

```shell

curl -X POST http://localhost:9090/api/management/v1/model-endpoints/{id}/api-keys/{apiKeyName}/regenerate \
  -u {username}:{password} \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -d @payload.json

```

Regenerate an existing API key of a model endpoint.

> Payload

```json
{}
```

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

Required roles: `admin`, `consumer`

</aside>

<h3 id="regenerate-an-api-key-of-a-model-endpoint-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|string|true|Unique handle of a model endpoint|
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

<h3 id="regenerate-an-api-key-of-a-model-endpoint-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|API key rotated successfully|[APIKeyCreationResponse](schemas.md#schemaapikeycreationresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Invalid configuration (validation failed)|[ErrorResponse](schemas.md#schemaerrorresponse)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|A model endpoint or API key not found|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

## Update an API key of a model endpoint

<a id="opIdupdateModelEndpointAPIKey"></a>

`PUT /model-endpoints/{id}/api-keys/{apiKeyName}`

> Code samples

```shell

curl -X PUT http://localhost:9090/api/management/v1/model-endpoints/{id}/api-keys/{apiKeyName} \
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

<h3 id="update-an-api-key-of-a-model-endpoint-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|string|true|Unique handle of a model endpoint|
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

<h3 id="update-an-api-key-of-a-model-endpoint-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|API key updated successfully|[APIKeyCreationResponse](schemas.md#schemaapikeycreationresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Invalid request (validation failed)|[ErrorResponse](schemas.md#schemaerrorresponse)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|A model endpoint or API key not found|[ErrorResponse](schemas.md#schemaerrorresponse)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Conflict (duplicate key or conflicting update)|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

## Revoke an API key of a model endpoint

<a id="opIdrevokeModelEndpointAPIKey"></a>

`DELETE /model-endpoints/{id}/api-keys/{apiKeyName}`

> Code samples

```shell

curl -X DELETE http://localhost:9090/api/management/v1/model-endpoints/{id}/api-keys/{apiKeyName} \
  -u {username}:{password} \
  -H 'Accept: application/json'

```

Revoke an API key. Once revoked, it can no longer be used to authenticate requests.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

Required roles: `admin`, `consumer`

</aside>

<h3 id="revoke-an-api-key-of-a-model-endpoint-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|string|true|Unique handle of a model endpoint|
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

<h3 id="revoke-an-api-key-of-a-model-endpoint-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|API key revoked successfully|[APIKeyRevocationResponse](schemas.md#schemaapikeyrevocationresponse)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|A model endpoint or API key not found|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|
