# Rule: Go Post-Quantum Cryptography Standards

## Context & Scope

Apply this rule whenever writing, refactoring, or reviewing Go (`.go`) code that performs key exchange, digital signatures, encryption, or any operation that relies on the hardness of integer factorisation or discrete-logarithm problems (RSA, ECDH, ECDSA, DH). The goal is to ensure cryptographic primitives remain secure against adversaries with access to a cryptographically relevant quantum computer (CRQC), following NIST SP 800-208 and the NIST PQC Round 4 / Final standards (FIPS 203 Kyber/ML-KEM, FIPS 204 Dilithium/ML-DSA, FIPS 205 SPHINCS+/SLH-DSA).

---

## Directives

### 1. Prohibited Quantum-Vulnerable Algorithms

* **Never use for new code:** RSA (any key size), ECDH, ECDSA, Ed25519/Ed448, X25519/X448, or classic Diffie-Hellman for key establishment or digital signatures in new code paths.
* **Existing code:** Mark any use of the above with a `// TODO(pqc): migrate` comment and open a tracking issue. Do not silently leave quantum-vulnerable code undocumented.
* **Symmetric exception:** AES-256, ChaCha20-Poly1305, and SHA-3/BLAKE3 are considered quantum-safe at their current key/digest sizes (Grover halves effective strength; 256-bit → 128-bit effective). AES-128 and SHA-256 are borderline — prefer 256-bit variants in new code.

### 2. Approved Algorithm Selection

| Purpose | NIST Standard | Algorithm | Go Package |
|---|---|---|---|
| Key Encapsulation (KEM) | FIPS 203 | ML-KEM-768 (Kyber-768) | `github.com/cloudflare/circl/kem/kyber/kyber768` |
| Digital Signatures | FIPS 204 | ML-DSA-65 (Dilithium3) | `github.com/cloudflare/circl/sign/dilithium/mode3` |
| Hash-based Signatures | FIPS 205 | SLH-DSA-SHA2-128s (SPHINCS+) | `github.com/cloudflare/circl/sign/sphincsplus` |
| Symmetric Encryption | — | AES-256-GCM, ChaCha20-Poly1305 | `crypto/aes`, `golang.org/x/crypto/chacha20poly1305` |
| Hashing | — | SHA-3-256 / SHA-3-512, BLAKE3 | `golang.org/x/crypto/sha3` |

Use security level `-768` / `mode3` (NIST Level 3) as the minimum. Escalate to `-1024` / `mode5` for long-lived keys or high-assurance contexts.

### 3. Hybrid Classical + PQC (Transition Period)

* **Mandate Hybrid KEM** during the transition: combine X25519 + ML-KEM-768 so that security degrades gracefully to classical if the PQC primitive is found to have a flaw, and to PQC if a CRQC appears. This is the pattern recommended by IETF RFC 9180 hybrid KEM and NIST SP 800-227.
* **Do not use PQC standalone** until your deployment has validated interoperability and library maturity at v1.0+.
* **TLS:** Prefer Go 1.23+ `crypto/tls` with `X25519MLKEM768` (`tls.X25519MLKEM768`) as the first key share in `CurvePreferences`. Remove P-256 and P-384 from the curve list for new services.

### 4. Key and Ciphertext Size Awareness

* ML-KEM-768 public keys are **1184 bytes** and ciphertexts are **1088 bytes** — do not store them in database columns sized for RSA public keys (typically 512 bytes for 4096-bit). Size schema migrations accordingly.
* ML-DSA-65 signatures are **3309 bytes** — JWT/JWS payloads that embed signatures must account for this. Avoid base64-encoding large PQC artifacts in HTTP headers.
* Never truncate PQC keys or signatures for storage convenience; truncation silently invalidates all cryptographic guarantees.

### 5. Randomness and Nonce Safety

* Key generation must use `crypto/rand` exclusively — never `math/rand`, `time.Now().UnixNano()`, or seeded PRNGs.
* AES-GCM nonces (96-bit) must be generated fresh per encryption with `crypto/rand.Read`; never reuse a nonce under the same key. After 2³² encryptions under one key, rotate the key.
* For ML-KEM: the `Encapsulate` function in CIRCL generates the randomness internally from `crypto/rand`; do not pass external randomness unless explicitly required by the API.

### 6. No Algorithm Negotiation in Sensitive Paths

* **Never accept the algorithm from the peer** in authentication or key-exchange flows. Allowlist the exact algorithm identifiers expected and reject any deviation with a generic error — algorithm confusion attacks apply equally to PQC negotiation.
* In JWS/JWT contexts, set `algorithms: ["ML-DSA-65"]` (or the registered IANA codepoint once standardised) explicitly; never accept `"alg": "none"` or legacy `"alg": "RS256"`.

---

## Code Examples for Enforcement

### ❌ Anti-Pattern (What to Reject)

```go
// BAD: quantum-vulnerable key exchange and signing, no PQC migration
priv, _ := ecdh.P256().GenerateKey(rand.Reader)
shared, _ := priv.ECDH(peerPub) // quantum-vulnerable, no TODO(pqc)

func signLegacy(data []byte, key *rsa.PrivateKey) []byte {
    sig, _ := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, data)
    return sig // undocumented legacy algorithm
}

// BAD: PQC used without classical hybrid, and weak symmetric key size
pub, _, _ := kyber768.GenerateKeyPair(rand.Reader)
ct, sharedSecret, _ := kyber768.Scheme().Encapsulate(pub) // no X25519 hybrid leg

key := make([]byte, 16) // AES-128 — reduced post-quantum security
```

### Best Practice (What to Generate)

```go
// crypto/pqc/hybrid_kem.go
// Hybrid KEM: X25519 + ML-KEM-768 (NIST FIPS 203)
// Both shared secrets are combined — security holds if either primitive is unbroken.

package pqc

import (
    "crypto/ecdh"
    "crypto/rand"
    "crypto/sha3"
    "fmt"

    "github.com/cloudflare/circl/kem/kyber/kyber768"
)

type HybridKEMCiphertext struct {
    ClassicalECDHPublic []byte // Ephemeral X25519 public key (32 bytes)
    PQCCiphertext       []byte // ML-KEM-768 ciphertext (1088 bytes)
}

// Encapsulate derives a shared secret for the given recipient hybrid public key.
// Returns (ciphertext, 32-byte shared secret, error).
func Encapsulate(recipientClassical *ecdh.PublicKey, recipientPQC *kyber768.PublicKey) (HybridKEMCiphertext, []byte, error) {
    // Classical leg: ephemeral X25519
    ephemeral, err := ecdh.X25519().GenerateKey(rand.Reader)
    if err != nil {
        return HybridKEMCiphertext{}, nil, fmt.Errorf("x25519 key generation failed: %w", err)
    }
    classicalShared, err := ephemeral.ECDH(recipientClassical)
    if err != nil {
        return HybridKEMCiphertext{}, nil, fmt.Errorf("x25519 ecdh failed: %w", err)
    }

    // PQC leg: ML-KEM-768 (FIPS 203)
    pqcCT, pqcShared, err := kyber768.Encapsulate(recipientPQC)
    if err != nil {
        return HybridKEMCiphertext{}, nil, fmt.Errorf("ml-kem encapsulation failed: %w", err)
    }

    // Combine: SHA3-256(classicalShared || pqcShared || classicalPublic || pqcCiphertext)
    // Binding all inputs prevents a downgrade to one leg by an active attacker.
    h := sha3.New256()
    h.Write(classicalShared)
    h.Write(pqcShared)
    h.Write(ephemeral.PublicKey().Bytes())
    h.Write(pqcCT)
    combined := h.Sum(nil)

    ct := HybridKEMCiphertext{
        ClassicalECDHPublic: ephemeral.PublicKey().Bytes(),
        PQCCiphertext:       pqcCT,
    }
    return ct, combined, nil
}

// Decapsulate recovers the shared secret from a ciphertext.
func Decapsulate(ct HybridKEMCiphertext, classicalPriv *ecdh.PrivateKey, pqcPriv *kyber768.PrivateKey) ([]byte, error) {
    ephemeralPub, err := ecdh.X25519().NewPublicKey(ct.ClassicalECDHPublic)
    if err != nil {
        return nil, fmt.Errorf("invalid classical public key in ciphertext: %w", err)
    }
    classicalShared, err := classicalPriv.ECDH(ephemeralPub)
    if err != nil {
        return nil, fmt.Errorf("x25519 decapsulation failed: %w", err)
    }

    pqcShared, err := kyber768.Decapsulate(pqcPriv, ct.PQCCiphertext)
    if err != nil {
        return nil, fmt.Errorf("ml-kem decapsulation failed: %w", err)
    }

    h := sha3.New256()
    h.Write(classicalShared)
    h.Write(pqcShared)
    h.Write(ct.ClassicalECDHPublic)
    h.Write(ct.PQCCiphertext)
    return h.Sum(nil), nil
}

// crypto/pqc/signing.go
// ML-DSA-65 (NIST FIPS 204 / Dilithium3) digital signatures.
// Signature size: 3309 bytes. Public key size: 1952 bytes.

package pqc

import (
    "crypto/rand"
    "fmt"

    mode3 "github.com/cloudflare/circl/sign/dilithium/mode3"
)

// Sign signs msg with the ML-DSA-65 private key.
func Sign(msg []byte, priv mode3.PrivateKey) ([]byte, error) {
    sig := mode3.Sign(&priv, msg)
    if sig == nil {
        return nil, fmt.Errorf("ml-dsa signing failed")
    }
    return sig, nil
}

// Verify verifies an ML-DSA-65 signature.
// Returns a typed error on failure — never returns nil error with invalid result.
func Verify(msg, sig []byte, pub mode3.PublicKey) error {
    if !mode3.Verify(&pub, msg, sig) {
        return fmt.Errorf("signature verification failed")
    }
    return nil
}

// tls/tls_config.go
// GOOD: TLS 1.3 with hybrid X25519+ML-KEM-768 key share (Go 1.23+).
// X25519MLKEM768 is listed first to ensure it is offered in ClientHello.

package tlsconfig

import (
    "crypto/tls"
)

func NewServerTLSConfig(cert tls.Certificate) *tls.Config {
    return &tls.Config{
        MinVersion: tls.VersionTLS13,
        CurvePreferences: []tls.CurveID{
            tls.X25519MLKEM768, // Hybrid PQC-safe key share (FIPS 203 + X25519)
            tls.X25519,         // Classical fallback for peers without PQC support
        },
        Certificates: []tls.Certificate{cert},
        // Cipher suites in TLS 1.3 are fixed; no further configuration needed.
    }
}

// db/migrations/20260706_pqc_key_columns.go
// GOOD: Schema sized for PQC artifacts.
// ML-KEM-768 public key = 1184 bytes, ciphertext = 1088 bytes.
// ML-DSA-65  public key = 1952 bytes, signature   = 3309 bytes.
// Use BYTEA (PostgreSQL) or BLOB — never VARCHAR(512) sized for RSA.

const MigrationPQCKeyColumns = `
ALTER TABLE session_keys
    ADD COLUMN IF NOT EXISTS pqc_public_key  BYTEA,  -- ML-KEM-768 public key (1184 B)
    ADD COLUMN IF NOT EXISTS pqc_ciphertext  BYTEA;  -- ML-KEM-768 ciphertext (1088 B)

ALTER TABLE api_signing_keys
    ADD COLUMN IF NOT EXISTS pqc_public_key  BYTEA,  -- ML-DSA-65 public key (1952 B)
    ADD COLUMN IF NOT EXISTS pqc_signature   BYTEA;  -- ML-DSA-65 signature (3309 B)
`

// TODO(pqc): migrate — existing RSA key columns (rsa_public_key VARCHAR(1024)) remain
// for backward-compat during transition. Remove after all clients are upgraded.
```

---

> **Verification Checklist before outputting code:**
> * Does any new key exchange use RSA, ECDH, X25519, or ECDSA without a `// TODO(pqc): migrate` comment? (If yes, migrate to hybrid KEM or add the comment with a tracking issue.)
> * Is the PQC KEM used in isolation (not hybrid)? (If yes, wrap it in a hybrid X25519+ML-KEM-768 construction.)
> * Are key, ciphertext, or signature byte sizes accounted for in database column types and HTTP payload budgets? (ML-KEM-768 CT = 1088 B, ML-DSA-65 sig = 3309 B — never VARCHAR.)
> * Does any nonce or key generation use a non-`crypto/rand` source? (If yes, replace with `crypto/rand.Read`.)
> * Does TLS configuration include `tls.X25519MLKEM768` as the first `CurvePreferences` entry? (If no, add it for Go 1.23+ services.)
> * Does any JWT/JWS verification accept the algorithm from the token header? (If yes, add an explicit asymmetric-only allowlist.)
