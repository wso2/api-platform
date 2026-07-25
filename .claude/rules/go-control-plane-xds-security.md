# Rule: Go xDS / Envoy Control-Plane Security Standards

## Context & Scope

Apply this rule whenever writing, refactoring, or reviewing Go (`.go`) code in `gateway-controller` or `gateway-runtime` that constructs or serves an xDS gRPC server speaking to Envoy or to the policy-engine runtime (`pkg/xds`, `pkg/policyxds`), generates Envoy bootstrap/listener/HTTP-connection-manager configuration (`pkg/xds/translator.go`), or configures the Envoy admin interface (`gateway-runtime/router/config/envoy-bootstrap.yaml`).

xDS is itself a security control plane, not just a config-distribution mechanism: a spoofed or unauthenticated xDS channel can push arbitrary listener/cluster/route configuration to every data-plane instance it serves — including routes that bypass authn/authz, or upstreams that reach internal-only services. An unauthenticated Envoy admin interface allows live traffic reconfiguration and full runtime/config disclosure — including TLS private-key material when a certificate is embedded inline (`inline_bytes`) rather than served via SDS (see directive 3); Envoy redacts SDS-sourced secret material by default, but that redaction does not cover inline key bytes outside the Secret/SDS path. Treat every directive below as being at the same severity tier as `authentication_authorization.md`, because a compromise here compromises everything that rule protects downstream.

---

## Directives

### 1. TLS + Mutual TLS Are Mandatory for Every xDS gRPC Server — Never Config-Optional in Production

* **Default `Enabled: true` for TLS:** Every xDS/policy-xDS server (`:18000`, `:18001`-style ports) must default to TLS enabled with a required client CA (`tls.RequireAndVerifyClientCert`), not TLS-disabled-by-default with an opt-in flag.
* **Refuse to Start on an Insecure Bind:** If TLS is disabled and the listener is bound to anything other than `127.0.0.1`/a loopback address, refuse to start (fatal error), per the fail-closed startup pattern in `authentication_authorization.md` GO-AUTH-011. A plaintext xDS server is acceptable only when both the controller and the runtime it serves are strictly co-located on loopback.

### 2. Authenticate AND Authorize Every xDS Stream — mTLS Alone Is Not Authorization

* **A Verified Client Certificate Proves Identity, Not Entitlement:** In the go-control-plane `OnStreamOpen`/`OnStreamRequest` callbacks, extract the verified peer identity (certificate SAN, or a SPIFFE ID) and reject any stream whose identity is not on an explicit allowlist bound to the expected runtime/policy-engine node. Stream callbacks that only log the connecting peer, with no accept/reject decision, are equivalent to no authorization at all — any client that can complete the TLS handshake (including one holding a leaked or over-broadly-issued cert) receives the full snapshot.
* **The Snapshot Is Sensitive:** An xDS snapshot routinely contains API-key hashes, subscription data, and full policy chains for every tenant served by that gateway — authorize the stream as carefully as you would authorize a request to read that data directly via a REST API.

### 3. Never Embed Private Key Material Inline in an xDS Resource Body

* **Use SDS References, Not `inline_bytes`:** Serve the downstream listener's TLS certificate and private key via a `TlsCertificateSdsSecretConfig` reference, exactly like the upstream CA path already does — never compose the raw private key as `inline_bytes` directly inside the LDS `Listener` resource. An inline key means a single successful LDS request (from any peer that clears directives 1–2) exfiltrates the listener's private key in the resource body itself, independent of any other compromise.

### 4. Harden the Envoy Admin Interface Independently of the xDS Channel

* **Bind to Loopback Only:** Envoy's admin listener (`:9901`-style) must bind `127.0.0.1` in the bootstrap config, never `0.0.0.0` — it exposes TLS private keys, full runtime config, and a `/runtime_modify` endpoint capable of live traffic reconfiguration.
* **Disable Runtime Modification:** Point `flags_path` (or the equivalent runtime-layering mechanism) at a read-only mount so `/runtime_modify` cannot be used even by a caller that reaches the loopback-bound admin port through some other path (a compromised sidecar, a debug shell in the pod).
* **No Network Exposure by Construction:** Ship a default-deny NetworkPolicy template for the admin port in the Helm chart, and never provision a Kubernetes `Service`/`Ingress`/`NodePort` that exposes it.

### 5. Resource-Limit Every xDS gRPC Server

* **Explicit Message-Size and Stream Limits:** Set `grpc.MaxRecvMsgSize`, `grpc.MaxSendMsgSize`, and `grpc.MaxConcurrentStreams` on every xDS gRPC server construction (see `go-network-service-hardening.md` directive 2 for the general form) — an unbounded default lets a single client (malicious or merely malfunctioning) exhaust memory or consume the full stream-slot budget that other clients depend on.

### 6. Canonicalize Request Paths Before Any Route/Policy/Authz Match Happens Downstream

* **Set Path-Normalization Options on Every Generated HCM:** When the xDS translator generates a downstream `HttpConnectionManager`, always set `NormalizePath: true`, `MergeSlashes: true`, and an explicit `PathWithEscapedSlashesAction` (`UNESCAPE_AND_REDIRECT` or `REJECT_REQUEST`). Leaving these unset means Envoy performs route, policy, and authz matching against an un-normalized path (`//`, `/./`, `/../`, `%2F`), which can desynchronize the route Envoy actually selects from the route the operator's policy/authz configuration was written against — the same class of bypass `authentication_authorization.md` GO-AUTH-004 addresses for Go's own `net/http` routing, applied here to generated Envoy config.
* **Apply Consistently Across Every Generated HCM:** The main downstream listener, any WebSub-internal listener, and any dynamic-forward-proxy HCM must all set these fields identically — add a test asserting their presence in generated config so a new listener type added later doesn't silently omit them.

---

## Code Examples for Enforcement

### ❌ Anti-Pattern (What to Reject)

```go
// BAD: plaintext, unauthenticated xDS gRPC server.
func NewEnvoyXDSServer() *grpc.Server {
    return grpc.NewServer() // no TLS credentials
}

// BAD: TLS defaults to disabled, with no refusal to start on a non-loopback bind.
type PolicyServerTLSConfig struct {
    Enabled bool // defaults false — insecure zero value
}

// BAD: stream callback logs the peer but never makes an accept/reject decision.
func (cb *serverCallbacks) OnStreamOpen(ctx context.Context, id int64, typ string) error {
    log.Infof("xDS stream opened: id=%d type=%s", id, typ)
    return nil // no identity check — always allows
}

// BAD: private key embedded directly in the LDS resource as inline_bytes.
func createDownstreamTLSContext(cert, key []byte) *tlsv3.DownstreamTlsContext {
    return &tlsv3.DownstreamTlsContext{
        CommonTlsContext: &tlsv3.CommonTlsContext{
            TlsCertificates: []*tlsv3.TlsCertificate{{
                PrivateKey: &corev3.DataSource{Specifier: &corev3.DataSource_InlineBytes{InlineBytes: key}}, // exfiltratable via LDS
            }},
        },
    }
}

// BAD: Envoy admin interface bound to all interfaces.
// envoy-bootstrap.yaml
//   admin:
//     address:
//       socket_address: { address: 0.0.0.0, port_value: 9901 }

// BAD: no path normalization on the generated HCM.
func createListener() *listenerv3.Listener {
    hcm := &hcmv3.HttpConnectionManager{
        // NormalizePath, MergeSlashes, PathWithEscapedSlashesAction left unset
    }
    return buildListener(hcm)
}
```

### Best Practice (What to Generate)

```go
// GOOD: xDS gRPC server requires TLS with client-cert verification by
// construction — refuses to start otherwise unless bound to loopback. The
// loopback-plaintext exception is propagated into serverCallbacks so
// OnStreamOpen authorizes streams consistently with what was actually allowed
// to start — never left to independently re-derive (or contradict) that decision.
func NewEnvoyXDSServer(cfg XDSServerConfig, cb *serverCallbacks) (*grpc.Server, error) {
    plaintextLoopback := !cfg.TLS.Enabled
    if plaintextLoopback && !isLoopback(cfg.BindAddr) {
        return nil, fmt.Errorf("xDS server TLS is disabled and bind address %q is not loopback — refusing to start", cfg.BindAddr)
    }
    cb.allowPlaintextLoopback = plaintextLoopback // Only ever true for a validated loopback bind

    var opts []grpc.ServerOption
    if cfg.TLS.Enabled {
        pool := x509.NewCertPool()
        caBytes, err := os.ReadFile(cfg.TLS.ClientCAFile)
        if err != nil {
            return nil, fmt.Errorf("loading xDS client CA: %w", err)
        }
        if ok := pool.AppendCertsFromPEM(caBytes); !ok {
            return nil, fmt.Errorf("xDS client CA file %q contained no valid certificates", cfg.TLS.ClientCAFile)
        }

        cert, err := tls.LoadX509KeyPair(cfg.TLS.CertFile, cfg.TLS.KeyFile)
        if err != nil {
            return nil, fmt.Errorf("loading xDS server cert: %w", err)
        }

        creds := credentials.NewTLS(&tls.Config{
            Certificates: []tls.Certificate{cert},
            ClientAuth:   tls.RequireAndVerifyClientCert,
            ClientCAs:    pool,
        })
        opts = append(opts, grpc.Creds(creds))
    }

    opts = append(opts,
        grpc.MaxRecvMsgSize(50<<20),
        grpc.MaxSendMsgSize(50<<20),
        grpc.MaxConcurrentStreams(1000),
    )
    return grpc.NewServer(opts...), nil
}

// GOOD: the stream callback authenticates AND authorizes — a verified
// peer identity is checked against an explicit allowlist before the stream
// is allowed to proceed; anything else is rejected outright.
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
        // Only bypass the cert check when the server was constructed with the
        // validated loopback-plaintext exception (directive 1) — never as a
        // fallback for a TLS handshake that simply failed to present a cert.
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

// GOOD: downstream listener cert/key served via SDS — never inline_bytes.
func createDownstreamTLSContext(sdsSecretName string) *tlsv3.DownstreamTlsContext {
    return &tlsv3.DownstreamTlsContext{
        CommonTlsContext: &tlsv3.CommonTlsContext{
            TlsCertificateSdsSecretConfigs: []*tlsv3.SdsSecretConfig{{
                Name:      sdsSecretName,
                SdsConfig: sdsConfigSource(), // Delivered only over the mTLS-authenticated xDS channel from directives 1-2
            }},
        },
    }
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

```go
// GOOD: path normalization set explicitly on every generated HCM.
func createListener() *listenerv3.Listener {
    hcm := &hcmv3.HttpConnectionManager{
        NormalizePath:                wrapperspb.Bool(true),
        MergeSlashes:                 true,
        PathWithEscapedSlashesAction: hcmv3.HttpConnectionManager_UNESCAPE_AND_REDIRECT,
    }
    return buildListener(hcm)
}
```

---

> **Verification Checklist before outputting code:**
> * Does an xDS/policy-xDS gRPC server default to TLS-disabled, or allow a plaintext bind to a non-loopback address without refusing to start? (If yes, flip the default and add the fail-closed startup check.)
> * Does a go-control-plane stream callback (`OnStreamOpen`/`OnStreamRequest`) only log the connecting peer without making an explicit accept/reject decision against an identity allowlist? (Logging is not authorization — add the check.)
> * Is any private key composed as `inline_bytes` inside an xDS resource (LDS `Listener`, or any other resource type)? (If yes, replace with an SDS secret reference.)
> * Does the Envoy admin interface bind `0.0.0.0`, or is `/runtime_modify` reachable without a read-only `flags_path`? (If yes, bind loopback-only and lock down the runtime layer; confirm no Service/Ingress/NodePort exposes the port.)
> * Does an xDS gRPC server construction omit `MaxRecvMsgSize`/`MaxSendMsgSize`/`MaxConcurrentStreams`? (If yes, add explicit limits per `go-network-service-hardening.md` directive 2.)
> * Does the xDS translator generate an `HttpConnectionManager` without `NormalizePath`/`MergeSlashes`/`PathWithEscapedSlashesAction` set? (If yes, set all three on every HCM the translator produces, including non-primary listeners.)
