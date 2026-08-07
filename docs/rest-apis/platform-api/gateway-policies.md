<h1 id="wso2-api-platform-platform-api-gateway-policies">Gateway Policies</h1>

Custom gateway policy management operations

## Get synced custom policies for the current organization

<a id="opIdListGatewayCustomPolicies"></a>

`GET /gateway-custom-policies`

> Code samples

```shell

curl -X GET https://localhost:9243/api/v0.9/gateway-custom-policies \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Accept: application/json'

```

Returns all custom policies synced to the current organization (from the JWT `organization` claim).

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:gateway_custom_policy:read`, `ap:gateway_custom_policy:manage`

</aside>

<h3 id="get-synced-custom-policies-for-the-current-organization-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|limit|query|integer|false|Maximum number of items to return per page.|
|offset|query|integer|false|Zero-based index of the first item to return.|

> Example responses
>
> 200 Response

```json
{
  "count": 0,
  "list": [
    {
      "uuid": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
      "organizationUuid": "bc554ded-7e40-44a7-b397-48480793ad03",
      "name": "rate-limit-custom",
      "version": "1.0.0",
      "description": "Custom rate limiting policy",
      "policyDefinition": {},
      "createdAt": "2019-08-24T14:15:22Z",
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

<h3 id="get-synced-custom-policies-for-the-current-organization-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|List of custom policies|[CustomPolicyListResponse](schemas.md#schemacustompolicylistresponse)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|

## Sync a custom policy from the gateway manifest

<a id="opIdSyncCustomPolicy"></a>

`POST /gateway-custom-policies/sync`

> Code samples

```shell

curl -X POST https://localhost:9243/api/v0.9/gateway-custom-policies/sync?gatewayId=prod-gateway-01&policyName=set-wso2-headers&policyVersion=1.0.0 \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Accept: application/json'

```

Syncs a custom policy from the gateway manifest into the organization's custom policy registry.
Version-based rules apply:
- **New major version**: creates a new policy record (e.g. v1.x.x and v2.x.x coexist).
- **New minor version** (same major): updates the existing record (e.g. v1.1.0 → v1.2.0).
- **Patch version change** (same major.minor): not allowed.
- **Downgrade**: not allowed.
Policy names are case-insensitive.
After syncing, the policy can be applied to APIs in the organization.

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:gateway_custom_policy:create`, `ap:gateway_custom_policy:manage`

</aside>

<h3 id="sync-a-custom-policy-from-the-gateway-manifest-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|gatewayId|query|string|true|Handle (URL-friendly slug) of the gateway whose manifest contains the policy|
|policyName|query|string|true|Name of the custom policy (case-insensitive)|
|policyVersion|query|string|true|Version of the custom policy in MAJOR.MINOR.PATCH format|

> Example responses
>
> 200 Response

```json
{
  "uuid": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
  "organizationUuid": "bc554ded-7e40-44a7-b397-48480793ad03",
  "name": "rate-limit-custom",
  "version": "1.0.0",
  "description": "Custom rate limiting policy",
  "policyDefinition": {},
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

> 422 Response

```json
{
  "status": "error",
  "code": "POLICY_INVALID_STATE",
  "message": "The policy is not a custom policy, or its manifest is unavailable."
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

<h3 id="sync-a-custom-policy-from-the-gateway-manifest-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Custom policy synced successfully|[CustomPolicyResponse](schemas.md#schemacustompolicyresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad Request. Invalid request or validation error.|[Error](schemas.md#schemaerror)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Forbidden. The authenticated user does not have permission to access this resource.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Conflict. The request conflicts with the current state of the resource.|[Error](schemas.md#schemaerror)|
|422|[Unprocessable Entity](https://tools.ietf.org/html/rfc2518#section-10.3)|Policy is not a custom policy or manifest is unavailable|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|

## Get a specific custom policy version

<a id="opIdGetGatewayCustomPolicy"></a>

`GET /gateway-custom-policies/{gatewayCustomPolicyId}/versions/{version}`

> Code samples

```shell

curl -X GET https://localhost:9243/api/v0.9/gateway-custom-policies/{gatewayCustomPolicyId}/versions/{version} \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Accept: application/json'

```

Returns a custom policy by its UUID and version for the current organization.

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:gateway_custom_policy:read`, `ap:gateway_custom_policy:manage`

</aside>

<h3 id="get-a-specific-custom-policy-version-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|gatewayCustomPolicyId|path|string(uuid)|true|UUID of the custom policy record|
|version|path|string|true|Version of the custom policy (e.g. "1.0.0")|

> Example responses
>
> 200 Response

```json
{
  "uuid": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
  "organizationUuid": "bc554ded-7e40-44a7-b397-48480793ad03",
  "name": "rate-limit-custom",
  "version": "1.0.0",
  "description": "Custom rate limiting policy",
  "policyDefinition": {},
  "createdAt": "2019-08-24T14:15:22Z",
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

<h3 id="get-a-specific-custom-policy-version-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Custom policy retrieved successfully|[CustomPolicyResponse](schemas.md#schemacustompolicyresponse)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|

## Delete a specific custom policy version

<a id="opIdDeleteGatewayCustomPolicy"></a>

`DELETE /gateway-custom-policies/{gatewayCustomPolicyId}/versions/{version}`

> Code samples

```shell

curl -X DELETE https://localhost:9243/api/v0.9/gateway-custom-policies/{gatewayCustomPolicyId}/versions/{version} \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Accept: application/json'

```

Deletes a custom policy by its UUID and version. The policy must not be in use by any APIs.

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:gateway_custom_policy:delete`, `ap:gateway_custom_policy:manage`

</aside>

<h3 id="delete-a-specific-custom-policy-version-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|gatewayCustomPolicyId|path|string(uuid)|true|UUID of the custom policy record|
|version|path|string|true|Version of the custom policy (e.g. "1.0.0")|

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

<h3 id="delete-a-specific-custom-policy-version-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|204|[No Content](https://tools.ietf.org/html/rfc7231#section-6.3.5)|Custom policy deleted successfully|None|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Forbidden. The authenticated user does not have permission to access this resource.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Conflict. The request conflicts with the current state of the resource.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|
