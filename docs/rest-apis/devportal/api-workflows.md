<h1 id="wso2-api-developer-portal-core-devportal-routes-api-workflows">API Workflows</h1>

## Create an API workflow

<a id="opIdcreateApiWorkflow"></a>

`POST /devportal/v1/views/{viewName}/api-workflows`

> Code samples

```shell

curl -X POST https://devportal.api-platform.io/devportal/v1/views/{viewName}/api-workflows \
  -u {username}:{password} \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}' \
  -d @payload.json

```

Creates an API workflow in the selected view. If `handle` is omitted, the service generates one from the name. `ARAZZO` content is parsed from JSON or YAML; invalid Arazzo content returns a bad request.

> Payload

```json
{
  "name": "Weather onboarding",
  "handle": "weather-onboarding",
  "description": "Guides users through the Weather API onboarding workflow.",
  "contentType": "ARAZZO",
  "apiWorkflowDefinition": {
    "arazzo": "1.0.1",
    "info": {
      "title": "Weather onboarding",
      "version": "1.0.0"
    },
    "workflows": []
  }
}
```

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="create-an-api-workflow-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|[APIWorkflowCreateRequest](schemas.md#schemaapiworkflowcreaterequest)|true|API workflow creation payload. Use `contentType` `ARAZZO` for JSON/YAML workflow content or `MD` for Markdown workflow content.|
|viewName|path|string|true|none|

> Example responses

> 201 Response

```json
{
  "apiWorkflowId": "workflow-12345",
  "name": "Weather onboarding",
  "status": "PUBLISHED"
}
```

> 400 Response

```json
{
  "message": "string"
}
```

<h3 id="create-an-api-workflow-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|201|[Created](https://tools.ietf.org/html/rfc7231#section-6.3.2)|Created API workflow summary.|[APIWorkflowCreateResponse](schemas.md#schemaapiworkflowcreateresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|JSON message response.|[MessageResponse](schemas.md#schemamessageresponse)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|JSON message response.|[MessageResponse](schemas.md#schemamessageresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|JSON message response.|[MessageResponse](schemas.md#schemamessageresponse)|

## List API workflows

<a id="opIdgetAllApiWorkflows"></a>

`GET /devportal/v1/views/{viewName}/api-workflows`

> Code samples

```shell

curl -X GET https://devportal.api-platform.io/devportal/v1/views/{viewName}/api-workflows \
  -u {username}:{password} \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}'

```

Lists all API workflows for the selected organization view.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="list-api-workflows-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|limit|query|integer|false|Maximum number of records to return.|
|offset|query|integer|false|Number of records to skip before returning results.|
|viewName|path|string|true|none|

> Example responses

> 200 Response

```json
{
  "list": [
    {
      "apiWorkflowId": "workflow-12345",
      "name": "Weather onboarding",
      "handle": "weather-onboarding",
      "description": "string",
      "agentPrompt": "string",
      "status": "PUBLISHED",
      "agentVisibility": "VISIBLE",
      "contentType": "ARAZZO",
      "apiWorkflowDefinition": "string",
      "markdownContent": "string",
      "createdAt": "May 7, 2026",
      "updatedAt": "string"
    }
  ],
  "pagination": {
    "total": 42,
    "limit": 20,
    "offset": 0
  }
}
```

<h3 id="list-api-workflows-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|List of API workflow DTOs.|Inline|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|JSON message response.|[MessageResponse](schemas.md#schemamessageresponse)|

<h3 id="list-api-workflows-responseschema">Response Schema</h3>

Status Code **200**

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|» list|[[APIWorkflowResponse](schemas.md#schemaapiworkflowresponse)]|false|none|none|
|»» apiWorkflowId|string|false|none|none|
|»» name|string|false|none|none|
|»» handle|string|false|none|none|
|»» description|string|false|none|none|
|»» agentPrompt|string|false|none|none|
|»» status|string|false|none|none|
|»» agentVisibility|string|false|none|none|
|»» contentType|string|false|none|none|
|»» apiWorkflowDefinition|string¦null|false|none|none|
|»» markdownContent|string¦null|false|none|none|
|»» createdAt|string|false|none|none|
|»» updatedAt|string¦null|false|none|none|
|» pagination|[Pagination](schemas.md#schemapagination)|false|none|Standard pagination metadata returned with collection responses.|
|»» total|integer|true|none|Total number of records matching the query.|
|»» limit|integer|true|none|Maximum number of records returned in this response.|
|»» offset|integer|true|none|Number of records skipped before this page.|

#### Enumerated Values

|Property|Value|
|---|---|
|status|DRAFT|
|status|PUBLISHED|
|agentVisibility|VISIBLE|
|agentVisibility|HIDDEN|
|contentType|ARAZZO|
|contentType|MD|

## Get an API workflow

<a id="opIdgetApiWorkflow"></a>

`GET /devportal/v1/views/{viewName}/api-workflows/{apiWorkflowId}`

> Code samples

```shell

curl -X GET https://devportal.api-platform.io/devportal/v1/views/{viewName}/api-workflows/{apiWorkflowId} \
  -u {username}:{password} \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}'

```

Retrieves a single API workflow by ID from the selected view.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="get-an-api-workflow-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|viewName|path|string|true|none|
|apiWorkflowId|path|string|true|none|

> Example responses

> 200 Response

```json
{
  "apiWorkflowId": "workflow-12345",
  "name": "Weather onboarding",
  "handle": "weather-onboarding",
  "description": "Guides users through the Weather API onboarding workflow.",
  "agentPrompt": "Follow this workflow to onboard a Weather API user.",
  "status": "PUBLISHED",
  "agentVisibility": "VISIBLE",
  "contentType": "ARAZZO",
  "apiWorkflowDefinition": "{\"arazzo\":\"1.0.1\",\"info\":{\"title\":\"Weather onboarding\",\"version\":\"1.0.0\"},\"workflows\":[]}",
  "markdownContent": null,
  "createdAt": "May 7, 2026",
  "updatedAt": "2026-05-07T08:30:00Z"
}
```

> 404 Response

```json
{
  "message": "string"
}
```

<h3 id="get-an-api-workflow-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|API workflow DTO.|[APIWorkflowResponse](schemas.md#schemaapiworkflowresponse)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|JSON message response.|[MessageResponse](schemas.md#schemamessageresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|JSON message response.|[MessageResponse](schemas.md#schemamessageresponse)|

## Update an API workflow

<a id="opIdupdateApiWorkflow"></a>

`PUT /devportal/v1/views/{viewName}/api-workflows/{apiWorkflowId}`

> Code samples

```shell

curl -X PUT https://devportal.api-platform.io/devportal/v1/views/{viewName}/api-workflows/{apiWorkflowId} \
  -u {username}:{password} \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}' \
  -d @payload.json

```

Updates API workflow metadata and content for the selected view. Duplicate handles return a conflict.

> Payload

```json
{
  "name": "Weather onboarding v2",
  "handle": "weather-onboarding-v2",
  "description": "Updated Weather API onboarding workflow.",
  "agentPrompt": "string",
  "status": "PUBLISHED",
  "agentVisibility": "VISIBLE",
  "contentType": "ARAZZO",
  "apiWorkflowDefinition": {},
  "markdownContent": "string"
}
```

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="update-an-api-workflow-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|[APIWorkflowUpdateRequest](schemas.md#schemaapiworkflowupdaterequest)|true|API workflow update payload. Include only the fields that should change.|
|viewName|path|string|true|none|
|apiWorkflowId|path|string|true|none|

> Example responses

> 200 Response

```json
{
  "message": "string"
}
```

<h3 id="update-an-api-workflow-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|JSON message response.|[MessageResponse](schemas.md#schemamessageresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|JSON message response.|[MessageResponse](schemas.md#schemamessageresponse)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|JSON message response.|[MessageResponse](schemas.md#schemamessageresponse)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|JSON message response.|[MessageResponse](schemas.md#schemamessageresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|JSON message response.|[MessageResponse](schemas.md#schemamessageresponse)|

## Delete an API workflow

<a id="opIddeleteApiWorkflow"></a>

`DELETE /devportal/v1/views/{viewName}/api-workflows/{apiWorkflowId}`

> Code samples

```shell

curl -X DELETE https://devportal.api-platform.io/devportal/v1/views/{viewName}/api-workflows/{apiWorkflowId} \
  -u {username}:{password} \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}'

```

Deletes an API workflow from the selected view.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="delete-an-api-workflow-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|viewName|path|string|true|none|
|apiWorkflowId|path|string|true|none|

> Example responses

> 200 Response

```json
{
  "message": "string"
}
```

<h3 id="delete-an-api-workflow-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|JSON message response.|[MessageResponse](schemas.md#schemamessageresponse)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|JSON message response.|[MessageResponse](schemas.md#schemamessageresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|JSON message response.|[MessageResponse](schemas.md#schemamessageresponse)|

## Generate an API workflow agent prompt

<a id="opIdgeneratePrompt"></a>

`POST /devportal/v1/views/{viewName}/api-workflows/generate-prompt`

> Code samples

```shell

curl -X POST https://devportal.api-platform.io/devportal/v1/views/{viewName}/api-workflows/generate-prompt \
  -u {username}:{password} \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}' \
  -d @payload.json

```

Generates the default agent prompt text for a proposed API workflow using the supplied name, description, APIs, and view context.

> Payload

```json
{
  "name": "Weather onboarding",
  "description": "Guides users through the Weather API onboarding workflow.",
  "apis": [
    {}
  ],
  "orgHandle": "acme",
  "viewName": "default",
  "handle": "weather-onboarding"
}
```

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="generate-an-api-workflow-agent-prompt-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|[APIWorkflowPromptRequest](schemas.md#schemaapiworkflowpromptrequest)|true|API workflow prompt-generation payload.|
|viewName|path|string|true|none|

> Example responses

> 200 Response

```json
{
  "agentPrompt": "You are an API workflow assistant. Help the user complete Weather onboarding."
}
```

> 500 Response

```json
{
  "message": "string"
}
```

<h3 id="generate-an-api-workflow-agent-prompt-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Generated API workflow agent prompt.|[APIWorkflowPromptResponse](schemas.md#schemaapiworkflowpromptresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|JSON message response.|[MessageResponse](schemas.md#schemamessageresponse)|
