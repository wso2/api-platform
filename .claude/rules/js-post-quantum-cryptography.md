# Rule: JavaScript (Node.js/Express) Post-Quantum Cryptography Standards

## Context & Scope

Apply this rule whenever writing, refactoring, or reviewing JavaScript (`.js`) code in `portals/developer-portal` that performs key exchange, digital signatures, encryption, or any operation relying on the hardness of integer factorisation or discrete-logarithm problems (RSA, ECDH, ECDSA, Node.js built-in `crypto.generateKeyPair` with classic algorithms). The goal is to ensure cryptographic primitives remain secure against adversaries with a cryptographically relevant quantum computer (CRQC), following NIST FIPS 203 (ML-KEM / Kyber), FIPS 204 (ML-DSA / Dilithium), and FIPS 205 (SLH-DSA / SPHINCS+). This is the JavaScript counterpart to `post-quantum-cryptography.md` (Go).

---

## Directives

### 1. Prohibited Quantum-Vulnerable Algorithms

* **Never use for new code:** RSA (any key size), ECDH, ECDSA, Ed25519/Ed448, X25519, or classic Diffie-Hellman in new key-exchange or signing code paths.
* **Existing code:** Annotate any remaining quantum-vulnerable call with `// TODO(pqc): migrate` and open a tracking issue. Do not leave undocumented quantum-vulnerable cryptography.
* **Symmetric exception:** AES-256-GCM, ChaCha20-Poly1305, and SHA-3 / BLAKE3 are considered quantum-safe at their current sizes. Prefer 256-bit variants; avoid AES-128 and SHA-256 for new long-lived keys or digests.
* **`node:crypto` built-ins:** `crypto.generateKeyPair('rsa', ...)`, `crypto.createECDH(...)`, `crypto.sign` with `'RSA-SHA256'` are all quantum-vulnerable — do not introduce them in new paths.

### 2. Approved Algorithm Selection

| Purpose | NIST Standard | Algorithm | npm Package |
|---|---|---|---|
| Key Encapsulation (KEM) | FIPS 203 | ML-KEM-768 (Kyber-768) | `liboqs-node` or `@noble/post-quantum` (kyber768) |
| Digital Signatures | FIPS 204 | ML-DSA-65 (Dilithium3) | `liboqs-node` or `@noble/post-quantum` (dilithium3) |
| Hash-based Signatures | FIPS 205 | SLH-DSA-SHA2-128s | `liboqs-node` |
| Symmetric Encryption | — | AES-256-GCM, ChaCha20-Poly1305 | `node:crypto` |
| Hashing | — | SHA3-256 / SHA3-512 | `node:crypto` (`sha3-256`), `@noble/hashes` |

Prefer `@noble/post-quantum` for pure-JS environments (no native bindings, audited by security researchers). Use `liboqs-node` when FIPS 140-3 validation or HSM integration is required.

Use security level `-768` / `dilithium3` (NIST Level 3) as minimum. Escalate to `-1024` / `dilithium5` for long-lived keys or high-assurance contexts.

### 3. Hybrid Classical + PQC (Transition Period)

* **Mandate Hybrid KEM** during the transition: combine X25519 + ML-KEM-768 so that security degrades gracefully to classical if the PQC primitive is found to have a flaw, and to PQC if a CRQC appears (IETF RFC 9180 pattern).
* **Do not deploy PQC standalone** until the chosen library has reached a stable 1.x release with a public security audit.
* **TLS:** Node.js 22+ with OpenSSL 3.2+ supports `X25519MLKEM768` via `tls.createServer({ ecdhCurve: 'X25519MLKEM768:X25519' })`. List the hybrid curve first.

### 4. Key and Ciphertext Size Awareness

* ML-KEM-768 public keys are **1184 bytes** and ciphertexts are **1088 bytes**. Do not store them in database columns or Sequelize `STRING` / `VARCHAR(512)` fields sized for RSA keys. Use `BLOB` / `BYTEA` or `TEXT` (base64).
* ML-DSA-65 signatures are **3309 bytes**. JWTs or HTTP headers embedding PQC signatures must account for this — avoid setting PQC signatures in `Authorization` headers where size limits apply; use the request body instead.
* Never truncate PQC keys or signatures for storage convenience.

### 5. Randomness and Nonce Safety

* Key generation must use `crypto.randomBytes` (Node.js built-in `node:crypto`) — never `Math.random()`, `Date.now()`, or third-party PRNGs without a `crypto/rand` equivalent.
* AES-256-GCM nonces (12 bytes / 96 bits) must be freshly generated per encryption using `crypto.randomBytes(12)`. Never reuse a nonce under the same key. Rotate the key after 2³² encryptions.
* For `@noble/post-quantum` KEM: `kyber768.encapsulate(recipientPublicKey)` generates its own randomness internally from `crypto.getRandomValues`; do not supply external randomness unless the API explicitly requires it.

### 6. No Algorithm Negotiation in Sensitive Paths

* **Never accept the algorithm from a JWT header or request payload** in authentication or key-exchange flows. Allowlist exact algorithm identifiers and reject any deviation with a generic `401 Unauthorized`.
* In `jose` JWS/JWT verification, always pass `algorithms: ['ML-DSA-65']` (or the IANA-registered codepoint when standardised). Never accept `'none'` or legacy `'RS256'`.

---

## Code Examples for Enforcement

### ❌ Anti-Pattern (What to Reject)

```js
// BAD: ECDH key exchange — quantum-vulnerable
const ecdh = crypto.createECDH('prime256v1');
ecdh.generateKeys();
const sharedSecret = ecdh.computeSecret(peerPublicKey); // Broken by Shor's algorithm

// BAD: RSA encryption — quantum-vulnerable
const { publicKey, privateKey } = crypto.generateKeyPairSync('rsa', { modulusLength: 4096 });
const ciphertext = crypto.publicEncrypt(publicKey, plaintext);

// BAD: PQC KEM used without classical hybrid
const { cipherText, sharedSecret } = kyber768.encapsulate(recipientPub);
// No X25519 leg — no fallback if ML-KEM is broken

// BAD: AES-128 for long-lived keys
const key = crypto.randomBytes(16); // AES-128 — only ~80-bit post-quantum security

// BAD: Nonce reuse
const FIXED_NONCE = Buffer.alloc(12, 0); // Never reuse — nonce collision breaks AES-GCM
const cipher = crypto.createCipheriv('aes-256-gcm', key, FIXED_NONCE);

// BAD: Undocumented legacy algorithm
function signLegacy(data, privateKey) {
    return crypto.sign('SHA256', data, privateKey); // No TODO(pqc) — silently quantum-vulnerable
}
```

### Best Practice (What to Generate)

```js
// crypto/hybridKem.js
// Hybrid KEM: X25519 + ML-KEM-768 (NIST FIPS 203).
// Security holds if either primitive is unbroken.

const { x25519 } = require('@noble/curves/ed25519'); // classical leg
const { ml_kem768 } = require('@noble/post-quantum/ml-kem'); // PQC leg — FIPS 203
const { sha3_256 } = require('@noble/hashes/sha3');
const crypto = require('node:crypto');

/**
 * Encapsulate a shared secret for a recipient.
 * @param {Uint8Array} recipientClassicalPub  - 32-byte X25519 public key
 * @param {Uint8Array} recipientPqcPub        - 1184-byte ML-KEM-768 public key
 * @returns {{ ciphertext: { classical: Uint8Array, pqc: Uint8Array }, sharedSecret: Uint8Array }}
 */
function encapsulate(recipientClassicalPub, recipientPqcPub) {
    // Classical leg: ephemeral X25519
    const ephemeralPriv = x25519.utils.randomPrivateKey(); // uses crypto.getRandomValues internally
    const ephemeralPub = x25519.getPublicKey(ephemeralPriv);
    const classicalShared = x25519.getSharedSecret(ephemeralPriv, recipientClassicalPub);

    // PQC leg: ML-KEM-768 (FIPS 203)
    const { cipherText: pqcCT, sharedSecret: pqcShared } = ml_kem768.encapsulate(recipientPqcPub);

    // Combine: SHA3-256(classicalShared || pqcShared || ephemeralPub || pqcCT)
    // Binding all inputs prevents downgrade to one leg by an active attacker.
    const combined = sha3_256(
        Buffer.concat([classicalShared, pqcShared, ephemeralPub, pqcCT])
    );

    return {
        ciphertext: { classical: ephemeralPub, pqc: pqcCT },
        sharedSecret: combined,
    };
}

/**
 * Decapsulate a shared secret from a received ciphertext.
 */
function decapsulate(ciphertext, classicalPriv, pqcPriv) {
    const classicalShared = x25519.getSharedSecret(classicalPriv, ciphertext.classical);
    const pqcShared = ml_kem768.decapsulate(ciphertext.pqc, pqcPriv);

    return sha3_256(
        Buffer.concat([classicalShared, pqcShared, ciphertext.classical, ciphertext.pqc])
    );
}

module.exports = { encapsulate, decapsulate };
```

```js
// crypto/pqcSign.js
// ML-DSA-65 (NIST FIPS 204 / Dilithium3) digital signatures.
// Signature size: 3309 bytes. Public key: 1952 bytes.

const { ml_dsa65 } = require('@noble/post-quantum/ml-dsa'); // FIPS 204

function generateSigningKeypair() {
    return ml_dsa65.keygen(); // { publicKey: Uint8Array(1952), secretKey: Uint8Array(...) }
}

function sign(message, secretKey) {
    return ml_dsa65.sign(secretKey, message); // Uint8Array — 3309 bytes
}

function verify(message, signature, publicKey) {
    const valid = ml_dsa65.verify(publicKey, message, signature);
    if (!valid) {
        throw Object.assign(new Error('Signature verification failed'), { statusCode: 401 });
    }
}

module.exports = { generateSigningKeypair, sign, verify };
```

```js
// crypto/symmetricEncryption.js
// AES-256-GCM with fresh per-message nonce. Quantum-safe at 256-bit key size.

const crypto = require('node:crypto');

const KEY_BYTES = 32; // AES-256 — 128-bit post-quantum security (Grover halves strength)
const NONCE_BYTES = 12; // 96-bit GCM nonce — must be unique per (key, message) pair

function generateKey() {
    return crypto.randomBytes(KEY_BYTES); // Never use Math.random() or Date.now()
}

function encrypt(plaintext, key) {
    const nonce = crypto.randomBytes(NONCE_BYTES); // Fresh per encryption
    const cipher = crypto.createCipheriv('aes-256-gcm', key, nonce);
    const ciphertext = Buffer.concat([cipher.update(plaintext), cipher.final()]);
    const authTag = cipher.getAuthTag(); // 16-byte GCM authentication tag
    return Buffer.concat([nonce, authTag, ciphertext]); // Prepend nonce and tag for decryption
}

function decrypt(payload, key) {
    const nonce = payload.subarray(0, NONCE_BYTES);
    const authTag = payload.subarray(NONCE_BYTES, NONCE_BYTES + 16);
    const ciphertext = payload.subarray(NONCE_BYTES + 16);
    const decipher = crypto.createDecipheriv('aes-256-gcm', key, nonce);
    decipher.setAuthTag(authTag);
    return Buffer.concat([decipher.update(ciphertext), decipher.final()]);
}

module.exports = { generateKey, encrypt, decrypt };
```

```js
// tls/tlsConfig.js
// GOOD: TLS 1.3 with hybrid X25519+ML-KEM-768 (Node.js 22+ / OpenSSL 3.2+).

const tls = require('node:tls');
const fs = require('node:fs');

function createPqcTlsServer(requestHandler) {
    return tls.createServer(
        {
            key: fs.readFileSync(process.env.TLS_KEY_PATH),
            cert: fs.readFileSync(process.env.TLS_CERT_PATH),
            minVersion: 'TLSv1.3',
            // X25519MLKEM768 is the hybrid PQC key share (FIPS 203 + X25519).
            // Listed first so it is offered as the preferred key share in ClientHello.
            // X25519 is a classical fallback for peers without PQC support.
            ecdhCurve: 'X25519MLKEM768:X25519',
        },
        requestHandler
    );
}

module.exports = { createPqcTlsServer };
```

```js
// db/migrations/20260706_pqc_key_columns.js
// GOOD: Sequelize migration sized for PQC artifacts.
// ML-KEM-768 public key = 1184 bytes, ciphertext = 1088 bytes.
// ML-DSA-65  public key = 1952 bytes, signature   = 3309 bytes.
// Use DataTypes.BLOB or TEXT (base64) — never STRING(512) sized for RSA.

module.exports = {
    up: async (queryInterface, Sequelize) => {
        await queryInterface.addColumn('session_keys', 'pqc_public_key', {
            type: Sequelize.DataTypes.BLOB, // ML-KEM-768 public key (1184 B)
            allowNull: true,
        });
        await queryInterface.addColumn('session_keys', 'pqc_ciphertext', {
            type: Sequelize.DataTypes.BLOB, // ML-KEM-768 ciphertext (1088 B)
            allowNull: true,
        });
        await queryInterface.addColumn('api_signing_keys', 'pqc_public_key', {
            type: Sequelize.DataTypes.BLOB, // ML-DSA-65 public key (1952 B)
            allowNull: true,
        });
        // TODO(pqc): remove legacy rsa_public_key column after all clients migrate
    },
    down: async (queryInterface) => {
        await queryInterface.removeColumn('session_keys', 'pqc_public_key');
        await queryInterface.removeColumn('session_keys', 'pqc_ciphertext');
        await queryInterface.removeColumn('api_signing_keys', 'pqc_public_key');
    },
};
```

---

> **Verification Checklist before outputting code:**
> * Does any new key exchange use `crypto.createECDH`, `generateKeyPair('rsa', ...)`, or `x25519` without a `// TODO(pqc): migrate` comment? (If yes, migrate to hybrid KEM or add the comment with a tracking issue.)
> * Is the PQC KEM used in isolation (no X25519 leg)? (If yes, add a hybrid X25519+ML-KEM-768 construction.)
> * Are key, ciphertext, or signature sizes accounted for in Sequelize model definitions and HTTP payload budgets? (ML-KEM-768 CT = 1088 B, ML-DSA-65 sig = 3309 B — never `STRING(512)`.)
> * Does any nonce or key generation use `Math.random()`, `Date.now()`, or a non-`crypto.randomBytes` source? (If yes, replace with `crypto.randomBytes`.)
> * Does TLS configuration include `X25519MLKEM768` as the first entry in `ecdhCurve`? (If no, add it for Node.js 22+ services.)
> * Does any JWT/JWS verification in `jose` accept the algorithm from the token header without an explicit allowlist? (If yes, add `algorithms: ['ML-DSA-65']` or the appropriate asymmetric-only list.)
