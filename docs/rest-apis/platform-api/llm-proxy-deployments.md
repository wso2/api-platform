<h1 id="wso2-api-platform-platform-api-llm-proxy-deployments">LLM Proxy Deployments</h1>

LLM proxy deployment operations

## Create and deploy a new LLM proxy deployment

<a id="opIddeployLLMProxy"></a>

`POST /llm-proxies/{llmProxyId}/deployments`

> Code samples

```shell

curl -X POST https://localhost:9243/api/v0.9/llm-proxies/{llmProxyId}/deployments \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -d @payload.json

```

Creates an immutable deployment artifact for an LLM proxy and deploys it to a specified gateway.
Each deployment targets a single gateway. The proxyId parameter is the LLM proxy handle (identifier),
not the UUID. The operation returns a transitional DEPLOYING status. Final success or failure will be reported asynchronously via the deployment's status and statusReason once the gateway acknowledges.
Access is validated against the organization in the JWT token.

> Payload

```json
{
  "name": "v1.0-production",
  "base": "current",
  "gatewayId": "prod-gateway-01",
  "metadata": {}
}
```

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:llm_proxy:deployment:create`, `ap:llm_proxy:deployment:manage`, `ap:llm_proxy:manage`

</aside>

<h3 id="create-and-deploy-a-new-llm-proxy-deployment-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|llmProxyId|path|string|true|Unique identifier of the LLM proxy|
|body|body|[DeployRequest](schemas.md#schemadeployrequest)|true|Deployment request with gateway ID, base reference, and metadata|

> Example responses

> 201 Response

```json
{
  "deploymentId": "a73c85a1-d857-491e-a6b2-51dce05de7a2",
  "name": "v1.0-production",
  "gatewayId": "prod-gateway-01",
  "status": "DEPLOYED",
  "baseDeploymentId": "be6d8692-b9de-400e-b6c1-14db50154e27",
  "metadata": {},
  "createdAt": "2019-08-24T14:15:22Z",
  "statusReason": "string",
  "updatedAt": "2019-08-24T14:15:22Z"
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

<h3 id="create-and-deploy-a-new-llm-proxy-deployment-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|201|[Created](https://tools.ietf.org/html/rfc7231#section-6.3.2)|LLM proxy deployed successfully|[DeploymentResponse](schemas.md#schemadeploymentresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad Request. Invalid request or validation error.|[Error](schemas.md#schemaerror)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Forbidden. The authenticated user does not have permission to access this resource.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|

### Response Headers

|Status|Header|Type|Format|Description|
|---|---|---|---|---|
|201|Location|string|uri|URL of the newly created resource.|

## Get deployments for an LLM proxy

<a id="opIdgetLLMProxyDeployments"></a>

`GET /llm-proxies/{llmProxyId}/deployments`

> Code samples

```shell

curl -X GET https://localhost:9243/api/v0.9/llm-proxies/{llmProxyId}/deployments \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Accept: application/json'

```

Retrieves all deployment artifacts for a specific LLM proxy. The proxyId parameter is the
LLM proxy handle (identifier), not the UUID. Supports filtering by gateway UUID and deployment status.
Access is validated against the organization in the JWT token.

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:llm_proxy:deployment:read`, `ap:llm_proxy:deployment:manage`, `ap:llm_proxy:manage`

</aside>

<h3 id="get-deployments-for-an-llm-proxy-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|llmProxyId|path|string|true|Unique identifier of the LLM proxy|
|gatewayId|query|string|false|**Gateway ID** consisting of the **UUID** of the Gateway to filter status by.|
|status|query|string|false|Filter deployments by status (DEPLOYED, UNDEPLOYED, DEPLOYING, UNDEPLOYING, FAILED, or ARCHIVED)|
|limit|query|integer|false|Maximum number of items to return per page.|
|offset|query|integer|false|Zero-based index of the first item to return.|

#### Detailed descriptions

**gatewayId**: **Gateway ID** consisting of the **UUID** of the Gateway to filter status by.

#### Enumerated Values

|Parameter|Value|
|---|---|
|status|DEPLOYED|
|status|UNDEPLOYED|
|status|DEPLOYING|
|status|UNDEPLOYING|
|status|FAILED|
|status|ARCHIVED|

> Example responses

> 200 Response

```json
{
  "count": 0,
  "list": [
    {
      "deploymentId": "a73c85a1-d857-491e-a6b2-51dce05de7a2",
      "name": "v1.0-production",
      "gatewayId": "prod-gateway-01",
      "status": "DEPLOYED",
      "baseDeploymentId": "be6d8692-b9de-400e-b6c1-14db50154e27",
      "metadata": {},
      "createdAt": "2019-08-24T14:15:22Z",
      "statusReason": "string",
      "updatedAt": "2019-08-24T14:15:22Z"
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

<h3 id="get-deployments-for-an-llm-proxy-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Deployments retrieved successfully|[DeploymentListResponse](schemas.md#schemadeploymentlistresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad Request. Invalid request or validation error.|[Error](schemas.md#schemaerror)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|

## Get LLM proxy deployment by ID

<a id="opIdgetLLMProxyDeployment"></a>

`GET /llm-proxies/{llmProxyId}/deployments/{deploymentId}`

> Code samples

```shell

curl -X GET https://localhost:9243/api/v0.9/llm-proxies/{llmProxyId}/deployments/{deploymentId} \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Accept: application/json'

```

Retrieves metadata for a specific LLM proxy deployment artifact including status, gateway association,
and timestamps. Access is validated against the organization in the JWT token.

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:llm_proxy:deployment:read`, `ap:llm_proxy:deployment:manage`, `ap:llm_proxy:manage`

</aside>

<h3 id="get-llm-proxy-deployment-by-id-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|llmProxyId|path|string|true|Unique identifier of the LLM proxy|
|deploymentId|path|string(uuid)|true|The UUID of the deployment|

> Example responses

> 200 Response

```json
{
  "deploymentId": "a73c85a1-d857-491e-a6b2-51dce05de7a2",
  "name": "v1.0-production",
  "gatewayId": "prod-gateway-01",
  "status": "DEPLOYED",
  "baseDeploymentId": "be6d8692-b9de-400e-b6c1-14db50154e27",
  "metadata": {},
  "createdAt": "2019-08-24T14:15:22Z",
  "statusReason": "string",
  "updatedAt": "2019-08-24T14:15:22Z"
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

<h3 id="get-llm-proxy-deployment-by-id-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Deployment metadata retrieved successfully|[DeploymentResponse](schemas.md#schemadeploymentresponse)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|

## Delete LLM proxy deployment

<a id="opIddeleteLLMProxyDeployment"></a>

`DELETE /llm-proxies/{llmProxyId}/deployments/{deploymentId}`

> Code samples

```shell

curl -X DELETE https://localhost:9243/api/v0.9/llm-proxies/{llmProxyId}/deployments/{deploymentId} \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Accept: application/json'

```

Deletes a deployment artifact. Deletion is only allowed when the deployment is in UNDEPLOYED status.
Access is validated against the organization in the JWT token.

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:llm_proxy:deployment:delete`, `ap:llm_proxy:deployment:manage`, `ap:llm_proxy:manage`

</aside>

<h3 id="delete-llm-proxy-deployment-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|llmProxyId|path|string|true|Unique identifier of the LLM proxy|
|deploymentId|path|string(uuid)|true|The UUID of the deployment|

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

<h3 id="delete-llm-proxy-deployment-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|204|[No Content](https://tools.ietf.org/html/rfc7231#section-6.3.5)|Deployment deleted successfully|None|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad Request. Invalid request or validation error.|[Error](schemas.md#schemaerror)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Forbidden. The authenticated user does not have permission to access this resource.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Conflict. Specified resource already exists.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|

## Undeploy LLM proxy deployment from gateway

<a id="opIdundeployLLMProxyDeployment"></a>

`POST /llm-proxies/{llmProxyId}/deployments/{deploymentId}/undeploy`

> Code samples

```shell

curl -X POST https://localhost:9243/api/v0.9/llm-proxies/{llmProxyId}/deployments/{deploymentId}/undeploy?gatewayId=string \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Accept: application/json'

```

Undeploys an active LLM proxy deployment, stopping it from being served on the specified gateway.
The deployment artifact remains in the system and can be restored later.
Returns the updated deployment object with initial status UNDEPLOYING. Final status (UNDEPLOYED or FAILED) will be reported asynchronously via the deployment's status and statusReason once the gateway acknowledges.

The gatewayId query parameter is validated against the deployment's bound gateway to prevent unintended operations.
Access is validated against the organization in the JWT token.

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:llm_proxy:deployment:undeploy`, `ap:llm_proxy:deployment:manage`, `ap:llm_proxy:manage`

</aside>

<h3 id="undeploy-llm-proxy-deployment-from-gateway-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|llmProxyId|path|string|true|Unique identifier of the LLM proxy|
|deploymentId|path|string|true|UUID of the deployment to undeploy|
|gatewayId|query|string|true|UUID of the gateway (validated against deployment's bound gateway)|

> Example responses

> 200 Response

```json
{
  "deploymentId": "a73c85a1-d857-491e-a6b2-51dce05de7a2",
  "name": "v1.0-production",
  "gatewayId": "prod-gateway-01",
  "status": "DEPLOYED",
  "baseDeploymentId": "be6d8692-b9de-400e-b6c1-14db50154e27",
  "metadata": {},
  "createdAt": "2019-08-24T14:15:22Z",
  "statusReason": "string",
  "updatedAt": "2019-08-24T14:15:22Z"
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

<h3 id="undeploy-llm-proxy-deployment-from-gateway-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Undeploy initiated successfully. Returns the deployment with initial status UNDEPLOYING. Poll status for final result.|[DeploymentResponse](schemas.md#schemadeploymentresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad Request. Invalid request or validation error.|[Error](schemas.md#schemaerror)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Forbidden. The authenticated user does not have permission to access this resource.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Conflict. Specified resource already exists.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|

## Restore a previous LLM proxy deployment

<a id="opIdrestoreLLMProxyDeployment"></a>

`POST /llm-proxies/{llmProxyId}/deployments/{deploymentId}/restore`

> Code samples

```shell

curl -X POST https://localhost:9243/api/v0.9/llm-proxies/{llmProxyId}/deployments/{deploymentId}/restore?gatewayId=string \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Accept: application/json'

```

Initiates restoring a previous deployment (ARCHIVED or UNDEPLOYED) on the specified gateway.
Returns the deployment with initial status DEPLOYING. Final success or failure will be reported asynchronously via the deployment's status and statusReason once the gateway acknowledges.
The target deployment must not already be in DEPLOYED status.

The gatewayId query parameter is validated against the deployment's bound gateway to prevent unintended operations.
Access is validated against the organization in the JWT token.

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:llm_proxy:deployment:restore`, `ap:llm_proxy:deployment:manage`, `ap:llm_proxy:manage`

</aside>

<h3 id="restore-a-previous-llm-proxy-deployment-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|llmProxyId|path|string|true|Unique identifier of the LLM proxy|
|deploymentId|path|string|true|UUID of the deployment to restore (must be ARCHIVED or UNDEPLOYED)|
|gatewayId|query|string|true|UUID of the gateway (validated against deployment's bound gateway)|

> Example responses

> 200 Response

```json
{
  "deploymentId": "a73c85a1-d857-491e-a6b2-51dce05de7a2",
  "name": "v1.0-production",
  "gatewayId": "prod-gateway-01",
  "status": "DEPLOYED",
  "baseDeploymentId": "be6d8692-b9de-400e-b6c1-14db50154e27",
  "metadata": {},
  "createdAt": "2019-08-24T14:15:22Z",
  "statusReason": "string",
  "updatedAt": "2019-08-24T14:15:22Z"
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

<h3 id="restore-a-previous-llm-proxy-deployment-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Restore initiated successfully. Returns the deployment with initial status DEPLOYING. Poll status for final result.|[DeploymentResponse](schemas.md#schemadeploymentresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad Request. Invalid request or validation error.|[Error](schemas.md#schemaerror)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Forbidden. The authenticated user does not have permission to access this resource.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Conflict. Specified resource already exists.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|
