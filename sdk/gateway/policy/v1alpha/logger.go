package policyv1alpha

import "log/slog"

// EnsureLogger returns the provided logger if non-nil, otherwise returns slog.Default().
// Use this in GetPolicy to ensure the policy's logger field is never nil.
//
// Example:
//
//	func GetPolicy(metadata PolicyMetadata, params map[string]interface{}, logger *slog.Logger) (Policy, error) {
//	    p := &MyPolicy{
//	        logger: policy.EnsureLogger(logger),
//	    }
//	    return p, nil
//	}
func EnsureLogger(logger *slog.Logger) *slog.Logger {
	if logger == nil {
		return slog.Default()
	}
	return logger
}

// WithRequestID enriches a logger with a request ID for correlation.
// Use this in OnRequest and OnResponse methods to add request-specific context.
// If logger is nil, it uses slog.Default() to ensure safe operation.
//
// Example:
//
//	func (p *MyPolicy) OnRequest(ctx *RequestContext, params map[string]interface{}) RequestAction {
//	    log := policy.WithRequestID(p.logger, ctx.RequestID)
//	    log.Debug("Processing request", "path", ctx.Path)
//	    return RequestAction{}
//	}
func WithRequestID(logger *slog.Logger, requestID string) *slog.Logger {
	if logger == nil {
		logger = slog.Default()
	}
	return logger.With("requestId", requestID)
}
