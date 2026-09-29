# Key managers

Registers and manages OAuth2 applications ("keys") on identity providers via
Dynamic Client Registration (RFC 7591) and the client configuration endpoint
(RFC 7592), behind the portal's own REST API.

Backs three operations in `docs/api-portal-openapi-spec-v0.9.yaml`:

| Endpoint | What this subsystem does |
|---|---|
| `GET /key-managers/metadata` | lists the configured key managers and the properties each accepts |
| `POST /oauth2-keys` | registers a new OAuth application and returns the issued credentials |
| `GET`/`PUT`/`DELETE /oauth2-keys/{keyId}` | reads, replaces and deletes that client at the key manager |

The REST layer is `src/services/oauth2KeyService.js`; this directory is only the
key-manager side.

## Extension model: built-in drivers, config-activated

Every driver ships inside the image. Operators do not add code — they add an
`[[api_portal.key_manager]]` entry that activates a built-in driver and supplies
its endpoints and credentials. **The driver set is closed; the instance config is
open.** Several entries may share a `type` (two Asgardeo organisations, say).

## Layout

    src/keymanagers/
      index.js            getFactory() — the one entry point for the rest of the portal
      core/
        registry.js       driver registry: register / getDriver / registeredTypes
        keyManager.js     KeyManager base class, KeyManagerCallError, metadata + DCR helpers
        authenticators.js how the portal authenticates ITSELF to a key manager
        httpClient.js     the outbound axios client: SSRF guard, TLS, pooling, ceilings
        factory.js        builds instances, resolving each `type` against the registry
      drivers/
        index.js          requires every built-in driver (fills the registry)
        custom.js         Generic DCR — RFC 7591 registration, RFC 7592 read/update/delete,
                          with the full RFC 7591 metadata set. For any spec-conforming key
                          manager that has no driver of its own here
        thunderid.js      ThunderID — the same protocol, with a short property list tuned
                          to that key manager
        wso2is.js         WSO2 Identity Server 7.x — DCR v1.1 create/read/update/delete
        asgardeo.js       Asgardeo — DCR v1.1 create/read/update/delete. Same API as
                          wso2is.js today, kept separate on purpose: separate products,
                          separate release cadences, and neither imports the other
        provision.js      no registration; an externally-created client id is recorded

Config validation lives one directory up, in `src/config/keyManagerConfig.js`,
because `configLoader` calls it during its own fail-closed startup check.

## Adding a built-in driver

1. Create `src/keymanagers/drivers/<type>.js`: a class extending `KeyManager`
   that implements `metadata()` plus whichever key operations that key manager
   can actually perform, and ends with

   ```js
   register('<type>', (cfg, authRequest) => new YourKeyManager(cfg, authRequest),
            'Your Product Name');
   ```

   The third argument is the label the settings UI shows for this type, and it is
   required — registering without one throws. It lives beside the type so the two
   cannot drift, and it exists because a `type` is a config token typed into TOML:
   deriving a label from `wso2is` yields "Wso2is", which is nobody's product.

2. Add one `require('./<type>')` line to `drivers/index.js`.

No other file changes. This is a code change shipped in the image, not a runtime
plugin — operators activate a released driver via config.

Override only what the key manager can really do. Anything left to the base
class rejects with `unsupported_operation`, which the REST layer answers as
`409` — so capability is simply whatever the driver implemented, with no
separate declaration that could claim an operation the code cannot perform.

A driver that overrides nothing still appears in metadata, because its property
descriptors are useful on their own — but it must not pretend to register. A
placeholder like the POC's `client_id: "kc-generated"` would look like a
successful registration while handing the developer credentials no authorization
server has ever heard of; leaving the method to the base class refuses with 409
instead.

## Configuration

See the `KEY MANAGERS` section of `configs/config-template.toml` for the full
reference. Minimal working entry:

```toml
[[api_portal.key_manager]]
id   = "thunder-local"        # sent as keyManagerId by a caller; unique
name = "ThunderID"            # display name
type = "thunderid"            # which built-in driver handles it
registration_endpoint = "https://localhost:8090/oauth2/dcr/register"
token_endpoint        = "https://localhost:8090/oauth2/token"
authorize_endpoint    = "https://localhost:8090/oauth2/authorize"
insecure_skip_verify  = true  # development only

  [api_portal.key_manager.auth]
  method        = "client_credentials"   # client_credentials | basic | mtls
  client_id     = "my-system-app"
  client_secret = '{{ env "THUNDER_CLIENT_SECRET" }}'
  scopes        = ["system"]
```

Secrets use the portal's own `{{ env "NAME" }}` interpolation, which aborts
startup when the variable is unset. (The standalone POC used `${NAME}`; that
second mechanism was dropped in favour of the one the rest of `config.toml`
already uses.)

Everything checkable without I/O is validated at startup — required fields,
unknown `type`, duplicate `id`, unknown `auth.method`, and each endpoint's scheme
and address range. Minting the provisioning token and reading mTLS certificates
happen on first use, since building an authenticator is async and the config
bootstrap is not.

## Two things to know

**The portal's record of each key is still in memory.** There is no `OAUTH2_KEY`
table yet, so `oauth2KeyService.js` keeps the `keyId` → key manager / consumer key
/ registration-credentials mapping in a per-process `Map`. The OAuth applications
themselves persist at the key manager; the portal's record of them does not
survive a restart, which orphans them. See the note at the top of that file.

**Config entries and `key_managers` rows share one id space, and are merged on
read.** A key manager is declared in one of two places and never both: an
`[[api_portal.key_manager]]` entry here, or a row in `key_managers` created
through `POST /key-managers` / the UI. Nothing is copied between them — config
stays in the file, and the database holds only what the API created.

`src/services/keyManagerRegistry.js` is the one resolver that reads both and
merges them behind the handle (`id`). `GET /key-managers` returns the union, each
item tagged `source: "config" | "api"`, and `GET /key-managers/{kmId}` resolves
against either. The mutating operations only reach `api` entries: `PUT` and
`DELETE` on a config-declared id answer `409`, and so does creating or renaming a
row onto one — a row under a config-declared handle would be written and then
shadowed on every read, since config wins the tie-break.

Only config-declared entries can register clients. A `key_managers` row carries a
token endpoint and nothing else — no driver `type`, no registration endpoint, no
provisioning credential — so `POST /oauth2-keys` cannot use one, and
`oauth2KeyService` resolves against the factory rather than through this registry.
Those rows serve the older `POST /applications/{id}/generate-keys` mapping flow,
which only records a `client_id` created out of band. Giving a UI-created key
manager real DCR capability needs somewhere to keep its driver config; that table
does not exist yet.

## Outbound request posture

Every call — the provisioning token request and the DCR calls alike — goes
through `core/httpClient.js`, which applies:

- the SSRF guard from `src/utils/ssrfGuard.js`. A key manager URL is
  operator-supplied rather than attacker-supplied, but `js-ssrf-prevention.md`
  covers "any admin-configurable endpoint": a mistyped endpoint pointed at
  `169.254.169.254` would otherwise hand a caller the instance metadata service.
  Link-local and cloud-metadata ranges are refused unconditionally;
  private/loopback needs `allow_private_endpoints = true`.
- the shared TLS tuning and connection pooling from
  `src/config/httpClientOptions.js`, the same as every other outbound client.
- `maxRedirects: 0`, a request timeout, and response/request size ceilings.

Address checking happens twice, and both are needed: at startup for endpoints
that are IP literals, and at dial time via the Agent's `lookup` hook for
hostnames. Node skips DNS for a literal, so the hook never sees one; and a
hostname can only be judged when it actually resolves, which is also what closes
the DNS-rebinding window.

A key manager's own error body is logged **in full** (bounded to 2048 chars)
under a tracking id — it is where the diagnosis actually lives, and which field
carries it differs per vendor, so it is not field-filtered. Credentials the
portal sent (the configured client secret, a client's registration access token)
are redacted from it first, since some servers echo a submitted parameter back
in an error; they are otherwise never logged and never returned.

The client gets none of that: every server-side cause — unreachable, rejected
provisioning credential, upstream 5xx, rate limit — answers with one identical
generic message plus the tracking id, so a caller cannot use error text to probe
the deployment. Only caller-caused failures (rejected client metadata, unknown
key, conflict) get a specific message, because that specificity is about the
request rather than about the portal.
