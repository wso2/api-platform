# Authenticate clients with certificates

An API that attaches the `mtls-auth` policy authenticates each caller by the client certificate it presents. This page shows you how to require a certificate on an API, narrow which certificates it accepts, and keep working when a load balancer terminates TLS in front of the gateway.

This page is for the **AI developer** who protects an API with client certificates, and the **platform administrator** who adds the authorities and relay entries it depends on.

## Before you start

The policy holds no certificates. It names entries from the gateway's certificate pool, so the authority that issues your callers' certificates must be in the pool as a `usage: downstream` entry before you deploy. See [Add a client authority](manage-certificates.md#add-a-client-authority). The examples on this page use an authority named `partner-a`.

Callers must connect to the gateway's HTTPS listener, on port 8443 by default. The gateway refuses to deploy the policy in two cases:

- when the HTTPS listener is disabled: `mtls-auth requires the HTTPS listener, which is disabled on this gateway`
- when the pool holds no client authority: `mtls-auth requires at least one client authority; add one with POST /certificates and usage: downstream`

## Require a client certificate

Attach `mtls-auth` to the API's policies and name the authority it accepts:

```yaml
apiVersion: gateway.api-platform.wso2.com/v1
kind: RestApi
metadata:
  name: orders-api
spec:
  displayName: Orders API
  version: v1.0
  context: /orders/$version
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

Attach the policy at API level to protect every operation, or to one operation's `policies` to protect only that operation. Use it once per scope, and at one level only. To require a token as well as a certificate, list `jwt-auth` after `mtls-auth` in the same chain.

## Call the API with a certificate

A caller presents its certificate and key on the TLS connection:

```bash
curl https://localhost:8443/orders/v1.0/orders \
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

The reason is recorded in the gateway's logs and traces instead. The policy's `onFailureStatusCode`, `errorMessageFormat`, and `errorMessage` parameters change the response.

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
              - "sha256:5b0d9c2f7e4a1b8c3d6e9f0a2b4c6d8e0f1a3b5c7d9e1f2a4b6c8d0e2f4a6b8c"
```

A thumbprint is 64 hex characters, with or without colons and a `sha256:` prefix. When a pinned caller renews its certificate, list the new thumbprint alongside the old one, let the caller switch, then remove the old one.

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
