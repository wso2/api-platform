# Schemas

<h2 id="tocS_APIKeyItem">APIKeyItem</h2>

<a id="schemaapikeyitem"></a>
<a id="schema_APIKeyItem"></a>
<a id="tocSapikeyitem"></a>
<a id="tocsapikeyitem"></a>

```json
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
  "allowedTargets": "string"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|id|string|false|none|URL-safe handle (identifier) of the API key (generated from displayName when not supplied)|
|displayName|string|true|none|Human-readable display name of the API key|
|maskedApiKey|string|true|none|Masked representation of the API key for display purposes|
|status|string|true|none|Current status of the key|
|createdAt|string(date-time)|true|none|Timestamp when the key was created|
|createdBy|string|true|read-only|User identifier of the user who created this resource|
|updatedAt|string(date-time)|true|none|Timestamp when the key was last updated|
|expiresAt|string(date-time)|false|none|Optional expiration timestamp|
|issuer|string|false|none|Optional identifier of the developer portal that provisioned this key|
|allowedTargets|string|true|none|Comma-separated list of allowed gateways; 'ALL' means unrestricted|

#### Enumerated Values

|Property|Value|
|---|---|
|status|active|
|status|revoked|
|status|expired|

<h2 id="tocS_UserAPIKeyItem">UserAPIKeyItem</h2>

<a id="schemauserapikeyitem"></a>
<a id="schema_UserAPIKeyItem"></a>
<a id="tocSuserapikeyitem"></a>
<a id="tocsuserapikeyitem"></a>

```json
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

```

### Properties

allOf

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|[APIKeyItem](#schemaapikeyitem)|false|none|none|

and

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|object|false|none|none|
|» artifactId|string|true|none|Handle (ID) of the artifact this key belongs to|
|» artifactType|string|true|none|Type of the artifact this key belongs to|

#### Enumerated Values

|Property|Value|
|---|---|
|artifactType|RestApi|
|artifactType|LlmProvider|
|artifactType|LlmProxy|

<h2 id="tocS_UserAPIKeyListResponse">UserAPIKeyListResponse</h2>

<a id="schemauserapikeylistresponse"></a>
<a id="schema_UserAPIKeyListResponse"></a>
<a id="tocSuserapikeylistresponse"></a>
<a id="tocsuserapikeylistresponse"></a>

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

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|list|[[UserAPIKeyItem](#schemauserapikeyitem)]|true|none|List of API keys|
|count|integer|true|none|Number of API keys in current response|
|pagination|[Pagination](#schemapagination)|true|none|none|

<h2 id="tocS_LLMProviderAPIKeyListResponse">LLMProviderAPIKeyListResponse</h2>

<a id="schemallmproviderapikeylistresponse"></a>
<a id="schema_LLMProviderAPIKeyListResponse"></a>
<a id="tocSllmproviderapikeylistresponse"></a>
<a id="tocsllmproviderapikeylistresponse"></a>

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
      "allowedTargets": "string"
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

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|list|[[APIKeyItem](#schemaapikeyitem)]|true|none|List of API keys|
|count|integer|true|none|Number of API keys in current response|
|pagination|[Pagination](#schemapagination)|true|none|none|

<h2 id="tocS_LLMProxyAPIKeyListResponse">LLMProxyAPIKeyListResponse</h2>

<a id="schemallmproxyapikeylistresponse"></a>
<a id="schema_LLMProxyAPIKeyListResponse"></a>
<a id="tocSllmproxyapikeylistresponse"></a>
<a id="tocsllmproxyapikeylistresponse"></a>

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
      "allowedTargets": "string"
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

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|list|[[APIKeyItem](#schemaapikeyitem)]|true|none|List of API keys|
|count|integer|true|none|Number of API keys in current response|
|pagination|[Pagination](#schemapagination)|true|none|none|

<h2 id="tocS_Organization">Organization</h2>

<a id="schemaorganization"></a>
<a id="schema_Organization"></a>
<a id="tocSorganization"></a>
<a id="tocsorganization"></a>

```json
{
  "id": "acme",
  "displayName": "Acme Corporation",
  "region": "us",
  "createdBy": "john.doe",
  "updatedBy": "john.doe",
  "createdAt": "2023-10-12T10:30:00Z",
  "updatedAt": "2023-10-12T10:30:00Z"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|id|string|true|read-only|Handle (URL-friendly slug) for the organization|
|displayName|string|true|none|Human-readable name for the organization|
|region|string|true|none|Geographic region where the organization operates|
|createdBy|string|false|read-only|User identifier of the user who created this resource|
|updatedBy|string|false|read-only|User identifier of the user who last updated this resource|
|createdAt|string(date-time)|false|read-only|Timestamp when the organization was created|
|updatedAt|string(date-time)|false|read-only|Timestamp when the organization was last updated|

<h2 id="tocS_CreateProjectRequest">CreateProjectRequest</h2>

<a id="schemacreateprojectrequest"></a>
<a id="schema_CreateProjectRequest"></a>
<a id="tocScreateprojectrequest"></a>
<a id="tocscreateprojectrequest"></a>

```json
{
  "id": "default-project",
  "displayName": "Default Project",
  "description": "This is the default project for development"
}

```

Request body for creating a project. Organization ID is automatically extracted
from the JWT token and does not need to be provided.

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|id|string|false|none|Handle (URL-friendly slug) for the project. Auto-generated from displayName if not provided.|
|displayName|string|true|none|Human-readable name for the project|
|description|string|false|none|Description of the project|

<h2 id="tocS_Project">Project</h2>

<a id="schemaproject"></a>
<a id="schema_Project"></a>
<a id="tocSproject"></a>
<a id="tocsproject"></a>

```json
{
  "id": "default-project",
  "displayName": "Default Project",
  "description": "This is the default project for development",
  "organizationId": "acme",
  "createdBy": "john.doe",
  "updatedBy": "john.doe",
  "createdAt": "2023-10-12T10:30:00Z",
  "updatedAt": "2023-10-12T10:30:00Z"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|id|string|true|read-only|Handle (URL-friendly slug) for the project|
|displayName|string|true|none|Human-readable name for the project|
|description|string|false|none|Description of the project|
|organizationId|string|true|read-only|Handle (URL-friendly slug) of the organization this project belongs to|
|createdBy|string|false|read-only|User identifier of the user who created this resource|
|updatedBy|string|false|read-only|User identifier of the user who last updated this resource. Only present in the detail response (GET /projects/{projectId}), omitted from list responses.|
|createdAt|string(date-time)|false|read-only|Timestamp when the project was created|
|updatedAt|string(date-time)|false|read-only|Timestamp when the project was last updated|

<h2 id="tocS_ApplicationType">ApplicationType</h2>

<a id="schemaapplicationtype"></a>
<a id="schema_ApplicationType"></a>
<a id="tocSapplicationtype"></a>
<a id="tocsapplicationtype"></a>

```json
"genai"

```

Type of the application

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|string|false|none|Type of the application|

#### Enumerated Values

|Property|Value|
|---|---|
|*anonymous*|genai|

<h2 id="tocS_CreateApplicationRequest">CreateApplicationRequest</h2>

<a id="schemacreateapplicationrequest"></a>
<a id="schema_CreateApplicationRequest"></a>
<a id="tocScreateapplicationrequest"></a>
<a id="tocscreateapplicationrequest"></a>

```json
{
  "id": "my-app-handle",
  "displayName": "GenAI Demo App",
  "projectId": "default-project",
  "type": "genai",
  "description": "Sample GenAI application"
}

```

Request body for creating an application.

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|id|string|false|none|Unique handle/identifier for the application. Can be provided during creation or auto-generated.|
|displayName|string|true|none|Human-readable name for the application|
|projectId|string|true|none|Handle (URL-friendly slug) of the project this application belongs to.|
|type|[ApplicationType](#schemaapplicationtype)|true|none|Type of the application|
|description|string|false|none|Description of the application|

<h2 id="tocS_Application">Application</h2>

<a id="schemaapplication"></a>
<a id="schema_Application"></a>
<a id="tocSapplication"></a>
<a id="tocsapplication"></a>

```json
{
  "id": "my-app-handle",
  "displayName": "GenAI Demo App",
  "projectId": "default-project",
  "type": "genai",
  "description": "Sample GenAI application",
  "createdBy": "john.doe",
  "updatedBy": "john.doe",
  "createdAt": "2025-11-15T10:30:00Z",
  "updatedAt": "2025-11-15T11:30:00Z"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|id|string|true|none|Application handle/identifier|
|displayName|string|true|none|Human-readable name for the application|
|projectId|string|true|none|Handle (URL-friendly slug) of the project this application belongs to|
|type|[ApplicationType](#schemaapplicationtype)|true|none|Type of the application|
|description|string|false|none|Description of the application|
|createdBy|string|false|read-only|User identifier of the user who created this resource|
|updatedBy|string|false|read-only|User identifier of the user who last updated this resource. Only present in the detail response (GET /applications/{appId}), omitted from list responses.|
|createdAt|string(date-time)|false|read-only|none|
|updatedAt|string(date-time)|false|read-only|none|

<h2 id="tocS_ApplicationListResponse">ApplicationListResponse</h2>

<a id="schemaapplicationlistresponse"></a>
<a id="schema_ApplicationListResponse"></a>
<a id="tocSapplicationlistresponse"></a>
<a id="tocsapplicationlistresponse"></a>

```json
{
  "count": 2,
  "list": [
    {
      "id": "my-app-handle",
      "displayName": "GenAI Demo App",
      "projectId": "default-project",
      "type": "genai",
      "description": "Sample GenAI application",
      "createdBy": "john.doe",
      "updatedBy": "john.doe",
      "createdAt": "2025-11-15T10:30:00Z",
      "updatedAt": "2025-11-15T11:30:00Z"
    }
  ],
  "pagination": {
    "total": 10,
    "offset": 0,
    "limit": 10
  }
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|count|integer|true|none|Number of items in current response|
|list|[[Application](#schemaapplication)]|true|none|none|
|pagination|[Pagination](#schemapagination)|true|none|none|

<h2 id="tocS_AddApplicationAPIKeysRequest">AddApplicationAPIKeysRequest</h2>

<a id="schemaaddapplicationapikeysrequest"></a>
<a id="schema_AddApplicationAPIKeysRequest"></a>
<a id="tocSaddapplicationapikeysrequest"></a>
<a id="tocsaddapplicationapikeysrequest"></a>

```json
{
  "apiKeys": [
    {
      "keyId": "client-key-1",
      "associatedEntity": {
        "id": "pizza-api"
      }
    }
  ]
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|apiKeys|[[APIKeyMappingSelector](#schemaapikeymappingselector)]|true|none|List of API key selectors to add to the application mappings|

<h2 id="tocS_AddApplicationAssociationsRequest">AddApplicationAssociationsRequest</h2>

<a id="schemaaddapplicationassociationsrequest"></a>
<a id="schema_AddApplicationAssociationsRequest"></a>
<a id="tocSaddapplicationassociationsrequest"></a>
<a id="tocsaddapplicationassociationsrequest"></a>

```json
{
  "associations": [
    {
      "id": "provider-handle",
      "kind": "LlmProvider"
    }
  ]
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|associations|[[ApplicationAssociationSelector](#schemaapplicationassociationselector)]|true|none|List of association selectors to add to the application|

<h2 id="tocS_ApplicationAssociationSelector">ApplicationAssociationSelector</h2>

<a id="schemaapplicationassociationselector"></a>
<a id="schema_ApplicationAssociationSelector"></a>
<a id="tocSapplicationassociationselector"></a>
<a id="tocsapplicationassociationselector"></a>

```json
{
  "id": "provider-handle",
  "kind": "LlmProvider"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|id|string|true|none|ID or handle of the association target|
|kind|string|true|none|Type of the association target|

#### Enumerated Values

|Property|Value|
|---|---|
|kind|LlmProvider|
|kind|LlmProxy|

<h2 id="tocS_ApplicationAssociation">ApplicationAssociation</h2>

<a id="schemaapplicationassociation"></a>
<a id="schema_ApplicationAssociation"></a>
<a id="tocSapplicationassociation"></a>
<a id="tocsapplicationassociation"></a>

```json
{
  "id": "provider-handle",
  "displayName": "OpenAI Provider",
  "version": "v1.0",
  "kind": "LlmProvider",
  "createdAt": "2025-11-15T10:30:00Z",
  "updatedAt": "2025-11-15T11:30:00Z"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|id|string|true|none|Handle/ID of the associated target|
|displayName|string|true|none|Human-readable name for the associated target|
|version|string|true|none|Version of the associated target|
|kind|string|true|none|Type of associated target|
|createdAt|string(date-time)|false|none|Timestamp when the association was created|
|updatedAt|string(date-time)|false|none|Timestamp when the association was updated|

<h2 id="tocS_ApplicationAssociationListResponse">ApplicationAssociationListResponse</h2>

<a id="schemaapplicationassociationlistresponse"></a>
<a id="schema_ApplicationAssociationListResponse"></a>
<a id="tocSapplicationassociationlistresponse"></a>
<a id="tocsapplicationassociationlistresponse"></a>

```json
{
  "count": 2,
  "list": [
    {
      "id": "provider-handle",
      "displayName": "OpenAI Provider",
      "version": "v1.0",
      "kind": "LlmProvider",
      "createdAt": "2025-11-15T10:30:00Z",
      "updatedAt": "2025-11-15T11:30:00Z"
    }
  ],
  "pagination": {
    "total": 10,
    "offset": 0,
    "limit": 10
  }
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|count|integer|true|none|Number of items in current response|
|list|[[ApplicationAssociation](#schemaapplicationassociation)]|true|none|none|
|pagination|[Pagination](#schemapagination)|true|none|none|

<h2 id="tocS_APIKeyMappingSelector">APIKeyMappingSelector</h2>

<a id="schemaapikeymappingselector"></a>
<a id="schema_APIKeyMappingSelector"></a>
<a id="tocSapikeymappingselector"></a>
<a id="tocsapikeymappingselector"></a>

```json
{
  "keyId": "client-key-1",
  "associatedEntity": {
    "id": "pizza-api"
  }
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|keyId|string|true|none|API key name|
|associatedEntity|[APIKeyMappingAssociatedEntity](#schemaapikeymappingassociatedentity)|true|none|none|

<h2 id="tocS_APIKeyMappingAssociatedEntity">APIKeyMappingAssociatedEntity</h2>

<a id="schemaapikeymappingassociatedentity"></a>
<a id="schema_APIKeyMappingAssociatedEntity"></a>
<a id="tocSapikeymappingassociatedentity"></a>
<a id="tocsapikeymappingassociatedentity"></a>

```json
{
  "id": "pizza-api"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|id|string|true|none|ID of the entity that owns the API key|

<h2 id="tocS_AssociatedEntity">AssociatedEntity</h2>

<a id="schemaassociatedentity"></a>
<a id="schema_AssociatedEntity"></a>
<a id="tocSassociatedentity"></a>
<a id="tocsassociatedentity"></a>

```json
{
  "id": "pizza-api",
  "kind": "RestApi"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|id|string|true|none|ID of the associated entity|
|kind|string|true|none|Type of the associated entity|

<h2 id="tocS_MappedAPIKey">MappedAPIKey</h2>

<a id="schemamappedapikey"></a>
<a id="schema_MappedAPIKey"></a>
<a id="tocSmappedapikey"></a>
<a id="tocsmappedapikey"></a>

```json
{
  "keyId": "client-key-1",
  "associatedEntity": {
    "id": "pizza-api",
    "kind": "RestApi"
  },
  "status": "ACTIVE",
  "userId": "john.doe",
  "createdAt": "2025-11-15T10:30:00Z",
  "updatedAt": "2025-11-15T11:30:00Z",
  "expiresAt": "2026-11-15T10:30:00Z"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|keyId|string|true|none|Name of the API key|
|associatedEntity|[AssociatedEntity](#schemaassociatedentity)|true|none|none|
|status|string|false|none|Status of the API key|
|userId|string|false|read-only|User identifier of the user who created this resource|
|createdAt|string(date-time)|false|none|Timestamp when the API key was created|
|updatedAt|string(date-time)|false|none|Timestamp when the API key was updated|
|expiresAt|string(date-time)|false|none|Expiration timestamp of the API key|

<h2 id="tocS_MappedAPIKeyListResponse">MappedAPIKeyListResponse</h2>

<a id="schemamappedapikeylistresponse"></a>
<a id="schema_MappedAPIKeyListResponse"></a>
<a id="tocSmappedapikeylistresponse"></a>
<a id="tocsmappedapikeylistresponse"></a>

```json
{
  "count": 2,
  "list": [
    {
      "keyId": "client-key-1",
      "associatedEntity": {
        "id": "pizza-api",
        "kind": "RestApi"
      },
      "status": "ACTIVE",
      "userId": "john.doe",
      "createdAt": "2025-11-15T10:30:00Z",
      "updatedAt": "2025-11-15T11:30:00Z",
      "expiresAt": "2026-11-15T10:30:00Z"
    }
  ],
  "pagination": {
    "total": 10,
    "offset": 0,
    "limit": 10
  }
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|count|integer|true|none|Number of items in current response|
|list|[[MappedAPIKey](#schemamappedapikey)]|true|none|none|
|pagination|[Pagination](#schemapagination)|true|none|none|

<h2 id="tocS_RESTAPI">RESTAPI</h2>

<a id="schemarestapi"></a>
<a id="schema_RESTAPI"></a>
<a id="tocSrestapi"></a>
<a id="tocsrestapi"></a>

```json
{
  "id": "my-rest-api-handle",
  "displayName": "PizzaShackAPI",
  "description": "This is a simple API for Pizza Shack online pizza delivery store",
  "context": "/pizza",
  "version": "1.0.0",
  "createdBy": "john.doe",
  "updatedBy": "john.doe",
  "projectId": "default-project",
  "createdAt": "2023-10-12T10:30:00Z",
  "updatedAt": "2023-10-12T10:30:00Z",
  "readOnly": false,
  "upstream": {
    "main": {
      "url": "http://prod-backend:5000/api/v2",
      "auth": {
        "type": "api-key",
        "header": "X-API-Key",
        "value": "my-api-key-value"
      }
    },
    "sandbox": {
      "url": "http://prod-backend:5000/api/v2",
      "auth": {
        "type": "api-key",
        "header": "X-API-Key",
        "value": "my-api-key-value"
      }
    }
  },
  "lifeCycleStatus": "CREATED",
  "kind": "RestApi",
  "transport": [
    "http",
    "https"
  ],
  "policies": [
    {
      "executionCondition": "request.header.x-custom == 'enabled'",
      "name": "SET_HEADER",
      "params": {
        "key": "MyHeader",
        "value": "MyValue"
      },
      "version": "v1"
    }
  ],
  "operations": [
    {
      "name": "getPetById",
      "description": "Find pet by ID",
      "request": {
        "method": "GET",
        "path": "/pet/{petId}",
        "policies": [
          {
            "executionCondition": "request.header.x-custom == 'enabled'",
            "name": "SET_HEADER",
            "params": {
              "key": "MyHeader",
              "value": "MyValue"
            },
            "version": "v1"
          }
        ]
      }
    }
  ],
  "channels": [
    {
      "name": "issues",
      "description": "Channel for order events",
      "request": {
        "method": "SUB",
        "name": "issues",
        "policies": [
          {
            "executionCondition": "request.header.x-custom == 'enabled'",
            "name": "SET_HEADER",
            "params": {
              "key": "MyHeader",
              "value": "MyValue"
            },
            "version": "v1"
          }
        ]
      }
    }
  ],
  "subscriptionPlans": [
    "Gold",
    "Silver"
  ]
}

```

API object

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|id|string|false|none|Unique handle/identifier for the API. Can be provided during creation or auto-generated. On update (PUT), if provided must match the path parameter — returns 400 if they differ.|
|displayName|string|true|none|Human-readable name for the API|
|description|string|false|none|none|
|context|string|true|none|none|
|version|string|true|none|none|
|createdBy|string|false|read-only|User identifier of the user who created this resource|
|updatedBy|string|false|read-only|User identifier of the user who last updated this resource. Only present in the detail response (GET /rest-apis/{apiId}), omitted from list responses.|
|projectId|string|true|none|Handle (URL-friendly slug) of the project this API belongs to|
|createdAt|string(date-time)|false|read-only|Timestamp when the api was created|
|updatedAt|string(date-time)|false|read-only|Timestamp when the api was last updated|
|readOnly|boolean|false|read-only|True if the artifact originated from a data-plane gateway (origin gateway_api) and is read-only in the control plane; false for control-plane created artifacts.|
|upstream|[Upstream](#schemaupstream)|true|none|Upstream backend configuration with main and sandbox endpoints|
|lifeCycleStatus|string|false|none|Current lifecycle status of the API|
|kind|string|false|none|Kind of the API based on its communication protocol or architectural style|
|transport|[string]|false|none|Supported transports for the API (http and/or https)|
|policies|[[Policy](#schemapolicy)]|false|none|List of policies to be applied on the API|
|operations|[[Operation](#schemaoperation)]|false|none|List of operations exposed by this API|
|channels|[[Channel](#schemachannel)]|false|none|List of channels exposed by this API|
|subscriptionPlans|[string]|false|none|List of subscription plan names enabled for this API (e.g. Gold, Silver).<br>When set, only these plans can be used when subscribing to the API.|

#### Enumerated Values

|Property|Value|
|---|---|
|lifeCycleStatus|STAGED|
|lifeCycleStatus|CREATED|
|lifeCycleStatus|PUBLISHED|
|lifeCycleStatus|DEPRECATED|
|lifeCycleStatus|RETIRED|
|lifeCycleStatus|BLOCKED|

<h2 id="tocS_SecurityConfig">SecurityConfig</h2>

<a id="schemasecurityconfig"></a>
<a id="schema_SecurityConfig"></a>
<a id="tocSsecurityconfig"></a>
<a id="tocssecurityconfig"></a>

```json
{
  "enabled": true,
  "apiKey": {
    "enabled": true,
    "key": "X-API-Key",
    "valuePrefix": "Bearer",
    "in": "header"
  }
}

```

Security Configuration

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|enabled|boolean|false|none|Whether security is enabled|
|apiKey|[APIKeySecurity](#schemaapikeysecurity)|false|none|Configuration for API key based authentication|

<h2 id="tocS_APIKeySecurity">APIKeySecurity</h2>

<a id="schemaapikeysecurity"></a>
<a id="schema_APIKeySecurity"></a>
<a id="tocSapikeysecurity"></a>
<a id="tocsapikeysecurity"></a>

```json
{
  "enabled": true,
  "key": "X-API-Key",
  "valuePrefix": "Bearer",
  "in": "header"
}

```

API Key Security

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|enabled|boolean|false|none|Whether API key authentication is enabled|
|key|string|false|none|Name of the header or query parameter to be used for the API key|
|valuePrefix|string|false|none|Optional prefix to strip from the inbound API key value before validation, for example "Bearer"|
|in|string|false|none|Location of the API key (header or query)|

#### Enumerated Values

|Property|Value|
|---|---|
|in|header|
|in|query|

<h2 id="tocS_Operation">Operation</h2>

<a id="schemaoperation"></a>
<a id="schema_Operation"></a>
<a id="tocSoperation"></a>
<a id="tocsoperation"></a>

```json
{
  "name": "getPetById",
  "description": "Find pet by ID",
  "request": {
    "method": "GET",
    "path": "/pet/{petId}",
    "policies": [
      {
        "executionCondition": "request.header.x-custom == 'enabled'",
        "name": "SET_HEADER",
        "params": {
          "key": "MyHeader",
          "value": "MyValue"
        },
        "version": "v1"
      }
    ]
  }
}

```

API Operation

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|name|string|false|none|Name of the operation|
|description|string|false|none|Description of the operation|
|request|[OperationRequest](#schemaoperationrequest)|true|none|Request details for an API operation|

<h2 id="tocS_Channel">Channel</h2>

<a id="schemachannel"></a>
<a id="schema_Channel"></a>
<a id="tocSchannel"></a>
<a id="tocschannel"></a>

```json
{
  "name": "issues",
  "description": "Channel for order events",
  "request": {
    "method": "SUB",
    "name": "issues",
    "policies": [
      {
        "executionCondition": "request.header.x-custom == 'enabled'",
        "name": "SET_HEADER",
        "params": {
          "key": "MyHeader",
          "value": "MyValue"
        },
        "version": "v1"
      }
    ]
  }
}

```

Async API Channel

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|name|string|false|none|Name of the channel|
|description|string|false|none|Description of the channel|
|request|[ChannelRequest](#schemachannelrequest)|true|none|Request details for a channel within the Async API|

<h2 id="tocS_OperationRequest">OperationRequest</h2>

<a id="schemaoperationrequest"></a>
<a id="schema_OperationRequest"></a>
<a id="tocSoperationrequest"></a>
<a id="tocsoperationrequest"></a>

```json
{
  "method": "GET",
  "path": "/pet/{petId}",
  "policies": [
    {
      "executionCondition": "request.header.x-custom == 'enabled'",
      "name": "SET_HEADER",
      "params": {
        "key": "MyHeader",
        "value": "MyValue"
      },
      "version": "v1"
    }
  ]
}

```

Operation Request

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|method|string|true|none|HTTP method for the operation|
|path|string|true|none|Resource path for the operation|
|policies|[[Policy](#schemapolicy)]|false|none|List of policies to be applied on the operation|

#### Enumerated Values

|Property|Value|
|---|---|
|method|GET|
|method|POST|
|method|PUT|
|method|DELETE|
|method|PATCH|
|method|HEAD|
|method|OPTIONS|

<h2 id="tocS_ChannelRequest">ChannelRequest</h2>

<a id="schemachannelrequest"></a>
<a id="schema_ChannelRequest"></a>
<a id="tocSchannelrequest"></a>
<a id="tocschannelrequest"></a>

```json
{
  "method": "SUB",
  "name": "issues",
  "policies": [
    {
      "executionCondition": "request.header.x-custom == 'enabled'",
      "name": "SET_HEADER",
      "params": {
        "key": "MyHeader",
        "value": "MyValue"
      },
      "version": "v1"
    }
  ]
}

```

Channel Request

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|method|string|true|none|Async method for the channel|
|name|string|true|none|Channel name for the event|
|policies|[[Policy](#schemapolicy)]|false|none|List of policies to be applied on the operation|

#### Enumerated Values

|Property|Value|
|---|---|
|method|SUB|

<h2 id="tocS_Policy">Policy</h2>

<a id="schemapolicy"></a>
<a id="schema_Policy"></a>
<a id="tocSpolicy"></a>
<a id="tocspolicy"></a>

```json
{
  "executionCondition": "request.header.x-custom == 'enabled'",
  "name": "SET_HEADER",
  "params": {
    "key": "MyHeader",
    "value": "MyValue"
  },
  "version": "v1"
}

```

Policy Configuration

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|executionCondition|string|false|none|Conditional expression that determines when this policy executes|
|name|string|true|none|Name of the policy to apply|
|params|object|false|none|Key-value pairs of parameters for the policy|
|version|string|true|none|Version of the policy. Only major-only version is allowed (e.g., v0, v1). Full semantic version (e.g., v1.0.0) is not accepted and will be rejected.|

<h2 id="tocS_AddGatewayToRESTAPIRequest">AddGatewayToRESTAPIRequest</h2>

<a id="schemaaddgatewaytorestapirequest"></a>
<a id="schema_AddGatewayToRESTAPIRequest"></a>
<a id="tocSaddgatewaytorestapirequest"></a>
<a id="tocsaddgatewaytorestapirequest"></a>

```json
{
  "gatewayId": "prod-gateway-01"
}

```

AddGatewayToAPIRequest object with basic gateway details

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|gatewayId|string|true|none|Handle (URL-friendly slug) of the gateway to associate with the REST API|

<h2 id="tocS_RESTAPIGatewayListResponse">RESTAPIGatewayListResponse</h2>

<a id="schemarestapigatewaylistresponse"></a>
<a id="schema_RESTAPIGatewayListResponse"></a>
<a id="tocSrestapigatewaylistresponse"></a>
<a id="tocsrestapigatewaylistresponse"></a>

```json
{
  "count": 3,
  "list": [
    {
      "id": "prod-gateway-01",
      "organizationId": "acme",
      "displayName": "Production Gateway 01",
      "description": "Production gateway for handling API traffic",
      "properties": {
        "region": "us-west",
        "tier": "premium"
      },
      "endpoints": [
        "https://api.example.com:8443/api/v1",
        "wss://events.example.com:8444"
      ],
      "isCritical": true,
      "functionalityType": "regular",
      "version": "1.0",
      "isActive": true,
      "createdBy": "john.doe",
      "updatedBy": "john.doe",
      "createdAt": "2025-10-14T10:30:00Z",
      "updatedAt": "2025-10-14T10:30:00Z",
      "associatedAt": "2025-10-15T10:30:00Z",
      "isDeployed": true,
      "deployment": {
        "status": "CREATED",
        "deployedAt": "2025-10-15T11:00:00Z"
      }
    }
  ],
  "pagination": {
    "total": 10,
    "offset": 0,
    "limit": 10
  }
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|count|integer|true|none|Number of gateways in current response|
|list|[[RESTAPIGatewayResponse](#schemarestapigatewayresponse)]|true|none|List of gateways associated with the API, including deployment details when deployed|
|pagination|[Pagination](#schemapagination)|true|none|none|

<h2 id="tocS_RESTAPIGatewayResponse">RESTAPIGatewayResponse</h2>

<a id="schemarestapigatewayresponse"></a>
<a id="schema_RESTAPIGatewayResponse"></a>
<a id="tocSrestapigatewayresponse"></a>
<a id="tocsrestapigatewayresponse"></a>

```json
{
  "id": "prod-gateway-01",
  "organizationId": "acme",
  "displayName": "Production Gateway 01",
  "description": "Production gateway for handling API traffic",
  "properties": {
    "region": "us-west",
    "tier": "premium"
  },
  "endpoints": [
    "https://api.example.com:8443/api/v1",
    "wss://events.example.com:8444"
  ],
  "isCritical": true,
  "functionalityType": "regular",
  "version": "1.0",
  "isActive": true,
  "createdBy": "john.doe",
  "updatedBy": "john.doe",
  "createdAt": "2025-10-14T10:30:00Z",
  "updatedAt": "2025-10-14T10:30:00Z",
  "associatedAt": "2025-10-15T10:30:00Z",
  "isDeployed": true,
  "deployment": {
    "status": "CREATED",
    "deployedAt": "2025-10-15T11:00:00Z"
  }
}

```

### Properties

allOf

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|[GatewayResponse](#schemagatewayresponse)|false|none|none|

and

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|object|false|none|none|
|» associatedAt|string(date-time)|true|none|Timestamp when the gateway was associated with the API|
|» isDeployed|boolean|true|none|Whether the API is currently deployed to this gateway|
|» deployment|[RESTAPIDeploymentDetails](#schemarestapideploymentdetails)|false|none|Deployment details (only present when isDeployed is true)|

<h2 id="tocS_RESTAPIDeploymentDetails">RESTAPIDeploymentDetails</h2>

<a id="schemarestapideploymentdetails"></a>
<a id="schema_RESTAPIDeploymentDetails"></a>
<a id="tocSrestapideploymentdetails"></a>
<a id="tocsrestapideploymentdetails"></a>

```json
{
  "status": "CREATED",
  "deployedAt": "2025-10-15T11:00:00Z"
}

```

Details about API deployment to a specific gateway

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|status|string|true|none|Current deployment status|
|deployedAt|string(date-time)|true|none|Timestamp when the API was deployed|

#### Enumerated Values

|Property|Value|
|---|---|
|status|CREATED|
|status|APPROVED|
|status|REJECTED|

<h2 id="tocS_CreateGatewayRequest">CreateGatewayRequest</h2>

<a id="schemacreategatewayrequest"></a>
<a id="schema_CreateGatewayRequest"></a>
<a id="tocScreategatewayrequest"></a>
<a id="tocscreategatewayrequest"></a>

```json
{
  "id": "prod-gateway-01",
  "displayName": "Production Gateway 01",
  "description": "Production gateway for handling API traffic",
  "endpoints": [
    "https://api.example.com:8443/api/v1",
    "wss://events.example.com:8444"
  ],
  "isCritical": true,
  "functionalityType": "regular",
  "properties": {
    "region": "us-west",
    "tier": "premium"
  },
  "version": "1.0"
}

```

Request body for creating a gateway. Organization ID is automatically extracted
from the JWT token and does not need to be provided.

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|id|string|false|none|Handle (URL-friendly slug) for the gateway. Immutable after creation.|
|displayName|string|true|none|Human-readable gateway name|
|description|string|false|none|Description of the gateway|
|endpoints|[string]|true|none|Network endpoints exposed by this gateway, each as a full URL string|
|isCritical|boolean|false|none|Whether the gateway is critical for production|
|functionalityType|string|true|none|Type of gateway functionality|
|properties|object|false|none|Custom key-value properties for the gateway|
|version|string|false|none|Gateway version in `major.minor` format (e.g. `1.0`) or CalVer `YYYY.MM.DD` format (e.g. `2026.05.13`). Defaults to `1.0` if not provided.|

#### Enumerated Values

|Property|Value|
|---|---|
|functionalityType|regular|
|functionalityType|ai|
|functionalityType|event|

<h2 id="tocS_CustomPolicyResponse">CustomPolicyResponse</h2>

<a id="schemacustompolicyresponse"></a>
<a id="schema_CustomPolicyResponse"></a>
<a id="tocScustompolicyresponse"></a>
<a id="tocscustompolicyresponse"></a>

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

A custom policy stored in the platform's custom policy registry.

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|uuid|string(uuid)|true|none|Unique identifier of the custom policy record|
|organizationUuid|string(uuid)|true|none|Organization this policy belongs to|
|name|string|true|none|Policy name|
|version|string|true|none|Policy version|
|description|string|false|none|Human-readable description of the policy|
|policyDefinition|object|true|none|The full policy schema as declared in the policy's policy-definition.yaml.<br>Contains `parameters` and `systemParameters` JSON Schema documents.|
|createdAt|string(date-time)|false|none|none|
|updatedAt|string(date-time)|false|none|none|

<h2 id="tocS_GatewayPolicyDefinition">GatewayPolicyDefinition</h2>

<a id="schemagatewaypolicydefinition"></a>
<a id="schema_GatewayPolicyDefinition"></a>
<a id="tocSgatewaypolicydefinition"></a>
<a id="tocsgatewaypolicydefinition"></a>

```json
{
  "name": "set-wso2-headers",
  "version": "v0.8.0",
  "description": "Sets WSO2-specific headers in the request and response.",
  "isCustomPolicy": true,
  "policyDefinition": {}
}

```

A policy installed on a gateway controller.

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|name|string|true|none|Unique policy name|
|version|string|true|none|Semantic version of the policy|
|description|string|false|none|Human-readable description of the policy|
|isCustomPolicy|boolean|true|none|Whether this is a user-installed custom policy.|
|policyDefinition|object|false|none|The full policy schema as declared in the policy's policy-definition.yaml.<br>Contains `parameters` and `systemParameters` JSON Schema documents.<br>Only present for custom policies.|

<h2 id="tocS_ManifestSyncResponse">ManifestSyncResponse</h2>

<a id="schemamanifestsyncresponse"></a>
<a id="schema_ManifestSyncResponse"></a>
<a id="tocSmanifestsyncresponse"></a>
<a id="tocsmanifestsyncresponse"></a>

```json
{
  "policies": [
    {
      "name": "set-wso2-headers",
      "version": "v0.8.0",
      "description": "Sets WSO2-specific headers in the request and response.",
      "isCustomPolicy": true,
      "policyDefinition": {}
    }
  ]
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|policies|[[GatewayPolicyDefinition](#schemagatewaypolicydefinition)]|false|none|All policies installed on the gateway. Each entry includes name, version, and isCustomPolicy.<br>Custom policies additionally include policyDefinition with their parameters and systemParameters schemas.|

<h2 id="tocS_GatewayResponse">GatewayResponse</h2>

<a id="schemagatewayresponse"></a>
<a id="schema_GatewayResponse"></a>
<a id="tocSgatewayresponse"></a>
<a id="tocsgatewayresponse"></a>

```json
{
  "id": "prod-gateway-01",
  "organizationId": "acme",
  "displayName": "Production Gateway 01",
  "description": "Production gateway for handling API traffic",
  "properties": {
    "region": "us-west",
    "tier": "premium"
  },
  "endpoints": [
    "https://api.example.com:8443/api/v1",
    "wss://events.example.com:8444"
  ],
  "isCritical": true,
  "functionalityType": "regular",
  "version": "1.0",
  "isActive": true,
  "createdBy": "john.doe",
  "updatedBy": "john.doe",
  "createdAt": "2025-10-14T10:30:00Z",
  "updatedAt": "2025-10-14T10:30:00Z"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|id|string|false|read-only|Handle (URL-friendly slug) for the gateway|
|organizationId|string|false|none|Handle (URL-friendly slug) of the organization this gateway belongs to|
|displayName|string|true|none|Human-readable gateway name|
|description|string|false|none|Description of the gateway|
|properties|object|false|none|Custom key-value properties for the gateway|
|endpoints|[string]|false|none|Network endpoints exposed by this gateway, each as a full URL string|
|isCritical|boolean|false|none|Whether the gateway is critical for production|
|functionalityType|string|false|none|Type of gateway functionality|
|version|string|false|none|Gateway version in `major.minor` format (e.g. `1.0`) or CalVer `YYYY.MM.DD` format (e.g. `2026.05.13`)|
|isActive|boolean|false|none|Indicates if the gateway is currently connected to the platform via WebSocket|
|createdBy|string|false|read-only|User identifier of the user who created this resource|
|updatedBy|string|false|read-only|User identifier of the user who last updated this resource. Only present in the detail response (GET /gateways/{gatewayId}), omitted from list responses.|
|createdAt|string(date-time)|false|none|Timestamp when gateway was registered|
|updatedAt|string(date-time)|false|none|Timestamp when gateway was last updated|

#### Enumerated Values

|Property|Value|
|---|---|
|functionalityType|regular|
|functionalityType|ai|
|functionalityType|event|

<h2 id="tocS_Pagination">Pagination</h2>

<a id="schemapagination"></a>
<a id="schema_Pagination"></a>
<a id="tocSpagination"></a>
<a id="tocspagination"></a>

```json
{
  "total": 10,
  "offset": 0,
  "limit": 10
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|total|integer|true|none|Total number of items available across all pages|
|offset|integer|true|none|Zero-based index of first item in current response|
|limit|integer|true|none|Maximum number of items returned per page|

<h2 id="tocS_GatewayListResponse">GatewayListResponse</h2>

<a id="schemagatewaylistresponse"></a>
<a id="schema_GatewayListResponse"></a>
<a id="tocSgatewaylistresponse"></a>
<a id="tocsgatewaylistresponse"></a>

```json
{
  "count": 2,
  "list": [
    {
      "id": "prod-gateway-01",
      "organizationId": "acme",
      "displayName": "Production Gateway 01",
      "description": "Production gateway for handling API traffic",
      "properties": {
        "region": "us-west",
        "tier": "premium"
      },
      "endpoints": [
        "https://api.example.com:8443/api/v1",
        "wss://events.example.com:8444"
      ],
      "isCritical": true,
      "functionalityType": "regular",
      "version": "1.0",
      "isActive": true,
      "createdBy": "john.doe",
      "updatedBy": "john.doe",
      "createdAt": "2025-10-14T10:30:00Z",
      "updatedAt": "2025-10-14T10:30:00Z"
    }
  ],
  "pagination": {
    "total": 10,
    "offset": 0,
    "limit": 10
  }
}

```

GatewayListResponse for paginated gateway results

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|count|integer|true|none|Number of items in current response|
|list|[[GatewayResponse](#schemagatewayresponse)]|true|none|none|
|pagination|[Pagination](#schemapagination)|true|none|none|

<h2 id="tocS_GatewayStatusResponse">GatewayStatusResponse</h2>

<a id="schemagatewaystatusresponse"></a>
<a id="schema_GatewayStatusResponse"></a>
<a id="tocSgatewaystatusresponse"></a>
<a id="tocsgatewaystatusresponse"></a>

```json
{
  "id": "prod-gateway-01",
  "isActive": true,
  "isCritical": true
}

```

Lightweight gateway status information optimized for frequent polling

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|id|string|false|read-only|Handle (URL-friendly slug) for the gateway|
|isActive|boolean|false|none|Indicates if the gateway is currently connected to the platform via WebSocket|
|isCritical|boolean|false|none|Whether the gateway is critical for production|

<h2 id="tocS_GatewayStatusListResponse">GatewayStatusListResponse</h2>

<a id="schemagatewaystatuslistresponse"></a>
<a id="schema_GatewayStatusListResponse"></a>
<a id="tocSgatewaystatuslistresponse"></a>
<a id="tocsgatewaystatuslistresponse"></a>

```json
{
  "count": 2,
  "list": [
    {
      "id": "prod-gateway-01",
      "isActive": true,
      "isCritical": true
    }
  ],
  "pagination": {
    "total": 10,
    "offset": 0,
    "limit": 10
  }
}

```

List of gateway status information for polling

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|count|integer|true|none|Number of items in current response|
|list|[[GatewayStatusResponse](#schemagatewaystatusresponse)]|true|none|[Lightweight gateway status information optimized for frequent polling]|
|pagination|[Pagination](#schemapagination)|true|none|none|

<h2 id="tocS_ProjectListResponse">ProjectListResponse</h2>

<a id="schemaprojectlistresponse"></a>
<a id="schema_ProjectListResponse"></a>
<a id="tocSprojectlistresponse"></a>
<a id="tocsprojectlistresponse"></a>

```json
{
  "count": 2,
  "list": [
    {
      "id": "default-project",
      "displayName": "Default Project",
      "description": "This is the default project for development",
      "organizationId": "acme",
      "createdBy": "john.doe",
      "updatedBy": "john.doe",
      "createdAt": "2023-10-12T10:30:00Z",
      "updatedAt": "2023-10-12T10:30:00Z"
    }
  ],
  "pagination": {
    "total": 10,
    "offset": 0,
    "limit": 10
  }
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|count|integer|true|none|Number of items in current response|
|list|[[Project](#schemaproject)]|true|none|none|
|pagination|[Pagination](#schemapagination)|true|none|none|

<h2 id="tocS_OrganizationListResponse">OrganizationListResponse</h2>

<a id="schemaorganizationlistresponse"></a>
<a id="schema_OrganizationListResponse"></a>
<a id="tocSorganizationlistresponse"></a>
<a id="tocsorganizationlistresponse"></a>

```json
{
  "count": 2,
  "list": [
    {
      "id": "acme",
      "displayName": "Acme Corporation",
      "region": "us",
      "createdBy": "john.doe",
      "updatedBy": "john.doe",
      "createdAt": "2023-10-12T10:30:00Z",
      "updatedAt": "2023-10-12T10:30:00Z"
    }
  ],
  "pagination": {
    "total": 10,
    "offset": 0,
    "limit": 10
  }
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|count|integer|true|none|Number of items in current response|
|list|[[Organization](#schemaorganization)]|true|none|none|
|pagination|[Pagination](#schemapagination)|true|none|none|

<h2 id="tocS_RESTAPIListResponse">RESTAPIListResponse</h2>

<a id="schemarestapilistresponse"></a>
<a id="schema_RESTAPIListResponse"></a>
<a id="tocSrestapilistresponse"></a>
<a id="tocsrestapilistresponse"></a>

```json
{
  "count": 2,
  "list": [
    {
      "id": "my-rest-api-handle",
      "displayName": "PizzaShackAPI",
      "description": "This is a simple API for Pizza Shack online pizza delivery store",
      "context": "/pizza",
      "version": "1.0.0",
      "createdBy": "john.doe",
      "updatedBy": "john.doe",
      "projectId": "default-project",
      "createdAt": "2023-10-12T10:30:00Z",
      "updatedAt": "2023-10-12T10:30:00Z",
      "readOnly": false,
      "upstream": {
        "main": {
          "url": "http://prod-backend:5000/api/v2",
          "auth": {
            "type": "api-key",
            "header": "X-API-Key",
            "value": "my-api-key-value"
          }
        },
        "sandbox": {
          "url": "http://prod-backend:5000/api/v2",
          "auth": {
            "type": "api-key",
            "header": "X-API-Key",
            "value": "my-api-key-value"
          }
        }
      },
      "lifeCycleStatus": "CREATED",
      "kind": "RestApi",
      "transport": [
        "http",
        "https"
      ],
      "policies": [
        {
          "executionCondition": "request.header.x-custom == 'enabled'",
          "name": "SET_HEADER",
          "params": {
            "key": "MyHeader",
            "value": "MyValue"
          },
          "version": "v1"
        }
      ],
      "operations": [
        {
          "name": "getPetById",
          "description": "Find pet by ID",
          "request": {
            "method": "GET",
            "path": "/pet/{petId}",
            "policies": [
              {
                "executionCondition": "request.header.x-custom == 'enabled'",
                "name": "SET_HEADER",
                "params": {
                  "key": "MyHeader",
                  "value": "MyValue"
                },
                "version": "v1"
              }
            ]
          }
        }
      ],
      "channels": [
        {
          "name": "issues",
          "description": "Channel for order events",
          "request": {
            "method": "SUB",
            "name": "issues",
            "policies": [
              {
                "executionCondition": "request.header.x-custom == 'enabled'",
                "name": "SET_HEADER",
                "params": {
                  "key": "MyHeader",
                  "value": "MyValue"
                },
                "version": "v1"
              }
            ]
          }
        }
      ],
      "subscriptionPlans": [
        "Gold",
        "Silver"
      ]
    }
  ],
  "pagination": {
    "total": 10,
    "offset": 0,
    "limit": 10
  }
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|count|integer|true|none|Number of items in current response|
|list|[[RESTAPI](#schemarestapi)]|true|none|none|
|pagination|[Pagination](#schemapagination)|true|none|none|

<h2 id="tocS_TokenRotationResponse">TokenRotationResponse</h2>

<a id="schematokenrotationresponse"></a>
<a id="schema_TokenRotationResponse"></a>
<a id="tocStokenrotationresponse"></a>
<a id="tocstokenrotationresponse"></a>

```json
{
  "id": "def45678-g901-23hi-j456-789012klmnop",
  "token": "REDACTED_TOKEN",
  "createdAt": "2025-10-15T14:20:00Z",
  "message": "New token generated successfully. Old token remains active until revoked."
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|id|string(uuid)|false|none|ID of the newly generated token|
|token|string|false|none|Plain-text new authentication token (only exposed once during rotation). The example value is a non-functional placeholder.|
|createdAt|string(date-time)|false|none|Timestamp when new token was created|
|message|string|false|none|Informational message about token rotation|

<h2 id="tocS_TokenInfoResponse">TokenInfoResponse</h2>

<a id="schematokeninforesponse"></a>
<a id="schema_TokenInfoResponse"></a>
<a id="tocStokeninforesponse"></a>
<a id="tocstokeninforesponse"></a>

```json
{
  "id": "abc12345-f678-90de-f123-456789abcdef",
  "status": "active",
  "createdAt": "2025-10-14T10:30:00Z",
  "revokedAt": null
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|id|string(uuid)|false|none|Token UUID|
|status|string|false|none|Current token status|
|createdAt|string(date-time)|false|none|Timestamp when token was created|
|revokedAt|string(date-time)¦null|false|none|Timestamp when token was revoked (null if active)|

#### Enumerated Values

|Property|Value|
|---|---|
|status|active|
|status|revoked|

<h2 id="tocS_CreateRESTAPIRequest">CreateRESTAPIRequest</h2>

<a id="schemacreaterestapirequest"></a>
<a id="schema_CreateRESTAPIRequest"></a>
<a id="tocScreaterestapirequest"></a>
<a id="tocscreaterestapirequest"></a>

```json
{
  "id": "my-rest-api-handle",
  "displayName": "PizzaShackAPI",
  "description": "This is a simple API for Pizza Shack online pizza delivery store",
  "context": "/pizza",
  "version": "1.0.0",
  "createdBy": "john.doe",
  "updatedBy": "john.doe",
  "projectId": "default-project",
  "createdAt": "2023-10-12T10:30:00Z",
  "updatedAt": "2023-10-12T10:30:00Z",
  "readOnly": false,
  "upstream": {
    "main": {
      "url": "http://prod-backend:5000/api/v2",
      "auth": {
        "type": "api-key",
        "header": "X-API-Key",
        "value": "my-api-key-value"
      }
    },
    "sandbox": {
      "url": "http://prod-backend:5000/api/v2",
      "auth": {
        "type": "api-key",
        "header": "X-API-Key",
        "value": "my-api-key-value"
      }
    }
  },
  "lifeCycleStatus": "CREATED",
  "kind": "RestApi",
  "transport": [
    "http",
    "https"
  ],
  "policies": [
    {
      "executionCondition": "request.header.x-custom == 'enabled'",
      "name": "SET_HEADER",
      "params": {
        "key": "MyHeader",
        "value": "MyValue"
      },
      "version": "v1"
    }
  ],
  "operations": [
    {
      "name": "getPetById",
      "description": "Find pet by ID",
      "request": {
        "method": "GET",
        "path": "/pet/{petId}",
        "policies": [
          {
            "executionCondition": "request.header.x-custom == 'enabled'",
            "name": "SET_HEADER",
            "params": {
              "key": "MyHeader",
              "value": "MyValue"
            },
            "version": "v1"
          }
        ]
      }
    }
  ],
  "channels": [
    {
      "name": "issues",
      "description": "Channel for order events",
      "request": {
        "method": "SUB",
        "name": "issues",
        "policies": [
          {
            "executionCondition": "request.header.x-custom == 'enabled'",
            "name": "SET_HEADER",
            "params": {
              "key": "MyHeader",
              "value": "MyValue"
            },
            "version": "v1"
          }
        ]
      }
    }
  ],
  "subscriptionPlans": [
    "Gold",
    "Silver"
  ]
}

```

### Properties

allOf

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|[RESTAPI](#schemarestapi)|false|none|none|

and

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|object|false|none|none|

<h2 id="tocS_TimeUnit">TimeUnit</h2>

<a id="schematimeunit"></a>
<a id="schema_TimeUnit"></a>
<a id="tocStimeunit"></a>
<a id="tocstimeunit"></a>

```json
"days"

```

Time unit for API key expiration duration

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|string|false|none|Time unit for API key expiration duration|

#### Enumerated Values

|Property|Value|
|---|---|
|*anonymous*|seconds|
|*anonymous*|minutes|
|*anonymous*|hours|
|*anonymous*|days|
|*anonymous*|weeks|
|*anonymous*|months|

<h2 id="tocS_ExpirationDuration">ExpirationDuration</h2>

<a id="schemaexpirationduration"></a>
<a id="schema_ExpirationDuration"></a>
<a id="tocSexpirationduration"></a>
<a id="tocsexpirationduration"></a>

```json
{
  "duration": 30,
  "unit": "days"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|duration|integer|true|none|Duration value (must be positive)|
|unit|[TimeUnit](#schematimeunit)|true|none|Time unit for API key expiration duration|

<h2 id="tocS_CreateAPIKeyRequest">CreateAPIKeyRequest</h2>

<a id="schemacreateapikeyrequest"></a>
<a id="schema_CreateAPIKeyRequest"></a>
<a id="tocScreateapikeyrequest"></a>
<a id="tocscreateapikeyrequest"></a>

```json
{
  "id": "production-key-01",
  "displayName": "Production API Key",
  "apiKey": "sk_example_1234567890abcdef",
  "externalRefId": "ext-ref-12345",
  "expiresAt": "2026-12-31T23:59:59Z",
  "expiresIn": {
    "duration": 30,
    "unit": "days"
  },
  "issuer": "api-platform-devportal"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|id|string|false|none|Unique identifier for this API key within the API (optional; if omitted,<br>generated from displayName)|
|displayName|string|true|none|Human-readable name for the API key|
|apiKey|string|true|none|The plain text API key value that will be hashed before storage|
|externalRefId|string¦null|false|none|Optional reference ID for tracing purposes (from external platforms)|
|expiresAt|string(date-time)¦null|false|none|Optional expiration time in ISO 8601 format|
|expiresIn|[ExpirationDuration](#schemaexpirationduration)|false|none|Optional expiration duration|
|issuer|string¦null|false|none|Identifier of the developer portal that provisioned this API key. Null if not provided.|

<h2 id="tocS_CreateAPIKeyResponse">CreateAPIKeyResponse</h2>

<a id="schemacreateapikeyresponse"></a>
<a id="schema_CreateAPIKeyResponse"></a>
<a id="tocScreateapikeyresponse"></a>
<a id="tocscreateapikeyresponse"></a>

```json
{
  "status": "success",
  "message": "API key created and broadcasted to gateways successfully",
  "keyId": "production-key-01"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|status|string|true|none|Status of the operation|
|message|string|true|none|Additional details about the operation result|
|keyId|string|false|none|The internal ID generated for tracking|

#### Enumerated Values

|Property|Value|
|---|---|
|status|success|
|status|error|

<h2 id="tocS_UpdateAPIKeyRequest">UpdateAPIKeyRequest</h2>

<a id="schemaupdateapikeyrequest"></a>
<a id="schema_UpdateAPIKeyRequest"></a>
<a id="tocSupdateapikeyrequest"></a>
<a id="tocsupdateapikeyrequest"></a>

```json
{
  "name": "production-key-01",
  "displayName": "Production API Key (Updated)",
  "apiKey": "sk_example_new1234567890abcdef",
  "externalRefId": "ext-ref-12345",
  "expiresAt": "2027-12-31T23:59:59Z",
  "expiresIn": {
    "duration": 30,
    "unit": "days"
  },
  "issuer": "api-platform-devportal"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|name|string|false|none|Unique identifier for this API key within the API (optional; if omitted, generated from displayName)|
|displayName|string|true|none|Human-readable name for the API key|
|apiKey|string|true|none|The new plain text API key value that will be hashed before storage|
|externalRefId|string¦null|false|none|Optional reference ID for tracing purposes (from external platforms)|
|expiresAt|string(date-time)¦null|false|none|Optional expiration time in ISO 8601 format|
|expiresIn|[ExpirationDuration](#schemaexpirationduration)|false|none|Optional expiration duration|
|issuer|string|false|none|Identifies the portal that created this key|

<h2 id="tocS_UpdateAPIKeyResponse">UpdateAPIKeyResponse</h2>

<a id="schemaupdateapikeyresponse"></a>
<a id="schema_UpdateAPIKeyResponse"></a>
<a id="tocSupdateapikeyresponse"></a>
<a id="tocsupdateapikeyresponse"></a>

```json
{
  "status": "success",
  "message": "API key updated and broadcasted to gateways successfully",
  "keyId": "production-key-01"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|status|string|true|none|Status of the operation|
|message|string|true|none|Additional details about the operation result|
|keyId|string|false|none|The internal ID of the updated key|

#### Enumerated Values

|Property|Value|
|---|---|
|status|success|
|status|error|

<h2 id="tocS_SubscriptionPlanLimit">SubscriptionPlanLimit</h2>

<a id="schemasubscriptionplanlimit"></a>
<a id="schema_SubscriptionPlanLimit"></a>
<a id="tocSsubscriptionplanlimit"></a>
<a id="tocssubscriptionplanlimit"></a>

```json
{
  "limitType": "REQUEST_COUNT",
  "timeUnit": "HOUR",
  "timeAmount": 1,
  "limitCount": 10000,
  "limitCountUnit": "string",
  "stopOnQuotaReach": true
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|limitType|string|false|none|Kind of quota this limit enforces. Only REQUEST_COUNT is currently enforced; other values are accepted by the schema for forward compatibility but are rejected by the API today.|
|timeUnit|string|true|none|Throttle window unit|
|timeAmount|integer|false|none|Number of timeUnit windows the limit applies over (e.g. 2 with timeUnit=HOUR means "per 2 hours")|
|limitCount|integer|true|none|Number of requests (or units, for BANDWIDTH/TOTAL_TOKEN_COUNT) allowed in the throttle window|
|limitCountUnit|string|false|none|Unit for limitCount when limitType is BANDWIDTH (e.g. MB, GB)|
|stopOnQuotaReach|boolean|false|none|Whether to block requests when this limit's quota is exhausted|

#### Enumerated Values

|Property|Value|
|---|---|
|limitType|REQUEST_COUNT|
|limitType|BANDWIDTH|
|limitType|TOTAL_TOKEN_COUNT|
|timeUnit|MINUTE|
|timeUnit|HOUR|
|timeUnit|DAY|
|timeUnit|MONTH|

<h2 id="tocS_CreateSubscriptionPlanRequest">CreateSubscriptionPlanRequest</h2>

<a id="schemacreatesubscriptionplanrequest"></a>
<a id="schema_CreateSubscriptionPlanRequest"></a>
<a id="tocScreatesubscriptionplanrequest"></a>
<a id="tocscreatesubscriptionplanrequest"></a>

```json
{
  "id": "gold",
  "displayName": "Gold",
  "limits": [
    {
      "limitType": "REQUEST_COUNT",
      "timeUnit": "HOUR",
      "timeAmount": 1,
      "limitCount": 10000,
      "limitCountUnit": "string",
      "stopOnQuotaReach": true
    }
  ],
  "expiryTime": "2019-08-24T14:15:22Z",
  "status": "ACTIVE"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|id|string|false|none|Handle (URL-friendly slug) for the plan. Immutable after creation.|
|displayName|string|true|none|Human-readable name for the subscription plan|
|limits|[[SubscriptionPlanLimit](#schemasubscriptionplanlimit)]|false|none|Throttling limits for the plan (e.g. requests per hour, requests per month). The table backing this API already supports multiple limits per plan, but the platform-api currently only persists and enforces the first entry in this array; any additional entries are accepted but ignored.|
|expiryTime|string(date-time)|false|none|Optional plan expiry time (RFC3339)|
|status|string|false|none|none|

#### Enumerated Values

|Property|Value|
|---|---|
|status|ACTIVE|
|status|INACTIVE|

<h2 id="tocS_SubscriptionPlan">SubscriptionPlan</h2>

<a id="schemasubscriptionplan"></a>
<a id="schema_SubscriptionPlan"></a>
<a id="tocSsubscriptionplan"></a>
<a id="tocssubscriptionplan"></a>

```json
{
  "id": "string",
  "displayName": "string",
  "limits": [
    {
      "limitType": "REQUEST_COUNT",
      "timeUnit": "HOUR",
      "timeAmount": 1,
      "limitCount": 10000,
      "limitCountUnit": "string",
      "stopOnQuotaReach": true
    }
  ],
  "expiryTime": "2019-08-24T14:15:22Z",
  "organizationId": "acme",
  "status": "ACTIVE",
  "createdBy": "john.doe",
  "updatedBy": "john.doe",
  "createdAt": "2019-08-24T14:15:22Z",
  "updatedAt": "2019-08-24T14:15:22Z"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|id|string|false|none|Handle (slug) for the subscription plan|
|displayName|string|true|none|Human-readable name for the subscription plan|
|limits|[[SubscriptionPlanLimit](#schemasubscriptionplanlimit)]|false|none|Throttling limits configured for the plan. Only one entry is currently supported and returned, even though the underlying storage allows multiple.|
|expiryTime|string(date-time)|false|none|none|
|organizationId|string|false|none|Handle (URL-friendly slug) of the organization this plan belongs to|
|status|string|false|none|none|
|createdBy|string|false|read-only|User identifier of the user who created this resource|
|updatedBy|string|false|read-only|User identifier of the user who last updated this resource. Only present in the detail response (GET /subscription-plans/{planId}), omitted from list responses.|
|createdAt|string(date-time)|false|none|none|
|updatedAt|string(date-time)|false|none|none|

#### Enumerated Values

|Property|Value|
|---|---|
|status|ACTIVE|
|status|INACTIVE|

<h2 id="tocS_SubscriptionPlanListResponse">SubscriptionPlanListResponse</h2>

<a id="schemasubscriptionplanlistresponse"></a>
<a id="schema_SubscriptionPlanListResponse"></a>
<a id="tocSsubscriptionplanlistresponse"></a>
<a id="tocssubscriptionplanlistresponse"></a>

```json
{
  "list": [
    {
      "id": "string",
      "displayName": "string",
      "limits": [
        {
          "limitType": "REQUEST_COUNT",
          "timeUnit": "HOUR",
          "timeAmount": 1,
          "limitCount": 10000,
          "limitCountUnit": "string",
          "stopOnQuotaReach": true
        }
      ],
      "expiryTime": "2019-08-24T14:15:22Z",
      "organizationId": "acme",
      "status": "ACTIVE",
      "createdBy": "john.doe",
      "updatedBy": "john.doe",
      "createdAt": "2019-08-24T14:15:22Z",
      "updatedAt": "2019-08-24T14:15:22Z"
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

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|list|[[SubscriptionPlan](#schemasubscriptionplan)]|true|none|List of subscription plans in current response|
|count|integer|true|none|Number of subscription plans in current response|
|pagination|[Pagination](#schemapagination)|true|none|none|

<h2 id="tocS_CreateSubscriptionRequest">CreateSubscriptionRequest</h2>

<a id="schemacreatesubscriptionrequest"></a>
<a id="schema_CreateSubscriptionRequest"></a>
<a id="tocScreatesubscriptionrequest"></a>
<a id="tocscreatesubscriptionrequest"></a>

```json
{
  "artifactId": "my-rest-api",
  "kind": "RestApi",
  "subscriberId": "user-123",
  "applicationId": "my-app-handle",
  "subscriptionPlanId": "gold",
  "status": "ACTIVE"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|artifactId|string|true|none|Handle (ID) of the artifact to subscribe to. Resolved against the table for the given kind.|
|kind|string|true|none|Type of the artifact identified by artifactId. Determines which artifact table artifactId is resolved against.|
|subscriberId|string|true|none|Unique subscriber identifier for the subscription (required)|
|applicationId|string|false|none|Handle (ID) of the application this subscription belongs to. Optional in token-based subscriptions.|
|subscriptionPlanId|string|false|none|Handle (slug) of the subscription plan. Links the subscription to rate limit and billing configuration.|
|status|string|false|none|Subscription status (default ACTIVE)|

#### Enumerated Values

|Property|Value|
|---|---|
|kind|RestApi|
|kind|LlmProvider|
|kind|LlmProxy|
|kind|Mcp|
|status|ACTIVE|
|status|INACTIVE|
|status|REVOKED|

<h2 id="tocS_Subscription">Subscription</h2>

<a id="schemasubscription"></a>
<a id="schema_Subscription"></a>
<a id="tocSsubscription"></a>
<a id="tocssubscription"></a>

```json
{
  "id": "497f6eca-6276-4993-bfeb-53cbbbba6f08",
  "artifactId": "my-rest-api",
  "kind": "RestApi",
  "subscriberId": "string",
  "applicationId": "my-app-handle",
  "subscriptionToken": "string",
  "subscriptionPlanId": "gold",
  "subscriptionPlanName": "string",
  "organizationId": "acme",
  "status": "ACTIVE",
  "createdBy": "john.doe",
  "updatedBy": "john.doe",
  "createdAt": "2019-08-24T14:15:22Z",
  "updatedAt": "2019-08-24T14:15:22Z"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|id|string(uuid)|false|none|Subscription ID|
|artifactId|string|false|none|Handle (ID) of the subscribed artifact|
|kind|string|false|none|Type of the subscribed artifact|
|subscriberId|string|false|none|Unique subscriber identifier for this API (required)|
|applicationId|string|false|none|Handle (ID) of the application this subscription belongs to (optional for token-based subscriptions)|
|subscriptionToken|string|false|none|Opaque subscription token for API invocation via Subscription-Key header|
|subscriptionPlanId|string|false|none|Handle (slug) of the subscription plan|
|subscriptionPlanName|string|false|none|Subscription plan display name (e.g. Bronze, Gold)|
|organizationId|string|false|none|Handle (URL-friendly slug) of the organization this subscription belongs to|
|status|string|false|none|none|
|createdBy|string|false|read-only|User identifier of the user who created this resource|
|updatedBy|string|false|read-only|User identifier of the user who last updated this resource. Only present in the detail response (GET /subscriptions/{subscriptionId}), omitted from list responses.|
|createdAt|string(date-time)|false|none|none|
|updatedAt|string(date-time)|false|none|none|

#### Enumerated Values

|Property|Value|
|---|---|
|kind|RestApi|
|kind|LlmProvider|
|kind|LlmProxy|
|kind|Mcp|
|status|ACTIVE|
|status|INACTIVE|
|status|REVOKED|

<h2 id="tocS_SubscriptionListResponse">SubscriptionListResponse</h2>

<a id="schemasubscriptionlistresponse"></a>
<a id="schema_SubscriptionListResponse"></a>
<a id="tocSsubscriptionlistresponse"></a>
<a id="tocssubscriptionlistresponse"></a>

```json
{
  "list": [
    {
      "id": "497f6eca-6276-4993-bfeb-53cbbbba6f08",
      "artifactId": "my-rest-api",
      "kind": "RestApi",
      "subscriberId": "string",
      "applicationId": "my-app-handle",
      "subscriptionToken": "string",
      "subscriptionPlanId": "gold",
      "subscriptionPlanName": "string",
      "organizationId": "acme",
      "status": "ACTIVE",
      "createdBy": "john.doe",
      "updatedBy": "john.doe",
      "createdAt": "2019-08-24T14:15:22Z",
      "updatedAt": "2019-08-24T14:15:22Z"
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

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|list|[[Subscription](#schemasubscription)]|true|none|List of subscriptions in current response|
|count|integer|true|none|Number of subscriptions in current response|
|pagination|[Pagination](#schemapagination)|true|none|none|

<h2 id="tocS_Error">Error</h2>

<a id="schemaerror"></a>
<a id="schema_Error"></a>
<a id="tocSerror"></a>
<a id="tocserror"></a>

```json
{
  "status": "error",
  "code": "REST_API_NOT_FOUND",
  "message": "The requested REST API could not be found.",
  "errors": [
    {
      "field": "<name of the offending field>",
      "message": "must start with /"
    }
  ],
  "details": {},
  "trackingId": "4f1c6f2e-8a4b-4c93-b1de-9f2f6f0c2a11"
}

```

Standard error response

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|status|string|true|none|Always the literal "error".|
|code|string|true|none|Stable, machine-readable error code from the error catalog, in the form `<DOMAIN>_<REASON>` (e.g. `REST_API_NOT_FOUND`). Clients and agents should branch on this, not on the HTTP status.|
|message|string|true|none|Human-readable description of the error.|
|errors|[[FieldError](#schemafielderror)]|false|none|Per-field validation failures. Present when the error is a validation failure.|
|details|object|false|none|Optional structured metadata specific to this error condition (e.g. the resources referencing a secret that blocked its deletion). Shape varies by `code`; absent when not applicable.|
|trackingId|string(uuid)|false|none|Correlation ID for server-side failures. Present only on 5xx responses; quote it when reporting the error so operators can find the corresponding server log entry.|

#### Enumerated Values

|Property|Value|
|---|---|
|status|error|

<h2 id="tocS_FieldError">FieldError</h2>

<a id="schemafielderror"></a>
<a id="schema_FieldError"></a>
<a id="tocSfielderror"></a>
<a id="tocsfielderror"></a>

```json
{
  "field": "<name of the offending field>",
  "message": "must start with /"
}

```

Field-level validation error

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|field|string|true|none|Path of the offending field.|
|message|string|true|none|Why the field failed validation.|

<h2 id="tocS_DeployRequest">DeployRequest</h2>

<a id="schemadeployrequest"></a>
<a id="schema_DeployRequest"></a>
<a id="tocSdeployrequest"></a>
<a id="tocsdeployrequest"></a>

```json
{
  "name": "v1.0-production",
  "base": "current",
  "gatewayId": "prod-gateway-01",
  "metadata": {}
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|name|string|true|none|Name/label for this deployment (e.g., "v1.0-prod", "hotfix-2024-01-15")|
|base|string|true|none|The source for the API definition. Can be "current" (latest working copy) or a deploymentId (existing deployment)|
|gatewayId|string|true|none|Handle (URL-friendly slug) of the target gateway for this deployment|
|metadata|object|false|none|Optional metadata for the deployment. Supported keys include `endpointUrl`, `vhostMain`, and `vhostSandbox`.|

<h2 id="tocS_DeploymentResponse">DeploymentResponse</h2>

<a id="schemadeploymentresponse"></a>
<a id="schema_DeploymentResponse"></a>
<a id="tocSdeploymentresponse"></a>
<a id="tocsdeploymentresponse"></a>

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

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|deploymentId|string(uuid)|true|none|Unique identifier for the deployment|
|name|string|true|none|Name/label for this deployment|
|gatewayId|string|true|none|Handle (URL-friendly slug) of the gateway|
|status|string|true|none|Current deployment lifecycle state:<br>- DEPLOYED: Currently active on the gateway<br>- UNDEPLOYED: Suspended but can be rolled back<br>- DEPLOYING: Deployment in progress, waiting for gateway acknowledgement<br>- UNDEPLOYING: Undeployment in progress, waiting for gateway acknowledgement<br>- FAILED: Deployment or undeployment failed (see statusReason for error code)<br>- ARCHIVED: Historical deployment, can be rolled back|
|baseDeploymentId|string(uuid)¦null|false|none|UUID of the base deployment this was created from|
|metadata|object|false|none|Metadata associated with the deployment|
|createdAt|string(date-time)|true|none|Timestamp when the deployment artifact was created|
|statusReason|string¦null|false|none|Error code explaining the failure reason. Null unless status is FAILED (e.g. DEPLOYMENT_TIMEOUT, GATEWAY_PROCESSING_ERROR)|
|updatedAt|string(date-time)¦null|false|none|Timestamp when the deployment status last changed (null for ARCHIVED deployments)|

#### Enumerated Values

|Property|Value|
|---|---|
|status|DEPLOYED|
|status|UNDEPLOYED|
|status|DEPLOYING|
|status|UNDEPLOYING|
|status|FAILED|
|status|ARCHIVED|

<h2 id="tocS_DeploymentListResponse">DeploymentListResponse</h2>

<a id="schemadeploymentlistresponse"></a>
<a id="schema_DeploymentListResponse"></a>
<a id="tocSdeploymentlistresponse"></a>
<a id="tocsdeploymentlistresponse"></a>

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

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|count|integer|true|none|Number of deployments in current response|
|list|[[DeploymentResponse](#schemadeploymentresponse)]|true|none|List of deployments|
|pagination|[Pagination](#schemapagination)|true|none|none|

<h2 id="tocS_Upstream">Upstream</h2>

<a id="schemaupstream"></a>
<a id="schema_Upstream"></a>
<a id="tocSupstream"></a>
<a id="tocsupstream"></a>

```json
{
  "main": {
    "url": "http://prod-backend:5000/api/v2",
    "auth": {
      "type": "api-key",
      "header": "X-API-Key",
      "value": "my-api-key-value"
    }
  },
  "sandbox": {
    "url": "http://prod-backend:5000/api/v2",
    "auth": {
      "type": "api-key",
      "header": "X-API-Key",
      "value": "my-api-key-value"
    }
  }
}

```

Upstream backend configuration with main and sandbox endpoints

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|main|[UpstreamDefinition](#schemaupstreamdefinition)|true|none|Upstream endpoint configuration. Provide exactly one of `url` (a direct backend URL) or<br>`ref` (a reference to a predefined upstream definition) — never both.|
|sandbox|[UpstreamDefinition](#schemaupstreamdefinition)|false|none|Upstream endpoint configuration. Provide exactly one of `url` (a direct backend URL) or<br>`ref` (a reference to a predefined upstream definition) — never both.|

<h2 id="tocS_UpstreamDefinition">UpstreamDefinition</h2>

<a id="schemaupstreamdefinition"></a>
<a id="schema_UpstreamDefinition"></a>
<a id="tocSupstreamdefinition"></a>
<a id="tocsupstreamdefinition"></a>

```json
{
  "url": "http://prod-backend:5000/api/v2",
  "auth": {
    "type": "api-key",
    "header": "X-API-Key",
    "value": "my-api-key-value"
  }
}

```

Upstream endpoint configuration. Provide exactly one of `url` (a direct backend URL) or
`ref` (a reference to a predefined upstream definition) — never both.

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|url|string(uri)|false|none|Direct backend URL to route traffic to. Mutually exclusive with `ref`.|
|ref|string|false|none|Reference to a predefined upstreamDefinition. Mutually exclusive with `url`.|
|auth|[UpstreamAuth](#schemaupstreamauth)|false|none|Authentication configuration for upstream endpoints|

oneOf

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|object|false|none|none|

xor

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|object|false|none|none|

<h2 id="tocS_UpstreamAuth">UpstreamAuth</h2>

<a id="schemaupstreamauth"></a>
<a id="schema_UpstreamAuth"></a>
<a id="tocSupstreamauth"></a>
<a id="tocsupstreamauth"></a>

```json
{
  "type": "api-key",
  "header": "X-API-Key",
  "value": "my-api-key-value"
}

```

Authentication configuration for upstream endpoints

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|type|string|false|none|Authentication type|
|header|string|false|none|Header name for api-key authentication (e.g., 'Authorization' for bearer/basic, custom header for api-key)|
|value|string(password)|false|write-only|Authentication value (API key, Bearer token, or Base64 encoded credentials for basic auth)|

#### Enumerated Values

|Property|Value|
|---|---|
|type|basic|
|type|bearer|
|type|api-key|
|type|other|
|type|none|

<h2 id="tocS_ExtractionIdentifier">ExtractionIdentifier</h2>

<a id="schemaextractionidentifier"></a>
<a id="schema_ExtractionIdentifier"></a>
<a id="tocSextractionidentifier"></a>
<a id="tocsextractionidentifier"></a>

```json
{
  "location": "payload",
  "identifier": "$.usage.inputTokens"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|location|string|true|none|Where to find the token information|
|identifier|string|true|none|JSONPath expression or header name to identify the token value|

#### Enumerated Values

|Property|Value|
|---|---|
|location|payload|
|location|header|
|location|queryParam|
|location|pathParam|

<h2 id="tocS_LLMProviderTemplateAuth">LLMProviderTemplateAuth</h2>

<a id="schemallmprovidertemplateauth"></a>
<a id="schema_LLMProviderTemplateAuth"></a>
<a id="tocSllmprovidertemplateauth"></a>
<a id="tocsllmprovidertemplateauth"></a>

```json
{
  "type": "bearer",
  "header": "Authorization",
  "valuePrefix": "Bearer "
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|type|string|false|none|Authentication type used by the LLM provider template|
|header|string|false|none|Header name to send the auth value|
|valuePrefix|string|false|none|Prefix to attach before the auth value|

<h2 id="tocS_LLMProviderTemplateMetadata">LLMProviderTemplateMetadata</h2>

<a id="schemallmprovidertemplatemetadata"></a>
<a id="schema_LLMProviderTemplateMetadata"></a>
<a id="tocSllmprovidertemplatemetadata"></a>
<a id="tocsllmprovidertemplatemetadata"></a>

```json
{
  "endpointUrl": "https://api.openai.com",
  "auth": {
    "type": "bearer",
    "header": "Authorization",
    "valuePrefix": "Bearer "
  },
  "logoUrl": "https://cdn.example.com/logos/openai.svg",
  "openapiSpecUrl": "https://api.openai.com/openapi.json"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|endpointUrl|string(uri)|false|none|Default endpoint URL for the template|
|auth|[LLMProviderTemplateAuth](#schemallmprovidertemplateauth)|false|none|none|
|logoUrl|string(uri)|false|none|URL of the provider logo|
|openapiSpecUrl|string(uri)|false|none|URL to the OpenAPI specification for the provider|

<h2 id="tocS_LLMProviderTemplateResourceMapping">LLMProviderTemplateResourceMapping</h2>

<a id="schemallmprovidertemplateresourcemapping"></a>
<a id="schema_LLMProviderTemplateResourceMapping"></a>
<a id="tocSllmprovidertemplateresourcemapping"></a>
<a id="tocsllmprovidertemplateresourcemapping"></a>

```json
{
  "resource": "/responses",
  "promptTokens": {
    "location": "payload",
    "identifier": "$.usage.inputTokens"
  },
  "completionTokens": {
    "location": "payload",
    "identifier": "$.usage.outputTokens"
  },
  "totalTokens": {
    "location": "payload",
    "identifier": "$.usage.totalTokens"
  },
  "remainingTokens": {
    "location": "header",
    "identifier": "x-ratelimit-remaining-tokens"
  },
  "requestModel": {
    "location": "payload",
    "identifier": "$.model"
  },
  "responseModel": {
    "location": "payload",
    "identifier": "$.model"
  }
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|resource|string|true|none|Resource path pattern this mapping applies to (for example /responses or /chat/*)|
|promptTokens|[ExtractionIdentifier](#schemaextractionidentifier)|false|none|none|
|completionTokens|[ExtractionIdentifier](#schemaextractionidentifier)|false|none|none|
|totalTokens|[ExtractionIdentifier](#schemaextractionidentifier)|false|none|none|
|remainingTokens|[ExtractionIdentifier](#schemaextractionidentifier)|false|none|none|
|requestModel|[ExtractionIdentifier](#schemaextractionidentifier)|false|none|none|
|responseModel|[ExtractionIdentifier](#schemaextractionidentifier)|false|none|none|

<h2 id="tocS_LLMProviderTemplateResourceMappings">LLMProviderTemplateResourceMappings</h2>

<a id="schemallmprovidertemplateresourcemappings"></a>
<a id="schema_LLMProviderTemplateResourceMappings"></a>
<a id="tocSllmprovidertemplateresourcemappings"></a>
<a id="tocsllmprovidertemplateresourcemappings"></a>

```json
{
  "resources": [
    {
      "resource": "/responses",
      "promptTokens": {
        "location": "payload",
        "identifier": "$.usage.inputTokens"
      },
      "completionTokens": {
        "location": "payload",
        "identifier": "$.usage.outputTokens"
      },
      "totalTokens": {
        "location": "payload",
        "identifier": "$.usage.totalTokens"
      },
      "remainingTokens": {
        "location": "header",
        "identifier": "x-ratelimit-remaining-tokens"
      },
      "requestModel": {
        "location": "payload",
        "identifier": "$.model"
      },
      "responseModel": {
        "location": "payload",
        "identifier": "$.model"
      }
    }
  ]
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|resources|[[LLMProviderTemplateResourceMapping](#schemallmprovidertemplateresourcemapping)]|false|none|none|

<h2 id="tocS_LLMProviderTemplate">LLMProviderTemplate</h2>

<a id="schemallmprovidertemplate"></a>
<a id="schema_LLMProviderTemplate"></a>
<a id="tocSllmprovidertemplate"></a>
<a id="tocsllmprovidertemplate"></a>

```json
{
  "id": "openai",
  "groupId": "openai",
  "displayName": "OpenAI",
  "managedBy": "wso2",
  "description": "Default OpenAI template",
  "createdBy": "john.doe",
  "updatedBy": "john.doe",
  "readOnly": false,
  "version": "v1.0",
  "isLatest": true,
  "enabled": true,
  "openapi": "openapi: 3.0.3\ninfo:\n  title: Provider API\n  version: v1.0\npaths: {}\n",
  "metadata": {
    "endpointUrl": "https://api.openai.com",
    "auth": {
      "type": "bearer",
      "header": "Authorization",
      "valuePrefix": "Bearer "
    },
    "logoUrl": "https://cdn.example.com/logos/openai.svg",
    "openapiSpecUrl": "https://api.openai.com/openapi.json"
  },
  "promptTokens": {
    "location": "payload",
    "identifier": "$.usage.inputTokens"
  },
  "completionTokens": {
    "location": "payload",
    "identifier": "$.usage.outputTokens"
  },
  "totalTokens": {
    "location": "payload",
    "identifier": "$.usage.totalTokens"
  },
  "remainingTokens": {
    "location": "header",
    "identifier": "x-ratelimit-remaining-tokens"
  },
  "requestModel": {
    "location": "payload",
    "identifier": "$.model"
  },
  "responseModel": {
    "location": "payload",
    "identifier": "$.model"
  },
  "resourceMappings": {
    "resources": [
      {
        "resource": "/responses",
        "promptTokens": {
          "location": "payload",
          "identifier": "$.usage.inputTokens"
        },
        "completionTokens": {
          "location": "payload",
          "identifier": "$.usage.outputTokens"
        },
        "totalTokens": {
          "location": "payload",
          "identifier": "$.usage.totalTokens"
        },
        "remainingTokens": {
          "location": "header",
          "identifier": "x-ratelimit-remaining-tokens"
        },
        "requestModel": {
          "location": "payload",
          "identifier": "$.model"
        },
        "responseModel": {
          "location": "payload",
          "identifier": "$.model"
        }
      }
    ]
  },
  "createdAt": "2023-10-12T10:30:00Z",
  "updatedAt": "2023-10-12T10:30:00Z"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|id|string|false|none|Unique handle for the template|
|groupId|string|false|read-only|Stable identifier shared by every version of a template family.|
|displayName|string|true|none|Human-readable LLM Template name|
|managedBy|string|false|none|Identifies who manages the template. Built-in templates use 'wso2';<br>custom templates default to 'organization' and may be set to any value.|
|description|string|false|none|Description of the LLM provider template|
|createdBy|string|false|read-only|User identifier of the user who created this resource|
|updatedBy|string|false|read-only|User identifier of the user who last updated this resource|
|readOnly|boolean|false|read-only|True if the artifact originated from a data-plane gateway (origin gateway_api) and is read-only in the control plane; false for control-plane created artifacts.|
|version|string|true|none|Content version, matching the v<major>.<minor> pattern (e.g. v1.0, v2.0).<br>Must be unique for this template.|
|isLatest|boolean|false|read-only|Whether this is the latest version of the template.|
|enabled|boolean|false|read-only|Whether this version is offered when creating providers. If false, the<br>template version is hidden from the provider creation UI and API.|
|openapi|string|false|none|OpenAPI specification content (JSON or YAML) for the provider, when<br>uploaded/pasted. Use metadata.openapiSpecUrl instead to reference the<br>spec by URL.|
|metadata|[LLMProviderTemplateMetadata](#schemallmprovidertemplatemetadata)|false|none|none|
|promptTokens|[ExtractionIdentifier](#schemaextractionidentifier)|false|none|none|
|completionTokens|[ExtractionIdentifier](#schemaextractionidentifier)|false|none|none|
|totalTokens|[ExtractionIdentifier](#schemaextractionidentifier)|false|none|none|
|remainingTokens|[ExtractionIdentifier](#schemaextractionidentifier)|false|none|none|
|requestModel|[ExtractionIdentifier](#schemaextractionidentifier)|false|none|none|
|responseModel|[ExtractionIdentifier](#schemaextractionidentifier)|false|none|none|
|resourceMappings|[LLMProviderTemplateResourceMappings](#schemallmprovidertemplateresourcemappings)|false|none|none|
|createdAt|string(date-time)|false|read-only|Timestamp when the resource was created|
|updatedAt|string(date-time)|false|read-only|Timestamp when the resource was last updated|

<h2 id="tocS_CreateLLMProviderTemplateVersionRequest">CreateLLMProviderTemplateVersionRequest</h2>

<a id="schemacreatellmprovidertemplateversionrequest"></a>
<a id="schema_CreateLLMProviderTemplateVersionRequest"></a>
<a id="tocScreatellmprovidertemplateversionrequest"></a>
<a id="tocscreatellmprovidertemplateversionrequest"></a>

```json
{
  "displayName": "OpenAI",
  "version": "v2.0",
  "managedBy": "organization",
  "description": "Default OpenAI template",
  "openapi": "string",
  "metadata": {
    "endpointUrl": "https://api.openai.com",
    "auth": {
      "type": "bearer",
      "header": "Authorization",
      "valuePrefix": "Bearer "
    },
    "logoUrl": "https://cdn.example.com/logos/openai.svg",
    "openapiSpecUrl": "https://api.openai.com/openapi.json"
  },
  "promptTokens": {
    "location": "payload",
    "identifier": "$.usage.inputTokens"
  },
  "completionTokens": {
    "location": "payload",
    "identifier": "$.usage.outputTokens"
  },
  "totalTokens": {
    "location": "payload",
    "identifier": "$.usage.totalTokens"
  },
  "remainingTokens": {
    "location": "header",
    "identifier": "x-ratelimit-remaining-tokens"
  },
  "requestModel": {
    "location": "payload",
    "identifier": "$.model"
  },
  "responseModel": {
    "location": "payload",
    "identifier": "$.model"
  },
  "resourceMappings": {
    "resources": [
      {
        "resource": "/responses",
        "promptTokens": {
          "location": "payload",
          "identifier": "$.usage.inputTokens"
        },
        "completionTokens": {
          "location": "payload",
          "identifier": "$.usage.outputTokens"
        },
        "totalTokens": {
          "location": "payload",
          "identifier": "$.usage.totalTokens"
        },
        "remainingTokens": {
          "location": "header",
          "identifier": "x-ratelimit-remaining-tokens"
        },
        "requestModel": {
          "location": "payload",
          "identifier": "$.model"
        },
        "responseModel": {
          "location": "payload",
          "identifier": "$.model"
        }
      }
    ]
  }
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|displayName|string|false|none|Human-readable LLM Template name. Optional — when omitted, the new<br>version inherits the family's existing name. Supplying a different<br>value renames the template family.|
|version|string|true|none|New version identifier, e.g. v2.0. Must be unique for this template.|
|managedBy|string|false|none|Identifies who manages the template. Custom templates default to 'organization'.|
|description|string|false|none|Description of the LLM provider template|
|openapi|string|false|none|OpenAPI specification content (JSON or YAML) for the provider, when<br>uploaded/pasted. Use metadata.openapiSpecUrl instead to reference the<br>spec by URL.|
|metadata|[LLMProviderTemplateMetadata](#schemallmprovidertemplatemetadata)|false|none|none|
|promptTokens|[ExtractionIdentifier](#schemaextractionidentifier)|false|none|none|
|completionTokens|[ExtractionIdentifier](#schemaextractionidentifier)|false|none|none|
|totalTokens|[ExtractionIdentifier](#schemaextractionidentifier)|false|none|none|
|remainingTokens|[ExtractionIdentifier](#schemaextractionidentifier)|false|none|none|
|requestModel|[ExtractionIdentifier](#schemaextractionidentifier)|false|none|none|
|responseModel|[ExtractionIdentifier](#schemaextractionidentifier)|false|none|none|
|resourceMappings|[LLMProviderTemplateResourceMappings](#schemallmprovidertemplateresourcemappings)|false|none|none|

<h2 id="tocS_LLMProviderTemplateListItem">LLMProviderTemplateListItem</h2>

<a id="schemallmprovidertemplatelistitem"></a>
<a id="schema_LLMProviderTemplateListItem"></a>
<a id="tocSllmprovidertemplatelistitem"></a>
<a id="tocsllmprovidertemplatelistitem"></a>

```json
{
  "id": "openai",
  "groupId": "openai",
  "displayName": "OpenAI",
  "managedBy": "wso2",
  "description": "Default OpenAI template",
  "createdBy": "john.doe",
  "version": "v1.0",
  "isLatest": true,
  "enabled": true,
  "logoUrl": "https://cdn.example.com/logos/openai.svg",
  "createdAt": "2025-11-25T10:30:00Z",
  "updatedAt": "2025-11-25T10:30:00Z",
  "readOnly": false
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|id|string|false|none|Unique handle for this specific template version.|
|groupId|string|false|read-only|Stable identifier shared by every version of a template family<br>(defaults to the first version's handle). Read-only.|
|displayName|string|true|none|Human-readable name for the LLM provider template|
|managedBy|string|false|none|Who manages the template ('wso2' for built-in, otherwise custom-defined).|
|description|string|false|none|none|
|createdBy|string|false|read-only|User identifier of the user who created this resource|
|version|string|false|none|Content version, matching the v<major>.<minor> pattern (e.g. v1.0, v2.0).|
|isLatest|boolean|false|none|Whether this is the latest version of the template.|
|enabled|boolean|false|none|Whether this version is offered when creating providers.|
|logoUrl|string(uri)|false|none|URL of the provider logo|
|createdAt|string(date-time)|false|none|none|
|updatedAt|string(date-time)|false|none|none|
|readOnly|boolean|false|none|True when the artifact originated from a data-plane gateway (origin gateway_api) and is read-only in the control plane.|

<h2 id="tocS_LLMProviderTemplateListResponse">LLMProviderTemplateListResponse</h2>

<a id="schemallmprovidertemplatelistresponse"></a>
<a id="schema_LLMProviderTemplateListResponse"></a>
<a id="tocSllmprovidertemplatelistresponse"></a>
<a id="tocsllmprovidertemplatelistresponse"></a>

```json
{
  "count": 5,
  "list": [
    {
      "id": "openai",
      "groupId": "openai",
      "displayName": "OpenAI",
      "managedBy": "wso2",
      "description": "Default OpenAI template",
      "createdBy": "john.doe",
      "version": "v1.0",
      "isLatest": true,
      "enabled": true,
      "logoUrl": "https://cdn.example.com/logos/openai.svg",
      "createdAt": "2025-11-25T10:30:00Z",
      "updatedAt": "2025-11-25T10:30:00Z",
      "readOnly": false
    }
  ],
  "pagination": {
    "total": 10,
    "offset": 0,
    "limit": 10
  }
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|count|integer|true|none|none|
|list|[[LLMProviderTemplateListItem](#schemallmprovidertemplatelistitem)]|true|none|none|
|pagination|[Pagination](#schemapagination)|true|none|none|

<h2 id="tocS_LLMAccessControl">LLMAccessControl</h2>

<a id="schemallmaccesscontrol"></a>
<a id="schema_LLMAccessControl"></a>
<a id="tocSllmaccesscontrol"></a>
<a id="tocsllmaccesscontrol"></a>

```json
{
  "mode": "deny_all",
  "exceptions": [
    {
      "path": "/chat/completions",
      "methods": [
        "GET"
      ]
    }
  ]
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|mode|string|true|none|Access control mode|
|exceptions|[[RouteException](#schemarouteexception)]|false|none|Path exceptions to the access control mode|

#### Enumerated Values

|Property|Value|
|---|---|
|mode|allow_all|
|mode|deny_all|

<h2 id="tocS_RouteException">RouteException</h2>

<a id="schemarouteexception"></a>
<a id="schema_RouteException"></a>
<a id="tocSrouteexception"></a>
<a id="tocsrouteexception"></a>

```json
{
  "path": "/chat/completions",
  "methods": [
    "GET"
  ]
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|path|string|true|none|Path pattern|
|methods|[string]|true|none|HTTP methods|

<h2 id="tocS_LLMPolicy">LLMPolicy</h2>

<a id="schemallmpolicy"></a>
<a id="schema_LLMPolicy"></a>
<a id="tocSllmpolicy"></a>
<a id="tocsllmpolicy"></a>

```json
{
  "name": "budgetControl",
  "version": "v1",
  "paths": [
    {
      "path": "/chat/completions",
      "methods": [
        "GET"
      ],
      "params": {}
    }
  ]
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|name|string|true|none|none|
|version|string|true|none|Version of the policy. Only major-only version is allowed (e.g., v0, v1). Full semantic version (e.g., v1.0.0) is not accepted and will be rejected.|
|paths|[[LLMPolicyPath](#schemallmpolicypath)]|true|none|none|

<h2 id="tocS_LLMPolicyPath">LLMPolicyPath</h2>

<a id="schemallmpolicypath"></a>
<a id="schema_LLMPolicyPath"></a>
<a id="tocSllmpolicypath"></a>
<a id="tocsllmpolicypath"></a>

```json
{
  "path": "/chat/completions",
  "methods": [
    "GET"
  ],
  "params": {}
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|path|string|true|none|none|
|methods|[string]|true|none|none|
|params|object|true|none|JSON Schema describing the parameters accepted by this policy. This itself is a JSON Schema document.|

<h2 id="tocS_OperationPolicy">OperationPolicy</h2>

<a id="schemaoperationpolicy"></a>
<a id="schema_OperationPolicy"></a>
<a id="tocSoperationpolicy"></a>
<a id="tocsoperationpolicy"></a>

```json
{
  "name": "token-based-ratelimit",
  "version": "v1",
  "executionCondition": "string",
  "paths": [
    {
      "path": "/chat/completions",
      "methods": [
        "GET"
      ],
      "params": {}
    }
  ]
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|name|string|true|none|none|
|version|string|true|none|Version of the policy. Only major-only version is allowed (e.g., v0, v1). Full semantic version (e.g., v1.0.0) is not accepted and will be rejected.|
|executionCondition|string|false|none|Optional per-request CEL expression controlling whether the policy runs|
|paths|[[OperationPolicyPath](#schemaoperationpolicypath)]|true|none|none|

<h2 id="tocS_OperationPolicyPath">OperationPolicyPath</h2>

<a id="schemaoperationpolicypath"></a>
<a id="schema_OperationPolicyPath"></a>
<a id="tocSoperationpolicypath"></a>
<a id="tocsoperationpolicypath"></a>

```json
{
  "path": "/chat/completions",
  "methods": [
    "GET"
  ],
  "params": {}
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|path|string|true|none|none|
|methods|[string]|true|none|none|
|params|object|true|none|Policy parameters|

<h2 id="tocS_LLMRateLimitingConfig">LLMRateLimitingConfig</h2>

<a id="schemallmratelimitingconfig"></a>
<a id="schema_LLMRateLimitingConfig"></a>
<a id="tocSllmratelimitingconfig"></a>
<a id="tocsllmratelimitingconfig"></a>

```json
{
  "providerLevel": {
    "global": {
      "request": {
        "enabled": true,
        "count": 1500,
        "reset": {
          "duration": 2,
          "unit": "week"
        }
      },
      "token": {
        "enabled": true,
        "count": 1000000,
        "reset": {
          "duration": 1,
          "unit": "month"
        }
      }
    }
  },
  "consumerLevel": {
    "resourceWise": {
      "default": {
        "request": {
          "enabled": true,
          "count": 50,
          "reset": {
            "duration": 2,
            "unit": "week"
          }
        },
        "cost": {
          "enabled": true,
          "amount": 100,
          "reset": {
            "duration": 1,
            "unit": "month"
          }
        }
      },
      "resources": [
        {
          "resource": "/models",
          "limit": {
            "request": {
              "enabled": true,
              "count": 200,
              "reset": {
                "duration": 1,
                "unit": "week"
              }
            },
            "token": {
              "enabled": true,
              "count": 100000,
              "reset": {
                "duration": 1,
                "unit": "month"
              }
            }
          }
        },
        {
          "resource": "/chat/completions",
          "limit": {
            "request": {
              "enabled": true,
              "count": 25,
              "reset": {
                "duration": 1,
                "unit": "week"
              }
            },
            "cost": {
              "enabled": true,
              "amount": 10,
              "reset": {
                "duration": 1,
                "unit": "month"
              }
            }
          }
        }
      ]
    }
  }
}

```

Rate limiting configuration for an LLM provider at provider and consumer levels.

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|providerLevel|[RateLimitingScopeConfig](#schemaratelimitingscopeconfig)|false|none|Rate limiting configuration for a scope (provider or consumer). Either global or resource-wise limits can be defined.|
|consumerLevel|[RateLimitingScopeConfig](#schemaratelimitingscopeconfig)|false|none|Rate limiting configuration for a scope (provider or consumer). Either global or resource-wise limits can be defined.|

<h2 id="tocS_RateLimitingScopeConfig">RateLimitingScopeConfig</h2>

<a id="schemaratelimitingscopeconfig"></a>
<a id="schema_RateLimitingScopeConfig"></a>
<a id="tocSratelimitingscopeconfig"></a>
<a id="tocsratelimitingscopeconfig"></a>

```json
{
  "global": {
    "request": {
      "enabled": true,
      "count": 1500,
      "reset": {
        "duration": 2,
        "unit": "week"
      }
    },
    "token": {
      "enabled": true,
      "count": 1000000,
      "reset": {
        "duration": 2,
        "unit": "week"
      }
    },
    "cost": {
      "enabled": true,
      "amount": 1000,
      "reset": {
        "duration": 2,
        "unit": "week"
      }
    }
  },
  "resourceWise": {
    "default": {
      "request": {
        "enabled": true,
        "count": 1500,
        "reset": {
          "duration": 2,
          "unit": "week"
        }
      },
      "token": {
        "enabled": true,
        "count": 1000000,
        "reset": {
          "duration": 2,
          "unit": "week"
        }
      },
      "cost": {
        "enabled": true,
        "amount": 1000,
        "reset": {
          "duration": 2,
          "unit": "week"
        }
      }
    },
    "resources": [
      {
        "resource": "/chat/completions",
        "limit": {
          "request": {
            "enabled": true,
            "count": 1500,
            "reset": {
              "duration": 2,
              "unit": "week"
            }
          },
          "token": {
            "enabled": true,
            "count": 1000000,
            "reset": {
              "duration": 2,
              "unit": "week"
            }
          },
          "cost": {
            "enabled": true,
            "amount": 1000,
            "reset": {
              "duration": 2,
              "unit": "week"
            }
          }
        }
      }
    ]
  }
}

```

Rate limiting configuration for a scope (provider or consumer). Either global or resource-wise limits can be defined.

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|global|[RateLimitingLimitConfig](#schemaratelimitinglimitconfig)|false|none|Limit definition with independent request/token/cost dimensions. If all dimensions are disabled (or absent), rate limiting is off for that scope.|
|resourceWise|[ResourceWiseRateLimitingConfig](#schemaresourcewiseratelimitingconfig)|false|none|Resource-specific limits with a required default limit.|

oneOf

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|object|false|none|none|

xor

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|object|false|none|none|

<h2 id="tocS_ResourceWiseRateLimitingConfig">ResourceWiseRateLimitingConfig</h2>

<a id="schemaresourcewiseratelimitingconfig"></a>
<a id="schema_ResourceWiseRateLimitingConfig"></a>
<a id="tocSresourcewiseratelimitingconfig"></a>
<a id="tocsresourcewiseratelimitingconfig"></a>

```json
{
  "default": {
    "request": {
      "enabled": true,
      "count": 1500,
      "reset": {
        "duration": 2,
        "unit": "week"
      }
    },
    "token": {
      "enabled": true,
      "count": 1000000,
      "reset": {
        "duration": 2,
        "unit": "week"
      }
    },
    "cost": {
      "enabled": true,
      "amount": 1000,
      "reset": {
        "duration": 2,
        "unit": "week"
      }
    }
  },
  "resources": [
    {
      "resource": "/chat/completions",
      "limit": {
        "request": {
          "enabled": true,
          "count": 1500,
          "reset": {
            "duration": 2,
            "unit": "week"
          }
        },
        "token": {
          "enabled": true,
          "count": 1000000,
          "reset": {
            "duration": 2,
            "unit": "week"
          }
        },
        "cost": {
          "enabled": true,
          "amount": 1000,
          "reset": {
            "duration": 2,
            "unit": "week"
          }
        }
      }
    }
  ]
}

```

Resource-specific limits with a required default limit.

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|default|[RateLimitingLimitConfig](#schemaratelimitinglimitconfig)|true|none|Limit definition with independent request/token/cost dimensions. If all dimensions are disabled (or absent), rate limiting is off for that scope.|
|resources|[[RateLimitingResourceLimit](#schemaratelimitingresourcelimit)]|true|none|Explicit resource limits that override the default limit.|

<h2 id="tocS_RateLimitingResourceLimit">RateLimitingResourceLimit</h2>

<a id="schemaratelimitingresourcelimit"></a>
<a id="schema_RateLimitingResourceLimit"></a>
<a id="tocSratelimitingresourcelimit"></a>
<a id="tocsratelimitingresourcelimit"></a>

```json
{
  "resource": "/chat/completions",
  "limit": {
    "request": {
      "enabled": true,
      "count": 1500,
      "reset": {
        "duration": 2,
        "unit": "week"
      }
    },
    "token": {
      "enabled": true,
      "count": 1000000,
      "reset": {
        "duration": 2,
        "unit": "week"
      }
    },
    "cost": {
      "enabled": true,
      "amount": 1000,
      "reset": {
        "duration": 2,
        "unit": "week"
      }
    }
  }
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|resource|string|true|none|Explicit resource path to apply the limit to.|
|limit|[RateLimitingLimitConfig](#schemaratelimitinglimitconfig)|true|none|Limit definition with independent request/token/cost dimensions. If all dimensions are disabled (or absent), rate limiting is off for that scope.|

<h2 id="tocS_RateLimitResetWindow">RateLimitResetWindow</h2>

<a id="schemaratelimitresetwindow"></a>
<a id="schema_RateLimitResetWindow"></a>
<a id="tocSratelimitresetwindow"></a>
<a id="tocsratelimitresetwindow"></a>

```json
{
  "duration": 2,
  "unit": "week"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|duration|integer|true|none|Reset duration for the limit window.|
|unit|string|true|none|Reset time unit for the limit window.|

#### Enumerated Values

|Property|Value|
|---|---|
|unit|minute|
|unit|hour|
|unit|day|
|unit|week|
|unit|month|

<h2 id="tocS_RequestRateLimitDimension">RequestRateLimitDimension</h2>

<a id="schemarequestratelimitdimension"></a>
<a id="schema_RequestRateLimitDimension"></a>
<a id="tocSrequestratelimitdimension"></a>
<a id="tocsrequestratelimitdimension"></a>

```json
{
  "enabled": true,
  "count": 1500,
  "reset": {
    "duration": 2,
    "unit": "week"
  }
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|enabled|boolean|false|none|Enable request-count based limiting.|
|count|integer|false|none|Maximum number of requests in the reset window.|
|reset|[RateLimitResetWindow](#schemaratelimitresetwindow)|false|none|none|

<h2 id="tocS_TokenRateLimitDimension">TokenRateLimitDimension</h2>

<a id="schematokenratelimitdimension"></a>
<a id="schema_TokenRateLimitDimension"></a>
<a id="tocStokenratelimitdimension"></a>
<a id="tocstokenratelimitdimension"></a>

```json
{
  "enabled": true,
  "count": 1000000,
  "reset": {
    "duration": 2,
    "unit": "week"
  }
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|enabled|boolean|false|none|Enable token-count based limiting.|
|count|integer|false|none|Maximum number of tokens in the reset window.|
|reset|[RateLimitResetWindow](#schemaratelimitresetwindow)|false|none|none|

<h2 id="tocS_CostRateLimitDimension">CostRateLimitDimension</h2>

<a id="schemacostratelimitdimension"></a>
<a id="schema_CostRateLimitDimension"></a>
<a id="tocScostratelimitdimension"></a>
<a id="tocscostratelimitdimension"></a>

```json
{
  "enabled": true,
  "amount": 1000,
  "reset": {
    "duration": 2,
    "unit": "week"
  }
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|enabled|boolean|false|none|Enable cost-based limiting.|
|amount|number(float)|false|none|Maximum cost in the reset window.|
|reset|[RateLimitResetWindow](#schemaratelimitresetwindow)|false|none|none|

<h2 id="tocS_RateLimitingLimitConfig">RateLimitingLimitConfig</h2>

<a id="schemaratelimitinglimitconfig"></a>
<a id="schema_RateLimitingLimitConfig"></a>
<a id="tocSratelimitinglimitconfig"></a>
<a id="tocsratelimitinglimitconfig"></a>

```json
{
  "request": {
    "enabled": true,
    "count": 1500,
    "reset": {
      "duration": 2,
      "unit": "week"
    }
  },
  "token": {
    "enabled": true,
    "count": 1000000,
    "reset": {
      "duration": 2,
      "unit": "week"
    }
  },
  "cost": {
    "enabled": true,
    "amount": 1000,
    "reset": {
      "duration": 2,
      "unit": "week"
    }
  }
}

```

Limit definition with independent request/token/cost dimensions. If all dimensions are disabled (or absent), rate limiting is off for that scope.

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|request|[RequestRateLimitDimension](#schemarequestratelimitdimension)|false|none|none|
|token|[TokenRateLimitDimension](#schematokenratelimitdimension)|false|none|none|
|cost|[CostRateLimitDimension](#schemacostratelimitdimension)|false|none|none|

<h2 id="tocS_LLMProvider">LLMProvider</h2>

<a id="schemallmprovider"></a>
<a id="schema_LLMProvider"></a>
<a id="tocSllmprovider"></a>
<a id="tocsllmprovider"></a>

```json
{
  "id": "wso2-openai-provider",
  "displayName": "WSO2 OpenAI Provider",
  "description": "Primary OpenAI provider",
  "createdBy": "john.doe",
  "readOnly": false,
  "updatedBy": "john.doe",
  "version": "v1.0",
  "context": "/openai",
  "vhost": "api.openai.com",
  "template": "openai",
  "openapi": "openapi: 3.0.3\ninfo:\n  title: Provider API\n  version: v1.0\npaths: {}\n",
  "modelProviders": [
    {
      "id": "claude",
      "displayName": "Claude",
      "models": [
        {
          "id": "claude-3-5-sonnet",
          "displayName": "Claude 3.5 Sonnet",
          "description": "High-quality reasoning model"
        }
      ]
    }
  ],
  "upstream": {
    "main": {
      "url": "http://prod-backend:5000/api/v2",
      "auth": {
        "type": "api-key",
        "header": "X-API-Key",
        "value": "my-api-key-value"
      }
    },
    "sandbox": {
      "url": "http://prod-backend:5000/api/v2",
      "auth": {
        "type": "api-key",
        "header": "X-API-Key",
        "value": "my-api-key-value"
      }
    }
  },
  "accessControl": {
    "mode": "deny_all",
    "exceptions": [
      {
        "path": "/chat/completions",
        "methods": [
          "GET"
        ]
      }
    ]
  },
  "rateLimiting": {
    "providerLevel": {
      "global": {
        "request": {
          "enabled": true,
          "count": 1500,
          "reset": {
            "duration": 2,
            "unit": "week"
          }
        },
        "token": {
          "enabled": true,
          "count": 1000000,
          "reset": {
            "duration": 1,
            "unit": "month"
          }
        }
      }
    },
    "consumerLevel": {
      "resourceWise": {
        "default": {
          "request": {
            "enabled": true,
            "count": 50,
            "reset": {
              "duration": 2,
              "unit": "week"
            }
          },
          "cost": {
            "enabled": true,
            "amount": 100,
            "reset": {
              "duration": 1,
              "unit": "month"
            }
          }
        },
        "resources": [
          {
            "resource": "/models",
            "limit": {
              "request": {
                "enabled": true,
                "count": 200,
                "reset": {
                  "duration": 1,
                  "unit": "week"
                }
              },
              "token": {
                "enabled": true,
                "count": 100000,
                "reset": {
                  "duration": 1,
                  "unit": "month"
                }
              }
            }
          },
          {
            "resource": "/chat/completions",
            "limit": {
              "request": {
                "enabled": true,
                "count": 25,
                "reset": {
                  "duration": 1,
                  "unit": "week"
                }
              },
              "cost": {
                "enabled": true,
                "amount": 10,
                "reset": {
                  "duration": 1,
                  "unit": "month"
                }
              }
            }
          }
        ]
      }
    }
  },
  "globalPolicies": [
    {
      "executionCondition": "request.header.x-custom == 'enabled'",
      "name": "SET_HEADER",
      "params": {
        "key": "MyHeader",
        "value": "MyValue"
      },
      "version": "v1"
    }
  ],
  "operationPolicies": [
    {
      "name": "token-based-ratelimit",
      "version": "v1",
      "executionCondition": "string",
      "paths": [
        {
          "path": "/chat/completions",
          "methods": [
            "GET"
          ],
          "params": {}
        }
      ]
    }
  ],
  "policies": [
    {
      "name": "budgetControl",
      "version": "v1",
      "paths": [
        {
          "path": "/chat/completions",
          "methods": [
            "GET"
          ],
          "params": {}
        }
      ]
    }
  ],
  "security": {
    "enabled": true,
    "apiKey": {
      "enabled": true,
      "key": "X-API-Key",
      "valuePrefix": "Bearer",
      "in": "header"
    }
  },
  "associatedGateways": [
    {
      "id": "prod-eu",
      "configurations": {
        "host": "prod-eu.platform-gw.local"
      }
    },
    {
      "id": "prod-us",
      "configurations": {
        "host": "prod-us.platform-gw.local"
      }
    }
  ],
  "createdAt": "2023-10-12T10:30:00Z",
  "updatedAt": "2023-10-12T10:30:00Z"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|id|string|false|none|Unique handle for the provider|
|displayName|string|true|none|Human-readable LLM Provider name|
|description|string|false|none|Description of the LLM provider|
|createdBy|string|false|read-only|User identifier of the user who created this resource|
|readOnly|boolean|false|read-only|True if the artifact originated from a data-plane gateway (origin gateway_api) and is read-only in the control plane; false for control-plane created artifacts.|
|updatedBy|string|false|read-only|User identifier of the user who last updated this resource. Only present in the detail response (GET /llm-providers/{id}), omitted from list responses.|
|version|string|true|none|Semantic version of the LLM Provider|
|context|string|false|none|Base path for all routes exposed by this proxy. Must start with / and carry no trailing slash; the single exception is the root path "/", which is the default.|
|vhost|string|false|none|Virtual host name used for routing. Supports standard domain names, subdomains, or wildcard domains. Must follow RFC-compliant hostname rules. Wildcards are only allowed in the left-most label (e.g., *.example.com).|
|template|string|true|none|Template name to use for this LLM Provider|
|openapi|string|false|none|OpenAPI specification (JSON or YAML) for the provider endpoint|
|modelProviders|[[LLMModelProvider](#schemallmmodelprovider)]|false|none|List of model providers and their models supported by this provider. For non-aggregator templates, only a single model provider is allowed (aggregator templates: awsbedrock, azureaifoundry).|
|upstream|[Upstream](#schemaupstream)|true|none|Upstream backend configuration with main and sandbox endpoints|
|accessControl|[LLMAccessControl](#schemallmaccesscontrol)|true|none|none|
|rateLimiting|[LLMRateLimitingConfig](#schemallmratelimitingconfig)|false|none|Rate limiting configuration for an LLM provider at provider and consumer levels.|
|globalPolicies|[[Policy](#schemapolicy)]|false|none|Global (api-level) policies applied across ALL operations as one shared scope, evaluated before operation-level policies.|
|operationPolicies|[[OperationPolicy](#schemaoperationpolicy)]|false|none|Operation-level policies scoped to specific paths/methods, evaluated after global policies.|
|policies|[[LLMPolicy](#schemallmpolicy)]|false|none|DEPRECATED - use operationPolicies. Still honoured (treated identically to operationPolicies).|
|security|[SecurityConfig](#schemasecurityconfig)|false|none|Defines security mechanisms (API key, OAuth2) applicable to the API|
|associatedGateways|[[AssociatedGateway](#schemaassociatedgateway)]|false|none|Optional list of gateways this LLM provider can be deployed to, along with per-gateway configuration overrides. This field is optional; omitting it does not change existing behaviour.|
|createdAt|string(date-time)|false|read-only|Timestamp when the resource was created|
|updatedAt|string(date-time)|false|read-only|Timestamp when the resource was last updated|

<h2 id="tocS_AssociatedGateway">AssociatedGateway</h2>

<a id="schemaassociatedgateway"></a>
<a id="schema_AssociatedGateway"></a>
<a id="tocSassociatedgateway"></a>
<a id="tocsassociatedgateway"></a>

```json
{
  "id": "prod-eu",
  "configurations": {
    "someKey": "someValue"
  }
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|id|string|true|none|Handle of the gateway this artifact can be deployed to|
|configurations|object|false|none|Per-gateway configuration overrides for this artifact. This is a free-form object; the supported keys depend on the deployed artifact type.|

<h2 id="tocS_LLMProviderListItem">LLMProviderListItem</h2>

<a id="schemallmproviderlistitem"></a>
<a id="schema_LLMProviderListItem"></a>
<a id="tocSllmproviderlistitem"></a>
<a id="tocsllmproviderlistitem"></a>

```json
{
  "id": "wso2-openai-provider",
  "displayName": "WSO2 OpenAI Provider",
  "description": "Primary OpenAI provider",
  "createdBy": "john.doe",
  "readOnly": false,
  "version": "v1.0",
  "template": "openai",
  "status": "deployed",
  "createdAt": "2025-11-25T10:30:00Z",
  "updatedAt": "2025-11-25T10:30:00Z"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|id|string|false|none|none|
|displayName|string|true|none|Human-readable name for the LLM provider|
|description|string|false|none|none|
|createdBy|string|false|read-only|User identifier of the user who created this resource|
|readOnly|boolean|false|none|True when the artifact originated from a data-plane gateway (origin gateway_api) and is read-only in the control plane.|
|version|string|false|none|none|
|template|string|false|none|none|
|status|string|false|none|none|
|createdAt|string(date-time)|false|none|none|
|updatedAt|string(date-time)|false|none|none|

#### Enumerated Values

|Property|Value|
|---|---|
|status|pending|
|status|deployed|
|status|failed|

<h2 id="tocS_LLMModelProvider">LLMModelProvider</h2>

<a id="schemallmmodelprovider"></a>
<a id="schema_LLMModelProvider"></a>
<a id="tocSllmmodelprovider"></a>
<a id="tocsllmmodelprovider"></a>

```json
{
  "id": "claude",
  "displayName": "Claude",
  "models": [
    {
      "id": "claude-3-5-sonnet",
      "displayName": "Claude 3.5 Sonnet",
      "description": "High-quality reasoning model"
    }
  ]
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|id|string|false|none|Unique model provider identifier|
|displayName|string|true|none|Human-readable model provider name|
|models|[[LLMModel](#schemallmmodel)]|false|none|Models under this model provider|

<h2 id="tocS_LLMModel">LLMModel</h2>

<a id="schemallmmodel"></a>
<a id="schema_LLMModel"></a>
<a id="tocSllmmodel"></a>
<a id="tocsllmmodel"></a>

```json
{
  "id": "claude-3-5-sonnet",
  "displayName": "Claude 3.5 Sonnet",
  "description": "High-quality reasoning model"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|id|string|false|none|Unique model identifier|
|displayName|string|true|none|Human-readable model name|
|description|string|false|none|Model description|

<h2 id="tocS_LLMProviderListResponse">LLMProviderListResponse</h2>

<a id="schemallmproviderlistresponse"></a>
<a id="schema_LLMProviderListResponse"></a>
<a id="tocSllmproviderlistresponse"></a>
<a id="tocsllmproviderlistresponse"></a>

```json
{
  "count": 2,
  "list": [
    {
      "id": "wso2-openai-provider",
      "displayName": "WSO2 OpenAI Provider",
      "description": "Primary OpenAI provider",
      "createdBy": "john.doe",
      "readOnly": false,
      "version": "v1.0",
      "template": "openai",
      "status": "deployed",
      "createdAt": "2025-11-25T10:30:00Z",
      "updatedAt": "2025-11-25T10:30:00Z"
    }
  ],
  "pagination": {
    "total": 10,
    "offset": 0,
    "limit": 10
  }
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|count|integer|true|none|none|
|list|[[LLMProviderListItem](#schemallmproviderlistitem)]|true|none|none|
|pagination|[Pagination](#schemapagination)|true|none|none|

<h2 id="tocS_LLMProxy">LLMProxy</h2>

<a id="schemallmproxy"></a>
<a id="schema_LLMProxy"></a>
<a id="tocSllmproxy"></a>
<a id="tocsllmproxy"></a>

```json
{
  "id": "wso2-con-assistant",
  "displayName": "WSO2 Con Assistant",
  "description": "Customer support assistant",
  "createdBy": "john.doe",
  "readOnly": false,
  "updatedBy": "john.doe",
  "version": "v1.0",
  "projectId": "550e8400-e29b-41d4-a716-446655440000",
  "context": "/openai",
  "vhost": "api.openai.com",
  "provider": {
    "id": "wso2-openai-provider",
    "auth": {
      "type": "api-key",
      "header": "X-API-Key",
      "value": "my-api-key-value"
    }
  },
  "additionalProviders": [
    {
      "id": "anthropic-provider",
      "as": "anthropic-upstream",
      "transformer": {
        "type": "openai-to-anthropic",
        "version": "v1",
        "params": {}
      }
    }
  ],
  "openapi": "openapi: 3.0.3\ninfo:\n  title: Proxy API\n  version: v1.0\npaths: {}\n",
  "globalPolicies": [
    {
      "executionCondition": "request.header.x-custom == 'enabled'",
      "name": "SET_HEADER",
      "params": {
        "key": "MyHeader",
        "value": "MyValue"
      },
      "version": "v1"
    }
  ],
  "operationPolicies": [
    {
      "name": "token-based-ratelimit",
      "version": "v1",
      "executionCondition": "string",
      "paths": [
        {
          "path": "/chat/completions",
          "methods": [
            "GET"
          ],
          "params": {}
        }
      ]
    }
  ],
  "policies": [
    {
      "name": "budgetControl",
      "version": "v1",
      "paths": [
        {
          "path": "/chat/completions",
          "methods": [
            "GET"
          ],
          "params": {}
        }
      ]
    }
  ],
  "security": {
    "enabled": true,
    "apiKey": {
      "enabled": true,
      "key": "X-API-Key",
      "valuePrefix": "Bearer",
      "in": "header"
    }
  },
  "associatedGateways": [
    {
      "id": "prod-eu"
    },
    {
      "id": "prod-us"
    }
  ],
  "createdAt": "2023-10-12T10:30:00Z",
  "updatedAt": "2023-10-12T10:30:00Z"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|id|string|false|none|Unique handle for the proxy|
|displayName|string|true|none|Human-readable LLM proxy name|
|description|string|false|none|Description of the LLM proxy|
|createdBy|string|false|read-only|User identifier of the user who created this resource|
|readOnly|boolean|false|read-only|True if the artifact originated from a data-plane gateway (origin gateway_api) and is read-only in the control plane; false for control-plane created artifacts.|
|updatedBy|string|false|read-only|User identifier of the user who last updated this resource. Only present in the detail response (GET /llm-proxies/{id}), omitted from list responses.|
|version|string|true|none|Semantic version of the LLM proxy|
|projectId|string|true|none|UUID of the project this proxy belongs to|
|context|string|false|none|Base path for all routes exposed by this proxy. Must start with / and carry no trailing slash; the single exception is the root path "/", which is the default.|
|vhost|string|false|none|Virtual host name used for routing. Supports standard domain names, subdomains, or wildcard domains. Must follow RFC-compliant hostname rules. Wildcards are only allowed in the left-most label (e.g., *.example.com).|
|provider|[LLMProxyProvider](#schemallmproxyprovider)|true|none|none|
|additionalProviders|[[LLMProxyAdditionalProvider](#schemallmproxyadditionalprovider)]|false|none|Optional list of additional LLM providers attached to this proxy as selectable upstreams. Policies route requests to any of these by setting the upstream name. The primary `provider` field above remains the default upstream and the FK target.|
|openapi|string|false|none|OpenAPI specification (JSON or YAML) for the proxy endpoint|
|globalPolicies|[[Policy](#schemapolicy)]|false|none|Global (api-level) policies applied across ALL operations as one shared scope, evaluated before operation-level policies.|
|operationPolicies|[[OperationPolicy](#schemaoperationpolicy)]|false|none|Operation-level policies scoped to specific paths/methods, evaluated after global policies.|
|policies|[[LLMPolicy](#schemallmpolicy)]|false|none|DEPRECATED - use operationPolicies. Still honoured (treated identically to operationPolicies).|
|security|[SecurityConfig](#schemasecurityconfig)|false|none|Defines security mechanisms (API key, OAuth2) applicable to the API|
|associatedGateways|[[AssociatedGateway](#schemaassociatedgateway)]|false|none|Optional list of gateways this LLM proxy can be deployed to, along with per-gateway configuration overrides. This field is optional; omitting it does not change existing behaviour.|
|createdAt|string(date-time)|false|read-only|Timestamp when the resource was created|
|updatedAt|string(date-time)|false|read-only|Timestamp when the resource was last updated|

<h2 id="tocS_LLMProxyListItem">LLMProxyListItem</h2>

<a id="schemallmproxylistitem"></a>
<a id="schema_LLMProxyListItem"></a>
<a id="tocSllmproxylistitem"></a>
<a id="tocsllmproxylistitem"></a>

```json
{
  "id": "wso2-con-assistant",
  "displayName": "WSO2 Con Assistant",
  "description": "Customer support assistant",
  "createdBy": "john.doe",
  "context": "/wso2-con-assistant",
  "version": "v1.0",
  "projectId": "550e8400-e29b-41d4-a716-446655440000",
  "provider": "wso2-openai-provider",
  "status": "deployed",
  "createdAt": "2025-11-25T10:30:00Z",
  "updatedAt": "2025-11-25T10:30:00Z",
  "readOnly": false
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|id|string|false|none|none|
|displayName|string|true|none|Human-readable name for the LLM proxy|
|description|string|false|none|none|
|createdBy|string|false|read-only|User identifier of the user who created this resource|
|context|string|false|none|Context path where the proxy is exposed|
|version|string|false|none|none|
|projectId|string|false|none|UUID of the project this proxy belongs to|
|provider|string|false|none|Unique id of a deployed llm provider|
|status|string|false|none|none|
|createdAt|string(date-time)|false|none|none|
|updatedAt|string(date-time)|false|none|none|
|readOnly|boolean|false|none|True when the artifact originated from a data-plane gateway (origin gateway_api) and is read-only in the control plane.|

#### Enumerated Values

|Property|Value|
|---|---|
|status|pending|
|status|deployed|
|status|failed|

<h2 id="tocS_LLMProxyListResponse">LLMProxyListResponse</h2>

<a id="schemallmproxylistresponse"></a>
<a id="schema_LLMProxyListResponse"></a>
<a id="tocSllmproxylistresponse"></a>
<a id="tocsllmproxylistresponse"></a>

```json
{
  "count": 2,
  "list": [
    {
      "id": "wso2-con-assistant",
      "displayName": "WSO2 Con Assistant",
      "description": "Customer support assistant",
      "createdBy": "john.doe",
      "context": "/wso2-con-assistant",
      "version": "v1.0",
      "projectId": "550e8400-e29b-41d4-a716-446655440000",
      "provider": "wso2-openai-provider",
      "status": "deployed",
      "createdAt": "2025-11-25T10:30:00Z",
      "updatedAt": "2025-11-25T10:30:00Z",
      "readOnly": false
    }
  ],
  "pagination": {
    "total": 10,
    "offset": 0,
    "limit": 10
  }
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|count|integer|true|none|none|
|list|[[LLMProxyListItem](#schemallmproxylistitem)]|true|none|none|
|pagination|[Pagination](#schemapagination)|true|none|none|

<h2 id="tocS_LLMProxyProvider">LLMProxyProvider</h2>

<a id="schemallmproxyprovider"></a>
<a id="schema_LLMProxyProvider"></a>
<a id="tocSllmproxyprovider"></a>
<a id="tocsllmproxyprovider"></a>

```json
{
  "id": "wso2-openai-provider",
  "auth": {
    "type": "api-key",
    "header": "X-API-Key",
    "value": "my-api-key-value"
  }
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|id|string|true|none|Unique id of a deployed llm provider|
|auth|[UpstreamAuth](#schemaupstreamauth)|false|none|Authentication configuration for upstream endpoints|

<h2 id="tocS_LLMProxyAdditionalProvider">LLMProxyAdditionalProvider</h2>

<a id="schemallmproxyadditionalprovider"></a>
<a id="schema_LLMProxyAdditionalProvider"></a>
<a id="tocSllmproxyadditionalprovider"></a>
<a id="tocsllmproxyadditionalprovider"></a>

```json
{
  "id": "anthropic-provider",
  "as": "anthropic-upstream",
  "transformer": {
    "type": "openai-to-anthropic",
    "version": "v1",
    "params": {}
  }
}

```

Additional LLM provider attached to this proxy as a selectable upstream. Policies route to it by referring to the `as` name (defaults to `id`).

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|id|string|true|none|Unique id of a deployed llm provider|
|as|string|false|none|Logical LLM Provider name used by policies to select this provider. Must be unique within the proxy. Defaults to `id` when omitted.|
|transformer|[LLMProxyTransformer](#schemallmproxytransformer)|false|none|Request/response translator applied when this provider is the selected upstream. The proxy injects the translator as a conditional policy whose execution condition matches this provider, so it runs only when the provider is selected. The provider's `as` name (defaults to `id`) is passed to the translator as its target upstream.|

<h2 id="tocS_LLMProxyTransformer">LLMProxyTransformer</h2>

<a id="schemallmproxytransformer"></a>
<a id="schema_LLMProxyTransformer"></a>
<a id="tocSllmproxytransformer"></a>
<a id="tocsllmproxytransformer"></a>

```json
{
  "type": "openai-to-anthropic",
  "version": "v1",
  "params": {}
}

```

Request/response translator applied when this provider is the selected upstream. The proxy injects the translator as a conditional policy whose execution condition matches this provider, so it runs only when the provider is selected. The provider's `as` name (defaults to `id`) is passed to the translator as its target upstream.

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|type|string|true|none|Translator policy name (for example openai-to-anthropic).|
|version|string|true|none|Major-only translator policy version (for example v1). The Gateway Controller resolves it to the installed full version.|
|params|object|false|none|Translator-specific parameters (for example model, apiVersion).|

<h2 id="tocS_CreateLLMProviderAPIKeyRequest">CreateLLMProviderAPIKeyRequest</h2>

<a id="schemacreatellmproviderapikeyrequest"></a>
<a id="schema_CreateLLMProviderAPIKeyRequest"></a>
<a id="tocScreatellmproviderapikeyrequest"></a>
<a id="tocscreatellmproviderapikeyrequest"></a>

```json
{
  "id": "production-key",
  "displayName": "Production Key",
  "expiresAt": "2026-12-31T23:59:59Z",
  "issuer": "api-platform-devportal",
  "allowedTargets": "dev_gateway,test_gateway"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|id|string|false|none|Unique identifier for the API key within the LLM provider. If not provided, generated from displayName.|
|displayName|string|true|none|Human-readable name for the API key|
|expiresAt|string(date-time)|false|none|Optional expiration time in ISO 8601 format|
|issuer|string¦null|false|none|Identifier of the developer portal that provisioned this API key. Null if not provided.|
|allowedTargets|string¦null|false|none|Comma-separated list of gateways this key is valid for.<br>Use 'ALL' to allow all targets (default).|

<h2 id="tocS_CreateLLMProviderAPIKeyResponse">CreateLLMProviderAPIKeyResponse</h2>

<a id="schemacreatellmproviderapikeyresponse"></a>
<a id="schema_CreateLLMProviderAPIKeyResponse"></a>
<a id="tocScreatellmproviderapikeyresponse"></a>
<a id="tocscreatellmproviderapikeyresponse"></a>

```json
{
  "status": "success",
  "message": "API key created and broadcasted to gateways successfully",
  "id": "production-key",
  "apiKey": "REDACTED_API_KEY"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|status|string|true|none|Status of the operation|
|message|string|true|none|Detailed message about the operation result|
|id|string|true|none|Unique identifier of the generated key|
|apiKey|string|true|none|The generated API key value — 64 hexadecimal characters, returned only in this creation response and never retrievable afterwards. The example value is a non-functional placeholder.|

<h2 id="tocS_CreateLLMProxyAPIKeyRequest">CreateLLMProxyAPIKeyRequest</h2>

<a id="schemacreatellmproxyapikeyrequest"></a>
<a id="schema_CreateLLMProxyAPIKeyRequest"></a>
<a id="tocScreatellmproxyapikeyrequest"></a>
<a id="tocscreatellmproxyapikeyrequest"></a>

```json
{
  "id": "production-key",
  "displayName": "Production Key",
  "expiresAt": "2026-12-31T23:59:59Z",
  "issuer": "api-platform-devportal",
  "allowedTargets": "dev_gateway,test_gateway"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|id|string|false|none|Unique identifier for the API key within the LLM proxy. If not provided, generated from displayName.|
|displayName|string|true|none|Human-readable name for the API key|
|expiresAt|string(date-time)|false|none|Optional expiration time in ISO 8601 format|
|issuer|string¦null|false|none|Identifier of the developer portal that provisioned this API key. Null if not provided.|
|allowedTargets|string¦null|false|none|Comma-separated list of gateways this key is valid for.<br>Use 'ALL' to allow all targets (default).|

<h2 id="tocS_CreateLLMProxyAPIKeyResponse">CreateLLMProxyAPIKeyResponse</h2>

<a id="schemacreatellmproxyapikeyresponse"></a>
<a id="schema_CreateLLMProxyAPIKeyResponse"></a>
<a id="tocScreatellmproxyapikeyresponse"></a>
<a id="tocscreatellmproxyapikeyresponse"></a>

```json
{
  "status": "success",
  "message": "API key created and broadcasted to gateways successfully",
  "id": "production-key",
  "apiKey": "REDACTED_API_KEY"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|status|string|true|none|Status of the operation|
|message|string|true|none|Detailed message about the operation result|
|id|string|true|none|Unique identifier of the generated key|
|apiKey|string|true|none|The generated API key value — 64 hexadecimal characters, returned only in this creation response and never retrievable afterwards. The example value is a non-functional placeholder.|

<h2 id="tocS_MCPProxy">MCPProxy</h2>

<a id="schemamcpproxy"></a>
<a id="schema_MCPProxy"></a>
<a id="tocSmcpproxy"></a>
<a id="tocsmcpproxy"></a>

```json
{
  "id": "weather-mcp-proxy",
  "displayName": "Weather Server",
  "description": "An MCP server which provides weather information",
  "createdBy": "john.doe",
  "readOnly": false,
  "updatedBy": "john.doe",
  "version": "v1.0",
  "projectId": "550e8400-e29b-41d4-a716-446655440000",
  "context": "/",
  "vhost": "mcp.gw.com",
  "upstream": {
    "main": {
      "url": "http://prod-backend:5000/api/v2",
      "auth": {
        "type": "api-key",
        "header": "X-API-Key",
        "value": "my-api-key-value"
      }
    },
    "sandbox": {
      "url": "http://prod-backend:5000/api/v2",
      "auth": {
        "type": "api-key",
        "header": "X-API-Key",
        "value": "my-api-key-value"
      }
    }
  },
  "mcpSpecVersion": "2025-06-18",
  "policies": [
    {
      "executionCondition": "request.header.x-custom == 'enabled'",
      "name": "SET_HEADER",
      "params": {
        "key": "MyHeader",
        "value": "MyValue"
      },
      "version": "v1"
    }
  ],
  "kind": "Mcp",
  "capabilities": {
    "tools": [
      {}
    ],
    "resources": [
      {}
    ],
    "prompts": [
      {}
    ]
  },
  "associatedGateways": [
    {
      "id": "prod-eu"
    },
    {
      "id": "prod-us"
    }
  ],
  "createdAt": "2019-08-24T14:15:22Z",
  "updatedAt": "2019-08-24T14:15:22Z"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|id|string|false|none|Unique handle for the proxy|
|displayName|string|true|none|Human-readable MCP proxy name|
|description|string|false|none|Description of the MCP proxy|
|createdBy|string|false|read-only|User identifier of the user who created this resource|
|readOnly|boolean|false|read-only|True if the artifact originated from a data-plane gateway (origin gateway_api) and is read-only in the control plane; false for control-plane created artifacts.|
|updatedBy|string|false|read-only|User identifier of the user who last updated this resource. Only present in the detail response (GET /mcp-proxies/{id}), omitted from list responses.|
|version|string|true|none|Semantic version of the MCP proxy|
|projectId|string|false|none|UUID of the project this proxy belongs to|
|context|string|false|none|Base path for all routes exposed by this proxy. Must start with / and carry no trailing slash; the single exception is the root path "/", which is the default.|
|vhost|string|false|none|Virtual host name used for routing. Supports standard domain names, subdomains, or wildcard domains. Must follow RFC-compliant hostname rules. Wildcards are only allowed in the left-most label (e.g., *.example.com).|
|upstream|[Upstream](#schemaupstream)|true|none|Upstream backend configuration with main and sandbox endpoints|
|mcpSpecVersion|string|false|none|MCP specification version supported by this proxy|
|policies|[[Policy](#schemapolicy)]|false|none|List of policies to be applied|
|kind|string|false|none|Kind of the API based on its communication protocol or architectural style|
|capabilities|[MCPProxyCapabilities](#schemamcpproxycapabilities)|false|none|List of capabilities supported by this proxy. This will be stored as-is and can be used in the future if we need this for governance purposes|
|associatedGateways|[[AssociatedGateway](#schemaassociatedgateway)]|false|none|Optional list of gateways this MCP proxy can be deployed to, along with per-gateway configuration overrides. This field is optional; omitting it does not change existing behaviour.|
|createdAt|string(date-time)|false|read-only|Timestamp when the resource was created|
|updatedAt|string(date-time)|false|read-only|Timestamp when the resource was last updated|

#### Enumerated Values

|Property|Value|
|---|---|
|mcpSpecVersion|2025-06-18|
|mcpSpecVersion|2025-11-25|

<h2 id="tocS_MCPProxyListItem">MCPProxyListItem</h2>

<a id="schemamcpproxylistitem"></a>
<a id="schema_MCPProxyListItem"></a>
<a id="tocSmcpproxylistitem"></a>
<a id="tocsmcpproxylistitem"></a>

```json
{
  "id": "weather-mcp-proxy",
  "displayName": "Weather Server",
  "description": "An MCP server which provides weather information",
  "createdBy": "john.doe",
  "context": "/weather-mcp-proxy",
  "version": "v1.0",
  "projectId": "550e8400-e29b-41d4-a716-446655440000",
  "status": "deployed",
  "mcpSpecVersion": "2025-11-25",
  "createdAt": "2025-11-25T10:30:00Z",
  "updatedAt": "2025-11-25T10:30:00Z",
  "readOnly": false
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|id|string|false|none|none|
|displayName|string|true|none|Human-readable name for the MCP proxy|
|description|string|false|none|none|
|createdBy|string|false|read-only|User identifier of the user who created this resource|
|context|string|false|none|Context path where the proxy is exposed|
|version|string|false|none|none|
|projectId|string|false|none|UUID of the project this proxy belongs to|
|status|string|false|none|none|
|mcpSpecVersion|string|false|none|none|
|createdAt|string(date-time)|false|none|none|
|updatedAt|string(date-time)|false|none|none|
|readOnly|boolean|false|none|True when the artifact originated from a data-plane gateway (origin gateway_api) and is read-only in the control plane.|

#### Enumerated Values

|Property|Value|
|---|---|
|status|pending|
|status|deployed|
|status|failed|

<h2 id="tocS_MCPProxyListResponse">MCPProxyListResponse</h2>

<a id="schemamcpproxylistresponse"></a>
<a id="schema_MCPProxyListResponse"></a>
<a id="tocSmcpproxylistresponse"></a>
<a id="tocsmcpproxylistresponse"></a>

```json
{
  "count": 2,
  "list": [
    {
      "id": "weather-mcp-proxy",
      "displayName": "Weather Server",
      "description": "An MCP server which provides weather information",
      "createdBy": "john.doe",
      "context": "/weather-mcp-proxy",
      "version": "v1.0",
      "projectId": "550e8400-e29b-41d4-a716-446655440000",
      "status": "deployed",
      "mcpSpecVersion": "2025-11-25",
      "createdAt": "2025-11-25T10:30:00Z",
      "updatedAt": "2025-11-25T10:30:00Z",
      "readOnly": false
    }
  ],
  "pagination": {
    "total": 10,
    "offset": 0,
    "limit": 10
  }
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|count|integer|true|none|none|
|list|[[MCPProxyListItem](#schemamcpproxylistitem)]|true|none|none|
|pagination|[Pagination](#schemapagination)|true|none|none|

<h2 id="tocS_MCPServerInfoFetchRequest">MCPServerInfoFetchRequest</h2>

<a id="schemamcpserverinfofetchrequest"></a>
<a id="schema_MCPServerInfoFetchRequest"></a>
<a id="tocSmcpserverinfofetchrequest"></a>
<a id="tocsmcpserverinfofetchrequest"></a>

```json
{
  "url": "https://mcp.server.com/mcp",
  "proxyId": "my-mcp-proxy",
  "auth": {
    "type": "api-key",
    "header": "X-API-Key",
    "value": "my-api-key-value"
  }
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|url|string(uri)|false|none|Endpoint URL of the MCP server to fetch information from.<br>Required when proxyId is not provided. When proxyId is provided,<br>the URL from the stored proxy configuration is used.|
|proxyId|string|false|none|MCP proxy handle (identifier) for refresh operations. When provided,<br>the server fetches URL and auth from the stored proxy configuration.<br>Auth override is not allowed in refetch mode.|
|auth|[UpstreamAuth](#schemaupstreamauth)|false|none|Authentication configuration for the fetch request.<br>Only allowed when proxyId is not provided (initial creation flow).|

<h2 id="tocS_MCPServerInfoFetchResponse">MCPServerInfoFetchResponse</h2>

<a id="schemamcpserverinfofetchresponse"></a>
<a id="schema_MCPServerInfoFetchResponse"></a>
<a id="tocSmcpserverinfofetchresponse"></a>
<a id="tocsmcpserverinfofetchresponse"></a>

```json
{
  "serverInfo": {},
  "tools": [
    {}
  ],
  "resources": [
    {}
  ],
  "prompts": [
    {}
  ]
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|serverInfo|object|false|none|none|
|tools|[object]|false|none|none|
|resources|[object]|false|none|none|
|prompts|[object]|false|none|none|

<h2 id="tocS_MCPProxyCapabilities">MCPProxyCapabilities</h2>

<a id="schemamcpproxycapabilities"></a>
<a id="schema_MCPProxyCapabilities"></a>
<a id="tocSmcpproxycapabilities"></a>
<a id="tocsmcpproxycapabilities"></a>

```json
{
  "tools": [
    {}
  ],
  "resources": [
    {}
  ],
  "prompts": [
    {}
  ]
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|tools|[object]|false|none|List of tool capabilities supported by this proxy|
|resources|[object]|false|none|List of resource capabilities supported by this proxy|
|prompts|[object]|false|none|List of prompt capabilities supported by this proxy|

<h2 id="tocS_SecretCreateRequest">SecretCreateRequest</h2>

<a id="schemasecretcreaterequest"></a>
<a id="schema_SecretCreateRequest"></a>
<a id="tocSsecretcreaterequest"></a>
<a id="tocssecretcreaterequest"></a>

```json
{
  "id": "wso2-openai-key",
  "displayName": "WSO2 OpenAI API Key",
  "description": "Primary API key for WSO2 OpenAI integration",
  "value": "sk-xxx",
  "type": "GENERIC"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|id|string|false|none|Handle (slug) used in {{ secret "id" }} placeholders. Immutable after creation.|
|displayName|string|true|none|Human-readable name for the secret|
|description|string|false|none|none|
|value|string|true|none|Plaintext secret value — encrypted at rest, never returned in any response|
|type|string|false|none|none|

#### Enumerated Values

|Property|Value|
|---|---|
|type|GENERIC|
|type|CERTIFICATE|

<h2 id="tocS_SecretUpdateRequest">SecretUpdateRequest</h2>

<a id="schemasecretupdaterequest"></a>
<a id="schema_SecretUpdateRequest"></a>
<a id="tocSsecretupdaterequest"></a>
<a id="tocssecretupdaterequest"></a>

```json
{
  "id": "wso2-openai-key",
  "displayName": "string",
  "description": "string",
  "value": "string"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|id|string|false|none|Secret handle — if provided, must match the path parameter; returns 400 if they differ. The handle is immutable and cannot be changed via update.|
|displayName|string|true|none|Human-readable name for the secret|
|description|string|false|none|none|
|value|string|true|none|New plaintext secret value — re-encrypted at rest|

<h2 id="tocS_SecretResponse">SecretResponse</h2>

<a id="schemasecretresponse"></a>
<a id="schema_SecretResponse"></a>
<a id="tocSsecretresponse"></a>
<a id="tocssecretresponse"></a>

```json
{
  "id": "wso2-openai-key",
  "displayName": "WSO2 OpenAI API Key",
  "createdBy": "john.doe",
  "updatedBy": "john.doe",
  "createdAt": "2019-08-24T14:15:22Z",
  "updatedAt": "2019-08-24T14:15:22Z"
}

```

Returned on create (201) and rotate (200). The plaintext value is never included.

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|id|string|false|none|none|
|displayName|string|true|none|Human-readable name for the secret|
|createdBy|string|false|read-only|User identifier of the user who created this resource|
|updatedBy|string|false|read-only|User identifier of the user who last updated this resource|
|createdAt|string(date-time)|false|none|none|
|updatedAt|string(date-time)|false|none|none|

<h2 id="tocS_SecretSummary">SecretSummary</h2>

<a id="schemasecretsummary"></a>
<a id="schema_SecretSummary"></a>
<a id="tocSsecretsummary"></a>
<a id="tocssecretsummary"></a>

```json
{
  "id": "wso2-openai-key",
  "displayName": "WSO2 OpenAI API Key",
  "description": "string",
  "type": "GENERIC",
  "provider": "IN_BUILT",
  "status": "ACTIVE",
  "hash": "hmac-sha256:b94d27b9934d3e08a52e52d7da7dabfac484efe04294e576d4b3d4c57e3f428a",
  "createdBy": "john.doe",
  "createdAt": "2019-08-24T14:15:22Z",
  "updatedAt": "2019-08-24T14:15:22Z"
}

```

Secret metadata — never includes the plaintext value.

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|id|string|false|none|none|
|displayName|string|true|none|Human-readable name for the secret|
|description|string|false|none|none|
|type|string|false|none|none|
|provider|string|false|none|none|
|status|string|false|none|none|
|hash|string|false|none|none|
|createdBy|string|false|read-only|User identifier of the user who created this resource|
|createdAt|string(date-time)|false|none|none|
|updatedAt|string(date-time)|false|none|none|

#### Enumerated Values

|Property|Value|
|---|---|
|type|GENERIC|
|type|CERTIFICATE|
|provider|IN_BUILT|
|status|ACTIVE|
|status|DEPRECATED|

<h2 id="tocS_SecretListResponse">SecretListResponse</h2>

<a id="schemasecretlistresponse"></a>
<a id="schema_SecretListResponse"></a>
<a id="tocSsecretlistresponse"></a>
<a id="tocssecretlistresponse"></a>

```json
{
  "count": 0,
  "list": [
    {
      "id": "wso2-openai-key",
      "displayName": "WSO2 OpenAI API Key",
      "description": "string",
      "type": "GENERIC",
      "provider": "IN_BUILT",
      "status": "ACTIVE",
      "hash": "hmac-sha256:b94d27b9934d3e08a52e52d7da7dabfac484efe04294e576d4b3d4c57e3f428a",
      "createdBy": "john.doe",
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

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|count|integer|true|none|Number of secrets in current response|
|list|[[SecretSummary](#schemasecretsummary)]|true|none|[Secret metadata — never includes the plaintext value.]|
|pagination|[Pagination](#schemapagination)|true|none|none|

<h2 id="tocS_GatewayTokenListResponse">GatewayTokenListResponse</h2>

<a id="schemagatewaytokenlistresponse"></a>
<a id="schema_GatewayTokenListResponse"></a>
<a id="tocSgatewaytokenlistresponse"></a>
<a id="tocsgatewaytokenlistresponse"></a>

```json
{
  "count": 0,
  "list": [
    {
      "id": "abc12345-f678-90de-f123-456789abcdef",
      "status": "active",
      "createdAt": "2025-10-14T10:30:00Z",
      "revokedAt": null
    }
  ],
  "pagination": {
    "total": 10,
    "offset": 0,
    "limit": 10
  }
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|count|integer|true|none|Number of tokens in current response|
|list|[[TokenInfoResponse](#schematokeninforesponse)]|true|none|List of active tokens|
|pagination|[Pagination](#schemapagination)|true|none|none|

<h2 id="tocS_CustomPolicyListResponse">CustomPolicyListResponse</h2>

<a id="schemacustompolicylistresponse"></a>
<a id="schema_CustomPolicyListResponse"></a>
<a id="tocScustompolicylistresponse"></a>
<a id="tocscustompolicylistresponse"></a>

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

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|count|integer|true|none|Number of custom policies in current response|
|list|[[CustomPolicyResponse](#schemacustompolicyresponse)]|true|none|List of custom policies|
|pagination|[Pagination](#schemapagination)|true|none|none|
