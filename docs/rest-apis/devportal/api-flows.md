<h1 id="wso2-api-developer-portal-core-devportal-routes-api-flows">API Flows</h1>

## Create an API flow

<a id="opIdcreateApiFlow"></a>

`POST /o/{orgId}/devportal/v1/views/{viewName}/api-flows`

> Code samples

```shell

curl -X POST https://devportal.api-platform.io/o/{orgId}/devportal/v1/views/{viewName}/api-flows \
  -u {username}:{password} \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}' \
  -d @payload.json

```

Creates an API flow in the selected view. If `handle` is omitted, the service generates one from the name. `ARAZZO` content is parsed from JSON or YAML; invalid Arazzo content returns a bad request.

> Payload

```json
{
  "name": "Weather onboarding",
  "handle": "weather-onboarding",
  "description": "Guides users through the Weather API onboarding workflow.",
  "contentType": "ARAZZO",
  "apiFlowDefinition": {
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

<h3 id="create-an-api-flow-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|[APIFlowCreateRequest](schemas.md#schemaapiflowcreaterequest)|true|API flow creation payload. Use `contentType` `ARAZZO` for JSON/YAML workflow content or `MD` for Markdown flow content.|
|orgId|path|string|true|none|
|viewName|path|string|true|none|

> Example responses

> 201 Response

```json
{
  "apiFlowId": "flow-12345",
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

<h3 id="create-an-api-flow-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|201|[Created](https://tools.ietf.org/html/rfc7231#section-6.3.2)|Created API flow summary.|[APIFlowCreateResponse](schemas.md#schemaapiflowcreateresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|JSON message response.|[MessageResponse](schemas.md#schemamessageresponse)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|JSON message response.|[MessageResponse](schemas.md#schemamessageresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|JSON message response.|[MessageResponse](schemas.md#schemamessageresponse)|

## List API flows

<a id="opIdgetAllApiFlows"></a>

`GET /o/{orgId}/devportal/v1/views/{viewName}/api-flows`

> Code samples

```shell

curl -X GET https://devportal.api-platform.io/o/{orgId}/devportal/v1/views/{viewName}/api-flows \
  -u {username}:{password} \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}'

```

Lists all API flows for the selected organization view.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="list-api-flows-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|limit|query|integer|false|Maximum number of records to return.|
|offset|query|integer|false|Number of records to skip before returning results.|
|orgId|path|string|true|none|
|viewName|path|string|true|none|

> Example responses

> 200 Response

```json
{
  "list": [
    {
      "apiFlowId": "flow-12345",
      "name": "Weather onboarding",
      "handle": "weather-onboarding",
      "description": "string",
      "agentPrompt": "string",
      "status": "PUBLISHED",
      "agentVisibility": "VISIBLE",
      "contentType": "ARAZZO",
      "apiFlowDefinition": "string",
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

<h3 id="list-api-flows-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|List of API flow DTOs.|Inline|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|JSON message response.|[MessageResponse](schemas.md#schemamessageresponse)|

<h3 id="list-api-flows-responseschema">Response Schema</h3>

Status Code **200**

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|» list|[[APIFlowResponse](schemas.md#schemaapiflowresponse)]|false|none|none|
|»» apiFlowId|string|false|none|none|
|»» name|string|false|none|none|
|»» handle|string|false|none|none|
|»» description|string|false|none|none|
|»» agentPrompt|string|false|none|none|
|»» status|string|false|none|none|
|»» agentVisibility|string|false|none|none|
|»» contentType|string|false|none|none|
|»» apiFlowDefinition|string¦null|false|none|none|
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
|contentType|ARAZZO|
|contentType|MD|

## Get an API flow

<a id="opIdgetApiFlow"></a>

`GET /o/{orgId}/devportal/v1/views/{viewName}/api-flows/{apiFlowId}`

> Code samples

```shell

curl -X GET https://devportal.api-platform.io/o/{orgId}/devportal/v1/views/{viewName}/api-flows/{apiFlowId} \
  -u {username}:{password} \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}'

```

Retrieves a single API flow by ID from the selected view.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="get-an-api-flow-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|orgId|path|string|true|none|
|viewName|path|string|true|none|
|apiFlowId|path|string|true|none|

> Example responses

> 200 Response

```json
{
  "apiFlowId": "flow-12345",
  "name": "Weather onboarding",
  "handle": "weather-onboarding",
  "description": "Guides users through the Weather API onboarding workflow.",
  "agentPrompt": "Follow this workflow to onboard a Weather API user.",
  "status": "PUBLISHED",
  "agentVisibility": "VISIBLE",
  "contentType": "ARAZZO",
  "apiFlowDefinition": "{\"arazzo\":\"1.0.1\",\"info\":{\"title\":\"Weather onboarding\",\"version\":\"1.0.0\"},\"workflows\":[]}",
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

<h3 id="get-an-api-flow-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|API flow DTO.|[APIFlowResponse](schemas.md#schemaapiflowresponse)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|JSON message response.|[MessageResponse](schemas.md#schemamessageresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|JSON message response.|[MessageResponse](schemas.md#schemamessageresponse)|

## Update an API flow

<a id="opIdupdateApiFlow"></a>

`PUT /o/{orgId}/devportal/v1/views/{viewName}/api-flows/{apiFlowId}`

> Code samples

```shell

curl -X PUT https://devportal.api-platform.io/o/{orgId}/devportal/v1/views/{viewName}/api-flows/{apiFlowId} \
  -u {username}:{password} \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}' \
  -d @payload.json

```

Updates API flow metadata and content for the selected view. Duplicate handles return a conflict.

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
  "apiFlowDefinition": {},
  "markdownContent": "string"
}
```

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="update-an-api-flow-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|[APIFlowUpdateRequest](schemas.md#schemaapiflowupdaterequest)|true|API flow update payload. Include only the fields that should change.|
|orgId|path|string|true|none|
|viewName|path|string|true|none|
|apiFlowId|path|string|true|none|

> Example responses

> 200 Response

```json
{
  "message": "string"
}
```

<h3 id="update-an-api-flow-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|JSON message response.|[MessageResponse](schemas.md#schemamessageresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|JSON message response.|[MessageResponse](schemas.md#schemamessageresponse)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|JSON message response.|[MessageResponse](schemas.md#schemamessageresponse)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|JSON message response.|[MessageResponse](schemas.md#schemamessageresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|JSON message response.|[MessageResponse](schemas.md#schemamessageresponse)|

## Delete an API flow

<a id="opIddeleteApiFlow"></a>

`DELETE /o/{orgId}/devportal/v1/views/{viewName}/api-flows/{apiFlowId}`

> Code samples

```shell

curl -X DELETE https://devportal.api-platform.io/o/{orgId}/devportal/v1/views/{viewName}/api-flows/{apiFlowId} \
  -u {username}:{password} \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}'

```

Deletes an API flow from the selected view.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="delete-an-api-flow-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|orgId|path|string|true|none|
|viewName|path|string|true|none|
|apiFlowId|path|string|true|none|

> Example responses

> 200 Response

```json
{
  "message": "string"
}
```

<h3 id="delete-an-api-flow-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|JSON message response.|[MessageResponse](schemas.md#schemamessageresponse)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|JSON message response.|[MessageResponse](schemas.md#schemamessageresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|JSON message response.|[MessageResponse](schemas.md#schemamessageresponse)|

## Generate an API flow agent prompt

<a id="opIdgeneratePrompt"></a>

`POST /o/{orgId}/devportal/v1/views/{viewName}/api-flows/generate-prompt`

> Code samples

```shell

curl -X POST https://devportal.api-platform.io/o/{orgId}/devportal/v1/views/{viewName}/api-flows/generate-prompt \
  -u {username}:{password} \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}' \
  -d @payload.json

```

Generates the default agent prompt text for a proposed API flow using the supplied name, description, APIs, and view context.

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

<h3 id="generate-an-api-flow-agent-prompt-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|[APIFlowPromptRequest](schemas.md#schemaapiflowpromptrequest)|true|API flow prompt-generation payload.|
|orgId|path|string|true|none|
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

<h3 id="generate-an-api-flow-agent-prompt-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Generated API flow agent prompt.|[APIFlowPromptResponse](schemas.md#schemaapiflowpromptresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|JSON message response.|[MessageResponse](schemas.md#schemamessageresponse)|
