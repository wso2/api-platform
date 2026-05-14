<h1 id="gateway-controller-management-api-webbroker-api-management">WebBroker API Management</h1>

## Create a new WebBrokerAPI

<a id="opIdcreateWebBrokerApi"></a>

`POST /webbroker-apis`

> Code samples

```shell

curl -X POST http://localhost:9090/api/management/v0.9/webbroker-apis \
  -u {username}:{password} \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -d @payload.json

```

Add a new WebBrokerAPI to the Gateway. WebBrokerAPI provides bidirectional streaming between WebSocket clients and Kafka brokers with per-connection isolation.

> Payload

```json
{
  "apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
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

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

Required roles: `admin`, `developer`

</aside>

<h3 id="create-a-new-webbrokerapi-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|[WebBrokerApiRequest](schemas.md#schemawebbrokerapirequest)|true|none|

> Example responses

> 201 Response

```json
{
  "apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
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

<h3 id="create-a-new-webbrokerapi-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|201|[Created](https://tools.ietf.org/html/rfc7231#section-6.3.2)|WebBrokerAPI created successfully|[WebBrokerApi](schemas.md#schemawebbrokerapi)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Invalid configuration (validation failed)|[ErrorResponse](schemas.md#schemaerrorresponse)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Conflict - WebBroker API with same name and version already exists|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

## List all WebBrokerAPIs

<a id="opIdlistWebBrokerApis"></a>

`GET /webbroker-apis`

> Code samples

```shell

curl -X GET http://localhost:9090/api/management/v0.9/webbroker-apis \
  -u {username}:{password} \
  -H 'Accept: application/json'

```

List WebBrokerAPIs registered in the Gateway, optionally filtered by name, version, or status.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

Required roles: `admin`, `developer`

</aside>

<h3 id="list-all-webbrokerapis-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|displayName|query|string|false|Filter by WebBroker API display name|
|version|query|string|false|Filter by WebBroker API version|
|status|query|string|false|Filter by deployment status|

#### Enumerated Values

|Parameter|Value|
|---|---|
|status|deployed|
|status|undeployed|

> Example responses

> 200 Response

```json
{
  "status": "success",
  "count": 3,
  "apis": [
    {
      "apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
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
  ]
}
```

<h3 id="list-all-webbrokerapis-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|List of WebBrokerAPIs|Inline|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="list-all-webbrokerapis-responseschema">Response Schema</h3>

Status Code **200**

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|» status|string|false|none|none|
|» count|integer|false|none|none|
|» apis|[allOf]|false|none|none|

*allOf*

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|»» *anonymous*|[WebBrokerApiRequest](schemas.md#schemawebbrokerapirequest)|false|none|none|
|»»» apiVersion|string|true|none|API specification version|
|»»» kind|string|true|none|API type|
|»»» metadata|[Metadata](schemas.md#schemametadata)|true|none|none|
|»»»» name|string|true|none|Unique handle for the resource|
|»»»» labels|object|false|none|Labels are key-value pairs for organizing and selecting APIs. Keys must not contain spaces.|
|»»»»» **additionalProperties**|string|false|none|none|
|»»»» annotations|object|false|none|Annotations are arbitrary non-identifying metadata. Use domain-prefixed keys.|
|»»»»» **additionalProperties**|string|false|none|none|
|»»» spec|[WebBrokerApiData](schemas.md#schemawebbrokerapidata)|true|none|none|
|»»»» displayName|string|true|none|Human-readable API name (must be URL-friendly - only letters, numbers, spaces, hyphens, underscores, and dots allowed)|
|»»»» version|string|true|none|Semantic version of the API|
|»»»» context|string|true|none|Base path for all API routes (must start with /, no trailing slash)|
|»»»» receiver|[WebBrokerApiReceiver](schemas.md#schemawebbrokerapireceiver)|true|none|WebSocket receiver configuration|
|»»»»» name|string|true|none|Receiver name|
|»»»»» type|string|true|none|Receiver type|
|»»»»» properties|object|false|none|Additional receiver properties|
|»»»» broker|[WebBrokerApiBroker](schemas.md#schemawebbrokerapibroker)|true|none|Message broker driver configuration|
|»»»»» name|string|true|none|Broker driver name|
|»»»»» type|string|true|none|Broker driver type|
|»»»»» properties|object|true|none|Broker driver properties (e.g., bootstrap servers)|
|»»»» allChannels|[WebBrokerApiAllChannelPolicies](schemas.md#schemawebbrokerapiallchannelpolicies)|false|none|Protocol mediation policies applied to all channels|
|»»»»» on_connection_init|[WebBrokerApiPolicyGroup](schemas.md#schemawebbrokerapipolicygroup)|false|none|Group of policies|
|»»»»»» policies|[[Policy](schemas.md#schemapolicy)]|false|none|List of policies to apply|
|»»»»»»» name|string|true|none|Name of the policy|
|»»»»»»» version|string|true|none|Version of the policy. Only major-only version is allowed (e.g., v0, v1). Full semantic version (e.g., v1.0.0) is not accepted and will be rejected. The Gateway Controller resolves the major version to the single matching full version installed in the gateway image.|
|»»»»»»» executionCondition|string|false|none|Expression controlling conditional execution of the policy|
|»»»»»»» params|object|false|none|Arbitrary parameters for the policy (free-form key/value structure)|
|»»»»» on_produce|[WebBrokerApiPolicyGroup](schemas.md#schemawebbrokerapipolicygroup)|false|none|Group of policies|
|»»»»» on_consume|[WebBrokerApiPolicyGroup](schemas.md#schemawebbrokerapipolicygroup)|false|none|Group of policies|
|»»»» channels|object|true|none|Map of WebSocket channels for bidirectional streaming with Kafka (key is channel name)|
|»»»»» **additionalProperties**|[WebBrokerApiChannel](schemas.md#schemawebbrokerapichannel)|false|none|WebSocket channel configuration with Kafka topic mapping|
|»»»»»» produceTo|[WebBrokerApiProduceConfig](schemas.md#schemawebbrokerapiproduceconfig)|false|none|Configuration for producing messages from WebSocket to Kafka|
|»»»»»»» topic|string|true|none|Kafka topic to produce messages to|
|»»»»»» consumeFrom|[WebBrokerApiConsumeConfig](schemas.md#schemawebbrokerapiconsumeconfig)|false|none|Configuration for consuming messages from Kafka to WebSocket|
|»»»»»»» topic|string|true|none|Kafka topic to consume messages from|
|»»»»»» on_connection_init|[WebBrokerApiPolicyGroup](schemas.md#schemawebbrokerapipolicygroup)|false|none|Group of policies|
|»»»»»» on_produce|[WebBrokerApiPolicyGroup](schemas.md#schemawebbrokerapipolicygroup)|false|none|Group of policies|
|»»»»»» on_consume|[WebBrokerApiPolicyGroup](schemas.md#schemawebbrokerapipolicygroup)|false|none|Group of policies|
|»»»» vhosts|object|false|none|Custom virtual hosts/domains for the API|
|»»»»» main|string|true|none|Custom virtual host/domain for production traffic|
|»»»»» sandbox|string|false|none|Custom virtual host/domain for sandbox traffic|
|»»»» deploymentState|string|false|none|Desired deployment state - 'deployed' (default) or 'undeployed'. When set to 'undeployed', the API is removed from router traffic but configuration and policies are preserved for potential redeployment.|

*and*

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|»» *anonymous*|object|false|none|none|
|»»» status|[ResourceStatus](schemas.md#schemaresourcestatus)|false|read-only|Server-managed lifecycle fields. Populated on responses.|
|»»»» id|string|false|none|Unique identifier assigned by the server (equal to metadata.name)|
|»»»» state|string|false|none|Desired deployment state reported by the server|
|»»»» createdAt|string(date-time)|false|none|Timestamp when the resource was first created (UTC)|
|»»»» updatedAt|string(date-time)|false|none|Timestamp when the resource was last updated (UTC)|
|»»»» deployedAt|string(date-time)|false|none|Timestamp when the resource was last deployed (omitted when undeployed)|

#### Enumerated Values

|Property|Value|
|---|---|
|apiVersion|gateway.api-platform.wso2.com/v1alpha1|
|kind|WebBrokerApi|
|deploymentState|deployed|
|deploymentState|undeployed|
|state|deployed|
|state|undeployed|

## Get WebBrokerAPI by id

<a id="opIdgetWebBrokerApiById"></a>

`GET /webbroker-apis/{id}`

> Code samples

```shell

curl -X GET http://localhost:9090/api/management/v0.9/webbroker-apis/{id} \
  -u {username}:{password} \
  -H 'Accept: application/json'

```

Get a WebBrokerAPI by its ID.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

Required roles: `admin`, `developer`

</aside>

<h3 id="get-webbrokerapi-by-id-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|string|true|Unique public identifier for the WebBroker API.|

#### Detailed descriptions

**id**: Unique public identifier for the WebBroker API.

> Example responses

> 200 Response

```json
{
  "apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
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

<h3 id="get-webbrokerapi-by-id-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|WebBrokerAPI details|[WebBrokerApi](schemas.md#schemawebbrokerapi)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|WebBrokerAPI not found|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

## Delete a WebBrokerAPI

<a id="opIddeleteWebBrokerApiById"></a>

`DELETE /webbroker-apis/{id}`

> Code samples

```shell

curl -X DELETE http://localhost:9090/api/management/v0.9/webbroker-apis/{id} \
  -u {username}:{password} \
  -H 'Accept: application/json'

```

Delete a WebBrokerAPI from the Gateway.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

Required roles: `admin`, `developer`

</aside>

<h3 id="delete-a-webbrokerapi-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|string|true|Unique public identifier of the WebBroker API to delete.|

#### Detailed descriptions

**id**: Unique public identifier of the WebBroker API to delete.

> Example responses

> 200 Response

```json
{
  "status": "success",
  "message": "WebBrokerAPI deleted successfully",
  "id": "stock-trading-webbroker-api"
}
```

<h3 id="delete-a-webbrokerapi-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|WebBrokerAPI deleted successfully|Inline|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|WebBrokerAPI not found|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="delete-a-webbrokerapi-responseschema">Response Schema</h3>

Status Code **200**

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|» status|string|false|none|none|
|» message|string|false|none|none|
|» id|string|false|none|none|
