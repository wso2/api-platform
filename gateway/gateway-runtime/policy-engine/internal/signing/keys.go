/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.com) All Rights Reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 */

package signing

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"

	sdksigning "github.com/wso2/api-platform/sdk/core/signing"
)

// Algorithm is the SDK's JOSE "alg" type; the engine produces only its values.
type Algorithm = sdksigning.Algorithm

const (
	RS256      = sdksigning.RS256
	PS256      = sdksigning.PS256
	ES256      = sdksigning.ES256
	ES384      = sdksigning.ES384
	minRSABits = 2048
)

// signingKey is a parsed private key bound to the one algorithm it signs with.
type signingKey struct {
	alg Algorithm
	// signer is *rsa.PrivateKey or *ecdsa.PrivateKey.
	signer crypto.Signer
}

// parsePrivateKeyPEM accepts the encodings operators actually have on disk:
// PKCS #1 RSA, SEC 1 EC, and PKCS #8 for RSA and EC.
func parsePrivateKeyPEM(data []byte) (crypto.Signer, error) {
	block, rest := pem.Decode(data)
	if block == nil {
		return nil, errors.New("no PEM block found")
	}
	if extra, _ := pem.Decode(rest); extra != nil {
		return nil, errors.New("more than one PEM block found; supply exactly one private key")
	}

	switch block.Type {
	case "RSA PRIVATE KEY":
		return x509.ParsePKCS1PrivateKey(block.Bytes)
	case "EC PRIVATE KEY":
		return x509.ParseECPrivateKey(block.Bytes)
	case "PRIVATE KEY":
		parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, err
		}
		signer, ok := parsed.(crypto.Signer)
		if !ok {
			return nil, fmt.Errorf("unsupported PKCS #8 key type %T", parsed)
		}
		return signer, nil
	default:
		return nil, fmt.Errorf("unsupported PEM block type %q", block.Type)
	}
}

// resolveAlgorithm returns the algorithm a key signs with: the one its type
// implies, or an explicit override that the key type can actually honour.
func resolveAlgorithm(signer crypto.Signer, override Algorithm) (Algorithm, error) {
	var inferred Algorithm
	allowed := map[Algorithm]bool{}

	switch k := signer.(type) {
	case *rsa.PrivateKey:
		if k.N.BitLen() < minRSABits {
			return "", fmt.Errorf("RSA key is %d bits; at least %d are required", k.N.BitLen(), minRSABits)
		}
		inferred = RS256
		allowed[RS256], allowed[PS256] = true, true
	case *ecdsa.PrivateKey:
		switch k.Curve {
		case elliptic.P256():
			inferred = ES256
		case elliptic.P384():
			inferred = ES384
		default:
			return "", fmt.Errorf("unsupported EC curve %s", k.Curve.Params().Name)
		}
		allowed[inferred] = true
	default:
		return "", fmt.Errorf("unsupported key type %T", signer)
	}

	if override == "" {
		return inferred, nil
	}
	if !allowed[override] {
		return "", fmt.Errorf("algorithm %q cannot be used with this key", override)
	}
	return override, nil
}

// sign produces the raw JWS signature bytes for data (RFC 7518 §3, and the
// fixed-width r||s encoding for ECDSA).
func (k *signingKey) sign(data []byte) ([]byte, error) {
	switch k.alg {
	case RS256:
		digest := sha256.Sum256(data)
		return rsa.SignPKCS1v15(rand.Reader, k.signer.(*rsa.PrivateKey), crypto.SHA256, digest[:])
	case PS256:
		digest := sha256.Sum256(data)
		return rsa.SignPSS(rand.Reader, k.signer.(*rsa.PrivateKey), crypto.SHA256, digest[:],
			&rsa.PSSOptions{SaltLength: rsa.PSSSaltLengthEqualsHash})
	case ES256:
		digest := sha256.Sum256(data)
		return signECDSA(k.signer.(*ecdsa.PrivateKey), digest[:], 32)
	case ES384:
		digest := sha512.Sum384(data)
		return signECDSA(k.signer.(*ecdsa.PrivateKey), digest[:], 48)
	default:
		return nil, fmt.Errorf("unsupported algorithm %q", k.alg)
	}
}

func signECDSA(key *ecdsa.PrivateKey, digest []byte, size int) ([]byte, error) {
	r, s, err := ecdsa.Sign(rand.Reader, key, digest)
	if err != nil {
		return nil, err
	}
	out := make([]byte, 2*size)
	r.FillBytes(out[:size])
	s.FillBytes(out[size:])
	return out, nil
}
