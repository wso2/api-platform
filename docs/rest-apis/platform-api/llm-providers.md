<h1 id="wso2-api-platform-platform-api-llm-providers">LLM Providers</h1>

LLM provider management operations

## Create a new LLM provider

<a id="opIdcreateLLMProvider"></a>

`POST /llm-providers`

> Code samples

```shell

curl -X POST https://localhost:9243/api/v0.9/llm-providers \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -d @payload.json

```

Deploy a new LLM provider configuration.

> Payload

```json
{
  "id": "wso2-openai-provider",
  "displayName": "WSO2 OpenAI Provider",
  "description": "Primary OpenAI provider",
  "version": "v1.0",
  "context": "/openai",
  "vhost": "api.openai.com",
  "template": "openai",
  "openapi": "openapi: 3.0.3\ninfo:\n  title: Provider API\n  version: v1.0\npaths: {}\n",
  "modelProviders": [
    {
      "id": "claude",
      "displayName": "Claude",
      "models": [
        {
          "id": "claude-3-5-sonnet",
          "displayName": "Claude 3.5 Sonnet",
          "description": "High-quality reasoning model"
        }
      ]
    }
  ],
  "upstream": {
    "main": {
      "url": "http://prod-backend:5000/api/v2",
      "auth": {
        "type": "api-key",
        "header": "X-API-Key",
        "value": "my-api-key-value"
      }
    },
    "sandbox": {
      "url": "http://prod-backend:5000/api/v2",
      "auth": {
        "type": "api-key",
        "header": "X-API-Key",
        "value": "my-api-key-value"
      }
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
  "rateLimiting": {
    "providerLevel": {
      "global": {
        "request": {
          "enabled": true,
          "count": 1500,
          "reset": {
            "duration": 2,
            "unit": "week"
          }
        },
        "token": {
          "enabled": true,
          "count": 1000000,
          "reset": {
            "duration": 1,
            "unit": "month"
          }
        }
      }
    },
    "consumerLevel": {
      "resourceWise": {
        "default": {
          "request": {
            "enabled": true,
            "count": 50,
            "reset": {
              "duration": 2,
              "unit": "week"
            }
          },
          "cost": {
            "enabled": true,
            "amount": 100,
            "reset": {
              "duration": 1,
              "unit": "month"
            }
          }
        },
        "resources": [
          {
            "resource": "/models",
            "limit": {
              "request": {
                "enabled": true,
                "count": 200,
                "reset": {
                  "duration": 1,
                  "unit": "week"
                }
              },
              "token": {
                "enabled": true,
                "count": 100000,
                "reset": {
                  "duration": 1,
                  "unit": "month"
                }
              }
            }
          },
          {
            "resource": "/chat/completions",
            "limit": {
              "request": {
                "enabled": true,
                "count": 25,
                "reset": {
                  "duration": 1,
                  "unit": "week"
                }
              },
              "cost": {
                "enabled": true,
                "amount": 10,
                "reset": {
                  "duration": 1,
                  "unit": "month"
                }
              }
            }
          }
        ]
      }
    }
  },
  "globalPolicies": [
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
      "name": "budgetControl",
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
  "security": {
    "enabled": true,
    "apiKey": {
      "enabled": true,
      "key": "X-API-Key",
      "valuePrefix": "Bearer",
      "in": "header"
    }
  },
  "associatedGateways": [
    {
      "id": "prod-eu",
      "configurations": {
        "host": "prod-eu.platform-gw.local"
      }
    },
    {
      "id": "prod-us",
      "configurations": {
        "host": "prod-us.platform-gw.local"
      }
    }
  ]
}
```

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:llm_provider:create`, `ap:llm_provider:manage`

</aside>

<h3 id="create-a-new-llm-provider-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|[LLMProvider](schemas.md#schemallmprovider)|true|none|

> Example responses
>
> 201 Response

```json
{
  "id": "wso2-openai-provider",
  "displayName": "WSO2 OpenAI Provider",
  "description": "Primary OpenAI provider",
  "createdBy": "john.doe",
  "readOnly": false,
  "updatedBy": "john.doe",
  "version": "v1.0",
  "context": "/openai",
  "vhost": "api.openai.com",
  "template": "openai",
  "openapi": "openapi: 3.0.3\ninfo:\n  title: Provider API\n  version: v1.0\npaths: {}\n",
  "modelProviders": [
    {
      "id": "claude",
      "displayName": "Claude",
      "models": [
        {
          "id": "claude-3-5-sonnet",
          "displayName": "Claude 3.5 Sonnet",
          "description": "High-quality reasoning model"
        }
      ]
    }
  ],
  "upstream": {
    "main": {
      "url": "http://prod-backend:5000/api/v2",
      "auth": {
        "type": "api-key",
        "header": "X-API-Key",
        "value": "my-api-key-value"
      }
    },
    "sandbox": {
      "url": "http://prod-backend:5000/api/v2",
      "auth": {
        "type": "api-key",
        "header": "X-API-Key",
        "value": "my-api-key-value"
      }
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
  "rateLimiting": {
    "providerLevel": {
      "global": {
        "request": {
          "enabled": true,
          "count": 1500,
          "reset": {
            "duration": 2,
            "unit": "week"
          }
        },
        "token": {
          "enabled": true,
          "count": 1000000,
          "reset": {
            "duration": 1,
            "unit": "month"
          }
        }
      }
    },
    "consumerLevel": {
      "resourceWise": {
        "default": {
          "request": {
            "enabled": true,
            "count": 50,
            "reset": {
              "duration": 2,
              "unit": "week"
            }
          },
          "cost": {
            "enabled": true,
            "amount": 100,
            "reset": {
              "duration": 1,
              "unit": "month"
            }
          }
        },
        "resources": [
          {
            "resource": "/models",
            "limit": {
              "request": {
                "enabled": true,
                "count": 200,
                "reset": {
                  "duration": 1,
                  "unit": "week"
                }
              },
              "token": {
                "enabled": true,
                "count": 100000,
                "reset": {
                  "duration": 1,
                  "unit": "month"
                }
              }
            }
          },
          {
            "resource": "/chat/completions",
            "limit": {
              "request": {
                "enabled": true,
                "count": 25,
                "reset": {
                  "duration": 1,
                  "unit": "week"
                }
              },
              "cost": {
                "enabled": true,
                "amount": 10,
                "reset": {
                  "duration": 1,
                  "unit": "month"
                }
              }
            }
          }
        ]
      }
    }
  },
  "globalPolicies": [
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
      "name": "budgetControl",
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
  "security": {
    "enabled": true,
    "apiKey": {
      "enabled": true,
      "key": "X-API-Key",
      "valuePrefix": "Bearer",
      "in": "header"
    }
  },
  "associatedGateways": [
    {
      "id": "prod-eu",
      "configurations": {
        "host": "prod-eu.platform-gw.local"
      }
    },
    {
      "id": "prod-us",
      "configurations": {
        "host": "prod-us.platform-gw.local"
      }
    }
  ],
  "createdAt": "2023-10-12T10:30:00Z",
  "updatedAt": "2023-10-12T10:30:00Z"
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
      "field": "<name of the offending field>",
      "message": "<reason this field failed validation>"
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

> 409 Response

```json
{
  "status": "error",
  "code": "CONFLICT",
  "message": "The request conflicts with the current state of the resource."
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

<h3 id="create-a-new-llm-provider-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|201|[Created](https://tools.ietf.org/html/rfc7231#section-6.3.2)|LLM provider created successfully|[LLMProvider](schemas.md#schemallmprovider)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad Request. Invalid request or validation error.|[Error](schemas.md#schemaerror)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Forbidden. The authenticated user does not have permission to access this resource.|[Error](schemas.md#schemaerror)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Conflict. The request conflicts with the current state of the resource.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|

### Response Headers

|Status|Header|Type|Format|Description|
|---|---|---|---|---|
|201|Location|string|uri|URL of the newly created resource.|

## List all LLM providers

<a id="opIdlistLLMProviders"></a>

`GET /llm-providers`

> Code samples

```shell

curl -X GET https://localhost:9243/api/v0.9/llm-providers \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Accept: application/json'

```

Retrieve a list of all LLM providers.

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:llm_provider:read`, `ap:llm_provider:manage`

</aside>

<h3 id="list-all-llm-providers-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|limit|query|integer|false|Maximum number of items to return per page.|
|offset|query|integer|false|Zero-based index of the first item to return.|

> Example responses
>
> 200 Response

```json
{
  "count": 2,
  "list": [
    {
      "id": "wso2-openai-provider",
      "displayName": "WSO2 OpenAI Provider",
      "description": "Primary OpenAI provider",
      "createdBy": "john.doe",
      "readOnly": false,
      "version": "v1.0",
      "template": "openai",
      "status": "deployed",
      "createdAt": "2025-11-25T10:30:00Z",
      "updatedAt": "2025-11-25T10:30:00Z"
    }
  ],
  "pagination": {
    "total": 10,
    "offset": 0,
    "limit": 10
  }
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

> 500 Response

```json
{
  "status": "error",
  "code": "INTERNAL_ERROR",
  "message": "An unexpected error occurred.",
  "trackingId": "4f1c6f2e-8a4b-4c93-b1de-9f2f6f0c2a11"
}
```

<h3 id="list-all-llm-providers-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|List of LLM providers|[LLMProviderListResponse](schemas.md#schemallmproviderlistresponse)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|

## Get LLM provider by identifier

<a id="opIdgetLLMProvider"></a>

`GET /llm-providers/{llmProviderId}`

> Code samples

```shell

curl -X GET https://localhost:9243/api/v0.9/llm-providers/{llmProviderId} \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Accept: application/json'

```

Retrieve the complete configuration for a specific LLM provider.

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:llm_provider:read`, `ap:llm_provider:manage`

</aside>

<h3 id="get-llm-provider-by-identifier-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|llmProviderId|path|string|true|Unique identifier of the LLM provider|

> Example responses
>
> 200 Response

```json
{
  "id": "wso2-openai-provider",
  "displayName": "WSO2 OpenAI Provider",
  "description": "Primary OpenAI provider",
  "createdBy": "john.doe",
  "readOnly": false,
  "updatedBy": "john.doe",
  "version": "v1.0",
  "context": "/openai",
  "vhost": "api.openai.com",
  "template": "openai",
  "openapi": "openapi: 3.0.3\ninfo:\n  title: Provider API\n  version: v1.0\npaths: {}\n",
  "modelProviders": [
    {
      "id": "claude",
      "displayName": "Claude",
      "models": [
        {
          "id": "claude-3-5-sonnet",
          "displayName": "Claude 3.5 Sonnet",
          "description": "High-quality reasoning model"
        }
      ]
    }
  ],
  "upstream": {
    "main": {
      "url": "http://prod-backend:5000/api/v2",
      "auth": {
        "type": "api-key",
        "header": "X-API-Key",
        "value": "my-api-key-value"
      }
    },
    "sandbox": {
      "url": "http://prod-backend:5000/api/v2",
      "auth": {
        "type": "api-key",
        "header": "X-API-Key",
        "value": "my-api-key-value"
      }
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
  "rateLimiting": {
    "providerLevel": {
      "global": {
        "request": {
          "enabled": true,
          "count": 1500,
          "reset": {
            "duration": 2,
            "unit": "week"
          }
        },
        "token": {
          "enabled": true,
          "count": 1000000,
          "reset": {
            "duration": 1,
            "unit": "month"
          }
        }
      }
    },
    "consumerLevel": {
      "resourceWise": {
        "default": {
          "request": {
            "enabled": true,
            "count": 50,
            "reset": {
              "duration": 2,
              "unit": "week"
            }
          },
          "cost": {
            "enabled": true,
            "amount": 100,
            "reset": {
              "duration": 1,
              "unit": "month"
            }
          }
        },
        "resources": [
          {
            "resource": "/models",
            "limit": {
              "request": {
                "enabled": true,
                "count": 200,
                "reset": {
                  "duration": 1,
                  "unit": "week"
                }
              },
              "token": {
                "enabled": true,
                "count": 100000,
                "reset": {
                  "duration": 1,
                  "unit": "month"
                }
              }
            }
          },
          {
            "resource": "/chat/completions",
            "limit": {
              "request": {
                "enabled": true,
                "count": 25,
                "reset": {
                  "duration": 1,
                  "unit": "week"
                }
              },
              "cost": {
                "enabled": true,
                "amount": 10,
                "reset": {
                  "duration": 1,
                  "unit": "month"
                }
              }
            }
          }
        ]
      }
    }
  },
  "globalPolicies": [
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
      "name": "budgetControl",
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
  "security": {
    "enabled": true,
    "apiKey": {
      "enabled": true,
      "key": "X-API-Key",
      "valuePrefix": "Bearer",
      "in": "header"
    }
  },
  "associatedGateways": [
    {
      "id": "prod-eu",
      "configurations": {
        "host": "prod-eu.platform-gw.local"
      }
    },
    {
      "id": "prod-us",
      "configurations": {
        "host": "prod-us.platform-gw.local"
      }
    }
  ],
  "createdAt": "2023-10-12T10:30:00Z",
  "updatedAt": "2023-10-12T10:30:00Z"
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

<h3 id="get-llm-provider-by-identifier-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|LLM provider details|[LLMProvider](schemas.md#schemallmprovider)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|

## Update an existing LLM provider

<a id="opIdupdateLLMProvider"></a>

`PUT /llm-providers/{llmProviderId}`

> Code samples

```shell

curl -X PUT https://localhost:9243/api/v0.9/llm-providers/{llmProviderId} \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -d @payload.json

```

Update the configuration of an existing LLM provider.

> Payload

```json
{
  "id": "wso2-openai-provider",
  "displayName": "WSO2 OpenAI Provider",
  "description": "Primary OpenAI provider",
  "version": "v1.0",
  "context": "/openai",
  "vhost": "api.openai.com",
  "template": "openai",
  "openapi": "openapi: 3.0.3\ninfo:\n  title: Provider API\n  version: v1.0\npaths: {}\n",
  "modelProviders": [
    {
      "id": "claude",
      "displayName": "Claude",
      "models": [
        {
          "id": "claude-3-5-sonnet",
          "displayName": "Claude 3.5 Sonnet",
          "description": "High-quality reasoning model"
        }
      ]
    }
  ],
  "upstream": {
    "main": {
      "url": "http://prod-backend:5000/api/v2",
      "auth": {
        "type": "api-key",
        "header": "X-API-Key",
        "value": "my-api-key-value"
      }
    },
    "sandbox": {
      "url": "http://prod-backend:5000/api/v2",
      "auth": {
        "type": "api-key",
        "header": "X-API-Key",
        "value": "my-api-key-value"
      }
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
  "rateLimiting": {
    "providerLevel": {
      "global": {
        "request": {
          "enabled": true,
          "count": 1500,
          "reset": {
            "duration": 2,
            "unit": "week"
          }
        },
        "token": {
          "enabled": true,
          "count": 1000000,
          "reset": {
            "duration": 1,
            "unit": "month"
          }
        }
      }
    },
    "consumerLevel": {
      "resourceWise": {
        "default": {
          "request": {
            "enabled": true,
            "count": 50,
            "reset": {
              "duration": 2,
              "unit": "week"
            }
          },
          "cost": {
            "enabled": true,
            "amount": 100,
            "reset": {
              "duration": 1,
              "unit": "month"
            }
          }
        },
        "resources": [
          {
            "resource": "/models",
            "limit": {
              "request": {
                "enabled": true,
                "count": 200,
                "reset": {
                  "duration": 1,
                  "unit": "week"
                }
              },
              "token": {
                "enabled": true,
                "count": 100000,
                "reset": {
                  "duration": 1,
                  "unit": "month"
                }
              }
            }
          },
          {
            "resource": "/chat/completions",
            "limit": {
              "request": {
                "enabled": true,
                "count": 25,
                "reset": {
                  "duration": 1,
                  "unit": "week"
                }
              },
              "cost": {
                "enabled": true,
                "amount": 10,
                "reset": {
                  "duration": 1,
                  "unit": "month"
                }
              }
            }
          }
        ]
      }
    }
  },
  "globalPolicies": [
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
      "name": "budgetControl",
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
  "security": {
    "enabled": true,
    "apiKey": {
      "enabled": true,
      "key": "X-API-Key",
      "valuePrefix": "Bearer",
      "in": "header"
    }
  },
  "associatedGateways": [
    {
      "id": "prod-eu",
      "configurations": {
        "host": "prod-eu.platform-gw.local"
      }
    },
    {
      "id": "prod-us",
      "configurations": {
        "host": "prod-us.platform-gw.local"
      }
    }
  ]
}
```

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:llm_provider:update`, `ap:llm_provider:manage`

</aside>

<h3 id="update-an-existing-llm-provider-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|llmProviderId|path|string|true|Unique identifier of the LLM provider|
|body|body|[LLMProvider](schemas.md#schemallmprovider)|true|none|

> Example responses
>
> 200 Response

```json
{
  "id": "wso2-openai-provider",
  "displayName": "WSO2 OpenAI Provider",
  "description": "Primary OpenAI provider",
  "createdBy": "john.doe",
  "readOnly": false,
  "updatedBy": "john.doe",
  "version": "v1.0",
  "context": "/openai",
  "vhost": "api.openai.com",
  "template": "openai",
  "openapi": "openapi: 3.0.3\ninfo:\n  title: Provider API\n  version: v1.0\npaths: {}\n",
  "modelProviders": [
    {
      "id": "claude",
      "displayName": "Claude",
      "models": [
        {
          "id": "claude-3-5-sonnet",
          "displayName": "Claude 3.5 Sonnet",
          "description": "High-quality reasoning model"
        }
      ]
    }
  ],
  "upstream": {
    "main": {
      "url": "http://prod-backend:5000/api/v2",
      "auth": {
        "type": "api-key",
        "header": "X-API-Key",
        "value": "my-api-key-value"
      }
    },
    "sandbox": {
      "url": "http://prod-backend:5000/api/v2",
      "auth": {
        "type": "api-key",
        "header": "X-API-Key",
        "value": "my-api-key-value"
      }
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
  "rateLimiting": {
    "providerLevel": {
      "global": {
        "request": {
          "enabled": true,
          "count": 1500,
          "reset": {
            "duration": 2,
            "unit": "week"
          }
        },
        "token": {
          "enabled": true,
          "count": 1000000,
          "reset": {
            "duration": 1,
            "unit": "month"
          }
        }
      }
    },
    "consumerLevel": {
      "resourceWise": {
        "default": {
          "request": {
            "enabled": true,
            "count": 50,
            "reset": {
              "duration": 2,
              "unit": "week"
            }
          },
          "cost": {
            "enabled": true,
            "amount": 100,
            "reset": {
              "duration": 1,
              "unit": "month"
            }
          }
        },
        "resources": [
          {
            "resource": "/models",
            "limit": {
              "request": {
                "enabled": true,
                "count": 200,
                "reset": {
                  "duration": 1,
                  "unit": "week"
                }
              },
              "token": {
                "enabled": true,
                "count": 100000,
                "reset": {
                  "duration": 1,
                  "unit": "month"
                }
              }
            }
          },
          {
            "resource": "/chat/completions",
            "limit": {
              "request": {
                "enabled": true,
                "count": 25,
                "reset": {
                  "duration": 1,
                  "unit": "week"
                }
              },
              "cost": {
                "enabled": true,
                "amount": 10,
                "reset": {
                  "duration": 1,
                  "unit": "month"
                }
              }
            }
          }
        ]
      }
    }
  },
  "globalPolicies": [
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
      "name": "budgetControl",
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
  "security": {
    "enabled": true,
    "apiKey": {
      "enabled": true,
      "key": "X-API-Key",
      "valuePrefix": "Bearer",
      "in": "header"
    }
  },
  "associatedGateways": [
    {
      "id": "prod-eu",
      "configurations": {
        "host": "prod-eu.platform-gw.local"
      }
    },
    {
      "id": "prod-us",
      "configurations": {
        "host": "prod-us.platform-gw.local"
      }
    }
  ],
  "createdAt": "2023-10-12T10:30:00Z",
  "updatedAt": "2023-10-12T10:30:00Z"
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
      "field": "<name of the offending field>",
      "message": "<reason this field failed validation>"
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

<h3 id="update-an-existing-llm-provider-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|LLM provider updated successfully|[LLMProvider](schemas.md#schemallmprovider)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad Request. Invalid request or validation error.|[Error](schemas.md#schemaerror)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Forbidden. The authenticated user does not have permission to access this resource.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|

## Delete an LLM provider

<a id="opIddeleteLLMProvider"></a>

`DELETE /llm-providers/{llmProviderId}`

> Code samples

```shell

curl -X DELETE https://localhost:9243/api/v0.9/llm-providers/{llmProviderId} \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Accept: application/json'

```

Remove an LLM provider.

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:llm_provider:delete`, `ap:llm_provider:manage`

</aside>

<h3 id="delete-an-llm-provider-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|llmProviderId|path|string|true|Unique identifier of the LLM provider|

> Example responses
>
> 400 Response

```json
{
  "status": "error",
  "code": "VALIDATION_FAILED",
  "message": "The request failed validation.",
  "errors": [
    {
      "field": "<name of the offending field>",
      "message": "<reason this field failed validation>"
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

<h3 id="delete-an-llm-provider-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|204|[No Content](https://tools.ietf.org/html/rfc7231#section-6.3.5)|LLM provider deleted successfully|None|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad Request. Invalid request or validation error.|[Error](schemas.md#schemaerror)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Forbidden. The authenticated user does not have permission to access this resource.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|

## List LLM proxies by provider

<a id="opIdlistLLMProxiesByProvider"></a>

`GET /llm-providers/{llmProviderId}/llm-proxies`

> Code samples

```shell

curl -X GET https://localhost:9243/api/v0.9/llm-providers/{llmProviderId}/llm-proxies \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Accept: application/json'

```

Retrieve a list of LLM proxies that use the specified LLM provider.

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:llm_proxy:deployment:read`, `ap:llm_proxy:deployment:manage`, `ap:llm_proxy:manage`

</aside>

<h3 id="list-llm-proxies-by-provider-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|llmProviderId|path|string|true|Unique identifier of the LLM provider|
|limit|query|integer|false|Maximum number of items to return per page.|
|offset|query|integer|false|Zero-based index of the first item to return.|

> Example responses
>
> 200 Response

```json
{
  "count": 2,
  "list": [
    {
      "id": "wso2-con-assistant",
      "displayName": "WSO2 Con Assistant",
      "description": "Customer support assistant",
      "createdBy": "john.doe",
      "context": "/wso2-con-assistant",
      "version": "v1.0",
      "projectId": "550e8400-e29b-41d4-a716-446655440000",
      "provider": "wso2-openai-provider",
      "status": "deployed",
      "createdAt": "2025-11-25T10:30:00Z",
      "updatedAt": "2025-11-25T10:30:00Z",
      "readOnly": false
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
      "field": "<name of the offending field>",
      "message": "<reason this field failed validation>"
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

<h3 id="list-llm-proxies-by-provider-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|List of LLM proxies|[LLMProxyListResponse](schemas.md#schemallmproxylistresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad Request. Invalid request or validation error.|[Error](schemas.md#schemaerror)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|

## Create a new API key for an LLM provider

<a id="opIdcreateLLMProviderAPIKey"></a>

`POST /llm-providers/{llmProviderId}/api-keys`

> Code samples

```shell

curl -X POST https://localhost:9243/api/v0.9/llm-providers/{llmProviderId}/api-keys \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -d @payload.json

```

Generates a new API key for the specified LLM provider. The generated key
is broadcasted to all gateways in the organization and can be used to
authenticate requests to the LLM provider when API key validation is enabled.

> Payload

```json
{
  "id": "production-key",
  "displayName": "Production Key",
  "expiresAt": "2026-12-31T23:59:59Z",
  "issuer": "api-platform-devportal",
  "allowedTargets": "dev_gateway,test_gateway"
}
```

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:llm_provider:api_key:create`, `ap:llm_provider:api_key:manage`, `ap:llm_provider:manage`

</aside>

<h3 id="create-a-new-api-key-for-an-llm-provider-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|llmProviderId|path|string|true|Unique identifier of the LLM provider|
|body|body|[CreateLLMProviderAPIKeyRequest](schemas.md#schemacreatellmproviderapikeyrequest)|true|API key creation details|

> Example responses
>
> 201 Response

```json
{
  "status": "success",
  "message": "API key created and broadcasted to gateways successfully",
  "id": "production-key",
  "apiKey": "REDACTED_API_KEY"
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
      "field": "<name of the offending field>",
      "message": "<reason this field failed validation>"
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

> 503 Response

```json
{
  "status": "error",
  "code": "GATEWAY_CONNECTION_UNAVAILABLE",
  "message": "No gateway connections are currently available.",
  "trackingId": "4f1c6f2e-8a4b-4c93-b1de-9f2f6f0c2a11"
}
```

<h3 id="create-a-new-api-key-for-an-llm-provider-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|201|[Created](https://tools.ietf.org/html/rfc7231#section-6.3.2)|API key created successfully|[CreateLLMProviderAPIKeyResponse](schemas.md#schemacreatellmproviderapikeyresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad Request. Invalid request or validation error.|[Error](schemas.md#schemaerror)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|
|503|[Service Unavailable](https://tools.ietf.org/html/rfc7231#section-6.6.4)|Service Unavailable. No gateway connections are currently available to service this request.|[Error](schemas.md#schemaerror)|

### Response Headers

|Status|Header|Type|Format|Description|
|---|---|---|---|---|
|201|Location|string|uri|URL of the newly created resource.|

## List API keys for an LLM provider

<a id="opIdlistLLMProviderAPIKeys"></a>

`GET /llm-providers/{llmProviderId}/api-keys`

> Code samples

```shell

curl -X GET https://localhost:9243/api/v0.9/llm-providers/{llmProviderId}/api-keys \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Accept: application/json'

```

Returns all API keys associated with the specified LLM provider. The plain key value is never returned.

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:llm_provider:api_key:read`, `ap:llm_provider:api_key:manage`, `ap:llm_provider:manage`

</aside>

<h3 id="list-api-keys-for-an-llm-provider-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|llmProviderId|path|string|true|Unique identifier of the LLM provider|
|limit|query|integer|false|Maximum number of items to return per page.|
|offset|query|integer|false|Zero-based index of the first item to return.|

> Example responses
>
> 200 Response

```json
{
  "list": [
    {
      "id": "string",
      "displayName": "string",
      "maskedApiKey": "string",
      "status": "active",
      "createdAt": "2019-08-24T14:15:22Z",
      "createdBy": "john.doe",
      "updatedAt": "2019-08-24T14:15:22Z",
      "expiresAt": "2019-08-24T14:15:22Z",
      "issuer": "api-platform-devportal",
      "allowedTargets": "string"
    }
  ],
  "count": 0,
  "pagination": {
    "total": 10,
    "offset": 0,
    "limit": 10
  }
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

<h3 id="list-api-keys-for-an-llm-provider-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|List of API keys retrieved successfully|[LLMProviderAPIKeyListResponse](schemas.md#schemallmproviderapikeylistresponse)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|

## Delete an API key for an LLM provider

<a id="opIddeleteLLMProviderAPIKey"></a>

`DELETE /llm-providers/{llmProviderId}/api-keys/{apiKeyId}`

> Code samples

```shell

curl -X DELETE https://localhost:9243/api/v0.9/llm-providers/{llmProviderId}/api-keys/{apiKeyId} \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Accept: application/json'

```

Deletes the key from the database and broadcasts a revoke event to the allowed gateways.

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:llm_provider:api_key:delete`, `ap:llm_provider:api_key:manage`, `ap:llm_provider:manage`

</aside>

<h3 id="delete-an-api-key-for-an-llm-provider-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|llmProviderId|path|string|true|Unique identifier of the LLM provider|
|apiKeyId|path|string|true|Name of the API key to delete|

> Example responses
>
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

> 503 Response

```json
{
  "status": "error",
  "code": "GATEWAY_CONNECTION_UNAVAILABLE",
  "message": "No gateway connections are currently available.",
  "trackingId": "4f1c6f2e-8a4b-4c93-b1de-9f2f6f0c2a11"
}
```

<h3 id="delete-an-api-key-for-an-llm-provider-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|204|[No Content](https://tools.ietf.org/html/rfc7231#section-6.3.5)|API key deleted successfully|None|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|
|503|[Service Unavailable](https://tools.ietf.org/html/rfc7231#section-6.6.4)|Service Unavailable. No gateway connections are currently available to service this request.|[Error](schemas.md#schemaerror)|
