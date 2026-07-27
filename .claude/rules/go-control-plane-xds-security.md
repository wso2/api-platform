# Rule: Go xDS / Envoy Control-Plane Security Standards

## Context & Scope

Apply whenever writing, refactoring, or reviewing Go (`.go`) code in `gateway-controller` or `gateway-runtime` that constructs/serves an xDS gRPC server speaking to Envoy or the policy-engine runtime (`pkg/xds`, `pkg/policyxds`), generates Envoy bootstrap/listener/HCM configuration (`pkg/xds/translator.go`), or configures the Envoy admin interface (`gateway-runtime/router/config/envoy-bootstrap.yaml`).

xDS is itself a security control plane, not just config distribution: a spoofed or unauthenticated xDS channel can push arbitrary listener/cluster/route config to every data-plane instance — including routes that bypass authn/authz or upstreams reaching internal-only services. An unauthenticated Envoy admin interface allows live traffic reconfiguration and full runtime/config disclosure, including TLS private keys when embedded inline (`inline_bytes`) rather than via SDS — Envoy's default secret redaction doesn't cover inline key bytes outside the SDS path. Treat every directive here at the same severity tier as `authentication_authorization.md`.

## Directives

1. **TLS + mTLS mandatory for every xDS gRPC server, never config-optional in production.** Every xDS/policy-xDS server (`:18000`/`:18001`-style) must default to `Enabled: true` with `tls.RequireAndVerifyClientCert`, not TLS-disabled-by-default. If TLS is disabled and the listener binds anywhere other than loopback, refuse to start (fatal error, per GO-AUTH-011's fail-closed pattern) — plaintext is acceptable only when controller and runtime are strictly co-located on loopback.
2. **Authenticate AND authorize every xDS stream — mTLS alone is not authorization.** In `OnStreamOpen`/`OnStreamRequest`, extract the verified peer identity (cert SAN or SPIFFE ID) and reject any stream not on an explicit allowlist for the expected runtime/policy-engine node. A callback that only logs the peer with no accept/reject decision is equivalent to no authorization — any client completing the handshake (including a leaked/over-broad cert) gets the full snapshot, which routinely contains API-key hashes, subscription data, and full policy chains for every tenant.
3. **Never embed private key material inline in an xDS resource.** Serve the downstream listener's cert/key via a `TlsCertificateSdsSecretConfig` reference — never `inline_bytes` in the LDS `Listener` resource. An inline key means a single successful LDS request (from any peer clearing directives 1-2) exfiltrates the private key directly.
4. **Harden the Envoy admin interface independently of the xDS channel.** Bind the admin listener (`:9901`-style) to `127.0.0.1` only, never `0.0.0.0` — it exposes TLS keys, full runtime config, and `/runtime_modify`. Point `flags_path` at a read-only mount so `/runtime_modify` is inert even if reached via another path (compromised sidecar, debug shell). Ship a default-deny NetworkPolicy for the admin port and never provision a `Service`/`Ingress`/`NodePort` exposing it.
5. **Resource-limit every xDS gRPC server.** Set `grpc.MaxRecvMsgSize`, `grpc.MaxSendMsgSize`, and `grpc.MaxConcurrentStreams` explicitly on every construction (see `go-network-service-hardening.md` directive 2) — unbounded defaults let one client exhaust memory or the stream-slot budget other clients depend on.
6. **Canonicalize request paths before any route/policy/authz match downstream.** Every generated `HttpConnectionManager` must set `NormalizePath: true`, `MergeSlashes: true`, and an explicit `PathWithEscapedSlashesAction` (`UNESCAPE_AND_REDIRECT` or `REJECT_REQUEST`). Unset, Envoy matches routes/policy/authz against an un-normalized path (`//`, `/./`, `/../`, `%2F`), desynchronizing the selected route from what the operator's policy was written against — the same bypass class as GO-AUTH-004, applied to generated Envoy config. Apply identically across the main listener, any WebSub-internal listener, and any dynamic-forward-proxy HCM; test that a new listener type can't silently omit it.

## Example

```go
// BAD: logs the peer but never makes an accept/reject decision — no authorization at all.
func (cb *serverCallbacks) OnStreamOpen(ctx context.Context, id int64, typ string) error {
    log.Infof("xDS stream opened: id=%d type=%s", id, typ)
    return nil
}

// GOOD: verified peer identity checked against an explicit allowlist; anything
// else is rejected. allowPlaintextLoopback is only ever true when the server
// was constructed with the validated loopback-plaintext exception (directive 1).
var allowedNodeIdentities = map[string]bool{
    "spiffe://cluster.local/ns/gw-perf/sa/gateway-runtime": true,
}

func (cb *serverCallbacks) OnStreamOpen(ctx context.Context, id int64, typ string) error {
    p, ok := peer.FromContext(ctx)
    if !ok {
        return status.Error(codes.Unauthenticated, "no peer information")
    }
    tlsInfo, isTLS := p.AuthInfo.(credentials.TLSInfo)
    if !isTLS || len(tlsInfo.State.PeerCertificates) == 0 {
        if cb.allowPlaintextLoopback {
            return nil
        }
        return status.Error(codes.Unauthenticated, "no client certificate presented")
    }
    identity := extractSPIFFEID(tlsInfo.State.PeerCertificates[0])
    if !allowedNodeIdentities[identity] {
        logger.Warn("xDS stream rejected: unrecognized node identity", "identity", identity)
        return status.Error(codes.PermissionDenied, "node identity not authorized for this snapshot")
    }
    return nil
}
```

```yaml
# GOOD: Envoy admin bound to loopback only; runtime_modify disabled via a
# read-only flags_path; no Service/Ingress/NodePort ever exposes this port.
admin:
  address:
    socket_address: { address: 127.0.0.1, port_value: 9901 }
layered_runtime:
  layers:
    - name: static_layer
      static_layer: {}
    - name: disk
      disk_layer:
        symlink_root: /etc/envoy/runtime  # Read-only mount — no /runtime_modify effect
```

> **Verification Checklist before outputting code:**
> * Does an xDS/policy-xDS gRPC server default to TLS-disabled, or allow a plaintext bind to a non-loopback address without refusing to start?
> * Does a stream callback only log the peer without an accept/reject decision against an identity allowlist?
> * Is any private key composed as `inline_bytes` inside an xDS resource, instead of an SDS secret reference?
> * Does the Envoy admin interface bind `0.0.0.0`, or is `/runtime_modify` reachable without a read-only `flags_path`/NetworkPolicy?
> * Does an xDS gRPC server construction omit `MaxRecvMsgSize`/`MaxSendMsgSize`/`MaxConcurrentStreams`?
> * Does the xDS translator generate an HCM without `NormalizePath`/`MergeSlashes`/`PathWithEscapedSlashesAction` set, on every listener type?
