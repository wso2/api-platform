<h1 id="wso2-api-developer-portal-core-devportal-routes-billing">Billing</h1>

## Get billing usage data

<a id="opIdgetUsageData"></a>

`GET /organizations/{orgId}/billing/usage-data`

> Code samples

```shell

curl -X GET http://localhost:3000/devportal/organizations/{orgId}/billing/usage-data \
  -u {username}:{password} \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}'

```

Returns billing usage totals for the authenticated user in the organization. The optional period can be `current`, `last30`, or `last90`; `from` and `to` can be used for a custom date range.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="get-billing-usage-data-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|period|query|string|false|Usage period used when `from` and `to` are not supplied.|
|from|query|string(date)|false|Custom usage start date. The service expands this to the beginning of the day in UTC.|
|to|query|string(date)|false|Custom usage end date. The service expands this to the end of the day in UTC.|
|orgId|path|string|true|none|

#### Enumerated Values

|Parameter|Value|
|---|---|
|period|current|
|period|last30|
|period|last90|

> Example responses

> 200 Response

```json
{
  "totalRequests": 12450,
  "activeSubscriptions": 2,
  "estimatedCost": 29.99,
  "currency": "USD",
  "avgResponseTime": 142,
  "subscriptions": [
    {
      "apiName": "Weather API",
      "applicationName": "Weather App",
      "planName": "Pro",
      "requests": 12000,
      "pricingModel": "METERED",
      "cost": 24.99,
      "currency": "USD",
      "avgResponseTime": 145
    }
  ]
}
```

> Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.

```json
[
  {
    "code": "400",
    "message": "input validation failed",
    "description": "Invalid value"
  }
]
```

```json
{
  "code": "400",
  "message": "Bad Request",
  "description": "Missing required parameter: 'orgId'"
}
```

```json
{
  "message": "Missing or invalid fields in the request payload"
}
```

> 404 Response

```json
{
  "code": "404",
  "message": "Resource Not Found",
  "description": "Organization not found"
}
```

> 500 Response

```json
{
  "code": "500",
  "message": "Internal Server Error",
  "description": "Internal Server Error"
}
```

<h3 id="get-billing-usage-data-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Billing usage summary for the authenticated user.|[BillingUsageDataResponse](schemas.md#schemabillingusagedataresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.|Inline|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Resource not found.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="get-billing-usage-data-responseschema">Response Schema</h3>

## List payment methods

<a id="opIdgetPaymentMethods"></a>

`GET /organizations/{orgId}/billing/payment-methods`

> Code samples

```shell

curl -X GET http://localhost:3000/devportal/organizations/{orgId}/billing/payment-methods \
  -u {username}:{password} \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}'

```

Lists Stripe payment methods for the authenticated billing user. If no Stripe customer exists yet, the service can create or locate one by user email before listing methods.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="list-payment-methods-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|orgId|path|string|true|none|

> Example responses

> 200 Response

```json
{
  "paymentMethods": [
    {
      "id": "pm_12345",
      "brand": "visa",
      "last4": "4242",
      "expMonth": 12,
      "expYear": 2030,
      "isDefault": true
    }
  ]
}
```

> Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.

```json
[
  {
    "code": "400",
    "message": "input validation failed",
    "description": "Invalid value"
  }
]
```

```json
{
  "code": "400",
  "message": "Bad Request",
  "description": "Missing required parameter: 'orgId'"
}
```

```json
{
  "message": "Missing or invalid fields in the request payload"
}
```

> 500 Response

```json
{
  "code": "500",
  "message": "Internal Server Error",
  "description": "Internal Server Error"
}
```

<h3 id="list-payment-methods-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Stripe payment methods for the authenticated billing user.|[PaymentMethodsResponse](schemas.md#schemapaymentmethodsresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.|Inline|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="list-payment-methods-responseschema">Response Schema</h3>

## Add billing engine keys

<a id="opIdaddBillingEngineKeys"></a>

`POST /organizations/{orgId}/billing-engine-keys`

> Code samples

```shell

curl -X POST http://localhost:3000/devportal/organizations/{orgId}/billing-engine-keys \
  -u {username}:{password} \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}' \
  -d @payload.json

```

Stores encrypted billing engine credentials for the organization. Currently used for Stripe secret, publishable, and webhook keys.

> Payload

```json
{
  "billingEngine": "STRIPE",
  "secretKey": "sk_test_123",
  "publishableKey": "pk_test_123",
  "webhookSecret": "whsec_123"
}
```

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="add-billing-engine-keys-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|[BillingEngineKeysRequest](schemas.md#schemabillingenginekeysrequest)|true|Billing engine credentials to encrypt and store for the organization.|
|orgId|path|string|true|none|

> Example responses

> 201 Response

```json
{
  "message": "string"
}
```

> Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.

```json
[
  {
    "code": "400",
    "message": "input validation failed",
    "description": "Invalid value"
  }
]
```

```json
{
  "code": "400",
  "message": "Bad Request",
  "description": "Missing required parameter: 'orgId'"
}
```

```json
{
  "message": "Missing or invalid fields in the request payload"
}
```

> 500 Response

```json
{
  "code": "500",
  "message": "Internal Server Error",
  "description": "Internal Server Error"
}
```

<h3 id="add-billing-engine-keys-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|201|[Created](https://tools.ietf.org/html/rfc7231#section-6.3.2)|JSON message response.|[MessageResponse](schemas.md#schemamessageresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.|Inline|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="add-billing-engine-keys-responseschema">Response Schema</h3>

## Update billing engine keys

<a id="opIdupdateBillingEngineKeys"></a>

`PUT /organizations/{orgId}/billing-engine-keys`

> Code samples

```shell

curl -X PUT http://localhost:3000/devportal/organizations/{orgId}/billing-engine-keys \
  -u {username}:{password} \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}' \
  -d @payload.json

```

Updates the encrypted billing engine credentials for an existing organization billing configuration.

> Payload

```json
{
  "billingEngine": "STRIPE",
  "secretKey": "sk_test_123",
  "publishableKey": "pk_test_123",
  "webhookSecret": "whsec_123"
}
```

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="update-billing-engine-keys-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|[BillingEngineKeysRequest](schemas.md#schemabillingenginekeysrequest)|true|Billing engine credentials to encrypt and store for the organization.|
|orgId|path|string|true|none|

> Example responses

> 200 Response

```json
{
  "message": "string"
}
```

> Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.

```json
[
  {
    "code": "400",
    "message": "input validation failed",
    "description": "Invalid value"
  }
]
```

```json
{
  "code": "400",
  "message": "Bad Request",
  "description": "Missing required parameter: 'orgId'"
}
```

```json
{
  "message": "Missing or invalid fields in the request payload"
}
```

> 500 Response

```json
{
  "code": "500",
  "message": "Internal Server Error",
  "description": "Internal Server Error"
}
```

<h3 id="update-billing-engine-keys-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|JSON message response.|[MessageResponse](schemas.md#schemamessageresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.|Inline|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|JSON message response.|[MessageResponse](schemas.md#schemamessageresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="update-billing-engine-keys-responseschema">Response Schema</h3>

## Delete billing engine keys

<a id="opIddeleteBillingEngineKeys"></a>

`DELETE /organizations/{orgId}/billing-engine-keys`

> Code samples

```shell

curl -X DELETE http://localhost:3000/devportal/organizations/{orgId}/billing-engine-keys \
  -u {username}:{password} \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}' \
  -d @payload.json

```

Deletes billing engine credentials for the organization. `billingEngine` may be supplied in the request body or as a query parameter.

> Payload

```json
{
  "billingEngine": "STRIPE"
}
```

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="delete-billing-engine-keys-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|billingEngine|query|string|false|Alternative to sending `billingEngine` in the delete request body.|
|body|body|[BillingEngineDeleteRequest](schemas.md#schemabillingenginedeleterequest)|false|Optional billing engine selector. The same value can also be supplied with the `billingEngine` query parameter.|
|orgId|path|string|true|none|

> Example responses

> 200 Response

```json
{
  "message": "string"
}
```

> Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.

```json
[
  {
    "code": "400",
    "message": "input validation failed",
    "description": "Invalid value"
  }
]
```

```json
{
  "code": "400",
  "message": "Bad Request",
  "description": "Missing required parameter: 'orgId'"
}
```

```json
{
  "message": "Missing or invalid fields in the request payload"
}
```

> 500 Response

```json
{
  "code": "500",
  "message": "Internal Server Error",
  "description": "Internal Server Error"
}
```

<h3 id="delete-billing-engine-keys-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|JSON message response.|[MessageResponse](schemas.md#schemamessageresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.|Inline|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|JSON message response.|[MessageResponse](schemas.md#schemamessageresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="delete-billing-engine-keys-responseschema">Response Schema</h3>

## Get billing engine keys

<a id="opIdgetBillingEngineKeys"></a>

`GET /organizations/{orgId}/billing-engine-keys`

> Code samples

```shell

curl -X GET http://localhost:3000/devportal/organizations/{orgId}/billing-engine-keys \
  -u {username}:{password} \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}'

```

Returns masked billing engine key metadata. Secret values are never returned; configured values are represented as `****`.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="get-billing-engine-keys-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|billingEngine|query|string|false|Alternative to sending `billingEngine` in the delete request body.|
|orgId|path|string|true|none|

> Example responses

> 200 Response

```json
{
  "billingEngine": "STRIPE",
  "secretKey": "****",
  "publishableKey": "****",
  "webhookSecret": "****"
}
```

> Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.

```json
[
  {
    "code": "400",
    "message": "input validation failed",
    "description": "Invalid value"
  }
]
```

```json
{
  "code": "400",
  "message": "Bad Request",
  "description": "Missing required parameter: 'orgId'"
}
```

```json
{
  "message": "Missing or invalid fields in the request payload"
}
```

> 404 Response

```json
{
  "message": "string"
}
```

> 500 Response

```json
{
  "code": "500",
  "message": "Internal Server Error",
  "description": "Internal Server Error"
}
```

<h3 id="get-billing-engine-keys-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Masked billing engine credentials.|[BillingEngineKeysResponse](schemas.md#schemabillingenginekeysresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.|Inline|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|JSON message response.|[MessageResponse](schemas.md#schemamessageresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="get-billing-engine-keys-responseschema">Response Schema</h3>

## Get billing profile information

<a id="opIdgetBillingInfo"></a>

`GET /organizations/{orgId}/billing/info`

> Code samples

```shell

curl -X GET http://localhost:3000/devportal/organizations/{orgId}/billing/info \
  -u {username}:{password} \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}'

```

Returns billing profile information for the authenticated user and organization. When a Stripe customer exists, the response includes customer address and tax ID information from Stripe.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="get-billing-profile-information-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|orgId|path|string|true|none|

> Example responses

> 200 Response

```json
{
  "organizationName": "Acme Inc",
  "email": "dev@example.com",
  "address": {
    "line1": "100 Market Street",
    "city": "San Francisco",
    "country": "US"
  },
  "taxId": "123456789"
}
```

> Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.

```json
[
  {
    "code": "400",
    "message": "input validation failed",
    "description": "Invalid value"
  }
]
```

```json
{
  "code": "400",
  "message": "Bad Request",
  "description": "Missing required parameter: 'orgId'"
}
```

```json
{
  "message": "Missing or invalid fields in the request payload"
}
```

> 500 Response

```json
{
  "code": "500",
  "message": "Internal Server Error",
  "description": "Internal Server Error"
}
```

<h3 id="get-billing-profile-information-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Billing profile information for the authenticated user.|[BillingInfoResponse](schemas.md#schemabillinginforesponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.|Inline|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="get-billing-profile-information-responseschema">Response Schema</h3>

## List subscriptions for billing

<a id="opIdgetActiveSubscriptions"></a>

`GET /organizations/{orgId}/billing/subscriptions`

> Code samples

```shell

curl -X GET http://localhost:3000/devportal/organizations/{orgId}/billing/subscriptions \
  -u {username}:{password} \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}'

```

Returns the authenticated user's subscriptions formatted for the billing page, including plan, billing cycle, amount, currency, next billing date, and payment status.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="list-subscriptions-for-billing-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|orgId|path|string|true|none|

> Example responses

> 200 Response

```json
{
  "subscriptions": [
    {
      "id": "sub-12345",
      "apiName": "Weather API",
      "applicationName": "Weather App",
      "planName": "Pro",
      "billingCycle": "Month",
      "amount": 2999,
      "currency": "usd",
      "nextBillingDate": 1775088000000,
      "status": "active"
    }
  ]
}
```

> 500 Response

```json
{
  "code": "500",
  "message": "Internal Server Error",
  "description": "Internal Server Error"
}
```

<h3 id="list-subscriptions-for-billing-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Subscriptions formatted for billing page display.|[BillingSubscriptionsResponse](schemas.md#schemabillingsubscriptionsresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

## Create a checkout session

<a id="opIdcreateCheckoutSessionForSubscription"></a>

`POST /organizations/{orgId}/monetization/checkout`

> Code samples

```shell

curl -X POST http://localhost:3000/devportal/organizations/{orgId}/monetization/checkout \
  -u {username}:{password} \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}' \
  -d @payload.json

```

Creates a pending Developer Portal subscription row and a Stripe embedded checkout session for a paid subscription policy.

> Payload

```json
{
  "applicationID": "app-12345",
  "apiId": "api-7f4c2a6b",
  "apiReferenceID": "cp-api-12345",
  "policyId": "policy-pro",
  "policyName": "Pro",
  "sourcePage": "/devportal/apis/weather-api-v1"
}
```

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="create-a-checkout-session-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|[CheckoutSessionRequest](schemas.md#schemacheckoutsessionrequest)|true|Paid subscription checkout payload.|
|orgId|path|string|true|none|

> Example responses

> 200 Response

```json
{
  "subId": "sub-12345",
  "checkoutSessionId": "cs_test_12345",
  "clientSecret": "cs_test_12345_secret_abc",
  "publishableKey": "pk_test_123",
  "billingCustomerId": "cus_12345",
  "paymentProvider": "STRIPE",
  "paymentStatus": "PENDING"
}
```

> Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.

```json
[
  {
    "code": "400",
    "message": "input validation failed",
    "description": "Invalid value"
  }
]
```

```json
{
  "code": "400",
  "message": "Bad Request",
  "description": "Missing required parameter: 'orgId'"
}
```

```json
{
  "message": "Missing or invalid fields in the request payload"
}
```

> 404 Response

```json
{
  "code": "404",
  "message": "Resource Not Found",
  "description": "Organization not found"
}
```

> 409 Response

```json
{
  "message": "string"
}
```

> 500 Response

```json
{
  "code": "500",
  "message": "Internal Server Error",
  "description": "Internal Server Error"
}
```

<h3 id="create-a-checkout-session-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Stripe checkout session details for a pending paid subscription.|[CheckoutSessionResponse](schemas.md#schemacheckoutsessionresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.|Inline|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Resource not found.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|JSON message response.|[MessageResponse](schemas.md#schemamessageresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|502|[Bad Gateway](https://tools.ietf.org/html/rfc7231#section-6.6.3)|JSON message response.|[MessageResponse](schemas.md#schemamessageresponse)|

<h3 id="create-a-checkout-session-responseschema">Response Schema</h3>

## Register a Stripe checkout session

<a id="opIdregisterStripeCheckoutSession"></a>

`POST /organizations/{orgId}/monetization/stripe/register/{checkoutSessionId}`

> Code samples

```shell

curl -X POST http://localhost:3000/devportal/organizations/{orgId}/monetization/stripe/register/{checkoutSessionId} \
  -u {username}:{password} \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}'

```

Verifies the Stripe checkout session, activates the Developer Portal subscription, and synchronizes the subscription into the control plane when application key mappings already exist.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="register-a-stripe-checkout-session-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|orgId|path|string|true|none|
|checkoutSessionId|path|string|true|none|

> Example responses

> 201 Response

```json
{
  "subId": "sub-12345",
  "billingCustomerId": "cus_12345",
  "billingSubscriptionId": "sub_stripe_12345",
  "paymentStatus": "ACTIVE",
  "paymentProvider": "STRIPE"
}
```

> Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.

```json
[
  {
    "code": "400",
    "message": "input validation failed",
    "description": "Invalid value"
  }
]
```

```json
{
  "code": "400",
  "message": "Bad Request",
  "description": "Missing required parameter: 'orgId'"
}
```

```json
{
  "message": "Missing or invalid fields in the request payload"
}
```

> 404 Response

```json
{
  "code": "404",
  "message": "Resource Not Found",
  "description": "Organization not found"
}
```

> 409 Response

```json
{
  "message": "string"
}
```

> 500 Response

```json
{
  "code": "500",
  "message": "Internal Server Error",
  "description": "Internal Server Error"
}
```

<h3 id="register-a-stripe-checkout-session-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|201|[Created](https://tools.ietf.org/html/rfc7231#section-6.3.2)|Registered checkout session and activated subscription details.|[CheckoutRegistrationResponse](schemas.md#schemacheckoutregistrationresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.|Inline|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Resource not found.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|JSON message response.|[MessageResponse](schemas.md#schemamessageresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|502|[Bad Gateway](https://tools.ietf.org/html/rfc7231#section-6.6.3)|JSON message response.|[MessageResponse](schemas.md#schemamessageresponse)|

<h3 id="register-a-stripe-checkout-session-responseschema">Response Schema</h3>

## Cancel a paid subscription

<a id="opIdcancelSubscription"></a>

`POST /organizations/{orgId}/subscriptions/{subId}/cancel`

> Code samples

```shell

curl -X POST http://localhost:3000/devportal/organizations/{orgId}/subscriptions/{subId}/cancel \
  -u {username}:{password} \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}'

```

Cancels a paid Stripe subscription, marks the Developer Portal subscription as canceled, and schedules non-fatal Moesif billing cleanup.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="cancel-a-paid-subscription-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|orgId|path|string|true|none|
|subId|path|string|true|none|

> Example responses

> 200 Response

```json
{
  "subId": "sub-12345",
  "paymentStatus": "CANCELED"
}
```

> Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.

```json
[
  {
    "code": "400",
    "message": "input validation failed",
    "description": "Invalid value"
  }
]
```

```json
{
  "code": "400",
  "message": "Bad Request",
  "description": "Missing required parameter: 'orgId'"
}
```

```json
{
  "message": "Missing or invalid fields in the request payload"
}
```

> 404 Response

```json
{
  "code": "404",
  "message": "Resource Not Found",
  "description": "Organization not found"
}
```

> 500 Response

```json
{
  "code": "500",
  "message": "Internal Server Error",
  "description": "Internal Server Error"
}
```

> 502 Response

```json
{
  "message": "string"
}
```

<h3 id="cancel-a-paid-subscription-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Paid subscription cancellation result.|[BillingCancelResponse](schemas.md#schemabillingcancelresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.|Inline|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Resource not found.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|502|[Bad Gateway](https://tools.ietf.org/html/rfc7231#section-6.6.3)|JSON message response.|[MessageResponse](schemas.md#schemamessageresponse)|

<h3 id="cancel-a-paid-subscription-responseschema">Response Schema</h3>

## Get subscription billing status

<a id="opIdgetSubscriptionBillingStatus"></a>

`GET /organizations/{orgId}/subscriptions/{subId}/billing-status`

> Code samples

```shell

curl -X GET http://localhost:3000/devportal/organizations/{orgId}/subscriptions/{subId}/billing-status \
  -u {username}:{password} \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}'

```

Returns billing status and Stripe billing identifiers for a Developer Portal subscription.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="get-subscription-billing-status-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|orgId|path|string|true|none|
|subId|path|string|true|none|

> Example responses

> 200 Response

```json
{
  "subId": "sub-12345",
  "paymentProvider": "STRIPE",
  "paymentStatus": "ACTIVE",
  "billingCustomerId": "cus_12345",
  "billingSubscriptionId": "sub_stripe_12345"
}
```

> Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.

```json
[
  {
    "code": "400",
    "message": "input validation failed",
    "description": "Invalid value"
  }
]
```

```json
{
  "code": "400",
  "message": "Bad Request",
  "description": "Missing required parameter: 'orgId'"
}
```

```json
{
  "message": "Missing or invalid fields in the request payload"
}
```

> 404 Response

```json
{
  "code": "404",
  "message": "Resource Not Found",
  "description": "Organization not found"
}
```

> 500 Response

```json
{
  "code": "500",
  "message": "Internal Server Error",
  "description": "Internal Server Error"
}
```

<h3 id="get-subscription-billing-status-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Billing status for a Developer Portal subscription.|[SubscriptionBillingStatusResponse](schemas.md#schemasubscriptionbillingstatusresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.|Inline|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Resource not found.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|

<h3 id="get-subscription-billing-status-responseschema">Response Schema</h3>

## Create an organization billing portal session

<a id="opIdcreateBillingPortalByOrg"></a>

`POST /organizations/{orgId}/billing-portal`

> Code samples

```shell

curl -X POST http://localhost:3000/devportal/organizations/{orgId}/billing-portal \
  -u {username}:{password} \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}'

```

Creates a Stripe customer portal session for the authenticated user's organization. If no billed subscription exists yet, the service can create or locate a Stripe customer by user email.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="create-an-organization-billing-portal-session-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|orgId|path|string|true|none|

> Example responses

> 200 Response

```json
{
  "url": "https://billing.stripe.com/p/session/test_123"
}
```

> Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.

```json
[
  {
    "code": "400",
    "message": "input validation failed",
    "description": "Invalid value"
  }
]
```

```json
{
  "code": "400",
  "message": "Bad Request",
  "description": "Missing required parameter: 'orgId'"
}
```

```json
{
  "message": "Missing or invalid fields in the request payload"
}
```

> 500 Response

```json
{
  "code": "500",
  "message": "Internal Server Error",
  "description": "Internal Server Error"
}
```

> 502 Response

```json
{
  "message": "string"
}
```

<h3 id="create-an-organization-billing-portal-session-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Stripe billing portal session URL.|[BillingPortalResponse](schemas.md#schemabillingportalresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.|Inline|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|502|[Bad Gateway](https://tools.ietf.org/html/rfc7231#section-6.6.3)|JSON message response.|[MessageResponse](schemas.md#schemamessageresponse)|

<h3 id="create-an-organization-billing-portal-session-responseschema">Response Schema</h3>

## Create a subscription billing portal session

<a id="opIdcreateBillingPortal"></a>

`POST /organizations/{orgId}/subscriptions/{subId}/billing-portal`

> Code samples

```shell

curl -X POST http://localhost:3000/devportal/organizations/{orgId}/subscriptions/{subId}/billing-portal \
  -u {username}:{password} \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}'

```

Creates a Stripe customer portal session for a specific paid subscription.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="create-a-subscription-billing-portal-session-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|orgId|path|string|true|none|
|subId|path|string|true|none|

> Example responses

> 200 Response

```json
{
  "url": "https://billing.stripe.com/p/session/test_123"
}
```

> Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.

```json
[
  {
    "code": "400",
    "message": "input validation failed",
    "description": "Invalid value"
  }
]
```

```json
{
  "code": "400",
  "message": "Bad Request",
  "description": "Missing required parameter: 'orgId'"
}
```

```json
{
  "message": "Missing or invalid fields in the request payload"
}
```

> 404 Response

```json
{
  "code": "404",
  "message": "Resource Not Found",
  "description": "Organization not found"
}
```

> 500 Response

```json
{
  "code": "500",
  "message": "Internal Server Error",
  "description": "Internal Server Error"
}
```

> 502 Response

```json
{
  "message": "string"
}
```

<h3 id="create-a-subscription-billing-portal-session-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Stripe billing portal session URL.|[BillingPortalResponse](schemas.md#schemabillingportalresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request. Input validation failures are returned as an array; other bad request errors are returned as a standard error object.|Inline|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Resource not found.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error.|[ErrorResponse](schemas.md#schemaerrorresponse)|
|502|[Bad Gateway](https://tools.ietf.org/html/rfc7231#section-6.6.3)|JSON message response.|[MessageResponse](schemas.md#schemamessageresponse)|

<h3 id="create-a-subscription-billing-portal-session-responseschema">Response Schema</h3>
