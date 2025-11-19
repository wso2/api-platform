# SetHeader Policy v1.0.0

## Overview

The SetHeader policy allows you to manipulate HTTP headers on both requests and responses. It supports three operations:

- **SET**: Replace or set a header value
- **APPEND**: Append a value to an existing header
- **DELETE**: Remove a header

## Configuration

### Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `action` | string | Yes | Action to perform: `SET`, `APPEND`, or `DELETE` |
| `headerName` | string | Yes | Name of the header to modify (case-insensitive) |
| `headerValue` | string | Conditional | Value to set or append (required for SET/APPEND, ignored for DELETE) |

### Examples

#### Set a Custom Header

```yaml
name: setHeader
version: v1.0.0
enabled: true
parameters:
  action: SET
  headerName: X-Custom-Header
  headerValue: my-value
```

#### Append to Existing Header

```yaml
name: setHeader
version: v1.0.0
enabled: true
parameters:
  action: APPEND
  headerName: X-Forwarded-For
  headerValue: 192.168.1.1
```

#### Delete a Header

```yaml
name: setHeader
version: v1.0.0
enabled: true
parameters:
  action: DELETE
  headerName: X-Unwanted-Header
```

## Use Cases

### Request Phase
- Add authentication headers
- Set routing headers
- Remove sensitive headers before forwarding
- Add tracing headers

### Response Phase
- Add security headers (CSP, HSTS, etc.)
- Remove server identification headers
- Add custom response headers
- Modify cache control headers

## Performance

- **Latency Impact**: < 0.1ms per header operation
- **Body Access**: None (header-only policy)
- **Memory**: Negligible

## Compatibility

- **Execution Phases**: Request and Response
- **Body Processing**: Not required
- **Envoy Version**: Compatible with Envoy 1.36.2+
