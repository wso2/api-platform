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
  "orgId": "string",
  "orgName": "string",
  "businessOwner": "string",
  "businessOwnerContact": "string",
  "businessOwnerEmail": "user@example.com",
  "orgHandle": "string",
  "organizationIdentifier": "string",
  "orgConfiguration": {}
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|orgId|string|false|none|none|
|orgName|string|false|none|none|
|businessOwner|string¦null|false|none|none|
|businessOwnerContact|string¦null|false|none|none|
|businessOwnerEmail|string(email)¦null|false|none|none|
|orgHandle|string|false|none|none|
|organizationIdentifier|string|false|none|none|
|orgConfiguration|[GenericObject](#schemagenericobject)|false|none|none|

<h2 id="tocS_OrganizationListItemResponse">OrganizationListItemResponse</h2>

<a id="schemaorganizationlistitemresponse"></a>
<a id="schema_OrganizationListItemResponse"></a>
<a id="tocSorganizationlistitemresponse"></a>
<a id="tocsorganizationlistitemresponse"></a>

```json
{
  "orgID": "string",
  "orgName": "string",
  "businessOwner": "string",
  "businessOwnerContact": "string",
  "businessOwnerEmail": "user@example.com",
  "orgHandle": "string",
  "organizationIdentifier": "string",
  "orgConfiguration": {}
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|orgID|string|false|none|none|
|orgName|string|false|none|none|
|businessOwner|string¦null|false|none|none|
|businessOwnerContact|string¦null|false|none|none|
|businessOwnerEmail|string(email)¦null|false|none|none|
|orgHandle|string|false|none|none|
|organizationIdentifier|string|false|none|none|
|orgConfiguration|[GenericObject](#schemagenericobject)|false|none|none|

<h2 id="tocS_OrganizationContentUploadResponse">OrganizationContentUploadResponse</h2>

<a id="schemaorganizationcontentuploadresponse"></a>
<a id="schema_OrganizationContentUploadResponse"></a>
<a id="tocSorganizationcontentuploadresponse"></a>
<a id="tocsorganizationcontentuploadresponse"></a>

```json
{
  "orgId": "string",
  "fileName": "string"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|orgId|string|true|none|none|
|fileName|string|true|none|Original ZIP file name uploaded in the `file` multipart field.|

<h2 id="tocS_OrganizationContentListItemResponse">OrganizationContentListItemResponse</h2>

<a id="schemaorganizationcontentlistitemresponse"></a>
<a id="schema_OrganizationContentListItemResponse"></a>
<a id="tocSorganizationcontentlistitemresponse"></a>
<a id="tocsorganizationcontentlistitemresponse"></a>

```json
{
  "orgId": "string",
  "fileName": "string",
  "fileContent": "string"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|orgId|string|false|none|none|
|fileName|string|false|none|none|
|fileContent|string¦null|false|none|UTF-8 content string returned for stored organization content records.|

<h2 id="tocS_ApiMetadataCreateResponse">ApiMetadataCreateResponse</h2>

<a id="schemaapimetadatacreateresponse"></a>
<a id="schema_ApiMetadataCreateResponse"></a>
<a id="tocSapimetadatacreateresponse"></a>
<a id="tocsapimetadatacreateresponse"></a>

```json
{
  "apiID": "string",
  "apiReferenceID": "string",
  "apiHandle": "string",
  "dataSource": "string",
  "apiInfo": {
    "apiName": "string",
    "apiTitle": "string",
    "remotes": [
      {}
    ],
    "apiVersion": "string",
    "apiStatus": "PUBLISHED",
    "apiDescription": "string",
    "apiType": "string",
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
  },
  "endPoints": {
    "sandboxURL": "string",
    "productionURL": "string"
  },
  "subscriptionPlans": [
    {
      "planID": "string",
      "planName": "string",
      "displayName": "string",
      "description": "string",
      "requestCount": 0,
      "refId": "string",
      "orgID": "string"
    }
  ]
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|apiID|string|false|none|none|
|apiReferenceID|string|false|none|none|
|apiHandle|string|false|none|none|
|dataSource|string|false|none|none|
|apiInfo|[ApiInfoResponse](#schemaapiinforesponse)|false|none|none|
|endPoints|[ApiEndpointsResponse](#schemaapiendpointsresponse)|false|none|none|
|subscriptionPlans|[[SubscriptionPlanResponse](#schemasubscriptionplanresponse)]|false|none|none|

<h2 id="tocS_ApiMetadataResponse">ApiMetadataResponse</h2>

<a id="schemaapimetadataresponse"></a>
<a id="schema_ApiMetadataResponse"></a>
<a id="tocSapimetadataresponse"></a>
<a id="tocsapimetadataresponse"></a>

```json
{
  "apiID": "string",
  "apiReferenceID": "string",
  "apiHandle": "string",
  "dataSource": "string",
  "planID": "string",
  "apiInfo": {
    "apiName": "string",
    "apiTitle": "string",
    "remotes": [
      {}
    ],
    "apiVersion": "string",
    "apiStatus": "PUBLISHED",
    "apiDescription": "string",
    "apiType": "string",
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
  },
  "endPoints": {
    "sandboxURL": "string",
    "productionURL": "string"
  },
  "subscriptionPlans": [
    {
      "planID": "string",
      "planName": "string",
      "displayName": "string",
      "description": "string",
      "requestCount": 0,
      "refId": "string",
      "orgID": "string"
    }
  ]
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|apiID|string|false|none|none|
|apiReferenceID|string|false|none|none|
|apiHandle|string|false|none|none|
|dataSource|string|false|none|none|
|planID|string|false|none|none|
|apiInfo|[ApiInfoResponse](#schemaapiinforesponse)|false|none|none|
|endPoints|[ApiEndpointsResponse](#schemaapiendpointsresponse)|false|none|none|
|subscriptionPlans|[[SubscriptionPlanResponse](#schemasubscriptionplanresponse)]|false|none|none|

<h2 id="tocS_ApiInfoResponse">ApiInfoResponse</h2>

<a id="schemaapiinforesponse"></a>
<a id="schema_ApiInfoResponse"></a>
<a id="tocSapiinforesponse"></a>
<a id="tocsapiinforesponse"></a>

```json
{
  "apiName": "string",
  "apiTitle": "string",
  "remotes": [
    {}
  ],
  "apiVersion": "string",
  "apiStatus": "PUBLISHED",
  "apiDescription": "string",
  "apiType": "string",
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

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|apiName|string|false|none|none|
|apiTitle|string¦null|false|none|none|
|remotes|[object]|false|none|none|
|apiVersion|string|false|none|none|
|apiStatus|string|false|none|API lifecycle status.|
|apiDescription|string|false|none|none|
|apiType|string|false|none|none|
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
|apiStatus|PUBLISHED|
|apiStatus|DEPRECATED|
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
  "planID": "string",
  "planName": "string",
  "displayName": "string",
  "description": "string",
  "requestCount": 0,
  "refId": "string",
  "orgID": "string"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|planID|string|false|none|none|
|planName|string|false|none|none|
|displayName|string|false|none|none|
|description|string|false|none|none|
|requestCount|any|false|none|none|

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
|refId|string¦null|false|none|Platform API subscription plan UUID associated with this plan.|
|orgID|string|false|none|none|

<h2 id="tocS_LabelResponse">LabelResponse</h2>

<a id="schemalabelresponse"></a>
<a id="schema_LabelResponse"></a>
<a id="tocSlabelresponse"></a>
<a id="tocslabelresponse"></a>

```json
{
  "name": "premium",
  "displayName": "Premium APIs"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|name|string|false|none|none|
|displayName|string|false|none|none|

<h2 id="tocS_ApplicationResponse">ApplicationResponse</h2>

<a id="schemaapplicationresponse"></a>
<a id="schema_ApplicationResponse"></a>
<a id="tocSapplicationresponse"></a>
<a id="tocsapplicationresponse"></a>

```json
{
  "id": "app-12345",
  "name": "Weather App",
  "description": "Application used to call Weather APIs.",
  "type": "WEB",
  "appMap": [
    {
      "appRefID": "asgardeo-client-abc123",
      "kmID": "km-uuid-12345",
      "keyType": "PRODUCTION"
    }
  ]
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|id|string|false|none|none|
|name|string|false|none|none|
|description|string|false|none|none|
|type|string|false|none|none|
|appMap|[[ApplicationKeyMappingSummary](#schemaapplicationkeymappingsummary)]|false|none|[OAuth client ID mapping entry attached to an application.]|

<h2 id="tocS_ApplicationKeyMappingSummary">ApplicationKeyMappingSummary</h2>

<a id="schemaapplicationkeymappingsummary"></a>
<a id="schema_ApplicationKeyMappingSummary"></a>
<a id="tocSapplicationkeymappingsummary"></a>
<a id="tocsapplicationkeymappingsummary"></a>

```json
{
  "appRefID": "asgardeo-client-abc123",
  "kmID": "km-uuid-12345",
  "keyType": "PRODUCTION"
}

```

OAuth client ID mapping entry attached to an application.

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|appRefID|string|false|none|OAuth client ID, created directly in the key manager and linked to this application.|
|kmID|string|false|none|UUID of the key manager this client ID is linked to.|
|keyType|string|false|none|Key type for this mapping.|

#### Enumerated Values

|Property|Value|
|---|---|
|keyType|PRODUCTION|
|keyType|SANDBOX|

<h2 id="tocS_ViewResponse">ViewResponse</h2>

<a id="schemaviewresponse"></a>
<a id="schema_ViewResponse"></a>
<a id="tocSviewresponse"></a>
<a id="tocsviewresponse"></a>

```json
{
  "handle": "partner-apis",
  "name": "Partner APIs",
  "labels": [
    "partner",
    "public"
  ]
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|handle|string|true|none|none|
|name|string|true|none|none|
|labels|[string]|true|none|none|

<h2 id="tocS_OrganizationCreateRequest">OrganizationCreateRequest</h2>

<a id="schemaorganizationcreaterequest"></a>
<a id="schema_OrganizationCreateRequest"></a>
<a id="tocSorganizationcreaterequest"></a>
<a id="tocsorganizationcreaterequest"></a>

```json
{
  "orgName": "string",
  "businessOwner": "string",
  "businessOwnerContact": "string",
  "businessOwnerEmail": "user@example.com",
  "orgHandle": "string",
  "organizationIdentifier": "string"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|orgName|string|true|none|none|
|businessOwner|string|false|none|none|
|businessOwnerContact|string|false|none|none|
|businessOwnerEmail|string(email)|false|none|none|
|orgHandle|string|true|none|Public organization handle used in portal URLs.|
|organizationIdentifier|string|true|none|none|

<h2 id="tocS_OrganizationUpdateRequest">OrganizationUpdateRequest</h2>

<a id="schemaorganizationupdaterequest"></a>
<a id="schema_OrganizationUpdateRequest"></a>
<a id="tocSorganizationupdaterequest"></a>
<a id="tocsorganizationupdaterequest"></a>

```json
{
  "orgName": "string",
  "businessOwner": "string",
  "businessOwnerContact": "string",
  "businessOwnerEmail": "user@example.com",
  "orgHandle": "string",
  "organizationIdentifier": "string",
  "orgConfiguration": {
    "devportalMode": "DEFAULT"
  }
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|orgName|string|true|none|none|
|businessOwner|string|false|none|none|
|businessOwnerContact|string|false|none|none|
|businessOwnerEmail|string(email)|false|none|none|
|orgHandle|string|true|none|none|
|organizationIdentifier|string|true|none|none|
|orgConfiguration|object|false|none|none|
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
  "planID": "string",
  "refId": "string",
  "planName": "string",
  "displayName": "string",
  "description": "string",
  "type": "requestcount",
  "requestCount": 0,
  "eventCount": 0
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|planID|string|false|none|Optional external/APIM-assigned plan UUID.|
|refId|string|false|none|Platform API subscription plan UUID to associate with this plan.|
|planName|string|true|none|none|
|displayName|string|true|none|none|
|description|string|false|none|none|
|type|string|true|none|Service accepts case-insensitive `requestcount` or `eventcount`.|
|requestCount|any|false|none|Required for request-count plans. Use -1 for unlimited.|

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
|eventCount|any|false|none|Required for event-count plans. Use -1 for unlimited.|

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
|type|requestcount|
|type|eventcount|

<h2 id="tocS_LabelRequest">LabelRequest</h2>

<a id="schemalabelrequest"></a>
<a id="schema_LabelRequest"></a>
<a id="tocSlabelrequest"></a>
<a id="tocslabelrequest"></a>

```json
{
  "name": "premium",
  "displayName": "Premium APIs"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|name|string|true|none|none|
|displayName|string|true|none|none|

<h2 id="tocS_ApplicationRequest">ApplicationRequest</h2>

<a id="schemaapplicationrequest"></a>
<a id="schema_ApplicationRequest"></a>
<a id="tocSapplicationrequest"></a>
<a id="tocsapplicationrequest"></a>

```json
{
  "name": "Weather App",
  "description": "Application used to call Weather APIs.",
  "type": "WEB"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|name|string|true|none|none|
|description|string|true|none|none|
|type|string|false|none|none|

<h2 id="tocS_SubscriptionCreateRequest">SubscriptionCreateRequest</h2>

<a id="schemasubscriptioncreaterequest"></a>
<a id="schema_SubscriptionCreateRequest"></a>
<a id="tocSsubscriptioncreaterequest"></a>
<a id="tocssubscriptioncreaterequest"></a>

```json
{
  "apiId": "api-7f4c2a6b",
  "subscriptionPlanId": "plan-7f4c2a6b"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|apiId|string|true|none|Developer Portal API ID.|
|subscriptionPlanId|string|false|none|Developer Portal subscription plan ID.|

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

<h2 id="tocS_SubscriptionResponse">SubscriptionResponse</h2>

<a id="schemasubscriptionresponse"></a>
<a id="schema_SubscriptionResponse"></a>
<a id="tocSsubscriptionresponse"></a>
<a id="tocssubscriptionresponse"></a>

```json
{
  "subscriptionId": "sub-12345",
  "apiId": "api-7f4c2a6b",
  "subscriptionToken": "a3f1...",
  "subscriptionPlanName": "Gold",
  "status": "ACTIVE",
  "createdBy": "alice@example.com",
  "createdAt": "2019-08-24T14:15:22Z"
}

```

Subscription payload.

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|subscriptionId|string|false|none|none|
|apiId|string|false|none|Developer Portal API ID.|
|subscriptionToken|string|false|none|Plaintext subscription token. Present on create and when the token has not been encrypted at rest.|
|subscriptionPlanName|string|false|none|none|
|status|string|false|none|none|
|createdBy|string|false|none|Identity (sub claim) of the user who created the subscription.|
|createdAt|string(date-time)|false|none|none|

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
  "apiId": "api-7f4c2a6b",
  "name": "weather_prod_key",
  "subscriptionId": "sub-abc123",
  "appId": "app-12345",
  "expiresAt": "2026-12-31T23:59:59Z"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|apiId|string|true|none|Developer Portal API ID.|
|name|string|true|none|none|
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
  "name": "weather_prod_key",
  "apiId": "api-7f4c2a6b",
  "appId": "app-12345",
  "appName": "My Mobile App",
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
|name|string|false|none|none|
|apiId|string|false|none|Developer Portal API ID the key belongs to.|
|appId|string¦null|false|none|ID of the application this key is associated with, if any. Analytics attribution only.|
|appName|string¦null|false|none|Name of the associated application, if any.|
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
  "name": "weather_prod_key",
  "apiId": "api-7f4c2a6b",
  "appId": "app-12345",
  "appName": "My Mobile App",
  "status": "ACTIVE",
  "expiresAt": "2026-12-31T23:59:59Z",
  "createdAt": "2019-08-24T14:15:22Z",
  "revokedAt": "2019-08-24T14:15:22Z",
  "key": "ak_dGhpcyBpcyBub3QgYSByZWFsIGtleQ"
}

```

### Properties

allOf

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|[ApiKeyMetadataResponse](#schemaapikeymetadataresponse)|false|none|API key metadata returned by list operations. Secret material is omitted.|

and

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|object|false|none|API key response returned by generate/regenerate. The plaintext secret is returned exactly once and never persisted — store it securely.|
|» key|string|false|none|One-time plaintext API key secret.|

<h2 id="tocS_ApiKeyApplicationResponse">ApiKeyApplicationResponse</h2>

<a id="schemaapikeyapplicationresponse"></a>
<a id="schema_ApiKeyApplicationResponse"></a>
<a id="tocSapikeyapplicationresponse"></a>
<a id="tocsapikeyapplicationresponse"></a>

```json
{
  "keyId": "key-12345",
  "application": {
    "id": "app-12345",
    "name": "My Mobile App"
  }
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|keyId|string|false|none|none|
|application|object|false|none|none|
|» id|string|false|none|none|
|» name|string|false|none|none|

<h2 id="tocS_KeyManagerRequest">KeyManagerRequest</h2>

<a id="schemakeymanagerrequest"></a>
<a id="schema_KeyManagerRequest"></a>
<a id="tocSkeymanagerrequest"></a>
<a id="tocskeymanagerrequest"></a>

```json
{
  "name": "Asgardeo",
  "type": "ASGARDEO",
  "enabled": true,
  "tokenEndpoint": "https://api.asgardeo.io/t/myorg/oauth2/token"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|name|string|true|none|Unique name within the organization.|
|type|string|true|none|none|
|enabled|boolean|false|none|none|
|tokenEndpoint|string(uri)|true|none|OAuth2 token endpoint. The OAuth application itself must be created directly in this key manager; the portal only proxies `client_credentials` token requests to this endpoint.|

#### Enumerated Values

|Property|Value|
|---|---|
|type|ASGARDEO|
|type|WSO2IS|
|type|KEYCLOAK|
|type|GENERIC_OIDC|

<h2 id="tocS_KeyManagerUpdateRequest">KeyManagerUpdateRequest</h2>

<a id="schemakeymanagerupdaterequest"></a>
<a id="schema_KeyManagerUpdateRequest"></a>
<a id="tocSkeymanagerupdaterequest"></a>
<a id="tocskeymanagerupdaterequest"></a>

```json
{
  "name": "Asgardeo",
  "type": "ASGARDEO",
  "enabled": true,
  "tokenEndpoint": "https://api.asgardeo.io/t/myorg/oauth2/token"
}

```

Partial update payload for a key manager. All fields are optional; only supplied fields are applied. Omitted fields retain their stored values.

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|name|string|false|none|Unique name within the organization.|
|type|string|false|none|none|
|enabled|boolean|false|none|none|
|tokenEndpoint|string(uri)|false|none|none|

#### Enumerated Values

|Property|Value|
|---|---|
|type|ASGARDEO|
|type|WSO2IS|
|type|KEYCLOAK|
|type|GENERIC_OIDC|

<h2 id="tocS_KeyManagerResponseSchema">KeyManagerResponseSchema</h2>

<a id="schemakeymanagerresponseschema"></a>
<a id="schema_KeyManagerResponseSchema"></a>
<a id="tocSkeymanagerresponseschema"></a>
<a id="tocskeymanagerresponseschema"></a>

```json
{
  "id": "km-uuid-12345",
  "orgId": "org-12345",
  "name": "Asgardeo",
  "type": "ASGARDEO",
  "enabled": true,
  "tokenEndpoint": "https://api.asgardeo.io/t/myorg/oauth2/token"
}

```

Key manager configuration.

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|id|string|false|none|Key manager UUID.|
|orgId|string|false|none|none|
|name|string|false|none|none|
|type|string|false|none|none|
|enabled|boolean|false|none|none|
|tokenEndpoint|string(uri)|false|none|none|

#### Enumerated Values

|Property|Value|
|---|---|
|type|ASGARDEO|
|type|WSO2IS|
|type|KEYCLOAK|
|type|GENERIC_OIDC|

<h2 id="tocS_KeyManagerPublicResponseSchema">KeyManagerPublicResponseSchema</h2>

<a id="schemakeymanagerpublicresponseschema"></a>
<a id="schema_KeyManagerPublicResponseSchema"></a>
<a id="tocSkeymanagerpublicresponseschema"></a>
<a id="tocskeymanagerpublicresponseschema"></a>

```json
{
  "id": "km-uuid-12345",
  "name": "Asgardeo",
  "type": "ASGARDEO",
  "tokenEndpoint": "https://api.asgardeo.io/t/myorg/oauth2/token"
}

```

Minimal developer-facing key manager view.

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|id|string|false|none|none|
|name|string|false|none|none|
|type|string|false|none|none|
|tokenEndpoint|string(uri)|false|none|none|

#### Enumerated Values

|Property|Value|
|---|---|
|type|ASGARDEO|
|type|WSO2IS|
|type|KEYCLOAK|
|type|GENERIC_OIDC|

<h2 id="tocS_WebhookSubscriberRequest">WebhookSubscriberRequest</h2>

<a id="schemawebhooksubscriberrequest"></a>
<a id="schema_WebhookSubscriberRequest"></a>
<a id="tocSwebhooksubscriberrequest"></a>
<a id="tocswebhooksubscriberrequest"></a>

```json
{
  "name": "Production Gateway",
  "url": "https://gateway.example.com/devportal-webhook",
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
|name|string|true|none|Unique name within the organization.|
|url|string(uri)|true|none|Target URL events are POSTed to. Must be unique within the organization.|
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
  "id": "sub-uuid-12345",
  "orgId": "org-12345",
  "name": "Production Gateway",
  "url": "https://gateway.example.com/devportal-webhook",
  "enabled": true,
  "events": [
    "apikey.*",
    "subscription.*"
  ],
  "timeoutMs": 5000,
  "hasSecret": true,
  "hasPublicKey": false
}

```

Webhook subscriber configuration. The secret is never included.

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|id|string|false|none|Webhook subscriber UUID.|
|orgId|string|false|none|none|
|name|string|false|none|none|
|url|string(uri)|false|none|none|
|enabled|boolean|false|none|none|
|events|[string]|false|none|none|
|timeoutMs|integer|false|none|none|
|hasSecret|boolean|false|none|Whether a secret is configured for HMAC-signing outgoing payloads.|
|hasPublicKey|boolean|false|none|Whether a public key is configured for envelope-encrypting secret event payloads.|

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
  "keyType": "PRODUCTION",
  "consumerKey": "consumer-key-123"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|keyManager|string|true|none|none|
|keyType|string|false|none|none|
|consumerKey|string|true|none|The OAuth client_id, created directly in the key manager. The portal never sees or stores a client secret — it is supplied per-request when generating a token.|

#### Enumerated Values

|Property|Value|
|---|---|
|keyType|PRODUCTION|
|keyType|SANDBOX|

<h2 id="tocS_ViewCreateRequest">ViewCreateRequest</h2>

<a id="schemaviewcreaterequest"></a>
<a id="schema_ViewCreateRequest"></a>
<a id="tocSviewcreaterequest"></a>
<a id="tocsviewcreaterequest"></a>

```json
{
  "handle": "partner-apis",
  "name": "Partner APIs",
  "labels": [
    "partner",
    "public"
  ]
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|handle|string|true|none|none|
|name|string|false|none|Optional display name. Defaults to `handle` when omitted.|
|labels|[string]|true|none|Label names to attach to the view.|

<h2 id="tocS_ViewUpdateRequest">ViewUpdateRequest</h2>

<a id="schemaviewupdaterequest"></a>
<a id="schema_ViewUpdateRequest"></a>
<a id="tocSviewupdaterequest"></a>
<a id="tocsviewupdaterequest"></a>

```json
{
  "name": "Partner and Public APIs",
  "addedLabels": [
    "premium"
  ],
  "removedLabels": [
    "internal"
  ]
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|name|string|false|none|none|
|addedLabels|[string]|false|none|Label names to attach to the view.|
|removedLabels|[string]|false|none|Label names to detach from the view.|

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
|consumerSecret|string|false|none|Client secret for the OAuth application. Not stored by the portal — the caller must supply it on each token generation request.|
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
  "keyType": "PRODUCTION",
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
|keyType|string|false|none|none|
|consumerKey|string|false|none|none|
|tokenEndpoint|string(uri)|false|none|none|

<h2 id="tocS_OAuthTokenResponse">OAuthTokenResponse</h2>

<a id="schemaoauthtokenresponse"></a>
<a id="schema_OAuthTokenResponse"></a>
<a id="tocSoauthtokenresponse"></a>
<a id="tocsoauthtokenresponse"></a>

```json
{
  "access_token": "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.example",
  "token_type": "Bearer",
  "expires_in": 3600,
  "scope": "weather.read"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|access_token|string|false|none|none|
|token_type|string|false|none|none|
|expires_in|integer|false|none|none|
|scope|string|false|none|none|

<h2 id="tocS_APIFlowCreateResponse">APIFlowCreateResponse</h2>

<a id="schemaapiflowcreateresponse"></a>
<a id="schema_APIFlowCreateResponse"></a>
<a id="tocSapiflowcreateresponse"></a>
<a id="tocsapiflowcreateresponse"></a>

```json
{
  "apiFlowId": "flow-12345",
  "name": "Weather onboarding",
  "status": "PUBLISHED"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|apiFlowId|string|false|none|none|
|name|string|false|none|none|
|status|string|false|none|none|

#### Enumerated Values

|Property|Value|
|---|---|
|status|DRAFT|
|status|PUBLISHED|

<h2 id="tocS_APIFlowResponse">APIFlowResponse</h2>

<a id="schemaapiflowresponse"></a>
<a id="schema_APIFlowResponse"></a>
<a id="tocSapiflowresponse"></a>
<a id="tocsapiflowresponse"></a>

```json
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

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|apiFlowId|string|false|none|none|
|name|string|false|none|none|
|handle|string|false|none|none|
|description|string|false|none|none|
|agentPrompt|string|false|none|none|
|status|string|false|none|none|
|agentVisibility|string|false|none|none|
|contentType|string|false|none|none|
|apiFlowDefinition|string¦null|false|none|none|
|markdownContent|string¦null|false|none|none|
|createdAt|string|false|none|none|
|updatedAt|string¦null|false|none|none|

#### Enumerated Values

|Property|Value|
|---|---|
|status|DRAFT|
|status|PUBLISHED|
|agentVisibility|VISIBLE|
|agentVisibility|HIDDEN|
|contentType|ARAZZO|
|contentType|MD|

<h2 id="tocS_APIFlowPromptResponse">APIFlowPromptResponse</h2>

<a id="schemaapiflowpromptresponse"></a>
<a id="schema_APIFlowPromptResponse"></a>
<a id="tocSapiflowpromptresponse"></a>
<a id="tocsapiflowpromptresponse"></a>

```json
{
  "agentPrompt": "string"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|agentPrompt|string|false|none|none|

<h2 id="tocS_TempArazzoFileResponse">TempArazzoFileResponse</h2>

<a id="schematemparazzofileresponse"></a>
<a id="schema_TempArazzoFileResponse"></a>
<a id="tocStemparazzofileresponse"></a>
<a id="tocstemparazzofileresponse"></a>

```json
{
  "path": "/tmp/arazzo-abc123/workflow.arazzo.yaml"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|path|string|false|none|none|

<h2 id="tocS_APIFlowCreateRequest">APIFlowCreateRequest</h2>

<a id="schemaapiflowcreaterequest"></a>
<a id="schema_APIFlowCreateRequest"></a>
<a id="tocSapiflowcreaterequest"></a>
<a id="tocsapiflowcreaterequest"></a>

```json
{
  "name": "Weather onboarding",
  "handle": "weather-onboarding",
  "description": "Guides users through the Weather API onboarding workflow.",
  "agentPrompt": "Follow this workflow to onboard a Weather API user.",
  "status": "PUBLISHED",
  "agentVisibility": "VISIBLE",
  "contentType": "ARAZZO",
  "apiFlowDefinition": {},
  "markdownContent": "string"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|name|string|true|none|none|
|handle|string|false|none|none|
|description|string|true|none|none|
|agentPrompt|string|false|none|none|
|status|string|false|none|none|
|agentVisibility|string|false|none|none|
|contentType|string|false|none|none|
|apiFlowDefinition|any|false|none|JSON/YAML Arazzo content when `contentType` is `ARAZZO`.|

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

<h2 id="tocS_APIFlowUpdateRequest">APIFlowUpdateRequest</h2>

<a id="schemaapiflowupdaterequest"></a>
<a id="schema_APIFlowUpdateRequest"></a>
<a id="tocSapiflowupdaterequest"></a>
<a id="tocsapiflowupdaterequest"></a>

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

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|name|string|false|none|none|
|handle|string|false|none|none|
|description|string|false|none|none|
|agentPrompt|string|false|none|none|
|status|string|false|none|none|
|agentVisibility|string|false|none|none|
|contentType|string|false|none|none|
|apiFlowDefinition|any|false|none|none|

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

<h2 id="tocS_APIFlowPromptRequest">APIFlowPromptRequest</h2>

<a id="schemaapiflowpromptrequest"></a>
<a id="schema_APIFlowPromptRequest"></a>
<a id="tocSapiflowpromptrequest"></a>
<a id="tocsapiflowpromptrequest"></a>

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

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|name|string|true|none|none|
|description|string|true|none|none|
|apis|[object]|false|none|none|
|orgHandle|string|false|none|none|
|viewName|string|false|none|none|
|handle|string|false|none|none|

<h2 id="tocS_LoginRequest">LoginRequest</h2>

<a id="schemaloginrequest"></a>
<a id="schema_LoginRequest"></a>
<a id="tocSloginrequest"></a>
<a id="tocsloginrequest"></a>

```json
{
  "username": "string",
  "password": "pa$$word"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|username|string|true|none|none|
|password|string(password)|true|none|none|

<h2 id="tocS_TempArazzoFileRequest">TempArazzoFileRequest</h2>

<a id="schematemparazzofilerequest"></a>
<a id="schema_TempArazzoFileRequest"></a>
<a id="tocStemparazzofilerequest"></a>
<a id="tocstemparazzofilerequest"></a>

```json
{
  "content": "arazzo: 1.0.1\ninfo:\n  title: Weather onboarding\n  version: 1.0.0\nworkflows: []\n",
  "filename": "workflow.arazzo.yaml"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|content|string|true|none|Arazzo YAML content to write to a temporary file.|
|filename|string|false|none|none|

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
