# Mutual TLS

With ordinary TLS, only the server proves who it is. With mutual TLS, both sides of the connection present a certificate, so each side knows who is at the other end before any request is sent. The gateway supports mutual TLS in both directions: from a client to the gateway, and from the gateway to a backend.

This page is for the **platform administrator** who decides which certificates the gateway trusts and presents, and the **API developer** who applies them to an API.

## What each direction gives you

Mutual TLS works independently in the two directions, and you can use either one without the other.

- **Client to gateway.** A caller authenticates with the client certificate it presents on the TLS connection, instead of or as well as an API key or a token. You attach the `mtls-auth` policy to an API, and the gateway accepts only certificates issued by the authorities that API names. You can narrow an authority further to certificates with a given subject alternative name (SAN) or a given thumbprint.
- **Gateway to backend.** When a backend requires its callers to present a certificate, the gateway presents one of its own. You upload the certificate and its private key once, then name it in the upstream definition of each API that calls that backend. The same definition can also say which certificates the gateway trusts for that backend.

Client certificates work on the gateway's HTTPS listener, on port 8443 by default. The gateway asks callers for a certificate only while at least one deployed API attaches `mtls-auth`. By default, when each of those APIs has its own hostname, only connections to those hostnames are asked, so callers of other APIs aren't affected. Otherwise, or when you set the gateway to ask every connection, every connection is asked. See [Hostnames and the certificate request](authenticate-clients-with-certificates.md#hostnames-and-the-certificate-request).

## The two roles

The certificates the gateway uses live in one pool on the gateway, managed through the management API.

- **The platform administrator curates the pool.** Only an administrator can upload, rotate, or delete a certificate. The administrator decides which client authorities exist, which load balancers may relay client certificates, which identities the gateway can present, and which backend certificates it trusts.
- **The API developer selects from the pool.** A developer can list the pool, and names entries from it in an API: an authority in an `mtls-auth` `accept` list, or an identity and trusted certificates in an upstream definition's `tls` block. A developer never handles a private key.

Every pool entry has a name and a `usage`:

| Usage | What it holds | Where it's named |
|---|---|---|
| `downstream` | A certificate authority that issues client certificates, with a `role` of `client` or `relay` | `accept[].ca` in the `mtls-auth` policy |
| `identity` | A certificate chain and private key the gateway presents to a backend | `tls.identity` in an upstream definition |
| `upstream` | A certificate the gateway trusts when it verifies a backend | `tls.trustedCAs` in an upstream definition |

Names are one namespace across all three usages, so an authority and an identity can't share a name. The gateway checks at deploy time that each name an API uses refers to an entry of the right usage, and refuses to delete an entry while a deployed API still names it.

## How it fits together

```
  Partner client                                    Billing backend
  (presents a client certificate)                   (requires a client certificate)
         │                                                  ▲
         │ mutual TLS                            mutual TLS │
         ▼                                                  │
┌─────────────────────────────────────────────────────────────────────┐
│ Gateway                                                             │
│                                                                     │
│  mtls-auth policy                  upstream definition tls block    │
│    accept: partner-a                 identity: gateway-billing      │
│                                      trustedCAs: [billing-ca]       │
│         │                                   │                       │
│         ▼                                   ▼                       │
│  ┌───────────────────────────────────────────────────────────────┐  │
│  │ Certificate pool                                              │  │
│  │   partner-a        usage: downstream, role: client            │  │
│  │   edge-lb          usage: downstream, role: relay             │  │
│  │   gateway-billing  usage: identity                            │  │
│  │   billing-ca       usage: upstream                            │  │
│  └───────────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────────┘
```

The policy and the `tls` block hold no certificates themselves. They refer to pool entries by name, and the gateway resolves those names against the current pool. That lets an administrator add an authority or rotate an identity without asking every developer to redeploy.

## Gateway settings

The gateway-wide settings these pages name live in `configs/config.toml` of the gateway distribution. The shipped file lists only the most common settings, so when a setting's section isn't there, add it. For example:

```toml
[router.downstream_tls]
client_certificate_request = "all_connections"
```

A setting you don't add keeps its default. `configs/config-template.toml` lists every setting with its default. These settings are read at startup, so restart the gateway after changing them, for example with `docker compose restart`.

## Logs

Two gateway logs show what mutual TLS did with a request.

**The router's access log** has a line for every request. It's text by default; set `format = "json"` under `[router.access_logs]` in [`config.toml`](#gateway-settings) for named fields. The JSON format includes:

| Field | What it holds |
|---|---|
| `sni` | The hostname the caller sent in the TLS handshake |
| `tlsVer` | The TLS version of the caller's connection |
| `peerSubj` | The subject of the client certificate on the caller's connection, for a refused request too, and empty when there is none. Behind a load balancer it's the load balancer's certificate. |
| `peerFp` | The SHA-256 fingerprint of that certificate |
| `upTlsFail` | Why the TLS handshake with the backend failed |

`host` is the hostname the caller sent for a request the policy refused, and the host the gateway sent to the backend for a request it forwarded. To follow one API, filter on `sni`.

**The policy engine's debug log** records why `mtls-auth` refused a request, such as `no_certificate`, `expired`, `not_yet_valid`, `untrusted_chain`, `invalid_certificate`, `authority_not_accepted`, `san_mismatch`, or `thumbprint_mismatch`. Set `level = "debug"` under `[policy_engine.logging]` to see it. The debug level also logs every request the policy engine handles in full, so turn it on only while you investigate a refusal. The same reason is the `mtls_auth.reason` attribute of the request's trace span.

## In this section

| Page | What it covers |
|---|---|
| [Manage certificates](manage-certificates.md) | Upload, list, rotate, and delete pool entries, and configure the client certificate header. |
| [Authenticate clients with certificates](authenticate-clients-with-certificates.md) | Require a client certificate on an API, narrow it, and run the gateway behind a load balancer. |
| [Connect to backends with mutual TLS](connect-to-backends-with-mtls.md) | Present a gateway identity to a backend, choose what to trust, and rotate the identity. |

## Related topics

- [Mutual TLS Authentication policy](https://wso2.com/api-platform/policy-hub/policies/mtls-auth) — every `mtls-auth` parameter, the order the policy decides in, and its failure response.
- [Certificate management](../../rest-apis/gateway/certificate-management.md) — the management API operations for the certificate pool.
- [Key concepts](../README.md#key-concepts) — the artifacts an upstream definition belongs to.
