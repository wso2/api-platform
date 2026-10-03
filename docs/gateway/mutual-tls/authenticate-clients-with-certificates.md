# Authenticate clients with certificates

An API that attaches the `mtls-auth` policy authenticates each caller by the client certificate it presents. This page shows you how to require a certificate on an API, narrow which certificates it accepts, and keep working when a load balancer terminates TLS in front of the gateway.

This page is for the **AI developer** who protects an API with client certificates, and the **platform administrator** who adds the authorities and relay entries it depends on.

## Before you start

The policy holds no certificates. It names entries from the gateway's certificate pool, so the authority that issues your callers' certificates must be in the pool as a `usage: downstream` entry before you deploy. See [Add a client authority](manage-certificates.md#add-a-client-authority). The examples on this page use an authority named `partner-a`.

Callers must connect to the gateway's HTTPS listener, on port 8443 by default. The gateway refuses to deploy the policy in two cases:

- when the HTTPS listener is disabled: `mtls-auth requires the HTTPS listener, which is disabled on this gateway`
- when the pool holds no client authority: `mtls-auth requires at least one client authority; add one with POST /certificates and usage: downstream`

## Require a client certificate

Attach `mtls-auth` to the API's policies and name the authority it accepts. Every parameter is described in the [Mutual TLS Authentication policy](https://wso2.com/api-platform/policy-hub/policies/mtls-auth) documentation:

```yaml
apiVersion: gateway.api-platform.wso2.com/v1
kind: RestApi
metadata:
  name: orders-api
spec:
  displayName: Orders API
  version: v1.0
  context: /orders/$version
  vhosts:
    main: orders.example.com
  upstream:
    main:
      url: http://orders-backend:9080/api/v1
  policies:
    - name: mtls-auth
      version: v1
      params:
        accept:
          - ca: partner-a
  operations:
    - method: GET
      path: /orders
```

Deploy it through the management API:

```bash
curl -X POST http://localhost:9090/api/management/v1/rest-apis \
  -u admin:<password> \
  -H "Content-Type: application/yaml" \
  --data-binary "@orders-api.yaml"
```

The API has its own hostname in `vhosts.main`, so the gateway asks for a client certificate only on connections to `orders.example.com`, and callers of your other APIs aren't asked. Without `vhosts`, the API is served on the gateway's default hostname. It still works, but the deploy response carries an `MTLS_HOSTNAME_NOT_SCOPED` warning, and every connection to the gateway is asked for a certificate, including those of browsers calling other APIs. See [Hostnames and the certificate request](#hostnames-and-the-certificate-request).

Attach the policy at API level to protect every operation, or to one operation's `policies` to protect only that operation. Use it once per scope, and at one level only. To require a token as well as a certificate, list `jwt-auth` after `mtls-auth` in the same chain.

## Call the API with a certificate

A caller presents its certificate and key on the TLS connection, and sends the API's hostname. When testing locally, `--resolve` sends it to the gateway on your machine:

```bash
curl https://orders.example.com:8443/orders/v1.0/orders \
  --resolve orders.example.com:8443:127.0.0.1 \
  --cert partner-a-client.pem \
  --key partner-a-client.key \
  --cacert gateway-ca.pem
```

A certificate the API doesn't accept, an expired or untrusted one, and a missing one all get the same response, so a caller can't learn why it was refused:

```http
HTTP/1.1 401 Unauthorized
Content-Type: application/json

{"error":"Unauthorized","message":"Authentication failed"}
```

The reason is recorded in the gateway's [logs](index.md#logs) and traces instead. The policy's `onFailureStatusCode`, `errorMessageFormat`, and `errorMessage` parameters change the response.

## Hostnames and the certificate request

The HTTPS listener decides whether to ask a connection for a client certificate during the TLS handshake, before any request arrives. It decides from the hostname the caller sends in the handshake, its server name indication (SNI).

The `client_certificate_request` setting in [`config.toml`](index.md#gateway-settings) chooses which connections are asked:

```toml
[router.downstream_tls]
client_certificate_request = "mtls_hostnames"
```

- **`mtls_hostnames`** asks only connections to the hostnames of `mtls-auth` APIs. It's the default.
- **`all_connections`** asks every connection, whatever its hostname.

With either value, no connection is asked while no deployed API attaches `mtls-auth` or the pool holds no authority. The setting is read at startup, so restart the gateway after changing it.

Choose `all_connections` when callers of `mtls-auth` APIs send no SNI or a hostname other than the API's, or connect by IP address. It also keeps the listener unchanged when an `mtls-auth` API is added with a new hostname or changes its hostname, so Envoy applies fewer listener updates. In exchange, callers of every API are asked for a certificate, and no connection keeps TLS session resumption.

With `mtls_hostnames`, give each API that attaches `mtls-auth` its own hostname in `vhosts.main`. Use an exact name such as `orders.example.com`, or a single leading `*.` label such as `*.orders.example.com`. When the API has a sandbox upstream, give `vhosts.sandbox` its own hostname as well:

```yaml
spec:
  vhosts:
    main: orders.example.com
    sandbox: orders-sandbox.example.com
```

When every `mtls-auth` API has its own hostname, the gateway asks for a certificate only on connections to those hostnames. Connections to any other hostname are never asked and keep TLS session resumption.

The gateway asks every connection instead, whatever its hostname, in these cases:

- An `mtls-auth` API has no `vhosts.main`, or is served on the gateway's default hostname.
- A hostname is an IP address, ends with a dot, or is a pattern other than an exact name or a leading `*.`, such as `*` or `orders-*`.
- The pool holds a relay entry, because a load balancer can connect on any hostname.

When an API's own hostname is the cause, its deploy response carries an `MTLS_HOSTNAME_NOT_SCOPED` warning on `spec.vhosts.main` or `spec.vhosts.sandbox`. The warning is raised only with `mtls_hostnames`.

To refuse such APIs instead, turn on `mtls_requires_dedicated_hostname` in [`config.toml`](index.md#gateway-settings). It's off by default, and it applies only with `mtls_hostnames`:

```toml
[router.downstream_tls]
mtls_requires_dedicated_hostname = true
```

With it on, deploying or updating an `mtls-auth` API whose own hostname is the cause fails with `400`, on `spec.vhosts.main` or `spec.vhosts.sandbox`:

```text
this gateway requires every mtls-auth API to have its own hostname (an exact name or a leading *.); set vhosts.main
```

A relay entry in the pool is never a reason to refuse a deploy. APIs already stored when you turn the setting on keep being served, and the listener still asks every connection. The gateway logs a warning for each of them at startup; update each one to give it its own hostname. The setting is read at startup, so restart the gateway after changing it.

With `all_connections`, `mtls_requires_dedicated_hostname` has no effect: no deploy is refused and no API is named at startup. If both are set, the gateway logs one warning at startup saying so.

Callers must send the API's hostname as SNI. While the gateway asks only on the hostnames of `mtls-auth` APIs, a connection that isn't asked presents no certificate, so its requests get the policy's `401`. That happens when a connection:

- sends no SNI,
- connects by IP address,
- sends a hostname other than the API's, or
- is reused for a request to another hostname, as HTTP/2 clients do when they coalesce connections to hostnames that share an address and a certificate.

To serve such callers, set `client_certificate_request` to `all_connections`.

To send the hostname as SNI when testing locally, resolve it to the gateway:

```bash
curl https://orders.example.com:8443/orders/v1.0/orders \
  --resolve orders.example.com:8443:127.0.0.1 \
  --cert partner-a-client.pem \
  --key partner-a-client.key \
  --cacert gateway-ca.pem
```

Hostnames are compared without regard to case. A port in `vhosts` plays no part in the match: `orders.example.com:8443` asks every connection to `orders.example.com`, so an API without `mtls-auth` on that bare hostname is asked too.

## Choose which authorities to accept

`accept` decides which pooled authorities an API trusts:

- **Name the authorities.** Each `accept` entry names one `role: client` entry in `ca`. Entries are checked in order, and the first one the certificate satisfies authenticates the request. Adding an authority to the pool grants nothing to an API that doesn't name it.
- **Omit `accept`.** The API accepts a certificate from any client authority in the pool, now and as the pool changes. The deploy response carries an `MTLS_ACCEPT_INHERITS_POOL` warning so the choice is visible. An empty list is refused.

An entry can't name a relay entry. Deploying one returns `edge-lb is a relay (front proxy) entry and cannot be accepted as a client`.

The policy reads the pool on every request, so a change to the pool, or to an API's `accept` list, applies from the caller's next request.

## Narrow an authority by SAN

An authority often issues certificates to more callers than one API should admit. Add `match` to accept only certificates that carry a given subject alternative name (SAN):

```yaml
      params:
        accept:
          - ca: partner-a
            match:
              uriSANs:
                - "urn:partner-a:payments"
```

`match` takes `uriSANs`, `dnsSANs`, or both. The certificate must carry at least one SAN from each list you set. Values are compared exactly, with no wildcards or patterns. An entry without `match` or `thumbprints` accepts any certificate from its authority, and the deploy response warns about it with `MTLS_ACCEPT_UNNARROWED`.

## Pin exact certificates

To admit specific certificates and nothing else from an authority, list their SHA-256 thumbprints:

```yaml
      params:
        accept:
          - ca: partner-a
            thumbprints:
              - "5b0d9c2f7e4a1b8c3d6e9f0a2b4c6d8e0f1a3b5c7d9e1f2a4b6c8d0e2f4a6b8c"
```

A thumbprint is 64 hex characters, with or without colons and a `sha256:` prefix. The gateway stores it as 64 lowercase hex characters, the form `openssl x509 -in client.pem -noout -fingerprint -sha256 | cut -d= -f2 | tr -d : | tr A-F a-f` prints; one given in another form is converted, and the deploy response carries an `MTLS_THUMBPRINT_NORMALISED` warning. When a pinned caller renews its certificate, list the new thumbprint alongside the old one, let the caller switch, then remove the old one.

## Run behind a load balancer

When a load balancer terminates TLS in front of the gateway, the caller's certificate never reaches the gateway's handshake. The load balancer relays it in a request header instead, and the gateway believes that header only on a connection it has authenticated as the load balancer.

To set this up:

1. **Add a relay entry** for the authority that issues the load balancer's own certificate. See [Add a relay entry for a load balancer](manage-certificates.md#add-a-relay-entry-for-a-load-balancer).
2. **Have the load balancer connect with its certificate** and relay the caller's certificate in the configured header, `X-WSO2-CLIENT-CERTIFICATE` by default. See [Configure the client certificate header](manage-certificates.md#configure-the-client-certificate-header).
3. **Keep the API definition as it is.** The same `accept` list applies to the relayed certificate.

The header must carry exactly one certificate, as PEM, URL-encoded PEM, or base64-encoded DER. A header on a connection that isn't a relay is ignored, unless the administrator has turned on `trust_any`.

Don't pool the load balancer's authority as a `role: client` entry that the API accepts as well. The load balancer's own certificate then authenticates every request, and the callers behind it are no longer checked individually. The deploy response warns about this with `MTLS_ACCEPT_NAMES_RELAY_AUTHORITY`.

## What the backend receives

After a request is allowed, the backend receives at most one certificate header: `X-Forwarded-Client-Cert`, describing the certificate the caller authenticated with. When the certificate was relayed, the header describes the relayed certificate, not the load balancer's. The relayed header itself never reaches the backend.

Set `forwardCertificate` to `false` to remove `X-Forwarded-Client-Cert` too:

```yaml
      params:
        accept:
          - ca: partner-a
        forwardCertificate: false
```

Later policies in the chain can read the caller's identity from the authentication context the policy sets, and analytics records its subject as the request's user.

## How the policy decides

This page covers the tasks. The exact order the policy checks the connection certificate and a relayed certificate in, the authentication context fields, and every parameter are in the [Mutual TLS Authentication policy](https://wso2.com/api-platform/policy-hub/policies/mtls-auth) documentation.

## Related topics

- [Manage certificates](manage-certificates.md) — add the authorities and relay entries this page names.
- [Connect to backends with mutual TLS](connect-to-backends-with-mtls.md) — the certificate the gateway presents to a backend, which is a separate concern.
- [Mutual TLS](index.md) — how the certificate pool and the two roles fit together.
- [Mutual TLS Authentication policy](https://wso2.com/api-platform/policy-hub/policies/mtls-auth) — every `mtls-auth` parameter and the order the policy decides in.
