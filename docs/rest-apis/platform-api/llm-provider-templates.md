<h1 id="wso2-api-platform-platform-api-llm-provider-templates">LLM Provider Templates</h1>

LLM provider template management operations

## Create a new LLM provider template family

<a id="opIdcreateLLMProviderTemplate"></a>

`POST /llm-provider-templates`

> Code samples

```shell

curl -X POST https://localhost:9243/api/v0.9/llm-provider-templates \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -d @payload.json

```

Creates a new template family, starting at v1.0, with an
`LLMProviderTemplate` body. To add a new version to an existing family,
use `POST /llm-provider-templates/copy` instead. Organization is
identified via the JWT token.

> Payload

```json
{
  "id": "openai",
  "displayName": "OpenAI",
  "managedBy": "wso2",
  "description": "Default OpenAI template",
  "version": "v1.0",
  "openapi": "openapi: 3.0.3\ninfo:\n  title: Provider API\n  version: v1.0\npaths: {}\n",
  "metadata": {
    "endpointUrl": "https://api.openai.com",
    "auth": {
      "type": "bearer",
      "header": "Authorization",
      "valuePrefix": "Bearer "
    },
    "logoUrl": "https://cdn.example.com/logos/openai.svg",
    "openapiSpecUrl": "https://api.openai.com/openapi.json"
  },
  "promptTokens": {
    "location": "payload",
    "identifier": "$.usage.inputTokens"
  },
  "completionTokens": {
    "location": "payload",
    "identifier": "$.usage.outputTokens"
  },
  "totalTokens": {
    "location": "payload",
    "identifier": "$.usage.totalTokens"
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
          "identifier": "$.usage.outputTokens"
        },
        "totalTokens": {
          "location": "payload",
          "identifier": "$.usage.totalTokens"
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
    ]
  }
}
```

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:llm_template:create`, `ap:llm_template:manage`

</aside>

<h3 id="create-a-new-llm-provider-template-family-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|[LLMProviderTemplate](schemas.md#schemallmprovidertemplate)|true|The template family to create (starts at v1.0).|

> Example responses
>
> 201 Response

```json
{
  "id": "openai",
  "groupId": "openai",
  "displayName": "OpenAI",
  "managedBy": "wso2",
  "description": "Default OpenAI template",
  "createdBy": "john.doe",
  "updatedBy": "john.doe",
  "readOnly": false,
  "version": "v1.0",
  "isLatest": true,
  "enabled": true,
  "openapi": "openapi: 3.0.3\ninfo:\n  title: Provider API\n  version: v1.0\npaths: {}\n",
  "metadata": {
    "endpointUrl": "https://api.openai.com",
    "auth": {
      "type": "bearer",
      "header": "Authorization",
      "valuePrefix": "Bearer "
    },
    "logoUrl": "https://cdn.example.com/logos/openai.svg",
    "openapiSpecUrl": "https://api.openai.com/openapi.json"
  },
  "promptTokens": {
    "location": "payload",
    "identifier": "$.usage.inputTokens"
  },
  "completionTokens": {
    "location": "payload",
    "identifier": "$.usage.outputTokens"
  },
  "totalTokens": {
    "location": "payload",
    "identifier": "$.usage.totalTokens"
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
          "identifier": "$.usage.outputTokens"
        },
        "totalTokens": {
          "location": "payload",
          "identifier": "$.usage.totalTokens"
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
    ]
  },
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

<h3 id="create-a-new-llm-provider-template-family-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|201|[Created](https://tools.ietf.org/html/rfc7231#section-6.3.2)|LLM provider template created successfully|[LLMProviderTemplate](schemas.md#schemallmprovidertemplate)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad Request. Invalid request or validation error.|[Error](schemas.md#schemaerror)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Forbidden. The authenticated user does not have permission to access this resource.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Conflict. The request conflicts with the current state of the resource.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|

### Response Headers

|Status|Header|Type|Format|Description|
|---|---|---|---|---|
|201|Location|string|uri|URL of the newly created resource.|

## List templates, list a family's versions, or get a single version

<a id="opIdlistLLMProviderTemplates"></a>

`GET /llm-provider-templates`

> Code samples

```shell

curl -X GET https://localhost:9243/api/v0.9/llm-provider-templates \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Accept: application/json'

```

Retrieves LLM provider templates based on the `query` parameter:
  - no `query`                         -> list all template versions
  - `query=latest:true`                -> list the latest version of each template
  - `query=groupId:<id>`               -> list all versions for the template
  - `query=groupId:<id>&version:<ver>` -> retrieve a specific template version

All filters are provided via the URL-encoded `query` parameter (e.g.
`?query=groupId%3Awso2-openai%26version%3Av2.0`).

Returns a single `LLMProviderTemplate` only when both `groupId` and `version` are specified; 
otherwise returns an `LLMProviderTemplateListResponse`.

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:llm_template:read`, `ap:llm_template:manage`

</aside>

<h3 id="list-templates,-list-a-family's-versions,-or-get-a-single-version-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|query|query|string|false|URL-encoded search DSL. `query=latest:true` lists only the latest version of each family; `query=groupId:<id>` lists that family's versions; adding `&version:<ver>` returns the single full template for that version. Terms are `&`-separated `key:value` pairs and the whole value is percent-encoded (e.g. groupId%3Awso2-openai%26version%3Av2.0).|
|limit|query|integer|false|Maximum number of items to return per page.|
|offset|query|integer|false|Zero-based index of the first item to return.|

> Example responses
>
> 200 Response

```json
{
  "count": 5,
  "list": [
    {
      "id": "openai",
      "groupId": "openai",
      "displayName": "OpenAI",
      "managedBy": "wso2",
      "description": "Default OpenAI template",
      "createdBy": "john.doe",
      "version": "v1.0",
      "isLatest": true,
      "enabled": true,
      "logoUrl": "https://cdn.example.com/logos/openai.svg",
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

<h3 id="list-templates,-list-a-family's-versions,-or-get-a-single-version-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Without a `version` term: a list response — either templates (all versions by default, or the latest of each family when `query=latest:true`) or a family's versions. With `query=groupId:<id>&version:<ver>`: the single full template for that version.|Inline|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad Request. Invalid request or validation error.|[Error](schemas.md#schemaerror)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|

<h3 id="list-templates,-list-a-family's-versions,-or-get-a-single-version-responseschema">Response Schema</h3>

#### Enumerated Values

|Property|Value|
|---|---|
|location|payload|
|location|header|
|location|queryParam|
|location|pathParam|
|location|payload|
|location|header|
|location|queryParam|
|location|pathParam|
|location|payload|
|location|header|
|location|queryParam|
|location|pathParam|
|location|payload|
|location|header|
|location|queryParam|
|location|pathParam|
|location|payload|
|location|header|
|location|queryParam|
|location|pathParam|
|location|payload|
|location|header|
|location|queryParam|
|location|pathParam|
|location|payload|
|location|header|
|location|queryParam|
|location|pathParam|
|location|payload|
|location|header|
|location|queryParam|
|location|pathParam|
|location|payload|
|location|header|
|location|queryParam|
|location|pathParam|
|location|payload|
|location|header|
|location|queryParam|
|location|pathParam|
|location|payload|
|location|header|
|location|queryParam|
|location|pathParam|
|location|payload|
|location|header|
|location|queryParam|
|location|pathParam|

## Create a new version by copying an existing one

<a id="opIdcopyLLMProviderTemplateVersion"></a>

`POST /llm-provider-templates/copy`

> Code samples

```shell

curl -X POST https://localhost:9243/api/v0.9/llm-provider-templates/copy?fromTemplateId=openai-v3-0&toVersion=v4.0 \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -d @payload.json

```

Creates a new version within a template family by cloning an existing
version (`fromTemplateId`) and applying optional field overrides from
the body. The new version lands in the same family as the source and
becomes the latest.

Versions start at v1.0 and only go higher — `toVersion` must match the
`v<major>.<minor>` pattern with a major of at least 1 (e.g. v2.0); v0.x
is rejected. `toTemplateId`, when supplied, must equal the handle
derived from the family and `toVersion`. Organization is identified via
the JWT token.

Built-in (WSO2-managed) template families are immutable: adding a version
to one is rejected with `403`. To base a custom template on a built-in,
create a new template (`POST /llm-provider-templates`) instead — it gets
its own `group_id` and starts at v1.0.

> Payload

```json
{
  "displayName": "OpenAI",
  "version": "v2.0",
  "managedBy": "organization",
  "description": "Default OpenAI template",
  "openapi": "string",
  "metadata": {
    "endpointUrl": "https://api.openai.com",
    "auth": {
      "type": "bearer",
      "header": "Authorization",
      "valuePrefix": "Bearer "
    },
    "logoUrl": "https://cdn.example.com/logos/openai.svg",
    "openapiSpecUrl": "https://api.openai.com/openapi.json"
  },
  "promptTokens": {
    "location": "payload",
    "identifier": "$.usage.inputTokens"
  },
  "completionTokens": {
    "location": "payload",
    "identifier": "$.usage.outputTokens"
  },
  "totalTokens": {
    "location": "payload",
    "identifier": "$.usage.totalTokens"
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
          "identifier": "$.usage.outputTokens"
        },
        "totalTokens": {
          "location": "payload",
          "identifier": "$.usage.totalTokens"
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
    ]
  }
}
```

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:llm_template:create`, `ap:llm_template:manage`

</aside>

<h3 id="create-a-new-version-by-copying-an-existing-one-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|fromTemplateId|query|string|true|Handle (id) of the source version to copy from.|
|toTemplateId|query|string|false|Expected handle of the new version. Must equal the handle derived from the source family and toVersion; used to validate the target.|
|toVersion|query|string|true|New version identifier, e.g. v4.0. Must match v<major>.<minor> with a major of at least 1, and be unique within the family.|
|body|body|[CreateLLMProviderTemplateVersionRequest](schemas.md#schemacreatellmprovidertemplateversionrequest)|false|Optional overrides applied on top of the copied config. Any field present replaces the value cloned from the source version.|

> Example responses
>
> New version created successfully

```json
{
  "id": "openai-v4-0",
  "groupId": "openai",
  "displayName": "OpenAI",
  "managedBy": "organization",
  "description": "Default OpenAI template",
  "createdBy": "john.doe",
  "updatedBy": "john.doe",
  "readOnly": false,
  "version": "v4.0",
  "isLatest": true,
  "enabled": true
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

<h3 id="create-a-new-version-by-copying-an-existing-one-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|201|[Created](https://tools.ietf.org/html/rfc7231#section-6.3.2)|New version created successfully|[LLMProviderTemplate](schemas.md#schemallmprovidertemplate)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad Request. Invalid request or validation error.|[Error](schemas.md#schemaerror)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Forbidden. The authenticated user does not have permission to access this resource.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Conflict. The request conflicts with the current state of the resource.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|

### Response Headers

|Status|Header|Type|Format|Description|
|---|---|---|---|---|
|201|Location|string|uri|URL of the newly created resource.|

## Get LLM provider template by id

<a id="opIdgetLLMProviderTemplate"></a>

`GET /llm-provider-templates/{llmProviderTemplateId}`

> Code samples

```shell

curl -X GET https://localhost:9243/api/v0.9/llm-provider-templates/{llmProviderTemplateId} \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Accept: application/json'

```

Retrieve the complete configuration for a specific LLM provider template.

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:llm_template:read`, `ap:llm_template:manage`

</aside>

<h3 id="get-llm-provider-template-by-id-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|llmProviderTemplateId|path|string|true|Unique handle of the template.|

> Example responses
>
> 200 Response

```json
{
  "id": "openai",
  "groupId": "openai",
  "displayName": "OpenAI",
  "managedBy": "wso2",
  "description": "Default OpenAI template",
  "createdBy": "john.doe",
  "updatedBy": "john.doe",
  "readOnly": false,
  "version": "v1.0",
  "isLatest": true,
  "enabled": true,
  "openapi": "openapi: 3.0.3\ninfo:\n  title: Provider API\n  version: v1.0\npaths: {}\n",
  "metadata": {
    "endpointUrl": "https://api.openai.com",
    "auth": {
      "type": "bearer",
      "header": "Authorization",
      "valuePrefix": "Bearer "
    },
    "logoUrl": "https://cdn.example.com/logos/openai.svg",
    "openapiSpecUrl": "https://api.openai.com/openapi.json"
  },
  "promptTokens": {
    "location": "payload",
    "identifier": "$.usage.inputTokens"
  },
  "completionTokens": {
    "location": "payload",
    "identifier": "$.usage.outputTokens"
  },
  "totalTokens": {
    "location": "payload",
    "identifier": "$.usage.totalTokens"
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
          "identifier": "$.usage.outputTokens"
        },
        "totalTokens": {
          "location": "payload",
          "identifier": "$.usage.totalTokens"
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
    ]
  },
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

<h3 id="get-llm-provider-template-by-id-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|LLM provider template details|[LLMProviderTemplate](schemas.md#schemallmprovidertemplate)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|

## Update an existing LLM provider template

<a id="opIdupdateLLMProviderTemplate"></a>

`PUT /llm-provider-templates/{llmProviderTemplateId}`

> Code samples

```shell

curl -X PUT https://localhost:9243/api/v0.9/llm-provider-templates/{llmProviderTemplateId} \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -d @payload.json

```

Update the configuration of an existing LLM provider template. The template
is identified by its unique handle (id). This operation updates the latest
version of the template.

> Payload

```json
{
  "id": "openai",
  "displayName": "OpenAI",
  "managedBy": "wso2",
  "description": "Default OpenAI template",
  "version": "v1.0",
  "openapi": "openapi: 3.0.3\ninfo:\n  title: Provider API\n  version: v1.0\npaths: {}\n",
  "metadata": {
    "endpointUrl": "https://api.openai.com",
    "auth": {
      "type": "bearer",
      "header": "Authorization",
      "valuePrefix": "Bearer "
    },
    "logoUrl": "https://cdn.example.com/logos/openai.svg",
    "openapiSpecUrl": "https://api.openai.com/openapi.json"
  },
  "promptTokens": {
    "location": "payload",
    "identifier": "$.usage.inputTokens"
  },
  "completionTokens": {
    "location": "payload",
    "identifier": "$.usage.outputTokens"
  },
  "totalTokens": {
    "location": "payload",
    "identifier": "$.usage.totalTokens"
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
          "identifier": "$.usage.outputTokens"
        },
        "totalTokens": {
          "location": "payload",
          "identifier": "$.usage.totalTokens"
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
    ]
  }
}
```

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:llm_template:update`, `ap:llm_template:manage`

</aside>

<h3 id="update-an-existing-llm-provider-template-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|llmProviderTemplateId|path|string|true|Unique identifier of the template to update|
|body|body|[LLMProviderTemplate](schemas.md#schemallmprovidertemplate)|true|none|

> Example responses
>
> 200 Response

```json
{
  "id": "openai",
  "groupId": "openai",
  "displayName": "OpenAI",
  "managedBy": "wso2",
  "description": "Default OpenAI template",
  "createdBy": "john.doe",
  "updatedBy": "john.doe",
  "readOnly": false,
  "version": "v1.0",
  "isLatest": true,
  "enabled": true,
  "openapi": "openapi: 3.0.3\ninfo:\n  title: Provider API\n  version: v1.0\npaths: {}\n",
  "metadata": {
    "endpointUrl": "https://api.openai.com",
    "auth": {
      "type": "bearer",
      "header": "Authorization",
      "valuePrefix": "Bearer "
    },
    "logoUrl": "https://cdn.example.com/logos/openai.svg",
    "openapiSpecUrl": "https://api.openai.com/openapi.json"
  },
  "promptTokens": {
    "location": "payload",
    "identifier": "$.usage.inputTokens"
  },
  "completionTokens": {
    "location": "payload",
    "identifier": "$.usage.outputTokens"
  },
  "totalTokens": {
    "location": "payload",
    "identifier": "$.usage.totalTokens"
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
          "identifier": "$.usage.outputTokens"
        },
        "totalTokens": {
          "location": "payload",
          "identifier": "$.usage.totalTokens"
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
    ]
  },
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

<h3 id="update-an-existing-llm-provider-template-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|LLM provider template updated successfully|[LLMProviderTemplate](schemas.md#schemallmprovidertemplate)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad Request. Invalid request or validation error.|[Error](schemas.md#schemaerror)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Forbidden. The authenticated user does not have permission to access this resource.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|

## Enable or disable a template version

<a id="opIdsetLLMProviderTemplateVersionEnabled"></a>

`PATCH /llm-provider-templates/{llmProviderTemplateId}`

> Code samples

```shell

curl -X PATCH https://localhost:9243/api/v0.9/llm-provider-templates/{llmProviderTemplateId} \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -d @payload.json

```

Enable or disable the single template version identified by its handle
(id). Only built-in templates can be toggled; disabling a version in use
by a provider is rejected (409).

> Payload

```json
{
  "enabled": true
}
```

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:llm_template:update`, `ap:llm_template:manage`

</aside>

<h3 id="enable-or-disable-a-template-version-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|llmProviderTemplateId|path|string|true|Unique handle of the template version.|
|body|body|object|true|none|
|» enabled|body|boolean|true|Set to true to enable this template version, false to disable it.|

> Example responses
>
> 200 Response

```json
{
  "id": "openai",
  "groupId": "openai",
  "displayName": "OpenAI",
  "managedBy": "wso2",
  "description": "Default OpenAI template",
  "createdBy": "john.doe",
  "updatedBy": "john.doe",
  "readOnly": false,
  "version": "v1.0",
  "isLatest": true,
  "enabled": true,
  "openapi": "openapi: 3.0.3\ninfo:\n  title: Provider API\n  version: v1.0\npaths: {}\n",
  "metadata": {
    "endpointUrl": "https://api.openai.com",
    "auth": {
      "type": "bearer",
      "header": "Authorization",
      "valuePrefix": "Bearer "
    },
    "logoUrl": "https://cdn.example.com/logos/openai.svg",
    "openapiSpecUrl": "https://api.openai.com/openapi.json"
  },
  "promptTokens": {
    "location": "payload",
    "identifier": "$.usage.inputTokens"
  },
  "completionTokens": {
    "location": "payload",
    "identifier": "$.usage.outputTokens"
  },
  "totalTokens": {
    "location": "payload",
    "identifier": "$.usage.totalTokens"
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
          "identifier": "$.usage.outputTokens"
        },
        "totalTokens": {
          "location": "payload",
          "identifier": "$.usage.totalTokens"
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
    ]
  },
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

<h3 id="enable-or-disable-a-template-version-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Updated template version|[LLMProviderTemplate](schemas.md#schemallmprovidertemplate)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad Request. Invalid request or validation error.|[Error](schemas.md#schemaerror)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Forbidden. The authenticated user does not have permission to access this resource.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Conflict. The request conflicts with the current state of the resource.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|

## Delete a template version

<a id="opIddeleteLLMProviderTemplateVersion"></a>

`DELETE /llm-provider-templates/{llmProviderTemplateId}`

> Code samples

```shell

curl -X DELETE https://localhost:9243/api/v0.9/llm-provider-templates/{llmProviderTemplateId} \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Accept: application/json'

```

Delete the single template version identified by its handle (id).
Built-in versions are read-only (403); a version still in use by a
provider is blocked (409).

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:llm_template:delete`, `ap:llm_template:manage`

</aside>

<h3 id="delete-a-template-version-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|llmProviderTemplateId|path|string|true|Unique handle of the template version.|

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

<h3 id="delete-a-template-version-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|204|[No Content](https://tools.ietf.org/html/rfc7231#section-6.3.5)|Version deleted successfully|None|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad Request. Invalid request or validation error.|[Error](schemas.md#schemaerror)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Forbidden. The authenticated user does not have permission to access this resource.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Conflict. The request conflicts with the current state of the resource.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|
