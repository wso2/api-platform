package policyv1alpha

// UpstreamResponseModifications is a backward-compatible alias for DownstreamResponseModifications.
//
// The type was renamed in SDK v0.4.3 to better reflect that mutations are applied
// to the response sent downstream to the client (not a response going upstream).
// Policies written against SDK ≤ v0.4.1 that return UpstreamResponseModifications
// will continue to compile and behave identically without any code changes.
//
// Deprecated: Use DownstreamResponseModifications instead.
type UpstreamResponseModifications = DownstreamResponseModifications
