# JSON to XML

## Overview

The JSON to XML policy provides the capability to transform request and response payloads from JSON format to XML format.
This policy operates on both the request flow (transforming client JSON requests before forwarding to upstream services) and the response flow (transforming JSON responses from upstream services before returning to clients).

## Features

- Transforms JSON request bodies to XML format before forwarding to upstream services
- Transforms JSON response bodies to XML format before returning to clients
- Automatically handles all JSON data types (objects, arrays, strings, numbers, booleans, null)
- Intelligent XML element naming with array singularization
- Proper XML declaration and formatting
- Content-Type header management (updates to `application/xml`)
- Content-Length header updates for transformed payloads
- Robust error handling with appropriate HTTP status codes

## Configuration

The JSON to XML policy supports configuration of transformation directions through two boolean parameters. This allows you to control whether the transformation applies to requests, responses, or both.

### User Parameters (API Definition)

These parameters are configured per-API/route by the API developer:

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `onRequestFlow` | boolean | No | `false` | Enables JSON to XML transformation for incoming request payloads (client to upstream). When set to `true`, JSON request bodies will be converted to XML format before forwarding to upstream services. When set to `false`, request bodies will be passed through unchanged. |
| `onResponseFlow` | boolean | No | `false` | Enables JSON to XML transformation for outgoing response payloads (upstream to client). When set to `true`, JSON response bodies will be converted to XML format before returning to clients. When set to `false`, response bodies will be passed through unchanged. |

### System Parameters

This policy does not require any system-level configuration parameters.

## API Definition Examples

### Example 1: Basic JSON to XML Transformation

Apply JSON to XML transformation to both requests and responses (requires explicit configuration):

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
      url: http://legacy-backend:8080
  policies:
    - name: json-to-xml
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

Apply JSON to XML transformation only to incoming requests (client to upstream):

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
      url: http://xml-service:9000
  policies:
    - name: json-to-xml
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

Apply JSON to XML transformation only to outgoing responses (upstream to client):

```yaml
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: RestApi
metadata:
  name: json-backend-api-v1.0
spec:
  displayName: JSON-Backend-API
  version: v1.0
  context: /json-backend/$version
  upstream:
    main:
      url: http://json-service:8080
  policies:
    - name: json-to-xml
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

### Example 4: Operation-Specific Direction Control

Apply different transformation directions to different operations:

```yaml
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: RestApi
metadata:
  name: mixed-api-v1.0
spec:
  displayName: Mixed-API
  version: v1.0
  context: /mixed/$version
  upstream:
    main:
      url: http://backend-service:8080
  operations:
    - method: POST
      path: /xml-required
      policies:
        - name: json-to-xml
          version: v0.1.0
          params:
            onRequestFlow: true
            onResponseFlow: false
    - method: GET
      path: /xml-response
      policies:
        - name: json-to-xml
          version: v0.1.0
          params:
            onRequestFlow: false
            onResponseFlow: true
    - method: PUT
      path: /full-transform
      policies:
        - name: json-to-xml
          version: v0.1.0
          params:
            onRequestFlow: true
            onResponseFlow: true
```

### Example 6: Default Behavior

When no parameters are specified, no transformations are performed by default:

```yaml
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: RestApi
metadata:
  name: default-transform-api-v1.0
spec:
  displayName: Default-Transform-API
  version: v1.0
  context: /default-transform/$version
  upstream:
    main:
      url: http://backend-service:8080
  policies:
    - name: json-to-xml
      version: v0.1.0
      # No params specified - defaults to onRequestFlow: false, onResponseFlow: false
      # This policy will be effectively disabled unless explicitly configured
  operations:
    - method: GET
      path: /data
    - method: POST
      path: /submit
```

## Request Transformation Examples

### Basic JSON Object Transformation

**Original client request:**
```http
POST /users/v1.0/profile HTTP/1.1
Host: api-gateway.company.com
Content-Type: application/json

{
  "name": "John Doe",
  "age": 30,
  "email": "john@example.com"
}
```

**Resulting upstream request (Example 1):**
```
POST /profile HTTP/1.1
Host: legacy-backend:8080
Content-Type: application/xml
Content-Length: 135

<root>
  <name>John Doe</name>
  <age>30</age>
  <email>john@example.com</email>
</root>
```

### Complex JSON with Arrays Transformation

**Original client request:**
```http
POST /users/v1.0/profile HTTP/1.1
Host: api-gateway.company.com
Content-Type: application/json

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

**Resulting upstream request:**
```http
POST /profile HTTP/1.1
Host: legacy-backend:8080
Content-Type: application/xml
Content-Length: 298

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

- **`onRequestFlow: true`**: Transforms JSON request bodies to XML before forwarding to upstream services. When `false`, request bodies are left unchanged.
- **`onResponseFlow: true`**: Transforms JSON response bodies to XML before returning to clients. When `false`, response bodies are left unchanged.
- **Both `false` (default)**: No transformation is performed in either direction (policy effectively disabled by default).
- **Both `true`**: Transforms both JSON request bodies (to upstream) and JSON response bodies (to client).

### Content-Type Detection

The policy only processes requests and responses that have the `Content-Type` header set to `application/json` (case-insensitive). Other content types are passed through unchanged.

### JSON to XML Conversion Rules

1. **JSON Objects**: Converted to XML elements with child elements for each key-value pair
2. **JSON Arrays**: Each array item becomes a separate XML element using the array key name (no singularization):
   - Array `"books": [...]` becomes multiple `<books>...</books>` elements
   - Array `"items": [...]` becomes multiple `<items>...</items>` elements
3. **JSON Primitives**:
   - Strings: Converted to XML text content
   - Numbers: Formatted appropriately (integers without decimals, floats with necessary precision)
   - Booleans: Converted to `"true"` or `"false"` strings
   - Null: Converted to empty XML element content

### XML Output Format

- **No XML Declaration**: The output does not include `<?xml version="1.0" encoding="UTF-8"?>` declaration
- **Root Element**: All JSON data is wrapped in a `<root>` element
- **Indented Format**: XML is formatted with 2-space indentation for readability

### Header Management

- **Request Flow**: Updates `Content-Type` to `application/xml` and `Content-Length` to reflect the new XML payload size
- **Response Flow**: Updates `Content-Type` to `application/xml` and `Content-Length` to reflect the new XML payload size

### Error Handling

#### Request Flow Errors (Returns 400 Bad Request)

1. **Invalid Content-Type**: If the request `Content-Type` is not `application/json`
   ```json
   {
     "error": "Bad Request",
     "message": "Content-Type must be application/json for JSON to XML transformation"
   }
   ```

2. **Invalid JSON**: If the request body contains malformed JSON
   ```json
   {
     "error": "Bad Request", 
     "message": "Invalid JSON format in request body"
   }
   ```

3. **Conversion Failure**: If JSON to XML conversion fails
   ```json
   {
     "error": "Bad Request",
     "message": "Failed to convert JSON to XML format"  
   }
   ```

#### Response Flow Errors (Silent Handling)

For response transformations, errors are handled silently to avoid breaking the response chain:
- Invalid response content type → Response passed through unchanged
- Invalid JSON in response → Response passed through unchanged  
- Conversion failures → Response passed through unchanged

### Empty or Missing Bodies

- **Empty Request Body**: Request passed through unchanged
- **Missing Request Body**: Request passed through unchanged
- **Empty Response Body**: Response passed through unchanged
- **Missing Response Body**: Response passed through unchanged

## Common Use Cases

1. **Legacy System Integration**: Transform modern JSON APIs to work with legacy XML-based backend systems

2. **Protocol Bridging**: Enable JSON clients to interact with XML-only web services

3. **Data Format Migration**: Gradually migrate from XML to JSON while maintaining backward compatibility

4. **Third-Party Integration**: Integrate with external services that only accept XML format

5. **Enterprise Service Bus**: Convert JSON messages to XML for enterprise message routing

6. **SOAP Service Integration**: Transform REST JSON requests to XML format for SOAP service consumption

7. **XML Database Integration**: Convert JSON data to XML format for XML databases or storage systems

8. **Compliance Requirements**: Meet regulatory or industry standards that require XML data format

## Best Practices

1. **Content-Type Validation**: Ensure client applications send proper `Content-Type: application/json` headers

2. **Error Handling**: Implement proper error handling on the client side for 400 Bad Request responses

3. **Performance Considerations**: Be aware that JSON to XML conversion adds processing overhead

4. **Payload Size**: Monitor payload sizes as XML typically has larger overhead than JSON

5. **Testing**: Thoroughly test with various JSON structures including nested objects and arrays

6. **Documentation**: Document the XML schema expectations for upstream services

7. **Monitoring**: Monitor conversion success rates and error patterns

## Security Considerations

1. **Payload Validation**: Ensure upstream services validate the converted XML payloads

2. **Size Limits**: Implement appropriate payload size limits to prevent excessive resource usage

3. **XML Injection**: Be aware that XML format may be susceptible to XML injection attacks at the upstream service

4. **Error Information**: Error messages are returned to clients - ensure they don't expose sensitive information

5. **Content Validation**: Validate that JSON content is appropriate before transformation

6. **Logging**: Consider logging transformation activities for audit purposes (excluding sensitive data)

## Limitations

1. **Single Use**: This policy cannot be applied multiple times to the same resource since the payload becomes XML after the first transformation

2. **JSON Only**: Only processes payloads with `Content-Type: application/json` - other formats are ignored

3. **No Configuration**: The transformation behavior cannot be customized (e.g., custom root element names)

4. **Memory Usage**: Large JSON payloads require buffering in memory for transformation

5. **Processing Order**: Must be applied before any policies that expect JSON format

## Performance Considerations

- **Memory Buffering**: Both request and response bodies are buffered in memory during transformation
- **Processing Overhead**: JSON parsing and XML generation add latency to requests
- **Payload Size**: XML output is typically larger than equivalent JSON input
- **CPU Usage**: Recursive processing of nested JSON structures uses CPU resources

## Troubleshooting

### Common Issues

1. **400 Bad Request - Content-Type**: Ensure client sends `Content-Type: application/json`
2. **400 Bad Request - Invalid JSON**: Validate JSON format before sending requests  
3. **Transformation Ignored**: Check that the payload has the correct content type
4. **Large Payloads**: Consider payload size limits and memory constraints
5. **Performance Issues**: Monitor transformation time for large or complex JSON structures
