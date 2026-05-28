<h1 id="wso2-api-developer-portal-core-devportal-routes-invoices">Invoices</h1>

## List invoices

<a id="opIdlistInvoices"></a>

`GET /organizations/{orgId}/invoices`

> Code samples

```shell

curl -X GET http://localhost:3000/devportal/organizations/{orgId}/invoices \
  -u {username}:{password} \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}'

```

Lists invoices for the authenticated billing user. The `period` query controls the Stripe invoice created-date window.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="list-invoices-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|period|query|string|false|Invoice created-date window.|
|orgId|path|string|true|none|

#### Enumerated Values

|Parameter|Value|
|---|---|
|period|last3months|
|period|last6months|
|period|last12months|
|period|all|

> Example responses

> 200 Response

```json
{
  "invoices": [
    {
      "id": "in_12345",
      "number": "INV-001",
      "created": 1775088000,
      "amount": 2999,
      "currency": "usd",
      "status": "paid",
      "hostedInvoiceUrl": "https://invoice.stripe.com/i/acct/test",
      "invoicePdf": "https://pay.stripe.com/invoice/acct/test/pdf"
    }
  ]
}
```

> 400 Response

```json
{
  "error": "InternalServerError",
  "message": "An unexpected error occurred"
}
```

> 500 Response

```json
{
  "error": "InternalServerError",
  "message": "An unexpected error occurred"
}
```

> 502 Response

```json
{
  "error": "InternalServerError",
  "message": "An unexpected error occurred"
}
```

<h3 id="list-invoices-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Compact invoice list for the authenticated billing user.|[InvoiceListResponse](schemas.md#schemainvoicelistresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Invoice lookup error.|[ControllerErrorResponse](schemas.md#schemacontrollererrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Invoice lookup error.|[ControllerErrorResponse](schemas.md#schemacontrollererrorresponse)|
|502|[Bad Gateway](https://tools.ietf.org/html/rfc7231#section-6.6.3)|Invoice lookup error.|[ControllerErrorResponse](schemas.md#schemacontrollererrorresponse)|

## Get an invoice

<a id="opIdgetInvoice"></a>

`GET /organizations/{orgId}/invoices/{invoiceId}`

> Code samples

```shell

curl -X GET http://localhost:3000/devportal/organizations/{orgId}/invoices/{invoiceId} \
  -u {username}:{password} \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}'

```

Retrieves a Stripe invoice after verifying that the invoice customer belongs to the authenticated user.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="get-an-invoice-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|orgId|path|string|true|none|
|invoiceId|path|string|true|none|

> Example responses

> 200 Response

```json
{
  "id": "in_12345",
  "object": "invoice",
  "status": "paid",
  "customer": "cus_12345",
  "hosted_invoice_url": "https://invoice.stripe.com/i/acct/test",
  "invoice_pdf": "https://pay.stripe.com/invoice/acct/test/pdf"
}
```

> 400 Response

```json
{
  "error": "InternalServerError",
  "message": "An unexpected error occurred"
}
```

> 404 Response

```json
{
  "error": "InternalServerError",
  "message": "An unexpected error occurred"
}
```

> 500 Response

```json
{
  "error": "InternalServerError",
  "message": "An unexpected error occurred"
}
```

> 502 Response

```json
{
  "error": "InternalServerError",
  "message": "An unexpected error occurred"
}
```

<h3 id="get-an-invoice-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Stripe invoice object.|[StripeInvoiceResponse](schemas.md#schemastripeinvoiceresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Invoice lookup error.|[ControllerErrorResponse](schemas.md#schemacontrollererrorresponse)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Invoice lookup error.|[ControllerErrorResponse](schemas.md#schemacontrollererrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Invoice lookup error.|[ControllerErrorResponse](schemas.md#schemacontrollererrorresponse)|
|502|[Bad Gateway](https://tools.ietf.org/html/rfc7231#section-6.6.3)|Invoice lookup error.|[ControllerErrorResponse](schemas.md#schemacontrollererrorresponse)|

## List invoices by subscription

<a id="opIdlistInvoicesBySubscription"></a>

`GET /organizations/{orgId}/subscriptions/{subId}/invoices`

> Code samples

```shell

curl -X GET http://localhost:3000/devportal/organizations/{orgId}/subscriptions/{subId}/invoices \
  -u {username}:{password} \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}'

```

Lists Stripe invoices for a specific Developer Portal subscription after verifying subscription ownership. The response preserves Stripe's invoice list shape and filters it to the Stripe subscription linked to the Developer Portal subscription.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="list-invoices-by-subscription-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|orgId|path|string|true|none|
|subId|path|string|true|none|

> Example responses

> 200 Response

```json
{
  "object": "list",
  "data": [
    {
      "id": "in_12345",
      "object": "invoice",
      "status": "paid",
      "hosted_invoice_url": "https://invoice.stripe.com/i/acct/test",
      "invoice_pdf": "https://pay.stripe.com/invoice/acct/test/pdf"
    }
  ],
  "has_more": false
}
```

> 400 Response

```json
{
  "error": "InternalServerError",
  "message": "An unexpected error occurred"
}
```

> 404 Response

```json
{
  "error": "InternalServerError",
  "message": "An unexpected error occurred"
}
```

> 500 Response

```json
{
  "error": "InternalServerError",
  "message": "An unexpected error occurred"
}
```

> 502 Response

```json
{
  "error": "InternalServerError",
  "message": "An unexpected error occurred"
}
```

<h3 id="list-invoices-by-subscription-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Stripe invoice list response filtered by subscription.|[StripeInvoiceListResponse](schemas.md#schemastripeinvoicelistresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Invoice lookup error.|[ControllerErrorResponse](schemas.md#schemacontrollererrorresponse)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Invoice lookup error.|[ControllerErrorResponse](schemas.md#schemacontrollererrorresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Invoice lookup error.|[ControllerErrorResponse](schemas.md#schemacontrollererrorresponse)|
|502|[Bad Gateway](https://tools.ietf.org/html/rfc7231#section-6.6.3)|Invoice lookup error.|[ControllerErrorResponse](schemas.md#schemacontrollererrorresponse)|

## Get invoice PDF link

<a id="opIdgetInvoicePdfLink"></a>

`GET /organizations/{orgId}/invoices/{invoiceId}/pdf`

> Code samples

```shell

curl -X GET http://localhost:3000/devportal/organizations/{orgId}/invoices/{invoiceId}/pdf \
  -u {username}:{password} \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}'

```

Returns hosted invoice and PDF links for an invoice. A non-finalized invoice may not have a PDF link yet.

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="get-invoice-pdf-link-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|orgId|path|string|true|none|
|invoiceId|path|string|true|none|

> Example responses

> 200 Response

```json
{
  "invoiceId": "in_12345",
  "hosted_invoice_url": "https://invoice.stripe.com/i/acct/test",
  "invoice_pdf": "https://pay.stripe.com/invoice/acct/test/pdf"
}
```

> 400 Response

```json
{
  "error": "InternalServerError",
  "message": "An unexpected error occurred"
}
```

> 404 Response

```json
{
  "error": "InvoicePdfNotAvailable",
  "message": "Invoice PDF is not available for this invoice. It may not be finalized or paid yet.",
  "hosted_invoice_url": "https://invoice.stripe.com/i/acct/test"
}
```

> 500 Response

```json
{
  "error": "InternalServerError",
  "message": "An unexpected error occurred"
}
```

> 502 Response

```json
{
  "error": "InternalServerError",
  "message": "An unexpected error occurred"
}
```

<h3 id="get-invoice-pdf-link-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Hosted invoice and PDF links.|[InvoicePdfLinkResponse](schemas.md#schemainvoicepdflinkresponse)|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Invoice lookup error.|[ControllerErrorResponse](schemas.md#schemacontrollererrorresponse)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Invoice PDF is not available yet.|[InvoicePdfNotAvailableResponse](schemas.md#schemainvoicepdfnotavailableresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Invoice lookup error.|[ControllerErrorResponse](schemas.md#schemacontrollererrorresponse)|
|502|[Bad Gateway](https://tools.ietf.org/html/rfc7231#section-6.6.3)|Invoice lookup error.|[ControllerErrorResponse](schemas.md#schemacontrollererrorresponse)|

## redirectHostedInvoice

<a id="opIdredirectHostedInvoice"></a>

`GET /organizations/{orgId}/invoices/{invoiceId}/hosted`

> Code samples

```shell

curl -X GET http://localhost:3000/devportal/organizations/{orgId}/invoices/{invoiceId}/hosted \
  -u {username}:{password} \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer {access-token}'

```

### Authentication

<aside class="warning">
This operation requires <strong>Basic Auth</strong> authentication.

</aside>

<h3 id="redirecthostedinvoice-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|orgId|path|string|true|none|
|invoiceId|path|string|true|none|

> Example responses

> 400 Response

```json
{
  "error": "InternalServerError",
  "message": "An unexpected error occurred"
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
  "error": "InternalServerError",
  "message": "An unexpected error occurred"
}
```

> 502 Response

```json
{
  "error": "InternalServerError",
  "message": "An unexpected error occurred"
}
```

<h3 id="redirecthostedinvoice-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|302|[Found](https://tools.ietf.org/html/rfc7231#section-6.4.3)|Redirects to the hosted invoice URL.|None|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Invoice lookup error.|[ControllerErrorResponse](schemas.md#schemacontrollererrorresponse)|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|JSON message response.|[MessageResponse](schemas.md#schemamessageresponse)|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Invoice lookup error.|[ControllerErrorResponse](schemas.md#schemacontrollererrorresponse)|
|502|[Bad Gateway](https://tools.ietf.org/html/rfc7231#section-6.6.3)|Invoice lookup error.|[ControllerErrorResponse](schemas.md#schemacontrollererrorresponse)|

### Response Headers

|Status|Header|Type|Format|Description|
|---|---|---|---|---|
|302|Location|string|uri|Hosted Stripe invoice URL.|
