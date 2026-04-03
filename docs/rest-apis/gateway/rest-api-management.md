<h1 id="gateway-controller-management-api-rest-api-management">Rest API Management</h1>

CRUD operations for Rest APIs

## Create a new RestAPI

<a id="opIdcreateRestAPI"></a>

`POST /rest-apis`

> Code samples

```shell

curl -X POST http://localhost:9090/rest-apis \
  -u {username}:{password} \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -d @payload.json

```

Add a new RestAPI to the Gateway.

> Payload

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

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

Required roles: `admin`, `developer`

</aside>

<h3 id="create-a-new-restapi-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|[RestAPI](schemas.md#schemarestapi)|true|none|

> Example responses

> 201 Response

```json
{
  "status": "success",
  "message": "RestAPI created successfully",
  "id": "reading-list-api-v1.0",
  "createdAt": "2025-10-11T10:30:00Z"
}
```

<h3 id="create-a-new-restapi-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|201|[Created](https://tools.ietf.org/html/rfc7231#section-6.3.2)|RestAPI created successfully|[RestAPICreateResponse](schemas.md#schemarestapicreateresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Invalid configuration (validation failed)|[ErrorResponse](schemas.md#schemaerrorresponse)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Conflict - API with same name and version already exists|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

## List all RestAPIs

<a id="opIdlistRestAPIs"></a>

`GET /rest-apis`

> Code samples

```shell

curl -X GET http://localhost:9090/rest-apis \
  -u {username}:{password} \
  -H 'Accept: application/json'

```

List RestAPIs registered in the Gateway, optionally filtered by name, version, context, or status.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

Required roles: `admin`, `developer`

</aside>

<h3 id="list-all-restapis-parameters">Parameters</h3>

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

> 200 Response

```json
{
  "status": "success",
  "count": 5,
  "apis": [
    {
      "id": "reading-list-api-v1.0",
      "displayName": "Weather API",
      "version": "v1.0",
      "context": "/weather/$version",
      "status": "deployed",
      "createdAt": "2025-10-11T10:30:00Z",
      "updatedAt": "2025-10-11T10:30:00Z"
    }
  ]
}
```

<h3 id="list-all-restapis-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|List of RestAPIs|Inline|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="list-all-restapis-responseschema">Response Schema</h3>

Status Code **200**

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|» status|string|false|none|none|
|» count|integer|false|none|none|
|» apis|[[RestAPIListItem](schemas.md#schemarestapilistitem)]|false|none|none|
|»» id|string|false|none|none|
|»» displayName|string|false|none|none|
|»» version|string|false|none|none|
|»» context|string|false|none|none|
|»» status|string|false|none|none|
|»» createdAt|string(date-time)|false|none|none|
|»» updatedAt|string(date-time)|false|none|none|

#### Enumerated Values

|Property|Value|
|---|---|
|status|deployed|
|status|undeployed|

## Get RestAPI by id

<a id="opIdgetRestAPIById"></a>

`GET /rest-apis/{id}`

> Code samples

```shell

curl -X GET http://localhost:9090/rest-apis/{id} \
  -u {username}:{password} \
  -H 'Accept: application/json'

```

Get a RestAPI by its ID.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

Required roles: `admin`, `developer`

</aside>

<h3 id="get-restapi-by-id-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|string|true|Unique public identifier for the API.|

#### Detailed descriptions

**id**: Unique public identifier for the API.

> Example responses

> 200 Response

```json
{
  "status": "success",
  "api": {
    "id": "reading-list-api-v1.0",
    "configuration": {
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

<h3 id="get-restapi-by-id-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|RestAPI details|[RestAPIDetailResponse](schemas.md#schemarestapidetailresponse)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|RestAPI not found|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

## Update an existing RestAPI

<a id="opIdupdateRestAPI"></a>

`PUT /rest-apis/{id}`

> Code samples

```shell

curl -X PUT http://localhost:9090/rest-apis/{id} \
  -u {username}:{password} \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -d @payload.json

```

Update an existing RestAPI in the Gateway.

> Payload

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

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

Required roles: `admin`, `developer`

</aside>

<h3 id="update-an-existing-restapi-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|string|true|Unique public identifier of the API to update.|
|body|body|[RestAPI](schemas.md#schemarestapi)|true|none|

#### Detailed descriptions

**id**: Unique public identifier of the API to update.

> Example responses

> 200 Response

```json
{
  "status": "success",
  "message": "RestAPI updated successfully",
  "id": "reading-list-api-v1.0",
  "updatedAt": "2025-10-11T11:45:00Z"
}
```

<h3 id="update-an-existing-restapi-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|RestAPI updated successfully|[RestAPIUpdateResponse](schemas.md#schemarestapiupdateresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Invalid configuration (validation failed)|[ErrorResponse](schemas.md#schemaerrorresponse)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|RestAPI not found|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

## Delete a RestAPI

<a id="opIddeleteRestAPI"></a>

`DELETE /rest-apis/{id}`

> Code samples

```shell

curl -X DELETE http://localhost:9090/rest-apis/{id} \
  -u {username}:{password} \
  -H 'Accept: application/json'

```

Delete a RestAPI from the Gateway.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

Required roles: `admin`, `developer`

</aside>

<h3 id="delete-a-restapi-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|string|true|Unique public identifier of the API to delete.|

#### Detailed descriptions

**id**: Unique public identifier of the API to delete.

> Example responses

> 200 Response

```json
{
  "status": "success",
  "message": "RestAPI deleted successfully",
  "id": "reading-list-api-v1.0"
}
```

<h3 id="delete-a-restapi-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|RestAPI deleted successfully|Inline|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|RestAPI not found|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="delete-a-restapi-responseschema">Response Schema</h3>

Status Code **200**

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|» status|string|false|none|none|
|» message|string|false|none|none|
|» id|string|false|none|none|

## Create a new API key for an API

<a id="opIdcreateAPIKey"></a>

`POST /rest-apis/{id}/api-keys`

> Code samples

```shell

curl -X POST http://localhost:9090/rest-apis/{id}/api-keys \
  -u {username}:{password} \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -d @payload.json

```

Generate a new API key for a RestAPI in the Gateway. The key is a 32-byte random value encoded in hexadecimal, prefixed with `apip_`. Use the API Key policy on the API to validate incoming requests with this key.

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

<h3 id="create-a-new-api-key-for-an-api-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|string|true|Unique public identifier of the API to generate the key for|
|body|body|[APIKeyCreationRequest](schemas.md#schemaapikeycreationrequest)|true|none|

#### Detailed descriptions

**id**: Unique public identifier of the API to generate the key for

> Example responses

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

<h3 id="create-a-new-api-key-for-an-api-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|201|[Created](https://tools.ietf.org/html/rfc7231#section-6.3.2)|API key created successfully|[APIKeyCreationResponse](schemas.md#schemaapikeycreationresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Invalid configuration (validation failed)|[ErrorResponse](schemas.md#schemaerrorresponse)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|RestAPI not found|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

## Get the list of API keys for an API

<a id="opIdlistAPIKeys"></a>

`GET /rest-apis/{id}/api-keys`

> Code samples

```shell

curl -X GET http://localhost:9090/rest-apis/{id}/api-keys \
  -u {username}:{password} \
  -H 'Accept: application/json'

```

List all API keys for a RestAPI in the Gateway.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

Required roles: `admin`, `consumer`

</aside>

<h3 id="get-the-list-of-api-keys-for-an-api-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|string|true|Unique public identifier of the API to retrieve the keys for|

#### Detailed descriptions

**id**: Unique public identifier of the API to retrieve the keys for

> Example responses

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

<h3 id="get-the-list-of-api-keys-for-an-api-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|List of API keys|[APIKeyListResponse](schemas.md#schemaapikeylistresponse)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|RestAPI not found|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

## Regenerate API key for an API

<a id="opIdregenerateAPIKey"></a>

`POST /rest-apis/{id}/api-keys/{apiKeyName}/regenerate`

> Code samples

```shell

curl -X POST http://localhost:9090/rest-apis/{id}/api-keys/{apiKeyName}/regenerate \
  -u {username}:{password} \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -d @payload.json

```

Regenerate an existing API key for a RestAPI in the Gateway. The previous key is revoked and replaced with a new 32-byte random value encoded in hexadecimal, prefixed with `apip_`.

> Payload

```json
{}
```

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

Required roles: `admin`, `consumer`

</aside>

<h3 id="regenerate-api-key-for-an-api-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|string|true|Unique public identifier of the API to generate the key for|
|apiKeyName|path|string|true|Name of the API key to regenerate|
|body|body|[APIKeyRegenerationRequest](schemas.md#schemaapikeyregenerationrequest)|true|none|

#### Detailed descriptions

**id**: Unique public identifier of the API to generate the key for

**apiKeyName**: Name of the API key to regenerate

> Example responses

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

<h3 id="regenerate-api-key-for-an-api-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|API key rotated successfully|[APIKeyCreationResponse](schemas.md#schemaapikeycreationresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Invalid configuration (validation failed)|[ErrorResponse](schemas.md#schemaerrorresponse)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|RestAPI not found|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

## Update an API key with a new regenerated value

<a id="opIdupdateAPIKey"></a>

`PUT /rest-apis/{id}/api-keys/{apiKeyName}`

> Code samples

```shell

curl -X PUT http://localhost:9090/rest-apis/{id}/api-keys/{apiKeyName} \
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

<a id="opIdrevokeAPIKey"></a>

`DELETE /rest-apis/{id}/api-keys/{apiKeyName}`

> Code samples

```shell

curl -X DELETE http://localhost:9090/rest-apis/{id}/api-keys/{apiKeyName} \
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
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|RestAPI not found|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

## Create a subscription plan

<a id="opIdcreateSubscriptionPlan"></a>

`POST /subscription-plans`

> Code samples

```shell

curl -X POST http://localhost:9090/subscription-plans \
  -u {username}:{password} \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -d @payload.json

```

Create a subscription plan that defines rate limits and access tiers for API subscriptions.

> Payload

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

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

Required roles: `admin`, `developer`

</aside>

<h3 id="create-a-subscription-plan-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|[SubscriptionPlanCreateRequest](schemas.md#schemasubscriptionplancreaterequest)|true|none|

> Example responses

> 201 Response

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

<h3 id="create-a-subscription-plan-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|201|[Created](https://tools.ietf.org/html/rfc7231#section-6.3.2)|Subscription plan created|[SubscriptionPlanResponse](schemas.md#schemasubscriptionplanresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request|[ErrorResponse](schemas.md#schemaerrorresponse)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Conflict|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

## List subscription plans

<a id="opIdlistSubscriptionPlans"></a>

`GET /subscription-plans`

> Code samples

```shell

curl -X GET http://localhost:9090/subscription-plans \
  -u {username}:{password} \
  -H 'Accept: application/json'

```

List all subscription plans available in the Gateway.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

Required roles: `admin`, `developer`

</aside>

> Example responses

> 200 Response

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

<h3 id="list-subscription-plans-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|List of subscription plans|[SubscriptionPlanListResponse](schemas.md#schemasubscriptionplanlistresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

## Get a subscription plan by ID

<a id="opIdgetSubscriptionPlan"></a>

`GET /subscription-plans/{planId}`

> Code samples

```shell

curl -X GET http://localhost:9090/subscription-plans/{planId} \
  -u {username}:{password} \
  -H 'Accept: application/json'

```

Get the details of a subscription plan by its ID.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

Required roles: `admin`, `developer`

</aside>

<h3 id="get-a-subscription-plan-by-id-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|planId|path|string|true|none|

> Example responses

> 200 Response

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

<h3 id="get-a-subscription-plan-by-id-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Subscription plan details|[SubscriptionPlanResponse](schemas.md#schemasubscriptionplanresponse)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not found|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

## Update a subscription plan

<a id="opIdupdateSubscriptionPlan"></a>

`PUT /subscription-plans/{planId}`

> Code samples

```shell

curl -X PUT http://localhost:9090/subscription-plans/{planId} \
  -u {username}:{password} \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -d @payload.json

```

Update an existing subscription plan in the Gateway.

> Payload

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

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

Required roles: `admin`, `developer`

</aside>

<h3 id="update-a-subscription-plan-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|planId|path|string|true|none|
|body|body|[SubscriptionPlanUpdateRequest](schemas.md#schemasubscriptionplanupdaterequest)|false|none|

> Example responses

> 200 Response

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

<h3 id="update-a-subscription-plan-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Subscription plan updated|[SubscriptionPlanResponse](schemas.md#schemasubscriptionplanresponse)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not found|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

## Delete a subscription plan

<a id="opIddeleteSubscriptionPlan"></a>

`DELETE /subscription-plans/{planId}`

> Code samples

```shell

curl -X DELETE http://localhost:9090/subscription-plans/{planId} \
  -u {username}:{password} \
  -H 'Accept: application/json'

```

Delete a subscription plan from the Gateway.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

Required roles: `admin`, `developer`

</aside>

<h3 id="delete-a-subscription-plan-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|planId|path|string|true|none|

> Example responses

> 404 Response

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

<h3 id="delete-a-subscription-plan-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|204|[No Content](https://tools.ietf.org/html/rfc7231#section-6.3.5)|Subscription plan deleted|None|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not found|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

## Create a subscription

<a id="opIdcreateSubscription"></a>

`POST /subscriptions`

> Code samples

```shell

curl -X POST http://localhost:9090/subscriptions \
  -u {username}:{password} \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -d @payload.json

```

Subscribe an application to a RestAPI in the Gateway.

> Payload

```json
{
  "apiId": "c9f2b6ae-1234-5678-9abc-def012345678",
  "subscriptionToken": "sub-token-abc123xyz",
  "applicationId": "string",
  "subscriptionPlanId": "string",
  "status": "ACTIVE"
}
```

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

Required roles: `admin`, `developer`

</aside>

<h3 id="create-a-subscription-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|[SubscriptionCreateRequest](schemas.md#schemasubscriptioncreaterequest)|true|none|

> Example responses

> 201 Response

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

<h3 id="create-a-subscription-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|201|[Created](https://tools.ietf.org/html/rfc7231#section-6.3.2)|Subscription created|[SubscriptionResponse](schemas.md#schemasubscriptionresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request|[ErrorResponse](schemas.md#schemaerrorresponse)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Conflict - subscription already exists|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

## List subscriptions

<a id="opIdlistSubscriptions"></a>

`GET /subscriptions`

> Code samples

```shell

curl -X GET http://localhost:9090/subscriptions \
  -u {username}:{password} \
  -H 'Accept: application/json'

```

List subscriptions in the Gateway, optionally filtered by API, application, or status.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

Required roles: `admin`, `developer`

</aside>

<h3 id="list-subscriptions-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|apiId|query|string|false|Filter by API ID (deployment ID or handle)|
|applicationId|query|string|false|none|
|status|query|string|false|none|

#### Enumerated Values

|Parameter|Value|
|---|---|
|status|ACTIVE|
|status|INACTIVE|
|status|REVOKED|

> Example responses

> 200 Response

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

<h3 id="list-subscriptions-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|List of subscriptions|[SubscriptionListResponse](schemas.md#schemasubscriptionlistresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

## Get a subscription by ID

<a id="opIdgetSubscription"></a>

`GET /subscriptions/{subscriptionId}`

> Code samples

```shell

curl -X GET http://localhost:9090/subscriptions/{subscriptionId} \
  -u {username}:{password} \
  -H 'Accept: application/json'

```

Get the details of a subscription by its ID.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

Required roles: `admin`, `developer`

</aside>

<h3 id="get-a-subscription-by-id-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|subscriptionId|path|string|true|none|

> Example responses

> 200 Response

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

<h3 id="get-a-subscription-by-id-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Subscription details|[SubscriptionResponse](schemas.md#schemasubscriptionresponse)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Subscription not found|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

## Update a subscription

<a id="opIdupdateSubscription"></a>

`PUT /subscriptions/{subscriptionId}`

> Code samples

```shell

curl -X PUT http://localhost:9090/subscriptions/{subscriptionId} \
  -u {username}:{password} \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -d @payload.json

```

Update an existing subscription in the Gateway.

> Payload

```json
{
  "status": "ACTIVE"
}
```

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

Required roles: `admin`, `developer`

</aside>

<h3 id="update-a-subscription-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|subscriptionId|path|string|true|none|
|body|body|[SubscriptionUpdateRequest](schemas.md#schemasubscriptionupdaterequest)|false|none|

> Example responses

> 200 Response

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

<h3 id="update-a-subscription-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Subscription updated|[SubscriptionResponse](schemas.md#schemasubscriptionresponse)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Subscription not found|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

## Delete a subscription

<a id="opIddeleteSubscription"></a>

`DELETE /subscriptions/{subscriptionId}`

> Code samples

```shell

curl -X DELETE http://localhost:9090/subscriptions/{subscriptionId} \
  -u {username}:{password} \
  -H 'Accept: application/json'

```

Delete a subscription from the Gateway.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

Required roles: `admin`, `developer`

</aside>

<h3 id="delete-a-subscription-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|subscriptionId|path|string|true|none|

> Example responses

> 404 Response

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

<h3 id="delete-a-subscription-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|204|[No Content](https://tools.ietf.org/html/rfc7231#section-6.3.5)|Subscription deleted|None|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Subscription not found|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|
