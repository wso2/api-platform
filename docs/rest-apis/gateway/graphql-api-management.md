<h1 id="gateway-controller-management-api-graphql-api-management">GraphQL API Management</h1>

CRUD operations for GraphQL APIs

## Create a new GraphQLApi

<a id="opIdcreateGraphQLAPI"></a>

`POST /graphql-apis`

> Code samples

```shell

curl -X POST http://localhost:9090/api/management/v1/graphql-apis \
  -u {username}:{password} \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -d @payload.json

```

Add a new GraphQLApi to the Gateway.

> Payload

```json
{
  "apiVersion": "gateway.api-platform.wso2.com/v1",
  "kind": "GraphQLApi",
  "metadata": {
    "name": "countries-graphql-api-v1.0"
  },
  "spec": {
    "displayName": "Countries-GraphQL-API",
    "version": "v1.0",
    "context": "/countries/$version/graphql",
    "upstream": {
      "main": {
        "url": "https://countries.trevorblades.com/graphql"
      }
    },
    "policies": [
      {
        "name": "jwt-auth",
        "version": "v1"
      }
    ]
  }
}
```

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

Required roles: `admin`, `developer`

</aside>

<h3 id="create-a-new-graphqlapi-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|[GraphQLAPIRequest](schemas.md#schemagraphqlapirequest)|true|none|

> Example responses
>
> 201 Response

```json
{
  "apiVersion": "gateway.api-platform.wso2.com/v1",
  "kind": "GraphQLApi",
  "metadata": {
    "name": "countries-graphql-api-v1.0"
  },
  "spec": {
    "displayName": "Countries-GraphQL-API",
    "version": "v1.0",
    "context": "/countries/$version/graphql",
    "upstream": {
      "main": {
        "url": "https://countries.trevorblades.com/graphql"
      }
    },
    "policies": [
      {
        "name": "jwt-auth",
        "version": "v1"
      }
    ]
  },
  "status": {
    "id": "countries-graphql-api-v1.0",
    "state": "deployed",
    "createdAt": "2026-08-11T10:00:00Z",
    "updatedAt": "2026-08-11T10:00:00Z",
    "deployedAt": "2026-08-11T10:00:00Z"
  }
}
```

<h3 id="create-a-new-graphqlapi-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|201|[Created](https://tools.ietf.org/html/rfc7231#section-6.3.2)|GraphQLApi created successfully|[GraphQLAPI](schemas.md#schemagraphqlapi)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Invalid configuration (validation failed)|[ErrorResponse](schemas.md#schemaerrorresponse)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Conflict - API with same name and version already exists|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

## List all GraphQLApis

<a id="opIdlistGraphQLAPIs"></a>

`GET /graphql-apis`

> Code samples

```shell

curl -X GET http://localhost:9090/api/management/v1/graphql-apis \
  -u {username}:{password} \
  -H 'Accept: application/json'

```

List GraphQLApis registered in the Gateway, optionally filtered by name, version, context, or status.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

Required roles: `admin`, `developer`

</aside>

<h3 id="list-all-graphqlapis-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|displayName|query|string|false|Filter by API display name|
|version|query|string|false|Filter by API version|
|context|query|string|false|Filter by API context/path|
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
  "count": 1,
  "graphqlApis": [
    {
      "apiVersion": "gateway.api-platform.wso2.com/v1",
      "kind": "GraphQLApi",
      "metadata": {
        "name": "countries-graphql-api-v1.0"
      },
      "spec": {
        "displayName": "Countries-GraphQL-API",
        "version": "v1.0",
        "context": "/countries/$version/graphql",
        "upstream": {
          "main": {
            "url": "https://countries.trevorblades.com/graphql"
          }
        },
        "policies": [
          {
            "name": "jwt-auth",
            "version": "v1"
          }
        ]
      },
      "status": {
        "id": "countries-graphql-api-v1.0",
        "state": "deployed",
        "createdAt": "2026-08-11T10:00:00Z",
        "updatedAt": "2026-08-11T10:00:00Z",
        "deployedAt": "2026-08-11T10:00:00Z"
      }
    }
  ]
}
```

<h3 id="list-all-graphqlapis-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|List of GraphQLApis|Inline|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="list-all-graphqlapis-responseschema">Response Schema</h3>

Status Code **200**

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|» status|string|false|none|none|
|» count|integer|false|none|none|
|» graphqlApis|[allOf]|false|none|none|

*allOf*

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|»» *anonymous*|[GraphQLAPIRequest](schemas.md#schemagraphqlapirequest)|false|none|none|
|»»» apiVersion|string|true|none|API specification version|
|»»» kind|string|true|none|API type|
|»»» metadata|[Metadata](schemas.md#schemametadata)|true|none|none|
|»»»» name|string|true|none|Unique handle for the resource|
|»»»» labels|object|false|none|Labels are key-value pairs for organizing and selecting APIs. Keys must not contain spaces.|
|»»»»» **additionalProperties**|string|false|none|none|
|»»»» annotations|object|false|none|Annotations are arbitrary non-identifying metadata. Use domain-prefixed keys.|
|»»»»» **additionalProperties**|string|false|none|none|
|»»» spec|[GraphQLAPIConfigData](schemas.md#schemagraphqlapiconfigdata)|true|none|none|
|»»»» displayName|string|true|none|Human-readable API name (must be URL-friendly - only letters, numbers, spaces, hyphens, underscores, and dots allowed)|
|»»»» version|string|true|none|Semantic version of the API. Both major-only (v1) and major.minor (v1.0) forms are accepted.|
|»»»» context|string|true|none|Base path for the single GraphQL endpoint (must start with /, no trailing slash). Use $version to embed the version in the path (e.g., /countries/$version resolves to /countries/v1.0). A GraphQLApi always exposes exactly one POST route at this path — there is no per-operation path list. Suggested (not enforced) convention: end the path with /graphql, matching how most standalone GraphQL servers name their single endpoint (e.g. /countries/$version/graphql) — this is not validated or required.|
|»»»» upstream|object|true|none|API-level upstream configuration. A GraphQLApi has exactly one logical endpoint (no per-operation paths), so upstream.main.url is the single GraphQL endpoint to proxy to. Only a direct inline url is supported — GraphQLAPIConfigData has no upstreamDefinitions list, so upstream.ref (used by RestApi to reference a predefined upstreamDefinition) cannot be resolved and is rejected.|
|»»»»» main|[Upstream](schemas.md#schemaupstream)|true|none|Upstream backend configuration (single target or reference)|
|»»»»»» url|string(uri)|false|none|Direct backend URL to route traffic to|
|»»»»»» ref|string|false|none|Reference to a predefined upstreamDefinition|
|»»»»»» hostRewrite|string|false|none|Controls how the Host header is handled when routing to the upstream. `auto` delegates host rewriting to Envoy, which rewrites the Host header using the upstream cluster host. `manual` disables automatic rewriting and expects explicit configuration.|
|»»»»»» tls|object|false|none|Not supported on an inline upstream (a `url`-based main/sandbox entry); a tls block placed here is rejected at deploy time. Put it on the upstreamDefinitions entry referenced via `ref`.|

*oneOf*

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|»»»»»» *anonymous*|object|false|none|none|

*xor*

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|»»»»»» *anonymous*|object|false|none|none|

*continued*

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|»»»»» sandbox|[Upstream](schemas.md#schemaupstream)|false|none|Upstream backend configuration (single target or reference)|
|»»»» subscriptionPlans|[string]|false|none|List of subscription plan names available for this API|
|»»»» policies|[[Policy](schemas.md#schemapolicy)]|false|none|List of policies applied to the GraphQL route(s). A GraphQLApi has<br>no operations[] list to add an explicit `OPTIONS` entry to the way<br>a RestApi does, so when a `cors` policy is attached, the gateway<br>synthesizes an OPTIONS route for the same path itself, sharing this<br>same policy chain — this is what lets `cors` (and every other<br>policy in this list, in declared order) answer a browser's<br>preflight request.|
|»»»»» name|string|true|none|Name of the policy|
|»»»»» version|string|true|none|Version of the policy. Only major-only version is allowed (e.g., v0, v1). Full semantic version (e.g., v1.0.0) is not accepted and will be rejected. The Gateway Controller resolves the major version to the single matching full version installed in the gateway image.|
|»»»»» executionCondition|string|false|none|Expression controlling conditional execution of the policy|
|»»»»» params|object|false|none|Arbitrary parameters for the policy (free-form key/value structure)|
|»»»» deploymentState|string|false|none|Desired deployment state - 'deployed' (default) or 'undeployed'. When set to 'undeployed', the API is removed from router traffic but configuration, API keys, and policies are preserved for potential redeployment.|

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
|kind|GraphQLApi|
|hostRewrite|auto|
|hostRewrite|manual|
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

## Get GraphQLApi by id

<a id="opIdgetGraphQLAPIById"></a>

`GET /graphql-apis/{id}`

> Code samples

```shell

curl -X GET http://localhost:9090/api/management/v1/graphql-apis/{id} \
  -u {username}:{password} \
  -H 'Accept: application/json'

```

Get a GraphQLApi by its ID.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

Required roles: `admin`, `developer`

</aside>

<h3 id="get-graphqlapi-by-id-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|string|true|Unique public identifier for the API.|

#### Detailed descriptions

**id**: Unique public identifier for the API.

> Example responses
>
> 200 Response

```json
{
  "apiVersion": "gateway.api-platform.wso2.com/v1",
  "kind": "GraphQLApi",
  "metadata": {
    "name": "countries-graphql-api-v1.0"
  },
  "spec": {
    "displayName": "Countries-GraphQL-API",
    "version": "v1.0",
    "context": "/countries/$version/graphql",
    "upstream": {
      "main": {
        "url": "https://countries.trevorblades.com/graphql"
      }
    },
    "policies": [
      {
        "name": "jwt-auth",
        "version": "v1"
      }
    ]
  },
  "status": {
    "id": "countries-graphql-api-v1.0",
    "state": "deployed",
    "createdAt": "2026-08-11T10:00:00Z",
    "updatedAt": "2026-08-11T10:00:00Z",
    "deployedAt": "2026-08-11T10:00:00Z"
  }
}
```

<h3 id="get-graphqlapi-by-id-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|GraphQLApi details|[GraphQLAPI](schemas.md#schemagraphqlapi)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|GraphQLApi not found|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

## Update an existing GraphQLApi

<a id="opIdupdateGraphQLAPI"></a>

`PUT /graphql-apis/{id}`

> Code samples

```shell

curl -X PUT http://localhost:9090/api/management/v1/graphql-apis/{id} \
  -u {username}:{password} \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -d @payload.json

```

Update an existing GraphQLApi in the Gateway.

> Payload

```json
{
  "apiVersion": "gateway.api-platform.wso2.com/v1",
  "kind": "GraphQLApi",
  "metadata": {
    "name": "countries-graphql-api-v1.0"
  },
  "spec": {
    "displayName": "Countries-GraphQL-API",
    "version": "v1.0",
    "context": "/countries/$version/graphql",
    "upstream": {
      "main": {
        "url": "https://countries.trevorblades.com/graphql"
      }
    },
    "policies": [
      {
        "name": "jwt-auth",
        "version": "v1"
      }
    ]
  }
}
```

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

Required roles: `admin`, `developer`

</aside>

<h3 id="update-an-existing-graphqlapi-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|string|true|Unique public identifier of the API to update.|
|body|body|[GraphQLAPIRequest](schemas.md#schemagraphqlapirequest)|true|none|

#### Detailed descriptions

**id**: Unique public identifier of the API to update.

> Example responses
>
> 200 Response

```json
{
  "apiVersion": "gateway.api-platform.wso2.com/v1",
  "kind": "GraphQLApi",
  "metadata": {
    "name": "countries-graphql-api-v1.0"
  },
  "spec": {
    "displayName": "Countries-GraphQL-API",
    "version": "v1.0",
    "context": "/countries/$version/graphql",
    "upstream": {
      "main": {
        "url": "https://countries.trevorblades.com/graphql"
      }
    },
    "policies": [
      {
        "name": "jwt-auth",
        "version": "v1"
      }
    ]
  },
  "status": {
    "id": "countries-graphql-api-v1.0",
    "state": "deployed",
    "createdAt": "2026-08-11T10:00:00Z",
    "updatedAt": "2026-08-11T10:00:00Z",
    "deployedAt": "2026-08-11T10:00:00Z"
  }
}
```

<h3 id="update-an-existing-graphqlapi-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|GraphQLApi updated successfully|[GraphQLAPI](schemas.md#schemagraphqlapi)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Invalid configuration (validation failed)|[ErrorResponse](schemas.md#schemaerrorresponse)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|GraphQLApi not found|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

## Delete a GraphQLApi

<a id="opIddeleteGraphQLAPI"></a>

`DELETE /graphql-apis/{id}`

> Code samples

```shell

curl -X DELETE http://localhost:9090/api/management/v1/graphql-apis/{id} \
  -u {username}:{password} \
  -H 'Accept: application/json'

```

Delete a GraphQLApi from the Gateway.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

Required roles: `admin`, `developer`

</aside>

<h3 id="delete-a-graphqlapi-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|string|true|Unique public identifier of the API to delete.|

#### Detailed descriptions

**id**: Unique public identifier of the API to delete.

> Example responses
>
> 200 Response

```json
{
  "status": "success",
  "message": "GraphQLApi deleted successfully",
  "id": "countries-graphql-api-v1.0"
}
```

<h3 id="delete-a-graphqlapi-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|GraphQLApi deleted successfully|Inline|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|GraphQLApi not found|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="delete-a-graphqlapi-responseschema">Response Schema</h3>

Status Code **200**

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|» status|string|false|none|none|
|» message|string|false|none|none|
|» id|string|false|none|none|

## Create a new API key for a GraphQL API

<a id="opIdcreateGraphQLAPIKey"></a>

`POST /graphql-apis/{id}/api-keys`

> Code samples

```shell

curl -X POST http://localhost:9090/api/management/v1/graphql-apis/{id}/api-keys \
  -u {username}:{password} \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -d @payload.json

```

Generate a new API key for a GraphQLApi in the Gateway. The key is a 32-byte random value encoded in hexadecimal, prefixed with `apip_`. Use the API Key policy on the API to validate incoming requests with this key.

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

<h3 id="create-a-new-api-key-for-a-graphql-api-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|string|true|Unique public identifier of the API to generate the key for|
|body|body|[APIKeyCreationRequest](schemas.md#schemaapikeycreationrequest)|true|none|

#### Detailed descriptions

**id**: Unique public identifier of the API to generate the key for

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

<h3 id="create-a-new-api-key-for-a-graphql-api-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|201|[Created](https://tools.ietf.org/html/rfc7231#section-6.3.2)|API key created successfully|[APIKeyCreationResponse](schemas.md#schemaapikeycreationresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Invalid configuration (validation failed)|[ErrorResponse](schemas.md#schemaerrorresponse)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|GraphQLApi not found|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

## Get the list of API keys for a GraphQL API

<a id="opIdlistGraphQLAPIKeys"></a>

`GET /graphql-apis/{id}/api-keys`

> Code samples

```shell

curl -X GET http://localhost:9090/api/management/v1/graphql-apis/{id}/api-keys \
  -u {username}:{password} \
  -H 'Accept: application/json'

```

List all API keys for a GraphQLApi in the Gateway.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

Required roles: `admin`, `consumer`

</aside>

<h3 id="get-the-list-of-api-keys-for-a-graphql-api-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|string|true|Unique public identifier of the API to retrieve the keys for|

#### Detailed descriptions

**id**: Unique public identifier of the API to retrieve the keys for

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

<h3 id="get-the-list-of-api-keys-for-a-graphql-api-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|List of API keys|[APIKeyListResponse](schemas.md#schemaapikeylistresponse)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|GraphQLApi not found|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

## Regenerate API key for a GraphQL API

<a id="opIdregenerateGraphQLAPIKey"></a>

`POST /graphql-apis/{id}/api-keys/{apiKeyName}/regenerate`

> Code samples

```shell

curl -X POST http://localhost:9090/api/management/v1/graphql-apis/{id}/api-keys/{apiKeyName}/regenerate \
  -u {username}:{password} \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -d @payload.json

```

Regenerate an existing API key for a GraphQLApi in the Gateway. The previous key is revoked and replaced with a new 32-byte random value encoded in hexadecimal, prefixed with `apip_`.

> Payload

```json
{}
```

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

Required roles: `admin`, `consumer`

</aside>

<h3 id="regenerate-api-key-for-a-graphql-api-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|string|true|Unique public identifier of the API to generate the key for|
|apiKeyName|path|string|true|Name of the API key to regenerate|
|body|body|[APIKeyRegenerationRequest](schemas.md#schemaapikeyregenerationrequest)|true|none|

#### Detailed descriptions

**id**: Unique public identifier of the API to generate the key for

**apiKeyName**: Name of the API key to regenerate

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

<h3 id="regenerate-api-key-for-a-graphql-api-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|API key rotated successfully|[APIKeyCreationResponse](schemas.md#schemaapikeycreationresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Invalid configuration (validation failed)|[ErrorResponse](schemas.md#schemaerrorresponse)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|GraphQLApi not found|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

## Update an API key with a new regenerated value

<a id="opIdupdateGraphQLAPIKey"></a>

`PUT /graphql-apis/{id}/api-keys/{apiKeyName}`

> Code samples

```shell

curl -X PUT http://localhost:9090/api/management/v1/graphql-apis/{id}/api-keys/{apiKeyName} \
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

<h3 id="update-an-api-key-with-a-new-regenerated-value-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|string|true|Unique public identifier of the API|
|apiKeyName|path|string|true|Name of the API key to update|
|body|body|[APIKeyUpdateRequest](schemas.md#schemaapikeyupdaterequest)|true|none|

#### Detailed descriptions

**id**: Unique public identifier of the API

**apiKeyName**: Name of the API key to update

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

<h3 id="update-an-api-key-with-a-new-regenerated-value-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|API key updated successfully|[APIKeyCreationResponse](schemas.md#schemaapikeycreationresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Invalid request (validation failed)|[ErrorResponse](schemas.md#schemaerrorresponse)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|API or API key not found|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

## Revoke an API key

<a id="opIdrevokeGraphQLAPIKey"></a>

`DELETE /graphql-apis/{id}/api-keys/{apiKeyName}`

> Code samples

```shell

curl -X DELETE http://localhost:9090/api/management/v1/graphql-apis/{id}/api-keys/{apiKeyName} \
  -u {username}:{password} \
  -H 'Accept: application/json'

```

Revoke an API key. Once revoked, it can no longer be used to authenticate requests.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

Required roles: `admin`, `consumer`

</aside>

<h3 id="revoke-an-api-key-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|string|true|Unique public identifier of the API to revoke the key for|
|apiKeyName|path|string|true|Name of the API key to revoke|

#### Detailed descriptions

**id**: Unique public identifier of the API to revoke the key for

**apiKeyName**: Name of the API key to revoke

> Example responses
>
> 200 Response

```json
{
  "status": "success",
  "message": "API key revoked successfully"
}
```

<h3 id="revoke-an-api-key-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|API key revoked successfully|[APIKeyRevocationResponse](schemas.md#schemaapikeyrevocationresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Invalid configuration (validation failed)|[ErrorResponse](schemas.md#schemaerrorresponse)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|GraphQLApi not found|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|
