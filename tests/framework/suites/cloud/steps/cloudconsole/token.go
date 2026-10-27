/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except in compliance
 * with the License. You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package cloudconsole

import (
	"context"
	"crypto"
	"crypto/hmac"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

// Forgery kinds a scenario can ask for; each names a way a bearer token fails verification.
const (
	forgeExpired       = "that expired an hour ago"
	forgeUnsigned      = "with no signature"
	forgeUnknownKey    = "signed with an unknown key"
	forgeUnknownSecret = "signed with an unknown shared secret"
)

// organisationClaims are the token claims the control plane reads the caller's organisation
// from, in the order it tries them.
var organisationClaims = []string{"ouId", "organization"}

// decodeJWT splits a compact JWT and decodes its header and claims without verifying anything.
// Errors never include the token.
func decodeJWT(token string) (header, claims map[string]any, err error) {
	parts := strings.Split(strings.TrimSpace(token), ".")
	if len(parts) != 3 {
		return nil, nil, fmt.Errorf("a JWT has 3 dot-separated parts, got %d", len(parts))
	}
	if header, err = decodeJWTSegment(parts[0]); err != nil {
		return nil, nil, fmt.Errorf("decoding the JWT header: %w", err)
	}
	if claims, err = decodeJWTSegment(parts[1]); err != nil {
		return nil, nil, fmt.Errorf("decoding the JWT claims: %w", err)
	}
	return header, claims, nil
}

func decodeJWTSegment(segment string) (map[string]any, error) {
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(segment, "="))
	if err != nil {
		return nil, err
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return nil, err
	}
	return decoded, nil
}

// organisationClaim returns the first non-empty organisation claim of a token.
func organisationClaim(claims map[string]any) (name, value string, ok bool) {
	for _, claim := range organisationClaims {
		if text, isText := claims[claim].(string); isText && strings.TrimSpace(text) != "" {
			return claim, text, true
		}
	}
	return "", "", false
}

// tokenExpiry reads the exp claim as a time.
func tokenExpiry(claims map[string]any) (time.Time, bool) {
	exp, ok := claims["exp"].(float64)
	if !ok {
		return time.Time{}, false
	}
	return time.Unix(int64(exp), 0), true
}

func encodeJWTSegment(value map[string]any) (string, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func randomHex(size int) (string, error) {
	buffer := make([]byte, size)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return hex.EncodeToString(buffer), nil
}

// forgeToken builds a bearer token that the platform must refuse. The expired kind reuses the
// real token's claims and key id so only its lifetime and signature differ; the other kinds
// carry a random organisation, issuer and subject that no identity provider ever issued.
func forgeToken(kind string, realHeader, realClaims map[string]any, now time.Time) (string, error) {
	header := map[string]any{"typ": "JWT"}
	claims := map[string]any{}
	switch kind {
	case forgeExpired:
		for key, value := range realClaims {
			claims[key] = value
		}
		claims["iat"] = now.Add(-2 * time.Hour).Unix()
		claims["exp"] = now.Add(-1 * time.Hour).Unix()
		header["alg"] = "RS256"
		if keyID, ok := realHeader["kid"].(string); ok && keyID != "" {
			header["kid"] = keyID
		}
	case forgeUnsigned, forgeUnknownKey, forgeUnknownSecret:
		org, err := randomHex(16)
		if err != nil {
			return "", err
		}
		subject, err := randomHex(16)
		if err != nil {
			return "", err
		}
		claims["ouId"] = org
		claims["ouHandle"] = "forged-" + org[:8]
		claims["sub"] = subject
		claims["iss"] = "https://issuer.invalid"
		claims["iat"] = now.Unix()
		claims["exp"] = now.Add(time.Hour).Unix()
		switch kind {
		case forgeUnsigned:
			header["alg"] = "none"
		case forgeUnknownKey:
			header["alg"] = "RS256"
			header["kid"] = "unknown-" + org[:8]
		default:
			header["alg"] = "HS256"
		}
	default:
		return "", fmt.Errorf("unsupported token forgery %q", kind)
	}
	encodedHeader, err := encodeJWTSegment(header)
	if err != nil {
		return "", err
	}
	encodedClaims, err := encodeJWTSegment(claims)
	if err != nil {
		return "", err
	}
	signingInput := encodedHeader + "." + encodedClaims
	signature, err := signForgery(header["alg"].(string), signingInput)
	if err != nil {
		return "", err
	}
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(signature), nil
}

// signForgery signs with a key generated for this one token, so no verifier can know it.
func signForgery(alg, signingInput string) ([]byte, error) {
	switch alg {
	case "none":
		return nil, nil
	case "HS256":
		secret := make([]byte, 32)
		if _, err := rand.Read(secret); err != nil {
			return nil, err
		}
		mac := hmac.New(sha256.New, secret)
		mac.Write([]byte(signingInput))
		return mac.Sum(nil), nil
	case "RS256":
		key, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			return nil, err
		}
		digest := sha256.Sum256([]byte(signingInput))
		return rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	default:
		return nil, fmt.Errorf("unsupported signing algorithm %q", alg)
	}
}

// tokenParts returns the decoded header and claims of the console token in scenario state.
func tokenParts(ctx context.Context) (header, claims map[string]any, err error) {
	token, err := bearerToken(ctx)
	if err != nil {
		return nil, nil, err
	}
	return decodeJWT(token)
}

// assertTokenIsLiveJWT checks that the console token is a three-part JWT that has not expired
// and records which organisation it names.
func (s *Steps) assertTokenIsLiveJWT(ctx context.Context) error {
	_, claims, err := tokenParts(ctx)
	if err != nil {
		return fmt.Errorf("the console token is not a readable JWT: %w", err)
	}
	expiry, ok := tokenExpiry(claims)
	if !ok {
		return fmt.Errorf("the console token has no exp claim")
	}
	if !expiry.After(time.Now()) {
		return fmt.Errorf("the console token expired at %s", expiry.UTC().Format(time.RFC3339))
	}
	return nil
}

// assertTokenCarriesOrganisation checks that the console token names an organisation and logs
// which one, so a run reports the organisation it acted on instead of assuming it.
func (s *Steps) assertTokenCarriesOrganisation(ctx context.Context) error {
	_, claims, err := tokenParts(ctx)
	if err != nil {
		return fmt.Errorf("the console token is not a readable JWT: %w", err)
	}
	claim, value, ok := organisationClaim(claims)
	if !ok {
		return fmt.Errorf("the console token carries none of the organisation claims %v", organisationClaims)
	}
	handle, _ := claims["ouHandle"].(string)
	slog.Info("console token organisation", "claim", claim, "organisation", value, "handle", handle)
	return nil
}
