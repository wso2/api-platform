---
title: "Overview"
---
# JSON/XML Mediator

## Overview

The JSON/XML Mediator policy provides bidirectional payload transformation between JSON and XML formats. It operates on both request and response flows, converting requests from the downstream client format to the upstream service format, and converting responses from the upstream format back to the downstream format.

This policy is designed for integration scenarios where downstream clients and upstream services use different payload formats.

## Features

- **Bidirectional Transformation**: Converts payloads in both request and response flows
- **Format Configuration**: Specify expected formats for downstream clients and upstream services
- **Automatic Content-Type Management**: Updates Content-Type headers appropriately after transformation
- **Intelligent Element Naming**: Handles JSON arrays with singularized XML element names
- **Proper XML Formatting**: Generates well-formed XML with declaration and proper escaping
- **Robust JSON Parsing**: Handles all JSON data types (objects, arrays, strings, numbers, booleans, null)
- **Content-Length Updates**: Automatically adjusts Content-Length headers for transformed payloads

## Configuration

The JSON/XML Mediator policy uses a single-level configuration where all parameters are configured in the API definition YAML.

### User Parameters (API Definition)

These parameters are configured per-API/route by the API developer:

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `downsteamPayloadFormat` | string | Yes | Defines the payload format expected by downstream clients. Requests are converted from this format before reaching upstream, and responses are converted back to this format before returning to the client. Valid values: `"xml"`, `"json"`. This value must differ from `upstreamPayloadFormat`. |
| `upstreamPayloadFormat` | string | Yes | Defines the payload format expected by the upstream service. Requests are converted to this format before reaching upstream, and responses are interpreted as arriving in this format before any downstream conversion is applied. Valid values: `"xml"`, `"json"`. |

**Note:**

- The `downsteamPayloadFormat` and `upstreamPayloadFormat` must be different. Configuring both as `"xml"` or both as `"json"` is not allowed.

Inside the `gateway/build.yaml`, ensure the policy module is added under `policies:`:

```yaml
- name: json-xml-mediator
  gomodule: github.com/wso2/gateway-controllers/policies/json-xml-mediator@v0
```

## Reference Scenarios

### Example 1: JSON Client to XML Backend

Transform JSON requests from clients to XML for a legacy backend, and XML responses back to JSON for clients:

```yaml
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: RestApi
metadata:
  name: legacy-integration-api-v1.0
spec:
  displayName: Legacy Integration API
  version: v1.0
  context: /legacy/$version
  upstream:
    main:
      url: http://xml-backend:8080
  policies:
    - name: json-xml-mediator
      version: v0
      params:
        downsteamPayloadFormat: json
        upstreamPayloadFormat: xml
  operations:
    - method: POST
      path: /orders
    - method: PUT
      path: /orders/{id}
    - method: GET
      path: /orders/{id}
```

**Request transformation:**

Original client request (JSON):
```http
POST /legacy/v1.0/orders HTTP/1.1
Host: api-gateway.example.com
Content-Type: application/json

{
  "orderId": "12345",
  "customer": "John Doe",
  "items": [
    {"name": "Widget", "quantity": 2},
    {"name": "Gadget", "quantity": 1}
  ]
}
```

Transformed upstream request (XML):
```http
POST /orders HTTP/1.1
Host: xml-backend:8080
Content-Type: application/xml

<?xml version="1.0" encoding="UTF-8"?>
<root>
  <orderId>12345</orderId>
  <customer>John Doe</customer>
  <items>
    <item>
      <name>Widget</name>
      <quantity>2</quantity>
    </item>
    <item>
      <name>Gadget</name>
      <quantity>1</quantity>
    </item>
  </items>
</root>
```

**Response transformation:**

Original upstream response (XML):
```http
HTTP/1.1 200 OK
Content-Type: application/xml

<?xml version="1.0" encoding="UTF-8"?>
<orderResponse>
  <status>created</status>
  <orderId>12345</orderId>
</orderResponse>
```

Transformed client response (JSON):
```http
HTTP/1.1 200 OK
Content-Type: application/json

{
  "orderResponse": {
    "status": "created",
    "orderId": "12345"
  }
}
```

### Example 2: XML Client to JSON Backend

Transform XML requests from clients to JSON for a modern REST backend, and JSON responses back to XML for clients:

```yaml
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: RestApi
metadata:
  name: soap-to-rest-api-v1.0
spec:
  displayName: SOAP to REST API
  version: v1.0
  context: /soap-bridge/$version
  upstream:
    main:
      url: http://rest-service:8080
  policies:
    - name: json-xml-mediator
      version: v0
      params:
        downsteamPayloadFormat: xml
        upstreamPayloadFormat: json
  operations:
    - method: POST
      path: /users
    - method: GET
      path: /users/{id}
```

**Request transformation:**

Original client request (XML):
```http
POST /soap-bridge/v1.0/users HTTP/1.1
Host: api-gateway.example.com
Content-Type: application/xml

<?xml version="1.0" encoding="UTF-8"?>
<user>
  <name>Jane Smith</name>
  <email>jane@example.com</email>
  <role>admin</role>
</user>
```

Transformed upstream request (JSON):
```http
POST /users HTTP/1.1
Host: rest-service:8080
Content-Type: application/json

{
  "user": {
    "name": "Jane Smith",
    "email": "jane@example.com",
    "role": "admin"
  }
}
```

### Example 3: Operation-Level Configuration

Apply different mediation settings to specific operations:

```yaml
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: RestApi
metadata:
  name: hybrid-api-v1.0
spec:
  displayName: Hybrid API
  version: v1.0
  context: /hybrid/$version
  upstream:
    main:
      url: http://backend:8080
  operations:
    - method: POST
      path: /xml-endpoint
      policies:
        - name: json-xml-mediator
          version: v0
          params:
            downsteamPayloadFormat: json
            upstreamPayloadFormat: xml
    - method: POST
      path: /json-endpoint
      policies:
        - name: json-xml-mediator
          version: v0
          params:
            downsteamPayloadFormat: xml
            upstreamPayloadFormat: json
    - method: GET
      path: /passthrough
      # No mediator - payload passed through unchanged
```


## How it Works

* **Request Flow**: When a request arrives, the policy checks the `downsteamPayloadFormat`. If transformation is needed (downstream format differs from upstream format), the request body is converted to the `upstreamPayloadFormat` before forwarding to the upstream service.

* **Response Flow**: When a response returns from upstream, the policy converts the body from the `upstreamPayloadFormat` back to the `downsteamPayloadFormat` before returning to the client.

* **JSON to XML**: Converts JSON objects to XML elements, JSON arrays to repeated XML elements with singularized names, and handles all JSON primitive types appropriately.

* **XML to JSON**: Parses XML elements into JSON objects, handles attributes and text content, and converts repeated elements to JSON arrays.

* **Content-Type Headers**: The policy automatically updates the `Content-Type` header to match the transformed payload format (`application/json` or `application/xml`).


## Limitations

1. **Binary Content**: Does not support transformation of binary payloads
2. **Streaming**: Large payloads are buffered in memory for transformation
3. **XML Namespaces**: Complex XML namespace handling may require additional configuration
4. **Schema Validation**: Does not validate transformed payloads against schemas


## Notes

* **Format Validation**: The policy validates that `downsteamPayloadFormat` and `upstreamPayloadFormat` are different. Configuring both as the same format will result in a validation error.

* **Empty Payloads**: Requests or responses with empty bodies are passed through without transformation.

* **Error Handling**: If transformation fails (for example, malformed JSON or XML), the policy returns an appropriate error response to the client.


## Related Policies

- **JSON to XML**: Use for one-way JSON to XML transformation when bidirectional mediation is not needed
- **XML to JSON**: Use for one-way XML to JSON transformation when bidirectional mediation is not needed
- **Request Rewrite**: Combine with payload transformation for complete request modification
- **Log Message**: Log payloads before and after transformation for debugging
