<h1 id="wso2-api-platform-platform-api-api-keys">API Keys</h1>

API key management operations for REST APIs and LLM Providers

## List API keys for the current user

<a id="opIdlistUserAPIKeys"></a>

`GET /me/api-keys`

> Code samples

```shell

curl -X GET https://localhost:9243/api/v0.9/me/api-keys \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Accept: application/json'

```

Returns API keys created by the caller within the organization.
Optionally filter by one or more artifact types using a comma-separated `type` query parameter.
The plain key value is never returned.

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:api_key:read`

</aside>

<h3 id="list-api-keys-for-the-current-user-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|type|query|array[string]|false|Comma-separated list of artifact types to filter by.|
|limit|query|integer|false|Maximum number of items to return per page.|
|offset|query|integer|false|Zero-based index of the first item to return.|

#### Detailed descriptions

**type**: Comma-separated list of artifact types to filter by.
If omitted, all types are returned.

#### Enumerated Values

|Parameter|Value|
|---|---|
|type|RestApi|
|type|LlmProvider|
|type|LlmProxy|

> Example responses

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
      "allowedTargets": "string",
      "artifactId": "wso2-openai-provider",
      "artifactType": "RestApi"
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

> 500 Response

```json
{
  "status": "error",
  "code": "INTERNAL_ERROR",
  "message": "An unexpected error occurred.",
  "trackingId": "4f1c6f2e-8a4b-4c93-b1de-9f2f6f0c2a11"
}
```

<h3 id="list-api-keys-for-the-current-user-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|List of API keys retrieved successfully|[UserAPIKeyListResponse](schemas.md#schemauserapikeylistresponse)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|
