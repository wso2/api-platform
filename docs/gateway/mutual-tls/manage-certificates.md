# Manage certificates

Every certificate the gateway uses for mutual TLS is an entry in its certificate pool. This page shows you how to add each kind of entry, list the pool, rotate a gateway identity, and delete an entry, and how to configure the header a load balancer relays client certificates in.

This page is for the **platform administrator** who curates the pool. The **AI developer** can list the pool to find the names to use in an API, but can't change it.

## Before you start

The examples send requests to the management API at `http://localhost:9090/api/management/v1` as the `admin` user. Replace *`<password>`* with that user's password. Uploading, rotating, and deleting require the `admin` role. Listing also accepts the `developer` role.

Each upload is a JSON body, and PEM text has to be escaped to fit in a JSON string. The examples build the body with `jq --rawfile`, which reads a PEM file as it is, and pipe it into `curl`. That keeps certificates and private keys out of your shell history.

Names may contain letters, digits, `.`, `_`, and `-`, up to 100 characters. They are one namespace across every usage: uploading a second entry under a name that's already taken, of any usage, returns `409`.

## Add a client authority

A client authority is the certificate authority that issues your callers' certificates. Upload it with `usage: downstream`:

```bash
jq -n --arg name partner-a --rawfile certificate partner-a-ca.pem \
  '{name: $name, usage: "downstream", role: "client", certificate: $certificate}' |
curl -s -X POST http://localhost:9090/api/management/v1/certificates \
  -u admin:<password> \
  -H "Content-Type: application/json" \
  --data-binary @-
```

`role` defaults to `client`, so you can leave it out. The file can hold a root authority, an issuing intermediate, or the intermediate followed by its root. The response can carry non-fatal `warnings`: `CLIENT_CA_IS_LEAF` when the certificate isn't itself an authority and is pooled as a one-member authority that trusts exactly that certificate, `CLIENT_CA_NOT_YET_VALID` when it isn't valid yet, and `CERT_EXPIRES_SOON` when it expires within thirty days. An expired certificate is refused.

A new authority grants no access on its own. An API accepts it only if the API's `mtls-auth` policy names it, or omits `accept` and so accepts every client authority in the pool. See [Authenticate clients with certificates](authenticate-clients-with-certificates.md).

## Add a relay entry for a load balancer

When a load balancer terminates TLS in front of the gateway, it can relay the caller's certificate in a request header. A relay entry identifies a load balancer that may forward client certificates. It isn't an accepted caller itself, and an `accept` list can't name it.

Upload the authority that issues the load balancer's own certificate with `role: relay`. An optional `match` narrows the entry to connections whose certificate carries one of the listed SANs:

```bash
jq -n --arg name edge-lb --rawfile certificate edge-lb-ca.pem \
  '{name: $name, usage: "downstream", role: "relay",
    match: {dnsSANs: ["lb.example.com"]}, certificate: $certificate}' |
curl -s -X POST http://localhost:9090/api/management/v1/certificates \
  -u admin:<password> \
  -H "Content-Type: application/json" \
  --data-binary @-
```

`match` takes `dnsSANs`, `uriSANs`, or both, and each list must name at least one SAN. Only a relay entry takes `match`. Without `match`, any connection presenting a certificate from this authority can relay a header.

## Add a gateway identity

A gateway identity is the certificate and private key the gateway presents to a backend that requires mutual TLS. Upload the chain leaf first, with any intermediates after it, and the unencrypted private key that matches the leaf:

```bash
jq -n --arg name gateway-billing \
  --rawfile certificate gateway-billing-chain.pem \
  --rawfile privateKey gateway-billing.key \
  '{name: $name, usage: "identity", certificate: $certificate, privateKey: $privateKey}' |
curl -s -X POST http://localhost:9090/api/management/v1/certificates \
  -u admin:<password> \
  -H "Content-Type: application/json" \
  --data-binary @-
```

The gateway checks the upload before it stores anything:

- **The key must be strong enough.** It must be an RSA key of 2048 bits or larger, an ECDSA key on P-256, P-384, or P-521, or an Ed25519 key.
- **The key must match the leaf certificate,** and the chain must be ordered leaf first.
- **The key must be unencrypted.** A passphrase-protected key is refused, because the gateway never stores a passphrase.
- **The certificate must not be expired.** One that expires within thirty days is accepted with a `CERT_EXPIRES_SOON` warning, and one whose extended key usage excludes client authentication with an `IDENTITY_NO_CLIENTAUTH_EKU` warning.

The gateway encrypts the private key at rest. No response ever returns it, including the upload response and the list. The response reports the identity's `subject`, `issuer`, `notAfter`, `keyAlgorithm`, and `chainLength` instead.

### Make an identity the default

Give an identity `role: default` to make it the one the gateway presents to backends whose upstream definition names no `tls.identity`, when `router.upstream.tls.present_default_identity` is on. See [Present a default identity](connect-to-backends-with-mtls.md#present-a-default-identity).

```bash
jq -n --arg name gateway-default \
  --rawfile certificate gateway-default-chain.pem \
  --rawfile privateKey gateway-default.key \
  '{name: $name, usage: "identity", role: "default", certificate: $certificate, privateKey: $privateKey}' |
curl -s -X POST http://localhost:9090/api/management/v1/certificates \
  -u admin:<password> \
  -H "Content-Type: application/json" \
  --data-binary @-
```

Issue this certificate for client authentication. A backend that checks the extended key usage refuses one that leaves it out.

Only one identity can have `role: default`. Uploading a second returns `409` with `gateway identity gateway-default already has role: default; delete it before uploading another default identity`. The role is fixed at upload: rotating the identity keeps it. To make another identity the default, delete this one and upload the other with `role: default`.

`role` pairs with a usage. `client` and `relay` apply only to `usage: downstream`, and `default` applies only to `usage: identity`. Any other pairing returns `400`, for example `role default applies only to usage: identity certificates`.

## Add an upstream trust certificate

An upstream trust certificate is one the gateway trusts when it verifies a backend's certificate. Upload the authority that issued the backend's certificate with `usage: upstream`, which is also the default:

```bash
jq -n --arg name billing-ca --rawfile certificate billing-ca.pem \
  '{name: $name, usage: "upstream", certificate: $certificate}' |
curl -s -X POST http://localhost:9090/api/management/v1/certificates \
  -u admin:<password> \
  -H "Content-Type: application/json" \
  --data-binary @-
```

Upstream trust certificates join the gateway-wide trust bundle that backends are verified against by default, alongside the system certificate authorities. An upstream definition can list some of them in `trustedCAs` to trust only those for its backend. See [Connect to backends with mutual TLS](connect-to-backends-with-mtls.md).

## List the pool

List every entry, or filter by `usage`:

```bash
curl -s "http://localhost:9090/api/management/v1/certificates?usage=downstream" \
  -u admin:<password>
```

`usage` takes `downstream`, `identity`, or `upstream`. Each entry carries its `id`, `name`, `usage`, `subject`, `issuer`, and `notAfter`, plus `role` and `match` for a client authority, and `role: default` on the default identity. For a client authority or an identity, `referencedByApis` counts the deployed APIs that name it. An API that omits `accept` and inherits the whole pool doesn't count toward it. An upstream trust certificate has no `referencedByApis`: it joins the gateway-wide trust bundle, so it can be in use for backends without any API naming it. Deleting one is still refused while an API names it in `trustedCAs`. An entry that expires within thirty days carries a `CERT_EXPIRES_SOON` warning in the list, whatever its usage.

## Rotate a gateway identity

Replace an identity's certificate and key in place with `PUT`. The name stays the same, so every upstream definition that names it keeps working without a redeploy. Find the identity's `id` first:

```bash
ID=$(curl -s "http://localhost:9090/api/management/v1/certificates?usage=identity" \
  -u admin:<password> | jq -r '.certificates[] | select(.name == "gateway-billing") | .id')

jq -n --rawfile certificate gateway-billing-2027-chain.pem \
  --rawfile privateKey gateway-billing-2027.key \
  '{certificate: $certificate, privateKey: $privateKey}' |
curl -s -X PUT "http://localhost:9090/api/management/v1/certificates/$ID" \
  -u admin:<password> \
  -H "Content-Type: application/json" \
  --data-binary @-
```

The new certificate and key go through the same checks as an upload. New connections to the backend present the new identity.

Only identities can be updated. A `PUT` to any other entry returns `400` with `only usage: identity certificates can be updated; delete and re-upload other certificates`. To replace a client authority or a trust certificate, upload the new one under a new name, point the APIs at it, and delete the old one.

## Delete an entry

Delete an entry by its `id`:

```bash
curl -s -X DELETE "http://localhost:9090/api/management/v1/certificates/$ID" \
  -u admin:<password>
```

The gateway refuses to delete an entry that a deployed API still depends on, and returns `409`. The response lists each reference by field path and API name, so you know what to change first. An entry is in use when it is:

- a client authority named in an `mtls-auth` `accept` list,
- the last client authority in the pool while any deployed API attaches `mtls-auth`,
- an identity named in an upstream definition's `tls.identity`, or
- an upstream trust certificate named in an upstream definition's `tls.trustedCAs`.

`role: default` doesn't block a delete, because no API refers to an identity by its role. Once the default identity is gone, backends are presented the HTTPS listener certificate instead. A relay entry can always be deleted. Once the last relay entry is gone, relayed headers are no longer believed, unless `trust_any` is on.

## Configure the client certificate header

Two gateway-wide settings in [`config.toml`](index.md#gateway-settings) control how a relayed client certificate is read. They apply to every API and can't be set in an API definition:

```toml
[router.downstream_tls.client_certificate_header]
name = "X-WSO2-CLIENT-CERTIFICATE"
trust_any = false
```

- **`name`** is the header your load balancer relays the client certificate in. It defaults to `X-WSO2-CLIENT-CERTIFICATE` and must be a valid HTTP header name.
- **`trust_any`** believes the header on any connection whose own certificate wasn't rejected, not only on a connection from a relay entry. It's off by default.

Both settings are read at startup. Restart the gateway after changing them.

> **Warning:** With `trust_any` on, any client that can open a connection to the gateway can put any certificate in the header and be authenticated as its subject. Turn it on only when nothing but a trusted load balancer can reach the gateway. While it's on, the gateway logs a warning at startup, and every `mtls-auth` deployment carries a `HEADER_CERT_BYPASS_ACTIVE` warning.

Prefer a relay entry to `trust_any`. A relay entry makes the gateway authenticate the load balancer before it believes a header.

Two more settings under `[router.downstream_tls]` concern hostnames. `client_certificate_request` chooses whether the listener asks only connections to the hostnames of `mtls-auth` APIs, the default, or every connection. `mtls_requires_dedicated_hostname` refuses to deploy an `mtls-auth` API without its own hostname, and applies only when the listener asks by hostname. See [Hostnames and the certificate request](authenticate-clients-with-certificates.md#hostnames-and-the-certificate-request).

## Upgrade note

If your gateway stores its configuration in PostgreSQL or SQL Server, apply the schema file for your release to the database before you upgrade. See the release notes for your version.

## Related topics

- [Mutual TLS](index.md) — the two directions, the three usages, and how APIs refer to pool entries.
- [Certificate management](../../rest-apis/gateway/certificate-management.md) — the full request and response reference for these operations.
- [Mutual TLS Authentication policy](https://wso2.com/api-platform/policy-hub/policies/mtls-auth) — how the policy uses client authorities and relay entries.
