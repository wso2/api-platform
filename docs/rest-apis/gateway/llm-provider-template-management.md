<h1 id="gateway-controller-management-api-llm-provider-template-management">LLM Provider Template Management</h1>

CRUD operations for LLM Provider Template configurations

## Create a new LLM provider template

<a id="opIdcreateLLMProviderTemplate"></a>

`POST /llm-provider-templates`

> Code samples

```shell

curl -X POST http://localhost:9090/api/management/v0.9/llm-provider-templates \
  -u {username}:{password} \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -d @payload.json

```

Add a new LLM provider template to the Gateway. A template defines token tracking and model extraction metadata for an LLM provider.

> Payload

```json
{
  "apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
  "kind": "LlmProviderTemplate",
  "metadata": {
    "name": "openai-template"
  },
  "spec": {
    "displayName": "OpenAI",
    "promptTokens": {
      "location": "payload",
      "identifier": "$.usage.prompt_tokens"
    },
    "completionTokens": {
      "location": "payload",
      "identifier": "$.usage.completion_tokens"
    },
    "totalTokens": {
      "location": "payload",
      "identifier": "$.usage.total_tokens"
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
}
```

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

Required roles: `admin`

</aside>

<h3 id="create-a-new-llm-provider-template-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|[LLMProviderTemplateRequest](schemas.md#schemallmprovidertemplaterequest)|true|none|

> Example responses

> 201 Response

```json
{
  "apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
  "kind": "LlmProviderTemplate",
  "metadata": {
    "name": "openai-template"
  },
  "spec": {
    "displayName": "OpenAI",
    "promptTokens": {
      "location": "payload",
      "identifier": "$.usage.prompt_tokens"
    },
    "completionTokens": {
      "location": "payload",
      "identifier": "$.usage.completion_tokens"
    },
    "totalTokens": {
      "location": "payload",
      "identifier": "$.usage.total_tokens"
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
  },
  "status": {
    "id": "openai-template",
    "createdAt": "2026-04-24T07:21:13Z",
    "updatedAt": "2026-04-24T07:21:13Z"
  }
}
```

<h3 id="create-a-new-llm-provider-template-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|201|[Created](https://tools.ietf.org/html/rfc7231#section-6.3.2)|LLM provider template created successfully|[LLMProviderTemplate](schemas.md#schemallmprovidertemplate)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Invalid configuration (validation failed)|[ErrorResponse](schemas.md#schemaerrorresponse)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Conflict - Template with same name already exists|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

## List all LLM provider templates

<a id="opIdlistLLMProviderTemplates"></a>

`GET /llm-provider-templates`

> Code samples

```shell

curl -X GET http://localhost:9090/api/management/v0.9/llm-provider-templates \
  -u {username}:{password} \
  -H 'Accept: application/json'

```

List LLM provider templates registered in the Gateway, optionally filtered by name.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

Required roles: `admin`

</aside>

<h3 id="list-all-llm-provider-templates-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|displayName|query|string|false|Filter by template display name|

> Example responses

> 200 Response

```json
{
  "status": "success",
  "count": 3,
  "templates": [
    {
      "apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
      "kind": "LlmProviderTemplate",
      "metadata": {
        "name": "openai-template"
      },
      "spec": {
        "displayName": "OpenAI",
        "promptTokens": {
          "location": "payload",
          "identifier": "$.usage.prompt_tokens"
        },
        "completionTokens": {
          "location": "payload",
          "identifier": "$.usage.completion_tokens"
        },
        "totalTokens": {
          "location": "payload",
          "identifier": "$.usage.total_tokens"
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
      },
      "status": {
        "id": "openai-template",
        "createdAt": "2026-04-24T07:21:13Z",
        "updatedAt": "2026-04-24T07:21:13Z"
      }
    }
  ]
}
```

<h3 id="list-all-llm-provider-templates-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|List of LLM provider templates|Inline|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="list-all-llm-provider-templates-responseschema">Response Schema</h3>

Status Code **200**

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|» status|string|false|none|none|
|» count|integer|false|none|none|
|» templates|[allOf]|false|none|none|

*allOf*

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|»» *anonymous*|[LLMProviderTemplateRequest](schemas.md#schemallmprovidertemplaterequest)|false|none|none|
|»»» apiVersion|string|true|none|Template specification version|
|»»» kind|string|true|none|Template kind|
|»»» metadata|[Metadata](schemas.md#schemametadata)|true|none|none|
|»»»» name|string|true|none|Unique handle for the resource|
|»»»» labels|object|false|none|Labels are key-value pairs for organizing and selecting APIs. Keys must not contain spaces.|
|»»»»» **additionalProperties**|string|false|none|none|
|»»»» annotations|object|false|none|Annotations are arbitrary non-identifying metadata. Use domain-prefixed keys.|
|»»»»» **additionalProperties**|string|false|none|none|
|»»» spec|[LLMProviderTemplateData](schemas.md#schemallmprovidertemplatedata)|true|none|none|
|»»»» displayName|string|true|none|Human-readable LLM Template name|
|»»»» promptTokens|[ExtractionIdentifier](schemas.md#schemaextractionidentifier)|false|none|none|
|»»»»» location|string|true|none|Where to find the token information|
|»»»»» identifier|string|true|none|JSONPath expression or header name to identify the token value|
|»»»» completionTokens|[ExtractionIdentifier](schemas.md#schemaextractionidentifier)|false|none|none|
|»»»» totalTokens|[ExtractionIdentifier](schemas.md#schemaextractionidentifier)|false|none|none|
|»»»» remainingTokens|[ExtractionIdentifier](schemas.md#schemaextractionidentifier)|false|none|none|
|»»»» requestModel|[ExtractionIdentifier](schemas.md#schemaextractionidentifier)|false|none|none|
|»»»» responseModel|[ExtractionIdentifier](schemas.md#schemaextractionidentifier)|false|none|none|
|»»»» resourceMappings|[LLMProviderTemplateResourceMappings](schemas.md#schemallmprovidertemplateresourcemappings)|false|none|none|
|»»»»» resources|[[LLMProviderTemplateResourceMapping](schemas.md#schemallmprovidertemplateresourcemapping)]|false|none|none|
|»»»»»» resource|string|true|none|Resource path pattern for this mapping|
|»»»»»» promptTokens|[ExtractionIdentifier](schemas.md#schemaextractionidentifier)|false|none|none|
|»»»»»» completionTokens|[ExtractionIdentifier](schemas.md#schemaextractionidentifier)|false|none|none|
|»»»»»» totalTokens|[ExtractionIdentifier](schemas.md#schemaextractionidentifier)|false|none|none|
|»»»»»» remainingTokens|[ExtractionIdentifier](schemas.md#schemaextractionidentifier)|false|none|none|
|»»»»»» requestModel|[ExtractionIdentifier](schemas.md#schemaextractionidentifier)|false|none|none|
|»»»»»» responseModel|[ExtractionIdentifier](schemas.md#schemaextractionidentifier)|false|none|none|

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

#### Enumerated Values

|Property|Value|
|---|---|
|apiVersion|gateway.api-platform.wso2.com/v1alpha1|
|kind|LlmProviderTemplate|
|location|payload|
|location|header|
|location|queryParam|
|location|pathParam|
|state|deployed|
|state|undeployed|

## Get LLM provider template by id

<a id="opIdgetLLMProviderTemplateById"></a>

`GET /llm-provider-templates/{id}`

> Code samples

```shell

curl -X GET http://localhost:9090/api/management/v0.9/llm-provider-templates/{id} \
  -u {username}:{password} \
  -H 'Accept: application/json'

```

Get an LLM provider template by its ID.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

Required roles: `admin`

</aside>

<h3 id="get-llm-provider-template-by-id-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|string|true|Unique public identifier for the LLM provider template|

> Example responses

> 200 Response

```json
{
  "apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
  "kind": "LlmProviderTemplate",
  "metadata": {
    "name": "openai-template"
  },
  "spec": {
    "displayName": "OpenAI",
    "promptTokens": {
      "location": "payload",
      "identifier": "$.usage.prompt_tokens"
    },
    "completionTokens": {
      "location": "payload",
      "identifier": "$.usage.completion_tokens"
    },
    "totalTokens": {
      "location": "payload",
      "identifier": "$.usage.total_tokens"
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
  },
  "status": {
    "id": "openai-template",
    "createdAt": "2026-04-24T07:21:13Z",
    "updatedAt": "2026-04-24T07:21:13Z"
  }
}
```

<h3 id="get-llm-provider-template-by-id-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|LLM provider template details|[LLMProviderTemplate](schemas.md#schemallmprovidertemplate)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|LLM provider template not found|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

## Update an existing LLM provider template

<a id="opIdupdateLLMProviderTemplate"></a>

`PUT /llm-provider-templates/{id}`

> Code samples

```shell

curl -X PUT http://localhost:9090/api/management/v0.9/llm-provider-templates/{id} \
  -u {username}:{password} \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -d @payload.json

```

Update an existing LLM provider template in the Gateway.

> Payload

```json
{
  "apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
  "kind": "LlmProviderTemplate",
  "metadata": {
    "name": "openai-template"
  },
  "spec": {
    "displayName": "OpenAI",
    "promptTokens": {
      "location": "payload",
      "identifier": "$.usage.prompt_tokens"
    },
    "completionTokens": {
      "location": "payload",
      "identifier": "$.usage.completion_tokens"
    },
    "totalTokens": {
      "location": "payload",
      "identifier": "$.usage.total_tokens"
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
}
```

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

Required roles: `admin`

</aside>

<h3 id="update-an-existing-llm-provider-template-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|string|true|Unique public identifier of the template to update|
|body|body|[LLMProviderTemplateRequest](schemas.md#schemallmprovidertemplaterequest)|true|none|

> Example responses

> 200 Response

```json
{
  "apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
  "kind": "LlmProviderTemplate",
  "metadata": {
    "name": "openai-template"
  },
  "spec": {
    "displayName": "OpenAI",
    "promptTokens": {
      "location": "payload",
      "identifier": "$.usage.prompt_tokens"
    },
    "completionTokens": {
      "location": "payload",
      "identifier": "$.usage.completion_tokens"
    },
    "totalTokens": {
      "location": "payload",
      "identifier": "$.usage.total_tokens"
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
  },
  "status": {
    "id": "openai-template",
    "createdAt": "2026-04-24T07:21:13Z",
    "updatedAt": "2026-04-24T07:21:13Z"
  }
}
```

<h3 id="update-an-existing-llm-provider-template-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|LLM provider template updated successfully|[LLMProviderTemplate](schemas.md#schemallmprovidertemplate)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Invalid configuration (validation failed)|[ErrorResponse](schemas.md#schemaerrorresponse)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|LLM provider template not found|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

## Delete an LLM provider template

<a id="opIddeleteLLMProviderTemplate"></a>

`DELETE /llm-provider-templates/{id}`

> Code samples

```shell

curl -X DELETE http://localhost:9090/api/management/v0.9/llm-provider-templates/{id} \
  -u {username}:{password} \
  -H 'Accept: application/json'

```

Delete an LLM provider template from the Gateway.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

Required roles: `admin`

</aside>

<h3 id="delete-an-llm-provider-template-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|string|true|Unique public identifier of the template to delete|

> Example responses

> 200 Response

```json
{
  "status": "success",
  "message": "LLM provider template deleted successfully",
  "id": "openai"
}
```

<h3 id="delete-an-llm-provider-template-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|LLM provider template deleted successfully|Inline|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|LLM provider template not found|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="delete-an-llm-provider-template-responseschema">Response Schema</h3>

Status Code **200**

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|» status|string|false|none|none|
|» message|string|false|none|none|
|» id|string|false|none|none|
