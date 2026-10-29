<h1 id="wso2-api-platform-platform-api-api-publications">API Publications</h1>

Publishing, unpublishing and deprecating an API on an API Portal, and the per-portal draft and live listing that feed those actions

## List API publications across API Portals

<a id="opIdlistApiPublications"></a>

`GET /api-publications`

> Code samples

```shell

curl -X GET https://localhost:9243/api/v0.9/api-publications?apiType=rest-api&apiId=my-api-handle \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Accept: application/json'

```

Retrieves every API Portal whose registration has completed — id, name, description and URL — each annotated with this API's publication status and whether a draft exists. Portals still being provisioned, or whose provisioning failed, are not listed.

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:publication:read`, `ap:read`, `ap:manage`

</aside>

<h3 id="list-api-publications-across-api-portals-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|apiType|query|string|true|The API's type, required alongside apiId because a handle is unique only within its own type. Known values: rest-api, graphql-api, websub-api, webbroker-api. Values are resolved at runtime, so a type contributed by a plugin is accepted only on a build that includes it. An unrecognised value returns 404.|
|apiId|query|string|true|The API's handle, unique per organization within its own type.|
|limit|query|integer|false|Maximum number of items to return per page.|
|offset|query|integer|false|Zero-based index of the first item to return.|
|sortBy|query|string|false|Field to sort the collection by. An unrecognized value falls back to the default sort (createdAt).|
|sortOrder|query|string|false|Sort direction applied to `sortBy`.|
|query|query|string|false|Case-insensitive substring filter matched against the resource display name and id (handle).|

#### Enumerated Values

|Parameter|Value|
|---|---|
|sortBy|name|
|sortBy|createdAt|
|sortOrder|asc|
|sortOrder|desc|

> Example responses
>
> 200 Response

```json
{
  "list": [
    {
      "apiPortalId": "string",
      "apiPortalName": "string",
      "apiPortalDescription": "string",
      "apiPortalUrl": "string",
      "status": "NOT_PUBLISHED",
      "draftUpdatedAt": "2019-08-24T14:15:22Z",
      "publicationUpdatedAt": "2019-08-24T14:15:22Z"
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

<h3 id="list-api-publications-across-api-portals-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|OK|[PublicationSummaryResponse](schemas.md#schemapublicationsummaryresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad Request. Invalid request or validation error.|[Error](schemas.md#schemaerror)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Forbidden. The authenticated user does not have permission to access this resource.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|

## Get API publication

<a id="opIdgetApiPublication"></a>

`GET /api-portals/{apiPortalId}/apis/{apiType}/{apiId}/publication`

> Code samples

```shell

curl -X GET https://localhost:9243/api/v0.9/api-portals/{apiPortalId}/apis/{apiType}/{apiId}/publication \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Accept: application/json'

```

Retrieves the live listing's details, subscription plans and documents. 404 when the API is not published to this portal.

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:portal:publication:read`, `ap:portal:read`, `ap:portal:manage`, `ap:read`, `ap:manage`

</aside>

<h3 id="get-api-publication-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|apiPortalId|path|string|true|**API Portal ID** consisting of the **handle** (unique slug identifier) of the API Portal.|
|apiType|path|string|true|The API's type, required alongside apiId because a handle is unique only within its own type. Known values: rest-api, graphql-api, websub-api, webbroker-api. Values are resolved at runtime, so a type contributed by a plugin is accepted only on a build that includes it. An unrecognised value returns 404.|
|apiId|path|string|true|The API's handle, unique per organization within its own type.|

#### Detailed descriptions

**apiPortalId**: **API Portal ID** consisting of the **handle** (unique slug identifier) of the API Portal.

> Example responses
>
> 200 Response

```json
{
  "displayName": "string",
  "version": "string",
  "description": "string",
  "tags": [
    "string"
  ],
  "labels": [
    "string"
  ],
  "agentVisibility": "VISIBLE",
  "endpoints": {
    "productionUrl": "http://example.com",
    "sandboxUrl": "http://example.com"
  },
  "owners": {
    "businessOwner": "string",
    "businessOwnerEmail": "user@example.com",
    "technicalOwner": "string",
    "technicalOwnerEmail": "user@example.com"
  },
  "createdAt": "2019-08-24T14:15:22Z",
  "createdBy": "string",
  "updatedAt": "2019-08-24T14:15:22Z",
  "updatedBy": "string",
  "apiPortalId": "string",
  "apiPortalName": "string",
  "status": "PUBLISHED",
  "subscriptionPlanIds": [
    "string"
  ],
  "docIds": [
    "string"
  ],
  "hasThumbnail": true,
  "hasLandingPage": true
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

<h3 id="get-api-publication-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|OK|[Publication](schemas.md#schemapublication)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Forbidden. The authenticated user does not have permission to access this resource.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|

## Get published definition

<a id="opIdgetApiPublicationDefinition"></a>

`GET /api-portals/{apiPortalId}/apis/{apiType}/{apiId}/publication/definition`

> Code samples

```shell

curl -X GET https://localhost:9243/api/v0.9/api-portals/{apiPortalId}/apis/{apiType}/{apiId}/publication/definition \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Accept: application/json'

```

Retrieves the definition stored in this publication. 404 when the publication has none.

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:portal:publication:read`, `ap:portal:read`, `ap:portal:manage`, `ap:read`, `ap:manage`

</aside>

<h3 id="get-published-definition-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|apiPortalId|path|string|true|**API Portal ID** consisting of the **handle** (unique slug identifier) of the API Portal.|
|apiType|path|string|true|The API's type, required alongside apiId because a handle is unique only within its own type. Known values: rest-api, graphql-api, websub-api, webbroker-api. Values are resolved at runtime, so a type contributed by a plugin is accepted only on a build that includes it. An unrecognised value returns 404.|
|apiId|path|string|true|The API's handle, unique per organization within its own type.|

#### Detailed descriptions

**apiPortalId**: **API Portal ID** consisting of the **handle** (unique slug identifier) of the API Portal.

> Example responses
>
> 200 Response

```json
"string"
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

<h3 id="get-published-definition-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|The definition's raw content. Content-Type matches the serialization it was stored in: application/json or application/yaml for an OpenAPI or AsyncAPI contract, application/graphql for GraphQL SDL, application/xml for WSDL.|string|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Forbidden. The authenticated user does not have permission to access this resource.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|

## Get published landing page

<a id="opIdgetApiPublicationLandingPage"></a>

`GET /api-portals/{apiPortalId}/apis/{apiType}/{apiId}/publication/landing-page`

> Code samples

```shell

curl -X GET https://localhost:9243/api/v0.9/api-portals/{apiPortalId}/apis/{apiType}/{apiId}/publication/landing-page \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Accept: text/markdown'

```

Retrieves the landing page stored in this publication, as Markdown. 404 when the publication has none.

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:portal:publication:read`, `ap:portal:read`, `ap:portal:manage`, `ap:read`, `ap:manage`

</aside>

<h3 id="get-published-landing-page-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|apiPortalId|path|string|true|**API Portal ID** consisting of the **handle** (unique slug identifier) of the API Portal.|
|apiType|path|string|true|The API's type, required alongside apiId because a handle is unique only within its own type. Known values: rest-api, graphql-api, websub-api, webbroker-api. Values are resolved at runtime, so a type contributed by a plugin is accepted only on a build that includes it. An unrecognised value returns 404.|
|apiId|path|string|true|The API's handle, unique per organization within its own type.|

#### Detailed descriptions

**apiPortalId**: **API Portal ID** consisting of the **handle** (unique slug identifier) of the API Portal.

> Example responses
>
> 200 Response

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

<h3 id="get-published-landing-page-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|The landing page's raw Markdown content.|string|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Forbidden. The authenticated user does not have permission to access this resource.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|

## Get published thumbnail

<a id="opIdgetApiPublicationThumbnail"></a>

`GET /api-portals/{apiPortalId}/apis/{apiType}/{apiId}/publication/thumbnail`

> Code samples

```shell

curl -X GET https://localhost:9243/api/v0.9/api-portals/{apiPortalId}/apis/{apiType}/{apiId}/publication/thumbnail \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Accept: image/png'

```

Retrieves the thumbnail stored in this publication, as raw image bytes. 404 when the publication has none.

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:portal:publication:read`, `ap:portal:read`, `ap:portal:manage`, `ap:read`, `ap:manage`

</aside>

<h3 id="get-published-thumbnail-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|apiPortalId|path|string|true|**API Portal ID** consisting of the **handle** (unique slug identifier) of the API Portal.|
|apiType|path|string|true|The API's type, required alongside apiId because a handle is unique only within its own type. Known values: rest-api, graphql-api, websub-api, webbroker-api. Values are resolved at runtime, so a type contributed by a plugin is accepted only on a build that includes it. An unrecognised value returns 404.|
|apiId|path|string|true|The API's handle, unique per organization within its own type.|

#### Detailed descriptions

**apiPortalId**: **API Portal ID** consisting of the **handle** (unique slug identifier) of the API Portal.

> Example responses
>
> 200 Response

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

<h3 id="get-published-thumbnail-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|The thumbnail's raw image bytes. Content-Type is the stored image's own type — only the two accepted on upload can occur.|string|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Forbidden. The authenticated user does not have permission to access this resource.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|

## Get publication draft

<a id="opIdgetApiPublicationDraft"></a>

`GET /api-portals/{apiPortalId}/apis/{apiType}/{apiId}/draft`

> Code samples

```shell

curl -X GET https://localhost:9243/api/v0.9/api-portals/{apiPortalId}/apis/{apiType}/{apiId}/draft \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Accept: application/json'

```

Retrieves the draft's details, subscription plans and documents. 404 when no draft has been saved.

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:portal:draft:read`, `ap:portal:draft:manage`, `ap:portal:read`, `ap:portal:manage`, `ap:read`, `ap:manage`

</aside>

<h3 id="get-publication-draft-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|apiPortalId|path|string|true|**API Portal ID** consisting of the **handle** (unique slug identifier) of the API Portal.|
|apiType|path|string|true|The API's type, required alongside apiId because a handle is unique only within its own type. Known values: rest-api, graphql-api, websub-api, webbroker-api. Values are resolved at runtime, so a type contributed by a plugin is accepted only on a build that includes it. An unrecognised value returns 404.|
|apiId|path|string|true|The API's handle, unique per organization within its own type.|

#### Detailed descriptions

**apiPortalId**: **API Portal ID** consisting of the **handle** (unique slug identifier) of the API Portal.

> Example responses
>
> 200 Response

```json
{
  "displayName": "string",
  "version": "string",
  "description": "string",
  "tags": [
    "string"
  ],
  "labels": [
    "string"
  ],
  "agentVisibility": "VISIBLE",
  "endpoints": {
    "productionUrl": "http://example.com",
    "sandboxUrl": "http://example.com"
  },
  "owners": {
    "businessOwner": "string",
    "businessOwnerEmail": "user@example.com",
    "technicalOwner": "string",
    "technicalOwnerEmail": "user@example.com"
  },
  "subscriptionPlanIds": [
    "string"
  ],
  "docIds": [
    "string"
  ],
  "createdAt": "2019-08-24T14:15:22Z",
  "createdBy": "string",
  "updatedAt": "2019-08-24T14:15:22Z",
  "updatedBy": "string",
  "hasThumbnail": true,
  "hasLandingPage": true
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

<h3 id="get-publication-draft-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|OK|[PublicationDraftDetails](schemas.md#schemapublicationdraftdetails)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Forbidden. The authenticated user does not have permission to access this resource.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|

## Save publication draft

<a id="opIdsaveApiPublicationDraft"></a>

`PUT /api-portals/{apiPortalId}/apis/{apiType}/{apiId}/draft`

> Code samples

```shell

curl -X PUT https://localhost:9243/api/v0.9/api-portals/{apiPortalId}/apis/{apiType}/{apiId}/draft \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -d @payload.json

```

Replaces the draft's details, subscription plans and documents, creating the draft on first save. Every plan handle and document handle must exist in the organization.

> Payload

```json
{
  "displayName": "string",
  "version": "string",
  "description": "string",
  "tags": [
    "string"
  ],
  "labels": [
    "string"
  ],
  "agentVisibility": "VISIBLE",
  "endpoints": {
    "productionUrl": "http://example.com",
    "sandboxUrl": "http://example.com"
  },
  "owners": {
    "businessOwner": "string",
    "businessOwnerEmail": "user@example.com",
    "technicalOwner": "string",
    "technicalOwnerEmail": "user@example.com"
  },
  "subscriptionPlanIds": [
    "string"
  ],
  "docIds": [
    "string"
  ]
}
```

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:portal:draft:update`, `ap:portal:draft:manage`, `ap:portal:manage`, `ap:manage`

</aside>

<h3 id="save-publication-draft-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|[PublicationDraftDetailsInput](schemas.md#schemapublicationdraftdetailsinput)|true|none|
|apiPortalId|path|string|true|**API Portal ID** consisting of the **handle** (unique slug identifier) of the API Portal.|
|apiType|path|string|true|The API's type, required alongside apiId because a handle is unique only within its own type. Known values: rest-api, graphql-api, websub-api, webbroker-api. Values are resolved at runtime, so a type contributed by a plugin is accepted only on a build that includes it. An unrecognised value returns 404.|
|apiId|path|string|true|The API's handle, unique per organization within its own type.|

#### Detailed descriptions

**apiPortalId**: **API Portal ID** consisting of the **handle** (unique slug identifier) of the API Portal.

> Example responses
>
> 200 Response

```json
{
  "displayName": "string",
  "version": "string",
  "description": "string",
  "tags": [
    "string"
  ],
  "labels": [
    "string"
  ],
  "agentVisibility": "VISIBLE",
  "endpoints": {
    "productionUrl": "http://example.com",
    "sandboxUrl": "http://example.com"
  },
  "owners": {
    "businessOwner": "string",
    "businessOwnerEmail": "user@example.com",
    "technicalOwner": "string",
    "technicalOwnerEmail": "user@example.com"
  },
  "subscriptionPlanIds": [
    "string"
  ],
  "docIds": [
    "string"
  ],
  "createdAt": "2019-08-24T14:15:22Z",
  "createdBy": "string",
  "updatedAt": "2019-08-24T14:15:22Z",
  "updatedBy": "string",
  "hasThumbnail": true,
  "hasLandingPage": true
}
```

> 400 Response

```json
{
  "status": "error",
  "code": "PUBLICATION_VALIDATION_FAILED",
  "message": "The request could not be validated.",
  "errors": [
    {
      "field": "subscriptionPlanIds[0]",
      "message": "does not exist in the organization's subscription plan catalog"
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

> 413 Response

```json
{
  "status": "error",
  "code": "PAYLOAD_TOO_LARGE",
  "message": "The uploaded file exceeds the maximum allowed size."
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

<h3 id="save-publication-draft-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Draft saved|[PublicationDraftDetails](schemas.md#schemapublicationdraftdetails)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|The payload failed validation — a plan handle or document handle absent from the organization, a plan that is not active, or content over the configured size ceiling.|[Error](schemas.md#schemaerror)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Forbidden. The authenticated user does not have permission to access this resource.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|413|[Payload Too Large](https://tools.ietf.org/html/rfc7231#section-6.5.11)|Payload Too Large. The uploaded file exceeds the maximum allowed size.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|

## Get draft definition

<a id="opIdgetApiPublicationDraftDefinition"></a>

`GET /api-portals/{apiPortalId}/apis/{apiType}/{apiId}/draft/definition`

> Code samples

```shell

curl -X GET https://localhost:9243/api/v0.9/api-portals/{apiPortalId}/apis/{apiType}/{apiId}/draft/definition \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Accept: application/json'

```

Retrieves the definition stored in this draft. 404 when the draft has not stored one.

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:portal:draft:read`, `ap:portal:draft:manage`, `ap:portal:read`, `ap:portal:manage`, `ap:read`, `ap:manage`

</aside>

<h3 id="get-draft-definition-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|apiPortalId|path|string|true|**API Portal ID** consisting of the **handle** (unique slug identifier) of the API Portal.|
|apiType|path|string|true|The API's type, required alongside apiId because a handle is unique only within its own type. Known values: rest-api, graphql-api, websub-api, webbroker-api. Values are resolved at runtime, so a type contributed by a plugin is accepted only on a build that includes it. An unrecognised value returns 404.|
|apiId|path|string|true|The API's handle, unique per organization within its own type.|

#### Detailed descriptions

**apiPortalId**: **API Portal ID** consisting of the **handle** (unique slug identifier) of the API Portal.

> Example responses
>
> 200 Response

```json
"string"
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

<h3 id="get-draft-definition-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|The definition's raw content. Content-Type matches the serialization it was stored in: application/json or application/yaml for an OpenAPI or AsyncAPI contract, application/graphql for GraphQL SDL, application/xml for WSDL.|string|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Forbidden. The authenticated user does not have permission to access this resource.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|

## Save draft definition

<a id="opIdsaveApiPublicationDraftDefinition"></a>

`PUT /api-portals/{apiPortalId}/apis/{apiType}/{apiId}/draft/definition`

> Code samples

```shell

curl -X PUT https://localhost:9243/api/v0.9/api-portals/{apiPortalId}/apis/{apiType}/{apiId}/draft/definition \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -d @payload.json

```

Replaces the draft definition. Requires the draft to exist.

> Payload

```json
"string"
```

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:portal:draft:update`, `ap:portal:draft:manage`, `ap:portal:manage`, `ap:manage`

</aside>

<h3 id="save-draft-definition-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|string(binary)|true|The definition as raw bytes. The request's Content-Type selects the stored serialization; no other media type is accepted. Saving a draft never validates the content itself — including whether the Content-Type matches what the API's type actually uses — only publish does.|
|apiPortalId|path|string|true|**API Portal ID** consisting of the **handle** (unique slug identifier) of the API Portal.|
|apiType|path|string|true|The API's type, required alongside apiId because a handle is unique only within its own type. Known values: rest-api, graphql-api, websub-api, webbroker-api. Values are resolved at runtime, so a type contributed by a plugin is accepted only on a build that includes it. An unrecognised value returns 404.|
|apiId|path|string|true|The API's handle, unique per organization within its own type.|

#### Detailed descriptions

**apiPortalId**: **API Portal ID** consisting of the **handle** (unique slug identifier) of the API Portal.

> Example responses
>
> 400 Response

```json
{
  "status": "error",
  "code": "PUBLICATION_VALIDATION_FAILED",
  "message": "The request could not be validated.",
  "errors": [
    {
      "field": "subscriptionPlanIds[0]",
      "message": "does not exist in the organization's subscription plan catalog"
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

> 413 Response

```json
{
  "status": "error",
  "code": "PAYLOAD_TOO_LARGE",
  "message": "The uploaded file exceeds the maximum allowed size."
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

<h3 id="save-draft-definition-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|204|[No Content](https://tools.ietf.org/html/rfc7231#section-6.3.5)|Definition saved|None|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|The payload failed validation — a plan handle or document handle absent from the organization, a plan that is not active, or content over the configured size ceiling.|[Error](schemas.md#schemaerror)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Forbidden. The authenticated user does not have permission to access this resource.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|413|[Payload Too Large](https://tools.ietf.org/html/rfc7231#section-6.5.11)|Payload Too Large. The uploaded file exceeds the maximum allowed size.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|

## Get draft landing page

<a id="opIdgetApiPublicationDraftLandingPage"></a>

`GET /api-portals/{apiPortalId}/apis/{apiType}/{apiId}/draft/landing-page`

> Code samples

```shell

curl -X GET https://localhost:9243/api/v0.9/api-portals/{apiPortalId}/apis/{apiType}/{apiId}/draft/landing-page \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Accept: text/markdown'

```

Retrieves the landing page stored in this draft, as Markdown. 404 when the draft has not stored one.

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:portal:draft:read`, `ap:portal:draft:manage`, `ap:portal:read`, `ap:portal:manage`, `ap:read`, `ap:manage`

</aside>

<h3 id="get-draft-landing-page-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|apiPortalId|path|string|true|**API Portal ID** consisting of the **handle** (unique slug identifier) of the API Portal.|
|apiType|path|string|true|The API's type, required alongside apiId because a handle is unique only within its own type. Known values: rest-api, graphql-api, websub-api, webbroker-api. Values are resolved at runtime, so a type contributed by a plugin is accepted only on a build that includes it. An unrecognised value returns 404.|
|apiId|path|string|true|The API's handle, unique per organization within its own type.|

#### Detailed descriptions

**apiPortalId**: **API Portal ID** consisting of the **handle** (unique slug identifier) of the API Portal.

> Example responses
>
> 200 Response

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

<h3 id="get-draft-landing-page-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|The landing page's raw Markdown content.|string|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Forbidden. The authenticated user does not have permission to access this resource.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|

## Save draft landing page

<a id="opIdsaveApiPublicationDraftLandingPage"></a>

`PUT /api-portals/{apiPortalId}/apis/{apiType}/{apiId}/draft/landing-page`

> Code samples

```shell

curl -X PUT https://localhost:9243/api/v0.9/api-portals/{apiPortalId}/apis/{apiType}/{apiId}/draft/landing-page \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Content-Type: text/markdown' \
  -H 'Accept: application/json' \
  -d @payload.txt

```

Replaces the draft landing page. Content is Markdown; embedded raw HTML is stripped before storage. Requires the draft to exist.

> Payload

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:portal:draft:update`, `ap:portal:draft:manage`, `ap:portal:manage`, `ap:manage`

</aside>

<h3 id="save-draft-landing-page-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|string(binary)|true|The landing page as raw Markdown.|
|apiPortalId|path|string|true|**API Portal ID** consisting of the **handle** (unique slug identifier) of the API Portal.|
|apiType|path|string|true|The API's type, required alongside apiId because a handle is unique only within its own type. Known values: rest-api, graphql-api, websub-api, webbroker-api. Values are resolved at runtime, so a type contributed by a plugin is accepted only on a build that includes it. An unrecognised value returns 404.|
|apiId|path|string|true|The API's handle, unique per organization within its own type.|

#### Detailed descriptions

**apiPortalId**: **API Portal ID** consisting of the **handle** (unique slug identifier) of the API Portal.

> Example responses
>
> 400 Response

```json
{
  "status": "error",
  "code": "PUBLICATION_VALIDATION_FAILED",
  "message": "The request could not be validated.",
  "errors": [
    {
      "field": "subscriptionPlanIds[0]",
      "message": "does not exist in the organization's subscription plan catalog"
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

> 413 Response

```json
{
  "status": "error",
  "code": "PAYLOAD_TOO_LARGE",
  "message": "The uploaded file exceeds the maximum allowed size."
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

<h3 id="save-draft-landing-page-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|204|[No Content](https://tools.ietf.org/html/rfc7231#section-6.3.5)|Landing page saved|None|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|The payload failed validation — a plan handle or document handle absent from the organization, a plan that is not active, or content over the configured size ceiling.|[Error](schemas.md#schemaerror)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Forbidden. The authenticated user does not have permission to access this resource.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|413|[Payload Too Large](https://tools.ietf.org/html/rfc7231#section-6.5.11)|Payload Too Large. The uploaded file exceeds the maximum allowed size.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|

## Get draft thumbnail

<a id="opIdgetApiPublicationDraftThumbnail"></a>

`GET /api-portals/{apiPortalId}/apis/{apiType}/{apiId}/draft/thumbnail`

> Code samples

```shell

curl -X GET https://localhost:9243/api/v0.9/api-portals/{apiPortalId}/apis/{apiType}/{apiId}/draft/thumbnail \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Accept: image/png'

```

Retrieves the thumbnail stored in this draft, as raw image bytes. 404 when the draft has not stored one.

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:portal:draft:read`, `ap:portal:draft:manage`, `ap:portal:read`, `ap:portal:manage`, `ap:read`, `ap:manage`

</aside>

<h3 id="get-draft-thumbnail-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|apiPortalId|path|string|true|**API Portal ID** consisting of the **handle** (unique slug identifier) of the API Portal.|
|apiType|path|string|true|The API's type, required alongside apiId because a handle is unique only within its own type. Known values: rest-api, graphql-api, websub-api, webbroker-api. Values are resolved at runtime, so a type contributed by a plugin is accepted only on a build that includes it. An unrecognised value returns 404.|
|apiId|path|string|true|The API's handle, unique per organization within its own type.|

#### Detailed descriptions

**apiPortalId**: **API Portal ID** consisting of the **handle** (unique slug identifier) of the API Portal.

> Example responses
>
> 200 Response

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

<h3 id="get-draft-thumbnail-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|The thumbnail's raw image bytes. Content-Type is the stored image's own type — only the two accepted on upload can occur.|string|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Forbidden. The authenticated user does not have permission to access this resource.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|

## Upload draft thumbnail

<a id="opIdsaveApiPublicationDraftThumbnail"></a>

`PUT /api-portals/{apiPortalId}/apis/{apiType}/{apiId}/draft/thumbnail`

> Code samples

```shell

curl -X PUT https://localhost:9243/api/v0.9/api-portals/{apiPortalId}/apis/{apiType}/{apiId}/draft/thumbnail \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Content-Type: multipart/form-data' \
  -H 'Accept: application/json' \
  -F 'file=string'

```

Uploads or replaces the draft thumbnail. Accepts PNG and JPEG, determined by sniffing the uploaded bytes rather than the declared type or file name. Requires the draft to exist.

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:portal:draft:update`, `ap:portal:draft:manage`, `ap:portal:manage`, `ap:manage`

</aside>

<h3 id="upload-draft-thumbnail-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|object|true|The thumbnail image. Accepted content is sniffed from the uploaded bytes (PNG or JPEG only) rather than trusted from the declared type or file name.|
|» file|body|string(binary)|true|none|
|apiPortalId|path|string|true|**API Portal ID** consisting of the **handle** (unique slug identifier) of the API Portal.|
|apiType|path|string|true|The API's type, required alongside apiId because a handle is unique only within its own type. Known values: rest-api, graphql-api, websub-api, webbroker-api. Values are resolved at runtime, so a type contributed by a plugin is accepted only on a build that includes it. An unrecognised value returns 404.|
|apiId|path|string|true|The API's handle, unique per organization within its own type.|

#### Detailed descriptions

**apiPortalId**: **API Portal ID** consisting of the **handle** (unique slug identifier) of the API Portal.

> Example responses
>
> 400 Response

```json
{
  "status": "error",
  "code": "PUBLICATION_VALIDATION_FAILED",
  "message": "The request could not be validated.",
  "errors": [
    {
      "field": "subscriptionPlanIds[0]",
      "message": "does not exist in the organization's subscription plan catalog"
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

> 413 Response

```json
{
  "status": "error",
  "code": "PAYLOAD_TOO_LARGE",
  "message": "The uploaded file exceeds the maximum allowed size."
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

<h3 id="upload-draft-thumbnail-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|204|[No Content](https://tools.ietf.org/html/rfc7231#section-6.3.5)|Thumbnail saved|None|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|The payload failed validation — a plan handle or document handle absent from the organization, a plan that is not active, or content over the configured size ceiling.|[Error](schemas.md#schemaerror)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Forbidden. The authenticated user does not have permission to access this resource.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|413|[Payload Too Large](https://tools.ietf.org/html/rfc7231#section-6.5.11)|Payload Too Large. The uploaded file exceeds the maximum allowed size.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|

## Publish REST API to API Portal

<a id="opIdpublishRestApiToApiPortal"></a>

`POST /api-portals/{apiPortalId}/apis/rest-api/{apiId}/publish`

> Code samples

```shell

curl -X POST https://localhost:9243/api/v0.9/api-portals/{apiPortalId}/apis/rest-api/{apiId}/publish \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Accept: application/json'

```

Publishes the current draft. Takes no request body: details, plans, documents, definition,
landing page and thumbnail all come from the draft alone — publish never reads the API's
own content or the current publication to fill a gap. Requires a draft to exist.

The content is pushed to the API Portal before anything is written locally, so a failed
call leaves the draft intact and can simply be repeated.

Returns 201 on a first publish or a re-publish after unpublish, 200 otherwise.

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:portal:rest_api:publish`, `ap:portal:rest_api:manage`, `ap:portal:manage`, `ap:manage`

</aside>

<h3 id="publish-rest-api-to-api-portal-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|apiPortalId|path|string|true|**API Portal ID** consisting of the **handle** (unique slug identifier) of the API Portal.|
|apiId|path|string|true|The API's handle, unique per organization within its own type.|

#### Detailed descriptions

**apiPortalId**: **API Portal ID** consisting of the **handle** (unique slug identifier) of the API Portal.

> Example responses
>
> 200 Response

```json
{
  "displayName": "string",
  "version": "string",
  "description": "string",
  "tags": [
    "string"
  ],
  "labels": [
    "string"
  ],
  "agentVisibility": "VISIBLE",
  "endpoints": {
    "productionUrl": "http://example.com",
    "sandboxUrl": "http://example.com"
  },
  "owners": {
    "businessOwner": "string",
    "businessOwnerEmail": "user@example.com",
    "technicalOwner": "string",
    "technicalOwnerEmail": "user@example.com"
  },
  "createdAt": "2019-08-24T14:15:22Z",
  "createdBy": "string",
  "updatedAt": "2019-08-24T14:15:22Z",
  "updatedBy": "string",
  "apiPortalId": "string",
  "apiPortalName": "string",
  "status": "PUBLISHED",
  "subscriptionPlanIds": [
    "string"
  ],
  "docIds": [
    "string"
  ],
  "hasThumbnail": true,
  "hasLandingPage": true
}
```

> Either this API type has no projection onto the API Portal's own API types, so no listing can be created for it, the draft's stored definition is not a valid OpenAPI 3.x document — publish is the one point definition validity is enforced; the draft itself may hold anything — or a selected subscription plan is no longer active.

```json
{
  "status": "error",
  "code": "PUBLICATION_TYPE_UNSUPPORTED",
  "message": "APIs of this type cannot be published to an API Portal."
}
```

```json
{
  "status": "error",
  "code": "PUBLICATION_VALIDATION_FAILED",
  "message": "The definition is not a valid OpenAPI 3.x document: missing properties 'info', 'paths'"
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
  "code": "PUBLICATION_PORTAL_CONFLICT",
  "message": "The API Portal rejected this change."
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

> 503 Response

```json
{
  "status": "error",
  "code": "PUBLICATION_PORTAL_UNAVAILABLE",
  "message": "The API Portal could not be reached. Try again later.",
  "trackingId": "4f1c6f2e-8a4b-4c93-b1de-9f2f6f0c2a11"
}
```

<h3 id="publish-rest-api-to-api-portal-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Listing updated|[Publication](schemas.md#schemapublication)|
|201|[Created](https://tools.ietf.org/html/rfc7231#section-6.3.2)|Listing created|[Publication](schemas.md#schemapublication)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Either this API type has no projection onto the API Portal's own API types, so no listing can be created for it, the draft's stored definition is not a valid OpenAPI 3.x document — publish is the one point definition validity is enforced; the draft itself may hold anything — or a selected subscription plan is no longer active.|[Error](schemas.md#schemaerror)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Forbidden. The authenticated user does not have permission to access this resource.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Conflict. code identifies which: PUBLICATION_STATE_CONFLICT when the action is not valid for the publication's current status, PUBLICATION_DRAFT_CHANGED when the draft was saved while a publish of it was in flight (the API Portal may hold the earlier copy; review the draft and publish again), or PUBLICATION_PORTAL_CONFLICT when the API Portal refused the change: the handle or display name and version is taken, the listing still has subscriptions or active API keys, or a subscription plan could not be created because the portal already holds it. A portal conflict does not clear on retry (rename, remove the consumers, or deprecate instead), except a plan conflict from a concurrent publish, which usually does. No local state was changed by any of these.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|
|503|[Service Unavailable](https://tools.ietf.org/html/rfc7231#section-6.6.4)|The API Portal could not be reached, or failed while handling the request, after in-call retries. No local state was changed and the call is safe to repeat.|[Error](schemas.md#schemaerror)|

### Response Headers

|Status|Header|Type|Format|Description|
|---|---|---|---|---|
|201|Location|string|uri|URL of the newly created resource.|

## Unpublish REST API from API Portal

<a id="opIdunpublishRestApiFromApiPortal"></a>

`POST /api-portals/{apiPortalId}/apis/rest-api/{apiId}/unpublish`

> Code samples

```shell

curl -X POST https://localhost:9243/api/v0.9/api-portals/{apiPortalId}/apis/rest-api/{apiId}/unpublish \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Accept: application/json'

```

Removes the listing from the API Portal, along with its subscriptions and API keys, then
writes locally: if no draft exists, the publication is demoted into the draft in place; if
one exists, the publication is deleted and the existing draft is kept. A draft always
survives. Valid only when currently published or deprecated.

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:portal:rest_api:unpublish`, `ap:portal:rest_api:manage`, `ap:portal:manage`, `ap:manage`

</aside>

<h3 id="unpublish-rest-api-from-api-portal-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|apiPortalId|path|string|true|**API Portal ID** consisting of the **handle** (unique slug identifier) of the API Portal.|
|apiId|path|string|true|The API's handle, unique per organization within its own type.|

#### Detailed descriptions

**apiPortalId**: **API Portal ID** consisting of the **handle** (unique slug identifier) of the API Portal.

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
  "code": "PUBLICATION_PORTAL_CONFLICT",
  "message": "The API Portal rejected this change."
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

> 503 Response

```json
{
  "status": "error",
  "code": "PUBLICATION_PORTAL_UNAVAILABLE",
  "message": "The API Portal could not be reached. Try again later.",
  "trackingId": "4f1c6f2e-8a4b-4c93-b1de-9f2f6f0c2a11"
}
```

<h3 id="unpublish-rest-api-from-api-portal-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|204|[No Content](https://tools.ietf.org/html/rfc7231#section-6.3.5)|Listing removed|None|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Forbidden. The authenticated user does not have permission to access this resource.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Conflict. code identifies which: PUBLICATION_STATE_CONFLICT when the action is not valid for the publication's current status, PUBLICATION_DRAFT_CHANGED when the draft was saved while a publish of it was in flight (the API Portal may hold the earlier copy; review the draft and publish again), or PUBLICATION_PORTAL_CONFLICT when the API Portal refused the change: the handle or display name and version is taken, the listing still has subscriptions or active API keys, or a subscription plan could not be created because the portal already holds it. A portal conflict does not clear on retry (rename, remove the consumers, or deprecate instead), except a plan conflict from a concurrent publish, which usually does. No local state was changed by any of these.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|
|503|[Service Unavailable](https://tools.ietf.org/html/rfc7231#section-6.6.4)|The API Portal could not be reached, or failed while handling the request, after in-call retries. No local state was changed and the call is safe to repeat.|[Error](schemas.md#schemaerror)|

## Deprecate REST API on API Portal

<a id="opIddeprecateRestApiOnApiPortal"></a>

`POST /api-portals/{apiPortalId}/apis/rest-api/{apiId}/deprecate`

> Code samples

```shell

curl -X POST https://localhost:9243/api/v0.9/api-portals/{apiPortalId}/apis/rest-api/{apiId}/deprecate \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Accept: application/json'

```

Marks the live listing deprecated on the API Portal, where it stays visible and is flagged as deprecated. Nothing is deleted and any draft is untouched. Valid only when currently published.

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:portal:rest_api:deprecate`, `ap:portal:rest_api:manage`, `ap:portal:manage`, `ap:manage`

</aside>

<h3 id="deprecate-rest-api-on-api-portal-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|apiPortalId|path|string|true|**API Portal ID** consisting of the **handle** (unique slug identifier) of the API Portal.|
|apiId|path|string|true|The API's handle, unique per organization within its own type.|

#### Detailed descriptions

**apiPortalId**: **API Portal ID** consisting of the **handle** (unique slug identifier) of the API Portal.

> Example responses
>
> 200 Response

```json
{
  "displayName": "string",
  "version": "string",
  "description": "string",
  "tags": [
    "string"
  ],
  "labels": [
    "string"
  ],
  "agentVisibility": "VISIBLE",
  "endpoints": {
    "productionUrl": "http://example.com",
    "sandboxUrl": "http://example.com"
  },
  "owners": {
    "businessOwner": "string",
    "businessOwnerEmail": "user@example.com",
    "technicalOwner": "string",
    "technicalOwnerEmail": "user@example.com"
  },
  "createdAt": "2019-08-24T14:15:22Z",
  "createdBy": "string",
  "updatedAt": "2019-08-24T14:15:22Z",
  "updatedBy": "string",
  "apiPortalId": "string",
  "apiPortalName": "string",
  "status": "PUBLISHED",
  "subscriptionPlanIds": [
    "string"
  ],
  "docIds": [
    "string"
  ],
  "hasThumbnail": true,
  "hasLandingPage": true
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
  "code": "PUBLICATION_PORTAL_CONFLICT",
  "message": "The API Portal rejected this change."
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

> 503 Response

```json
{
  "status": "error",
  "code": "PUBLICATION_PORTAL_UNAVAILABLE",
  "message": "The API Portal could not be reached. Try again later.",
  "trackingId": "4f1c6f2e-8a4b-4c93-b1de-9f2f6f0c2a11"
}
```

<h3 id="deprecate-rest-api-on-api-portal-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Listing deprecated|[Publication](schemas.md#schemapublication)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Forbidden. The authenticated user does not have permission to access this resource.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Conflict. code identifies which: PUBLICATION_STATE_CONFLICT when the action is not valid for the publication's current status, PUBLICATION_DRAFT_CHANGED when the draft was saved while a publish of it was in flight (the API Portal may hold the earlier copy; review the draft and publish again), or PUBLICATION_PORTAL_CONFLICT when the API Portal refused the change: the handle or display name and version is taken, the listing still has subscriptions or active API keys, or a subscription plan could not be created because the portal already holds it. A portal conflict does not clear on retry (rename, remove the consumers, or deprecate instead), except a plan conflict from a concurrent publish, which usually does. No local state was changed by any of these.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|
|503|[Service Unavailable](https://tools.ietf.org/html/rfc7231#section-6.6.4)|The API Portal could not be reached, or failed while handling the request, after in-call retries. No local state was changed and the call is safe to repeat.|[Error](schemas.md#schemaerror)|

## Publish GraphQL API to API Portal

<a id="opIdpublishGraphQLApiToApiPortal"></a>

`POST /api-portals/{apiPortalId}/apis/graphql-api/{apiId}/publish`

> Code samples

```shell

curl -X POST https://localhost:9243/api/v0.9/api-portals/{apiPortalId}/apis/graphql-api/{apiId}/publish \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Accept: application/json'

```

Publishes the current draft. Takes no request body: details, plans, documents, definition,
landing page and thumbnail all come from the draft alone — publish never reads the API's
own content or the current publication to fill a gap. Requires a draft to exist.

The content is pushed to the API Portal before anything is written locally, so a failed
call leaves the draft intact and can simply be repeated.

Returns 201 on a first publish or a re-publish after unpublish, 200 otherwise.

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:portal:graphql_api:publish`, `ap:portal:graphql_api:manage`, `ap:portal:manage`, `ap:manage`

</aside>

<h3 id="publish-graphql-api-to-api-portal-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|apiPortalId|path|string|true|**API Portal ID** consisting of the **handle** (unique slug identifier) of the API Portal.|
|apiId|path|string|true|The API's handle, unique per organization within its own type.|

#### Detailed descriptions

**apiPortalId**: **API Portal ID** consisting of the **handle** (unique slug identifier) of the API Portal.

> Example responses
>
> 200 Response

```json
{
  "displayName": "string",
  "version": "string",
  "description": "string",
  "tags": [
    "string"
  ],
  "labels": [
    "string"
  ],
  "agentVisibility": "VISIBLE",
  "endpoints": {
    "productionUrl": "http://example.com",
    "sandboxUrl": "http://example.com"
  },
  "owners": {
    "businessOwner": "string",
    "businessOwnerEmail": "user@example.com",
    "technicalOwner": "string",
    "technicalOwnerEmail": "user@example.com"
  },
  "createdAt": "2019-08-24T14:15:22Z",
  "createdBy": "string",
  "updatedAt": "2019-08-24T14:15:22Z",
  "updatedBy": "string",
  "apiPortalId": "string",
  "apiPortalName": "string",
  "status": "PUBLISHED",
  "subscriptionPlanIds": [
    "string"
  ],
  "docIds": [
    "string"
  ],
  "hasThumbnail": true,
  "hasLandingPage": true
}
```

> Either this API type has no projection onto the API Portal's own API types, so no listing can be created for it, the draft's stored definition is not a valid OpenAPI 3.x document — publish is the one point definition validity is enforced; the draft itself may hold anything — or a selected subscription plan is no longer active.

```json
{
  "status": "error",
  "code": "PUBLICATION_TYPE_UNSUPPORTED",
  "message": "APIs of this type cannot be published to an API Portal."
}
```

```json
{
  "status": "error",
  "code": "PUBLICATION_VALIDATION_FAILED",
  "message": "The definition is not a valid OpenAPI 3.x document: missing properties 'info', 'paths'"
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
  "code": "PUBLICATION_PORTAL_CONFLICT",
  "message": "The API Portal rejected this change."
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

> 503 Response

```json
{
  "status": "error",
  "code": "PUBLICATION_PORTAL_UNAVAILABLE",
  "message": "The API Portal could not be reached. Try again later.",
  "trackingId": "4f1c6f2e-8a4b-4c93-b1de-9f2f6f0c2a11"
}
```

<h3 id="publish-graphql-api-to-api-portal-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Listing updated|[Publication](schemas.md#schemapublication)|
|201|[Created](https://tools.ietf.org/html/rfc7231#section-6.3.2)|Listing created|[Publication](schemas.md#schemapublication)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Either this API type has no projection onto the API Portal's own API types, so no listing can be created for it, the draft's stored definition is not a valid OpenAPI 3.x document — publish is the one point definition validity is enforced; the draft itself may hold anything — or a selected subscription plan is no longer active.|[Error](schemas.md#schemaerror)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Forbidden. The authenticated user does not have permission to access this resource.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Conflict. code identifies which: PUBLICATION_STATE_CONFLICT when the action is not valid for the publication's current status, PUBLICATION_DRAFT_CHANGED when the draft was saved while a publish of it was in flight (the API Portal may hold the earlier copy; review the draft and publish again), or PUBLICATION_PORTAL_CONFLICT when the API Portal refused the change: the handle or display name and version is taken, the listing still has subscriptions or active API keys, or a subscription plan could not be created because the portal already holds it. A portal conflict does not clear on retry (rename, remove the consumers, or deprecate instead), except a plan conflict from a concurrent publish, which usually does. No local state was changed by any of these.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|
|503|[Service Unavailable](https://tools.ietf.org/html/rfc7231#section-6.6.4)|The API Portal could not be reached, or failed while handling the request, after in-call retries. No local state was changed and the call is safe to repeat.|[Error](schemas.md#schemaerror)|

### Response Headers

|Status|Header|Type|Format|Description|
|---|---|---|---|---|
|201|Location|string|uri|URL of the newly created resource.|

## Unpublish GraphQL API from API Portal

<a id="opIdunpublishGraphQLApiFromApiPortal"></a>

`POST /api-portals/{apiPortalId}/apis/graphql-api/{apiId}/unpublish`

> Code samples

```shell

curl -X POST https://localhost:9243/api/v0.9/api-portals/{apiPortalId}/apis/graphql-api/{apiId}/unpublish \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Accept: application/json'

```

Removes the listing from the API Portal, along with its subscriptions and API keys, then
writes locally: if no draft exists, the publication is demoted into the draft in place; if
one exists, the publication is deleted and the existing draft is kept. A draft always
survives. Valid only when currently published or deprecated.

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:portal:graphql_api:unpublish`, `ap:portal:graphql_api:manage`, `ap:portal:manage`, `ap:manage`

</aside>

<h3 id="unpublish-graphql-api-from-api-portal-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|apiPortalId|path|string|true|**API Portal ID** consisting of the **handle** (unique slug identifier) of the API Portal.|
|apiId|path|string|true|The API's handle, unique per organization within its own type.|

#### Detailed descriptions

**apiPortalId**: **API Portal ID** consisting of the **handle** (unique slug identifier) of the API Portal.

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
  "code": "PUBLICATION_PORTAL_CONFLICT",
  "message": "The API Portal rejected this change."
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

> 503 Response

```json
{
  "status": "error",
  "code": "PUBLICATION_PORTAL_UNAVAILABLE",
  "message": "The API Portal could not be reached. Try again later.",
  "trackingId": "4f1c6f2e-8a4b-4c93-b1de-9f2f6f0c2a11"
}
```

<h3 id="unpublish-graphql-api-from-api-portal-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|204|[No Content](https://tools.ietf.org/html/rfc7231#section-6.3.5)|Listing removed|None|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Forbidden. The authenticated user does not have permission to access this resource.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Conflict. code identifies which: PUBLICATION_STATE_CONFLICT when the action is not valid for the publication's current status, PUBLICATION_DRAFT_CHANGED when the draft was saved while a publish of it was in flight (the API Portal may hold the earlier copy; review the draft and publish again), or PUBLICATION_PORTAL_CONFLICT when the API Portal refused the change: the handle or display name and version is taken, the listing still has subscriptions or active API keys, or a subscription plan could not be created because the portal already holds it. A portal conflict does not clear on retry (rename, remove the consumers, or deprecate instead), except a plan conflict from a concurrent publish, which usually does. No local state was changed by any of these.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|
|503|[Service Unavailable](https://tools.ietf.org/html/rfc7231#section-6.6.4)|The API Portal could not be reached, or failed while handling the request, after in-call retries. No local state was changed and the call is safe to repeat.|[Error](schemas.md#schemaerror)|

## Deprecate GraphQL API on API Portal

<a id="opIddeprecateGraphQLApiOnApiPortal"></a>

`POST /api-portals/{apiPortalId}/apis/graphql-api/{apiId}/deprecate`

> Code samples

```shell

curl -X POST https://localhost:9243/api/v0.9/api-portals/{apiPortalId}/apis/graphql-api/{apiId}/deprecate \
  -H 'Authorization: Bearer {access_token}' \
  -H 'Accept: application/json'

```

Marks the live listing deprecated on the API Portal, where it stays visible and is flagged as deprecated. Nothing is deleted and any draft is untouched. Valid only when currently published.

### Authentication

<aside class="warning">
This operation requires a <strong>Bearer JWT</strong> access token in the <code>Authorization</code> header.

Required scopes (the token must carry at least one of): `ap:portal:graphql_api:deprecate`, `ap:portal:graphql_api:manage`, `ap:portal:manage`, `ap:manage`

</aside>

<h3 id="deprecate-graphql-api-on-api-portal-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|apiPortalId|path|string|true|**API Portal ID** consisting of the **handle** (unique slug identifier) of the API Portal.|
|apiId|path|string|true|The API's handle, unique per organization within its own type.|

#### Detailed descriptions

**apiPortalId**: **API Portal ID** consisting of the **handle** (unique slug identifier) of the API Portal.

> Example responses
>
> 200 Response

```json
{
  "displayName": "string",
  "version": "string",
  "description": "string",
  "tags": [
    "string"
  ],
  "labels": [
    "string"
  ],
  "agentVisibility": "VISIBLE",
  "endpoints": {
    "productionUrl": "http://example.com",
    "sandboxUrl": "http://example.com"
  },
  "owners": {
    "businessOwner": "string",
    "businessOwnerEmail": "user@example.com",
    "technicalOwner": "string",
    "technicalOwnerEmail": "user@example.com"
  },
  "createdAt": "2019-08-24T14:15:22Z",
  "createdBy": "string",
  "updatedAt": "2019-08-24T14:15:22Z",
  "updatedBy": "string",
  "apiPortalId": "string",
  "apiPortalName": "string",
  "status": "PUBLISHED",
  "subscriptionPlanIds": [
    "string"
  ],
  "docIds": [
    "string"
  ],
  "hasThumbnail": true,
  "hasLandingPage": true
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
  "code": "PUBLICATION_PORTAL_CONFLICT",
  "message": "The API Portal rejected this change."
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

> 503 Response

```json
{
  "status": "error",
  "code": "PUBLICATION_PORTAL_UNAVAILABLE",
  "message": "The API Portal could not be reached. Try again later.",
  "trackingId": "4f1c6f2e-8a4b-4c93-b1de-9f2f6f0c2a11"
}
```

<h3 id="deprecate-graphql-api-on-api-portal-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Listing deprecated|[Publication](schemas.md#schemapublication)|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized. Authentication credentials are missing or invalid.|[Error](schemas.md#schemaerror)|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Forbidden. The authenticated user does not have permission to access this resource.|[Error](schemas.md#schemaerror)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not Found. The specified resource does not exist.|[Error](schemas.md#schemaerror)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Conflict. code identifies which: PUBLICATION_STATE_CONFLICT when the action is not valid for the publication's current status, PUBLICATION_DRAFT_CHANGED when the draft was saved while a publish of it was in flight (the API Portal may hold the earlier copy; review the draft and publish again), or PUBLICATION_PORTAL_CONFLICT when the API Portal refused the change: the handle or display name and version is taken, the listing still has subscriptions or active API keys, or a subscription plan could not be created because the portal already holds it. A portal conflict does not clear on retry (rename, remove the consumers, or deprecate instead), except a plan conflict from a concurrent publish, which usually does. No local state was changed by any of these.|[Error](schemas.md#schemaerror)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal Server Error.|[Error](schemas.md#schemaerror)|
|503|[Service Unavailable](https://tools.ietf.org/html/rfc7231#section-6.6.4)|The API Portal could not be reached, or failed while handling the request, after in-call retries. No local state was changed and the call is safe to repeat.|[Error](schemas.md#schemaerror)|
