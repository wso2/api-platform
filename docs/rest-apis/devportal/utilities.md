<h1 id="wso2-api-developer-portal-core-devportal-routes-utilities">Utilities</h1>

## Create a temporary Arazzo file

<a id="opIdcreateTempArazzoFile"></a>

`POST /temp-arazzo-file`

> Code samples

```shell

curl -X POST https://devportal.api-platform.io/temp-arazzo-file \
  -u {username}:{password} \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}' \
  -d @payload.json

```

Writes supplied Arazzo YAML content to a temporary file and returns its path.

> Payload

```json
{
  "content": "arazzo: 1.0.1\ninfo:\n  title: Weather onboarding\n  version: 1.0.0\nworkflows: []\n",
  "filename": "workflow.arazzo.yaml"
}
```

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="create-a-temporary-arazzo-file-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|[TempArazzoFileRequest](schemas.md#schematemparazzofilerequest)|true|none|

> Example responses

> 200 Response

```json
{
  "path": "/tmp/arazzo-abc123/workflow.arazzo.yaml"
}
```

> Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.

```json
[
  {
    "status": "error",
    "code": "COMMON_VALIDATION_ERROR",
    "message": "Input validation failed.",
    "errors": [
      {
        "field": "name",
        "message": "name is required."
      }
    ]
  }
]
```

```json
{
  "status": "error",
  "code": "MISSING_REQUIRED_PARAMETER",
  "message": "Missing required parameter."
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
  "status": "error",
  "code": "INTERNAL_SERVER_ERROR",
  "message": "An unexpected error occurred."
}
```

<h3 id="create-a-temporary-arazzo-file-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Path of the temporary Arazzo file.|[TempArazzoFileResponse](schemas.md#schematemparazzofileresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.|Inline|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="create-a-temporary-arazzo-file-responseschema">Response Schema</h3>

#### Enumerated Values

|Property|Value|
|---|---|
|status|error|
|status|error|
