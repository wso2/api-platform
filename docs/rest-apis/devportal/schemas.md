# Schemas

<h2 id="tocS_Pagination">Pagination</h2>

<a id="schemapagination"></a>
<a id="schema_Pagination"></a>
<a id="tocSpagination"></a>
<a id="tocspagination"></a>

```json
{
  "total": 42,
  "limit": 20,
  "offset": 0
}

```

Standard pagination metadata returned with collection responses.

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|total|integer|true|none|Total number of records matching the query.|
|limit|integer|true|none|Maximum number of records returned in this response.|
|offset|integer|true|none|Number of records skipped before this page.|

<h2 id="tocS_MessageResponse">MessageResponse</h2>

<a id="schemamessageresponse"></a>
<a id="schema_MessageResponse"></a>
<a id="tocSmessageresponse"></a>
<a id="tocsmessageresponse"></a>

```json
{
  "message": "string"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|message|string|true|none|none|

<h2 id="tocS_GenericValue">GenericValue</h2>

<a id="schemagenericvalue"></a>
<a id="schema_GenericValue"></a>
<a id="tocSgenericvalue"></a>
<a id="tocsgenericvalue"></a>

```json
{}

```

### Properties

oneOf

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|object|false|none|none|

xor

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|[any]|false|none|none|

xor

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|string|false|none|none|

xor

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|number|false|none|none|

xor

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|boolean|false|none|none|

<h2 id="tocS_GenericObject">GenericObject</h2>

<a id="schemagenericobject"></a>
<a id="schema_GenericObject"></a>
<a id="tocSgenericobject"></a>
<a id="tocsgenericobject"></a>

```json
{}

```

### Properties

*None*

<h2 id="tocS_ErrorResponse">ErrorResponse</h2>

<a id="schemaerrorresponse"></a>
<a id="schema_ErrorResponse"></a>
<a id="tocSerrorresponse"></a>
<a id="tocserrorresponse"></a>

```json
{
  "status": "error",
  "code": "ORG_NOT_FOUND",
  "message": "string",
  "errors": [
    {
      "field": "string",
      "message": "string"
    }
  ]
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|status|string|true|none|Always "error" for error responses.|
|code|string|true|none|Machine-readable SCREAMING_SNAKE_CASE catalog code.|
|message|string|true|none|Human-readable error message.|
|errors|[object]|false|none|Optional per-field validation errors.|
|» field|string|true|none|none|
|» message|string|true|none|none|

#### Enumerated Values

|Property|Value|
|---|---|
|status|error|

<h2 id="tocS_SimpleErrorResponse">SimpleErrorResponse</h2>

<a id="schemasimpleerrorresponse"></a>
<a id="schema_SimpleErrorResponse"></a>
<a id="tocSsimpleerrorresponse"></a>
<a id="tocssimpleerrorresponse"></a>

```json
{
  "code": "404",
  "message": "Not Found",
  "description": "Subscription not found"
}

```

Ad hoc error shape used by the Subscriptions and API Keys handlers, which build error bodies inline instead of going through the shared error formatter. `code` is the HTTP status code as a string, not a catalog code.

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|code|string|true|none|none|
|message|string|true|none|none|
|description|string¦null|false|none|none|

<h2 id="tocS_ValidationErrorList">ValidationErrorList</h2>

<a id="schemavalidationerrorlist"></a>
<a id="schema_ValidationErrorList"></a>
<a id="tocSvalidationerrorlist"></a>
<a id="tocsvalidationerrorlist"></a>

```json
[
  {
    "status": "error",
    "code": "COMMON_VALIDATION_ERROR",
    "message": "Input validation failed.",
    "errors": [
      {
        "field": "string",
        "message": "string"
      }
    ]
  }
]

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|[[ValidationError](#schemavalidationerror)]|false|none|none|

<h2 id="tocS_ValidationError">ValidationError</h2>

<a id="schemavalidationerror"></a>
<a id="schema_ValidationError"></a>
<a id="tocSvalidationerror"></a>
<a id="tocsvalidationerror"></a>

```json
{
  "status": "error",
  "code": "COMMON_VALIDATION_ERROR",
  "message": "Input validation failed.",
  "errors": [
    {
      "field": "string",
      "message": "string"
    }
  ]
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|status|string|true|none|none|
|code|string|true|none|Machine-readable catalog code.|
|message|string|true|none|none|
|errors|[object]|false|none|Per-field validation errors.|
|» field|string|true|none|none|
|» message|string|true|none|none|

#### Enumerated Values

|Property|Value|
|---|---|
|status|error|

<h2 id="tocS_OrganizationResponse">OrganizationResponse</h2>

<a id="schemaorganizationresponse"></a>
<a id="schema_OrganizationResponse"></a>
<a id="tocSorganizationresponse"></a>
<a id="tocsorganizationresponse"></a>

```json
{
  "id": "acme",
  "displayName": "Acme Corporation",
  "businessOwner": "string",
  "businessOwnerContact": "string",
  "businessOwnerEmail": "user@example.com",
  "idpRefId": "string",
  "cpRefId": "string",
  "configuration": {
    "devportalMode": "DEFAULT"
  },
  "createdAt": "2019-08-24T14:15:22Z",
  "updatedAt": "2019-08-24T14:15:22Z"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|id|string|false|none|The organization's handle (unique). Not the internal database uuid.|
|displayName|string|false|none|none|
|businessOwner|string¦null|false|none|none|
|businessOwnerContact|string¦null|false|none|none|
|businessOwnerEmail|string(email)¦null|false|none|none|
|idpRefId|string|false|none|The organization claim value asserted by the configured Identity Provider at SSO login. On every login, the portal matches the authenticated user's org claim against this value to resolve which organization they belong to — it must exactly match the IDP's claim, or login fails for that org's users. Distinct from `cpRefId`, which is unrelated to authentication.|
|cpRefId|string¦null|false|none|Control Plane reference ID. Included in outbound webhook event payloads so subscribers can correlate this organization with its Control Plane (Platform API) counterpart. Not used for authentication or org resolution.|
|configuration|object|false|none|Organization portal configuration. Always includes `devportalMode`; may contain additional free-form keys set by the caller.|
|» devportalMode|string|false|none|Controls the mode of the developer portal.|
|createdAt|string(date-time)¦null|false|none|none|
|updatedAt|string(date-time)¦null|false|none|none|

#### Enumerated Values

|Property|Value|
|---|---|
|devportalMode|DEFAULT|
|devportalMode|MCP_SERVERS_ONLY|
|devportalMode|APIS_ONLY|

<h2 id="tocS_OrganizationContentUploadResponse">OrganizationContentUploadResponse</h2>

<a id="schemaorganizationcontentuploadresponse"></a>
<a id="schema_OrganizationContentUploadResponse"></a>
<a id="tocSorganizationcontentuploadresponse"></a>
<a id="tocsorganizationcontentuploadresponse"></a>

```json
{
  "id": "string",
  "fileName": "string"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|id|string|true|none|none|
|fileName|string|true|none|Original ZIP file name uploaded in the `file` multipart field.|

<h2 id="tocS_OrganizationContentListItemResponse">OrganizationContentListItemResponse</h2>

<a id="schemaorganizationcontentlistitemresponse"></a>
<a id="schema_OrganizationContentListItemResponse"></a>
<a id="tocSorganizationcontentlistitemresponse"></a>
<a id="tocsorganizationcontentlistitemresponse"></a>

```json
{
  "id": "string",
  "fileName": "string",
  "fileContent": "string"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|id|string|false|none|none|
|fileName|string|false|none|none|
|fileContent|string¦null|false|none|UTF-8 content string returned for stored organization content records.|

<h2 id="tocS_ApiMetadataCreateResponse">ApiMetadataCreateResponse</h2>

<a id="schemaapimetadatacreateresponse"></a>
<a id="schema_ApiMetadataCreateResponse"></a>
<a id="tocSapimetadatacreateresponse"></a>
<a id="tocsapimetadatacreateresponse"></a>

```json
{
  "name": "string",
  "apiTitle": "string",
  "remotes": [
    {}
  ],
  "version": "string",
  "status": "PUBLISHED",
  "description": "string",
  "type": "RestApi",
  "referenceId": "string",
  "agentVisibility": "VISIBLE",
  "addedLabels": [
    "string"
  ],
  "removedLabels": [
    "string"
  ],
  "owners": {
    "technicalOwner": "string",
    "businessOwner": "string",
    "businessOwnerEmail": "string",
    "technicalOwnerEmail": "string"
  },
  "apiImageMetadata": {
    "property1": "string",
    "property2": "string"
  },
  "tags": [
    "string"
  ],
  "labels": [
    "string"
  ],
  "id": "string",
  "refId": "string",
  "endPoints": {
    "sandboxURL": "string",
    "productionURL": "string"
  },
  "subscriptionPlans": [
    {
      "id": "string",
      "displayName": "string",
      "description": "string",
      "limits": [
        {
          "limitType": "REQUEST_COUNT",
          "limitCount": 10000,
          "timeUnit": "MINUTE",
          "timeAmount": 1
        }
      ],
      "refId": "string",
      "orgId": "string",
      "createdBy": "alice@example.com",
      "updatedBy": "alice@example.com",
      "createdAt": "2019-08-24T14:15:22Z",
      "updatedAt": "2019-08-24T14:15:22Z"
    }
  ]
}

```

### Properties

allOf

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|[ApiInfoResponse](#schemaapiinforesponse)|false|none|Fields are returned at the root of ApiMetadataResponse / ApiMetadataCreateResponse (not nested under an `apiInfo` key) — this schema exists only to share the field set between the two via `allOf`.|

and

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|object|false|none|none|
|» id|string|false|none|The API's handle (unique per org). Not the internal database uuid.|
|» refId|string¦null|false|none|Platform API (Control Plane) reference ID for this API. Used for MCP registry visibility filtering and included in outbound webhook event payloads. Null/absent for APIs that exist only in the Developer Portal and are not registered with the Platform API — e.g. MCP servers published via the registry.|
|» endPoints|[ApiEndpointsResponse](#schemaapiendpointsresponse)|false|none|none|
|» subscriptionPlans|[[SubscriptionPlanResponse](#schemasubscriptionplanresponse)]|false|none|none|

<h2 id="tocS_ApiMetadataResponse">ApiMetadataResponse</h2>

<a id="schemaapimetadataresponse"></a>
<a id="schema_ApiMetadataResponse"></a>
<a id="tocSapimetadataresponse"></a>
<a id="tocsapimetadataresponse"></a>

```json
{
  "name": "string",
  "apiTitle": "string",
  "remotes": [
    {}
  ],
  "version": "string",
  "status": "PUBLISHED",
  "description": "string",
  "type": "RestApi",
  "referenceId": "string",
  "agentVisibility": "VISIBLE",
  "addedLabels": [
    "string"
  ],
  "removedLabels": [
    "string"
  ],
  "owners": {
    "technicalOwner": "string",
    "businessOwner": "string",
    "businessOwnerEmail": "string",
    "technicalOwnerEmail": "string"
  },
  "apiImageMetadata": {
    "property1": "string",
    "property2": "string"
  },
  "tags": [
    "string"
  ],
  "labels": [
    "string"
  ],
  "id": "string",
  "refId": "string",
  "dataSource": "string",
  "planId": "string",
  "endPoints": {
    "sandboxURL": "string",
    "productionURL": "string"
  },
  "subscriptionPlans": [
    {
      "id": "string",
      "displayName": "string",
      "description": "string",
      "limits": [
        {
          "limitType": "REQUEST_COUNT",
          "limitCount": 10000,
          "timeUnit": "MINUTE",
          "timeAmount": 1
        }
      ],
      "refId": "string",
      "orgId": "string",
      "createdBy": "alice@example.com",
      "updatedBy": "alice@example.com",
      "createdAt": "2019-08-24T14:15:22Z",
      "updatedAt": "2019-08-24T14:15:22Z"
    }
  ],
  "createdBy": "alice@example.com",
  "updatedBy": "alice@example.com",
  "createdAt": "2026-05-07T08:30:00Z",
  "updatedAt": "2026-05-07T08:30:00Z"
}

```

### Properties

allOf

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|[ApiInfoResponse](#schemaapiinforesponse)|false|none|Fields are returned at the root of ApiMetadataResponse / ApiMetadataCreateResponse (not nested under an `apiInfo` key) — this schema exists only to share the field set between the two via `allOf`.|

and

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|object|false|none|none|
|» id|string|false|none|The API's handle (unique per org). Not the internal database uuid.|
|» refId|string¦null|false|none|Platform API (Control Plane) reference ID for this API. Used for MCP registry visibility filtering and included in outbound webhook event payloads. Null/absent for APIs that exist only in the Developer Portal and are not registered with the Platform API — e.g. MCP servers published via the registry.|
|» dataSource|string¦null|false|none|Indicates which content matched the search term: `METADATA` if the match was in the API's own metadata, or a content type (e.g. a value from the API Content `type` field) if the match was inside an uploaded content file. Only computed by getAllApiMetadataForOrganization when both the `query` search parameter is supplied and the database is PostgreSQL — absent on SQLite (the dev default) and absent from every other operation (get/create/update single API).|
|» planId|string|false|none|none|
|» endPoints|[ApiEndpointsResponse](#schemaapiendpointsresponse)|false|none|none|
|» subscriptionPlans|[[SubscriptionPlanResponse](#schemasubscriptionplanresponse)]|false|none|none|
|» createdBy|string|false|none|Identity of the user who created this API, or `deleted_user` if that user's IDP reference no longer exists. Present on single-resource GET responses and list items.|
|» updatedBy|string|false|none|Identity of the user who last updated this API, or `deleted_user` if that user's IDP reference no longer exists. Present on single-resource GET responses only, omitted on list items.|
|» createdAt|string(date-time)|false|none|none|
|» updatedAt|string(date-time)|false|none|none|

<h2 id="tocS_ApiInfoResponse">ApiInfoResponse</h2>

<a id="schemaapiinforesponse"></a>
<a id="schema_ApiInfoResponse"></a>
<a id="tocSapiinforesponse"></a>
<a id="tocsapiinforesponse"></a>

```json
{
  "name": "string",
  "apiTitle": "string",
  "remotes": [
    {}
  ],
  "version": "string",
  "status": "PUBLISHED",
  "description": "string",
  "type": "RestApi",
  "referenceId": "string",
  "agentVisibility": "VISIBLE",
  "addedLabels": [
    "string"
  ],
  "removedLabels": [
    "string"
  ],
  "owners": {
    "technicalOwner": "string",
    "businessOwner": "string",
    "businessOwnerEmail": "string",
    "technicalOwnerEmail": "string"
  },
  "apiImageMetadata": {
    "property1": "string",
    "property2": "string"
  },
  "tags": [
    "string"
  ],
  "labels": [
    "string"
  ]
}

```

Fields are returned at the root of ApiMetadataResponse / ApiMetadataCreateResponse (not nested under an `apiInfo` key) — this schema exists only to share the field set between the two via `allOf`.

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|name|string|false|none|none|
|apiTitle|string¦null|false|none|none|
|remotes|[object]|false|none|none|
|version|string|false|none|none|
|status|string|false|none|API lifecycle status.|
|description|string|false|none|none|
|type|string|false|none|The stored/returned type constant (src/utils/constants.js API_TYPE) — distinct from the request-time keyword accepted on create/update (see `type` in ApiMetadataMultipartBody: REST, SOAP, MCP, WS, WEBSUB, GRAPHQL). REST maps to `RestApi` and WEBSUB maps to `WebSubApi`; the rest are returned unchanged.|
|referenceId|string¦null|false|none|External reference ID. Present when the API was created from a `devportal.yaml` artifact whose `spec` block sets `referenceId` — the create response echoes the parsed YAML back.|
|agentVisibility|string|false|none|none|
|addedLabels|[string]|false|none|none|
|removedLabels|[string]|false|none|none|
|owners|[ApiOwnersResponse](#schemaapiownersresponse)|false|none|none|
|apiImageMetadata|[ApiImageMetadataResponse](#schemaapiimagemetadataresponse)|false|none|none|
|tags|[string]|false|none|none|
|labels|[string]|false|none|none|

#### Enumerated Values

|Property|Value|
|---|---|
|status|PUBLISHED|
|status|DEPRECATED|
|type|RestApi|
|type|SOAP|
|type|Mcp|
|type|WS|
|type|WebSubApi|
|type|GRAPHQL|
|agentVisibility|VISIBLE|
|agentVisibility|HIDDEN|

<h2 id="tocS_ApiOwnersResponse">ApiOwnersResponse</h2>

<a id="schemaapiownersresponse"></a>
<a id="schema_ApiOwnersResponse"></a>
<a id="tocSapiownersresponse"></a>
<a id="tocsapiownersresponse"></a>

```json
{
  "technicalOwner": "string",
  "businessOwner": "string",
  "businessOwnerEmail": "string",
  "technicalOwnerEmail": "string"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|technicalOwner|string|false|none|none|
|businessOwner|string|false|none|none|
|businessOwnerEmail|string|false|none|none|
|technicalOwnerEmail|string|false|none|none|

<h2 id="tocS_ApiEndpointsResponse">ApiEndpointsResponse</h2>

<a id="schemaapiendpointsresponse"></a>
<a id="schema_ApiEndpointsResponse"></a>
<a id="tocSapiendpointsresponse"></a>
<a id="tocsapiendpointsresponse"></a>

```json
{
  "sandboxURL": "string",
  "productionURL": "string"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|sandboxURL|string|false|none|none|
|productionURL|string|false|none|none|

<h2 id="tocS_ApiImageMetadataResponse">ApiImageMetadataResponse</h2>

<a id="schemaapiimagemetadataresponse"></a>
<a id="schema_ApiImageMetadataResponse"></a>
<a id="tocSapiimagemetadataresponse"></a>
<a id="tocsapiimagemetadataresponse"></a>

```json
{
  "property1": "string",
  "property2": "string"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|**additionalProperties**|string|false|none|none|

<h2 id="tocS_SubscriptionPlanResponse">SubscriptionPlanResponse</h2>

<a id="schemasubscriptionplanresponse"></a>
<a id="schema_SubscriptionPlanResponse"></a>
<a id="tocSsubscriptionplanresponse"></a>
<a id="tocssubscriptionplanresponse"></a>

```json
{
  "id": "string",
  "displayName": "string",
  "description": "string",
  "limits": [
    {
      "limitType": "REQUEST_COUNT",
      "limitCount": 10000,
      "timeUnit": "MINUTE",
      "timeAmount": 1
    }
  ],
  "refId": "string",
  "orgId": "string",
  "createdBy": "alice@example.com",
  "updatedBy": "alice@example.com",
  "createdAt": "2019-08-24T14:15:22Z",
  "updatedAt": "2019-08-24T14:15:22Z"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|id|string|false|none|The plan's handle (unique per org). Not the internal database uuid.|
|displayName|string|false|none|none|
|description|string|false|none|none|
|limits|[object]|false|none|Rate/quota limits enforced for this plan. Empty when the plan is unlimited.|
|» limitType|string|false|none|none|
|» limitCount|any|false|none|Returned as a string when the stored count exceeds the safe integer range, otherwise a number. Unlimited plans have no limit entries — the `limits` array is empty.|

oneOf

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|»» *anonymous*|integer|false|none|none|

xor

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|»» *anonymous*|string|false|none|none|

continued

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|» timeUnit|string¦null|false|none|none|
|» timeAmount|integer|false|none|none|
|refId|string¦null|false|none|Platform API subscription plan UUID associated with this plan.|
|orgId|string|false|none|none|
|createdBy|string|false|none|Identity of the user who created this subscription plan, or `deleted_user` if that user's IDP reference no longer exists. Present on single-resource GET responses and list items.|
|updatedBy|string|false|none|Identity of the user who last updated this subscription plan, or `deleted_user` if that user's IDP reference no longer exists. Present on single-resource GET responses only, omitted on list items.|
|createdAt|string(date-time)|false|none|none|
|updatedAt|string(date-time)|false|none|none|

#### Enumerated Values

|Property|Value|
|---|---|
|limitType|REQUEST_COUNT|
|limitType|EVENT_COUNT|
|limitType|BANDWIDTH|
|limitType|TOTAL_TOKEN_COUNT|
|timeUnit|MINUTE|
|timeUnit|HOUR|
|timeUnit|DAY|
|timeUnit|MONTH|
|timeUnit|null|

<h2 id="tocS_LabelResponse">LabelResponse</h2>

<a id="schemalabelresponse"></a>
<a id="schema_LabelResponse"></a>
<a id="tocSlabelresponse"></a>
<a id="tocslabelresponse"></a>

```json
{
  "id": "premium",
  "displayName": "Premium APIs"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|id|string|false|none|The label's handle (unique per org). Not the internal database uuid.|
|displayName|string|false|none|none|

<h2 id="tocS_ApplicationResponse">ApplicationResponse</h2>

<a id="schemaapplicationresponse"></a>
<a id="schema_ApplicationResponse"></a>
<a id="tocSapplicationresponse"></a>
<a id="tocsapplicationresponse"></a>

```json
{
  "id": "my-weather-app",
  "displayName": "Weather App",
  "description": "Application used to call Weather APIs.",
  "appKeyMappings": [
    {
      "asClientId": "asgardeo-client-abc123",
      "kmId": "km-uuid-12345",
      "type": "PRODUCTION"
    }
  ],
  "createdBy": "alice@example.com",
  "updatedBy": "alice@example.com",
  "createdAt": "2026-05-07T08:30:00Z",
  "updatedAt": "2026-05-07T08:30:00Z"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|id|string|false|none|The application's handle (unique per org). Not the internal database uuid.|
|displayName|string|false|none|none|
|description|string|false|none|none|
|appKeyMappings|[[ApplicationKeyMappingSummary](#schemaapplicationkeymappingsummary)]|false|none|[OAuth client ID mapping entry attached to an application.]|
|createdBy|string|false|none|Identity of the user who created this application, or `deleted_user` if that user's IDP reference no longer exists. Present on single-resource GET responses and list items.|
|updatedBy|string|false|none|Identity of the user who last updated this application, or `deleted_user` if that user's IDP reference no longer exists. Present on single-resource GET responses only, omitted on list items.|
|createdAt|string(date-time)|false|none|none|
|updatedAt|string(date-time)|false|none|none|

<h2 id="tocS_ApplicationKeyMappingSummary">ApplicationKeyMappingSummary</h2>

<a id="schemaapplicationkeymappingsummary"></a>
<a id="schema_ApplicationKeyMappingSummary"></a>
<a id="tocSapplicationkeymappingsummary"></a>
<a id="tocsapplicationkeymappingsummary"></a>

```json
{
  "asClientId": "asgardeo-client-abc123",
  "kmId": "km-uuid-12345",
  "type": "PRODUCTION"
}

```

OAuth client ID mapping entry attached to an application.

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|asClientId|string|false|none|OAuth client ID, created directly in the key manager and linked to this application.|
|kmId|string|false|none|UUID of the key manager this client ID is linked to.|
|type|string|false|none|Key type for this mapping.|

#### Enumerated Values

|Property|Value|
|---|---|
|type|PRODUCTION|
|type|SANDBOX|

<h2 id="tocS_ViewResponse">ViewResponse</h2>

<a id="schemaviewresponse"></a>
<a id="schema_ViewResponse"></a>
<a id="tocSviewresponse"></a>
<a id="tocsviewresponse"></a>

```json
{
  "id": "partner-apis",
  "displayName": "Partner APIs",
  "labels": [
    "partner",
    "public"
  ],
  "createdBy": "alice@example.com",
  "updatedBy": "alice@example.com",
  "createdAt": "2019-08-24T14:15:22Z",
  "updatedAt": "2019-08-24T14:15:22Z"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|id|string|true|none|The view's handle (unique per org). Not the internal database uuid.|
|displayName|string|true|none|none|
|labels|[string]|true|none|none|
|createdBy|string|false|none|Identity of the user who created this view, or `deleted_user` if that user's IDP reference no longer exists. Present on single-resource GET responses and list items.|
|updatedBy|string|false|none|Identity of the user who last updated this view, or `deleted_user` if that user's IDP reference no longer exists. Present on single-resource GET responses only, omitted on list items.|
|createdAt|string(date-time)|false|none|none|
|updatedAt|string(date-time)|false|none|none|

<h2 id="tocS_OrganizationCreateRequest">OrganizationCreateRequest</h2>

<a id="schemaorganizationcreaterequest"></a>
<a id="schema_OrganizationCreateRequest"></a>
<a id="tocSorganizationcreaterequest"></a>
<a id="tocsorganizationcreaterequest"></a>

```json
{
  "displayName": "Acme Corporation",
  "businessOwner": "string",
  "businessOwnerContact": "string",
  "businessOwnerEmail": "user@example.com",
  "id": "acme",
  "idpRefId": "string",
  "cpRefId": "string",
  "configuration": {
    "devportalMode": "DEFAULT"
  }
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|displayName|string|true|none|none|
|businessOwner|string|false|none|none|
|businessOwnerContact|string|false|none|none|
|businessOwnerEmail|string(email)|false|none|none|
|id|string|true|none|Desired handle for the organization (unique), stored as-is. Used in portal URLs.|
|idpRefId|string|true|none|The organization claim value asserted by the configured Identity Provider at SSO login. Must exactly match the IDP's org claim for that org's users, or login will fail. Distinct from `cpRefId`.|
|cpRefId|string¦null|false|none|Control Plane reference ID, included in outbound webhook event payloads. Not used for authentication.|
|configuration|object|false|none|none|
|» devportalMode|string|false|none|none|

#### Enumerated Values

|Property|Value|
|---|---|
|devportalMode|DEFAULT|
|devportalMode|MCP_SERVERS_ONLY|
|devportalMode|APIS_ONLY|

<h2 id="tocS_OrganizationUpdateRequest">OrganizationUpdateRequest</h2>

<a id="schemaorganizationupdaterequest"></a>
<a id="schema_OrganizationUpdateRequest"></a>
<a id="tocSorganizationupdaterequest"></a>
<a id="tocsorganizationupdaterequest"></a>

```json
{
  "displayName": "Acme Corporation",
  "businessOwner": "string",
  "businessOwnerContact": "string",
  "businessOwnerEmail": "user@example.com",
  "id": "acme",
  "idpRefId": "string",
  "cpRefId": "string",
  "configuration": {
    "devportalMode": "DEFAULT"
  }
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|displayName|string|true|none|none|
|businessOwner|string|false|none|none|
|businessOwnerContact|string|false|none|none|
|businessOwnerEmail|string(email)|false|none|none|
|id|string|true|none|Desired handle for the organization (unique), stored as-is. Used in portal URLs.|
|idpRefId|string|true|none|The organization claim value asserted by the configured Identity Provider at SSO login. Must exactly match the IDP's org claim for that org's users, or login will fail. Distinct from `cpRefId`.|
|cpRefId|string¦null|false|none|Control Plane reference ID, included in outbound webhook event payloads. Not used for authentication.|
|configuration|object|false|none|none|
|» devportalMode|string|false|none|none|

#### Enumerated Values

|Property|Value|
|---|---|
|devportalMode|DEFAULT|
|devportalMode|MCP_SERVERS_ONLY|
|devportalMode|APIS_ONLY|

<h2 id="tocS_SubscriptionPlanRequest">SubscriptionPlanRequest</h2>

<a id="schemasubscriptionplanrequest"></a>
<a id="schema_SubscriptionPlanRequest"></a>
<a id="tocSsubscriptionplanrequest"></a>
<a id="tocssubscriptionplanrequest"></a>

```json
{
  "id": "Gold",
  "refId": "string",
  "displayName": "string",
  "description": "string",
  "limits": [
    {
      "limitType": "REQUEST_COUNT",
      "limitCount": 10000,
      "timeUnit": "MINUTE",
      "timeAmount": 1
    }
  ]
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|id|string|true|none|Desired handle for the plan (unique per org), stored as-is. When the plan is created from a SubscriptionPlan YAML artifact instead, the handle is always taken from `metadata.name`.|
|refId|string|false|none|Platform API subscription plan UUID to associate with this plan.|
|displayName|string|true|none|none|
|description|string|false|none|none|
|limits|[object]|false|none|Rate/quota limits enforced for this plan. Omit or leave empty for an unlimited plan. Replaces the whole limit set on update.|
|» limitType|string|false|none|none|
|» limitCount|integer|true|none|Use -1 for unlimited, otherwise a positive number.|
|» timeUnit|string¦null|false|none|Omit for a limit with no time window.|
|» timeAmount|integer|false|none|Size of the time window, in `timeUnit` units.|
|type|string|false|none|Legacy shorthand accepted only via SubscriptionPlan/SubscriptionPlanList YAML upload (`multipart/form-data`); converted into `limits` before storage. Ignored for JSON requests — use `limits` instead.|
|requestCount|any|false|none|Legacy YAML shorthand paired with `type: requestcount`. Use -1 for unlimited.|

oneOf

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|» *anonymous*|integer|false|none|none|

xor

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|» *anonymous*|string|false|none|none|

continued

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|eventCount|any|false|none|Legacy YAML shorthand paired with `type: eventcount`. Use -1 for unlimited.|

oneOf

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|» *anonymous*|integer|false|none|none|

xor

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|» *anonymous*|string|false|none|none|

#### Enumerated Values

|Property|Value|
|---|---|
|limitType|REQUEST_COUNT|
|limitType|EVENT_COUNT|
|limitType|BANDWIDTH|
|limitType|TOTAL_TOKEN_COUNT|
|timeUnit|MINUTE|
|timeUnit|HOUR|
|timeUnit|DAY|
|timeUnit|MONTH|
|timeUnit|null|
|type|requestcount|
|type|eventcount|

<h2 id="tocS_LabelRequest">LabelRequest</h2>

<a id="schemalabelrequest"></a>
<a id="schema_LabelRequest"></a>
<a id="tocSlabelrequest"></a>
<a id="tocslabelrequest"></a>

```json
{
  "id": "premium",
  "displayName": "Premium APIs"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|id|string|true|none|Desired handle for the label (unique per org), stored as-is.|
|displayName|string|true|none|none|

<h2 id="tocS_ApplicationRequest">ApplicationRequest</h2>

<a id="schemaapplicationrequest"></a>
<a id="schema_ApplicationRequest"></a>
<a id="tocSapplicationrequest"></a>
<a id="tocsapplicationrequest"></a>

```json
{
  "displayName": "Weather App",
  "id": "my-weather-app",
  "description": "Application used to call Weather APIs."
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|displayName|string|true|none|none|
|id|string|false|none|Immutable, org-scoped slug for the application, stored as its handle. Optional — defaults to the application's `displayName` when omitted.|
|description|string|true|none|none|

<h2 id="tocS_SubscriptionCreateRequest">SubscriptionCreateRequest</h2>

<a id="schemasubscriptioncreaterequest"></a>
<a id="schema_SubscriptionCreateRequest"></a>
<a id="tocSsubscriptioncreaterequest"></a>
<a id="tocssubscriptioncreaterequest"></a>

```json
{
  "apiId": "weather-api-v1",
  "subscriptionPlanId": "Gold"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|apiId|string|true|none|Developer Portal API ID.|
|subscriptionPlanId|string|true|none|Developer Portal subscription plan ID.|

<h2 id="tocS_SubscriptionUpdateRequest">SubscriptionUpdateRequest</h2>

<a id="schemasubscriptionupdaterequest"></a>
<a id="schema_SubscriptionUpdateRequest"></a>
<a id="tocSsubscriptionupdaterequest"></a>
<a id="tocssubscriptionupdaterequest"></a>

```json
{
  "status": "ACTIVE"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|status|string|true|none|none|

#### Enumerated Values

|Property|Value|
|---|---|
|status|ACTIVE|
|status|INACTIVE|

<h2 id="tocS_SubscriptionChangePlanRequest">SubscriptionChangePlanRequest</h2>

<a id="schemasubscriptionchangeplanrequest"></a>
<a id="schema_SubscriptionChangePlanRequest"></a>
<a id="tocSsubscriptionchangeplanrequest"></a>
<a id="tocssubscriptionchangeplanrequest"></a>

```json
{
  "apiId": "weather-api-v1",
  "planId": "Gold"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|apiId|string|false|none|Developer Portal API ID the subscription belongs to. Optional — if provided, it is validated against the API derived from the existing subscription record and the request is rejected with 400 if they don't match. It is never used as a fallback: if the API cannot be derived from the subscription record, the request fails with 400 regardless of this value.|
|planId|string|true|none|Developer Portal subscription plan ID to switch to.|

<h2 id="tocS_SubscriptionResponse">SubscriptionResponse</h2>

<a id="schemasubscriptionresponse"></a>
<a id="schema_SubscriptionResponse"></a>
<a id="tocSsubscriptionresponse"></a>
<a id="tocssubscriptionresponse"></a>

```json
{
  "subscriptionId": "sub-12345",
  "apiId": "weather-api-v1",
  "subscriptionToken": "a3f1e8b2c4d6e8f0a1b3c5d7e9f10b2c4d6e8f0a1b3c5d7e9f10b2c4d6e8f0a1",
  "subscriptionPlanName": "Gold",
  "status": "ACTIVE",
  "createdBy": "alice@example.com",
  "updatedBy": "alice@example.com",
  "createdAt": "2019-08-24T14:15:22Z",
  "updatedAt": "2019-08-24T14:15:22Z"
}

```

Subscription payload.

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|subscriptionId|string|false|none|none|
|apiId|string|false|none|Developer Portal API ID.|
|subscriptionToken|string¦null|false|none|Plaintext subscription token, decrypted on every read (not just on create). Null if decryption fails (e.g. the encryption key changed since the token was stored).|
|subscriptionPlanName|string|false|none|none|
|status|string|false|none|none|
|createdBy|string|false|none|Identity of the user who created the subscription, or `deleted_user` if that user's IDP reference no longer exists. Present on single-resource GET responses and list items.|
|updatedBy|string|false|none|Identity of the user who last updated the subscription, or `deleted_user` if that user's IDP reference no longer exists. Present on single-resource GET responses only, omitted on list items.|
|createdAt|string(date-time)|false|none|none|
|updatedAt|string(date-time)|false|none|none|

#### Enumerated Values

|Property|Value|
|---|---|
|status|ACTIVE|
|status|INACTIVE|

<h2 id="tocS_ApiKeyRequest">ApiKeyRequest</h2>

<a id="schemaapikeyrequest"></a>
<a id="schema_ApiKeyRequest"></a>
<a id="tocSapikeyrequest"></a>
<a id="tocsapikeyrequest"></a>

```json
{
  "id": "weather_prod_key",
  "displayName": "Weather Prod Key",
  "subscriptionId": "sub-abc123",
  "appId": "my-weather-app",
  "expiresAt": "2026-12-31T23:59:59Z"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|id|string|true|none|none|
|displayName|string|false|none|Optional human-readable name for the key. Defaults to `id` when omitted.|
|subscriptionId|string|false|none|Optional subscription ID to associate the key with.|
|appId|string|false|none|Optional application ID to associate the key with, for analytics attribution only — it has no effect on the key's validity or authorization. Must belong to the same organization and be owned by the caller.|
|expiresAt|any|false|none|Optional ISO-8601 datetime with timezone, epoch seconds, or epoch milliseconds.|

oneOf

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|» *anonymous*|string(date-time)|false|none|none|

xor

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|» *anonymous*|number|false|none|none|

<h2 id="tocS_ApiKeyMetadataResponse">ApiKeyMetadataResponse</h2>

<a id="schemaapikeymetadataresponse"></a>
<a id="schema_ApiKeyMetadataResponse"></a>
<a id="tocSapikeymetadataresponse"></a>
<a id="tocsapikeymetadataresponse"></a>

```json
{
  "keyId": "key-12345",
  "id": "weather_prod_key",
  "displayName": "Weather Prod Key",
  "apiId": "weather-api-v1",
  "appId": "my-weather-app",
  "appDisplayName": "My Mobile App",
  "status": "ACTIVE",
  "expiresAt": "2026-12-31T23:59:59Z",
  "createdAt": "2019-08-24T14:15:22Z",
  "revokedAt": "2019-08-24T14:15:22Z"
}

```

API key metadata returned by list operations. Secret material is omitted.

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|keyId|string|false|none|Developer Portal key identifier.|
|id|string|false|none|none|
|displayName|string|false|none|none|
|apiId|string|false|none|Developer Portal API ID the key belongs to.|
|appId|string¦null|false|none|ID of the application this key is associated with, if any. Analytics attribution only.|
|appDisplayName|string¦null|false|none|Display name of the associated application, if any.|
|status|string|false|none|none|
|expiresAt|string(date-time)¦null|false|none|none|
|createdAt|string(date-time)|false|none|none|
|revokedAt|string(date-time)¦null|false|none|none|

#### Enumerated Values

|Property|Value|
|---|---|
|status|ACTIVE|
|status|REVOKED|

<h2 id="tocS_ApiKeyResponse">ApiKeyResponse</h2>

<a id="schemaapikeyresponse"></a>
<a id="schema_ApiKeyResponse"></a>
<a id="tocSapikeyresponse"></a>
<a id="tocsapikeyresponse"></a>

```json
{
  "keyId": "key-12345",
  "id": "weather_prod_key",
  "displayName": "Weather Prod Key",
  "key": "ak_dGhpcyBpcyBub3QgYSByZWFsIGtleQ",
  "expiresAt": "2026-12-31T23:59:59Z",
  "status": "ACTIVE"
}

```

API key response returned by generate/regenerate only. Unlike ApiKeyMetadataResponse, this does not include apiId, appId, appDisplayName, createdAt, or revokedAt — generate/regenerate return only these six fields.

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|keyId|string|false|none|Developer Portal key identifier.|
|id|string|false|none|none|
|displayName|string|false|none|none|
|key|string|false|none|One-time plaintext API key secret.|
|expiresAt|string(date-time)¦null|false|none|none|
|status|string|false|none|none|

#### Enumerated Values

|Property|Value|
|---|---|
|status|ACTIVE|
|status|REVOKED|

<h2 id="tocS_ApiKeyApplicationResponse">ApiKeyApplicationResponse</h2>

<a id="schemaapikeyapplicationresponse"></a>
<a id="schema_ApiKeyApplicationResponse"></a>
<a id="tocSapikeyapplicationresponse"></a>
<a id="tocsapikeyapplicationresponse"></a>

```json
{
  "keyId": "key-12345",
  "application": {
    "id": "my-weather-app",
    "displayName": "My Mobile App"
  }
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|keyId|string|false|none|none|
|application|object|false|none|none|
|» id|string|false|none|none|
|» displayName|string|false|none|none|

<h2 id="tocS_KeyManagerRequest">KeyManagerRequest</h2>

<a id="schemakeymanagerrequest"></a>
<a id="schema_KeyManagerRequest"></a>
<a id="tocSkeymanagerrequest"></a>
<a id="tocskeymanagerrequest"></a>

```json
{
  "displayName": "Asgardeo",
  "id": "asgardeo-prod",
  "enabled": true,
  "tokenEndpoint": "https://api.asgardeo.io/t/myorg/oauth2/token"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|displayName|string|true|none|none|
|id|string|false|none|Desired handle for the key manager (unique per org), stored as-is. Optional — defaults to the key manager's `displayName` when omitted.|
|enabled|boolean|false|none|none|
|tokenEndpoint|string(uri)|true|none|OAuth2 token endpoint. The OAuth application itself must be created directly in this key manager; the portal only proxies `client_appKeyMappings` token requests to this endpoint.|

<h2 id="tocS_KeyManagerUpdateRequest">KeyManagerUpdateRequest</h2>

<a id="schemakeymanagerupdaterequest"></a>
<a id="schema_KeyManagerUpdateRequest"></a>
<a id="tocSkeymanagerupdaterequest"></a>
<a id="tocskeymanagerupdaterequest"></a>

```json
{
  "displayName": "Asgardeo",
  "id": "asgardeo-prod",
  "enabled": true,
  "tokenEndpoint": "https://api.asgardeo.io/t/myorg/oauth2/token"
}

```

Partial update payload for a key manager. All fields are optional; only supplied fields are applied. Omitted fields retain their stored values.

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|displayName|string|false|none|none|
|id|string|false|none|Desired handle for the key manager (unique per org), stored as-is.|
|enabled|boolean|false|none|none|
|tokenEndpoint|string(uri)|false|none|none|

<h2 id="tocS_KeyManagerResponseSchema">KeyManagerResponseSchema</h2>

<a id="schemakeymanagerresponseschema"></a>
<a id="schema_KeyManagerResponseSchema"></a>
<a id="tocSkeymanagerresponseschema"></a>
<a id="tocskeymanagerresponseschema"></a>

```json
{
  "id": "asgardeo-prod",
  "displayName": "Asgardeo",
  "orgId": "org-12345",
  "enabled": true,
  "tokenEndpoint": "https://api.asgardeo.io/t/myorg/oauth2/token",
  "createdBy": "alice@example.com",
  "updatedBy": "alice@example.com",
  "createdAt": "2019-08-24T14:15:22Z",
  "updatedAt": "2019-08-24T14:15:22Z"
}

```

Key manager configuration.

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|id|string|false|none|The key manager's handle (unique per org). Not the internal database uuid.|
|displayName|string|false|none|none|
|orgId|string|false|none|none|
|enabled|boolean|false|none|none|
|tokenEndpoint|string(uri)|false|none|none|
|createdBy|string|false|none|Identity of the user who created this key manager, or `deleted_user` if that user's IDP reference no longer exists. Present on single-resource GET responses and list items.|
|updatedBy|string|false|none|Identity of the user who last updated this key manager, or `deleted_user` if that user's IDP reference no longer exists. Present on single-resource GET responses only, omitted on list items.|
|createdAt|string(date-time)|false|none|none|
|updatedAt|string(date-time)|false|none|none|

<h2 id="tocS_KeyManagerPublicResponseSchema">KeyManagerPublicResponseSchema</h2>

<a id="schemakeymanagerpublicresponseschema"></a>
<a id="schema_KeyManagerPublicResponseSchema"></a>
<a id="tocSkeymanagerpublicresponseschema"></a>
<a id="tocskeymanagerpublicresponseschema"></a>

```json
{
  "id": "asgardeo-prod",
  "displayName": "Asgardeo",
  "tokenEndpoint": "https://api.asgardeo.io/t/myorg/oauth2/token"
}

```

Minimal developer-facing key manager view.

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|id|string|false|none|The key manager's handle (unique per org). Not the internal database uuid.|
|displayName|string|false|none|none|
|tokenEndpoint|string(uri)|false|none|none|

<h2 id="tocS_WebhookSubscriberRequest">WebhookSubscriberRequest</h2>

<a id="schemawebhooksubscriberrequest"></a>
<a id="schema_WebhookSubscriberRequest"></a>
<a id="tocSwebhooksubscriberrequest"></a>
<a id="tocswebhooksubscriberrequest"></a>

```json
{
  "id": "production-gateway",
  "displayName": "Production Gateway",
  "targetUrl": "https://gateway.example.com/devportal-webhook",
  "secret": "<shared-secret>",
  "publicKey": "string",
  "events": [
    "apikey.*",
    "subscription.*"
  ],
  "enabled": true,
  "timeoutMs": 5000
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|id|string|true|none|Desired handle for the webhook subscriber (unique per org), stored as-is.|
|displayName|string|false|none|Optional display name. Defaults to the handle when omitted.|
|targetUrl|string(uri)|true|none|Target URL events are POSTed to. Must be unique within the organization.|
|secret|string|false|none|Shared secret used to sign outgoing payloads (HMAC). Stored encrypted; never returned in responses.|
|publicKey|string|false|none|PEM-encoded public key. When set, secret event payloads (apikey.*, subscription.*) are additionally encrypted to this key so only the subscriber can read the plaintext key.|
|events|[string]|false|none|Glob-style event type allowlist (only a trailing `*` wildcard is supported, e.g. `apikey.*`). Omit or leave empty to receive all event types.|
|enabled|boolean|false|none|none|
|timeoutMs|integer|false|none|none|

<h2 id="tocS_WebhookSubscriberResponseSchema">WebhookSubscriberResponseSchema</h2>

<a id="schemawebhooksubscriberresponseschema"></a>
<a id="schema_WebhookSubscriberResponseSchema"></a>
<a id="tocSwebhooksubscriberresponseschema"></a>
<a id="tocswebhooksubscriberresponseschema"></a>

```json
{
  "id": "production-gateway",
  "orgId": "org-12345",
  "displayName": "Production Gateway",
  "targetUrl": "https://gateway.example.com/devportal-webhook",
  "enabled": true,
  "events": [
    "apikey.*",
    "subscription.*"
  ],
  "timeoutMs": 5000,
  "hasSecret": true,
  "hasPublicKey": false,
  "createdBy": "alice@example.com",
  "updatedBy": "alice@example.com",
  "createdAt": "2019-08-24T14:15:22Z",
  "updatedAt": "2019-08-24T14:15:22Z"
}

```

Webhook subscriber configuration. The secret is never included.

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|id|string|false|none|The webhook subscriber's handle (unique per org). Not the internal database uuid.|
|orgId|string|false|none|none|
|displayName|string|false|none|none|
|targetUrl|string(uri)|false|none|none|
|enabled|boolean|false|none|none|
|events|[string]|false|none|none|
|timeoutMs|integer|false|none|none|
|hasSecret|boolean|false|none|Whether a secret is configured for HMAC-signing outgoing payloads.|
|hasPublicKey|boolean|false|none|Whether a public key is configured for envelope-encrypting secret event payloads.|
|createdBy|string|false|none|Identity of the user who created this webhook subscriber, or `deleted_user` if that user's IDP reference no longer exists. Present on single-resource GET responses and list items.|
|updatedBy|string|false|none|Identity of the user who last updated this webhook subscriber, or `deleted_user` if that user's IDP reference no longer exists. Present on single-resource GET responses only, omitted on list items.|
|createdAt|string(date-time)|false|none|none|
|updatedAt|string(date-time)|false|none|none|

<h2 id="tocS_WebhookSubscriberDeliverySummary">WebhookSubscriberDeliverySummary</h2>

<a id="schemawebhooksubscriberdeliverysummary"></a>
<a id="schema_WebhookSubscriberDeliverySummary"></a>
<a id="tocSwebhooksubscriberdeliverysummary"></a>
<a id="tocswebhooksubscriberdeliverysummary"></a>

```json
{
  "deliveryId": "del-abc123",
  "eventType": "apikey.generated",
  "occurredAt": "2019-08-24T14:15:22Z",
  "status": "DELIVERED",
  "lastHttpStatus": 200,
  "lastError": "string",
  "lastAttemptAt": "2019-08-24T14:15:22Z",
  "deliveredAt": "2019-08-24T14:15:22Z"
}

```

A single delivery attempt made to a webhook subscriber.

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|deliveryId|string|false|none|none|
|eventType|string¦null|false|none|none|
|occurredAt|string(date-time)¦null|false|none|none|
|status|string|false|none|none|
|lastHttpStatus|integer¦null|false|none|none|
|lastError|string¦null|false|none|none|
|lastAttemptAt|string(date-time)¦null|false|none|none|
|deliveredAt|string(date-time)¦null|false|none|none|

#### Enumerated Values

|Property|Value|
|---|---|
|status|PENDING|
|status|IN_FLIGHT|
|status|DELIVERED|
|status|FAILED|

<h2 id="tocS_AppKeyMappingRequest">AppKeyMappingRequest</h2>

<a id="schemaappkeymappingrequest"></a>
<a id="schema_AppKeyMappingRequest"></a>
<a id="tocSappkeymappingrequest"></a>
<a id="tocsappkeymappingrequest"></a>

```json
{
  "keyManager": "Resident Key Manager",
  "type": "PRODUCTION",
  "consumerKey": "consumer-key-123"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|keyManager|string|true|none|none|
|type|string|false|none|none|
|consumerKey|string|true|none|The OAuth client_id, created directly in the key manager. The portal does not store or persist the client secret — it is supplied per-request when generating a token and is only seen transiently during that request.|

#### Enumerated Values

|Property|Value|
|---|---|
|type|PRODUCTION|
|type|SANDBOX|

<h2 id="tocS_ViewCreateRequest">ViewCreateRequest</h2>

<a id="schemaviewcreaterequest"></a>
<a id="schema_ViewCreateRequest"></a>
<a id="tocSviewcreaterequest"></a>
<a id="tocsviewcreaterequest"></a>

```json
{
  "id": "partner-apis",
  "displayName": "Partner APIs",
  "labels": [
    "partner",
    "public"
  ]
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|id|string|true|none|Desired handle for the view (unique per org), stored as-is.|
|displayName|string|false|none|Optional display name. Defaults to the handle when omitted.|
|labels|[string]|true|none|Label names to attach to the view.|

<h2 id="tocS_ViewUpdateRequest">ViewUpdateRequest</h2>

<a id="schemaviewupdaterequest"></a>
<a id="schema_ViewUpdateRequest"></a>
<a id="tocSviewupdaterequest"></a>
<a id="tocsviewupdaterequest"></a>

```json
{
  "displayName": "Partner and Public APIs",
  "labels": [
    "partner",
    "premium"
  ]
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|displayName|string|false|none|none|
|labels|[string]|false|none|Full desired set of label names for the view. Labels present here but not currently attached are attached; labels currently attached but absent here are detached. Omit to leave labels unchanged.|

<h2 id="tocS_OAuthGenerateTokenRequest">OAuthGenerateTokenRequest</h2>

<a id="schemaoauthgeneratetokenrequest"></a>
<a id="schema_OAuthGenerateTokenRequest"></a>
<a id="tocSoauthgeneratetokenrequest"></a>
<a id="tocsoauthgeneratetokenrequest"></a>

```json
{
  "consumerSecret": "my-consumer-secret",
  "scopes": [
    "weather.read"
  ],
  "validityPeriod": 3600
}

```

OAuth access token generation payload. `consumerSecret` is required — the portal uses it to call the Authorization Server token endpoint directly.

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|consumerSecret|string|true|none|Client secret for the OAuth application. Not stored by the portal — the caller must supply it on each token generation request.|
|scopes|[string]|false|none|none|
|validityPeriod|integer|false|none|none|

<h2 id="tocS_ApplicationOAuthKeyResponse">ApplicationOAuthKeyResponse</h2>

<a id="schemaapplicationoauthkeyresponse"></a>
<a id="schema_ApplicationOAuthKeyResponse"></a>
<a id="tocSapplicationoauthkeyresponse"></a>
<a id="tocsapplicationoauthkeyresponse"></a>

```json
{
  "keyMappingId": "km-12345",
  "keyManager": "Resident Key Manager",
  "type": "PRODUCTION",
  "consumerKey": "consumer-key-123",
  "tokenEndpoint": "https://api.asgardeo.io/t/myorg/oauth2/token"
}

```

OAuth key mapping payload.

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|keyMappingId|string|false|none|none|
|keyManager|string|false|none|none|
|type|string|false|none|none|
|consumerKey|string|false|none|none|
|tokenEndpoint|string(uri)|false|none|none|

<h2 id="tocS_OAuthTokenResponse">OAuthTokenResponse</h2>

<a id="schemaoauthtokenresponse"></a>
<a id="schema_OAuthTokenResponse"></a>
<a id="tocSoauthtokenresponse"></a>
<a id="tocsoauthtokenresponse"></a>

```json
{
  "accessToken": "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.example",
  "validityTime": 3600,
  "tokenScopes": [
    "weather.read"
  ]
}

```

Access token response proxied from the key manager's token endpoint. Field names are the portal's own camelCase, not the underlying OAuth2 token response's snake_case.

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|accessToken|string|false|none|none|
|validityTime|integer¦null|false|none|Token lifetime in seconds, as reported by the key manager (`expires_in`).|
|tokenScopes|[string]|false|none|none|

<h2 id="tocS_APIWorkflowCreateResponse">APIWorkflowCreateResponse</h2>

<a id="schemaapiworkflowcreateresponse"></a>
<a id="schema_APIWorkflowCreateResponse"></a>
<a id="tocSapiworkflowcreateresponse"></a>
<a id="tocsapiworkflowcreateresponse"></a>

```json
{
  "id": "workflow-12345",
  "displayName": "Weather onboarding",
  "status": "PUBLISHED"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|id|string|false|none|none|
|displayName|string|false|none|none|
|status|string|false|none|none|

#### Enumerated Values

|Property|Value|
|---|---|
|status|DRAFT|
|status|PUBLISHED|

<h2 id="tocS_APIWorkflowResponse">APIWorkflowResponse</h2>

<a id="schemaapiworkflowresponse"></a>
<a id="schema_APIWorkflowResponse"></a>
<a id="tocSapiworkflowresponse"></a>
<a id="tocsapiworkflowresponse"></a>

```json
{
  "id": "workflow-12345",
  "displayName": "Weather onboarding",
  "description": "string",
  "agentPrompt": "string",
  "status": "PUBLISHED",
  "agentVisibility": "VISIBLE",
  "contentType": "ARAZZO",
  "apiWorkflowDefinition": "string",
  "markdownContent": "string",
  "createdAt": "May 7, 2026",
  "updatedAt": "string",
  "createdBy": "string",
  "updatedBy": "string"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|id|string|false|none|The workflow's handle (unique per org and view). Not the internal database uuid.|
|displayName|string|false|none|none|
|description|string|false|none|none|
|agentPrompt|string|false|none|none|
|status|string|false|none|none|
|agentVisibility|string|false|none|none|
|contentType|string|false|none|none|
|apiWorkflowDefinition|string¦null|false|none|none|
|markdownContent|string¦null|false|none|none|
|createdAt|string|false|none|none|
|updatedAt|string¦null|false|none|none|
|createdBy|string¦null|false|none|none|
|updatedBy|string¦null|false|none|none|

#### Enumerated Values

|Property|Value|
|---|---|
|status|DRAFT|
|status|PUBLISHED|
|agentVisibility|VISIBLE|
|agentVisibility|HIDDEN|
|contentType|ARAZZO|
|contentType|MD|

<h2 id="tocS_APIWorkflowPromptResponse">APIWorkflowPromptResponse</h2>

<a id="schemaapiworkflowpromptresponse"></a>
<a id="schema_APIWorkflowPromptResponse"></a>
<a id="tocSapiworkflowpromptresponse"></a>
<a id="tocsapiworkflowpromptresponse"></a>

```json
{
  "agentPrompt": "string"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|agentPrompt|string|false|none|none|

<h2 id="tocS_APIWorkflowCreateRequest">APIWorkflowCreateRequest</h2>

<a id="schemaapiworkflowcreaterequest"></a>
<a id="schema_APIWorkflowCreateRequest"></a>
<a id="tocSapiworkflowcreaterequest"></a>
<a id="tocsapiworkflowcreaterequest"></a>

```json
{
  "displayName": "Weather onboarding",
  "id": "weather-onboarding",
  "description": "Guides users through the Weather API onboarding workflow.",
  "agentPrompt": "Follow this workflow to onboard a Weather API user.",
  "status": "PUBLISHED",
  "agentVisibility": "VISIBLE",
  "contentType": "ARAZZO",
  "apiWorkflowDefinition": {},
  "markdownContent": "string"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|displayName|string|true|none|none|
|id|string|false|none|Desired handle for the workflow (unique per org and view), stored as-is.|
|description|string|true|none|none|
|agentPrompt|string|false|none|none|
|status|string|false|none|none|
|agentVisibility|string|false|none|none|
|contentType|string|false|none|none|
|apiWorkflowDefinition|any|false|none|JSON/YAML Arazzo content when `contentType` is `ARAZZO`.|

oneOf

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|» *anonymous*|object|false|none|none|

xor

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|» *anonymous*|string|false|none|none|

continued

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|markdownContent|string|false|none|Markdown content when `contentType` is `MD`.|

#### Enumerated Values

|Property|Value|
|---|---|
|status|DRAFT|
|status|PUBLISHED|
|agentVisibility|VISIBLE|
|agentVisibility|HIDDEN|
|contentType|ARAZZO|
|contentType|MD|

<h2 id="tocS_APIWorkflowUpdateRequest">APIWorkflowUpdateRequest</h2>

<a id="schemaapiworkflowupdaterequest"></a>
<a id="schema_APIWorkflowUpdateRequest"></a>
<a id="tocSapiworkflowupdaterequest"></a>
<a id="tocsapiworkflowupdaterequest"></a>

```json
{
  "displayName": "Weather onboarding v2",
  "id": "weather-onboarding-v2",
  "description": "Updated Weather API onboarding workflow.",
  "agentPrompt": "string",
  "status": "PUBLISHED",
  "agentVisibility": "VISIBLE",
  "contentType": "ARAZZO",
  "apiWorkflowDefinition": {},
  "markdownContent": "string"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|displayName|string|false|none|none|
|id|string|false|none|Desired handle for the workflow (unique per org and view), stored as-is.|
|description|string|false|none|none|
|agentPrompt|string|false|none|none|
|status|string|false|none|none|
|agentVisibility|string|false|none|none|
|contentType|string|false|none|none|
|apiWorkflowDefinition|any|false|none|none|

oneOf

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|» *anonymous*|object|false|none|none|

xor

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|» *anonymous*|string|false|none|none|

continued

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|markdownContent|string|false|none|none|

#### Enumerated Values

|Property|Value|
|---|---|
|status|DRAFT|
|status|PUBLISHED|
|agentVisibility|VISIBLE|
|agentVisibility|HIDDEN|
|contentType|ARAZZO|
|contentType|MD|

<h2 id="tocS_APIWorkflowPromptRequest">APIWorkflowPromptRequest</h2>

<a id="schemaapiworkflowpromptrequest"></a>
<a id="schema_APIWorkflowPromptRequest"></a>
<a id="tocSapiworkflowpromptrequest"></a>
<a id="tocsapiworkflowpromptrequest"></a>

```json
{
  "displayName": "Weather onboarding",
  "description": "Guides users through the Weather API onboarding workflow.",
  "apis": [
    {}
  ],
  "orgHandle": "acme",
  "viewName": "default",
  "id": "weather-onboarding"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|displayName|string|true|none|none|
|description|string|true|none|none|
|apis|[object]|false|none|none|
|orgHandle|string|false|none|none|
|viewName|string|false|none|none|
|id|string|false|none|The workflow's (would-be) handle, used only to build the workflow detail URL referenced in the generated prompt.|

<h2 id="tocS_WebhookEventDelivery">WebhookEventDelivery</h2>

<a id="schemawebhookeventdelivery"></a>
<a id="schema_WebhookEventDelivery"></a>
<a id="tocSwebhookeventdelivery"></a>
<a id="tocswebhookeventdelivery"></a>

```json
{
  "deliveryId": "del-abc123",
  "subscriberId": "sub-xyz789",
  "targetUrl": "https://example.com/webhook",
  "status": "DELIVERED",
  "lastHttpStatus": 200,
  "lastError": "string",
  "lastAttemptAt": "2019-08-24T14:15:22Z",
  "deliveredAt": "2019-08-24T14:15:22Z"
}

```

A single webhook delivery attempt.

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|deliveryId|string|false|none|none|
|subscriberId|string|false|none|none|
|targetUrl|string¦null|false|none|none|
|status|string|false|none|none|
|lastHttpStatus|integer¦null|false|none|none|
|lastError|string¦null|false|none|none|
|lastAttemptAt|string(date-time)¦null|false|none|none|
|deliveredAt|string(date-time)¦null|false|none|none|

#### Enumerated Values

|Property|Value|
|---|---|
|status|PENDING|
|status|IN_FLIGHT|
|status|DELIVERED|
|status|FAILED|

<h2 id="tocS_WebhookEvent">WebhookEvent</h2>

<a id="schemawebhookevent"></a>
<a id="schema_WebhookEvent"></a>
<a id="tocSwebhookevent"></a>
<a id="tocswebhookevent"></a>

```json
{
  "eventId": "evt-abc123",
  "eventType": "apikey.generated",
  "orgId": "org-default",
  "aggregateType": "apikey",
  "aggregateId": "key-12345",
  "status": "ALL_DELIVERED",
  "occurredAt": "2019-08-24T14:15:22Z",
  "deliveries": [
    {
      "deliveryId": "del-abc123",
      "subscriberId": "sub-xyz789",
      "targetUrl": "https://example.com/webhook",
      "status": "DELIVERED",
      "lastHttpStatus": 200,
      "lastError": "string",
      "lastAttemptAt": "2019-08-24T14:15:22Z",
      "deliveredAt": "2019-08-24T14:15:22Z"
    }
  ]
}

```

A webhook event with its delivery rows.

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|eventId|string|false|none|none|
|eventType|string|false|none|none|
|orgId|string|false|none|none|
|aggregateType|string|false|none|none|
|aggregateId|string|false|none|none|
|status|string|false|none|none|
|occurredAt|string(date-time)|false|none|none|
|deliveries|[[WebhookEventDelivery](#schemawebhookeventdelivery)]|false|none|[A single webhook delivery attempt.]|

#### Enumerated Values

|Property|Value|
|---|---|
|status|PENDING|
|status|DISPATCHED|
|status|ALL_DELIVERED|
|status|FAILED|
