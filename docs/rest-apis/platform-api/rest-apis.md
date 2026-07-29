<h1 id="wso2-api-platform-platform-api-rest-apis">REST APIs</h1>

API management operations

## Get all REST APIs for an organization

<a id="opIdListRESTAPIs"></a>

`GET /rest-apis`

> Code samples

```shell

curl -X GET https://localhost:9243/api/v0.9/rest-apis?projectId=default-project \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Accept: application/json'

```

Retrieves all REST APIs belonging to an organization.
Requires the projectId query parameter to filter APIs by project.
Provide name and version together to check uniqueness (empty result = name/version combination is available).
Access is validated against the organization in the JWT token.

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:rest_api:read`, `ap:rest_api:manage`

</aside>

<h3 id="get-all-rest-apis-for-an-organization-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|projectId|query|string|true|**Project ID** consisting of the **handle** (unique slug identifier) of the Project to filter APIs by.|
|limit|query|integer|false|Maximum number of items to return per page.|
|offset|query|integer|false|Zero-based index of the first item to return.|
|sortBy|query|string|false|Field to sort the collection by. An unrecognized value falls back to the default sort (createdAt).|
|sortOrder|query|string|false|Sort direction applied to `sortBy`.|
|query|query|string|false|Case-insensitive substring filter matched against the resource id (handle).|

#### Detailed descriptions

**projectId**: **Project ID** consisting of the **handle** (unique slug identifier) of the Project to filter APIs by.

#### Enumerated Values

|Parameter|Value|
|---|---|
|sortBy|name|
|sortBy|createdAt|
|sortOrder|asc|
|sortOrder|desc|

> Example responses

> 200 Response

```json
{
  "count": 2,
  "list": [
    {
      "id": "my-rest-api-handle",
      "displayName": "PizzaShackAPI",
      "description": "This is a simple API for Pizza Shack online pizza delivery store",
      "context": "pizza",
      "version": "1.0.0",
      "createdBy": "john.doe",
      "updatedBy": "john.doe",
      "projectId": "default-project",
      "createdAt": "2023-10-12T10:30:00Z",
      "updatedAt": "2023-10-12T10:30:00Z",
      "readOnly": false,
      "upstream": {
        "main": {
          "url": "http://prod-backend:5000/api/v2",
          "ref": "string",
          "auth": {
            "type": "api-key",
            "header": "X-API-Key"
          }
        },
        "sandbox": {
          "url": "http://prod-backend:5000/api/v2",
          "ref": "string",
          "auth": {
            "type": "api-key",
            "header": "X-API-Key"
          }
        }
      },
      "lifeCycleStatus": "CREATED",
      "kind": "RestApi",
      "transport": [
        "http",
        "https"
      ],
      "policies": [
        {
          "executionCondition": "request.header.x-custom == 'enabled'",
          "name": "SET_HEADER",
          "params": {
            "key": "MyHeader",
            "value": "MyValue"
          },
          "version": "v1"
        }
      ],
      "operations": [
        {
          "name": "getPetById",
          "description": "Find pet by ID",
          "request": {
            "method": "GET",
            "path": "/pet/{petId}",
            "policies": [
              {
                "executionCondition": "request.header.x-custom == 'enabled'",
                "name": "SET_HEADER",
                "params": {
                  "key": "MyHeader",
                  "value": "MyValue"
                },
                "version": "v1"
              }
            ]
          }
        }
      ],
      "channels": [
        {
          "name": "issues",
          "description": "Channel for order events",
          "request": {
            "method": "SUB",
            "name": "issues",
            "policies": [
              {
                "executionCondition": "request.header.x-custom == 'enabled'",
                "name": "SET_HEADER",
                "params": {
                  "key": "MyHeader",
                  "value": "MyValue"
                },
                "version": "v1"
              }
            ]
          }
        }
      ],
      "subscriptionPlans": [
        "Gold",
        "Silver"
      ]
    }
  ],
  "pagination": {
    "total": 10,
    "offset": 0,
    "limit": 10
  }
}
```

> 400 Response

```json
{
  "status": "error",
  "code": "VALIDATION_FAILED",
  "message": "The request failed validation.",
  "errors": [
    {
      "field": "spec.context",
      "message": "must start with /"
    }
  ]
}
```

> 401 Response

```json
{
  "status": "error",
  "code": "UNAUTHORIZED",
  "message": "Authorization header is required, or the token is invalid or expired."
}
```

> 404 Response

```json
{
  "status": "error",
  "code": "NOT_FOUND",
  "message": "The specified resource does not exist."
}
```

> 500 Response

```json
{
  "status": "error",
  "code": "INTERNAL_ERROR",
  "message": "An unexpected error occurred.",
  "trackingId": "4f1c6f2e-8a4b-4c93-b1de-9f2f6f0c2a11"
}
```

<h3 id="get-all-rest-apis-for-an-organization-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|REST APIs retrieved successfully|[RESTAPIListResponse](schemas.md#schemarestapilistresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad Request. Invalid request or validation error.|[Error](schemas.md#schemaerror)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|

## Create a new REST API

<a id="opIdCreateRESTAPI"></a>

`POST /rest-apis`

> Code samples

```shell

curl -X POST https://localhost:9243/api/v0.9/rest-apis \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -d @payload.json

```

Creates a new REST API in the platform. The API is associated with a project, which must 
belong to the organization specified in the JWT token.

> Payload

```json
{
  "id": "my-rest-api-handle",
  "displayName": "PizzaShackAPI",
  "description": "This is a simple API for Pizza Shack online pizza delivery store",
  "context": "pizza",
  "version": "1.0.0",
  "projectId": "default-project",
  "upstream": {
    "main": {
      "url": "http://prod-backend:5000/api/v2",
      "ref": "string",
      "auth": {
        "type": "api-key",
        "header": "X-API-Key",
        "value": "my-api-key-value"
      }
    },
    "sandbox": {
      "url": "http://prod-backend:5000/api/v2",
      "ref": "string",
      "auth": {
        "type": "api-key",
        "header": "X-API-Key",
        "value": "my-api-key-value"
      }
    }
  },
  "lifeCycleStatus": "CREATED",
  "kind": "RestApi",
  "transport": [
    "http",
    "https"
  ],
  "policies": [
    {
      "executionCondition": "request.header.x-custom == 'enabled'",
      "name": "SET_HEADER",
      "params": {
        "key": "MyHeader",
        "value": "MyValue"
      },
      "version": "v1"
    }
  ],
  "operations": [
    {
      "name": "getPetById",
      "description": "Find pet by ID",
      "request": {
        "method": "GET",
        "path": "/pet/{petId}",
        "policies": [
          {
            "executionCondition": "request.header.x-custom == 'enabled'",
            "name": "SET_HEADER",
            "params": {
              "key": "MyHeader",
              "value": "MyValue"
            },
            "version": "v1"
          }
        ]
      }
    }
  ],
  "channels": [
    {
      "name": "issues",
      "description": "Channel for order events",
      "request": {
        "method": "SUB",
        "name": "issues",
        "policies": [
          {
            "executionCondition": "request.header.x-custom == 'enabled'",
            "name": "SET_HEADER",
            "params": {
              "key": "MyHeader",
              "value": "MyValue"
            },
            "version": "v1"
          }
        ]
      }
    }
  ],
  "subscriptionPlans": [
    "Gold",
    "Silver"
  ]
}
```

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:rest_api:create`, `ap:rest_api:manage`

</aside>

<h3 id="create-a-new-rest-api-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|[CreateRESTAPIRequest](schemas.md#schemacreaterestapirequest)|true|API object that needs to be added|

> Example responses

> 201 Response

```json
{
  "id": "my-rest-api-handle",
  "displayName": "PizzaShackAPI",
  "description": "This is a simple API for Pizza Shack online pizza delivery store",
  "context": "pizza",
  "version": "1.0.0",
  "createdBy": "john.doe",
  "updatedBy": "john.doe",
  "projectId": "default-project",
  "createdAt": "2023-10-12T10:30:00Z",
  "updatedAt": "2023-10-12T10:30:00Z",
  "readOnly": false,
  "upstream": {
    "main": {
      "url": "http://prod-backend:5000/api/v2",
      "ref": "string",
      "auth": {
        "type": "api-key",
        "header": "X-API-Key"
      }
    },
    "sandbox": {
      "url": "http://prod-backend:5000/api/v2",
      "ref": "string",
      "auth": {
        "type": "api-key",
        "header": "X-API-Key"
      }
    }
  },
  "lifeCycleStatus": "CREATED",
  "kind": "RestApi",
  "transport": [
    "http",
    "https"
  ],
  "policies": [
    {
      "executionCondition": "request.header.x-custom == 'enabled'",
      "name": "SET_HEADER",
      "params": {
        "key": "MyHeader",
        "value": "MyValue"
      },
      "version": "v1"
    }
  ],
  "operations": [
    {
      "name": "getPetById",
      "description": "Find pet by ID",
      "request": {
        "method": "GET",
        "path": "/pet/{petId}",
        "policies": [
          {
            "executionCondition": "request.header.x-custom == 'enabled'",
            "name": "SET_HEADER",
            "params": {
              "key": "MyHeader",
              "value": "MyValue"
            },
            "version": "v1"
          }
        ]
      }
    }
  ],
  "channels": [
    {
      "name": "issues",
      "description": "Channel for order events",
      "request": {
        "method": "SUB",
        "name": "issues",
        "policies": [
          {
            "executionCondition": "request.header.x-custom == 'enabled'",
            "name": "SET_HEADER",
            "params": {
              "key": "MyHeader",
              "value": "MyValue"
            },
            "version": "v1"
          }
        ]
      }
    }
  ],
  "subscriptionPlans": [
    "Gold",
    "Silver"
  ]
}
```

> 400 Response

```json
{
  "status": "error",
  "code": "VALIDATION_FAILED",
  "message": "The request failed validation.",
  "errors": [
    {
      "field": "spec.context",
      "message": "must start with /"
    }
  ]
}
```

> 401 Response

```json
{
  "status": "error",
  "code": "UNAUTHORIZED",
  "message": "Authorization header is required, or the token is invalid or expired."
}
```

> 403 Response

```json
{
  "status": "error",
  "code": "FORBIDDEN",
  "message": "You do not have permission to perform this action."
}
```

> 404 Response

```json
{
  "status": "error",
  "code": "NOT_FOUND",
  "message": "The specified resource does not exist."
}
```

> 409 Response

```json
{
  "status": "error",
  "code": "CONFLICT",
  "message": "The specified resource already exists."
}
```

> 500 Response

```json
{
  "status": "error",
  "code": "INTERNAL_ERROR",
  "message": "An unexpected error occurred.",
  "trackingId": "4f1c6f2e-8a4b-4c93-b1de-9f2f6f0c2a11"
}
```

<h3 id="create-a-new-rest-api-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|201|[Created](https://tools.ietf.org/html/rfc7231#section-6.3.2)|API created successfully|[RESTAPI](schemas.md#schemarestapi)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad Request. Invalid request or validation error.|[Error](schemas.md#schemaerror)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Forbidden. The authenticated user does not have permission to access this resource.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Conflict. Specified resource already exists.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|

### Response Headers

|Status|Header|Type|Format|Description|
|---|---|---|---|---|
|201|Location|string|uri|URL of the newly created resource.|

## Get REST API by UUID

<a id="opIdGetRESTAPI"></a>

`GET /rest-apis/{restApiId}`

> Code samples

```shell

curl -X GET https://localhost:9243/api/v0.9/rest-apis/{restApiId} \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Accept: application/json'

```

Retrieves a specific API by its UUID. Access is validated against the organization 
in the JWT token.

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:rest_api:read`, `ap:rest_api:manage`

</aside>

<h3 id="get-rest-api-by-uuid-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|restApiId|path|string|true|**API ID** consisting of the **handle** (unique identifier) of the API.|

#### Detailed descriptions

**restApiId**: **API ID** consisting of the **handle** (unique identifier) of the API.

> Example responses

> 200 Response

```json
{
  "id": "my-rest-api-handle",
  "displayName": "PizzaShackAPI",
  "description": "This is a simple API for Pizza Shack online pizza delivery store",
  "context": "pizza",
  "version": "1.0.0",
  "createdBy": "john.doe",
  "updatedBy": "john.doe",
  "projectId": "default-project",
  "createdAt": "2023-10-12T10:30:00Z",
  "updatedAt": "2023-10-12T10:30:00Z",
  "readOnly": false,
  "upstream": {
    "main": {
      "url": "http://prod-backend:5000/api/v2",
      "ref": "string",
      "auth": {
        "type": "api-key",
        "header": "X-API-Key"
      }
    },
    "sandbox": {
      "url": "http://prod-backend:5000/api/v2",
      "ref": "string",
      "auth": {
        "type": "api-key",
        "header": "X-API-Key"
      }
    }
  },
  "lifeCycleStatus": "CREATED",
  "kind": "RestApi",
  "transport": [
    "http",
    "https"
  ],
  "policies": [
    {
      "executionCondition": "request.header.x-custom == 'enabled'",
      "name": "SET_HEADER",
      "params": {
        "key": "MyHeader",
        "value": "MyValue"
      },
      "version": "v1"
    }
  ],
  "operations": [
    {
      "name": "getPetById",
      "description": "Find pet by ID",
      "request": {
        "method": "GET",
        "path": "/pet/{petId}",
        "policies": [
          {
            "executionCondition": "request.header.x-custom == 'enabled'",
            "name": "SET_HEADER",
            "params": {
              "key": "MyHeader",
              "value": "MyValue"
            },
            "version": "v1"
          }
        ]
      }
    }
  ],
  "channels": [
    {
      "name": "issues",
      "description": "Channel for order events",
      "request": {
        "method": "SUB",
        "name": "issues",
        "policies": [
          {
            "executionCondition": "request.header.x-custom == 'enabled'",
            "name": "SET_HEADER",
            "params": {
              "key": "MyHeader",
              "value": "MyValue"
            },
            "version": "v1"
          }
        ]
      }
    }
  ],
  "subscriptionPlans": [
    "Gold",
    "Silver"
  ]
}
```

> 400 Response

```json
{
  "status": "error",
  "code": "VALIDATION_FAILED",
  "message": "The request failed validation.",
  "errors": [
    {
      "field": "spec.context",
      "message": "must start with /"
    }
  ]
}
```

> 401 Response

```json
{
  "status": "error",
  "code": "UNAUTHORIZED",
  "message": "Authorization header is required, or the token is invalid or expired."
}
```

> 404 Response

```json
{
  "status": "error",
  "code": "NOT_FOUND",
  "message": "The specified resource does not exist."
}
```

> 500 Response

```json
{
  "status": "error",
  "code": "INTERNAL_ERROR",
  "message": "An unexpected error occurred.",
  "trackingId": "4f1c6f2e-8a4b-4c93-b1de-9f2f6f0c2a11"
}
```

<h3 id="get-rest-api-by-uuid-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|API retrieved successfully|[RESTAPI](schemas.md#schemarestapi)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad Request. Invalid request or validation error.|[Error](schemas.md#schemaerror)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|

## Update REST API

<a id="opIdUpdateRESTAPI"></a>

`PUT /rest-apis/{restApiId}`

> Code samples

```shell

curl -X PUT https://localhost:9243/api/v0.9/rest-apis/{restApiId} \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -d @payload.json

```

Updates an existing API's details. Access is validated against the organization 
in the JWT token.

> Payload

```json
{
  "id": "my-rest-api-handle",
  "displayName": "PizzaShackAPI",
  "description": "This is a simple API for Pizza Shack online pizza delivery store",
  "context": "pizza",
  "version": "1.0.0",
  "projectId": "default-project",
  "upstream": {
    "main": {
      "url": "http://prod-backend:5000/api/v2",
      "ref": "string",
      "auth": {
        "type": "api-key",
        "header": "X-API-Key",
        "value": "my-api-key-value"
      }
    },
    "sandbox": {
      "url": "http://prod-backend:5000/api/v2",
      "ref": "string",
      "auth": {
        "type": "api-key",
        "header": "X-API-Key",
        "value": "my-api-key-value"
      }
    }
  },
  "lifeCycleStatus": "CREATED",
  "kind": "RestApi",
  "transport": [
    "http",
    "https"
  ],
  "policies": [
    {
      "executionCondition": "request.header.x-custom == 'enabled'",
      "name": "SET_HEADER",
      "params": {
        "key": "MyHeader",
        "value": "MyValue"
      },
      "version": "v1"
    }
  ],
  "operations": [
    {
      "name": "getPetById",
      "description": "Find pet by ID",
      "request": {
        "method": "GET",
        "path": "/pet/{petId}",
        "policies": [
          {
            "executionCondition": "request.header.x-custom == 'enabled'",
            "name": "SET_HEADER",
            "params": {
              "key": "MyHeader",
              "value": "MyValue"
            },
            "version": "v1"
          }
        ]
      }
    }
  ],
  "channels": [
    {
      "name": "issues",
      "description": "Channel for order events",
      "request": {
        "method": "SUB",
        "name": "issues",
        "policies": [
          {
            "executionCondition": "request.header.x-custom == 'enabled'",
            "name": "SET_HEADER",
            "params": {
              "key": "MyHeader",
              "value": "MyValue"
            },
            "version": "v1"
          }
        ]
      }
    }
  ],
  "subscriptionPlans": [
    "Gold",
    "Silver"
  ]
}
```

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:rest_api:update`, `ap:rest_api:manage`

</aside>

<h3 id="update-rest-api-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|restApiId|path|string|true|**API ID** consisting of the **handle** (unique identifier) of the API.|
|body|body|[RESTAPI](schemas.md#schemarestapi)|true|API object that needs to be updated|

#### Detailed descriptions

**restApiId**: **API ID** consisting of the **handle** (unique identifier) of the API.

> Example responses

> 200 Response

```json
{
  "id": "my-rest-api-handle",
  "displayName": "PizzaShackAPI",
  "description": "This is a simple API for Pizza Shack online pizza delivery store",
  "context": "pizza",
  "version": "1.0.0",
  "createdBy": "john.doe",
  "updatedBy": "john.doe",
  "projectId": "default-project",
  "createdAt": "2023-10-12T10:30:00Z",
  "updatedAt": "2023-10-12T10:30:00Z",
  "readOnly": false,
  "upstream": {
    "main": {
      "url": "http://prod-backend:5000/api/v2",
      "ref": "string",
      "auth": {
        "type": "api-key",
        "header": "X-API-Key"
      }
    },
    "sandbox": {
      "url": "http://prod-backend:5000/api/v2",
      "ref": "string",
      "auth": {
        "type": "api-key",
        "header": "X-API-Key"
      }
    }
  },
  "lifeCycleStatus": "CREATED",
  "kind": "RestApi",
  "transport": [
    "http",
    "https"
  ],
  "policies": [
    {
      "executionCondition": "request.header.x-custom == 'enabled'",
      "name": "SET_HEADER",
      "params": {
        "key": "MyHeader",
        "value": "MyValue"
      },
      "version": "v1"
    }
  ],
  "operations": [
    {
      "name": "getPetById",
      "description": "Find pet by ID",
      "request": {
        "method": "GET",
        "path": "/pet/{petId}",
        "policies": [
          {
            "executionCondition": "request.header.x-custom == 'enabled'",
            "name": "SET_HEADER",
            "params": {
              "key": "MyHeader",
              "value": "MyValue"
            },
            "version": "v1"
          }
        ]
      }
    }
  ],
  "channels": [
    {
      "name": "issues",
      "description": "Channel for order events",
      "request": {
        "method": "SUB",
        "name": "issues",
        "policies": [
          {
            "executionCondition": "request.header.x-custom == 'enabled'",
            "name": "SET_HEADER",
            "params": {
              "key": "MyHeader",
              "value": "MyValue"
            },
            "version": "v1"
          }
        ]
      }
    }
  ],
  "subscriptionPlans": [
    "Gold",
    "Silver"
  ]
}
```

> 400 Response

```json
{
  "status": "error",
  "code": "VALIDATION_FAILED",
  "message": "The request failed validation.",
  "errors": [
    {
      "field": "spec.context",
      "message": "must start with /"
    }
  ]
}
```

> 401 Response

```json
{
  "status": "error",
  "code": "UNAUTHORIZED",
  "message": "Authorization header is required, or the token is invalid or expired."
}
```

> 403 Response

```json
{
  "status": "error",
  "code": "FORBIDDEN",
  "message": "You do not have permission to perform this action."
}
```

> 404 Response

```json
{
  "status": "error",
  "code": "NOT_FOUND",
  "message": "The specified resource does not exist."
}
```

> 409 Response

```json
{
  "status": "error",
  "code": "CONFLICT",
  "message": "The specified resource already exists."
}
```

> 500 Response

```json
{
  "status": "error",
  "code": "INTERNAL_ERROR",
  "message": "An unexpected error occurred.",
  "trackingId": "4f1c6f2e-8a4b-4c93-b1de-9f2f6f0c2a11"
}
```

<h3 id="update-rest-api-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|API updated successfully|[RESTAPI](schemas.md#schemarestapi)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad Request. Invalid request or validation error.|[Error](schemas.md#schemaerror)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Forbidden. The authenticated user does not have permission to access this resource.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Conflict. Specified resource already exists.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|

## Delete REST API

<a id="opIdDeleteRESTAPI"></a>

`DELETE /rest-apis/{restApiId}`

> Code samples

```shell

curl -X DELETE https://localhost:9243/api/v0.9/rest-apis/{restApiId} \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Accept: application/json'

```

Deletes a specific API by its UUID. Access is validated against the organization 
in the JWT token.

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:rest_api:delete`, `ap:rest_api:manage`

</aside>

<h3 id="delete-rest-api-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|restApiId|path|string|true|**API ID** consisting of the **handle** (unique identifier) of the API.|

#### Detailed descriptions

**restApiId**: **API ID** consisting of the **handle** (unique identifier) of the API.

> Example responses

> 400 Response

```json
{
  "status": "error",
  "code": "VALIDATION_FAILED",
  "message": "The request failed validation.",
  "errors": [
    {
      "field": "spec.context",
      "message": "must start with /"
    }
  ]
}
```

> 401 Response

```json
{
  "status": "error",
  "code": "UNAUTHORIZED",
  "message": "Authorization header is required, or the token is invalid or expired."
}
```

> 403 Response

```json
{
  "status": "error",
  "code": "FORBIDDEN",
  "message": "You do not have permission to perform this action."
}
```

> 404 Response

```json
{
  "status": "error",
  "code": "NOT_FOUND",
  "message": "The specified resource does not exist."
}
```

> 500 Response

```json
{
  "status": "error",
  "code": "INTERNAL_ERROR",
  "message": "An unexpected error occurred.",
  "trackingId": "4f1c6f2e-8a4b-4c93-b1de-9f2f6f0c2a11"
}
```

<h3 id="delete-rest-api-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|204|[No Content](https://tools.ietf.org/html/rfc7231#section-6.3.5)|API deleted successfully|None|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad Request. Invalid request or validation error.|[Error](schemas.md#schemaerror)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Forbidden. The authenticated user does not have permission to access this resource.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|

## Get gateways for REST API

<a id="opIdGetRESTAPIGateways"></a>

`GET /rest-apis/{restApiId}/gateways`

> Code samples

```shell

curl -X GET https://localhost:9243/api/v0.9/rest-apis/{restApiId}/gateways \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Accept: application/json'

```

Retrieves all gateways associated with the specified API, including deployment details.
Returns gateway information along with association timestamps and deployment status.
Access is validated against the organization in the JWT token.

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:rest_api:gateway:read`, `ap:rest_api:gateway:manage`, `ap:rest_api:manage`, `ap:gateway:read`, `ap:gateway:manage`

</aside>

<h3 id="get-gateways-for-rest-api-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|restApiId|path|string|true|**API ID** consisting of the **handle** (unique identifier) of the API.|
|limit|query|integer|false|Maximum number of items to return per page.|
|offset|query|integer|false|Zero-based index of the first item to return.|

#### Detailed descriptions

**restApiId**: **API ID** consisting of the **handle** (unique identifier) of the API.

> Example responses

> 200 Response

```json
{
  "count": 3,
  "list": [
    {
      "id": "prod-gateway-01",
      "organizationId": "acme",
      "displayName": "Production Gateway 01",
      "description": "Production gateway for handling API traffic",
      "properties": {
        "region": "us-west",
        "tier": "premium"
      },
      "endpoints": [
        "https://api.example.com:8443/api/v1",
        "wss://events.example.com:8444"
      ],
      "isCritical": true,
      "functionalityType": "regular",
      "version": "1.0",
      "isActive": true,
      "createdBy": "john.doe",
      "updatedBy": "john.doe",
      "createdAt": "2025-10-14T10:30:00Z",
      "updatedAt": "2025-10-14T10:30:00Z",
      "associatedAt": "2025-10-15T10:30:00Z",
      "isDeployed": true,
      "deployment": {
        "status": "CREATED",
        "deployedAt": "2025-10-15T11:00:00Z"
      }
    }
  ],
  "pagination": {
    "total": 10,
    "offset": 0,
    "limit": 10
  }
}
```

> 400 Response

```json
{
  "status": "error",
  "code": "VALIDATION_FAILED",
  "message": "The request failed validation.",
  "errors": [
    {
      "field": "spec.context",
      "message": "must start with /"
    }
  ]
}
```

> 401 Response

```json
{
  "status": "error",
  "code": "UNAUTHORIZED",
  "message": "Authorization header is required, or the token is invalid or expired."
}
```

> 404 Response

```json
{
  "status": "error",
  "code": "NOT_FOUND",
  "message": "The specified resource does not exist."
}
```

> 500 Response

```json
{
  "status": "error",
  "code": "INTERNAL_ERROR",
  "message": "An unexpected error occurred.",
  "trackingId": "4f1c6f2e-8a4b-4c93-b1de-9f2f6f0c2a11"
}
```

<h3 id="get-gateways-for-rest-api-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|List of gateways associated with the API, including deployment details|[RESTAPIGatewayListResponse](schemas.md#schemarestapigatewaylistresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad Request. Invalid request or validation error.|[Error](schemas.md#schemaerror)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|

## Add gateways for REST API

<a id="opIdAddGatewaysToAPI"></a>

`POST /rest-apis/{restApiId}/gateways`

> Code samples

```shell

curl -X POST https://localhost:9243/api/v0.9/rest-apis/{restApiId}/gateways \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -d @payload.json

```

Associates gateways to the specified API. If gateways are already associated,
updates the association timestamp. Returns all gateways associated with the API
including deployment details. Access is validated against the organization 
in the JWT token.

> Payload

```json
[
  {
    "gatewayId": "prod-gateway-01"
  }
]
```

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:rest_api:gateway:create`, `ap:rest_api:gateway:manage`, `ap:rest_api:manage`

</aside>

<h3 id="add-gateways-for-rest-api-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|restApiId|path|string|true|**API ID** consisting of the **handle** (unique identifier) of the API.|
|body|body|[AddGatewayToRESTAPIRequest](schemas.md#schemaaddgatewaytorestapirequest)|false|List of gateways to associate with the API|

#### Detailed descriptions

**restApiId**: **API ID** consisting of the **handle** (unique identifier) of the API.

> Example responses

> 200 Response

```json
{
  "count": 3,
  "list": [
    {
      "id": "prod-gateway-01",
      "organizationId": "acme",
      "displayName": "Production Gateway 01",
      "description": "Production gateway for handling API traffic",
      "properties": {
        "region": "us-west",
        "tier": "premium"
      },
      "endpoints": [
        "https://api.example.com:8443/api/v1",
        "wss://events.example.com:8444"
      ],
      "isCritical": true,
      "functionalityType": "regular",
      "version": "1.0",
      "isActive": true,
      "createdBy": "john.doe",
      "updatedBy": "john.doe",
      "createdAt": "2025-10-14T10:30:00Z",
      "updatedAt": "2025-10-14T10:30:00Z",
      "associatedAt": "2025-10-15T10:30:00Z",
      "isDeployed": true,
      "deployment": {
        "status": "CREATED",
        "deployedAt": "2025-10-15T11:00:00Z"
      }
    }
  ],
  "pagination": {
    "total": 10,
    "offset": 0,
    "limit": 10
  }
}
```

> 400 Response

```json
{
  "status": "error",
  "code": "VALIDATION_FAILED",
  "message": "The request failed validation.",
  "errors": [
    {
      "field": "spec.context",
      "message": "must start with /"
    }
  ]
}
```

> 401 Response

```json
{
  "status": "error",
  "code": "UNAUTHORIZED",
  "message": "Authorization header is required, or the token is invalid or expired."
}
```

> 403 Response

```json
{
  "status": "error",
  "code": "FORBIDDEN",
  "message": "You do not have permission to perform this action."
}
```

> 404 Response

```json
{
  "status": "error",
  "code": "NOT_FOUND",
  "message": "The specified resource does not exist."
}
```

> 500 Response

```json
{
  "status": "error",
  "code": "INTERNAL_ERROR",
  "message": "An unexpected error occurred.",
  "trackingId": "4f1c6f2e-8a4b-4c93-b1de-9f2f6f0c2a11"
}
```

<h3 id="add-gateways-for-rest-api-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|List of all gateways associated with the API, including deployment details|[RESTAPIGatewayListResponse](schemas.md#schemarestapigatewaylistresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad Request. Invalid request or validation error.|[Error](schemas.md#schemaerror)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Forbidden. The authenticated user does not have permission to access this resource.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|

## Create API key

<a id="opIdCreateAPIKey"></a>

`POST /rest-apis/{restApiId}/api-keys`

> Code samples

```shell

curl -X POST https://localhost:9243/api/v0.9/rest-apis/{restApiId}/api-keys \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -d @payload.json

```

Creates a new API key for the specified API. The API key will be hashed before
storage and broadcasted to all gateways where the API is deployed. This endpoint
allows external platforms to inject API keys to hybrid gateways.

> Payload

```json
{
  "id": "production-key-01",
  "displayName": "Production API Key",
  "apiKey": "sk_example_1234567890abcdef",
  "externalRefId": "ext-ref-12345",
  "expiresAt": "2026-12-31T23:59:59Z",
  "expiresIn": {
    "duration": 30,
    "unit": "days"
  },
  "issuer": "api-platform-devportal"
}
```

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:rest_api:api_key:create`, `ap:rest_api:api_key:manage`, `ap:rest_api:manage`

</aside>

<h3 id="create-api-key-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|restApiId|path|string|true|**API ID** consisting of the **handle** (unique identifier) of the API.|
|body|body|[CreateAPIKeyRequest](schemas.md#schemacreateapikeyrequest)|true|API key creation request|

#### Detailed descriptions

**restApiId**: **API ID** consisting of the **handle** (unique identifier) of the API.

> Example responses

> 201 Response

```json
{
  "status": "success",
  "message": "API key created and broadcasted to gateways successfully",
  "keyId": "production-key-01"
}
```

> 400 Response

```json
{
  "status": "error",
  "code": "VALIDATION_FAILED",
  "message": "The request failed validation.",
  "errors": [
    {
      "field": "spec.context",
      "message": "must start with /"
    }
  ]
}
```

> 401 Response

```json
{
  "status": "error",
  "code": "UNAUTHORIZED",
  "message": "Authorization header is required, or the token is invalid or expired."
}
```

> 403 Response

```json
{
  "status": "error",
  "code": "FORBIDDEN",
  "message": "You do not have permission to perform this action."
}
```

> 404 Response

```json
{
  "status": "error",
  "code": "NOT_FOUND",
  "message": "The specified resource does not exist."
}
```

> 500 Response

```json
{
  "status": "error",
  "code": "INTERNAL_ERROR",
  "message": "An unexpected error occurred.",
  "trackingId": "4f1c6f2e-8a4b-4c93-b1de-9f2f6f0c2a11"
}
```

> 503 Response

```json
{
  "status": "error",
  "code": "GATEWAY_CONNECTION_UNAVAILABLE",
  "message": "No gateway connections are currently available.",
  "trackingId": "4f1c6f2e-8a4b-4c93-b1de-9f2f6f0c2a11"
}
```

<h3 id="create-api-key-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|201|[Created](https://tools.ietf.org/html/rfc7231#section-6.3.2)|API key created successfully|[CreateAPIKeyResponse](schemas.md#schemacreateapikeyresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad Request. Invalid request or validation error.|[Error](schemas.md#schemaerror)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Forbidden. The authenticated user does not have permission to access this resource.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|
|503|[Service Unavailable](https://tools.ietf.org/html/rfc7231#section-6.6.4)|Service Unavailable. No gateway connections are currently available to service this request.|[Error](schemas.md#schemaerror)|

### Response Headers

|Status|Header|Type|Format|Description|
|---|---|---|---|---|
|201|Location|string|uri|URL of the newly created resource.|

## Update API key

<a id="opIdUpdateAPIKey"></a>

`PUT /rest-apis/{restApiId}/api-keys/{apiKeyId}`

> Code samples

```shell

curl -X PUT https://localhost:9243/api/v0.9/rest-apis/{restApiId}/api-keys/{apiKeyId} \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -d @payload.json

```

Updates an existing API key for the specified API. The new API key value will
be hashed before storage and broadcasted to all gateways where the API is deployed.
This endpoint allows external platforms to rotate API keys on hybrid gateways.

> Payload

```json
{
  "name": "production-key-01",
  "displayName": "Production API Key (Updated)",
  "apiKey": "sk_example_new1234567890abcdef",
  "externalRefId": "ext-ref-12345",
  "expiresAt": "2027-12-31T23:59:59Z",
  "expiresIn": {
    "duration": 30,
    "unit": "days"
  },
  "issuer": "api-platform-devportal"
}
```

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:rest_api:api_key:update`, `ap:rest_api:api_key:manage`, `ap:rest_api:manage`

</aside>

<h3 id="update-api-key-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|restApiId|path|string|true|**API ID** consisting of the **handle** (unique identifier) of the API.|
|apiKeyId|path|string|true|The unique name/identifier of the API key|
|body|body|[UpdateAPIKeyRequest](schemas.md#schemaupdateapikeyrequest)|true|API key update request|

#### Detailed descriptions

**restApiId**: **API ID** consisting of the **handle** (unique identifier) of the API.

> Example responses

> 200 Response

```json
{
  "status": "success",
  "message": "API key updated and broadcasted to gateways successfully",
  "keyId": "production-key-01"
}
```

> 400 Response

```json
{
  "status": "error",
  "code": "VALIDATION_FAILED",
  "message": "The request failed validation.",
  "errors": [
    {
      "field": "spec.context",
      "message": "must start with /"
    }
  ]
}
```

> 401 Response

```json
{
  "status": "error",
  "code": "UNAUTHORIZED",
  "message": "Authorization header is required, or the token is invalid or expired."
}
```

> 403 Response

```json
{
  "status": "error",
  "code": "FORBIDDEN",
  "message": "You do not have permission to perform this action."
}
```

> 404 Response

```json
{
  "status": "error",
  "code": "NOT_FOUND",
  "message": "The specified resource does not exist."
}
```

> 500 Response

```json
{
  "status": "error",
  "code": "INTERNAL_ERROR",
  "message": "An unexpected error occurred.",
  "trackingId": "4f1c6f2e-8a4b-4c93-b1de-9f2f6f0c2a11"
}
```

> 503 Response

```json
{
  "status": "error",
  "code": "GATEWAY_CONNECTION_UNAVAILABLE",
  "message": "No gateway connections are currently available.",
  "trackingId": "4f1c6f2e-8a4b-4c93-b1de-9f2f6f0c2a11"
}
```

<h3 id="update-api-key-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|API key updated successfully|[UpdateAPIKeyResponse](schemas.md#schemaupdateapikeyresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad Request. Invalid request or validation error.|[Error](schemas.md#schemaerror)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Forbidden. The authenticated user does not have permission to access this resource.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|
|503|[Service Unavailable](https://tools.ietf.org/html/rfc7231#section-6.6.4)|Service Unavailable. No gateway connections are currently available to service this request.|[Error](schemas.md#schemaerror)|

## Revoke API key

<a id="opIdRevokeAPIKey"></a>

`DELETE /rest-apis/{restApiId}/api-keys/{apiKeyId}`

> Code samples

```shell

curl -X DELETE https://localhost:9243/api/v0.9/rest-apis/{restApiId}/api-keys/{apiKeyId} \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Accept: application/json'

```

Revokes an API key for the specified API. The revocation will be broadcasted
to all gateways where the API is deployed. This endpoint allows external platforms
to revoke API keys on hybrid gateways.

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:rest_api:api_key:delete`, `ap:rest_api:api_key:manage`, `ap:rest_api:manage`

</aside>

<h3 id="revoke-api-key-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|restApiId|path|string|true|**API ID** consisting of the **handle** (unique identifier) of the API.|
|apiKeyId|path|string|true|The unique name/identifier of the API key to revoke|

#### Detailed descriptions

**restApiId**: **API ID** consisting of the **handle** (unique identifier) of the API.

> Example responses

> 400 Response

```json
{
  "status": "error",
  "code": "VALIDATION_FAILED",
  "message": "The request failed validation.",
  "errors": [
    {
      "field": "spec.context",
      "message": "must start with /"
    }
  ]
}
```

> 401 Response

```json
{
  "status": "error",
  "code": "UNAUTHORIZED",
  "message": "Authorization header is required, or the token is invalid or expired."
}
```

> 403 Response

```json
{
  "status": "error",
  "code": "FORBIDDEN",
  "message": "You do not have permission to perform this action."
}
```

> 404 Response

```json
{
  "status": "error",
  "code": "NOT_FOUND",
  "message": "The specified resource does not exist."
}
```

> 500 Response

```json
{
  "status": "error",
  "code": "INTERNAL_ERROR",
  "message": "An unexpected error occurred.",
  "trackingId": "4f1c6f2e-8a4b-4c93-b1de-9f2f6f0c2a11"
}
```

> 503 Response

```json
{
  "status": "error",
  "code": "GATEWAY_CONNECTION_UNAVAILABLE",
  "message": "No gateway connections are currently available.",
  "trackingId": "4f1c6f2e-8a4b-4c93-b1de-9f2f6f0c2a11"
}
```

<h3 id="revoke-api-key-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|204|[No Content](https://tools.ietf.org/html/rfc7231#section-6.3.5)|API key revoked successfully (no content)|None|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad Request. Invalid request or validation error.|[Error](schemas.md#schemaerror)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Forbidden. The authenticated user does not have permission to access this resource.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|
|503|[Service Unavailable](https://tools.ietf.org/html/rfc7231#section-6.6.4)|Service Unavailable. No gateway connections are currently available to service this request.|[Error](schemas.md#schemaerror)|
