package policyv1alpha

import "log/slog"

// WithRequestID enriches a logger with a request ID for correlation.
// Use this in OnRequest and OnResponse methods to add request-specific context.
//
// Example:
//
//	func (p *MyPolicy) OnRequest(ctx *RequestContext, params map[string]interface{}) RequestAction {
//	    log := policy.WithRequestID(p.logger, ctx.RequestID)
//	    log.Debug("Processing request", "path", ctx.Path)
//	    return RequestAction{}
//	}
func WithRequestID(logger *slog.Logger, requestID string) *slog.Logger {
	return logger.With("requestId", requestID)
}
