# Refactoring Notes: PolicyExecutionContext Pattern

**Date**: 2025-11-21
**Type**: Architecture Improvement
**Impact**: Code quality, memory management, lifecycle clarity

## Summary

Refactored the ext_proc streaming handler to use a `PolicyExecutionContext` pattern that manages the complete request-response lifecycle as a local variable within the streaming loop, eliminating the need for global context storage.

## Motivation

**Before**: Request context was stored in a global `Kernel.ContextStorage` map during request phase and retrieved during response phase, requiring explicit storage, retrieval, and cleanup logic.

**After**: Request context lives within a `PolicyExecutionContext` instance that is created as a local variable in the `Process()` streaming loop and naturally goes out of scope when the stream iteration completes.

## Changes

### 1. New File: `execution_context.go`

Created `PolicyExecutionContext` struct that encapsulates:
- Request context data
- Policy chain reference
- Request ID
- Server component reference

Methods:
- `processRequestHeaders()` - Handles request header phase
- `processRequestBody()` - Handles request body phase
- `processResponseHeaders()` - Handles response header phase
- `processResponseBody()` - Handles response body phase
- `buildRequestContext()` - Converts Envoy headers to RequestContext
- `buildResponseContext()` - Builds ResponseContext from stored request data

### 2. Modified: `extproc.go`

**Process() function**:
```go
// Before: No execution context tracking
func (s *ExternalProcessorServer) Process(stream) {
    for {
        req := stream.Recv()
        resp := s.processRequest(req)  // Each phase handled independently
        stream.Send(resp)
    }
}

// After: Execution context lives for entire request-response cycle
func (s *ExternalProcessorServer) Process(stream) {
    var execCtx *PolicyExecutionContext  // Lives in loop scope
    for {
        req := stream.Recv()
        resp := s.handleProcessingPhase(req, &execCtx)
        stream.Send(resp)
    }
}
```

**New methods**:
- `initializeExecutionContext()` - Sets up execution context (extracts metadata, gets policy chain, generates request ID)
- `handleProcessingPhase()` - Routes to appropriate phase handler
- `skipAllProcessing()` - Returns response that skips all processing

**Removed methods**:
- `handleRequestHeaders()` - Moved to PolicyExecutionContext
- `handleRequestBody()` - Moved to PolicyExecutionContext
- `handleResponseHeaders()` - Moved to PolicyExecutionContext
- `handleResponseBody()` - Moved to PolicyExecutionContext
- `buildRequestContext()` - Moved to PolicyExecutionContext
- `buildResponseContext()` - Moved to PolicyExecutionContext
- `extractRequestID()` - No longer needed (context not stored by ID)

### 3. Modified: `mapper.go`

**Kernel struct**:
```go
// Before
type Kernel struct {
    Routes map[string]*PolicyChain
    ContextStorage map[string]*storedContext  // Global storage
}

// After
type Kernel struct {
    Routes map[string]*PolicyChain
    // ContextStorage removed - context now managed locally
}
```

### 4. Removed: `context_storage.go`

Deleted entire file containing:
- `storedContext` struct
- `storeContextForResponse()` method
- `getStoredContext()` method
- `removeStoredContext()` method

## Benefits

### 1. **Automatic Lifecycle Management**
- Context automatically cleaned up when loop iteration ends
- No manual cleanup logic needed
- No risk of memory leaks from forgotten cleanup

### 2. **Better Memory Management**
- Go's garbage collector handles cleanup
- No need for mutex-protected global map
- Reduced memory footprint (no storage overhead)

### 3. **Simpler Code Flow**
```
Before: initialize → store → retrieve → cleanup
After:  initialize → use → automatic cleanup
```

### 4. **Thread Safety**
- Each streaming connection has its own execution context
- No shared mutable state between requests
- Eliminates race condition risks from global storage

### 5. **Clearer Separation of Concerns**
- `ExternalProcessorServer`: Routes between phases
- `PolicyExecutionContext`: Manages policy execution and context
- `Kernel`: Maps routes to policy chains (only)

## Architecture Diagram

```
┌─────────────────────────────────────────────────────────────┐
│ ExternalProcessorServer.Process(stream)                     │
│                                                              │
│  var execCtx *PolicyExecutionContext  ← Local variable      │
│                                                              │
│  loop:                                                       │
│    ┌─────────────────────────────────────────┐             │
│    │ 1. Receive request headers              │             │
│    │ 2. initializeExecutionContext()         │             │
│    │    - Extract metadata key               │             │
│    │    - Get policy chain from Kernel       │             │
│    │    - Generate request ID                │             │
│    │    - Create PolicyExecutionContext      │             │
│    │ 3. execCtx.processRequestHeaders()      │             │
│    └─────────────────────────────────────────┘             │
│    ┌─────────────────────────────────────────┐             │
│    │ 4. Receive request body (if needed)     │             │
│    │ 5. execCtx.processRequestBody()         │             │
│    └─────────────────────────────────────────┘             │
│    ┌─────────────────────────────────────────┐             │
│    │ 6. Receive response headers             │             │
│    │ 7. execCtx.processResponseHeaders()     │             │
│    │    (uses stored requestContext)         │             │
│    └─────────────────────────────────────────┘             │
│    ┌─────────────────────────────────────────┐             │
│    │ 8. Receive response body (if needed)    │             │
│    │ 9. execCtx.processResponseBody()        │             │
│    └─────────────────────────────────────────┘             │
│                                                              │
│  execCtx goes out of scope → automatic cleanup              │
└─────────────────────────────────────────────────────────────┘
```

## Testing Impact

- No changes to external interfaces (ext_proc protocol unchanged)
- Existing integration tests should pass without modification
- Unit tests for removed storage methods can be deleted
- No new tests required (internal refactoring only)

## Migration Notes

No migration needed - this is an internal implementation change with no external API impact.

## Related Documentation Updates

- `data-model.md`: Added PolicyExecutionContext entity, removed ContextStorage references
- `plan.md`: Updated storage description, updated file structure
- `tasks.md`: Marked T052-T054 as removed, added T050a for PolicyExecutionContext

## References

- Implementation: `policy-engine/internal/kernel/execution_context.go`
- Updated files: `extproc.go`, `mapper.go`
- Removed files: `context_storage.go`
