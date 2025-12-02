# Envoy ext_proc gRPC Contract

**Feature**: 001-policy-engine | **Date**: 2025-11-18

## Overview

The Policy Engine implements Envoy's External Processor (ext_proc) gRPC service interface for HTTP request/response processing.

## Service Definition

The implementation uses Envoy's official protobuf definitions from `envoyproxy/go-control-plane`:

```
import "envoy/service/ext_proc/v3/external_processor.proto"
```

## Service Interface

```go
service ExternalProcessor {
  // Bidirectional streaming RPC
  // Envoy streams processing requests → Policy Engine streams processing responses
  rpc Process(stream ProcessingRequest) returns (stream ProcessingResponse);
}
```

## Message Flow

### 1. Request Headers Phase
```
Envoy → Policy Engine: ProcessingRequest{request_headers}
Policy Engine executes RequestPolicies
Policy Engine → Envoy: ProcessingResponse{
  request_headers: HeadersResponse{...},
  mode_override: ProcessingMode{request_body_mode, response_body_mode}
}
```

### 2. Request Body Phase (if BUFFERED mode)
```
Envoy → Policy Engine: ProcessingRequest{request_body}
Policy Engine applies body modifications
Policy Engine → Envoy: ProcessingResponse{request_body: BodyResponse{...}}
```

### 3. Response Headers Phase
```
Envoy → Policy Engine: ProcessingRequest{response_headers}
Policy Engine executes ResponsePolicies
Policy Engine → Envoy: ProcessingResponse{response_headers: HeadersResponse{...}}
```

### 4. Response Body Phase (if BUFFERED mode)
```
Envoy → Policy Engine: ProcessingRequest{response_body}
Policy Engine applies body modifications
Policy Engine → Envoy: ProcessingResponse{response_body: BodyResponse{...}}
```

## Key Configurations

### ProcessingMode
```go
message ProcessingMode {
  ProcessingMode.BodySendMode request_body_mode = SKIP | BUFFERED;
  ProcessingMode.BodySendMode response_body_mode = SKIP | BUFFERED;
}
```

- **SKIP**: Don't send body to ext_proc (headers only, minimal latency)
- **BUFFERED**: Buffer entire body before sending (required for body modification)

### ImmediateResponse (Short-Circuit)
```go
message ImmediateResponse {
  HttpStatus status = {...};
  HeaderMutation headers = {...};
  string body = "...";
  GrpcStatus grpc_status = {...};
}
```

Used when RequestPolicy returns `ImmediateResponse` action.

## Implementation Location

`src/kernel/extproc.go` implements the `ExternalProcessorServer` interface.

## Contract Tests

Located in `tests/contract/extproc_test.go`

## References

- Envoy ext_proc documentation: https://www.envoyproxy.io/docs/envoy/v1.36.2/configuration/http/http_filters/ext_proc_filter
- Protobuf definitions: https://github.com/envoyproxy/envoy/blob/v1.36.2/api/envoy/service/ext_proc/v3/external_processor.proto
