# Schemas

<h2 id="tocS_APIKey">APIKey</h2>

<a id="schemaapikey"></a>
<a id="schema_APIKey"></a>
<a id="tocSapikey"></a>
<a id="tocsapikey"></a>

```json
{
  "name": "my-production-key",
  "displayName": "My Production Key",
  "apiKey": "apip_1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
  "apiId": "reading-list-api-v1.0",
  "status": "active",
  "createdAt": "2026-04-01T10:30:00Z",
  "createdBy": "admin",
  "expiresAt": null,
  "source": "local"
}

```

Details of an API key

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|name|string|true|none|URL-safe identifier for the API key (auto-generated from displayName, immutable, used as path parameter)|
|displayName|string|false|none|Human-readable name for the API key (user-provided, mutable)|
|apiKey|string|false|none|Generated API key with apip_ prefix|
|apiId|string|true|none|Unique public identifier of the API that the key is associated with|
|status|string|true|none|Status of the API key|
|createdAt|string(date-time)|true|none|Timestamp when the API key was generated|
|createdBy|string|true|none|Identifier of the user who generated the API key|
|expiresAt|string(date-time)¦null|true|none|Expiration timestamp (null if no expiration)|
|source|string|true|none|Source of the API key (local or external)|
|externalRefId|string|false|none|External reference ID for the API key|

#### Enumerated Values

|Property|Value|
|---|---|
|status|active|
|status|revoked|
|status|expired|
|source|local|
|source|external|

<h2 id="tocS_APIKeyCreationRequest">APIKeyCreationRequest</h2>

<a id="schemaapikeycreationrequest"></a>
<a id="schema_APIKeyCreationRequest"></a>
<a id="tocSapikeycreationrequest"></a>
<a id="tocsapikeycreationrequest"></a>

```json
{
  "name": "my-production-key"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|name|string|false|none|Identifier of the API key. If not provided, a default identifier will be generated|
|apiKey|string|false|none|Optional plain-text API key value for external key injection.<br>If provided, this key will be used instead of generating a new one.<br>The key will be hashed before storage. The key can be in any format<br>(minimum 36 characters). Use this for injecting externally generated<br>API keys.|
|maskedApiKey|string|false|none|Masked version of the API key for display purposes.<br>Provided by the platform API when injecting pre-hashed keys.|
|expiresIn|object|false|none|Expiration duration for the API key|
|» unit|string|true|none|Time unit for expiration|
|» duration|integer|true|none|Duration value for expiration|
|expiresAt|string(date-time)|false|none|Expiration timestamp. If both expiresIn and expiresAt are provided, expiresAt takes precedence.|
|externalRefId|string|false|none|External reference ID for the API key.<br>This field is optional and used for tracing purposes only.<br>The gateway generates its own internal ID for tracking.|
|issuer|string|false|none|Identifies the portal that created this key. If provided, only api keys generated from<br>the same portal will be accepted. If not provided, there is no portal restriction.|

#### Enumerated Values

|Property|Value|
|---|---|
|unit|seconds|
|unit|minutes|
|unit|hours|
|unit|days|
|unit|weeks|
|unit|months|

<h2 id="tocS_APIKeyCreationResponse">APIKeyCreationResponse</h2>

<a id="schemaapikeycreationresponse"></a>
<a id="schema_APIKeyCreationResponse"></a>
<a id="tocSapikeycreationresponse"></a>
<a id="tocsapikeycreationresponse"></a>

```json
{
  "status": "success",
  "message": "API key generated successfully",
  "remainingApiKeyQuota": 9,
  "apiKey": {
    "name": "my-production-key",
    "displayName": "My Production Key",
    "apiKey": "apip_1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
    "apiId": "reading-list-api-v1.0",
    "status": "active",
    "createdAt": "2026-04-01T10:30:00Z",
    "createdBy": "admin",
    "expiresAt": null,
    "source": "local"
  }
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|status|string|true|none|none|
|message|string|true|none|none|
|remainingApiKeyQuota|integer|false|none|Remaining API key quota for the user|
|apiKey|[APIKey](#schemaapikey)|false|none|Details of an API key|

<h2 id="tocS_APIKeyListResponse">APIKeyListResponse</h2>

<a id="schemaapikeylistresponse"></a>
<a id="schema_APIKeyListResponse"></a>
<a id="tocSapikeylistresponse"></a>
<a id="tocsapikeylistresponse"></a>

```json
{
  "apiKeys": [
    {
      "name": "my-production-key",
      "displayName": "My Production Key",
      "apiKey": "apip_1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
      "apiId": "reading-list-api-v1.0",
      "status": "active",
      "createdAt": "2026-04-01T10:30:00Z",
      "createdBy": "admin",
      "expiresAt": null,
      "source": "local"
    }
  ],
  "totalCount": 3,
  "status": "success"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|apiKeys|[[APIKey](#schemaapikey)]|false|none|[Details of an API key]|
|totalCount|integer|false|none|Total number of API keys|
|status|string|false|none|none|

<h2 id="tocS_APIKeyRegenerationRequest">APIKeyRegenerationRequest</h2>

<a id="schemaapikeyregenerationrequest"></a>
<a id="schema_APIKeyRegenerationRequest"></a>
<a id="tocSapikeyregenerationrequest"></a>
<a id="tocsapikeyregenerationrequest"></a>

```json
{}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|expiresIn|object|false|none|Expiration duration for the API key|
|» unit|string|true|none|Time unit for expiration|
|» duration|integer|true|none|Duration value for expiration|
|expiresAt|string(date-time)|false|none|Expiration timestamp|

#### Enumerated Values

|Property|Value|
|---|---|
|unit|seconds|
|unit|minutes|
|unit|hours|
|unit|days|
|unit|weeks|
|unit|months|

<h2 id="tocS_APIKeyRevocationResponse">APIKeyRevocationResponse</h2>

<a id="schemaapikeyrevocationresponse"></a>
<a id="schema_APIKeyRevocationResponse"></a>
<a id="tocSapikeyrevocationresponse"></a>
<a id="tocsapikeyrevocationresponse"></a>

```json
{
  "status": "success",
  "message": "API key revoked successfully"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|status|string|true|none|none|
|message|string|true|none|none|

<h2 id="tocS_APIKeyUpdateRequest">APIKeyUpdateRequest</h2>

<a id="schemaapikeyupdaterequest"></a>
<a id="schema_APIKeyUpdateRequest"></a>
<a id="tocSapikeyupdaterequest"></a>
<a id="tocsapikeyupdaterequest"></a>

```json
{
  "name": "my-production-key"
}

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
  "message": "Configuration validation failed",
  "errors": [
    {
      "field": "spec.context",
      "message": "Context must start with / and cannot end with /"
    }
  ]
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|status|string|true|none|none|
|message|string|true|none|High-level error description|
|errors|[[ValidationError](#schemavalidationerror)]|false|none|Detailed validation errors|

<h2 id="tocS_Metadata">Metadata</h2>

<a id="schemametadata"></a>
<a id="schema_Metadata"></a>
<a id="tocSmetadata"></a>
<a id="tocsmetadata"></a>

```json
{
  "name": "reading-list-api-v1.0",
  "labels": {
    "environment": "production",
    "team": "backend",
    "version": "v1"
  },
  "annotations": {
    "gateway.api-platform.wso2.com/project-id": "019d953f-d386-7a64-aa92-1869a28292e0"
  }
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|name|string|true|none|Unique handle for the resource|
|labels|object|false|none|Labels are key-value pairs for organizing and selecting APIs. Keys must not contain spaces.|
|» **additionalProperties**|string|false|none|none|
|annotations|object|false|none|Annotations are arbitrary non-identifying metadata. Use domain-prefixed keys.|
|» **additionalProperties**|string|false|none|none|

<h2 id="tocS_Policy">Policy</h2>

<a id="schemapolicy"></a>
<a id="schema_Policy"></a>
<a id="tocSpolicy"></a>
<a id="tocspolicy"></a>

```json
{
  "name": "cors",
  "version": "v1",
  "executionCondition": "request.metadata[authenticated] != true",
  "params": {}
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|name|string|true|none|Name of the policy|
|version|string|true|none|Version of the policy. Only major-only version is allowed (e.g., v0, v1). Full semantic version (e.g., v1.0.0) is not accepted and will be rejected. The Gateway Controller resolves the major version to the single matching full version installed in the gateway image.|
|executionCondition|string|false|none|Expression controlling conditional execution of the policy|
|params|object|false|none|Arbitrary parameters for the policy (free-form key/value structure)|

<h2 id="tocS_ResourceStatus">ResourceStatus</h2>

<a id="schemaresourcestatus"></a>
<a id="schema_ResourceStatus"></a>
<a id="tocSresourcestatus"></a>
<a id="tocsresourcestatus"></a>

```json
{
  "id": "reading-list-api-v1.0",
  "state": "deployed",
  "createdAt": "2026-04-24T07:21:13Z",
  "updatedAt": "2026-04-24T07:21:13Z",
  "deployedAt": "2026-04-24T07:21:13Z"
}

```

Server-managed lifecycle information for a resource

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|id|string|false|none|Unique identifier assigned by the server (equal to metadata.name)|
|state|string|false|none|Desired deployment state reported by the server|
|createdAt|string(date-time)|false|none|Timestamp when the resource was first created (UTC)|
|updatedAt|string(date-time)|false|none|Timestamp when the resource was last updated (UTC)|
|deployedAt|string(date-time)|false|none|Timestamp when the resource was last deployed (omitted when undeployed)|

#### Enumerated Values

|Property|Value|
|---|---|
|state|deployed|
|state|undeployed|

<h2 id="tocS_ValidationError">ValidationError</h2>

<a id="schemavalidationerror"></a>
<a id="schema_ValidationError"></a>
<a id="tocSvalidationerror"></a>
<a id="tocsvalidationerror"></a>

```json
{
  "field": "spec.context",
  "message": "Context must start with / and cannot end with /"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|field|string|false|none|Field that failed validation|
|message|string|false|none|Human-readable error message|

<h2 id="tocS_WebBrokerApi">WebBrokerApi</h2>

<a id="schemawebbrokerapi"></a>
<a id="schema_WebBrokerApi"></a>
<a id="tocSwebbrokerapi"></a>
<a id="tocswebbrokerapi"></a>

```json
{
  "apiVersion": "gateway.api-platform.wso2.com/v1",
  "kind": "WebBrokerApi",
  "metadata": {
    "name": "stock-trading-v1.0"
  },
  "spec": {
    "displayName": "Stock Trading WebBroker API",
    "version": "v1.0",
    "context": "/stock-trading/$version",
    "receiver": {
      "name": "websocket-receiver",
      "type": "websocket"
    },
    "broker": {
      "name": "kafka-driver",
      "type": "kafka",
      "properties": {
        "brokers": [
          "kafka-broker-1:9092",
          "kafka-broker-2:9092"
        ]
      }
    },
    "allChannels": {
      "on_connection_init": {
        "policies": []
      },
      "on_produce": {
        "policies": []
      },
      "on_consume": {
        "policies": []
      }
    },
    "channels": {
      "prices": {
        "produceTo": {
          "topic": "stock.prices"
        },
        "consumeFrom": {
          "topic": "stock.prices"
        },
        "on_connection_init": {
          "policies": []
        },
        "on_produce": {
          "policies": []
        },
        "on_consume": {
          "policies": []
        }
      }
    }
  },
  "status": {
    "id": "stock-trading-v1.0",
    "state": "deployed",
    "createdAt": "2026-04-24T07:21:13Z",
    "updatedAt": "2026-04-24T07:21:13Z",
    "deployedAt": "2026-04-24T07:21:13Z"
  }
}

```

### Properties

allOf

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|[WebBrokerApiRequest](#schemawebbrokerapirequest)|false|none|none|

and

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|object|false|none|none|
|» status|[ResourceStatus](#schemaresourcestatus)|false|read-only|Server-managed lifecycle fields. Populated on responses.|

<h2 id="tocS_WebBrokerApiAllChannelPolicies">WebBrokerApiAllChannelPolicies</h2>

<a id="schemawebbrokerapiallchannelpolicies"></a>
<a id="schema_WebBrokerApiAllChannelPolicies"></a>
<a id="tocSwebbrokerapiallchannelpolicies"></a>
<a id="tocswebbrokerapiallchannelpolicies"></a>

```json
{
  "on_connection_init": {
    "policies": [
      {
        "name": "cors",
        "version": "v1",
        "executionCondition": "request.metadata[authenticated] != true",
        "params": {}
      }
    ]
  },
  "on_produce": {
    "policies": [
      {
        "name": "cors",
        "version": "v1",
        "executionCondition": "request.metadata[authenticated] != true",
        "params": {}
      }
    ]
  },
  "on_consume": {
    "policies": [
      {
        "name": "cors",
        "version": "v1",
        "executionCondition": "request.metadata[authenticated] != true",
        "params": {}
      }
    ]
  }
}

```

Protocol mediation policies applied to all channels

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|on_connection_init|[WebBrokerApiPolicyGroup](#schemawebbrokerapipolicygroup)|false|none|Group of policies|
|on_produce|[WebBrokerApiPolicyGroup](#schemawebbrokerapipolicygroup)|false|none|Group of policies|
|on_consume|[WebBrokerApiPolicyGroup](#schemawebbrokerapipolicygroup)|false|none|Group of policies|

<h2 id="tocS_WebBrokerApiBroker">WebBrokerApiBroker</h2>

<a id="schemawebbrokerapibroker"></a>
<a id="schema_WebBrokerApiBroker"></a>
<a id="tocSwebbrokerapibroker"></a>
<a id="tocswebbrokerapibroker"></a>

```json
{
  "name": "kafka-driver",
  "type": "kafka",
  "properties": {
    "brokers": [
      "kafka-broker-1:9092",
      "kafka-broker-2:9092"
    ]
  }
}

```

Message broker driver configuration

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|name|string|true|none|Broker driver name|
|type|string|true|none|Broker driver type|
|properties|object|true|none|Broker driver properties (e.g., bootstrap servers)|

<h2 id="tocS_WebBrokerApiChannel">WebBrokerApiChannel</h2>

<a id="schemawebbrokerapichannel"></a>
<a id="schema_WebBrokerApiChannel"></a>
<a id="tocSwebbrokerapichannel"></a>
<a id="tocswebbrokerapichannel"></a>

```json
{
  "produceTo": {
    "topic": "stock.prices"
  },
  "consumeFrom": {
    "topic": "stock.prices"
  },
  "on_connection_init": {
    "policies": [
      {
        "name": "cors",
        "version": "v1",
        "executionCondition": "request.metadata[authenticated] != true",
        "params": {}
      }
    ]
  },
  "on_produce": {
    "policies": [
      {
        "name": "cors",
        "version": "v1",
        "executionCondition": "request.metadata[authenticated] != true",
        "params": {}
      }
    ]
  },
  "on_consume": {
    "policies": [
      {
        "name": "cors",
        "version": "v1",
        "executionCondition": "request.metadata[authenticated] != true",
        "params": {}
      }
    ]
  }
}

```

WebSocket channel configuration with Kafka topic mapping

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|produceTo|[WebBrokerApiProduceConfig](#schemawebbrokerapiproduceconfig)|false|none|Configuration for producing messages from WebSocket to Kafka|
|consumeFrom|[WebBrokerApiConsumeConfig](#schemawebbrokerapiconsumeconfig)|false|none|Configuration for consuming messages from Kafka to WebSocket|
|on_connection_init|[WebBrokerApiPolicyGroup](#schemawebbrokerapipolicygroup)|false|none|Group of policies|
|on_produce|[WebBrokerApiPolicyGroup](#schemawebbrokerapipolicygroup)|false|none|Group of policies|
|on_consume|[WebBrokerApiPolicyGroup](#schemawebbrokerapipolicygroup)|false|none|Group of policies|

<h2 id="tocS_WebBrokerApiConsumeConfig">WebBrokerApiConsumeConfig</h2>

<a id="schemawebbrokerapiconsumeconfig"></a>
<a id="schema_WebBrokerApiConsumeConfig"></a>
<a id="tocSwebbrokerapiconsumeconfig"></a>
<a id="tocswebbrokerapiconsumeconfig"></a>

```json
{
  "topic": "stock.prices"
}

```

Configuration for consuming messages from Kafka to WebSocket

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|topic|string|true|none|Kafka topic to consume messages from|

<h2 id="tocS_WebBrokerApiData">WebBrokerApiData</h2>

<a id="schemawebbrokerapidata"></a>
<a id="schema_WebBrokerApiData"></a>
<a id="tocSwebbrokerapidata"></a>
<a id="tocswebbrokerapidata"></a>

```json
{
  "displayName": "Stock Trading WebBroker API",
  "version": "v1.0",
  "context": "/stock-trading",
  "receiver": {
    "name": "websocket-receiver",
    "type": "websocket",
    "properties": {}
  },
  "broker": {
    "name": "kafka-driver",
    "type": "kafka",
    "properties": {
      "brokers": [
        "kafka-broker-1:9092",
        "kafka-broker-2:9092"
      ]
    }
  },
  "allChannels": {
    "on_connection_init": {
      "policies": [
        {
          "name": "cors",
          "version": "v1",
          "executionCondition": "request.metadata[authenticated] != true",
          "params": {}
        }
      ]
    },
    "on_produce": {
      "policies": [
        {
          "name": "cors",
          "version": "v1",
          "executionCondition": "request.metadata[authenticated] != true",
          "params": {}
        }
      ]
    },
    "on_consume": {
      "policies": [
        {
          "name": "cors",
          "version": "v1",
          "executionCondition": "request.metadata[authenticated] != true",
          "params": {}
        }
      ]
    }
  },
  "channels": {
    "property1": {
      "produceTo": {
        "topic": "stock.prices"
      },
      "consumeFrom": {
        "topic": "stock.prices"
      },
      "on_connection_init": {
        "policies": [
          {
            "name": "cors",
            "version": "v1",
            "executionCondition": "request.metadata[authenticated] != true",
            "params": {}
          }
        ]
      },
      "on_produce": {
        "policies": [
          {
            "name": "cors",
            "version": "v1",
            "executionCondition": "request.metadata[authenticated] != true",
            "params": {}
          }
        ]
      },
      "on_consume": {
        "policies": [
          {
            "name": "cors",
            "version": "v1",
            "executionCondition": "request.metadata[authenticated] != true",
            "params": {}
          }
        ]
      }
    },
    "property2": {
      "produceTo": {
        "topic": "stock.prices"
      },
      "consumeFrom": {
        "topic": "stock.prices"
      },
      "on_connection_init": {
        "policies": [
          {
            "name": "cors",
            "version": "v1",
            "executionCondition": "request.metadata[authenticated] != true",
            "params": {}
          }
        ]
      },
      "on_produce": {
        "policies": [
          {
            "name": "cors",
            "version": "v1",
            "executionCondition": "request.metadata[authenticated] != true",
            "params": {}
          }
        ]
      },
      "on_consume": {
        "policies": [
          {
            "name": "cors",
            "version": "v1",
            "executionCondition": "request.metadata[authenticated] != true",
            "params": {}
          }
        ]
      }
    }
  },
  "vhosts": {
    "main": "api.example.com",
    "sandbox": "sandbox-api.example.com"
  },
  "deploymentState": "deployed"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|displayName|string|true|none|Human-readable API name (must be URL-friendly - only letters, numbers, spaces, hyphens, underscores, and dots allowed)|
|version|string|true|none|Semantic version of the API|
|context|string|true|none|Base path for all API routes (must start with /, no trailing slash)|
|receiver|[WebBrokerApiReceiver](#schemawebbrokerapireceiver)|true|none|WebSocket receiver configuration|
|broker|[WebBrokerApiBroker](#schemawebbrokerapibroker)|true|none|Message broker driver configuration|
|allChannels|[WebBrokerApiAllChannelPolicies](#schemawebbrokerapiallchannelpolicies)|false|none|Protocol mediation policies applied to all channels|
|channels|object|true|none|Map of WebSocket channels for bidirectional streaming with Kafka (key is channel name)|
|» **additionalProperties**|[WebBrokerApiChannel](#schemawebbrokerapichannel)|false|none|WebSocket channel configuration with Kafka topic mapping|
|vhosts|object|false|none|Custom virtual hosts/domains for the API|
|» main|string|true|none|Custom virtual host/domain for production traffic|
|» sandbox|string|false|none|Custom virtual host/domain for sandbox traffic|
|deploymentState|string|false|none|Desired deployment state - 'deployed' (default) or 'undeployed'. When set to 'undeployed', the API is removed from router traffic but configuration and policies are preserved for potential redeployment.|

#### Enumerated Values

|Property|Value|
|---|---|
|deploymentState|deployed|
|deploymentState|undeployed|

<h2 id="tocS_WebBrokerApiPolicyGroup">WebBrokerApiPolicyGroup</h2>

<a id="schemawebbrokerapipolicygroup"></a>
<a id="schema_WebBrokerApiPolicyGroup"></a>
<a id="tocSwebbrokerapipolicygroup"></a>
<a id="tocswebbrokerapipolicygroup"></a>

```json
{
  "policies": [
    {
      "name": "cors",
      "version": "v1",
      "executionCondition": "request.metadata[authenticated] != true",
      "params": {}
    }
  ]
}

```

Group of policies

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|policies|[[Policy](#schemapolicy)]|false|none|List of policies to apply|

<h2 id="tocS_WebBrokerApiProduceConfig">WebBrokerApiProduceConfig</h2>

<a id="schemawebbrokerapiproduceconfig"></a>
<a id="schema_WebBrokerApiProduceConfig"></a>
<a id="tocSwebbrokerapiproduceconfig"></a>
<a id="tocswebbrokerapiproduceconfig"></a>

```json
{
  "topic": "stock.prices"
}

```

Configuration for producing messages from WebSocket to Kafka

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|topic|string|true|none|Kafka topic to produce messages to|

<h2 id="tocS_WebBrokerApiReceiver">WebBrokerApiReceiver</h2>

<a id="schemawebbrokerapireceiver"></a>
<a id="schema_WebBrokerApiReceiver"></a>
<a id="tocSwebbrokerapireceiver"></a>
<a id="tocswebbrokerapireceiver"></a>

```json
{
  "name": "websocket-receiver",
  "type": "websocket",
  "properties": {}
}

```

WebSocket receiver configuration

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|name|string|true|none|Receiver name|
|type|string|true|none|Receiver type|
|properties|object|false|none|Additional receiver properties|

<h2 id="tocS_WebBrokerApiRequest">WebBrokerApiRequest</h2>

<a id="schemawebbrokerapirequest"></a>
<a id="schema_WebBrokerApiRequest"></a>
<a id="tocSwebbrokerapirequest"></a>
<a id="tocswebbrokerapirequest"></a>

```json
{
  "apiVersion": "gateway.api-platform.wso2.com/v1",
  "kind": "WebBrokerApi",
  "metadata": {
    "name": "stock-trading-v1.0"
  },
  "spec": {
    "displayName": "Stock Trading WebBroker API",
    "version": "v1.0",
    "context": "/stock-trading/$version",
    "receiver": {
      "name": "websocket-receiver",
      "type": "websocket"
    },
    "broker": {
      "name": "kafka-driver",
      "type": "kafka",
      "properties": {
        "brokers": [
          "kafka-broker-1:9092",
          "kafka-broker-2:9092"
        ]
      }
    },
    "allChannels": {
      "on_connection_init": {
        "policies": []
      },
      "on_produce": {
        "policies": []
      },
      "on_consume": {
        "policies": []
      }
    },
    "channels": {
      "prices": {
        "produceTo": {
          "topic": "stock.prices"
        },
        "consumeFrom": {
          "topic": "stock.prices"
        },
        "on_connection_init": {
          "policies": []
        },
        "on_produce": {
          "policies": []
        },
        "on_consume": {
          "policies": []
        }
      }
    }
  }
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|apiVersion|string|true|none|API specification version|
|kind|string|true|none|API type|
|metadata|[Metadata](#schemametadata)|true|none|none|
|spec|[WebBrokerApiData](#schemawebbrokerapidata)|true|none|none|

#### Enumerated Values

|Property|Value|
|---|---|
|apiVersion|gateway.api-platform.wso2.com/v1|
|kind|WebBrokerApi|

<h2 id="tocS_WebSubAPI">WebSubAPI</h2>

<a id="schemawebsubapi"></a>
<a id="schema_WebSubAPI"></a>
<a id="tocSwebsubapi"></a>
<a id="tocswebsubapi"></a>

```json
{
  "apiVersion": "gateway.api-platform.wso2.com/v1",
  "kind": "WebSubApi",
  "metadata": {
    "name": "github-events-v1.0"
  },
  "spec": {
    "displayName": "GitHub Events",
    "version": "v1.0",
    "context": "/github-events/$version",
    "channels": [
      {
        "name": "issues",
        "method": "SUB"
      },
      {
        "name": "pull_requests",
        "method": "SUB"
      }
    ]
  },
  "status": {
    "id": "github-events-v1.0",
    "state": "deployed",
    "createdAt": "2026-04-24T07:21:13Z",
    "updatedAt": "2026-04-24T07:21:13Z",
    "deployedAt": "2026-04-24T07:21:13Z"
  }
}

```

### Properties

allOf

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|[WebSubAPIRequest](#schemawebsubapirequest)|false|none|none|

and

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|object|false|none|none|
|» status|[ResourceStatus](#schemaresourcestatus)|false|read-only|Server-managed lifecycle fields. Populated on responses.|

<h2 id="tocS_WebSubAPIRequest">WebSubAPIRequest</h2>

<a id="schemawebsubapirequest"></a>
<a id="schema_WebSubAPIRequest"></a>
<a id="tocSwebsubapirequest"></a>
<a id="tocswebsubapirequest"></a>

```json
{
  "apiVersion": "gateway.api-platform.wso2.com/v1",
  "kind": "WebSubApi",
  "metadata": {
    "name": "github-events-v1.0"
  },
  "spec": {
    "displayName": "GitHub Events",
    "version": "v1.0",
    "context": "/github-events/$version",
    "channels": [
      {
        "name": "issues",
        "method": "SUB"
      },
      {
        "name": "pull_requests",
        "method": "SUB"
      }
    ]
  }
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|apiVersion|string|true|none|API specification version|
|kind|string|true|none|API type|
|metadata|[Metadata](#schemametadata)|true|none|none|
|spec|[WebhookAPIData](#schemawebhookapidata)|true|none|none|

#### Enumerated Values

|Property|Value|
|---|---|
|apiVersion|gateway.api-platform.wso2.com/v1|
|kind|WebSubApi|

<h2 id="tocS_WebSubAllChannelPolicies">WebSubAllChannelPolicies</h2>

<a id="schemawebsuballchannelpolicies"></a>
<a id="schema_WebSubAllChannelPolicies"></a>
<a id="tocSwebsuballchannelpolicies"></a>
<a id="tocswebsuballchannelpolicies"></a>

```json
{
  "on_subscription": {
    "policies": [
      {
        "name": "cors",
        "version": "v1",
        "executionCondition": "request.metadata[authenticated] != true",
        "params": {}
      }
    ]
  },
  "on_unsubscription": {
    "policies": [
      {
        "name": "cors",
        "version": "v1",
        "executionCondition": "request.metadata[authenticated] != true",
        "params": {}
      }
    ]
  },
  "on_message_received": {
    "policies": [
      {
        "name": "cors",
        "version": "v1",
        "executionCondition": "request.metadata[authenticated] != true",
        "params": {}
      }
    ]
  },
  "on_message_delivery": {
    "policies": [
      {
        "name": "cors",
        "version": "v1",
        "executionCondition": "request.metadata[authenticated] != true",
        "params": {}
      }
    ]
  }
}

```

Policies applied to all channels, organized by event type.

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|on_subscription|[WebSubEventPolicies](#schemawebsubeventpolicies)|false|none|Policies for a single event type.|
|on_unsubscription|[WebSubEventPolicies](#schemawebsubeventpolicies)|false|none|Policies for a single event type.|
|on_message_received|[WebSubEventPolicies](#schemawebsubeventpolicies)|false|none|Policies for a single event type.|
|on_message_delivery|[WebSubEventPolicies](#schemawebsubeventpolicies)|false|none|Policies for a single event type.|

<h2 id="tocS_WebSubChannel">WebSubChannel</h2>

<a id="schemawebsubchannel"></a>
<a id="schema_WebSubChannel"></a>
<a id="tocSwebsubchannel"></a>
<a id="tocswebsubchannel"></a>

```json
{
  "on_subscription": {
    "policies": [
      {
        "name": "cors",
        "version": "v1",
        "executionCondition": "request.metadata[authenticated] != true",
        "params": {}
      }
    ]
  },
  "on_unsubscription": {
    "policies": [
      {
        "name": "cors",
        "version": "v1",
        "executionCondition": "request.metadata[authenticated] != true",
        "params": {}
      }
    ]
  },
  "on_message_received": {
    "policies": [
      {
        "name": "cors",
        "version": "v1",
        "executionCondition": "request.metadata[authenticated] != true",
        "params": {}
      }
    ]
  },
  "on_message_delivery": {
    "policies": [
      {
        "name": "cors",
        "version": "v1",
        "executionCondition": "request.metadata[authenticated] != true",
        "params": {}
      }
    ]
  }
}

```

A single channel definition with optional per-channel policy overrides.

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|on_subscription|[WebSubEventPolicies](#schemawebsubeventpolicies)|false|none|Policies for a single event type.|
|on_unsubscription|[WebSubEventPolicies](#schemawebsubeventpolicies)|false|none|Policies for a single event type.|
|on_message_received|[WebSubEventPolicies](#schemawebsubeventpolicies)|false|none|Policies for a single event type.|
|on_message_delivery|[WebSubEventPolicies](#schemawebsubeventpolicies)|false|none|Policies for a single event type.|

<h2 id="tocS_WebSubEventPolicies">WebSubEventPolicies</h2>

<a id="schemawebsubeventpolicies"></a>
<a id="schema_WebSubEventPolicies"></a>
<a id="tocSwebsubeventpolicies"></a>
<a id="tocswebsubeventpolicies"></a>

```json
{
  "policies": [
    {
      "name": "cors",
      "version": "v1",
      "executionCondition": "request.metadata[authenticated] != true",
      "params": {}
    }
  ]
}

```

Policies for a single event type.

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|policies|[[Policy](#schemapolicy)]|false|none|List of policies applied for this event type.|

<h2 id="tocS_WebhookAPIData">WebhookAPIData</h2>

<a id="schemawebhookapidata"></a>
<a id="schema_WebhookAPIData"></a>
<a id="tocSwebhookapidata"></a>
<a id="tocswebhookapidata"></a>

```json
{
  "displayName": "reading-list-api",
  "version": "v1.0",
  "context": "/weather",
  "vhosts": {
    "main": "api.example.com",
    "sandbox": "sandbox-api.example.com"
  },
  "allChannels": {
    "on_subscription": {
      "policies": [
        {
          "name": "cors",
          "version": "v1",
          "executionCondition": "request.metadata[authenticated] != true",
          "params": {}
        }
      ]
    },
    "on_unsubscription": {
      "policies": [
        {
          "name": "cors",
          "version": "v1",
          "executionCondition": "request.metadata[authenticated] != true",
          "params": {}
        }
      ]
    },
    "on_message_received": {
      "policies": [
        {
          "name": "cors",
          "version": "v1",
          "executionCondition": "request.metadata[authenticated] != true",
          "params": {}
        }
      ]
    },
    "on_message_delivery": {
      "policies": [
        {
          "name": "cors",
          "version": "v1",
          "executionCondition": "request.metadata[authenticated] != true",
          "params": {}
        }
      ]
    }
  },
  "channels": {
    "property1": {
      "on_subscription": {
        "policies": [
          {
            "name": "cors",
            "version": "v1",
            "executionCondition": "request.metadata[authenticated] != true",
            "params": {}
          }
        ]
      },
      "on_unsubscription": {
        "policies": [
          {
            "name": "cors",
            "version": "v1",
            "executionCondition": "request.metadata[authenticated] != true",
            "params": {}
          }
        ]
      },
      "on_message_received": {
        "policies": [
          {
            "name": "cors",
            "version": "v1",
            "executionCondition": "request.metadata[authenticated] != true",
            "params": {}
          }
        ]
      },
      "on_message_delivery": {
        "policies": [
          {
            "name": "cors",
            "version": "v1",
            "executionCondition": "request.metadata[authenticated] != true",
            "params": {}
          }
        ]
      }
    },
    "property2": {
      "on_subscription": {
        "policies": [
          {
            "name": "cors",
            "version": "v1",
            "executionCondition": "request.metadata[authenticated] != true",
            "params": {}
          }
        ]
      },
      "on_unsubscription": {
        "policies": [
          {
            "name": "cors",
            "version": "v1",
            "executionCondition": "request.metadata[authenticated] != true",
            "params": {}
          }
        ]
      },
      "on_message_received": {
        "policies": [
          {
            "name": "cors",
            "version": "v1",
            "executionCondition": "request.metadata[authenticated] != true",
            "params": {}
          }
        ]
      },
      "on_message_delivery": {
        "policies": [
          {
            "name": "cors",
            "version": "v1",
            "executionCondition": "request.metadata[authenticated] != true",
            "params": {}
          }
        ]
      }
    }
  },
  "deploymentState": "deployed"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|displayName|string|true|none|Human-readable API name (must be URL-friendly - only letters, numbers, spaces, hyphens, underscores, and dots allowed)|
|version|string|true|none|Semantic version of the API|
|context|string|true|none|Base path for all API routes (must start with /, no trailing slash)|
|vhosts|object|false|none|Custom virtual hosts/domains for the API|
|» main|string|true|none|Custom virtual host/domain for production traffic|
|» sandbox|string|false|none|Custom virtual host/domain for sandbox traffic|
|allChannels|[WebSubAllChannelPolicies](#schemawebsuballchannelpolicies)|false|none|Policies applied to all channels, organized by event type.|
|channels|object|false|none|Per-channel configuration keyed by channel name. Each key is a channel name and defines policies applied only to that channel.|
|» **additionalProperties**|[WebSubChannel](#schemawebsubchannel)|false|none|A single channel definition with optional per-channel policy overrides.|
|deploymentState|string|false|none|Desired deployment state - 'deployed' (default) or 'undeployed'. When set to 'undeployed', the API is removed from router traffic but configuration, API keys, and policies are preserved for potential redeployment.|

#### Enumerated Values

|Property|Value|
|---|---|
|deploymentState|deployed|
|deploymentState|undeployed|

<h2 id="tocS_WebhookSecretCreationRequest">WebhookSecretCreationRequest</h2>

<a id="schemawebhooksecretcreationrequest"></a>
<a id="schema_WebhookSecretCreationRequest"></a>
<a id="tocSwebhooksecretcreationrequest"></a>
<a id="tocswebhooksecretcreationrequest"></a>

```json
{
  "displayName": "GitHub Webhook"
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|displayName|string|true|none|Human-readable label for this secret (used to derive the immutable name slug).|

<h2 id="tocS_WebhookSecretCreationResponse">WebhookSecretCreationResponse</h2>

<a id="schemawebhooksecretcreationresponse"></a>
<a id="schema_WebhookSecretCreationResponse"></a>
<a id="tocSwebhooksecretcreationresponse"></a>
<a id="tocswebhooksecretcreationresponse"></a>

```json
{
  "status": "success",
  "message": "Webhook secret generated successfully",
  "secret": "whsec_1a2b3c4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0b1c2d3e4f5a6b7c8d9e0f1a2b",
  "webhookSecret": {
    "name": "github-webhook",
    "displayName": "GitHub Webhook",
    "status": "active",
    "createdAt": "2026-06-01T10:00:00Z",
    "updatedAt": "2026-06-01T10:00:00Z"
  }
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|status|string|true|none|none|
|message|string|true|none|none|
|secret|string|true|none|The generated plaintext secret value (whsec_ prefix + 64 hex chars).<br>Returned exactly once — store it immediately as it will not be retrievable again.|
|webhookSecret|[WebhookSecretInfo](#schemawebhooksecretinfo)|false|none|Metadata for an HMAC secret. The plaintext value is never included.|

<h2 id="tocS_WebhookSecretInfo">WebhookSecretInfo</h2>

<a id="schemawebhooksecretinfo"></a>
<a id="schema_WebhookSecretInfo"></a>
<a id="tocSwebhooksecretinfo"></a>
<a id="tocswebhooksecretinfo"></a>

```json
{
  "name": "github-webhook",
  "displayName": "GitHub Webhook",
  "status": "active",
  "createdAt": "2026-06-01T10:00:00Z",
  "updatedAt": "2026-06-01T10:00:00Z"
}

```

Metadata for an HMAC secret. The plaintext value is never included.

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|name|string|false|none|URL-safe slug (immutable, used as path parameter for regenerate/delete).|
|displayName|string|false|none|Human-readable label.|
|status|string|false|none|none|
|createdAt|string(date-time)|false|none|none|
|updatedAt|string(date-time)|false|none|none|

#### Enumerated Values

|Property|Value|
|---|---|
|status|active|
|status|revoked|

<h2 id="tocS_WebhookSecretListResponse">WebhookSecretListResponse</h2>

<a id="schemawebhooksecretlistresponse"></a>
<a id="schema_WebhookSecretListResponse"></a>
<a id="tocSwebhooksecretlistresponse"></a>
<a id="tocswebhooksecretlistresponse"></a>

```json
{
  "status": "success",
  "totalCount": 2,
  "secrets": [
    {
      "name": "github-webhook",
      "displayName": "GitHub Webhook",
      "status": "active",
      "createdAt": "2026-06-01T10:00:00Z",
      "updatedAt": "2026-06-01T10:00:00Z"
    }
  ]
}

```

### Properties

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|status|string|false|none|none|
|totalCount|integer|false|none|Total number of active secrets for this API|
|secrets|[[WebhookSecretInfo](#schemawebhooksecretinfo)]|false|none|[Metadata for an HMAC secret. The plaintext value is never included.]|
