# XML to JSON

## Overview

The XML to JSON policy provides the capability to transform request and response payloads from XML format to JSON format.
This policy operates on both the request flow (transforming client XML requests before forwarding to upstream services) and the response flow (transforming XML responses from upstream services before returning to clients).

## Features

- Transforms XML request bodies to JSON format before forwarding to upstream services
- Transforms XML response bodies to JSON format before returning to clients
- Automatically handles all XML structures (elements, attributes, arrays, text content)
- Preserves XML attributes with @ prefix notation in JSON
- Intelligent type conversion for element content (strings, numbers, booleans)
- Proper Content-Type header management (updates to `application/json`)
- Content-Length header updates for transformed payloads
- Robust error handling with 500 Internal Server Error status codes

## Configuration

The XML to JSON policy supports configuration of transformation directions through two boolean parameters. This allows you to control whether the transformation applies to requests, responses, or both.

### User Parameters (API Definition)

These parameters are configured per-API/route by the API developer:

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `onRequestFlow` | boolean | No | `false` | Enables XML to JSON transformation for incoming request payloads (client to upstream). When set to `true`, XML request bodies will be converted to JSON format before forwarding to upstream services. When set to `false`, request bodies will be passed through unchanged. |
| `onResponseFlow` | boolean | No | `false` | Enables XML to JSON transformation for outgoing response payloads (upstream to client). When set to `true`, XML response bodies will be converted to JSON format before returning to clients. When set to `false`, response bodies will be passed through unchanged. |

### System Parameters

This policy does not require any system-level configuration parameters.

## API Definition Examples

### Example 1: Basic XML to JSON Transformation

Apply XML to JSON transformation to both requests and responses (requires explicit configuration):

```yaml
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: RestApi
metadata:
  name: user-api-v1.0
spec:
  displayName: User-API
  version: v1.0
  context: /users/$version
  upstream:
    main:
      url: http://json-backend:8080
  policies:
    - name: xml-to-json
      version: v0.1.0
      params:
        onRequestFlow: true
        onResponseFlow: true
  operations:
    - method: GET
      path: /profile
    - method: POST
      path: /profile
    - method: PUT
      path: /settings
```

### Example 2: Request-Only Transformation

Apply XML to JSON transformation only to incoming requests (client to upstream):

```yaml
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: RestApi
metadata:
  name: integration-api-v1.0
spec:
  displayName: Integration-API
  version: v1.0
  context: /integration/$version
  upstream:
    main:
      url: http://json-service:9000
  policies:
    - name: xml-to-json
      version: v0.1.0
      params:
        onRequestFlow: true
        onResponseFlow: false
  operations:
    - method: POST
      path: /legacy-endpoint
    - method: PUT
      path: /xml-data
```

### Example 3: Response-Only Transformation

Apply XML to JSON transformation only to outgoing responses (upstream to client):

```yaml
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: RestApi
metadata:
  name: legacy-api-v1.0
spec:
  displayName: Legacy-API
  version: v1.0
  context: /legacy/$version
  upstream:
    main:
      url: http://legacy-xml-service:8080
  policies:
    - name: xml-to-json
      version: v0.1.0
      params:
        onRequestFlow: false
        onResponseFlow: true
  operations:
    - method: GET
      path: /data
    - method: GET
      path: /reports
```

## Transformation Examples

The XML to JSON policy handles various XML structures and converts them to appropriate JSON representations. Below are examples of how different XML structures are transformed.

### Simple XML Object

**Input XML:**
```xml
<person>
  <name>John Doe</name>
  <age>30</age>
  <active>true</active>
</person>
```

**Output JSON:**
```json
{
  "person": {
    "name": "John Doe",
    "age": 30,
    "active": true
  }
}
```

### XML with Attributes

**Input XML:**
```xml
<book id="123" isbn="978-0123456789">
  <title>Go Programming</title>
  <author>John Doe</author>
  <price>29.99</price>
</book>
```

**Output JSON:**
```json
{
  "book": {
    "@id": "123",
    "@isbn": "978-0123456789",
    "title": "Go Programming",
    "author": "John Doe",
    "price": 29.99
  }
}
```

### XML Arrays (Repeated Elements)

**Input XML:**
```xml
<users>
  <user>
    <id>1</id>
    <name>Alice</name>
  </user>
  <user>
    <id>2</id>
    <name>Bob</name>
  </user>
</users>
```

**Output JSON:**
```json
{
  "users": {
    "user": [
      {
        "id": 1,
        "name": "Alice"
      },
      {
        "id": 2,
        "name": "Bob"
      }
    ]
  }
}
```

### Empty Elements

**Input XML:**
```xml
<data>
  <empty></empty>
  <selfclosed/>
  <nonempty>value</nonempty>
</data>
```

**Output JSON:**
```json
{
  "data": {
    "empty": null,
    "selfclosed": null,
    "nonempty": "value"
  }
}
```

### Mixed Content and Text-Only Elements

**Input XML:**
```xml
<message>Hello World</message>
```

**Output JSON:**
```json
{
  "message": "Hello World"
}
```

### Complex XML with Mixed Attributes and Elements

**Input XML:**
```xml
<order id="12345" status="processing">
  <customer type="premium">
    <name>Jane Smith</name>
    <email>jane@example.com</email>
  </customer>
  <items>
    <item>
      <sku>BOOK-001</sku>
      <quantity>2</quantity>
      <price>29.99</price>
    </item>
    <item>
      <sku>PEN-002</sku>
      <quantity>5</quantity>
      <price>1.99</price>
    </item>
  </items>
  <total>69.93</total>
</order>
```

**Output JSON:**
```json
{
  "order": {
    "@id": "12345",
    "@status": "processing",
    "customer": {
      "@type": "premium",
      "name": "Jane Smith",
      "email": "jane@example.com"
    },
    "items": {
      "item": [
        {
          "sku": "BOOK-001",
          "quantity": 2,
          "price": 29.99
        },
        {
          "sku": "PEN-002",
          "quantity": 5,
          "price": 1.99
        }
      ]
    },
    "total": 69.93
  }
}
```

## Request Transformation Examples

### Basic JSON Object Transformation

**Original client request:**
```http
POST /users/v1.0/profile HTTP/1.1
Host: api-gateway.company.com
Content-Type: application/xml

<root>
  <name>John Doe</name>
  <age>30</age>
  <email>john@example.com</email>
</root>
```

**Resulting upstream request (Example 1):**
```
POST /profile HTTP/1.1
Host: legacy-backend:8080
Content-Type: application/json
Content-Length: 135

{
  "name": "John Doe",
  "age": 30,
  "email": "john@example.com"
}
```

### Complex JSON with Arrays Transformation

**Original client request:**
```http
POST /users/v1.0/profile HTTP/1.1
Host: api-gateway.company.com
Content-Type: application/xml

<root>
  <user>
    <name>Jane Smith</name>
    <skills>Java</skills>
    <skills>Python</skills>
    <skills>Go</skills>
    <address>
      <city>New York</city>
      <zipcode>10001</zipcode>
    </address>
  </user>
  <active>true</active>
</root>
```

**Resulting upstream request:**
```http
POST /profile HTTP/1.1
Host: legacy-backend:8080
Content-Type: application/json
Content-Length: 298

{
  "user": {
    "name": "Jane Smith",
    "skills": ["Java", "Python", "Go"],
    "address": {
      "city": "New York",
      "zipcode": "10001"
    }
  },
  "active": true
}
```

## Response Transformation Examples

### JSON Response to XML

**Original upstream response:**
```http
HTTP/1.1 200 OK
Content-Type: application/json
Content-Length: 78

{
  "status": "success",
  "data": {
    "id": 12345,
    "created": true
  }
}
```

**Resulting client response:**
```http
HTTP/1.1 200 OK
Content-Type: application/xml
Content-Length: 156

<root>
  <status>success</status>
  <data>
    <id>12345</id>
    <created>true</created>
  </data>
</root>
```

## Policy Behavior

### Direction Control

The policy behavior is controlled by two boolean parameters:

- **`onRequestFlow: true`**: Transforms XML request bodies to JSON before forwarding to upstream services. When `false`, request bodies are left unchanged.
- **`onResponseFlow: true`**: Transforms XML response bodies to JSON before returning to clients. When `false`, response bodies are left unchanged.
- **Both `false` (default)**: No transformation is performed in either direction (policy effectively disabled by default).
- **Both `true`**: Transforms both XML request bodies (to upstream) and XML response bodies (to client).

### Content-Type Detection

The policy only processes requests and responses that have the `Content-Type` header set to `application/xml` or `text/xml` (case-insensitive). Other content types will result in a 500 Internal Server Error.

### XML to JSON Conversion Rules

1. **XML Elements**: Converted to JSON objects with properties for each child element
2. **XML Attributes**: Converted to JSON properties with `@` prefix (e.g., `id="123"` becomes `"@id": "123"`)
3. **Repeated Elements**: Multiple elements with the same name become JSON arrays
4. **Text Content**: Element text content becomes the property value
5. **Empty Elements**: Converted to `null` values
6. **Type Conversion**:
   - Text containing "true" or "false" becomes boolean values
   - Numeric text containing decimals becomes float values
   - Simple numeric text in element content becomes integer values
   - Attribute values are preserved as strings to avoid converting IDs or codes

### JSON Output Format

- **Preserves Structure**: Maintains the hierarchical structure of the original XML
- **Attribute Handling**: XML attributes are converted to JSON properties with `@` prefix
- **Array Detection**: Repeated XML elements with the same name are automatically converted to JSON arrays
- **Indented Format**: JSON is formatted with 2-space indentation for readability

### Header Management

- **Request Flow**: Updates `Content-Type` to `application/json` and `Content-Length` to reflect the new JSON payload size
- **Response Flow**: Updates `Content-Type` to `application/json` and `Content-Length` to reflect the new JSON payload size

### Error Handling

#### Request Flow Errors (Returns 500 Internal Server Error)

1. **Invalid Content-Type**: If the request `Content-Type` is not `application/xml` or `text/xml`
   ```json
   {
     "error": "Internal Server Error",
     "message": "Content-Type must be application/xml or text/xml for XML to JSON transformation"
   }
   ```

2. **Invalid XML**: If the request body contains malformed XML
   ```json
   {
     "error": "Internal Server Error", 
     "message": "Failed to convert XML to JSON format: failed to parse XML: ..."
   }
   ```

#### Response Flow Errors (Returns 500 Internal Server Error)

1. **Invalid Content-Type**: If the response `Content-Type` is not `application/xml` or `text/xml`
2. **Invalid XML**: If the response body contains malformed XML
3. **Conversion Failure**: If XML to JSON conversion fails

Note: Response errors return 500 status codes with error details in the response body, similar to request flow errors.

### Empty or Missing Bodies

- **Empty Request Body**: Request passed through unchanged
- **Missing Request Body**: Request passed through unchanged
- **Empty Response Body**: Response passed through unchanged
- **Missing Response Body**: Response passed through unchanged

## Common Use Cases

1. **Legacy System Integration**: Transform XML-based legacy systems to work with modern JSON APIs

2. **Protocol Bridging**: Enable XML clients to interact with JSON-only web services

3. **Data Format Migration**: Gradually migrate from XML to JSON while maintaining backward compatibility

4. **Third-Party Integration**: Integrate with external services that only provide XML responses

5. **Enterprise Service Bus**: Convert XML messages to JSON for modern message routing

6. **REST API Integration**: Transform SOAP/XML responses to JSON format for REST API consumption

7. **JSON Database Integration**: Convert XML data to JSON format for JSON databases or storage systems

8. **Compliance Requirements**: Meet regulatory or industry standards that require JSON data format

## Best Practices

1. **Content-Type Validation**: Ensure client applications send proper `Content-Type: application/xml` headers

2. **Error Handling**: Implement proper error handling on the client side for 500 Internal Server Error responses

3. **Performance Considerations**: Be aware that XML to JSON conversion adds processing overhead

4. **Payload Size**: Monitor payload sizes as JSON typically has smaller overhead than XML

5. **Testing**: Thoroughly test with various XML structures including nested elements, attributes and arrays

6. **Documentation**: Document the JSON schema expectations for upstream services

7. **Monitoring**: Monitor conversion success rates and error patterns

## Security Considerations

1. **Payload Validation**: Ensure upstream services validate the converted JSON payloads

2. **Size Limits**: Implement appropriate payload size limits to prevent excessive resource usage

3. **XML Injection**: Be aware that malformed XML input may cause parsing errors

4. **Error Information**: Error messages are returned to clients - ensure they don't expose sensitive information

5. **Content Validation**: Validate that XML content is appropriate before transformation

6. **Logging**: Consider logging transformation activities for audit purposes (excluding sensitive data)

## Limitations

1. **Single Use**: This policy cannot be applied multiple times to the same resource since the payload becomes JSON after the first transformation

2. **XML Only**: Only processes payloads with `Content-Type: application/xml` or `text/xml` - other formats result in errors

3. **No Configuration**: The transformation behavior cannot be customized (e.g., custom attribute prefix)

4. **Memory Usage**: Large XML payloads require buffering in memory for transformation

5. **Processing Order**: Must be applied before any policies that expect XML format

## Performance Considerations

- **Memory Buffering**: Both request and response bodies are buffered in memory during transformation
- **Processing Overhead**: XML parsing and JSON generation add latency to requests
- **Payload Size**: JSON output is typically smaller than equivalent XML input
- **CPU Usage**: Recursive processing of nested XML structures uses CPU resources

## Troubleshooting

### Common Issues

1. **500 Internal Server Error - Content-Type**: Ensure client sends `Content-Type: application/xml` or `text/xml`
2. **500 Internal Server Error - Invalid XML**: Validate XML format before sending requests  
3. **Transformation Ignored**: Check that the payload has the correct content type
4. **Large Payloads**: Consider payload size limits and memory constraints
5. **Performance Issues**: Monitor transformation time for large or complex XML structures
