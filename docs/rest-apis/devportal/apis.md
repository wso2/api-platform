<h1 id="wso2-api-developer-portal-core-devportal-routes-apis">APIs</h1>

## Create API metadata

<a id="opIdcreateApiMetadata"></a>

`POST /o/{orgId}/devportal/v1/apis`

> Code samples

```shell

curl -X POST https://devportal.api-platform.io/o/{orgId}/devportal/v1/apis \
  -u {username}:{password} \
  -H 'Content-Type: multipart/form-data' \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}' \
  -d @payload.json

```

Creates Developer Portal API metadata from either a full API artifact ZIP, an API metadata YAML file, or an `apiMetadata` JSON string. An API definition file is required unless supplied by the artifact ZIP. The service also stores labels, subscription policy mappings, image metadata, and schema definitions for MCP or GraphQL APIs when provided.

> Payload

```yaml
api: string
apiDefinition: string
artifact: string
schemaDefinition: string
apiMetadata: '{"apiInfo":{"apiName":"Weather
  API","apiVersion":"v1","apiDescription":"Weather forecast
  API","apiType":"REST","visibility":"PUBLIC","provider":"WSO2","apiStatus":"PUBLISHED","tags":["weather"],
  "labels":["default"]},"endPoints":{"productionURL":"https://api.example.com/weather",
  "sandboxURL":"https://sandbox.example.com/weather"},"subscriptionPolicies":[{"policyName":"Gold"}]}'

```

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="create-api-metadata-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|object|true|API metadata upload. Send either `artifact`, or `api` with `apiDefinition`, or `apiMetadata` with `apiDefinition`. `schemaDefinition` is used for MCP APIs and GraphQL schema updates.|
|» api|body|string(binary)|false|API metadata YAML file.|
|» apiDefinition|body|string(binary)|false|API definition file.|
|» artifact|body|string(binary)|false|Full API ZIP artifact containing metadata and definition files.|
|» schemaDefinition|body|string(binary)|false|Schema definition file, used by MCP APIs.|
|» apiMetadata|body|string|false|JSON string accepted by the service when the `api` YAML file is not supplied.|
|orgId|path|string|true|none|

> Example responses

> 201 Response

```json
{
  "apiID": "api-7f4c2a6b",
  "apiReferenceID": "cp-api-12345",
  "apiHandle": "weather-api-v1",
  "provider": "WSO2",
  "dataSource": "DEVPORTAL",
  "apiInfo": {
    "apiName": "Weather API",
    "apiTitle": "Weather Forecast API",
    "apiVersion": "v1",
    "apiDescription": "Weather forecast API.",
    "apiType": "REST",
    "visibility": "PUBLIC",
    "agentVisibility": "VISIBLE",
    "gatewayVendor": "wso2",
    "tokenBasedSubscriptionEnabled": false,
    "gatewayType": null,
    "tags": [
      "weather"
    ],
    "labels": [
      "default"
    ]
  },
  "endPoints": {
    "productionURL": "https://api.example.com/weather",
    "sandboxURL": "https://sandbox.example.com/weather"
  },
  "subscriptionPolicies": [
    {
      "policyID": "policy-gold",
      "policyName": "Gold",
      "displayName": "Gold",
      "requestCount": 10000
    }
  ]
}
```

> Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.

```json
[
  {
    "code": "400",
    "message": "input validation failed",
    "description": "Invalid value"
  }
]
```

```json
{
  "code": "400",
  "message": "Bad Request",
  "description": "Missing required parameter: 'orgId'"
}
```

```json
{
  "message": "Missing or invalid fields in the request payload"
}
```

> 404 Response

```json
{
  "code": "404",
  "message": "Resource Not Found",
  "description": "Organization not found"
}
```

> 409 Response

```json
{
  "code": "409",
  "message": "Conflict",
  "description": "Organization already exists"
}
```

> 500 Response

```json
{
  "code": "500",
  "message": "Internal Server Error",
  "description": "Internal Server Error"
}
```

<h3 id="create-api-metadata-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|201|[Created](https://tools.ietf.org/html/rfc7231#section-6.3.2)|Created API metadata payload returned by the service.|[ApiMetadataCreateResponse](schemas.md#schemaapimetadatacreateresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.|Inline|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Resource not found.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Duplicate organization data conflicts with an existing record.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="create-api-metadata-responseschema">Response Schema</h3>

## List API metadata

<a id="opIdgetAllApiMetadataForOrganization"></a>

`GET /o/{orgId}/devportal/v1/apis`

> Code samples

```shell

curl -X GET https://devportal.api-platform.io/o/{orgId}/devportal/v1/apis \
  -u {username}:{password} \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}'

```

Lists API metadata for an organization. The service supports exact filters by API name, version, and tags, free-text search with `query`, group filtering, and view filtering. Unknown query parameters are rejected.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="list-api-metadata-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|query|query|string|false|Free-text API metadata search term.|
|apiName|query|string|false|Exact API name filter.|
|version|query|string|false|Exact API version filter.|
|tags|query|string|false|Exact API tags filter used by the metadata DAO.|
|groups|query|string|false|Space-separated visible groups used for API visibility filtering.|
|view|query|string|false|Developer Portal view name used to filter visible APIs.|
|orgId|path|string|true|none|

> Example responses

> 200 Response

```json
[
  {
    "apiID": "api-7f4c2a6b",
    "apiReferenceID": "cp-api-12345",
    "apiHandle": "weather-api-v1",
    "provider": "WSO2",
    "dataSource": "DEVPORTAL",
    "apiInfo": {
      "apiName": "Weather API",
      "apiVersion": "v1",
      "apiDescription": "Weather forecast API.",
      "apiType": "REST",
      "visibility": "PUBLIC",
      "agentVisibility": "VISIBLE",
      "gatewayVendor": "wso2",
      "tokenBasedSubscriptionEnabled": false,
      "gatewayType": null,
      "labels": [
        "default"
      ]
    },
    "endPoints": {
      "sandboxURL": "https://sandbox.example.com/weather",
      "productionURL": "https://api.example.com/weather"
    }
  }
]
```

> Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.

```json
[
  {
    "code": "400",
    "message": "input validation failed",
    "description": "Invalid value"
  }
]
```

```json
{
  "code": "400",
  "message": "Bad Request",
  "description": "Missing required parameter: 'orgId'"
}
```

```json
{
  "message": "Missing or invalid fields in the request payload"
}
```

> 500 Response

```json
{
  "code": "500",
  "message": "Internal Server Error",
  "description": "Internal Server Error"
}
```

<h3 id="list-api-metadata-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|List of API metadata DTOs.|Inline|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.|Inline|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="list-api-metadata-responseschema">Response Schema</h3>

Status Code **200**

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|[[ApiMetadataResponse](schemas.md#schemaapimetadataresponse)]|false|none|none|
|» apiID|string|false|none|none|
|» apiReferenceID|string|false|none|none|
|» apiHandle|string|false|none|none|
|» provider|string|false|none|none|
|» dataSource|string|false|none|none|
|» policyID|string|false|none|none|
|» apiInfo|[ApiInfoResponse](schemas.md#schemaapiinforesponse)|false|none|none|
|»» apiName|string|false|none|none|
|»» apiTitle|string¦null|false|none|none|
|»» remotes|[object]|false|none|none|
|»» apiVersion|string|false|none|none|
|»» apiDescription|string|false|none|none|
|»» apiType|string|false|none|none|
|»» visibility|string|false|none|none|
|»» agentVisibility|string|false|none|none|
|»» gatewayVendor|string|false|none|none|
|»» tokenBasedSubscriptionEnabled|boolean|false|none|none|
|»» gatewayType|string¦null|false|none|none|
|»» addedLabels|[string]|false|none|none|
|»» removedLabels|[string]|false|none|none|
|»» visibleGroups|[string]|false|none|none|
|»» owners|[ApiOwnersResponse](schemas.md#schemaapiownersresponse)|false|none|none|
|»»» technicalOwner|string|false|none|none|
|»»» businessOwner|string|false|none|none|
|»»» businessOwnerEmail|string|false|none|none|
|»»» technicalOwnerEmail|string|false|none|none|
|»» apiImageMetadata|[ApiImageMetadataResponse](schemas.md#schemaapiimagemetadataresponse)|false|none|none|
|»»» **additionalProperties**|string|false|none|none|
|»» tags|[string]|false|none|none|
|»» labels|[string]|false|none|none|
|» endPoints|[ApiEndpointsResponse](schemas.md#schemaapiendpointsresponse)|false|none|none|
|»» sandboxURL|string|false|none|none|
|»» productionURL|string|false|none|none|
|» subscriptionPolicies|[[SubscriptionPolicyResponse](schemas.md#schemasubscriptionpolicyresponse)]|false|none|none|
|»» policyID|string|false|none|none|
|»» policyName|string|false|none|none|
|»» displayName|string|false|none|none|
|»» description|string|false|none|none|
|»» requestCount|any|false|none|none|

*oneOf*

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|»»» *anonymous*|integer|false|none|none|

*xor*

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|»»» *anonymous*|string|false|none|none|

*continued*

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|»» orgID|string|false|none|none|

## Get API metadata

<a id="opIdgetApiMetadata"></a>

`GET /o/{orgId}/devportal/v1/apis/{apiId}`

> Code samples

```shell

curl -X GET https://devportal.api-platform.io/o/{orgId}/devportal/v1/apis/{apiId} \
  -u {username}:{password} \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}'

```

Retrieves a single API metadata record by Developer Portal API ID.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="get-api-metadata-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|orgId|path|string|true|none|
|apiId|path|string|true|none|

> Example responses

> 200 Response

```json
{
  "apiID": "api-7f4c2a6b",
  "apiReferenceID": "cp-api-12345",
  "apiHandle": "weather-api-v1",
  "provider": "WSO2",
  "dataSource": "DEVPORTAL",
  "apiInfo": {
    "apiName": "Weather API",
    "apiTitle": "Weather Forecast API",
    "remotes": [],
    "apiVersion": "v1",
    "apiDescription": "Weather forecast API.",
    "apiType": "REST",
    "visibility": "PUBLIC",
    "agentVisibility": "VISIBLE",
    "gatewayVendor": "wso2",
    "tokenBasedSubscriptionEnabled": false,
    "gatewayType": null,
    "labels": [
      "default"
    ]
  },
  "endPoints": {
    "sandboxURL": "https://sandbox.example.com/weather",
    "productionURL": "https://api.example.com/weather"
  },
  "subscriptionPolicies": [
    {
      "policyID": "policy-gold",
      "policyName": "Gold",
      "displayName": "Gold",
      "requestCount": 10000
    }
  ]
}
```

> Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.

```json
[
  {
    "code": "400",
    "message": "input validation failed",
    "description": "Invalid value"
  }
]
```

```json
{
  "code": "400",
  "message": "Bad Request",
  "description": "Missing required parameter: 'orgId'"
}
```

```json
{
  "message": "Missing or invalid fields in the request payload"
}
```

> 404 Response

```
"string"
```

> 500 Response

```json
{
  "code": "500",
  "message": "Internal Server Error",
  "description": "Internal Server Error"
}
```

<h3 id="get-api-metadata-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|API metadata DTO returned by the service.|[ApiMetadataResponse](schemas.md#schemaapimetadataresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.|Inline|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Plain text success response.|string|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="get-api-metadata-responseschema">Response Schema</h3>

## Update API metadata

<a id="opIdupdateApiMetadata"></a>

`PUT /o/{orgId}/devportal/v1/apis/{apiId}`

> Code samples

```shell

curl -X PUT https://devportal.api-platform.io/o/{orgId}/devportal/v1/apis/{apiId} \
  -u {username}:{password} \
  -H 'Content-Type: multipart/form-data' \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}' \
  -d @payload.json

```

Updates Developer Portal API metadata and its stored definition. The update flow can also adjust label mappings, subscription policy mappings, schema definitions, and image metadata. Status changes to unpublished are rejected when active subscriptions exist.

> Payload

```yaml
api: string
apiDefinition: string
artifact: string
schemaDefinition: string
apiMetadata: '{"apiInfo":{"apiName":"Weather
  API","apiVersion":"v1","apiDescription":"Weather forecast
  API","apiType":"REST","visibility":"PUBLIC","provider":"WSO2","apiStatus":"PUBLISHED","tags":["weather"],
  "labels":["default"]},"endPoints":{"productionURL":"https://api.example.com/weather",
  "sandboxURL":"https://sandbox.example.com/weather"},"subscriptionPolicies":[{"policyName":"Gold"}]}'

```

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="update-api-metadata-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|object|true|API metadata upload. Send either `artifact`, or `api` with `apiDefinition`, or `apiMetadata` with `apiDefinition`. `schemaDefinition` is used for MCP APIs and GraphQL schema updates.|
|» api|body|string(binary)|false|API metadata YAML file.|
|» apiDefinition|body|string(binary)|false|API definition file.|
|» artifact|body|string(binary)|false|Full API ZIP artifact containing metadata and definition files.|
|» schemaDefinition|body|string(binary)|false|Schema definition file, used by MCP APIs.|
|» apiMetadata|body|string|false|JSON string accepted by the service when the `api` YAML file is not supplied.|
|orgId|path|string|true|none|
|apiId|path|string|true|none|

> Example responses

> 200 Response

```json
{
  "apiID": "api-7f4c2a6b",
  "apiReferenceID": "cp-api-12345",
  "apiHandle": "weather-api-v1",
  "provider": "WSO2",
  "dataSource": "DEVPORTAL",
  "apiInfo": {
    "apiName": "Weather API",
    "apiTitle": "Weather Forecast API",
    "remotes": [],
    "apiVersion": "v1",
    "apiDescription": "Weather forecast API.",
    "apiType": "REST",
    "visibility": "PUBLIC",
    "agentVisibility": "VISIBLE",
    "gatewayVendor": "wso2",
    "tokenBasedSubscriptionEnabled": false,
    "gatewayType": null,
    "labels": [
      "default"
    ]
  },
  "endPoints": {
    "sandboxURL": "https://sandbox.example.com/weather",
    "productionURL": "https://api.example.com/weather"
  },
  "subscriptionPolicies": [
    {
      "policyID": "policy-gold",
      "policyName": "Gold",
      "displayName": "Gold",
      "requestCount": 10000
    }
  ]
}
```

> Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.

```json
[
  {
    "code": "400",
    "message": "input validation failed",
    "description": "Invalid value"
  }
]
```

```json
{
  "code": "400",
  "message": "Bad Request",
  "description": "Missing required parameter: 'orgId'"
}
```

```json
{
  "message": "Missing or invalid fields in the request payload"
}
```

> 404 Response

```json
{
  "code": "404",
  "message": "Resource Not Found",
  "description": "Organization not found"
}
```

> 409 Response

```json
{
  "code": "409",
  "message": "Conflict",
  "description": "Organization already exists"
}
```

> 500 Response

```json
{
  "code": "500",
  "message": "Internal Server Error",
  "description": "Internal Server Error"
}
```

<h3 id="update-api-metadata-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|API metadata DTO returned by the service.|[ApiMetadataResponse](schemas.md#schemaapimetadataresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.|Inline|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Resource not found.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Duplicate organization data conflicts with an existing record.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="update-api-metadata-responseschema">Response Schema</h3>

## Delete API metadata

<a id="opIddeleteApiMetadata"></a>

`DELETE /o/{orgId}/devportal/v1/apis/{apiId}`

> Code samples

```shell

curl -X DELETE https://devportal.api-platform.io/o/{orgId}/devportal/v1/apis/{apiId} \
  -u {username}:{password} \
  -H 'Accept: text/plain' \
  -H 'Authorization: Bearer {access-token}'

```

Deletes API metadata when the API has no active subscriptions.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="delete-api-metadata-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|orgId|path|string|true|none|
|apiId|path|string|true|none|

> Example responses

> 200 Response

```
"string"
```

> Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.

```json
[
  {
    "code": "400",
    "message": "input validation failed",
    "description": "Invalid value"
  }
]
```

```json
{
  "code": "400",
  "message": "Bad Request",
  "description": "Missing required parameter: 'orgId'"
}
```

```json
{
  "message": "Missing or invalid fields in the request payload"
}
```

> 404 Response

```json
{
  "code": "404",
  "message": "Resource Not Found",
  "description": "Organization not found"
}
```

> 409 Response

```json
{
  "code": "409",
  "message": "Conflict",
  "description": "Organization already exists"
}
```

> 500 Response

```json
{
  "code": "500",
  "message": "Internal Server Error",
  "description": "Internal Server Error"
}
```

<h3 id="delete-api-metadata-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Plain text success response.|string|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.|Inline|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Resource not found.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Duplicate organization data conflicts with an existing record.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="delete-api-metadata-responseschema">Response Schema</h3>
