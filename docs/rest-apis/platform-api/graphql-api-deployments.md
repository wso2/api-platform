<h1 id="wso2-api-platform-platform-api-graphql-api-deployments">GraphQL API Deployments</h1>

GraphQL API deployment artifact management and lifecycle operations

## Create and deploy a new deployment

<a id="opIdDeployGraphQLAPI"></a>

`POST /graphql-apis/{graphqlApiId}/deployments`

> Code samples

```shell

curl -X POST https://localhost:9243/api/v0.9/graphql-apis/{graphqlApiId}/deployments \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -d @payload.json

```

Creates an immutable deployment artifact for a GraphQL API and deploys it to a specified gateway.
Each deployment targets a single gateway. The graphqlApiId parameter is the API handle (identifier),
not the UUID. The operation returns a transitional DEPLOYING status. Final success or failure will be reported asynchronously via the deployment's status and statusReason once the gateway acknowledges.
Access is validated against the organization in the JWT token.

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

Required scopes (the token must carry at least one of): `ap:api:graphql:deployment:create`, `ap:api:graphql:deployment:manage`, `ap:api:graphql:manage`, `ap:api:manage`, `ap:manage`

</aside>

<h3 id="create-and-deploy-a-new-deployment-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|graphqlApiId|path|string|true|**GraphQL API ID** consisting of the **handle** (unique identifier) of the API.|
|body|body|[DeployRequest](schemas.md#schemadeployrequest)|true|Deployment request with gateway ID, base reference, and metadata|

#### Detailed descriptions

**graphqlApiId**: **GraphQL API ID** consisting of the **handle** (unique identifier) of the API.

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

<h3 id="create-and-deploy-a-new-deployment-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|201|[Created](https://tools.ietf.org/html/rfc7231#section-6.3.2)|GraphQL API deployed successfully|[DeploymentResponse](schemas.md#schemadeploymentresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad Request. Invalid request or validation error.|[Error](schemas.md#schemaerror)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Forbidden. The authenticated user does not have permission to access this resource.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|

### Response Headers

|Status|Header|Type|Format|Description|
|---|---|---|---|---|
|201|Location|string|uri|URL of the newly created resource.|

## Get deployments for a GraphQL API

<a id="opIdGetGraphQLAPIDeployments"></a>

`GET /graphql-apis/{graphqlApiId}/deployments`

> Code samples

```shell

curl -X GET https://localhost:9243/api/v0.9/graphql-apis/{graphqlApiId}/deployments \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Accept: application/json'

```

Retrieves all deployment artifacts for a specific API. The graphqlApiId parameter is the API handle (identifier),
not the UUID. Supports filtering by gateway handle and deployment status.
Access is validated against the organization in the JWT token.

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:api:graphql:deployment:read`, `ap:api:graphql:deployment:manage`, `ap:api:graphql:read`, `ap:api:graphql:manage`, `ap:api:read`, `ap:api:manage`, `ap:read`, `ap:manage`

</aside>

<h3 id="get-deployments-for-a-graphql-api-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|graphqlApiId|path|string|true|**GraphQL API ID** consisting of the **handle** (unique identifier) of the API.|
|gatewayId|query|string|false|**Gateway ID** consisting of the **handle** (unique slug identifier) of the Gateway to filter status by.|
|status|query|string|false|Filter deployments by status (DEPLOYED, UNDEPLOYED, DEPLOYING, UNDEPLOYING, FAILED, or ARCHIVED)|
|limit|query|integer|false|Maximum number of items to return per page.|
|offset|query|integer|false|Zero-based index of the first item to return.|

#### Detailed descriptions

**graphqlApiId**: **GraphQL API ID** consisting of the **handle** (unique identifier) of the API.

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

<h3 id="get-deployments-for-a-graphql-api-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Deployments retrieved successfully|[DeploymentListResponse](schemas.md#schemadeploymentlistresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad Request. Invalid request or validation error.|[Error](schemas.md#schemaerror)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|

## Get deployment by ID

<a id="opIdGetGraphQLAPIDeployment"></a>

`GET /graphql-apis/{graphqlApiId}/deployments/{deploymentId}`

> Code samples

```shell

curl -X GET https://localhost:9243/api/v0.9/graphql-apis/{graphqlApiId}/deployments/{deploymentId} \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Accept: application/json'

```

Retrieves metadata for a specific deployment artifact including status, gateway association,
and timestamps. Access is validated against the organization in the JWT token.

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:api:graphql:deployment:read`, `ap:api:graphql:deployment:manage`, `ap:api:graphql:read`, `ap:api:graphql:manage`, `ap:api:read`, `ap:api:manage`, `ap:read`, `ap:manage`

</aside>

<h3 id="get-deployment-by-id-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|graphqlApiId|path|string|true|**GraphQL API ID** consisting of the **handle** (unique identifier) of the API.|
|deploymentId|path|string(uuid)|true|The UUID of the deployment|

#### Detailed descriptions

**graphqlApiId**: **GraphQL API ID** consisting of the **handle** (unique identifier) of the API.

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

<h3 id="get-deployment-by-id-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Deployment metadata retrieved successfully|[DeploymentResponse](schemas.md#schemadeploymentresponse)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|

## Delete deployment

<a id="opIdDeleteGraphQLAPIDeployment"></a>

`DELETE /graphql-apis/{graphqlApiId}/deployments/{deploymentId}`

> Code samples

```shell

curl -X DELETE https://localhost:9243/api/v0.9/graphql-apis/{graphqlApiId}/deployments/{deploymentId} \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Accept: application/json'

```

Deletes a deployment artifact. Deletion is only allowed when the deployment is in UNDEPLOYED status.
Access is validated against the organization in the JWT token.

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:api:graphql:deployment:delete`, `ap:api:graphql:deployment:manage`, `ap:api:graphql:manage`, `ap:api:manage`, `ap:manage`

</aside>

<h3 id="delete-deployment-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|graphqlApiId|path|string|true|**GraphQL API ID** consisting of the **handle** (unique identifier) of the API.|
|deploymentId|path|string(uuid)|true|The UUID of the deployment|

#### Detailed descriptions

**graphqlApiId**: **GraphQL API ID** consisting of the **handle** (unique identifier) of the API.

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

<h3 id="delete-deployment-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|204|[No Content](https://tools.ietf.org/html/rfc7231#section-6.3.5)|Deployment deleted successfully|None|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad Request. Invalid request or validation error.|[Error](schemas.md#schemaerror)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Forbidden. The authenticated user does not have permission to access this resource.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Conflict. The deployment is still active and must be undeployed before deletion.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|

## Undeploy deployment from gateway

<a id="opIdUndeployGraphQLAPIDeployment"></a>

`POST /graphql-apis/{graphqlApiId}/deployments/{deploymentId}/undeploy`

> Code samples

```shell

curl -X POST https://localhost:9243/api/v0.9/graphql-apis/{graphqlApiId}/deployments/{deploymentId}/undeploy?gatewayId=string \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Accept: application/json'

```

Undeploys an active deployment, stopping the API from being served on the specified gateway.
The deployment artifact remains in the system and can be restored later.
Returns the updated deployment object with initial status UNDEPLOYING. Final status (UNDEPLOYED or FAILED) will be reported asynchronously via the deployment's status and statusReason once the gateway acknowledges.

The gatewayId query parameter is validated against deployment's bound gateway to prevent unintended operations.
Access is validated against the organization in the JWT token.

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:api:graphql:deployment:undeploy`, `ap:api:graphql:deployment:manage`, `ap:api:graphql:manage`, `ap:api:manage`, `ap:manage`

</aside>

<h3 id="undeploy-deployment-from-gateway-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|graphqlApiId|path|string|true|**GraphQL API ID** consisting of the **handle** (unique identifier) of the API.|
|deploymentId|path|string|true|UUID of the deployment to undeploy|
|gatewayId|query|string|true|Handle (URL-friendly slug) of the gateway (validated against deployment's bound gateway)|

#### Detailed descriptions

**graphqlApiId**: **GraphQL API ID** consisting of the **handle** (unique identifier) of the API.

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

<h3 id="undeploy-deployment-from-gateway-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Undeploy initiated successfully. Returns the deployment with initial status UNDEPLOYING. Poll status for final result.|[DeploymentResponse](schemas.md#schemadeploymentresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad Request. Invalid request or validation error.|[Error](schemas.md#schemaerror)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Forbidden. The authenticated user does not have permission to access this resource.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Conflict. The request conflicts with the current state of the resource.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|

## Restore a previous deployment

<a id="opIdRestoreGraphQLAPIDeployment"></a>

`POST /graphql-apis/{graphqlApiId}/deployments/{deploymentId}/restore`

> Code samples

```shell

curl -X POST https://localhost:9243/api/v0.9/graphql-apis/{graphqlApiId}/deployments/{deploymentId}/restore?gatewayId=string \
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

Required scopes (the token must carry at least one of): `ap:api:graphql:deployment:restore`, `ap:api:graphql:deployment:manage`, `ap:api:graphql:manage`, `ap:api:manage`, `ap:manage`

</aside>

<h3 id="restore-a-previous-deployment-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|graphqlApiId|path|string|true|**GraphQL API ID** consisting of the **handle** (unique identifier) of the API.|
|deploymentId|path|string|true|UUID of the deployment to restore (must be ARCHIVED or UNDEPLOYED)|
|gatewayId|query|string|true|Handle (URL-friendly slug) of the gateway (validated against deployment's bound gateway)|

#### Detailed descriptions

**graphqlApiId**: **GraphQL API ID** consisting of the **handle** (unique identifier) of the API.

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

<h3 id="restore-a-previous-deployment-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Restore initiated successfully. Returns the deployment with initial status DEPLOYING. Poll status for final result.|[DeploymentResponse](schemas.md#schemadeploymentresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad Request. Invalid request or validation error.|[Error](schemas.md#schemaerror)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Forbidden. The authenticated user does not have permission to access this resource.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Conflict. The request conflicts with the current state of the resource.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|

## Prepare a build of a GraphQL API

<a id="opIdCreateGraphQLAPIBuild"></a>

`POST /graphql-apis/{graphqlApiId}/builds`

> Code samples

```shell

curl -X POST https://localhost:9243/api/v0.9/graphql-apis/{graphqlApiId}/builds \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -d @payload.json

```

Renders the GraphQL API's current definition into an immutable snapshot and stores it,
without deploying it anywhere.

Preparing and deploying are separate steps so that what reaches a gateway is a
snapshot taken at a known moment: a deploy that names a build cannot silently
pick up edits made to the API since, and the same build can be deployed to any
number of gateways, and promoted onward, without being re-rendered.

The artifact is stored at the platform's own data version; it is translated to
the target gateway's version when it is deployed.

A GraphQL API keeps at most `deployments.max_builds_per_api` builds. Preparing another
first removes the oldest builds no current deployment is using; if every one is
in use, the request is refused with a `409` and a build has to be deleted to
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

Required scopes (the token must carry at least one of): `ap:api:graphql:build:create`, `ap:api:graphql:build:manage`, `ap:api:graphql:manage`, `ap:api:manage`, `ap:manage`

</aside>

<h3 id="prepare-a-build-of-a-graphql-api-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|graphqlApiId|path|string|true|Identifier of the GraphQL API|
|body|body|[BuildRequest](schemas.md#schemabuildrequest)|false|none|

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

<h3 id="prepare-a-build-of-a-graphql-api-responses">Responses</h3>

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

## Get builds for a GraphQL API

<a id="opIdGetGraphQLAPIBuilds"></a>

`GET /graphql-apis/{graphqlApiId}/builds`

> Code samples

```shell

curl -X GET https://localhost:9243/api/v0.9/graphql-apis/{graphqlApiId}/builds \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Accept: application/json'

```

Lists the GraphQL API's builds, newest first. The rendered artifact itself is not
included; a listing is for choosing which build to deploy.
Access is validated against the organization in the JWT token.

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:api:graphql:build:read`, `ap:api:graphql:build:manage`, `ap:api:graphql:read`, `ap:api:graphql:manage`, `ap:api:read`, `ap:api:manage`, `ap:read`, `ap:manage`

</aside>

<h3 id="get-builds-for-a-graphql-api-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|graphqlApiId|path|string|true|Identifier of the GraphQL API|
|limit|query|integer|false|Maximum number of items to return per page.|

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

<h3 id="get-builds-for-a-graphql-api-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Builds retrieved successfully|[BuildListResponse](schemas.md#schemabuildlistresponse)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Forbidden. The authenticated user does not have permission to access this resource.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|

## Get build by ID

<a id="opIdGetGraphQLAPIBuild"></a>

`GET /graphql-apis/{graphqlApiId}/builds/{buildId}`

> Code samples

```shell

curl -X GET https://localhost:9243/api/v0.9/graphql-apis/{graphqlApiId}/builds/{buildId} \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Accept: application/json'

```

Retrieves metadata for a single build.
Access is validated against the organization in the JWT token.

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:api:graphql:build:read`, `ap:api:graphql:build:manage`, `ap:api:graphql:read`, `ap:api:graphql:manage`, `ap:api:read`, `ap:api:manage`, `ap:read`, `ap:manage`

</aside>

<h3 id="get-build-by-id-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|graphqlApiId|path|string|true|Identifier of the GraphQL API|
|buildId|path|string|true|Identifier of the build|

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

<h3 id="get-build-by-id-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Build metadata retrieved successfully|[BuildResponse](schemas.md#schemabuildresponse)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Forbidden. The authenticated user does not have permission to access this resource.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|

## Delete a build

<a id="opIdDeleteGraphQLAPIBuild"></a>

`DELETE /graphql-apis/{graphqlApiId}/builds/{buildId}`

> Code samples

```shell

curl -X DELETE https://localhost:9243/api/v0.9/graphql-apis/{graphqlApiId}/builds/{buildId} \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Accept: application/json'

```

Deletes one of the GraphQL API's builds, freeing a slot when the API is at its build
limit.

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

Required scopes (the token must carry at least one of): `ap:api:graphql:build:delete`, `ap:api:graphql:build:manage`, `ap:api:graphql:manage`, `ap:api:manage`, `ap:manage`

</aside>

<h3 id="delete-a-build-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|graphqlApiId|path|string|true|Identifier of the GraphQL API|
|buildId|path|string|true|Identifier of the build|

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

<h3 id="delete-a-build-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|204|[No Content](https://tools.ietf.org/html/rfc7231#section-6.3.5)|Build deleted successfully|None|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Forbidden. The authenticated user does not have permission to access this resource.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Conflict. The request conflicts with the current state of the resource.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|
