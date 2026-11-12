<h1 id="gateway-controller-management-api-ai-cost-management">AI Cost Management</h1>

Budgets (spend limits on LLM traffic)

## Create a budget

<a id="opIdcreateBudget"></a>

`POST /budgets`

> Code samples

```shell

curl -X POST http://localhost:9090/api/management/v1/budgets \
  -u {username}:{password} \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -d @payload.json

```

Add a budget (a spend limit on LLM traffic) to the Gateway.

> Payload

```json
{
  "apiVersion": "gateway.api-platform.wso2.com/v1",
  "kind": "Budget",
  "metadata": {
    "name": "research-team-monthly"
  },
  "spec": {
    "limit": {
      "amount": "500.00",
      "period": "month",
      "action": "block",
      "blockAt": 95
    },
    "resource": {
      "type": "llmProvider",
      "allocation": "shared",
      "id": "openai"
    },
    "consumers": [
      {
        "type": "attribute",
        "allocation": "shared",
        "expression": "'research' in request.Auth.Claims[\"groups\"]"
      }
    ]
  }
}
```

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

Required roles: `admin`

</aside>

<h3 id="create-a-budget-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|[BudgetRequest](schemas.md#schemabudgetrequest)|true|none|

> Example responses
>
> 201 Response

```json
{
  "apiVersion": "gateway.api-platform.wso2.com/v1",
  "kind": "Budget",
  "metadata": {
    "name": "research-team-monthly"
  },
  "spec": {
    "limit": {
      "amount": "500.00",
      "period": "month",
      "action": "block",
      "blockAt": 95
    },
    "resource": {
      "type": "llmProvider",
      "allocation": "shared",
      "id": "openai"
    },
    "consumers": [
      {
        "type": "attribute",
        "allocation": "shared",
        "expression": "'research' in request.Auth.Claims[\"groups\"]"
      }
    ]
  },
  "status": {
    "id": "reading-list-api-v1.0",
    "state": "deployed",
    "createdAt": "2026-04-24T07:21:13Z",
    "updatedAt": "2026-04-24T07:21:13Z",
    "deployedAt": "2026-04-24T07:21:13Z",
    "warnings": [
      {
        "code": "TLS_VERIFY_HOSTNAME_DISABLED",
        "field": "spec.upstreamDefinitions[0].tls.verifyHostName",
        "message": "hostname verification is disabled for this upstream; the backend certificate's name is not checked against the target host"
      }
    ]
  }
}
```

<h3 id="create-a-budget-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|201|[Created](https://tools.ietf.org/html/rfc7231#section-6.3.2)|Budget created successfully|[Budget](schemas.md#schemabudget)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Invalid configuration|[ErrorResponse](schemas.md#schemaerrorresponse)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Conflict - a budget with the same name already exists|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

## List budgets

<a id="opIdlistBudgets"></a>

`GET /budgets`

> Code samples

```shell

curl -X GET http://localhost:9090/api/management/v1/budgets \
  -u {username}:{password} \
  -H 'Accept: application/json'

```

List the budgets on the Gateway, active and inactive.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

Required roles: `admin`

</aside>

> Example responses
>
> 200 Response

```json
{
  "status": "success",
  "count": 1,
  "budgets": [
    {
      "apiVersion": "gateway.api-platform.wso2.com/v1",
      "kind": "Budget",
      "metadata": {
        "name": "research-team-monthly"
      },
      "spec": {
        "limit": {
          "amount": "500.00",
          "period": "month",
          "action": "block",
          "blockAt": 95
        },
        "resource": {
          "type": "llmProvider",
          "allocation": "shared",
          "id": "openai"
        },
        "consumers": [
          {
            "type": "attribute",
            "allocation": "shared",
            "expression": "'research' in request.Auth.Claims[\"groups\"]"
          }
        ]
      },
      "status": {
        "id": "reading-list-api-v1.0",
        "state": "deployed",
        "createdAt": "2026-04-24T07:21:13Z",
        "updatedAt": "2026-04-24T07:21:13Z",
        "deployedAt": "2026-04-24T07:21:13Z",
        "warnings": [
          {
            "code": "TLS_VERIFY_HOSTNAME_DISABLED",
            "field": "spec.upstreamDefinitions[0].tls.verifyHostName",
            "message": "hostname verification is disabled for this upstream; the backend certificate's name is not checked against the target host"
          }
        ]
      }
    }
  ]
}
```

<h3 id="list-budgets-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|List of budgets|Inline|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="list-budgets-responseschema">Response Schema</h3>

Status Code **200**

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|» status|string|false|none|none|
|» count|integer|false|none|none|
|» budgets|[allOf]|false|none|none|

*allOf*

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|»» *anonymous*|[BudgetRequest](schemas.md#schemabudgetrequest)|false|none|none|
|»»» apiVersion|string|true|none|none|
|»»» kind|string|true|none|none|
|»»» metadata|[BudgetMetadata](schemas.md#schemabudgetmetadata)|true|none|none|
|»»»» name|string|true|none|The budget's handle, unique on the gateway. Lowercase letters, digits, '-' and '.'.|
|»»» spec|[BudgetSpec](schemas.md#schemabudgetspec)|true|none|none|
|»»»» status|string|false|none|An inactive budget is kept but matches no traffic.|
|»»»» limit|[BudgetLimit](schemas.md#schemabudgetlimit)|true|none|none|
|»»»»» amount|string|true|none|A positive amount with up to 2 decimal places, such as "100.00".|
|»»»»» currency|string|false|none|none|
|»»»»» period|string|true|none|UTC calendar day, ISO week starting Monday, or calendar month.|
|»»»»» action|string|true|none|block refuses requests at the block point; allow only counts.|
|»»»»» blockAt|integer|false|none|Percentage of the amount at which a block budget refuses requests.|
|»»»» resource|[BudgetResource](schemas.md#schemabudgetresource)|true|none|none|
|»»»»» type|string|true|none|none|
|»»»»» allocation|string|true|none|shared is one allowance; separate is one allowance per provider (id omitted).|
|»»»»» id|string|false|none|LLM provider handle. Omit for all providers.|
|»»»» consumers|[[BudgetConsumer](schemas.md#schemabudgetconsumer)]|false|none|Every consumer must match (AND). Empty means all traffic.|
|»»»»» type|string|true|none|none|
|»»»»» allocation|string|true|none|shared is one allowance; separate is one allowance per value of the expression.|
|»»»»» expression|string|true|none|CEL over the request. A condition when shared; the value to allocate by when separate.|
|»»»» exemptions|[[BudgetExemption](schemas.md#schemabudgetexemption)]|false|none|Matching traffic is left out of this budget.|
|»»»»» type|string|true|none|none|
|»»»»» expression|string|true|none|CEL condition over the request.|

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
|»»»» warnings|[[Warning](schemas.md#schemawarning)]|false|read-only|Non-fatal deploy-time findings (e.g. a policy's parameters resolved to something other than what was written, such as an omitted accept list inheriting the whole client-CA pool). Present only when non-empty.|
|»»»»» code|string|false|none|none|
|»»»»» field|string|false|none|none|
|»»»»» message|string|false|none|none|

#### Enumerated Values

|Property|Value|
|---|---|
|apiVersion|gateway.api-platform.wso2.com/v1|
|kind|Budget|
|status|active|
|status|inactive|
|currency|USD|
|period|day|
|period|week|
|period|month|
|action|block|
|action|allow|
|type|llmProvider|
|allocation|shared|
|allocation|separate|
|type|attribute|
|allocation|shared|
|allocation|separate|
|type|attribute|
|state|deployed|
|state|undeployed|
|code|MTLS_ACCEPT_INHERITS_POOL|
|code|MTLS_ACCEPT_UNNARROWED|
|code|MTLS_AUTH_NOT_FIRST|
|code|MTLS_ACCEPT_NAMES_RELAY_AUTHORITY|
|code|MTLS_THUMBPRINT_NORMALISED|
|code|HEADER_CERT_BYPASS_ACTIVE|
|code|MTLS_HOSTNAME_NOT_SCOPED|
|code|TLS_VERIFY_HOSTNAME_DISABLED|
|code|TLS_IDENTITY_EXPIRED|

## Get a budget

<a id="opIdgetBudget"></a>

`GET /budgets/{id}`

> Code samples

```shell

curl -X GET http://localhost:9090/api/management/v1/budgets/{id} \
  -u {username}:{password} \
  -H 'Accept: application/json'

```

Get a budget by its name.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

Required roles: `admin`

</aside>

<h3 id="get-a-budget-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|string|true|The budget's name (metadata.name)|

> Example responses
>
> 200 Response

```json
{
  "apiVersion": "gateway.api-platform.wso2.com/v1",
  "kind": "Budget",
  "metadata": {
    "name": "research-team-monthly"
  },
  "spec": {
    "limit": {
      "amount": "500.00",
      "period": "month",
      "action": "block",
      "blockAt": 95
    },
    "resource": {
      "type": "llmProvider",
      "allocation": "shared",
      "id": "openai"
    },
    "consumers": [
      {
        "type": "attribute",
        "allocation": "shared",
        "expression": "'research' in request.Auth.Claims[\"groups\"]"
      }
    ]
  },
  "status": {
    "id": "reading-list-api-v1.0",
    "state": "deployed",
    "createdAt": "2026-04-24T07:21:13Z",
    "updatedAt": "2026-04-24T07:21:13Z",
    "deployedAt": "2026-04-24T07:21:13Z",
    "warnings": [
      {
        "code": "TLS_VERIFY_HOSTNAME_DISABLED",
        "field": "spec.upstreamDefinitions[0].tls.verifyHostName",
        "message": "hostname verification is disabled for this upstream; the backend certificate's name is not checked against the target host"
      }
    ]
  }
}
```

<h3 id="get-a-budget-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Budget details|[Budget](schemas.md#schemabudget)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Budget not found|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

## Update a budget

<a id="opIdupdateBudget"></a>

`PUT /budgets/{id}`

> Code samples

```shell

curl -X PUT http://localhost:9090/api/management/v1/budgets/{id} \
  -u {username}:{password} \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -d @payload.json

```

Update a budget. Only spec.status, spec.limit.amount, spec.limit.action, spec.limit.blockAt and spec.exemptions can change; every other field must match the stored budget. To change the period, resource or consumers, create a new budget. Budgets from the control plane cannot be changed here.

> Payload

```json
{
  "apiVersion": "gateway.api-platform.wso2.com/v1",
  "kind": "Budget",
  "metadata": {
    "name": "research-team-monthly"
  },
  "spec": {
    "limit": {
      "amount": "500.00",
      "period": "month",
      "action": "block",
      "blockAt": 95
    },
    "resource": {
      "type": "llmProvider",
      "allocation": "shared",
      "id": "openai"
    },
    "consumers": [
      {
        "type": "attribute",
        "allocation": "shared",
        "expression": "'research' in request.Auth.Claims[\"groups\"]"
      }
    ]
  }
}
```

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

Required roles: `admin`

</aside>

<h3 id="update-a-budget-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|string|true|The budget's name (metadata.name)|
|body|body|[BudgetRequest](schemas.md#schemabudgetrequest)|true|none|

> Example responses
>
> 200 Response

```json
{
  "apiVersion": "gateway.api-platform.wso2.com/v1",
  "kind": "Budget",
  "metadata": {
    "name": "research-team-monthly"
  },
  "spec": {
    "limit": {
      "amount": "500.00",
      "period": "month",
      "action": "block",
      "blockAt": 95
    },
    "resource": {
      "type": "llmProvider",
      "allocation": "shared",
      "id": "openai"
    },
    "consumers": [
      {
        "type": "attribute",
        "allocation": "shared",
        "expression": "'research' in request.Auth.Claims[\"groups\"]"
      }
    ]
  },
  "status": {
    "id": "reading-list-api-v1.0",
    "state": "deployed",
    "createdAt": "2026-04-24T07:21:13Z",
    "updatedAt": "2026-04-24T07:21:13Z",
    "deployedAt": "2026-04-24T07:21:13Z",
    "warnings": [
      {
        "code": "TLS_VERIFY_HOSTNAME_DISABLED",
        "field": "spec.upstreamDefinitions[0].tls.verifyHostName",
        "message": "hostname verification is disabled for this upstream; the backend certificate's name is not checked against the target host"
      }
    ]
  }
}
```

<h3 id="update-a-budget-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Budget updated successfully|[Budget](schemas.md#schemabudget)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Invalid configuration, or a field that cannot change|[ErrorResponse](schemas.md#schemaerrorresponse)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Budget not found|[ErrorResponse](schemas.md#schemaerrorresponse)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Conflict - the budget comes from the control plane and is read-only here|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

## Delete a budget

<a id="opIddeleteBudget"></a>

`DELETE /budgets/{id}`

> Code samples

```shell

curl -X DELETE http://localhost:9090/api/management/v1/budgets/{id} \
  -u {username}:{password} \
  -H 'Accept: application/json'

```

Delete a budget from the Gateway. Budgets from the control plane cannot be deleted here.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

Required roles: `admin`

</aside>

<h3 id="delete-a-budget-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|string|true|The budget's name (metadata.name)|

> Example responses
>
> 200 Response

```json
{
  "status": "success",
  "message": "Budget deleted successfully",
  "id": "research-team-monthly"
}
```

<h3 id="delete-a-budget-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Budget deleted successfully|Inline|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Budget not found|[ErrorResponse](schemas.md#schemaerrorresponse)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Conflict - the budget comes from the control plane and is read-only here|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="delete-a-budget-responseschema">Response Schema</h3>

Status Code **200**

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|» status|string|false|none|none|
|» message|string|false|none|none|
|» id|string|false|none|none|
