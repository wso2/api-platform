/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package aws

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"slices"
	"sort"
	"strings"
	"time"
)

const (
	algorithm     = "AWS4-HMAC-SHA256"
	amzDateLayout = "20060102T150405Z"
	scopeTerminal = "aws4_request"
	// maxClockSkew bounds how far X-Amz-Date may be from the receiver's clock.
	maxClockSkew = 5 * time.Minute

	headerAmzDate          = "X-Amz-Date"
	headerAmzContentSHA256 = "X-Amz-Content-Sha256"
	headerAmzSecurityToken = "X-Amz-Security-Token"
)

// AWS error codes returned for rejected requests.
const (
	codeMissingAuthenticationToken = "MissingAuthenticationToken"
	codeIncompleteSignature        = "IncompleteSignature"
	codeInvalidClientTokenID       = "InvalidClientTokenId"
	codeExpiredToken               = "ExpiredToken"
	codeSignatureDoesNotMatch      = "SignatureDoesNotMatch"
	codeRequestExpired             = "RequestExpired"
	codeAccessDenied               = "AccessDeniedException"
	codeResourceNotFound           = "ResourceNotFoundException"
)

// awsError is a rejection in the shape AWS reports it.
type awsError struct {
	status  int
	code    string
	message string
}

func reject(code, message string) *awsError {
	return &awsError{status: http.StatusForbidden, code: code, message: message}
}

// scope is the signing service and region a receiver expects. An empty region accepts any.
type scope struct {
	service string
	region  string
}

// identity describes a request whose signature verified.
type identity struct {
	accessKeyID string
	service     string
	region      string
}

// authorization is the parsed Authorization header of a SigV4-signed request.
type authorization struct {
	accessKeyID   string
	date          string
	region        string
	service       string
	signedHeaders []string
	signature     string
}

// verifier checks SigV4 signatures against the credentials lookup resolves.
type verifier struct {
	lookup func(accessKeyID string) (credential, bool)
	now    func() time.Time
}

// controls are test-control adjustments a receiver applies to one request.
type controls struct {
	// revoked lists access key IDs to treat as deactivated.
	revoked map[string]bool
	// signerClockSkew is how far the signer's clock is taken to run ahead of the receiver's
	// (negative when behind). X-Amz-Date is judged as if it had been stamped by that clock.
	signerClockSkew time.Duration
}

// verify authenticates r, whose already-read body is body, for the expected scope, applying the
// request's test controls.
func (v verifier) verify(r *http.Request, body []byte, want scope, ctl controls) (identity, *awsError) {
	header := r.Header.Get("Authorization")
	if header == "" {
		return identity{}, reject(codeMissingAuthenticationToken, "Missing Authentication Token")
	}
	auth, err := parseAuthorization(header)
	if err != nil {
		return identity{}, err
	}

	amzDate, parseErr := time.Parse(amzDateLayout, r.Header.Get(headerAmzDate))
	if parseErr != nil {
		return identity{}, reject(codeIncompleteSignature, "X-Amz-Date is missing or malformed")
	}
	if amzDate.Format("20060102") != auth.date {
		return identity{}, reject(codeSignatureDoesNotMatch,
			"Date in Credential scope does not match the date of X-Amz-Date")
	}
	if skew := v.now().Sub(amzDate.Add(ctl.signerClockSkew)); skew > maxClockSkew || skew < -maxClockSkew {
		return identity{}, reject(codeRequestExpired, "Signature expired")
	}
	if auth.service != want.service {
		return identity{}, reject(codeSignatureDoesNotMatch,
			fmt.Sprintf("Credential should be scoped to correct service: '%s'", want.service))
	}
	if want.region != "" && auth.region != want.region {
		return identity{}, reject(codeSignatureDoesNotMatch,
			fmt.Sprintf("Credential should be scoped to a valid region: '%s'", want.region))
	}

	cred, ok := v.lookup(auth.accessKeyID)
	if !ok || ctl.revoked[auth.accessKeyID] {
		return identity{}, reject(codeInvalidClientTokenID, "The security token included in the request is invalid")
	}
	// A temporary credential needs its own session token, and a permanent one must send none.
	if !hmac.Equal([]byte(r.Header.Get(headerAmzSecurityToken)), []byte(cred.sessionToken)) {
		return identity{}, reject(codeInvalidClientTokenID, "The security token included in the request is invalid")
	}
	// X-Amz-Date is judged as the signer's clock stamped it, as the skew check above does.
	if cred.expired(v.now()) || cred.signedAfterExpiry(amzDate.Add(ctl.signerClockSkew)) {
		return identity{}, reject(codeExpiredToken, "The security token included in the request is expired")
	}

	payloadHash := hashHex(body)
	if declared := r.Header.Get(headerAmzContentSHA256); declared != "" && declared != payloadHash {
		return identity{}, reject(codeSignatureDoesNotMatch,
			"The provided x-amz-content-sha256 header does not match what was computed")
	}
	canonical, canonicalErr := canonicalRequest(r, auth.signedHeaders, payloadHash)
	if canonicalErr != nil {
		return identity{}, canonicalErr
	}
	credentialScope := strings.Join([]string{auth.date, auth.region, auth.service, scopeTerminal}, "/")
	expected := signature(cred.secretAccessKey, auth.date, auth.region, auth.service,
		stringToSign(r.Header.Get(headerAmzDate), credentialScope, canonical))
	if !hmac.Equal([]byte(expected), []byte(auth.signature)) {
		return identity{}, reject(codeSignatureDoesNotMatch,
			"The request signature we calculated does not match the signature you provided")
	}
	return identity{accessKeyID: auth.accessKeyID, service: auth.service, region: auth.region}, nil
}

// strictlyIncreasing reports whether names are sorted and none repeats, as SigV4 lists signed headers.
func strictlyIncreasing(names []string) bool {
	for i := 1; i < len(names); i++ {
		if names[i-1] >= names[i] {
			return false
		}
	}
	return true
}

// parseAuthorization parses "AWS4-HMAC-SHA256 Credential=..., SignedHeaders=..., Signature=...".
func parseAuthorization(header string) (authorization, *awsError) {
	incomplete := func(detail string) (authorization, *awsError) {
		return authorization{}, reject(codeIncompleteSignature, "Authorization header is invalid: "+detail)
	}
	rest, ok := strings.CutPrefix(header, algorithm+" ")
	if !ok {
		return incomplete("unsupported algorithm")
	}
	fields := map[string]string{}
	for _, part := range strings.Split(rest, ",") {
		key, value, found := strings.Cut(strings.TrimSpace(part), "=")
		if !found || value == "" {
			return incomplete("malformed component")
		}
		fields[key] = value
	}

	credentialParts := strings.Split(fields["Credential"], "/")
	if len(credentialParts) != 5 || credentialParts[4] != scopeTerminal || slices.Contains(credentialParts, "") {
		return incomplete("malformed Credential")
	}
	signedHeaders := strings.Split(fields["SignedHeaders"], ";")
	if fields["SignedHeaders"] == "" || !strictlyIncreasing(signedHeaders) ||
		!slices.Contains(signedHeaders, "host") || !slices.Contains(signedHeaders, "x-amz-date") {
		return incomplete("SignedHeaders must be sorted, name each header once, and include host and x-amz-date")
	}
	if len(fields["Signature"]) != 64 {
		return incomplete("malformed Signature")
	}
	return authorization{
		accessKeyID:   credentialParts[0],
		date:          credentialParts[1],
		region:        credentialParts[2],
		service:       credentialParts[3],
		signedHeaders: signedHeaders,
		signature:     fields["Signature"],
	}, nil
}

// canonicalRequest builds the SigV4 canonical request, canonicalizing exactly as the AWS SDK
// for Go v2 signer does for a service other than S3: the escaped path is URI-encoded again,
// query values are sorted per key, and signed header values are trimmed with inner spaces
// collapsed and repeated values joined by commas.
func canonicalRequest(r *http.Request, signedHeaders []string, payloadHash string) (string, *awsError) {
	path := r.URL.EscapedPath()
	if path == "" {
		path = "/"
	}

	query := r.URL.Query()
	for key := range query {
		sort.Strings(query[key])
	}

	var headers strings.Builder
	for _, name := range signedHeaders {
		values := r.Header.Values(name)
		if name == "host" {
			values = []string{r.Host}
		}
		if len(values) == 0 || (len(values) == 1 && values[0] == "") {
			return "", reject(codeSignatureDoesNotMatch,
				fmt.Sprintf("Signed header %q is not present in the request", name))
		}
		cleaned := make([]string, len(values))
		for i, value := range values {
			cleaned[i] = strings.TrimSpace(collapseSpaces(value))
		}
		headers.WriteString(name + ":" + strings.Join(cleaned, ",") + "\n")
	}

	return strings.Join([]string{
		r.Method,
		escapePath(path),
		strings.ReplaceAll(query.Encode(), "+", "%20"),
		headers.String(),
		strings.Join(signedHeaders, ";"),
		payloadHash,
	}, "\n"), nil
}

func stringToSign(amzDate, credentialScope, canonical string) string {
	return strings.Join([]string{algorithm, amzDate, credentialScope, hashHex([]byte(canonical))}, "\n")
}

// signature derives the SigV4 signing key and signs the string to sign with it.
func signature(secret, date, region, service, toSign string) string {
	key := hmacSHA256([]byte("AWS4"+secret), date)
	key = hmacSHA256(key, region)
	key = hmacSHA256(key, service)
	key = hmacSHA256(key, scopeTerminal)
	return hex.EncodeToString(hmacSHA256(key, toSign))
}

func hmacSHA256(key []byte, data string) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(data))
	return mac.Sum(nil)
}

func hashHex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// escapePath percent-encodes every byte except RFC 3986 unreserved characters and '/'.
func escapePath(path string) string {
	const upperHex = "0123456789ABCDEF"
	var b strings.Builder
	for i := 0; i < len(path); i++ {
		c := path[i]
		if c == '/' || c == '-' || c == '_' || c == '.' || c == '~' ||
			('a' <= c && c <= 'z') || ('A' <= c && c <= 'Z') || ('0' <= c && c <= '9') {
			b.WriteByte(c)
			continue
		}
		b.WriteByte('%')
		b.WriteByte(upperHex[c>>4])
		b.WriteByte(upperHex[c&0x0f])
	}
	return b.String()
}

// collapseSpaces replaces each run of spaces with a single space.
func collapseSpaces(value string) string {
	var b strings.Builder
	previousSpace := false
	for _, r := range value {
		if r == ' ' {
			if previousSpace {
				continue
			}
			previousSpace = true
		} else {
			previousSpace = false
		}
		b.WriteRune(r)
	}
	return b.String()
}
