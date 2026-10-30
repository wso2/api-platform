# Gateway signing keys for integration tests

`active.pem` (EC P-256) and `previous.pem` (RSA 2048) are fixed keys for the integration suite only.
A block opts in with the `signingKeys: true` Platform Gateway wiring, and the
`gateway-signing.toml` overlay loads them as the gateway's active and verify-only signing keys.
The keys are fixed so that scenarios can assert the key IDs the gateway publishes at
`/.well-known/jwks`.

They are public test fixtures. Never use them to sign anything outside a test gateway, and never
configure them in a real deployment.
