<h1 id="wso2-api-platform-platform-api-agent-proxy-deployments">Agent Proxy Deployments</h1>

Agent proxy deployment operations

## Create and deploy a new deployment for an Agent proxy

<a id="opIdcreateAgentProxyDeployment"></a>

`POST /agent-proxies/{agentProxyId}/deployments`

> Code samples

```shell

curl -X POST https://localhost:9243/api/v0.9/agent-proxies/{agentProxyId}/deployments \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -d @payload.json

```

Creates the immutable deployment record synchronously and begins gateway processing.
Each deployment targets a single gateway. The 201 body reports the transitional
`DEPLOYING` status; poll the `Location` URI for the eventual status and `statusReason`.
This is not a completed gateway deployment. Access is validated against the
organization in the JWT token.

The gateway reports the outcome asynchronously:

- `DEPLOYED` means the gateway accepted, validated and stored the Agent. It does not
  confirm that the gateway is serving the Agent's routes: route application happens
  after the acknowledgement, and a policy engine that cannot resolve A2A routes is not
  detected.
- `FAILED` carries a `statusReason` code: `AGENT_VALIDATION_FAILED` (the gateway
  rejected the definition), `AGENT_CONFIG_RENDER_FAILED` (a template or secret reference
  could not be resolved), `AGENT_CONFLICT` (the gateway already holds another Agent
  with the same handle, or the same name and version), `AGENT_ARTIFACT_FETCH_FAILED`
  (the gateway could not retrieve the deployment artifact), or
  `GATEWAY_PROCESSING_ERROR`.
- The control plane refuses a gateway that reports an LTS release without the Agent
  kind with `400 DEPLOYMENT_KIND_UNSUPPORTED_BY_GATEWAY` before anything is stored. A
  gateway that reports no version is taken to be a current build; if it still lacks
  Agent support it never acknowledges the deployment, which ends `FAILED` with
  `DEPLOYMENT_TIMEOUT`.

> Payload

```json
{
  "name": "v1.0-production",
  "base": "current",
  "buildId": "2026-01-31-2",
  "gatewayId": "prod-gateway-01",
  "metadata": {}
}
```

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:api:agent:deployment:create`, `ap:api:agent:deployment:manage`, `ap:api:agent:manage`, `ap:api:manage`, `ap:manage`

</aside>

<h3 id="create-and-deploy-a-new-deployment-for-an-agent-proxy-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|agentProxyId|path|string|true|**Agent Proxy ID** consisting of the **handle** (unique slug identifier) of the Agent proxy.|
|body|body|[DeployRequest](schemas.md#schemadeployrequest)|true|Deployment request with gateway ID, base reference, and metadata|

#### Detailed descriptions

**agentProxyId**: **Agent Proxy ID** consisting of the **handle** (unique slug identifier) of the Agent proxy.

> Example responses
>
> Asynchronous operation accepted; poll the deployment until status becomes DEPLOYED or FAILED.

```json
{
  "deploymentId": "a73c85a1-d857-491e-a6b2-51dce05de7a2",
  "name": "v1.0-production",
  "gatewayId": "prod-gateway-01",
  "status": "DEPLOYING",
  "baseDeploymentId": "be6d8692-b9de-400e-b6c1-14db50154e27",
  "metadata": {},
  "createdAt": "2019-08-24T14:15:22Z",
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

> 409 Response

```json
{
  "status": "error",
  "code": "CONFLICT",
  "message": "The request conflicts with the current state of the resource."
}
```

> 415 Response

```json
{
  "status": "error",
  "code": "UNSUPPORTED_MEDIA_TYPE",
  "message": "This operation accepts application/json only."
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

<h3 id="create-and-deploy-a-new-deployment-for-an-agent-proxy-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|201|[Created](https://tools.ietf.org/html/rfc7231#section-6.3.2)|Immutable deployment record created; gateway processing is pending.|[DeploymentResponse](schemas.md#schemadeploymentresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad Request. Invalid request or validation error.|[Error](schemas.md#schemaerror)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Conflict. The request conflicts with the current state of the resource.|[Error](schemas.md#schemaerror)|
|415|[Unsupported Media Type](https://tools.ietf.org/html/rfc7231#section-6.5.13)|Unsupported Media Type. The request body media type is not supported by this operation.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|

### Response Headers

|Status|Header|Type|Format|Description|
|---|---|---|---|---|
|201|Location|string|uri|URL of the newly created resource.|

## Get deployments for an Agent proxy

<a id="opIdlistAgentProxyDeployments"></a>

`GET /agent-proxies/{agentProxyId}/deployments`

> Code samples

```shell

curl -X GET https://localhost:9243/api/v0.9/agent-proxies/{agentProxyId}/deployments \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Accept: application/json'

```

Retrieves the Agent proxy's deployment records, optionally filtered by gateway handle
and deployment status. Access is validated against the organization in the JWT token.

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:api:agent:deployment:read`, `ap:api:agent:deployment:manage`, `ap:api:agent:read`, `ap:api:agent:manage`, `ap:api:read`, `ap:api:manage`, `ap:read`, `ap:manage`

</aside>

<h3 id="get-deployments-for-an-agent-proxy-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|agentProxyId|path|string|true|**Agent Proxy ID** consisting of the **handle** (unique slug identifier) of the Agent proxy.|
|gatewayId|query|string|false|**Gateway ID** consisting of the **handle** (unique slug identifier) of the Gateway to filter status by.|
|status|query|string|false|Filter deployments by status (DEPLOYED, UNDEPLOYED, DEPLOYING, UNDEPLOYING, FAILED, or ARCHIVED)|
|limit|query|integer|false|Maximum number of items to return per page.|
|offset|query|integer|false|Zero-based index of the first item to return.|

#### Detailed descriptions

**agentProxyId**: **Agent Proxy ID** consisting of the **handle** (unique slug identifier) of the Agent proxy.

**gatewayId**: **Gateway ID** consisting of the **handle** (unique slug identifier) of the Gateway to filter status by.

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
>
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
      "buildId": "2026-01-31-2",
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

<h3 id="get-deployments-for-an-agent-proxy-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Deployments retrieved successfully|[DeploymentListResponse](schemas.md#schemadeploymentlistresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad Request. Invalid request or validation error.|[Error](schemas.md#schemaerror)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|

## Get Agent proxy deployment by ID

<a id="opIdgetAgentProxyDeployment"></a>

`GET /agent-proxies/{agentProxyId}/deployments/{deploymentId}`

> Code samples

```shell

curl -X GET https://localhost:9243/api/v0.9/agent-proxies/{agentProxyId}/deployments/{deploymentId} \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Accept: application/json'

```

Returns the deployment's current lifecycle state, gateway association, timestamps and,
on failure, its `statusReason`. Access is validated against the organization in the
JWT token.

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:api:agent:deployment:read`, `ap:api:agent:deployment:manage`, `ap:api:agent:read`, `ap:api:agent:manage`, `ap:api:read`, `ap:api:manage`, `ap:read`, `ap:manage`

</aside>

<h3 id="get-agent-proxy-deployment-by-id-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|agentProxyId|path|string|true|**Agent Proxy ID** consisting of the **handle** (unique slug identifier) of the Agent proxy.|
|deploymentId|path|string(uuid)|true|The UUID of the deployment|

#### Detailed descriptions

**agentProxyId**: **Agent Proxy ID** consisting of the **handle** (unique slug identifier) of the Agent proxy.

> Example responses
>
> 200 Response

```json
{
  "deploymentId": "a73c85a1-d857-491e-a6b2-51dce05de7a2",
  "name": "v1.0-production",
  "gatewayId": "prod-gateway-01",
  "status": "DEPLOYED",
  "baseDeploymentId": "be6d8692-b9de-400e-b6c1-14db50154e27",
  "buildId": "2026-01-31-2",
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

<h3 id="get-agent-proxy-deployment-by-id-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Deployment metadata retrieved successfully|[DeploymentResponse](schemas.md#schemadeploymentresponse)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|

## Delete an Agent proxy deployment

<a id="opIddeleteAgentProxyDeployment"></a>

`DELETE /agent-proxies/{agentProxyId}/deployments/{deploymentId}`

> Code samples

```shell

curl -X DELETE https://localhost:9243/api/v0.9/agent-proxies/{agentProxyId}/deployments/{deploymentId} \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Accept: application/json'

```

Deletes the deployment record. Permitted only when the deployment is in `UNDEPLOYED`
status; any other state is a 409. Access is validated against the organization in the
JWT token.

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:api:agent:deployment:delete`, `ap:api:agent:deployment:manage`, `ap:api:agent:manage`, `ap:api:manage`, `ap:manage`

</aside>

<h3 id="delete-an-agent-proxy-deployment-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|agentProxyId|path|string|true|**Agent Proxy ID** consisting of the **handle** (unique slug identifier) of the Agent proxy.|
|deploymentId|path|string(uuid)|true|The UUID of the deployment|

#### Detailed descriptions

**agentProxyId**: **Agent Proxy ID** consisting of the **handle** (unique slug identifier) of the Agent proxy.

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

> 409 Response

```json
{
  "status": "error",
  "code": "DEPLOYMENT_ACTIVE",
  "message": "Cannot delete an active deployment - undeploy it first."
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

<h3 id="delete-an-agent-proxy-deployment-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|204|[No Content](https://tools.ietf.org/html/rfc7231#section-6.3.5)|Deployment deleted successfully|None|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Conflict. The deployment is still active and must be undeployed before deletion.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|

## Begin undeployment of an Agent proxy deployment

<a id="opIdundeployAgentProxyDeployment"></a>

`POST /agent-proxies/{agentProxyId}/deployments/{deploymentId}/undeploy`

> Code samples

```shell

curl -X POST https://localhost:9243/api/v0.9/agent-proxies/{agentProxyId}/deployments/{deploymentId}/undeploy?gatewayId=prod-gateway-01 \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Accept: application/json'

```

Accepts an undeployment request and returns the transitional deployment state
(`UNDEPLOYING`). Poll the `Location` URI for the terminal status (`UNDEPLOYED` or
`FAILED`) and `statusReason`; never retry the POST merely because it returned a
transitional state. The deployment artifact remains in the system and can be restored
later.

The `gatewayId` query parameter is validated against the deployment's bound gateway to
prevent unintended operations. Access is validated against the organization in the JWT
token.

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:api:agent:deployment:undeploy`, `ap:api:agent:deployment:manage`, `ap:api:agent:manage`, `ap:api:manage`, `ap:manage`

</aside>

<h3 id="begin-undeployment-of-an-agent-proxy-deployment-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|agentProxyId|path|string|true|**Agent Proxy ID** consisting of the **handle** (unique slug identifier) of the Agent proxy.|
|deploymentId|path|string(uuid)|true|The UUID of the deployment|
|gatewayId|query|string|true|**Gateway ID** consisting of the **handle** (unique slug identifier) of the Gateway|

#### Detailed descriptions

**agentProxyId**: **Agent Proxy ID** consisting of the **handle** (unique slug identifier) of the Agent proxy.

**gatewayId**: **Gateway ID** consisting of the **handle** (unique slug identifier) of the Gateway
bound to the deployment. Required, and validated against the deployment's bound gateway.

> Example responses
>
> Asynchronous operation accepted; poll the deployment until status becomes UNDEPLOYED or FAILED.

```json
{
  "deploymentId": "a73c85a1-d857-491e-a6b2-51dce05de7a2",
  "name": "v1.0-production",
  "gatewayId": "prod-gateway-01",
  "status": "UNDEPLOYING",
  "baseDeploymentId": "be6d8692-b9de-400e-b6c1-14db50154e27",
  "metadata": {},
  "createdAt": "2019-08-24T14:15:22Z",
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

<h3 id="begin-undeployment-of-an-agent-proxy-deployment-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|202|[Accepted](https://tools.ietf.org/html/rfc7231#section-6.3.3)|Undeployment accepted. Returns the deployment with the transitional status UNDEPLOYING; poll the Location for the final status.|[DeploymentResponse](schemas.md#schemadeploymentresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad Request. Invalid request or validation error.|[Error](schemas.md#schemaerror)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Conflict. The request conflicts with the current state of the resource.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|

### Response Headers

|Status|Header|Type|Format|Description|
|---|---|---|---|---|
|202|Location|string|uri|URL of the newly created resource.|

## Begin restoration of a previous Agent proxy deployment

<a id="opIdrestoreAgentProxyDeployment"></a>

`POST /agent-proxies/{agentProxyId}/deployments/{deploymentId}/restore`

> Code samples

```shell

curl -X POST https://localhost:9243/api/v0.9/agent-proxies/{agentProxyId}/deployments/{deploymentId}/restore?gatewayId=prod-gateway-01 \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Accept: application/json'

```

Accepts a restoration request for a previous deployment (`ARCHIVED` or `UNDEPLOYED`)
and returns the transitional deployment state (`DEPLOYING`). Poll the `Location` URI
for the terminal status and `statusReason`. The target deployment must not already be
in `DEPLOYED` status. The gateway acknowledges a restore as a deployment, so the
terminal statuses and reason codes are those described for
`createAgentProxyDeployment`.

The `gatewayId` query parameter is validated against the deployment's bound gateway to
prevent unintended operations. Access is validated against the organization in the JWT
token.

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:api:agent:deployment:restore`, `ap:api:agent:deployment:manage`, `ap:api:agent:manage`, `ap:api:manage`, `ap:manage`

</aside>

<h3 id="begin-restoration-of-a-previous-agent-proxy-deployment-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|agentProxyId|path|string|true|**Agent Proxy ID** consisting of the **handle** (unique slug identifier) of the Agent proxy.|
|deploymentId|path|string(uuid)|true|The UUID of the deployment|
|gatewayId|query|string|true|**Gateway ID** consisting of the **handle** (unique slug identifier) of the Gateway|

#### Detailed descriptions

**agentProxyId**: **Agent Proxy ID** consisting of the **handle** (unique slug identifier) of the Agent proxy.

**gatewayId**: **Gateway ID** consisting of the **handle** (unique slug identifier) of the Gateway
bound to the deployment. Required, and validated against the deployment's bound gateway.

> Example responses
>
> Asynchronous operation accepted; poll the deployment until status becomes DEPLOYED or FAILED.

```json
{
  "deploymentId": "a73c85a1-d857-491e-a6b2-51dce05de7a2",
  "name": "v1.0-production",
  "gatewayId": "prod-gateway-01",
  "status": "DEPLOYING",
  "baseDeploymentId": "be6d8692-b9de-400e-b6c1-14db50154e27",
  "metadata": {},
  "createdAt": "2019-08-24T14:15:22Z",
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

<h3 id="begin-restoration-of-a-previous-agent-proxy-deployment-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|202|[Accepted](https://tools.ietf.org/html/rfc7231#section-6.3.3)|Restoration accepted. Returns the deployment with the transitional status DEPLOYING; poll the Location for the final status.|[DeploymentResponse](schemas.md#schemadeploymentresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad Request. Invalid request or validation error.|[Error](schemas.md#schemaerror)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Conflict. The request conflicts with the current state of the resource.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|

### Response Headers

|Status|Header|Type|Format|Description|
|---|---|---|---|---|
|202|Location|string|uri|URL of the newly created resource.|

## Prepare a build of an Agent proxy

<a id="opIdcreateAgentProxyBuild"></a>

`POST /agent-proxies/{agentProxyId}/builds`

> Code samples

```shell

curl -X POST https://localhost:9243/api/v0.9/agent-proxies/{agentProxyId}/builds \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -d @payload.json

```

Renders the Agent proxy's current definition into an immutable snapshot and stores it,
without deploying it anywhere.

Preparing and deploying are separate steps so that what reaches a gateway is a
snapshot taken at a known moment: a deploy that names a build cannot silently
pick up edits made to the Agent proxy since, and the same build can be deployed to
any number of gateways, and promoted onward, without being re-rendered.

The artifact is stored at the platform's own data version; it is translated to
the target gateway's version when it is deployed.

An Agent proxy keeps at most `deployments.max_builds_per_api` builds. Preparing
another first removes the oldest builds no current deployment is using; if every one
is in use, the request is refused with a `409` and a build has to be deleted to
make room.

Access is validated against the organization in the JWT token.

> Payload

```json
{
  "description": "Adds the /reports endpoint",
  "metadata": {
    "commitId": "9f1c2ab"
  }
}
```

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:api:agent:build:create`, `ap:api:agent:build:manage`, `ap:api:agent:manage`, `ap:api:manage`, `ap:manage`

</aside>

<h3 id="prepare-a-build-of-an-agent-proxy-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|agentProxyId|path|string|true|**Agent Proxy ID** consisting of the **handle** (unique slug identifier) of the Agent proxy.|
|body|body|[BuildRequest](schemas.md#schemabuildrequest)|false|none|

#### Detailed descriptions

**agentProxyId**: **Agent Proxy ID** consisting of the **handle** (unique slug identifier) of the Agent proxy.

> Example responses
>
> 201 Response

```json
{
  "buildId": "2026-01-31-2",
  "uuid": "095be615-a8ad-4c33-8e9c-c7612fbf6c9f",
  "description": "string",
  "dataVersion": "string",
  "metadata": {},
  "createdBy": "string",
  "createdAt": "2019-08-24T14:15:22Z"
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

<h3 id="prepare-a-build-of-an-agent-proxy-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|201|[Created](https://tools.ietf.org/html/rfc7231#section-6.3.2)|Build prepared successfully|[BuildResponse](schemas.md#schemabuildresponse)|
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

## Get builds for an Agent proxy

<a id="opIdlistAgentProxyBuilds"></a>

`GET /agent-proxies/{agentProxyId}/builds`

> Code samples

```shell

curl -X GET https://localhost:9243/api/v0.9/agent-proxies/{agentProxyId}/builds \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Accept: application/json'

```

Lists the Agent proxy's builds, newest first. The rendered artifact itself is not
included; a listing is for choosing which build to deploy.
Access is validated against the organization in the JWT token.

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:api:agent:build:read`, `ap:api:agent:build:manage`, `ap:api:agent:read`, `ap:api:agent:manage`, `ap:api:read`, `ap:api:manage`, `ap:read`, `ap:manage`

</aside>

<h3 id="get-builds-for-an-agent-proxy-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|agentProxyId|path|string|true|**Agent Proxy ID** consisting of the **handle** (unique slug identifier) of the Agent proxy.|
|limit|query|integer|false|Maximum number of items to return per page.|

#### Detailed descriptions

**agentProxyId**: **Agent Proxy ID** consisting of the **handle** (unique slug identifier) of the Agent proxy.

> Example responses
>
> 200 Response

```json
{
  "count": 0,
  "list": [
    {
      "buildId": "2026-01-31-2",
      "uuid": "095be615-a8ad-4c33-8e9c-c7612fbf6c9f",
      "description": "string",
      "dataVersion": "string",
      "metadata": {},
      "createdBy": "string",
      "createdAt": "2019-08-24T14:15:22Z"
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

<h3 id="get-builds-for-an-agent-proxy-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Builds retrieved successfully|[BuildListResponse](schemas.md#schemabuildlistresponse)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Forbidden. The authenticated user does not have permission to access this resource.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|

## Get Agent proxy build by ID

<a id="opIdgetAgentProxyBuild"></a>

`GET /agent-proxies/{agentProxyId}/builds/{buildId}`

> Code samples

```shell

curl -X GET https://localhost:9243/api/v0.9/agent-proxies/{agentProxyId}/builds/{buildId} \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Accept: application/json'

```

Retrieves metadata for a single build.
Access is validated against the organization in the JWT token.

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:api:agent:build:read`, `ap:api:agent:build:manage`, `ap:api:agent:read`, `ap:api:agent:manage`, `ap:api:read`, `ap:api:manage`, `ap:read`, `ap:manage`

</aside>

<h3 id="get-agent-proxy-build-by-id-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|agentProxyId|path|string|true|**Agent Proxy ID** consisting of the **handle** (unique slug identifier) of the Agent proxy.|
|buildId|path|string|true|Identifier of the build|

#### Detailed descriptions

**agentProxyId**: **Agent Proxy ID** consisting of the **handle** (unique slug identifier) of the Agent proxy.

> Example responses
>
> 200 Response

```json
{
  "buildId": "2026-01-31-2",
  "uuid": "095be615-a8ad-4c33-8e9c-c7612fbf6c9f",
  "description": "string",
  "dataVersion": "string",
  "metadata": {},
  "createdBy": "string",
  "createdAt": "2019-08-24T14:15:22Z"
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

<h3 id="get-agent-proxy-build-by-id-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Build metadata retrieved successfully|[BuildResponse](schemas.md#schemabuildresponse)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Forbidden. The authenticated user does not have permission to access this resource.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|

## Delete an Agent proxy build

<a id="opIddeleteAgentProxyBuild"></a>

`DELETE /agent-proxies/{agentProxyId}/builds/{buildId}`

> Code samples

```shell

curl -X DELETE https://localhost:9243/api/v0.9/agent-proxies/{agentProxyId}/builds/{buildId} \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Accept: application/json'

```

Deletes one of the Agent proxy's builds, freeing a slot when the Agent proxy is at its
build limit.

Refused with a conflict while a gateway is serving the build — that is, while
any `DEPLOYED`, `DEPLOYING` or `UNDEPLOYING` deployment runs it. Undeploy it
first.

Undeployed, failed and superseded deployments release the build. They keep the
artifact they were created with, so they can still be redeployed, but they stop
reporting a `buildId` and can no longer be promoted to a later environment.

Access is validated against the organization in the JWT token.

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:api:agent:build:delete`, `ap:api:agent:build:manage`, `ap:api:agent:manage`, `ap:api:manage`, `ap:manage`

</aside>

<h3 id="delete-an-agent-proxy-build-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|agentProxyId|path|string|true|**Agent Proxy ID** consisting of the **handle** (unique slug identifier) of the Agent proxy.|
|buildId|path|string|true|Identifier of the build|

#### Detailed descriptions

**agentProxyId**: **Agent Proxy ID** consisting of the **handle** (unique slug identifier) of the Agent proxy.

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

<h3 id="delete-an-agent-proxy-build-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|204|[No Content](https://tools.ietf.org/html/rfc7231#section-6.3.5)|Build deleted successfully|None|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Forbidden. The authenticated user does not have permission to access this resource.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Conflict. The request conflicts with the current state of the resource.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|
