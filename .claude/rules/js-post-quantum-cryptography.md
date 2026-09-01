# Rule: JavaScript (Node.js/Express) Post-Quantum Cryptography Standards

## Context & Scope

Apply this rule whenever writing, refactoring, or reviewing JavaScript (`.js`) code in `portals/developer-portal` that performs key exchange, digital signatures, encryption, or any operation relying on the hardness of integer factorisation or discrete-logarithm problems (RSA, ECDH, ECDSA, `crypto.generateKeyPair` with classic algorithms). Cryptographic primitives must remain secure against an adversary with a cryptographically relevant quantum computer, per NIST FIPS 203 (ML-KEM), FIPS 204 (ML-DSA), and FIPS 205 (SLH-DSA). JS counterpart to `post-quantum-cryptography.md` (Go).

**PQC is optional-but-supported, not strictly mandated.** Backends this portal talks to (legacy gateway builds, third-party integrations, older IDPs) do not all negotiate PQC ciphers/curves yet. Configuration must make enabling PQC/hybrid easy — and default to it wherever the peer is known to support it — but code must not hard-fail or drop interoperability when talking to a peer that only speaks classical algorithms. Treat "PQC-capable" as a configurable posture, not an unconditional requirement in every code path.

## Directives

1. **Quantum-vulnerable algorithms are a configurable fallback, not a ban.** New key-exchange/signing code must offer ML-KEM/ML-DSA-based (hybrid) operation as the default when configuration enables PQC and the peer supports it. RSA, ECDH (any curve), ECDSA, Ed25519/Ed448, X448, or classic Diffie-Hellman — including `crypto.createECDH(...)`, `crypto.generateKeyPair('rsa', ...)`, and `crypto.sign` with `'RSA-SHA256'` — remain acceptable *only* as an explicit, configuration-gated fallback for legacy backends that don't yet support PQC, never as the silent, unconfigured default for new code, and never introduced or extended with a `// TODO(pqc): migrate`-style comment as the only nod to migration; a code comment is not a remediation plan. The one narrow exception for the PQC leg itself: X25519 may be used solely as the classical leg of the mandated X25519 + ML-KEM-768 hybrid construction in directive 3 — never standalone, never paired with any KEM other than ML-KEM-768/1024, and never as a substitute for it elsewhere. Existing classical-only uses that don't yet offer a PQC/hybrid configuration option must be filed as a tracked issue (not merely noted inline) with an owner and a migration deadline, and must gain that configuration option the next time that code is touched rather than being re-committed as classical-only. AES-256-GCM, ChaCha20-Poly1305, and SHA-3/BLAKE3 remain quantum-safe exceptions at 256-bit sizes; avoid AES-128/SHA-256 for new long-lived keys.
2. **Approved algorithm selection:**

   | Purpose | NIST Standard | Algorithm | npm Package |
   |---|---|---|---|
   | Key Encapsulation | FIPS 203 | ML-KEM-768 (Kyber-768) | `liboqs-node` or `@noble/post-quantum` (kyber768) |
   | Digital Signatures | FIPS 204 | ML-DSA-65 (Dilithium3) | `liboqs-node` or `@noble/post-quantum` (dilithium3) |
   | Hash-based Signatures | FIPS 205 | SLH-DSA-SHA2-128s | `liboqs-node` |
   | Symmetric Encryption | — | AES-256-GCM, ChaCha20-Poly1305 | `node:crypto` |
   | Hashing | — | SHA3-256 / SHA3-512 | `node:crypto`, `@noble/hashes` |

   Prefer `@noble/post-quantum` for pure-JS (no native bindings, audited); use `liboqs-node` when FIPS 140-3 or HSM integration is required. Use `-768`/`dilithium3` (NIST Level 3) as the minimum, escalating to `-1024`/`dilithium5` for long-lived or high-assurance keys.
3. **Hybrid classical + PQC as the configured default, with a documented classical fallback.** When PQC is enabled in configuration, combine X25519 + ML-KEM-768 (IETF RFC 9180 pattern) so security degrades gracefully to whichever primitive remains unbroken — never deploy PQC standalone until the library has a stable 1.x release with a public audit. For TLS, Node.js 22+/OpenSSL 3.2+ supports `tls.createServer({ ecdhCurve: 'X25519MLKEM768:X25519' })` — list the hybrid curve first, keeping `X25519` (and other configured classical curves) after it so a handshake with a peer that doesn't yet support the hybrid curve still succeeds instead of failing closed. Surface the negotiated/effective curve (config, logs, or a status field) so operators can tell whether a connection actually ran PQC or fell back to classical.
4. **Key and ciphertext size awareness.** ML-KEM-768 public keys are 1184 bytes and ciphertexts 1088 bytes; ML-DSA-65 signatures are 3309 bytes. Never store these in Sequelize `STRING`/`VARCHAR(512)` columns sized for RSA — use `BLOB`/`BYTEA` or `TEXT` (base64). Avoid putting PQC signatures in `Authorization` headers where size limits apply — use the request body instead. Never truncate a PQC key or signature for storage convenience.
5. **Randomness and nonce safety.** Key generation must use `crypto.randomBytes` — never `Math.random()`, `Date.now()`, or a non-CSPRNG. AES-256-GCM nonces (12 bytes) must be freshly generated per encryption via `crypto.randomBytes(12)` and never reused under the same key; rotate the key after 2³² encryptions. `@noble/post-quantum`'s `kyber768.encapsulate(...)` generates its own randomness internally — don't supply external randomness unless the API requires it.
6. **No algorithm negotiation in sensitive paths.** Never accept the algorithm from a JWT header or request payload in auth/key-exchange flows — allowlist exact identifiers and reject deviation with a generic `401`. In `jose` JWS/JWT verification, always pass an explicit `algorithms: ['ML-DSA-65']` (or the IANA codepoint once standardised); never accept `'none'` or legacy `'RS256'`.

## Example

```js
// BAD: classical-only key exchange with no configuration option to enable PQC at
// all, and (separately) a standalone PQC KEM with no hybrid classical leg.
const ecdh = crypto.createECDH('prime256v1');
const sharedSecret = ecdh.computeSecret(peerPublicKey); // quantum-vulnerable, not configurable — a TODO(pqc) comment would not excuse this
const { sharedSecret: pqcOnly } = ml_kem768.encapsulate(recipientPub); // no X25519 hybrid leg

// GOOD: hybrid X25519 + ML-KEM-768 (FIPS 203) when config.pqcEnabled and the
// recipient advertises PQC support — security holds if either leg is unbroken;
// all inputs bound into the combiner to prevent downgrade. When PQC isn't
// enabled or the recipient is a legacy peer (recipientPqcPub is undefined),
// falls back to the classical-only leg rather than failing closed.
const { x25519 } = require('@noble/curves/ed25519');
const { ml_kem768 } = require('@noble/post-quantum/ml-kem');
const { sha3_256 } = require('@noble/hashes/sha3');

function encapsulate(config, recipientClassicalPub, recipientPqcPub) {
    const ephemeralPriv = x25519.utils.randomPrivateKey(); // crypto.getRandomValues internally
    const ephemeralPub = x25519.getPublicKey(ephemeralPriv);
    const classicalShared = x25519.getSharedSecret(ephemeralPriv, recipientClassicalPub);

    if (!config.pqcEnabled || !recipientPqcPub) {
        return { ciphertext: { classical: ephemeralPub }, sharedSecret: classicalShared }; // documented, config-gated fallback
    }

    const { cipherText: pqcCT, sharedSecret: pqcShared } = ml_kem768.encapsulate(recipientPqcPub);

    const combined = sha3_256(
        Buffer.concat([classicalShared, pqcShared, ephemeralPub, pqcCT]) // binds both legs
    );
    return { ciphertext: { classical: ephemeralPub, pqc: pqcCT }, sharedSecret: combined };
}
```

> **Verification Checklist before outputting code:**
> * Does any RSA/ECDH/ECDSA/Ed25519/Ed448/X448/classic-DH use have no configuration option to enable PQC/hybrid at all, or is X25519 used outside its role as the classical leg of the mandated X25519+ML-KEM-768 hybrid (directive 3) — e.g. standalone, or paired with a non-ML-KEM KEM — or is any of this "justified" by an inline `// TODO(pqc)`-style comment instead of a tracked issue and an actual configuration option?
> * When PQC is enabled and the peer supports it, is the PQC KEM used as hybrid X25519+ML-KEM-768 rather than standalone?
> * Does a classical-only code path exist with no way to enable PQC/hybrid, instead of a config-gated fallback for legacy peers?
> * Are ML-KEM/ML-DSA key/ciphertext/signature sizes accounted for in Sequelize columns (`BLOB`, never `STRING(512)`) and payload budgets?
> * Any nonce/key generation using `Math.random()`/`Date.now()` instead of `crypto.randomBytes`, or a reused GCM nonce?
> * Does TLS config list `X25519MLKEM768` first in `ecdhCurve`, keeping a classical curve after it for legacy peers, for Node.js 22+ services?
> * Does any `jose` JWT/JWS verification omit an explicit `algorithms: ['ML-DSA-65']`-style allowlist? (Enabling a classical fallback via config is fine; accepting an algorithm the peer/token itself claims is not.)
