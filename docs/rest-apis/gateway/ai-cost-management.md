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
    "filters": [
      {
        "type": "llmProvider",
        "id": "openai"
      },
      {
        "type": "attribute",
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
    "filters": [
      {
        "type": "llmProvider",
        "id": "openai"
      },
      {
        "type": "attribute",
        "expression": "'research' in request.Auth.Claims[\"groups\"]"
      }
    ]
  },
  "status": {
    "id": "team-daily",
    "createdAt": "2026-04-24T07:21:13Z",
    "updatedAt": "2026-04-24T07:21:13Z",
    "currentPeriod": {
      "periodStart": "2026-11-01",
      "spent": "312.4",
      "unpricedRequests": 0
    }
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
        "filters": [
          {
            "type": "llmProvider",
            "id": "openai"
          },
          {
            "type": "attribute",
            "expression": "'research' in request.Auth.Claims[\"groups\"]"
          }
        ]
      },
      "status": {
        "id": "team-daily",
        "createdAt": "2026-04-24T07:21:13Z",
        "updatedAt": "2026-04-24T07:21:13Z",
        "currentPeriod": {
          "periodStart": "2026-11-01",
          "spent": "312.4",
          "unpricedRequests": 0
        }
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
|»»» apiVersion|string|true|none|Budget specification version|
|»»» kind|string|true|none|Budget kind|
|»»» metadata|[BudgetMetadata](schemas.md#schemabudgetmetadata)|true|none|none|
|»»»» name|string|true|none|The budget's handle, unique on the gateway. Lowercase letters, digits, '-' and '.'.|
|»»»» labels|[string]|false|none|One-word labels to organize and find budgets. Not used to enforce; kept with the budget and pushed to the control plane with it. At most 10, no repeats; each 1-63 characters of letters, digits, '.', '_' or '-', starting and ending with a letter or digit (no spaces).|
|»»»» annotations|object|false|none|Non-identifying metadata, as on the gateway's other kinds. gateway.api-platform.wso2.com/artifact-id sets the budget's UUID, which cannot change afterwards; without it the gateway assigns one. gateway.api-platform.wso2.com/project-id names the project that owns the budget in the control plane; without it, the organization owns it. A project budget counts only that project: when its filters have no project filter, {type: project, id} is added as the last filter. It exempts only that project's agents and cannot be split by project.|
|»»»»» **additionalProperties**|string|false|none|none|
|»»» spec|[BudgetSpec](schemas.md#schemabudgetspec)|true|none|none|
|»»»» status|string|false|none|An inactive budget is kept but matches no traffic.|
|»»»» limit|[BudgetLimit](schemas.md#schemabudgetlimit)|true|none|none|
|»»»»» amount|string|true|none|A positive amount with up to 2 decimal places, such as "100.00".|
|»»»»» currency|string|false|none|none|
|»»»»» period|string|true|none|UTC calendar day, ISO week starting Monday, or calendar month.|
|»»»»» action|string|true|none|block refuses requests at the block point; allow only counts.|
|»»»»» blockAt|integer|false|none|Percentage of the amount at which a block budget refuses requests.|
|»»»»» splitBy|[BudgetSplit](schemas.md#schemabudgetsplit)|false|none|Gives each value of the type its own limit: each LLM provider, project, agent or API key, or each value of an attribute expression. Omitted means one shared limit for all matching usage.|
|»»»»»» type|string|true|none|none|
|»»»»»» expression|string|false|none|attribute only. CEL over the request returning a string or a list of strings.|
|»»»» filters|[[BudgetFilter](schemas.md#schemabudgetfilter)]|false|none|Which LLM usage the budget counts. Every filter must match (AND). Empty means all LLM usage.|
|»»»»» type|string|true|none|none|
|»»»»» id|string|false|none|The LLM provider, project, agent or API key name. Required for llmProvider; omitted for project, agent or apiKey means any one. Not for an attribute.|
|»»»»» project|string|false|none|agent only, with id: the agent's project; agent names are unique only per project. Omit both for any agent.|
|»»»»» expression|string|false|none|attribute only. CEL condition over the request, returning bool.|
|»»»» exemptions|[[BudgetExemption](schemas.md#schemabudgetexemption)]|false|none|Matching traffic is left out of this budget.|
|»»»»» type|string|true|none|none|
|»»»»» expression|string|false|none|attribute only. CEL condition over the request.|
|»»»»» id|string|false|none|The project or agent name.|
|»»»»» project|string|false|none|agent only. The agent's project.|
|»»»» displayName|string|false|none|Accepted for compatibility with the control plane's Budget; ignored by the gateway (not validated, stored or returned).|
|»»»» description|string|false|none|Accepted for compatibility with the control plane's Budget; ignored by the gateway (not validated, stored or returned).|
|»»»» warnings|object|false|none|Accepted for compatibility with the control plane's Budget, which sends its warnings to notification channels; ignored by the gateway (not validated, stored or returned).|
|»»»»» thresholds|[integer]|false|none|none|
|»»»»» channels|[string]|false|none|none|

*and*

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|»» *anonymous*|object|false|none|none|
|»»» status|[BudgetStatus](schemas.md#schemabudgetstatus)|false|read-only|Server-managed fields. Populated on responses.|
|»»»» id|string|false|none|Unique identifier assigned by the server (equal to metadata.name)|
|»»»» createdAt|string(date-time)|false|none|Timestamp when the budget was first created (UTC)|
|»»»» updatedAt|string(date-time)|false|none|Timestamp when the budget was last updated (UTC)|
|»»»» currentPeriod|[BudgetCurrentPeriod](schemas.md#schemabudgetcurrentperiod)|false|none|Spend in the current period, as the control plane shows it: totals include every gateway sharing the counter store.|
|»»»»» periodStart|string(date)|true|none|none|
|»»»»» spent|string|true|none|USD, total across the budget's allowances|
|»»»»» unpricedRequests|integer(int64)|true|none|Requests charged $0 this period because they had no price|

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
|type|project|
|type|agent|
|type|apiKey|
|type|attribute|
|type|llmProvider|
|type|project|
|type|agent|
|type|apiKey|
|type|attribute|
|type|attribute|
|type|project|
|type|agent|

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
    "filters": [
      {
        "type": "llmProvider",
        "id": "openai"
      },
      {
        "type": "attribute",
        "expression": "'research' in request.Auth.Claims[\"groups\"]"
      }
    ]
  },
  "status": {
    "id": "team-daily",
    "createdAt": "2026-04-24T07:21:13Z",
    "updatedAt": "2026-04-24T07:21:13Z",
    "currentPeriod": {
      "periodStart": "2026-11-01",
      "spent": "312.4",
      "unpricedRequests": 0
    }
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

Update a budget. Only spec.status, spec.limit.amount, spec.limit.action, spec.limit.blockAt and spec.exemptions can change; every other field must match the stored budget. To change the period, splitBy or filters, create a new budget. Budgets from the control plane cannot be changed here.

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
    "filters": [
      {
        "type": "llmProvider",
        "id": "openai"
      },
      {
        "type": "attribute",
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
    "filters": [
      {
        "type": "llmProvider",
        "id": "openai"
      },
      {
        "type": "attribute",
        "expression": "'research' in request.Auth.Claims[\"groups\"]"
      }
    ]
  },
  "status": {
    "id": "team-daily",
    "createdAt": "2026-04-24T07:21:13Z",
    "updatedAt": "2026-04-24T07:21:13Z",
    "currentPeriod": {
      "periodStart": "2026-11-01",
      "spent": "312.4",
      "unpricedRequests": 0
    }
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

## List a budget's usage

<a id="opIdgetBudgetUsage"></a>

`GET /budgets/{id}/usage`

> Code samples

```shell

curl -X GET http://localhost:9090/api/management/v1/budgets/{id}/usage \
  -u {username}:{password} \
  -H 'Accept: application/json'

```

The budget's spend in the current period, one row per allowance, highest spend first: the same as the control plane's budget usage. Totals include every gateway sharing the counter store; an allowance's values are known once this gateway has served it, and are empty otherwise (and for a shared allowance).

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

Required roles: `admin`

</aside>

<h3 id="list-a-budget's-usage-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|string|true|The budget's name (metadata.name)|
|limit|query|integer|false|none|
|offset|query|integer|false|none|

> Example responses
>
> 200 Response

```json
{
  "periodStart": "2019-08-24",
  "allowances": [
    {
      "allowance": {
        "property1": "string",
        "property2": "string"
      },
      "spent": "104.21",
      "unpricedRequests": 0
    }
  ],
  "total": 0,
  "limit": 0,
  "offset": 0
}
```

<h3 id="list-a-budget's-usage-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|The budget's usage this period|[BudgetUsageListResponse](schemas.md#schemabudgetusagelistresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Invalid limit or offset|[ErrorResponse](schemas.md#schemaerrorresponse)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Budget not found|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|[ErrorResponse](schemas.md#schemaerrorresponse)|
