# Rule: Go Post-Quantum Cryptography Standards

## Context & Scope

Apply this rule whenever writing, refactoring, or reviewing Go (`.go`) code that performs key exchange, digital signatures, encryption, or any operation relying on the hardness of integer factorisation or discrete-logarithm problems (RSA, ECDH, ECDSA, DH). The goal is to keep cryptographic primitives secure against a cryptographically relevant quantum computer (CRQC), following NIST SP 800-208 and FIPS 203 (ML-KEM), 204 (ML-DSA), 205 (SLH-DSA).

**PQC is optional-but-supported, not strictly mandated.** Many upstream/legacy backends this platform talks to (Envoy, third-party services, older client integrations, some current gateway data-plane builds) do not yet negotiate PQC ciphers/curves. Code and configuration must make enabling PQC/hybrid straightforward — and default to it wherever the peer is known to support it — but must not hard-fail, refuse to start, or drop interoperability when talking to a peer that only speaks classical algorithms. Treat "PQC-capable" as a configurable posture (an explicit enable flag / negotiated capability), not an unconditional requirement baked into every code path.

## Directives

1. **Quantum-vulnerable algorithms are a configurable fallback, not a ban.** New key-exchange/signing code must offer ML-KEM/ML-DSA-based (hybrid) operation as the default when the configuration enables PQC and the peer supports it. RSA, ECDH (any curve), ECDSA, Ed25519/Ed448, X448, or classic Diffie-Hellman remain acceptable *only* as an explicit, configuration-gated fallback for legacy/unupgraded backends that don't yet support PQC — never as the silent, unconfigured default for new code, and never introduced or extended with a `// TODO(pqc): migrate`-style comment as the only nod to migration. A code comment is not a remediation plan and must never be treated as satisfying this directive; the actual configuration surface (an enable flag, capability negotiation, or equivalent) must exist and be wired in. The one narrow exception for the PQC leg itself: X25519 may be used solely as the classical leg of the mandated X25519 + ML-KEM-768 hybrid construction in directive 3 — never standalone, never paired with any KEM other than ML-KEM-768/1024, and never as a substitute for it elsewhere. Existing classical-only code paths that don't yet offer a PQC/hybrid configuration option must be filed as a tracked issue in the team's issue tracker (not merely noted inline) with an owner and a migration deadline, and must gain that configuration option the next time the code is touched rather than being re-committed as classical-only. Symmetric primitives are the exception: AES-256, ChaCha20-Poly1305, and SHA-3/BLAKE3 stay quantum-safe at current sizes (Grover halves effective strength); prefer 256-bit over AES-128/SHA-256 for new code.
2. **Approved algorithm selection:**

   | Purpose | Standard | Algorithm | Go Package |
   |---|---|---|---|
   | Key Encapsulation | FIPS 203 | ML-KEM-768 | `github.com/cloudflare/circl/kem/mlkem/mlkem768` |
   | Digital Signatures | FIPS 204 | ML-DSA-65 (Dilithium3) | `github.com/cloudflare/circl/sign/dilithium/mode3` |
   | Hash-based Signatures | FIPS 205 | SLH-DSA-SHA2-128s | `github.com/cloudflare/circl/sign/sphincsplus` |
   | Symmetric Encryption | — | AES-256-GCM, ChaCha20-Poly1305 | `crypto/aes`, `golang.org/x/crypto/chacha20poly1305` |
   | Hashing | — | SHA-3-256/512, BLAKE3 | `golang.org/x/crypto/sha3` |

   Use `-768`/`mode3` (NIST Level 3) as the minimum; escalate to `-1024`/`mode5` for long-lived keys or high-assurance contexts.
3. **Hybrid classical + PQC as the configured default, with a documented classical fallback.** When PQC is enabled in configuration, combine X25519 + ML-KEM-768 (IETF RFC 9180 / NIST SP 800-227 pattern) so security degrades gracefully to classical if the PQC primitive is flawed, and to PQC if a CRQC appears. Don't deploy PQC standalone until the library is validated at v1.0+. For TLS, use Go 1.23+ `crypto/tls` with `tls.X25519MLKEM768` as the first `CurvePreferences` entry — list P-256/P-384 after it (not remove them outright) so a handshake with a peer that doesn't yet support the hybrid curve, such as current Envoy/legacy gateway builds, still succeeds rather than failing closed. The negotiated/effective cipher suite must be surfaced (config, logs, or a status field) so operators can tell whether a given connection actually ran PQC or fell back to classical.
4. **Key/ciphertext size awareness.** ML-KEM-768 public keys are 1184 B and ciphertexts 1088 B; ML-DSA-65 signatures are 3309 B (public key 1952 B). These do not fit RSA-sized `VARCHAR(512)`/`STRING` columns — size schema migrations for `BYTEA`/`BLOB`, and account for the size in JWT/HTTP payload budgets. Never truncate a PQC key or signature for storage convenience — truncation silently invalidates the cryptographic guarantee.
5. **Randomness and nonce safety.** Key generation must use `crypto/rand` exclusively — never `math/rand`, `time.Now().UnixNano()`, or a seeded PRNG. AES-GCM nonces (96-bit) must be freshly generated per encryption via `crypto/rand.Read` and never reused under the same key; rotate the key after 2³² encryptions. CIRCL's ML-KEM `EncapsulateTo` draws its own randomness from `crypto/rand` internally when passed a `nil` seed — don't supply external randomness unless the API requires it.
6. **No algorithm negotiation in sensitive paths.** Never accept the algorithm from the peer/token header in authentication or key-exchange flows — allowlist the exact expected identifiers and reject any deviation with a generic error (algorithm-confusion attacks apply to PQC negotiation too). In JWS/JWT, set `algorithms: ["ML-DSA-65"]` explicitly; never accept `"none"` or legacy `"RS256"`.

## Example

```go
// BAD: PQC used standalone (no classical hybrid leg), or plain ECDH hardcoded
// with no configuration to enable PQC/hybrid at all — a "// TODO(pqc): migrate"
// comment does not excuse either of these.
pub, _, _ := mlkem768.GenerateKeyPair(rand.Reader)
ct, ss := make([]byte, mlkem768.CiphertextSize), make([]byte, mlkem768.SharedKeySize)
pub.EncapsulateTo(ct, ss, nil) // no X25519 hybrid leg

func Encapsulate(recipientClassical *ecdh.PublicKey) ([]byte, error) {
    ephemeral, _ := ecdh.X25519().GenerateKey(rand.Reader)
    return ephemeral.ECDH(recipientClassical) // no PQC path exists — not configurable, not a legacy fallback
}

// GOOD: hybrid KEM — X25519 + ML-KEM-768 (FIPS 203), combined via SHA3-256 so
// security holds if either primitive alone is broken — used when cfg.PQCEnabled
// and the peer advertises PQC support; recipientPQC is nil for a legacy peer,
// in which case the classical-only path below is the documented, config-gated
// fallback rather than a silent default.
func Encapsulate(cfg CryptoConfig, recipientClassical *ecdh.PublicKey, recipientPQC *mlkem768.PublicKey) (HybridKEMCiphertext, []byte, error) {
    ephemeral, err := ecdh.X25519().GenerateKey(rand.Reader) // crypto/rand only
    if err != nil {
        return HybridKEMCiphertext{}, nil, fmt.Errorf("x25519 keygen: %w", err)
    }
    classicalShared, err := ephemeral.ECDH(recipientClassical)
    if err != nil {
        return HybridKEMCiphertext{}, nil, fmt.Errorf("x25519 ecdh: %w", err)
    }
    if !cfg.PQCEnabled || recipientPQC == nil {
        return HybridKEMCiphertext{ClassicalECDHPublic: ephemeral.PublicKey().Bytes()}, classicalShared, nil
    }

    // EncapsulateTo writes into caller-provided buffers; seed=nil draws
    // randomness from crypto/rand internally (FIPS 203).
    pqcCT := make([]byte, mlkem768.CiphertextSize)
    pqcShared := make([]byte, mlkem768.SharedKeySize)
    recipientPQC.EncapsulateTo(pqcCT, pqcShared, nil)

    // Bind all inputs — prevents an active attacker downgrading to one leg.
    h := sha3.New256()
    h.Write(classicalShared)
    h.Write(pqcShared)
    h.Write(ephemeral.PublicKey().Bytes())
    h.Write(pqcCT)

    return HybridKEMCiphertext{ClassicalECDHPublic: ephemeral.PublicKey().Bytes(), PQCCiphertext: pqcCT}, h.Sum(nil), nil
}
```

> **Verification Checklist before outputting code:**
> * Does any new key exchange use RSA/ECDH/ECDSA/Ed25519/Ed448/X448/classic-DH with no configuration option to enable PQC/hybrid at all, or use X25519 outside its role as the classical leg of the mandated X25519+ML-KEM-768 hybrid (directive 3) — e.g. standalone, or paired with a non-ML-KEM KEM — or try to justify either with an inline `// TODO(pqc)`-style comment instead of a tracked issue and an actual configuration option?
> * When PQC is enabled and the peer supports it, is the chosen algorithm/package/security-level drawn from the approved table (directive 2)?
> * Is the PQC KEM used standalone rather than hybrid X25519+ML-KEM-768 when PQC is enabled?
> * Does a classical-only code path exist with no way to enable PQC/hybrid, instead of a config-gated fallback for legacy peers?
> * Are key/ciphertext/signature byte sizes accounted for in DB column types and payload budgets (never `VARCHAR`)?
> * Does any nonce or key generation use a non-`crypto/rand` source, or reuse an AES-GCM nonce?
> * Does anything accept the algorithm from the peer/token header instead of an explicit allowlist? (Operator-configured PQC-enablement is fine; trusting a peer's self-asserted algorithm choice is not.)
