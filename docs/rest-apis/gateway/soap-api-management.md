<h1 id="gateway-controller-management-api-soap-api-management">SOAP API Management</h1>

## Create a new SoapAPI

<a id="opIdcreateSoapAPI"></a>

`POST /soap-apis`

> Code samples

```shell

curl -X POST http://localhost:9090/api/management/v1/soap-apis \
  -u {username}:{password} \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -d @payload.json

```

Add a new SOAP API to the Gateway.

> Payload

```json
{
  "apiVersion": "gateway.api-platform.wso2.com/v1",
  "kind": "SoapApi",
  "metadata": {
    "name": "stockquote-api-v1.0"
  },
  "spec": {
    "displayName": "StockQuote API",
    "version": "v1.0",
    "context": "/stockquote/$version",
    "upstream": {
      "main": {
        "url": "http://stockquote-service:8080/services/StockQuoteService"
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

<h3 id="create-a-new-soapapi-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|[SoapAPIRequest](schemas.md#schemasoapapirequest)|true|none|

> Example responses
>
> 201 Response

```json
{
  "apiVersion": "gateway.api-platform.wso2.com/v1",
  "kind": "SoapApi",
  "metadata": {
    "name": "stockquote-api-v1.0"
  },
  "spec": {
    "displayName": "StockQuote API",
    "version": "v1.0",
    "context": "/stockquote/$version",
    "upstream": {
      "main": {
        "url": "http://stockquote-service:8080/services/StockQuoteService"
      }
    }
  },
  "status": {
    "id": "stockquote-api-v1.0",
    "state": "deployed",
    "createdAt": "2026-04-24T07:21:13Z",
    "updatedAt": "2026-04-24T07:21:13Z",
    "deployedAt": "2026-04-24T07:21:13Z"
  }
}
```

<h3 id="create-a-new-soapapi-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|201|[Created](https://tools.ietf.org/html/rfc7231#section-6.3.2)|SoapAPI created successfully|[SoapAPI](schemas.md#schemasoapapi)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Invalid configuration (validation failed)|[ErrorResponse](schemas.md#schemaerrorresponse)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Conflict - API with same name and version already exists|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

## List all SoapAPIs

<a id="opIdlistSoapAPIs"></a>

`GET /soap-apis`

> Code samples

```shell

curl -X GET http://localhost:9090/api/management/v1/soap-apis \
  -u {username}:{password} \
  -H 'Accept: application/json'

```

List SOAP APIs registered in the Gateway, optionally filtered by name, version, context, or status.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

Required roles: `admin`, `developer`

</aside>

<h3 id="list-all-soapapis-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|displayName|query|string|false|Filter by API display name|
|version|query|string|false|Filter by API version|
|context|query|string|false|Filter by API context path|
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
  "status": "string",
  "count": 0,
  "soapApis": [
    {
      "apiVersion": "gateway.api-platform.wso2.com/v1",
      "kind": "SoapApi",
      "metadata": {
        "name": "stockquote-api-v1.0"
      },
      "spec": {
        "displayName": "StockQuote API",
        "version": "v1.0",
        "context": "/stockquote/$version",
        "upstream": {
          "main": {
            "url": "http://stockquote-service:8080/services/StockQuoteService"
          }
        }
      },
      "status": {
        "id": "stockquote-api-v1.0",
        "state": "deployed",
        "createdAt": "2026-04-24T07:21:13Z",
        "updatedAt": "2026-04-24T07:21:13Z",
        "deployedAt": "2026-04-24T07:21:13Z"
      }
    }
  ]
}
```

<h3 id="list-all-soapapis-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|List of SoapAPIs|Inline|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="list-all-soapapis-responseschema">Response Schema</h3>

Status Code **200**

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|» status|string|false|none|none|
|» count|integer|false|none|none|
|» soapApis|[allOf]|false|none|none|

*allOf*

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|»» *anonymous*|[SoapAPIRequest](schemas.md#schemasoapapirequest)|false|none|none|
|»»» apiVersion|string|true|none|API specification version|
|»»» kind|string|true|none|API type|
|»»» metadata|[Metadata](schemas.md#schemametadata)|true|none|none|
|»»»» name|string|true|none|Unique handle for the resource|
|»»»» labels|object|false|none|Labels are key-value pairs for organizing and selecting APIs. Keys must not contain spaces.|
|»»»»» **additionalProperties**|string|false|none|none|
|»»»» annotations|object|false|none|Annotations are arbitrary non-identifying metadata. Use domain-prefixed keys.|
|»»»»» **additionalProperties**|string|false|none|none|
|»»» spec|[SoapAPIData](schemas.md#schemasoapapidata)|true|none|none|
|»»»» displayName|string|true|none|Human-readable API name (must be URL-friendly - only letters, numbers, spaces, hyphens, underscores, and dots allowed)|
|»»»» version|string|true|none|Semantic version of the API|
|»»»» context|string|true|none|Base path for the SOAP service endpoint on the gateway (must start with /, no trailing slash). Use $version to embed the version in the path (e.g., /stockquote/$version resolves to /stockquote/v1.0).|
|»»»» upstream|object|true|none|Backend SOAP service endpoint|
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
|»»»» vhosts|object|false|none|Custom virtual hosts/domains for the API|
|»»»»» main|string|true|none|Custom virtual host/domain for production traffic|
|»»»»» sandbox|string|false|none|Custom virtual host/domain for sandbox traffic|
|»»»» subscriptionPlans|[string]|false|none|List of subscription plan names available for this API|
|»»»» policies|[[Policy](schemas.md#schemapolicy)]|false|none|API-level policies applied to all SOAP traffic|
|»»»»» name|string|true|none|Name of the policy|
|»»»»» version|string|true|none|Version of the policy. Only major-only version is allowed (e.g., v0, v1). Full semantic version (e.g., v1.0.0) is not accepted and will be rejected. The Gateway Controller resolves the major version to the single matching full version installed in the gateway image.|
|»»»»» executionCondition|string|false|none|Expression controlling conditional execution of the policy|
|»»»»» params|object|false|none|Arbitrary parameters for the policy (free-form key/value structure)|
|»»»» faultPolicies|[[Policy](schemas.md#schemapolicy)]|false|none|Ordered list of policies executed only when a request fails, over the error response. Applies to both the POST and GET routes.<br><br>Each entry is labelled with the source of the failure, so an entry that should see only some failures narrows with executionCondition.|
|»»»» deploymentState|string|false|none|Desired deployment state. Defaults to 'deployed'.|

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
|kind|SoapApi|
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

## Get SoapAPI by id

<a id="opIdgetSoapAPIById"></a>

`GET /soap-apis/{id}`

> Code samples

```shell

curl -X GET http://localhost:9090/api/management/v1/soap-apis/{id} \
  -u {username}:{password} \
  -H 'Accept: application/json'

```

Get a SOAP API by its ID.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

Required roles: `admin`, `developer`

</aside>

<h3 id="get-soapapi-by-id-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|string|true|none|

> Example responses
>
> 200 Response

```json
{
  "apiVersion": "gateway.api-platform.wso2.com/v1",
  "kind": "SoapApi",
  "metadata": {
    "name": "stockquote-api-v1.0"
  },
  "spec": {
    "displayName": "StockQuote API",
    "version": "v1.0",
    "context": "/stockquote/$version",
    "upstream": {
      "main": {
        "url": "http://stockquote-service:8080/services/StockQuoteService"
      }
    }
  },
  "status": {
    "id": "stockquote-api-v1.0",
    "state": "deployed",
    "createdAt": "2026-04-24T07:21:13Z",
    "updatedAt": "2026-04-24T07:21:13Z",
    "deployedAt": "2026-04-24T07:21:13Z"
  }
}
```

<h3 id="get-soapapi-by-id-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|SoapAPI details|[SoapAPI](schemas.md#schemasoapapi)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|SoapAPI not found|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

## Update an existing SoapAPI

<a id="opIdupdateSoapAPI"></a>

`PUT /soap-apis/{id}`

> Code samples

```shell

curl -X PUT http://localhost:9090/api/management/v1/soap-apis/{id} \
  -u {username}:{password} \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -d @payload.json

```

Update an existing SOAP API in the Gateway.

> Payload

```json
{
  "apiVersion": "gateway.api-platform.wso2.com/v1",
  "kind": "SoapApi",
  "metadata": {
    "name": "stockquote-api-v1.0"
  },
  "spec": {
    "displayName": "StockQuote API",
    "version": "v1.0",
    "context": "/stockquote/$version",
    "upstream": {
      "main": {
        "url": "http://stockquote-service:8080/services/StockQuoteService"
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

<h3 id="update-an-existing-soapapi-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|string|true|none|
|body|body|[SoapAPIRequest](schemas.md#schemasoapapirequest)|true|none|

> Example responses
>
> 200 Response

```json
{
  "apiVersion": "gateway.api-platform.wso2.com/v1",
  "kind": "SoapApi",
  "metadata": {
    "name": "stockquote-api-v1.0"
  },
  "spec": {
    "displayName": "StockQuote API",
    "version": "v1.0",
    "context": "/stockquote/$version",
    "upstream": {
      "main": {
        "url": "http://stockquote-service:8080/services/StockQuoteService"
      }
    }
  },
  "status": {
    "id": "stockquote-api-v1.0",
    "state": "deployed",
    "createdAt": "2026-04-24T07:21:13Z",
    "updatedAt": "2026-04-24T07:21:13Z",
    "deployedAt": "2026-04-24T07:21:13Z"
  }
}
```

<h3 id="update-an-existing-soapapi-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|SoapAPI updated successfully|[SoapAPI](schemas.md#schemasoapapi)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Invalid configuration|[ErrorResponse](schemas.md#schemaerrorresponse)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|SoapAPI not found|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

## Delete a SoapAPI

<a id="opIddeleteSoapAPI"></a>

`DELETE /soap-apis/{id}`

> Code samples

```shell

curl -X DELETE http://localhost:9090/api/management/v1/soap-apis/{id} \
  -u {username}:{password} \
  -H 'Accept: application/json'

```

Delete a SOAP API from the Gateway.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

Required roles: `admin`, `developer`

</aside>

<h3 id="delete-a-soapapi-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|string|true|none|

> Example responses
>
> 200 Response

```json
{
  "status": "success",
  "message": "SoapAPI deleted successfully",
  "id": "string"
}
```

<h3 id="delete-a-soapapi-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|SoapAPI deleted successfully|Inline|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|SoapAPI not found|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="delete-a-soapapi-responseschema">Response Schema</h3>

Status Code **200**

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|» status|string|false|none|none|
|» message|string|false|none|none|
|» id|string|false|none|none|

## Generate a new API key for a SoapAPI

<a id="opIdcreateSoapAPIKey"></a>

`POST /soap-apis/{id}/api-keys`

> Code samples

```shell

curl -X POST http://localhost:9090/api/management/v1/soap-apis/{id}/api-keys \
  -u {username}:{password} \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -d @payload.json

```

Generate a new API key for a SoapAPI in the Gateway. The key is a 32-byte random value encoded in hexadecimal, prefixed with `apip_`. Use the API Key policy on the API to validate incoming requests with this key.

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

<h3 id="generate-a-new-api-key-for-a-soapapi-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|string|true|none|
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

<h3 id="generate-a-new-api-key-for-a-soapapi-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|201|[Created](https://tools.ietf.org/html/rfc7231#section-6.3.2)|API key created successfully|[APIKeyCreationResponse](schemas.md#schemaapikeycreationresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Invalid request|[ErrorResponse](schemas.md#schemaerrorresponse)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|SoapAPI not found|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

## List all API keys for a SoapAPI

<a id="opIdlistSoapAPIKeys"></a>

`GET /soap-apis/{id}/api-keys`

> Code samples

```shell

curl -X GET http://localhost:9090/api/management/v1/soap-apis/{id}/api-keys \
  -u {username}:{password} \
  -H 'Accept: application/json'

```

List all API keys for a SOAP API in the Gateway.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

Required roles: `admin`, `consumer`

</aside>

<h3 id="list-all-api-keys-for-a-soapapi-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|string|true|none|

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

<h3 id="list-all-api-keys-for-a-soapapi-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|List of API keys|[APIKeyListResponse](schemas.md#schemaapikeylistresponse)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|SoapAPI not found|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

## Regenerate an existing API key for a SoapAPI

<a id="opIdregenerateSoapAPIKey"></a>

`POST /soap-apis/{id}/api-keys/{apiKeyName}/regenerate`

> Code samples

```shell

curl -X POST http://localhost:9090/api/management/v1/soap-apis/{id}/api-keys/{apiKeyName}/regenerate \
  -u {username}:{password} \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -d @payload.json

```

Regenerate an existing API key for a SoapAPI in the Gateway. The previous key is revoked and replaced with a new 32-byte random value encoded in hexadecimal, prefixed with `apip_`.

> Payload

```json
{}
```

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

Required roles: `admin`, `consumer`

</aside>

<h3 id="regenerate-an-existing-api-key-for-a-soapapi-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|string|true|none|
|apiKeyName|path|string|true|none|
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

<h3 id="regenerate-an-existing-api-key-for-a-soapapi-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|API key regenerated successfully|[APIKeyCreationResponse](schemas.md#schemaapikeycreationresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Invalid request|[ErrorResponse](schemas.md#schemaerrorresponse)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|SoapAPI or API key not found|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

## Update an API key with a new regenerated value

<a id="opIdupdateSoapAPIKey"></a>

`PUT /soap-apis/{id}/api-keys/{apiKeyName}`

> Code samples

```shell

curl -X PUT http://localhost:9090/api/management/v1/soap-apis/{id}/api-keys/{apiKeyName} \
  -u {username}:{password} \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -d @payload.json

```

Update an existing API key of a SoapAPI with a regenerated value. The name in the request body must match the key name in the URL.

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
|id|path|string|true|none|
|apiKeyName|path|string|true|none|
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

<h3 id="update-an-api-key-with-a-new-regenerated-value-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|API key updated|[APIKeyCreationResponse](schemas.md#schemaapikeycreationresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Invalid request|[ErrorResponse](schemas.md#schemaerrorresponse)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not found|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

## Revoke an API key

<a id="opIdrevokeSoapAPIKey"></a>

`DELETE /soap-apis/{id}/api-keys/{apiKeyName}`

> Code samples

```shell

curl -X DELETE http://localhost:9090/api/management/v1/soap-apis/{id}/api-keys/{apiKeyName} \
  -u {username}:{password} \
  -H 'Accept: application/json'

```

Revoke an existing API key of a SoapAPI in the Gateway.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

Required roles: `admin`, `consumer`

</aside>

<h3 id="revoke-an-api-key-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|string|true|none|
|apiKeyName|path|string|true|none|

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
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|API key revoked|[APIKeyRevocationResponse](schemas.md#schemaapikeyrevocationresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Invalid request|[ErrorResponse](schemas.md#schemaerrorresponse)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not found|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|
