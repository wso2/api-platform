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
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const (
	backendURL      = "http://testbench/execute-api/us-east-1/items"
	stsURL          = "http://testbench/sts/us-east-1"
	permanentKey    = "AKIATESTPERMANENT001"
	restrictedKey   = "AKIATESTRESTRICTED01"
	temporaryKey    = "ASIATESTTEMPORARY001"
	expiredKey      = "ASIATESTEXPIRED00001"
	vectorAccessKey = "AKIDEXAMPLE"
	vectorSecret    = "wJalrXUtnFEMI/K7MDENG+bPxRfiCYEXAMPLEKEY"
	vectorAmzDate   = "20150830T123600Z"
)

var testNow = time.Date(2026, time.October, 6, 10, 0, 0, 0, time.UTC)

func fixedClock(at time.Time) func() time.Time { return func() time.Time { return at } }

// testService is the whole AWS service on a fixed clock. Every test sends its requests through
// the service's router, as the testbench does.
func testService(at time.Time) *Service {
	return newService(fixedClock(at))
}

// signRequest signs r as the AWS SDK for Go v2 signer does, with the given credential.
func signRequest(r *http.Request, body []byte, accessKeyID string, c credential, service, region string, at time.Time) {
	signRequestWithHeaders(r, body, accessKeyID, c, service, region, at)
}

// signRequestWithHeaders signs r as signRequest does, and also signs the named headers, which r
// must already carry.
func signRequestWithHeaders(r *http.Request, body []byte, accessKeyID string, c credential, service, region string,
	at time.Time, extra ...string) {
	amzDate := at.UTC().Format(amzDateLayout)
	r.Header.Set(headerAmzDate, amzDate)
	r.Header.Set(headerAmzContentSHA256, hashHex(body))
	signed := []string{"host", "x-amz-content-sha256", "x-amz-date"}
	for _, name := range extra {
		signed = append(signed, strings.ToLower(name))
	}
	if c.sessionToken != "" {
		r.Header.Set(headerAmzSecurityToken, c.sessionToken)
		signed = append(signed, "x-amz-security-token")
	}
	if r.Header.Get("Content-Type") != "" {
		signed = append(signed, "content-type")
	}
	slices.Sort(signed)
	canonical, err := canonicalRequest(r, signed, hashHex(body))
	if err != nil {
		panic(err.message)
	}
	date := at.UTC().Format("20060102")
	credentialScope := strings.Join([]string{date, region, service, scopeTerminal}, "/")
	sig := signature(c.secretAccessKey, date, region, service, stringToSign(amzDate, credentialScope, canonical))
	r.Header.Set("Authorization", fmt.Sprintf("%s Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		algorithm, accessKeyID, credentialScope, strings.Join(signed, ";"), sig))
}

func signedUpstreamRequest(method, target, body, accessKeyID string, c credential, at time.Time) *http.Request {
	r := httptest.NewRequest(method, target, strings.NewReader(body))
	r.Host = "testbench"
	if body != "" {
		r.Header.Set("Content-Type", "application/json")
	}
	signRequest(r, []byte(body), accessKeyID, c, "execute-api", "us-east-1", at)
	return r
}

func serve(handler http.Handler, r *http.Request) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, r)
	return recorder
}

func errorCode(t *testing.T, recorder *httptest.ResponseRecorder) string {
	t.Helper()
	var body map[string]string
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	require.Equal(t, body["code"], recorder.Header().Get("X-Amzn-ErrorType"))
	return body["code"]
}

// Shared by every endpoint: SigV4 verification (signature.go) and fixture credentials
// (credentials.go).

// sigv4Vector is a header-signed request in the format of the AWS SigV4 test suite: a request
// line whose target is the raw, unencoded path followed by the raw query, then "Name:value"
// header lines, a blank line, and the body.
type sigv4Vector struct {
	name    string
	service string
	token   string
	request string
}

// awsTestSuiteVectors are the cases of the official AWS SigV4 test suite
// (github.com/awslabs/aws-c-auth, tests/aws-signing-test-suite/v4) that apply to how the gateway
// signs. Every case is signed by AKIDEXAMPLE in us-east-1 for the service "service".
//
// These suite cases are not included:
//   - get-*-normalized (except get-space-normalized): they expect "//", "/./" and "/../" in the
//     path to be normalized before signing. The AWS SDK for Go v2 signer does not normalize, and
//     Envoy normalizes the request path before it reaches the policy.
//   - get-header-value-multiline: obsolete header line folding, which net/http cannot represent.
//   - get-space-normalized, get-space-unnormalized, get-utf8, get-vanilla-query-order-encoded:
//     the suite encodes the raw path once and sorts query keys by their encoded form, whereas the
//     SDK signer the gateway uses encodes the escaped path again and sorts by decoded key.
//     sdkSignerVectors covers these requests with the signer's own signatures.
var awsTestSuiteVectors = []sigv4Vector{
	{service: "service", name: "get-header-key-duplicate", token: "", request: `GET / HTTP/1.1
Host:example.amazonaws.com
My-Header1:value2
My-Header1:value2
My-Header1:value1
X-Amz-Date:20150830T123600Z
Authorization:AWS4-HMAC-SHA256 Credential=AKIDEXAMPLE/20150830/us-east-1/service/aws4_request, SignedHeaders=host;my-header1;x-amz-date, Signature=c9d5ea9f3f72853aea855b47ea873832890dbdd183b4468f858259531a5138ea

`},
	{service: "service", name: "get-header-value-order", token: "", request: `GET / HTTP/1.1
Host:example.amazonaws.com
My-Header1:value4
My-Header1:value1
My-Header1:value3
My-Header1:value2
X-Amz-Date:20150830T123600Z
Authorization:AWS4-HMAC-SHA256 Credential=AKIDEXAMPLE/20150830/us-east-1/service/aws4_request, SignedHeaders=host;my-header1;x-amz-date, Signature=08c7e5a9acfcfeb3ab6b2185e75ce8b1deb5e634ec47601a50643f830c755c01

`},
	{service: "service", name: "get-header-value-trim", token: "", request: `GET / HTTP/1.1
Host:example.amazonaws.com
My-Header1: value1
My-Header2: "a   b   c"
X-Amz-Date:20150830T123600Z
Authorization:AWS4-HMAC-SHA256 Credential=AKIDEXAMPLE/20150830/us-east-1/service/aws4_request, SignedHeaders=host;my-header1;my-header2;x-amz-date, Signature=acc3ed3afb60bb290fc8d2dd0098b9911fcaa05412b367055dee359757a9c736

`},
	{service: "service", name: "get-relative-relative-unnormalized", token: "", request: `GET /example1/example2/../.. HTTP/1.1
Host:example.amazonaws.com
X-Amz-Date:20150830T123600Z
Authorization:AWS4-HMAC-SHA256 Credential=AKIDEXAMPLE/20150830/us-east-1/service/aws4_request, SignedHeaders=host;x-amz-date, Signature=dc33e0856fd4baca4d7aa2146c38958283844764f38c74252a333df5e613003b

`},
	{service: "service", name: "get-relative-unnormalized", token: "", request: `GET /example/.. HTTP/1.1
Host:example.amazonaws.com
X-Amz-Date:20150830T123600Z
Authorization:AWS4-HMAC-SHA256 Credential=AKIDEXAMPLE/20150830/us-east-1/service/aws4_request, SignedHeaders=host;x-amz-date, Signature=eca7ead57bb5aa5c8e28007acd4ff04e1ff9a0ff3b237ec1554a184887ff9282

`},
	{service: "service", name: "get-slash-dot-slash-unnormalized", token: "", request: `GET /./ HTTP/1.1
Host:example.amazonaws.com
X-Amz-Date:20150830T123600Z
Authorization:AWS4-HMAC-SHA256 Credential=AKIDEXAMPLE/20150830/us-east-1/service/aws4_request, SignedHeaders=host;x-amz-date, Signature=68714168e6557f8f2de0ef956fc24dc2593a4bd2961f8df51898d8a134695145

`},
	{service: "service", name: "get-slash-pointless-dot-unnormalized", token: "", request: `GET /./example HTTP/1.1
Host:example.amazonaws.com
X-Amz-Date:20150830T123600Z
Authorization:AWS4-HMAC-SHA256 Credential=AKIDEXAMPLE/20150830/us-east-1/service/aws4_request, SignedHeaders=host;x-amz-date, Signature=beb03f223f7deae4146464f06e29eebbee9c8afbe15c290cf07aa8b119e14cff

`},
	{service: "service", name: "get-slash-unnormalized", token: "", request: `GET // HTTP/1.1
Host:example.amazonaws.com
X-Amz-Date:20150830T123600Z
Authorization:AWS4-HMAC-SHA256 Credential=AKIDEXAMPLE/20150830/us-east-1/service/aws4_request, SignedHeaders=host;x-amz-date, Signature=c88bcd3d312d75078c0cd961d6deae3f4c754924b01669efcfcb439fd5e5b76e

`},
	{service: "service", name: "get-slashes-unnormalized", token: "", request: `GET //example// HTTP/1.1
Host:example.amazonaws.com
X-Amz-Date:20150830T123600Z
Authorization:AWS4-HMAC-SHA256 Credential=AKIDEXAMPLE/20150830/us-east-1/service/aws4_request, SignedHeaders=host;x-amz-date, Signature=87cca117541a147f6df867677d98a7d80dff226d2bfca9e4ffa899665623c7e5

`},
	{service: "service", name: "get-unreserved", token: "", request: `GET /-._~0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz HTTP/1.1
Host:example.amazonaws.com
X-Amz-Date:20150830T123600Z
Authorization:AWS4-HMAC-SHA256 Credential=AKIDEXAMPLE/20150830/us-east-1/service/aws4_request, SignedHeaders=host;x-amz-date, Signature=07ef7494c76fa4850883e2b006601f940f8a34d404d0cfa977f52a65bbf5f24f

`},
	{service: "service", name: "get-vanilla", token: "", request: `GET / HTTP/1.1
Host:example.amazonaws.com
X-Amz-Date:20150830T123600Z
Authorization:AWS4-HMAC-SHA256 Credential=AKIDEXAMPLE/20150830/us-east-1/service/aws4_request, SignedHeaders=host;x-amz-date, Signature=5fa00fa31553b73ebf1942676e86291e8372ff2a2260956d9b8aae1d763fbf31

`},
	{service: "service", name: "get-vanilla-empty-query-key", token: "", request: `GET /?Param1=value1 HTTP/1.1
Host:example.amazonaws.com
X-Amz-Date:20150830T123600Z
Authorization:AWS4-HMAC-SHA256 Credential=AKIDEXAMPLE/20150830/us-east-1/service/aws4_request, SignedHeaders=host;x-amz-date, Signature=a67d582fa61cc504c4bae71f336f98b97f1ea3c7a6bfe1b6e45aec72011b9aeb

`},
	{service: "service", name: "get-vanilla-query", token: "", request: `GET / HTTP/1.1
Host:example.amazonaws.com
X-Amz-Date:20150830T123600Z
Authorization:AWS4-HMAC-SHA256 Credential=AKIDEXAMPLE/20150830/us-east-1/service/aws4_request, SignedHeaders=host;x-amz-date, Signature=5fa00fa31553b73ebf1942676e86291e8372ff2a2260956d9b8aae1d763fbf31

`},
	{service: "service", name: "get-vanilla-query-order-key-case", token: "", request: `GET /?Param2=value2&Param1=value1 HTTP/1.1
Host:example.amazonaws.com
X-Amz-Date:20150830T123600Z
Authorization:AWS4-HMAC-SHA256 Credential=AKIDEXAMPLE/20150830/us-east-1/service/aws4_request, SignedHeaders=host;x-amz-date, Signature=b97d918cfa904a5beff61c982a1b6f458b799221646efd99d3219ec94cdf2500

`},
	{service: "service", name: "get-vanilla-query-unreserved", token: "", request: `GET /?-._~0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz=-._~0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz HTTP/1.1
Host:example.amazonaws.com
X-Amz-Date:20150830T123600Z
Authorization:AWS4-HMAC-SHA256 Credential=AKIDEXAMPLE/20150830/us-east-1/service/aws4_request, SignedHeaders=host;x-amz-date, Signature=9c3e54bfcdf0b19771a7f523ee5669cdf59bc7cc0884027167c21bb143a40197

`},
	{service: "service", name: "get-vanilla-utf8-query", token: "", request: `GET /?ሴ=bar HTTP/1.1
Host:example.amazonaws.com
X-Amz-Date:20150830T123600Z
Authorization:AWS4-HMAC-SHA256 Credential=AKIDEXAMPLE/20150830/us-east-1/service/aws4_request, SignedHeaders=host;x-amz-date, Signature=2cdec8eed098649ff3a119c94853b13c643bcf08f8b0a1d91e12c9027818dd04

`},
	{service: "service", name: "get-vanilla-with-session-token", token: "6e86291e8372ff2a2260956d9b8aae1d763fbf315fa00fa31553b73ebf194267", request: `GET / HTTP/1.1
Host:example.amazonaws.com
X-Amz-Security-Token:6e86291e8372ff2a2260956d9b8aae1d763fbf315fa00fa31553b73ebf194267
X-Amz-Date:20150830T123600Z
Authorization:AWS4-HMAC-SHA256 Credential=AKIDEXAMPLE/20150830/us-east-1/service/aws4_request, SignedHeaders=host;x-amz-date;x-amz-security-token, Signature=07ec1639c89043aa0e3e2de82b96708f198cceab042d4a97044c66dd9f74e7f8

`},
	{service: "service", name: "post-header-key-case", token: "", request: `POST / HTTP/1.1
Host:example.amazonaws.com
X-Amz-Date:20150830T123600Z
Authorization:AWS4-HMAC-SHA256 Credential=AKIDEXAMPLE/20150830/us-east-1/service/aws4_request, SignedHeaders=host;x-amz-date, Signature=5da7c1a2acd57cee7505fc6676e4e544621c30862966e37dddb68e92efbe5d6b

`},
	{service: "service", name: "post-header-key-sort", token: "", request: `POST / HTTP/1.1
Host:example.amazonaws.com
My-Header1:value1
X-Amz-Date:20150830T123600Z
Authorization:AWS4-HMAC-SHA256 Credential=AKIDEXAMPLE/20150830/us-east-1/service/aws4_request, SignedHeaders=host;my-header1;x-amz-date, Signature=c5410059b04c1ee005303aed430f6e6645f61f4dc9e1461ec8f8916fdf18852c

`},
	{service: "service", name: "post-header-value-case", token: "", request: `POST / HTTP/1.1
Host:example.amazonaws.com
My-Header1:VALUE1
X-Amz-Date:20150830T123600Z
Authorization:AWS4-HMAC-SHA256 Credential=AKIDEXAMPLE/20150830/us-east-1/service/aws4_request, SignedHeaders=host;my-header1;x-amz-date, Signature=cdbc9802e29d2942e5e10b5bccfdd67c5f22c7c4e8ae67b53629efa58b974b7d

`},
	{service: "service", name: "post-sts-header-after", token: "AQoDYXdzEPT//////////wEXAMPLEtc764bNrC9SAPBSM22wDOk4x4HIZ8j4FZTwdQWLWsKWHGBuFqwAeMicRXmxfpSPfIeoIYRqTflfKD8YUuwthAx7mSEI/qkPpKPi/kMcGdQrmGdeehM4IC1NtBmUpp2wUE8phUZampKsburEDy0KPkyQDYwT7WZ0wq5VSXDvp75YU9HFvlRd8Tx6q6fE8YQcHNVXAkiY9q6d+xo0rKwT38xVqr7ZD0u0iPPkUL64lIZbqBAz+scqKmlzm8FDrypNC9Yjc8fPOLn9FX9KSYvKTr4rvx3iSIlTJabIQwj2ICCR/oLxBA==", request: `POST / HTTP/1.1
Host:example.amazonaws.com
X-Amz-Security-Token:AQoDYXdzEPT//////////wEXAMPLEtc764bNrC9SAPBSM22wDOk4x4HIZ8j4FZTwdQWLWsKWHGBuFqwAeMicRXmxfpSPfIeoIYRqTflfKD8YUuwthAx7mSEI/qkPpKPi/kMcGdQrmGdeehM4IC1NtBmUpp2wUE8phUZampKsburEDy0KPkyQDYwT7WZ0wq5VSXDvp75YU9HFvlRd8Tx6q6fE8YQcHNVXAkiY9q6d+xo0rKwT38xVqr7ZD0u0iPPkUL64lIZbqBAz+scqKmlzm8FDrypNC9Yjc8fPOLn9FX9KSYvKTr4rvx3iSIlTJabIQwj2ICCR/oLxBA==
X-Amz-Date:20150830T123600Z
Authorization:AWS4-HMAC-SHA256 Credential=AKIDEXAMPLE/20150830/us-east-1/service/aws4_request, SignedHeaders=host;x-amz-date, Signature=5da7c1a2acd57cee7505fc6676e4e544621c30862966e37dddb68e92efbe5d6b

`},
	{service: "service", name: "post-sts-header-before", token: "AQoDYXdzEPT//////////wEXAMPLEtc764bNrC9SAPBSM22wDOk4x4HIZ8j4FZTwdQWLWsKWHGBuFqwAeMicRXmxfpSPfIeoIYRqTflfKD8YUuwthAx7mSEI/qkPpKPi/kMcGdQrmGdeehM4IC1NtBmUpp2wUE8phUZampKsburEDy0KPkyQDYwT7WZ0wq5VSXDvp75YU9HFvlRd8Tx6q6fE8YQcHNVXAkiY9q6d+xo0rKwT38xVqr7ZD0u0iPPkUL64lIZbqBAz+scqKmlzm8FDrypNC9Yjc8fPOLn9FX9KSYvKTr4rvx3iSIlTJabIQwj2ICCR/oLxBA==", request: `POST / HTTP/1.1
Host:example.amazonaws.com
X-Amz-Security-Token:AQoDYXdzEPT//////////wEXAMPLEtc764bNrC9SAPBSM22wDOk4x4HIZ8j4FZTwdQWLWsKWHGBuFqwAeMicRXmxfpSPfIeoIYRqTflfKD8YUuwthAx7mSEI/qkPpKPi/kMcGdQrmGdeehM4IC1NtBmUpp2wUE8phUZampKsburEDy0KPkyQDYwT7WZ0wq5VSXDvp75YU9HFvlRd8Tx6q6fE8YQcHNVXAkiY9q6d+xo0rKwT38xVqr7ZD0u0iPPkUL64lIZbqBAz+scqKmlzm8FDrypNC9Yjc8fPOLn9FX9KSYvKTr4rvx3iSIlTJabIQwj2ICCR/oLxBA==
X-Amz-Date:20150830T123600Z
Authorization:AWS4-HMAC-SHA256 Credential=AKIDEXAMPLE/20150830/us-east-1/service/aws4_request, SignedHeaders=host;x-amz-date;x-amz-security-token, Signature=85d96828115b5dc0cfc3bd16ad9e210dd772bbebba041836c64533a82be05ead

`},
	{service: "service", name: "post-vanilla", token: "", request: `POST / HTTP/1.1
Host:example.amazonaws.com
X-Amz-Date:20150830T123600Z
Authorization:AWS4-HMAC-SHA256 Credential=AKIDEXAMPLE/20150830/us-east-1/service/aws4_request, SignedHeaders=host;x-amz-date, Signature=5da7c1a2acd57cee7505fc6676e4e544621c30862966e37dddb68e92efbe5d6b

`},
	{service: "service", name: "post-vanilla-empty-query-value", token: "", request: `POST /?Param1=value1 HTTP/1.1
Host:example.amazonaws.com
X-Amz-Date:20150830T123600Z
Authorization:AWS4-HMAC-SHA256 Credential=AKIDEXAMPLE/20150830/us-east-1/service/aws4_request, SignedHeaders=host;x-amz-date, Signature=28038455d6de14eafc1f9222cf5aa6f1a96197d7deb8263271d420d138af7f11

`},
	{service: "service", name: "post-vanilla-query", token: "", request: `POST /?Param1=value1 HTTP/1.1
Host:example.amazonaws.com
X-Amz-Date:20150830T123600Z
Authorization:AWS4-HMAC-SHA256 Credential=AKIDEXAMPLE/20150830/us-east-1/service/aws4_request, SignedHeaders=host;x-amz-date, Signature=28038455d6de14eafc1f9222cf5aa6f1a96197d7deb8263271d420d138af7f11

`},
	{service: "service", name: "post-x-www-form-urlencoded", token: "", request: `POST / HTTP/1.1
Content-Type:application/x-www-form-urlencoded
Host:example.amazonaws.com
Content-Length:13
X-Amz-Date:20150830T123600Z
x-amz-content-sha256:9095672bbd1f56dfc5b65f3e153adc8731a4a654192329106275f4c7b24d0b6e
Authorization:AWS4-HMAC-SHA256 Credential=AKIDEXAMPLE/20150830/us-east-1/service/aws4_request, SignedHeaders=content-length;content-type;host;x-amz-content-sha256;x-amz-date, Signature=d3875051da38690788ef43de4db0d8f280229d82040bfac253562e56c3f20e0b

Param1=value1`},
	{service: "service", name: "post-x-www-form-urlencoded-parameters", token: "", request: `POST / HTTP/1.1
Content-Type:application/x-www-form-urlencoded; charset=utf-8
Host:example.amazonaws.com
Content-Length:13
X-Amz-Date:20150830T123600Z
x-amz-content-sha256:9095672bbd1f56dfc5b65f3e153adc8731a4a654192329106275f4c7b24d0b6e
Authorization:AWS4-HMAC-SHA256 Credential=AKIDEXAMPLE/20150830/us-east-1/service/aws4_request, SignedHeaders=content-length;content-type;host;x-amz-content-sha256;x-amz-date, Signature=328d1b9eaadca9f5818ef05e8392801e091653bafec24fcab71e7344e7f51422

Param1=value1`},
}

// iamListUsersVector is the IAM ListUsers example from the AWS SigV4 documentation.
var iamListUsersVector = sigv4Vector{service: "iam", name: "iam-list-users", request: `GET /?Action=ListUsers&Version=2010-05-08 HTTP/1.1
Host:iam.amazonaws.com
Content-Type:application/x-www-form-urlencoded; charset=utf-8
X-Amz-Date:20150830T123600Z
Authorization:AWS4-HMAC-SHA256 Credential=AKIDEXAMPLE/20150830/us-east-1/iam/aws4_request, SignedHeaders=content-type;host;x-amz-date, Signature=5d672d79c15b13162d9279b0855cfba6789a8edb4c82c400e06b5924a6f2b5d7

`}

// sdkSignerVectors are the suite requests whose canonical form differs between the suite and the
// AWS SDK for Go v2 signer (v1.42.1), signed by that signer, which is what the gateway uses.
var sdkSignerVectors = []sigv4Vector{
	{service: "service", name: "sdk-get-utf8", request: `GET /ሴ HTTP/1.1
Host:example.amazonaws.com
X-Amz-Date:20150830T123600Z
Authorization:AWS4-HMAC-SHA256 Credential=AKIDEXAMPLE/20150830/us-east-1/service/aws4_request, SignedHeaders=host;x-amz-date, Signature=697b34846207a3f72246f99d74ae1ee4fe54f44bb06730c58a0d339eb079596d

`},
	{service: "service", name: "sdk-get-space", request: `GET /example space/ HTTP/1.1
Host:example.amazonaws.com
X-Amz-Date:20150830T123600Z
Authorization:AWS4-HMAC-SHA256 Credential=AKIDEXAMPLE/20150830/us-east-1/service/aws4_request, SignedHeaders=host;x-amz-date, Signature=446b817944c553435b35e813c261ff4e161fff982d1bacdef1c87f6785dd1662

`},
	{service: "service", name: "sdk-get-vanilla-query-order-encoded", request: `GET /?Param-3=Value3&Param=Value2&%E1%88%B4=Value1 HTTP/1.1
Host:example.amazonaws.com
X-Amz-Date:20150830T123600Z
Authorization:AWS4-HMAC-SHA256 Credential=AKIDEXAMPLE/20150830/us-east-1/service/aws4_request, SignedHeaders=host;x-amz-date, Signature=cb84b0f32cb0ccd7ed58acd7c6f6da0a621d75d216d20a6265f7e8ffbe98eb81

`},
}

// parseVector builds the request a SigV4 test-suite case describes and returns it with its body.
func parseVector(t *testing.T, raw string) (*http.Request, []byte) {
	t.Helper()
	head, body, _ := strings.Cut(raw, "\n\n")
	lines := strings.Split(head, "\n")
	method, rest, found := strings.Cut(lines[0], " ")
	require.True(t, found, "request line %q", lines[0])
	target := rest[:strings.LastIndex(rest, " ")]
	rawPath, rawQuery, _ := strings.Cut(target, "?")

	r, err := http.NewRequest(method, "http://vector"+(&url.URL{Path: rawPath}).EscapedPath(), strings.NewReader(body))
	require.NoError(t, err)
	r.URL.RawQuery = rawQuery
	r.Header = http.Header{}
	for _, line := range lines[1:] {
		name, value, found := strings.Cut(line, ":")
		require.True(t, found, "header line %q", line)
		if strings.EqualFold(name, "Host") {
			r.Host = value
			continue
		}
		r.Header.Add(name, value)
	}
	return r, []byte(body)
}

func vectorVerifier(token string) verifier {
	at, _ := time.Parse(amzDateLayout, vectorAmzDate)
	return verifier{
		lookup: func(id string) (credential, bool) {
			return credential{secretAccessKey: vectorSecret, sessionToken: token}, id == vectorAccessKey
		},
		now: fixedClock(at),
	}
}

func TestVerifyAcceptsAWSSignatureVectors(t *testing.T) {
	vectors := append(append(slices.Clone(awsTestSuiteVectors), iamListUsersVector), sdkSignerVectors...)
	require.Len(t, vectors, 31)
	for _, v := range vectors {
		t.Run(v.name, func(t *testing.T) {
			want := scope{service: v.service, region: "us-east-1"}
			r, body := parseVector(t, v.request)
			id, rejected := vectorVerifier(v.token).verify(r, body, want, controls{})
			require.Nil(t, rejected)
			require.Equal(t, identity{accessKeyID: vectorAccessKey, service: v.service, region: "us-east-1"}, id)

			// Changing one signature character must fail verification.
			r, body = parseVector(t, v.request)
			authorization := r.Header.Get("Authorization")
			last := authorization[len(authorization)-1:]
			flipped := map[bool]string{true: "1", false: "0"}[last == "0"]
			r.Header.Set("Authorization", authorization[:len(authorization)-1]+flipped)
			_, rejected = vectorVerifier(v.token).verify(r, body, want, controls{})
			require.NotNil(t, rejected)
			require.Equal(t, codeSignatureDoesNotMatch, rejected.code)
		})
	}
}

func TestCanonicalizationHelpers(t *testing.T) {
	// The path is already escaped, so its percent signs are encoded again, as SigV4 requires for
	// every service other than S3.
	require.Equal(t, "/a%2520b/c~d/%25E2%2582%25AC", escapePath("/a%20b/c~d/%E2%82%AC"))
	require.Equal(t, "/plain-path_1.2", escapePath("/plain-path_1.2"))
	require.Equal(t, " a b c ", collapseSpaces("   a  b   c "))
	require.Equal(t, "", collapseSpaces(""))

	r := httptest.NewRequest(http.MethodGet, "http://testbench/x?b=2&a=z&a=y&c=a+b", nil)
	r.Header.Set(headerAmzDate, "  multiple   spaces  ")
	canonical, err := canonicalRequest(r, []string{"host", "x-amz-date"}, "hash")
	require.Nil(t, err)
	require.Equal(t, "GET\n/x\na=y&a=z&b=2&c=a%20b\nhost:testbench\nx-amz-date:multiple spaces\n\nhost;x-amz-date\nhash",
		canonical)
}

func TestParseAuthorizationRejectsMalformedHeaders(t *testing.T) {
	valid := algorithm + " Credential=AKID/20260101/us-east-1/execute-api/aws4_request, " +
		"SignedHeaders=host;x-amz-date, Signature=" + strings.Repeat("a", 64)
	parsed, err := parseAuthorization(valid)
	require.Nil(t, err)
	require.Equal(t, "AKID", parsed.accessKeyID)
	require.Equal(t, []string{"host", "x-amz-date"}, parsed.signedHeaders)

	for name, header := range map[string]string{
		"empty":                "",
		"no components":        algorithm + " ",
		"empty credential":     strings.Replace(valid, "AKID/20260101/us-east-1/execute-api/aws4_request", "", 1),
		"empty credential key": strings.Replace(valid, "AKID/", "/", 1),
		"unsorted headers":     strings.Replace(valid, "host;x-amz-date", "x-amz-date;host", 1),
		"repeated header":      strings.Replace(valid, "host;x-amz-date", "host;host;x-amz-date", 1),
		"empty header name":    strings.Replace(valid, "host;x-amz-date", ";host;x-amz-date", 1),
		"no x-amz-date":        strings.Replace(valid, "host;x-amz-date", "host", 1),
		"short signature":      strings.Replace(valid, strings.Repeat("a", 64), "abc", 1),
		"component without =":  valid + ", Extra",
	} {
		t.Run(name, func(t *testing.T) {
			_, err := parseAuthorization(header)
			require.NotNil(t, err)
			require.Equal(t, codeIncompleteSignature, err.code)
		})
	}
}

func TestCredentialPermits(t *testing.T) {
	require.True(t, credential{}.permits("/anything"))
	restricted := credential{permittedResources: []string{"/items"}}
	require.True(t, restricted.permits("/items"))
	require.True(t, restricted.permits("/items/42"))
	require.False(t, restricted.permits("/items-archive"))
	require.False(t, restricted.permits("/orders"))
	require.False(t, restricted.permits("/"))
	require.False(t, credential{permittedResources: []string{}}.permits("/items"))
}

// The service: registration and routing (service.go).

func TestServiceMetadata(t *testing.T) {
	require.Equal(t, "aws", New().Name())
	require.Equal(t, 80, New().Port())
	require.False(t, New().Stateful())
}

func TestServiceRoutesEachRequestByItsServiceName(t *testing.T) {
	handler := testService(testNow).Handler()

	health := serve(handler, httptest.NewRequest(http.MethodGet, "http://testbench/health", nil))
	require.Equal(t, http.StatusOK, health.Code)
	require.JSONEq(t, `{"status":"ok","service":"aws"}`, health.Body.String())
	notGet := serve(handler, httptest.NewRequest(http.MethodPost, "http://testbench/health", nil))
	require.Equal(t, http.StatusMethodNotAllowed, notGet.Code)

	for _, path := range []string{"/", "/execute-api", "/execute-api/", "//us-east-1/items"} {
		recorder := serve(handler, httptest.NewRequest(http.MethodGet, "http://testbench"+path, nil))
		require.Equal(t, http.StatusNotFound, recorder.Code, path)
		require.Equal(t, "NotFound", errorCode(t, recorder), path)
	}

	// Any service other than sts is the upstream, verified for its own scope.
	r := httptest.NewRequest(http.MethodGet, "http://testbench/lambda/eu-west-1/fn", nil)
	r.Host = "testbench"
	signRequest(r, nil, permanentKey, fixtureCredentials[permanentKey], "lambda", "eu-west-1", testNow)
	upstream := serve(handler, r)
	require.Equal(t, http.StatusOK, upstream.Code, upstream.Body.String())
	var reflected reflection
	require.NoError(t, json.Unmarshal(upstream.Body.Bytes(), &reflected))
	require.Equal(t, "lambda", reflected.Service)
	require.Equal(t, "eu-west-1", reflected.Region)

	assumed := serve(handler, assumeRoleRequest(assumeRoleForm(), permanentKey, fixtureCredentials[permanentKey], testNow))
	require.Equal(t, http.StatusOK, assumed.Code, assumed.Body.String())
	require.Contains(t, assumed.Body.String(), "<AssumeRoleResponse")
}

func TestEndpointOf(t *testing.T) {
	for path, want := range map[string]endpoint{
		"/sts/us-east-1":                 {service: "sts", region: "us-east-1"},
		"/sts/us-east-1/":                {service: "sts", region: "us-east-1", rest: "/"},
		"/execute-api/us-east-1/items/1": {service: "execute-api", region: "us-east-1", rest: "/items/1"},
		"/lambda/eu-west-1/fn":           {service: "lambda", region: "eu-west-1", rest: "/fn"},
	} {
		got, ok := endpointOf(path)
		require.True(t, ok, path)
		require.Equal(t, want, got, path)
	}
	for _, path := range []string{"", "/", "/sts", "/sts/", "//us-east-1"} {
		_, ok := endpointOf(path)
		require.False(t, ok, path)
	}
}

// Upstream, the AWS Authentication policy's AWS-hosted backend (upstream.go).

func TestUpstreamAcceptsSignedRequestsAndReflectsThem(t *testing.T) {
	for _, tc := range []struct{ method, body string }{
		{http.MethodGet, ""},
		{http.MethodPost, `{"item":"one"}`},
	} {
		t.Run(tc.method, func(t *testing.T) {
			r := signedUpstreamRequest(tc.method, backendURL+"?b=2&a=1&a=0", tc.body, permanentKey,
				fixtureCredentials[permanentKey], testNow)
			recorder := serve(testService(testNow).Handler(), r)

			require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
			require.Equal(t, permanentKey, recorder.Header().Get(HeaderAccessKeyID))
			var reflected reflection
			require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &reflected))
			require.Equal(t, tc.method, reflected.Method)
			require.Equal(t, "/execute-api/us-east-1/items", reflected.Path)
			require.Equal(t, tc.body, reflected.Body)
			require.Equal(t, permanentKey, reflected.AccessKeyID)
			require.Equal(t, "execute-api", reflected.Service)
			require.Equal(t, "us-east-1", reflected.Region)
			require.Equal(t, []string{"testbench"}, reflected.Headers["Host"])
		})
	}
}

func TestUpstreamAcceptsTemporaryCredentialsWithTheirSessionToken(t *testing.T) {
	r := signedUpstreamRequest(http.MethodGet, backendURL, "", temporaryKey, fixtureCredentials[temporaryKey], testNow)
	recorder := serve(testService(testNow).Handler(), r)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, temporaryKey, recorder.Header().Get(HeaderAccessKeyID))
}

func TestUpstreamRejectsInvalidRequests(t *testing.T) {
	permanent := fixtureCredentials[permanentKey]
	cases := []struct {
		name   string
		build  func() *http.Request
		status int
		code   string
	}{
		{"missing authorization", func() *http.Request {
			return httptest.NewRequest(http.MethodGet, backendURL, nil)
		}, http.StatusForbidden, codeMissingAuthenticationToken},
		{"unsupported algorithm", func() *http.Request {
			r := signedUpstreamRequest(http.MethodGet, backendURL, "", permanentKey, permanent, testNow)
			r.Header.Set("Authorization", strings.Replace(r.Header.Get("Authorization"), algorithm, "AWS4-HMAC-SHA1", 1))
			return r
		}, http.StatusForbidden, codeIncompleteSignature},
		{"malformed credential", func() *http.Request {
			r := signedUpstreamRequest(http.MethodGet, backendURL, "", permanentKey, permanent, testNow)
			r.Header.Set("Authorization", strings.Replace(r.Header.Get("Authorization"), "/aws4_request", "", 1))
			return r
		}, http.StatusForbidden, codeIncompleteSignature},
		{"signed headers without host", func() *http.Request {
			r := signedUpstreamRequest(http.MethodGet, backendURL, "", permanentKey, permanent, testNow)
			r.Header.Set("Authorization", strings.Replace(r.Header.Get("Authorization"), "host;", "", 1))
			return r
		}, http.StatusForbidden, codeIncompleteSignature},
		{"missing x-amz-date", func() *http.Request {
			r := signedUpstreamRequest(http.MethodGet, backendURL, "", permanentKey, permanent, testNow)
			r.Header.Del(headerAmzDate)
			return r
		}, http.StatusForbidden, codeIncompleteSignature},
		{"signed too long ago", func() *http.Request {
			return signedUpstreamRequest(http.MethodGet, backendURL, "", permanentKey, permanent, testNow.Add(-6*time.Minute))
		}, http.StatusForbidden, codeRequestExpired},
		{"unknown access key", func() *http.Request {
			return signedUpstreamRequest(http.MethodGet, backendURL, "", "AKIATESTUNKNOWN00001", permanent, testNow)
		}, http.StatusForbidden, codeInvalidClientTokenID},
		{"wrong secret", func() *http.Request {
			wrong := credential{secretAccessKey: "testWrongSecretAccessKey0000000000000001"}
			return signedUpstreamRequest(http.MethodGet, backendURL, "", permanentKey, wrong, testNow)
		}, http.StatusForbidden, codeSignatureDoesNotMatch},
		{"wrong service", func() *http.Request {
			r := httptest.NewRequest(http.MethodGet, backendURL, nil)
			r.Host = "testbench"
			signRequest(r, nil, permanentKey, permanent, "lambda", "us-east-1", testNow)
			return r
		}, http.StatusForbidden, codeSignatureDoesNotMatch},
		{"wrong region", func() *http.Request {
			r := httptest.NewRequest(http.MethodGet, backendURL, nil)
			r.Host = "testbench"
			signRequest(r, nil, permanentKey, permanent, "execute-api", "eu-west-1", testNow)
			return r
		}, http.StatusForbidden, codeSignatureDoesNotMatch},
		{"host changed after signing", func() *http.Request {
			r := signedUpstreamRequest(http.MethodGet, backendURL, "", permanentKey, permanent, testNow)
			r.Host = "gateway:8080"
			return r
		}, http.StatusForbidden, codeSignatureDoesNotMatch},
		{"body changed after signing", func() *http.Request {
			r := httptest.NewRequest(http.MethodPost, backendURL, strings.NewReader(`{"item":"two"}`))
			r.Host = "testbench"
			signRequest(r, []byte(`{"item":"one"}`), permanentKey, permanent, "execute-api", "us-east-1", testNow)
			return r
		}, http.StatusForbidden, codeSignatureDoesNotMatch},
		{"signed header removed", func() *http.Request {
			r := signedUpstreamRequest(http.MethodPost, backendURL, `{"item":"one"}`, permanentKey, permanent, testNow)
			r.Header.Del("Content-Type")
			return r
		}, http.StatusForbidden, codeSignatureDoesNotMatch},
		{"temporary key without session token", func() *http.Request {
			withoutToken := fixtureCredentials[temporaryKey]
			withoutToken.sessionToken = ""
			return signedUpstreamRequest(http.MethodGet, backendURL, "", temporaryKey, withoutToken, testNow)
		}, http.StatusForbidden, codeInvalidClientTokenID},
		{"permanent key with a session token", func() *http.Request {
			withToken := permanent
			withToken.sessionToken = "testUnexpectedSessionToken0001"
			return signedUpstreamRequest(http.MethodGet, backendURL, "", permanentKey, withToken, testNow)
		}, http.StatusForbidden, codeInvalidClientTokenID},
		{"temporary key with another session token", func() *http.Request {
			other := fixtureCredentials[temporaryKey]
			other.sessionToken = "testOtherSessionToken0001"
			return signedUpstreamRequest(http.MethodGet, backendURL, "", temporaryKey, other, testNow)
		}, http.StatusForbidden, codeInvalidClientTokenID},
		{"expired temporary credentials", func() *http.Request {
			return signedUpstreamRequest(http.MethodGet, backendURL, "", expiredKey, fixtureCredentials[expiredKey], testNow)
		}, http.StatusForbidden, codeExpiredToken},
		{"revoked access key", func() *http.Request {
			r := signedUpstreamRequest(http.MethodGet, backendURL, "", permanentKey, permanent, testNow)
			r.Header.Set(HeaderRevokedAccessKeys, "AKIATESTPERMANENT002, "+permanentKey)
			return r
		}, http.StatusForbidden, codeInvalidClientTokenID},
		{"restricted key outside its permitted resources", func() *http.Request {
			return signedUpstreamRequest(http.MethodGet, "http://testbench/execute-api/us-east-1/orders", "",
				restrictedKey, fixtureCredentials[restrictedKey], testNow)
		}, http.StatusForbidden, codeAccessDenied},
		{"restricted key on a sibling of its permitted resource", func() *http.Request {
			return signedUpstreamRequest(http.MethodGet, "http://testbench/execute-api/us-east-1/items-archive", "",
				restrictedKey, fixtureCredentials[restrictedKey], testNow)
		}, http.StatusForbidden, codeAccessDenied},
		{"nonexistent resource", func() *http.Request {
			return signedUpstreamRequest(http.MethodGet, "http://testbench/execute-api/us-east-1/nonexistent/items", "",
				permanentKey, permanent, testNow)
		}, http.StatusNotFound, codeResourceNotFound},
		{"signer clock ahead beyond the tolerance", func() *http.Request {
			r := signedUpstreamRequest(http.MethodGet, backendURL, "", permanentKey, permanent, testNow)
			r.Header.Set(HeaderSignerClockSkew, "10m")
			return r
		}, http.StatusForbidden, codeRequestExpired},
		{"signer clock behind beyond the tolerance", func() *http.Request {
			r := signedUpstreamRequest(http.MethodGet, backendURL, "", permanentKey, permanent, testNow)
			r.Header.Set(HeaderSignerClockSkew, "-10m")
			return r
		}, http.StatusForbidden, codeRequestExpired},
		{"malformed signer clock skew", func() *http.Request {
			r := signedUpstreamRequest(http.MethodGet, backendURL, "", permanentKey, permanent, testNow)
			r.Header.Set(HeaderSignerClockSkew, "ten minutes")
			return r
		}, http.StatusBadRequest, "InvalidRequest"},
		{"two signer clock skews", func() *http.Request {
			r := signedUpstreamRequest(http.MethodGet, backendURL, "", permanentKey, permanent, testNow)
			r.Header.Add(HeaderSignerClockSkew, "0s")
			r.Header.Add(HeaderSignerClockSkew, "0s")
			return r
		}, http.StatusBadRequest, "InvalidRequest"},
		{"a valid then a malformed signer clock skew", func() *http.Request {
			r := signedUpstreamRequest(http.MethodGet, backendURL, "", permanentKey, permanent, testNow)
			r.Header.Add(HeaderSignerClockSkew, "0s")
			r.Header.Add(HeaderSignerClockSkew, "ten minutes")
			return r
		}, http.StatusBadRequest, "InvalidRequest"},
		{"unsigned, with a malformed signer clock skew", func() *http.Request {
			r := httptest.NewRequest(http.MethodGet, backendURL, nil)
			r.Host = "testbench"
			r.Header.Set(HeaderSignerClockSkew, "ten minutes")
			return r
		}, http.StatusForbidden, codeMissingAuthenticationToken},
		{"path without service and region", func() *http.Request {
			return signedUpstreamRequest(http.MethodGet, "http://testbench/items", "", permanentKey, permanent, testNow)
		}, http.StatusNotFound, "NotFound"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			recorder := serve(testService(testNow).Handler(), tc.build())
			require.Equal(t, tc.status, recorder.Code, recorder.Body.String())
			require.Equal(t, tc.code, errorCode(t, recorder))
			require.Empty(t, recorder.Header().Get(HeaderAccessKeyID))
			require.NotContains(t, recorder.Body.String(), permanent.secretAccessKey)
		})
	}
}

func TestUpstreamRevocationAppliesOnlyToListedKeys(t *testing.T) {
	r := signedUpstreamRequest(http.MethodGet, backendURL, "", "AKIATESTPERMANENT002",
		fixtureCredentials["AKIATESTPERMANENT002"], testNow)
	r.Header.Set(HeaderRevokedAccessKeys, permanentKey)
	recorder := serve(testService(testNow).Handler(), r)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, "AKIATESTPERMANENT002", recorder.Header().Get(HeaderAccessKeyID))
}

func TestUpstreamAuthorizesAndLocatesResourcesOnlyAfterVerifying(t *testing.T) {
	restricted := fixtureCredentials[restrictedKey]
	for _, target := range []string{backendURL, backendURL + "/42"} {
		r := signedUpstreamRequest(http.MethodGet, target, "", restrictedKey, restricted, testNow)
		recorder := serve(testService(testNow).Handler(), r)
		require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
		require.Equal(t, restrictedKey, recorder.Header().Get(HeaderAccessKeyID))
	}

	const ordersURL = "http://testbench/execute-api/us-east-1/orders"
	const nonexistentURL = "http://testbench/execute-api/us-east-1/nonexistent"
	wrongSecret := restricted
	wrongSecret.secretAccessKey = "testWrongSecretAccessKey0000000000000001"
	cases := []struct {
		name   string
		build  func() *http.Request
		status int
		code   string
	}{
		{"unsigned request for a resource the key may not invoke", func() *http.Request {
			return httptest.NewRequest(http.MethodGet, ordersURL, nil)
		}, http.StatusForbidden, codeMissingAuthenticationToken},
		{"wrong secret for a resource the key may not invoke", func() *http.Request {
			return signedUpstreamRequest(http.MethodGet, ordersURL, "", restrictedKey, wrongSecret, testNow)
		}, http.StatusForbidden, codeSignatureDoesNotMatch},
		{"unsigned request for a nonexistent resource", func() *http.Request {
			return httptest.NewRequest(http.MethodGet, nonexistentURL, nil)
		}, http.StatusForbidden, codeMissingAuthenticationToken},
		{"restricted key for a nonexistent resource", func() *http.Request {
			return signedUpstreamRequest(http.MethodGet, nonexistentURL, "", restrictedKey, restricted, testNow)
		}, http.StatusForbidden, codeAccessDenied},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			recorder := serve(testService(testNow).Handler(), tc.build())
			require.Equal(t, tc.status, recorder.Code, recorder.Body.String())
			require.Equal(t, tc.code, errorCode(t, recorder))
			require.NotContains(t, recorder.Body.String(), restricted.secretAccessKey)
		})
	}
}

func TestUpstreamJudgesXAmzDateByTheSignerClockSkew(t *testing.T) {
	permanent := fixtureCredentials[permanentKey]
	for _, tc := range []struct {
		name     string
		signedAt time.Time
		skew     string
	}{
		{"a correct clock", testNow, "0s"},
		{"a skew within the tolerance", testNow, "4m"},
		{"a skew that moves an old X-Amz-Date to now", testNow.Add(-8 * time.Minute), "8m"},
		{"a skew that moves a future X-Amz-Date to now", testNow.Add(8 * time.Minute), "-8m"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := signedUpstreamRequest(http.MethodGet, backendURL, "", permanentKey, permanent, tc.signedAt)
			r.Header.Set(HeaderSignerClockSkew, tc.skew)
			recorder := serve(testService(testNow).Handler(), r)
			require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
		})
	}
}

func TestRevokedKeysParsing(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, backendURL, nil)
	require.Empty(t, revokedKeys(r))
	r.Header.Add(HeaderRevokedAccessKeys, " A , ,B")
	r.Header.Add(HeaderRevokedAccessKeys, "C")
	require.Equal(t, map[string]bool{"A": true, "B": true, "C": true}, revokedKeys(r))
}

// STS, for policies that assume an IAM role (sts.go).

func TestIssuedCredentialsAreDerivedFromTheirExpiry(t *testing.T) {
	accessKeyID, issued := issueCredentials(testNow, issuedCredentialLifetime)
	require.Len(t, accessKeyID, 20)
	require.True(t, strings.HasPrefix(accessKeyID, issuedKeyPrefix))
	require.Equal(t, testNow.Add(issuedCredentialLifetime), issued.expiresAt)
	require.Len(t, issued.secretAccessKey, 40)
	require.Contains(t, issued.secretAccessKey, "SecretAccessKey000")
	require.Contains(t, issued.sessionToken, "SessionToken0001")
	other, otherCred := issueCredentials(testNow.Add(time.Second), issuedCredentialLifetime)
	require.NotEqual(t, accessKeyID, other)
	require.NotEqual(t, issued.secretAccessKey, otherCred.secretAccessKey)
	require.NotEqual(t, issued.sessionToken, otherCred.sessionToken)

	resolved, ok := lookupCredential(accessKeyID)
	require.True(t, ok)
	require.Equal(t, issued, resolved)

	later, _ := issueCredentials(testNow.Add(issuedCredentialLifetime), issuedCredentialLifetime)
	require.NotEqual(t, accessKeyID, later, "a refresh must yield a different access key ID")

	require.False(t, issued.expired(testNow))
	require.False(t, issued.expired(issued.expiresAt.Add(expiryGrace)))
	require.True(t, issued.expired(issued.expiresAt.Add(expiryGrace+time.Second)))

	for _, malformed := range []string{"ASIASTS", "ASIASTS000000000000x", "ASIASTS0000000000000", "AKIASTS1234567890123"} {
		_, ok := lookupCredential(malformed)
		require.False(t, ok, malformed)
	}
	_, ok = lookupFixtureCredential(accessKeyID)
	require.False(t, ok, "STS must not accept issued credentials as base credentials")
}

func assumeRoleRequest(form url.Values, accessKeyID string, c credential, at time.Time) *http.Request {
	body := form.Encode()
	r := httptest.NewRequest(http.MethodPost, stsURL, strings.NewReader(body))
	r.Host = "testbench"
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	signRequest(r, []byte(body), accessKeyID, c, "sts", "us-east-1", at)
	return r
}

func assumeRoleForm() url.Values {
	return url.Values{
		"Action":          {"AssumeRole"},
		"Version":         {"2011-06-15"},
		"RoleArn":         {"arn:aws:iam::123456789012:role/aws-authentication-it"},
		"RoleSessionName": {"aws-authentication-session"},
	}
}

func TestSTSIssuesCredentialsTheUpstreamAcceptsUntilTheyExpire(t *testing.T) {
	recorder := serve(testService(testNow).Handler(),
		assumeRoleRequest(assumeRoleForm(), permanentKey, fixtureCredentials[permanentKey], testNow))
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, "text/xml", recorder.Header().Get("Content-Type"))

	var response struct {
		Result struct {
			Credentials struct {
				AccessKeyID     string `xml:"AccessKeyId"`
				SecretAccessKey string `xml:"SecretAccessKey"`
				SessionToken    string `xml:"SessionToken"`
				Expiration      string `xml:"Expiration"`
			} `xml:"Credentials"`
			AssumedRoleUser struct {
				Arn string `xml:"Arn"`
			} `xml:"AssumedRoleUser"`
		} `xml:"AssumeRoleResult"`
	}
	require.NoError(t, xml.Unmarshal(recorder.Body.Bytes(), &response))
	issued := response.Result.Credentials
	require.True(t, strings.HasPrefix(issued.AccessKeyID, issuedKeyPrefix))
	require.Equal(t, testNow.Add(issuedCredentialLifetime).Format(time.RFC3339), issued.Expiration)
	require.Equal(t, "arn:aws:sts::123456789012:assumed-role/aws-authentication-it/aws-authentication-session",
		response.Result.AssumedRoleUser.Arn)

	c := credential{secretAccessKey: issued.SecretAccessKey, sessionToken: issued.SessionToken}
	accepted := serve(testService(testNow).Handler(),
		signedUpstreamRequest(http.MethodGet, backendURL, "", issued.AccessKeyID, c, testNow))
	require.Equal(t, http.StatusOK, accepted.Code, accepted.Body.String())
	require.Equal(t, issued.AccessKeyID, accepted.Header().Get(HeaderAccessKeyID))

	after := testNow.Add(issuedCredentialLifetime + expiryGrace + time.Second)
	expired := serve(testService(after).Handler(),
		signedUpstreamRequest(http.MethodGet, backendURL, "", issued.AccessKeyID, c, after))
	require.Equal(t, http.StatusForbidden, expired.Code)
	require.Equal(t, codeExpiredToken, errorCode(t, expired))
}

func TestSTSRejectsInvalidRequests(t *testing.T) {
	permanent := fixtureCredentials[permanentKey]
	issuedKey, issued := issueCredentials(testNow, issuedCredentialLifetime)
	cases := []struct {
		name   string
		build  func() *http.Request
		status int
		code   string
	}{
		{"get", func() *http.Request {
			return httptest.NewRequest(http.MethodGet, stsURL, nil)
		}, http.StatusMethodNotAllowed, "InvalidAction"},
		{"unsigned", func() *http.Request {
			return httptest.NewRequest(http.MethodPost, stsURL, strings.NewReader(assumeRoleForm().Encode()))
		}, http.StatusForbidden, codeMissingAuthenticationToken},
		{"wrong secret", func() *http.Request {
			return assumeRoleRequest(assumeRoleForm(), permanentKey,
				credential{secretAccessKey: "testWrongSecretAccessKey0000000000000001"}, testNow)
		}, http.StatusForbidden, codeSignatureDoesNotMatch},
		{"issued credentials as base credentials", func() *http.Request {
			return assumeRoleRequest(assumeRoleForm(), issuedKey, issued, testNow)
		}, http.StatusForbidden, codeInvalidClientTokenID},
		{"another action", func() *http.Request {
			form := assumeRoleForm()
			form.Set("Action", "GetCallerIdentity")
			return assumeRoleRequest(form, permanentKey, permanent, testNow)
		}, http.StatusBadRequest, "InvalidAction"},
		{"missing role ARN", func() *http.Request {
			form := assumeRoleForm()
			form.Del("RoleArn")
			return assumeRoleRequest(form, permanentKey, permanent, testNow)
		}, http.StatusBadRequest, "ValidationError"},
		{"missing session name", func() *http.Request {
			form := assumeRoleForm()
			form.Del("RoleSessionName")
			return assumeRoleRequest(form, permanentKey, permanent, testNow)
		}, http.StatusBadRequest, "ValidationError"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			recorder := serve(testService(testNow).Handler(), tc.build())
			require.Equal(t, tc.status, recorder.Code, recorder.Body.String())
			var response struct {
				Error struct {
					Code string `xml:"Code"`
				} `xml:"Error"`
			}
			require.NoError(t, xml.Unmarshal(recorder.Body.Bytes(), &response))
			require.Equal(t, tc.code, response.Error.Code)
			require.NotContains(t, recorder.Body.String(), "SecretAccessKey")
		})
	}
}

func assumeRoleFormFor(roleName string) url.Values {
	form := assumeRoleForm()
	form.Set("RoleArn", "arn:aws:iam::123456789012:role/"+roleName)
	return form
}

func TestSTSFailsForRolesThatSelectAFailure(t *testing.T) {
	permanent := fixtureCredentials[permanentKey]
	outage, open := time.Unix(4000, 0).UTC(), time.Unix(4010, 0).UTC()
	cases := []struct {
		name   string
		role   string
		at     time.Time
		status int
	}{
		{"unavailable", roleUnavailable, open, http.StatusServiceUnavailable},
		{"intermittent during an outage", roleIntermittent, outage, http.StatusServiceUnavailable},
		{"intermittent outside an outage", roleIntermittent, open, http.StatusOK},
		{"unresponsive", roleUnresponsive, open, http.StatusServiceUnavailable},
		{"short-lived during an outage", roleShortLived, outage, http.StatusOK},
		{"any other role", "aws-authentication-it", outage, http.StatusOK},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := testService(tc.at)
			svc.sts.hold = time.Millisecond
			recorder := serve(svc.Handler(), assumeRoleRequest(assumeRoleFormFor(tc.role), permanentKey, permanent, tc.at))
			require.Equal(t, tc.status, recorder.Code, recorder.Body.String())
			if tc.status == http.StatusOK {
				return
			}
			var response struct {
				Error struct {
					Type string `xml:"Type"`
					Code string `xml:"Code"`
				} `xml:"Error"`
			}
			require.NoError(t, xml.Unmarshal(recorder.Body.Bytes(), &response))
			require.Equal(t, "Receiver", response.Error.Type)
			require.Equal(t, "ServiceUnavailable", response.Error.Code)
		})
	}
}

func TestSTSIssuesShortLivedCredentialsForTheirRoles(t *testing.T) {
	open := time.Unix(4010, 0).UTC()
	expiration := func(role string) string {
		recorder := serve(testService(open).Handler(),
			assumeRoleRequest(assumeRoleFormFor(role), permanentKey, fixtureCredentials[permanentKey], open))
		require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
		var response struct {
			Expiration string `xml:"AssumeRoleResult>Credentials>Expiration"`
		}
		require.NoError(t, xml.Unmarshal(recorder.Body.Bytes(), &response))
		return response.Expiration
	}
	require.Equal(t, open.Add(shortCredentialLifetime).Format(time.RFC3339), expiration(roleIntermittent))
	require.Equal(t, open.Add(shortCredentialLifetime).Format(time.RFC3339), expiration(roleShortLived))
	require.Equal(t, open.Add(issuedCredentialLifetime).Format(time.RFC3339), expiration("aws-authentication-it"))
	require.Less(t, shortCredentialLifetime, outageDuration)
}

func TestSTSFailureFollowsVerification(t *testing.T) {
	recorder := serve(testService(testNow).Handler(), assumeRoleRequest(assumeRoleFormFor(roleUnavailable), permanentKey,
		credential{secretAccessKey: "testWrongSecretAccessKey0000000000000001"}, testNow))
	require.Equal(t, http.StatusForbidden, recorder.Code, recorder.Body.String())
}

func TestSTSReleasesAnUnresponsiveRequestWhenTheCallerLeaves(t *testing.T) {
	svc := testService(testNow)
	svc.sts.hold = time.Hour
	ctx, cancel := context.WithCancel(context.Background())
	request := assumeRoleRequest(assumeRoleFormFor(roleUnresponsive), permanentKey, fixtureCredentials[permanentKey], testNow).WithContext(ctx)
	done := make(chan *httptest.ResponseRecorder)
	go func() { done <- serve(svc.Handler(), request) }()
	cancel()
	select {
	case recorder := <-done:
		require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	case <-time.After(5 * time.Second):
		t.Fatal("the unresponsive request was not released when its caller left")
	}
}

// SigV4 requires every x-amz-* header to be signed: one added after signing is rejected, and the
// same header signed is accepted.
func TestUpstreamRequiresEveryAmzHeaderToBeSigned(t *testing.T) {
	permanent := fixtureCredentials[permanentKey]

	unsigned := signedUpstreamRequest(http.MethodPost, backendURL, "{}", permanentKey, permanent, testNow)
	unsigned.Header.Set("X-Amz-Target", "OrdersService.CreateOrder")
	recorder := serve(testService(testNow).Handler(), unsigned)
	require.Equal(t, http.StatusForbidden, recorder.Code, recorder.Body.String())
	require.Equal(t, codeUnsignedHeaders, errorCode(t, recorder))
	require.Contains(t, recorder.Body.String(), "x-amz-target")

	signed := httptest.NewRequest(http.MethodPost, backendURL, strings.NewReader("{}"))
	signed.Host = "testbench"
	signed.Header.Set("Content-Type", "application/json")
	signed.Header.Set("X-Amz-Target", "OrdersService.CreateOrder")
	signRequestWithHeaders(signed, []byte("{}"), permanentKey, permanent, "execute-api", "us-east-1", testNow,
		"X-Amz-Target")
	recorder = serve(testService(testNow).Handler(), signed)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, permanentKey, recorder.Header().Get(HeaderAccessKeyID))
}

// The receipt grace accepts a request signed before its credentials expired and received just
// after, but not one signed after they expired.
func TestCredentialsExpiryIsJudgedWhenSignedAndWhenReceived(t *testing.T) {
	const lifetime = time.Minute
	accessKeyID, issued := issueCredentials(testNow.Add(-lifetime), lifetime)
	expiry := issued.expiresAt
	for _, tc := range []struct {
		name               string
		signedAt, received time.Time
		status             int
		code               string
	}{
		{"signed before expiry, received within the grace", expiry.Add(-time.Second), expiry.Add(expiryGrace - time.Second),
			http.StatusOK, ""},
		{"signed after expiry, received within the grace", expiry.Add(time.Second), expiry.Add(2 * time.Second),
			http.StatusForbidden, codeExpiredToken},
		{"signed before expiry, received after the grace", expiry.Add(-time.Second), expiry.Add(expiryGrace + time.Second),
			http.StatusForbidden, codeExpiredToken},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := signedUpstreamRequest(http.MethodGet, backendURL, "", accessKeyID, issued, tc.signedAt)
			recorder := serve(testService(tc.received).Handler(), r)
			require.Equal(t, tc.status, recorder.Code, recorder.Body.String())
			if tc.code != "" {
				require.Equal(t, tc.code, errorCode(t, recorder))
			}
		})
	}
}

func TestInOutage(t *testing.T) {
	for seconds, want := range map[int64]bool{4000: true, 4009: true, 4010: false, 4019: false, 4020: true} {
		require.Equal(t, want, inOutage(time.Unix(seconds, 0)), "unix %d", seconds)
	}
	require.True(t, inOutage(time.Unix(4009, 999_999_999)))
}

// Bedrock, the AWS Bedrock Guardrail policy's ApplyGuardrail, on its own port (bedrock.go). The
// first tests are those of the Bedrock service from before it moved into this package; their
// requests are now signed and name the fixture guardrail.

const bedrockApplyPath = "/guardrail/test-guardrail-id/version/DRAFT/apply"

// bedrockApplyBody is an ApplyGuardrail request body for text.
func bedrockApplyBody(text string) string {
	content, _ := json.Marshal(text)
	return `{"source":"INPUT","content":[{"text":{"text":` + string(content) + `}}]}`
}

// signedBedrockRequest is an ApplyGuardrail request for path, signed at at with the given
// credential for the given service and region, as the AWS SDK signs it.
func signedBedrockRequest(path, body, accessKeyID string, c credential, service, region string, at time.Time) *http.Request {
	r := httptest.NewRequest(http.MethodPost, "http://testbench:3006"+path, strings.NewReader(body))
	r.Host = "testbench:3006"
	r.Header.Set("Content-Type", "application/json")
	signRequest(r, []byte(body), accessKeyID, c, service, region, at)
	return r
}

// bedrockApplyRequest is an ApplyGuardrail request for text to the fixture guardrail, signed now
// with a permanent fixture credential.
func bedrockApplyRequest(text string) *http.Request {
	return signedBedrockRequest(bedrockApplyPath, bedrockApplyBody(text), permanentKey,
		fixtureCredentials[permanentKey], serviceBedrock, bedrockRegion, time.Now())
}

func decodeBedrockResponse(t *testing.T, recorder *httptest.ResponseRecorder) applyGuardrailResponse {
	t.Helper()
	var resp applyGuardrailResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	return resp
}

func TestApplyGuardrailSafeContentReturnsNoneWithEchoedOutput(t *testing.T) {
	recorder := httptest.NewRecorder()
	NewBedrock().Handler().ServeHTTP(recorder, bedrockApplyRequest("Hello, this is safe content"))

	require.Equal(t, http.StatusOK, recorder.Code)
	resp := decodeBedrockResponse(t, recorder)
	require.Equal(t, "NONE", resp.Action)
	// A caller reading outputs[0].text (as real Bedrock guarantees is always present, even
	// for action NONE) must not find an empty slice here.
	require.Len(t, resp.Outputs, 1)
	require.Equal(t, "Hello, this is safe content", resp.Outputs[0]["text"])
	require.Empty(t, resp.Assessments)
}

func TestApplyGuardrailEmptyContentReturnsNoneWithEmptyEchoedOutput(t *testing.T) {
	recorder := httptest.NewRecorder()
	NewBedrock().Handler().ServeHTTP(recorder, bedrockApplyRequest(""))

	require.Equal(t, http.StatusOK, recorder.Code)
	resp := decodeBedrockResponse(t, recorder)
	require.Equal(t, "NONE", resp.Action)
	require.Len(t, resp.Outputs, 1)
	require.Equal(t, "", resp.Outputs[0]["text"])
}

func TestApplyGuardrailViolatingKeywordsAreIntervened(t *testing.T) {
	for _, keyword := range []string{"violence", "hate", "illegal"} {
		t.Run(keyword, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			NewBedrock().Handler().ServeHTTP(recorder, bedrockApplyRequest("this contains "+keyword))

			require.Equal(t, http.StatusOK, recorder.Code)
			resp := decodeBedrockResponse(t, recorder)
			require.Equal(t, "GUARDRAIL_INTERVENED", resp.Action)
			require.Len(t, resp.Outputs, 1)
			require.NotNil(t, resp.Assessments[0].ContentPolicy)
		})
	}
}

func TestApplyGuardrailSimulatedErrorKeywordsReturn500(t *testing.T) {
	recorder := httptest.NewRecorder()
	NewBedrock().Handler().ServeHTTP(recorder, bedrockApplyRequest("please simulate an error here"))

	require.Equal(t, http.StatusInternalServerError, recorder.Code)
	require.Equal(t, "InternalServerException", errorCode(t, recorder))
}

func TestApplyGuardrailMaskedEmailIsAnonymizedThenRestorable(t *testing.T) {
	recorder := httptest.NewRecorder()
	NewBedrock().Handler().ServeHTTP(recorder, bedrockApplyRequest("Contact me at mask-test@example.com"))

	require.Equal(t, http.StatusOK, recorder.Code)
	resp := decodeBedrockResponse(t, recorder)
	require.Equal(t, "GUARDRAIL_INTERVENED", resp.Action)
	text, _ := resp.Outputs[0]["text"].(string)
	require.Contains(t, text, "$ANONYMIZED_EMAIL$")
	require.NotContains(t, text, "mask-test@example.com")
	require.NotNil(t, resp.Assessments[0].SensitiveInformationPolicy)
}

func TestApplyGuardrailPlainEmailIsPermanentlyRedacted(t *testing.T) {
	recorder := httptest.NewRecorder()
	NewBedrock().Handler().ServeHTTP(recorder, bedrockApplyRequest("My SSN is test@example.com"))

	require.Equal(t, http.StatusOK, recorder.Code)
	resp := decodeBedrockResponse(t, recorder)
	text, _ := resp.Outputs[0]["text"].(string)
	require.Contains(t, text, "*****")
	require.NotContains(t, text, "test@example.com")
}

func TestApplyGuardrailInvalidJSONReturns400(t *testing.T) {
	recorder := httptest.NewRecorder()
	req := signedBedrockRequest(bedrockApplyPath, "{invalid", permanentKey, fixtureCredentials[permanentKey],
		serviceBedrock, bedrockRegion, time.Now())
	NewBedrock().Handler().ServeHTTP(recorder, req)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestApplyGuardrailRejectsMalformedGuardrailPath(t *testing.T) {
	for _, path := range []string{"/guardrail/x/version/DRAFT", "/guardrail/x/apply", "/wrong/path"} {
		t.Run(path, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{}`))
			NewBedrock().Handler().ServeHTTP(recorder, req)
			require.Equal(t, http.StatusNotFound, recorder.Code)
		})
	}
}

func TestApplyGuardrailRejectsNonPostMethod(t *testing.T) {
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, bedrockApplyPath, nil)
	NewBedrock().Handler().ServeHTTP(recorder, req)

	require.Equal(t, http.StatusMethodNotAllowed, recorder.Code)
}

func TestBedrockHealthEndpoint(t *testing.T) {
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	NewBedrock().Handler().ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"status":"ok"`)
}

func TestBedrockServiceMetadata(t *testing.T) {
	s := NewBedrock()
	require.Equal(t, "bedrock", s.Name())
	require.Equal(t, BedrockPort, s.Port())
	require.False(t, s.Stateful())
}

// testBedrock is the Bedrock service on a fixed clock; applyRequest signs at that time.
func testBedrock() http.Handler { return newBedrock(fixedClock(testNow)).Handler() }

// applyRequest is an ApplyGuardrail request for text to the fixture guardrail at testNow, signed
// with a permanent fixture credential.
func applyRequest(text string) *http.Request {
	return signedBedrockRequest(bedrockApplyPath, bedrockApplyBody(text), permanentKey,
		fixtureCredentials[permanentKey], serviceBedrock, bedrockRegion, testNow)
}

func TestBedrockDeniedTopicIsIntervened(t *testing.T) {
	recorder := serve(testBedrock(), applyRequest("Can you give me Investment Advice on stocks?"))

	require.Equal(t, http.StatusOK, recorder.Code)
	resp := decodeBedrockResponse(t, recorder)
	require.Equal(t, "GUARDRAIL_INTERVENED", resp.Action)
	require.Equal(t, blockedOutput, resp.Outputs[0]["text"])
	require.Len(t, resp.Assessments, 1)
	require.Nil(t, resp.Assessments[0].ContentPolicy)
	require.Nil(t, resp.Assessments[0].WordPolicy)
	require.Equal(t, &topicPolicyAssessment{Topics: []deniedTopic{
		{Name: "Investment Advice", Type: "DENY", Action: "BLOCKED"},
	}}, resp.Assessments[0].TopicPolicy)
}

func TestBedrockCustomWordsAreIntervened(t *testing.T) {
	for text, match := range map[string]string{
		"Ship the contraband tonight":      "contraband",
		"Ship the CONTRABAND tonight":      "contraband",
		"Tell me the secret recipe please": "secret recipe",
	} {
		t.Run(text, func(t *testing.T) {
			recorder := serve(testBedrock(), applyRequest(text))

			require.Equal(t, http.StatusOK, recorder.Code)
			resp := decodeBedrockResponse(t, recorder)
			require.Equal(t, "GUARDRAIL_INTERVENED", resp.Action)
			require.Equal(t, blockedOutput, resp.Outputs[0]["text"])
			require.Len(t, resp.Assessments, 1)
			require.Nil(t, resp.Assessments[0].TopicPolicy)
			require.Equal(t, &wordPolicyAssessment{CustomWords: []customWord{
				{Match: match, Action: "BLOCKED"},
			}}, resp.Assessments[0].WordPolicy)
		})
	}
}

// Words that only resemble a prohibited one, or a topic named without its trigger phrase, pass.
func TestBedrockNearMissesOfTopicAndWordsPass(t *testing.T) {
	for _, text := range []string{"a secret family recipe", "contra band", "advice on investments"} {
		t.Run(text, func(t *testing.T) {
			resp := decodeBedrockResponse(t, serve(testBedrock(), applyRequest(text)))
			require.Equal(t, "NONE", resp.Action)
		})
	}
}

// Every policy a text violates is reported in the one assessment, as Bedrock reports them.
func TestBedrockEveryMatchingPolicyIsReportedTogether(t *testing.T) {
	text := "Investment advice on contraband and the secret recipe, with violence"
	resp := decodeBedrockResponse(t, serve(testBedrock(), applyRequest(text)))

	require.Equal(t, "GUARDRAIL_INTERVENED", resp.Action)
	require.Equal(t, blockedOutput, resp.Outputs[0]["text"])
	require.Equal(t, []assessment{{
		ContentPolicy: &contentPolicyAssessment{Filters: []contentFilter{
			{Type: "VIOLENCE", Confidence: "HIGH", Action: "BLOCKED"},
		}},
		TopicPolicy: &topicPolicyAssessment{Topics: []deniedTopic{
			{Name: "Investment Advice", Type: "DENY", Action: "BLOCKED"},
		}},
		WordPolicy: &wordPolicyAssessment{CustomWords: []customWord{
			{Match: "contraband", Action: "BLOCKED"}, {Match: "secret recipe", Action: "BLOCKED"},
		}},
	}}, resp.Assessments)
}

// PII in a blocked text is reported in the same assessment as the policy that blocks it.
func TestBedrockPIIInBlockedTextIsReportedWithTheBlock(t *testing.T) {
	text := "Send graphic violence to jane.doe@example.com"
	resp := decodeBedrockResponse(t, serve(testBedrock(), applyRequest(text)))

	require.Equal(t, "GUARDRAIL_INTERVENED", resp.Action)
	require.Equal(t, blockedOutput, resp.Outputs[0]["text"])
	require.Len(t, resp.Assessments, 1)
	require.NotNil(t, resp.Assessments[0].ContentPolicy)
	require.Equal(t, &sensitiveInformationPolicyAssessment{PiiEntities: []piiEntity{
		{Type: "EMAIL", Match: "jane.doe@example.com", Action: "ANONYMIZED"},
	}}, resp.Assessments[0].SensitiveInformationPolicy)
}

func TestBedrockEachPIITypeIsAnonymized(t *testing.T) {
	for text, want := range map[string]piiEntity{
		"Write to Jane.Doe@EXAMPLE.com today": {Type: "EMAIL", Match: "Jane.Doe@EXAMPLE.com", Action: "ANONYMIZED"},
		"Call 555-123-4567 after noon":        {Type: "PHONE", Match: "555-123-4567", Action: "ANONYMIZED"},
		"SSN 123-45-6789 on file":             {Type: "US_SOCIAL_SECURITY_NUMBER", Match: "123-45-6789", Action: "ANONYMIZED"},
	} {
		t.Run(text, func(t *testing.T) {
			resp := decodeBedrockResponse(t, serve(testBedrock(), applyRequest(text)))
			require.Equal(t, "GUARDRAIL_INTERVENED", resp.Action)
			require.Len(t, resp.Assessments, 1)
			require.Equal(t, &sensitiveInformationPolicyAssessment{PiiEntities: []piiEntity{want}},
				resp.Assessments[0].SensitiveInformationPolicy)
			require.NotContains(t, resp.Outputs[0]["text"], want.Match)
		})
	}
}

// Every PII type in one text is reported in one assessment, in the order the matches appear.
func TestBedrockMixedPIITypesAreReportedTogether(t *testing.T) {
	text := "Email jane.doe@example.com, call 555-123-4567, SSN 123-45-6789, again 555-123-4567"
	resp := decodeBedrockResponse(t, serve(testBedrock(), applyRequest(text)))

	require.Equal(t, "GUARDRAIL_INTERVENED", resp.Action)
	require.Len(t, resp.Assessments, 1)
	require.Equal(t, []piiEntity{
		{Type: "EMAIL", Match: "jane.doe@example.com", Action: "ANONYMIZED"},
		{Type: "PHONE", Match: "555-123-4567", Action: "ANONYMIZED"},
		{Type: "US_SOCIAL_SECURITY_NUMBER", Match: "123-45-6789", Action: "ANONYMIZED"},
		{Type: "PHONE", Match: "555-123-4567", Action: "ANONYMIZED"},
	}, resp.Assessments[0].SensitiveInformationPolicy.PiiEntities)
	require.Equal(t, "Email *****, call *****, SSN *****, again *****", resp.Outputs[0]["text"])
}

// Values that only resemble a filtered type are not PII, and neither is an address at another
// domain.
func TestBedrockPIINearMissesPass(t *testing.T) {
	for _, text := range []string{
		"due 2025-01-01", "id 123456789", "call 5551234567", "ref 1234-567-8901",
		"jane@example.org", "jane@example.community",
	} {
		t.Run(text, func(t *testing.T) {
			resp := decodeBedrockResponse(t, serve(testBedrock(), applyRequest(text)))
			require.Equal(t, "NONE", resp.Action)
		})
	}
}

// Bedrock accepts the fixture and STS-issued credentials, and the two key pairs suites signed
// their Bedrock requests with before signatures were verified.
func TestBedrockAcceptsEveryValidCredential(t *testing.T) {
	issuedKey, issued := issueCredentials(testNow, time.Hour)
	for key, c := range map[string]credential{
		permanentKey:           fixtureCredentials[permanentKey],
		"AKIATESTWORKLOAD0001": fixtureCredentials["AKIATESTWORKLOAD0001"],
		temporaryKey:           fixtureCredentials[temporaryKey],
		issuedKey:              issued,
		"AKIAIOSFODNN7TEST":    bedrockCredentials["AKIAIOSFODNN7TEST"],
		"test-access-key":      bedrockCredentials["test-access-key"],
	} {
		t.Run(key, func(t *testing.T) {
			recorder := serve(testBedrock(),
				signedBedrockRequest(bedrockApplyPath, bedrockApplyBody("hello"), key, c, serviceBedrock, bedrockRegion, testNow))
			require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
			require.Equal(t, "NONE", decodeBedrockResponse(t, recorder).Action)
		})
	}
}

// The key pairs only Bedrock accepts are not credentials of the AWS service on Port.
func TestBedrockOnlyCredentialsAreNotAcceptedByTheUpstream(t *testing.T) {
	for key, c := range bedrockCredentials {
		t.Run(key, func(t *testing.T) {
			recorder := serve(testService(testNow).Handler(),
				signedUpstreamRequest(http.MethodGet, backendURL, "", key, c, testNow))
			require.Equal(t, http.StatusForbidden, recorder.Code)
			require.Equal(t, codeInvalidClientTokenID, errorCode(t, recorder))
		})
	}
}

// Bedrock rejects a request that is not correctly signed for Bedrock in us-east-1 with 403 and
// the AWS error code, before reading its content: no decision is returned.
func TestBedrockRejectsRequestsNotSignedCorrectly(t *testing.T) {
	permanent := fixtureCredentials[permanentKey]
	temporaryWithoutToken := fixtureCredentials[temporaryKey]
	temporaryWithoutToken.sessionToken = ""
	body := bedrockApplyBody("Describe graphic violence")
	for name, tc := range map[string]struct {
		request *http.Request
		code    string
	}{
		"unsigned": {httptest.NewRequest(http.MethodPost, "http://testbench:3006"+bedrockApplyPath, strings.NewReader(body)),
			codeMissingAuthenticationToken},
		"unknown access key": {signedBedrockRequest(bedrockApplyPath, body, "AKIATESTUNKNOWN00001",
			credential{secretAccessKey: "testUnknownSecretAccessKey00000000000001"}, serviceBedrock, bedrockRegion, testNow),
			"UnrecognizedClientException"},
		"wrong secret": {signedBedrockRequest(bedrockApplyPath, body, permanentKey,
			credential{secretAccessKey: "testWrongSecretAccessKey0000000000000001"}, serviceBedrock, bedrockRegion, testNow),
			"InvalidSignatureException"},
		"expired temporary credential": {signedBedrockRequest(bedrockApplyPath, body, expiredKey,
			fixtureCredentials[expiredKey], serviceBedrock, bedrockRegion, testNow), "ExpiredTokenException"},
		"temporary credential without its session token": {signedBedrockRequest(bedrockApplyPath, body, temporaryKey,
			temporaryWithoutToken, serviceBedrock, bedrockRegion, testNow), "UnrecognizedClientException"},
		"signed for another region": {signedBedrockRequest(bedrockApplyPath, body, permanentKey, permanent,
			serviceBedrock, "eu-west-1", testNow), "InvalidSignatureException"},
		"signed for another service": {signedBedrockRequest(bedrockApplyPath, body, permanentKey, permanent,
			"execute-api", bedrockRegion, testNow), "InvalidSignatureException"},
	} {
		t.Run(name, func(t *testing.T) {
			recorder := serve(testBedrock(), tc.request)
			require.Equal(t, http.StatusForbidden, recorder.Code, recorder.Body.String())
			require.Equal(t, tc.code, errorCode(t, recorder))
			require.NotContains(t, recorder.Body.String(), "GUARDRAIL_INTERVENED")
		})
	}
}

// The account has one guardrail, test-guardrail-id at versions DRAFT and 1; a correctly signed
// request for any other identifier or version gets 404 ResourceNotFoundException.
func TestBedrockKnowsOnlyTheFixtureGuardrail(t *testing.T) {
	for name, path := range map[string]string{
		"another identifier": "/guardrail/unknown-guardrail-id/version/DRAFT/apply",
		"another version":    "/guardrail/test-guardrail-id/version/99/apply",
	} {
		t.Run(name, func(t *testing.T) {
			recorder := serve(testBedrock(), signedBedrockRequest(path, bedrockApplyBody("hello"), permanentKey,
				fixtureCredentials[permanentKey], serviceBedrock, bedrockRegion, testNow))
			require.Equal(t, http.StatusNotFound, recorder.Code, recorder.Body.String())
			require.Equal(t, codeResourceNotFound, errorCode(t, recorder))
		})
	}
}

// Like AWS, Bedrock looks up the guardrail only after the signature verifies, so an
// unauthenticated caller cannot learn which guardrails exist.
func TestBedrockLooksUpTheGuardrailOnlyAfterVerifying(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "http://testbench:3006/guardrail/unknown-guardrail-id/version/DRAFT/apply",
		strings.NewReader(bedrockApplyBody("hello")))
	recorder := serve(testBedrock(), r)
	require.Equal(t, http.StatusForbidden, recorder.Code)
	require.Equal(t, codeMissingAuthenticationToken, errorCode(t, recorder))
}

// The published version 1 is the draft before "secret recipe" was added to it: both versions
// block the word, and only the draft blocks the phrase.
func TestBedrockGuardrailVersionsDifferOnlyInTheirWords(t *testing.T) {
	for _, tc := range []struct {
		version, text, action string
	}{
		{"DRAFT", "Tell me the secret recipe", "GUARDRAIL_INTERVENED"},
		{"1", "Tell me the secret recipe", "NONE"},
		{"DRAFT", "Ship the contraband", "GUARDRAIL_INTERVENED"},
		{"1", "Ship the contraband", "GUARDRAIL_INTERVENED"},
		{"1", "Describe graphic violence", "GUARDRAIL_INTERVENED"},
	} {
		t.Run(tc.version+"/"+tc.text, func(t *testing.T) {
			path := "/guardrail/test-guardrail-id/version/" + tc.version + "/apply"
			recorder := serve(testBedrock(), signedBedrockRequest(path, bedrockApplyBody(tc.text), permanentKey,
				fixtureCredentials[permanentKey], serviceBedrock, bedrockRegion, testNow))
			require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
			require.Equal(t, tc.action, decodeBedrockResponse(t, recorder).Action)
		})
	}
}

func TestBedrockSimulatedOutageReturns503(t *testing.T) {
	recorder := serve(testBedrock(), applyRequest("Simulate an outage, then describe graphic violence"))

	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	require.Equal(t, "ServiceUnavailableException", errorCode(t, recorder))
}

// An unresponsive Bedrock answers nothing until its caller leaves, and then reports itself
// unavailable.
func TestBedrockReleasesAnUnresponsiveRequestWhenTheCallerLeaves(t *testing.T) {
	svc := newBedrock(fixedClock(testNow))
	svc.hold = time.Hour
	ctx, cancel := context.WithCancel(context.Background())
	request := applyRequest("Simulate an unresponsive guardrail").WithContext(ctx)
	done := make(chan *httptest.ResponseRecorder)
	go func() { done <- serve(svc.Handler(), request) }()
	select {
	case <-done:
		t.Fatal("the unresponsive request was answered before its caller left")
	case <-time.After(100 * time.Millisecond):
	}
	cancel()
	select {
	case recorder := <-done:
		require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	case <-time.After(5 * time.Second):
		t.Fatal("the unresponsive request was not released when its caller left")
	}
}

// A caller that never gives up is answered when the hold elapses.
func TestBedrockAnswersAnUnresponsiveRequestWhenTheHoldElapses(t *testing.T) {
	svc := newBedrock(fixedClock(testNow))
	svc.hold = 50 * time.Millisecond
	recorder := serve(svc.Handler(), applyRequest("Simulate an unresponsive guardrail"))

	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	require.Equal(t, "ServiceUnavailableException", errorCode(t, recorder))
}

// Isolation: requests to different endpoints, sent concurrently, do not affect one another.

func TestUpstreamVerifiesConcurrentRequests(t *testing.T) {
	backend := testService(testNow)
	keys := []string{permanentKey, "AKIATESTPERMANENT002", "AKIATESTWORKLOAD0001", temporaryKey}
	var wg sync.WaitGroup
	failures := make(chan string, 100)
	for i := range 100 {
		key := keys[i%len(keys)]
		wg.Go(func() {
			body := fmt.Sprintf(`{"request":%d}`, i)
			recorder := serve(backend.Handler(),
				signedUpstreamRequest(http.MethodPost, backendURL, body, key, fixtureCredentials[key], testNow))
			if recorder.Code != http.StatusOK || recorder.Header().Get(HeaderAccessKeyID) != key {
				failures <- fmt.Sprintf("request %d: status %d, key %q", i, recorder.Code, recorder.Header().Get(HeaderAccessKeyID))
			}
		})
	}
	wg.Wait()
	close(failures)
	for failure := range failures {
		t.Error(failure)
	}
}

// Valid and invalid requests are sent at once to both services of the package: upstream requests
// with and without a request-scoped key revocation, STS AssumeRole for a normal and an unavailable
// role, and Bedrock ApplyGuardrail for safe, blocked and error-simulating content. Each must get
// exactly the answer it gets alone, which shows that no answer depends on another request.
func TestConcurrentRequestsToEveryEndpointAreIndependent(t *testing.T) {
	handler := testService(testNow).Handler()
	bedrock := NewBedrock().Handler()
	permanent := fixtureCredentials[permanentKey]
	type check struct {
		name   string
		build  func(i int) *http.Request
		verify func(i int, recorder *httptest.ResponseRecorder) error
	}
	expect := func(status int, code string) func(int, *httptest.ResponseRecorder) error {
		return func(_ int, recorder *httptest.ResponseRecorder) error {
			got := ""
			if recorder.Code != http.StatusOK {
				got = recorder.Header().Get("X-Amzn-ErrorType")
				if got == "" {
					got = recorder.Body.String()
				}
			}
			if recorder.Code != status || (code != "" && !strings.Contains(got, code)) {
				return fmt.Errorf("got %d %q, want %d %q", recorder.Code, got, status, code)
			}
			return nil
		}
	}
	checks := []check{
		{"upstream, valid", func(i int) *http.Request {
			return signedUpstreamRequest(http.MethodPost, backendURL, fmt.Sprintf(`{"n":%d}`, i), permanentKey, permanent, testNow)
		}, func(i int, recorder *httptest.ResponseRecorder) error {
			var reflected reflection
			if recorder.Code != http.StatusOK || json.Unmarshal(recorder.Body.Bytes(), &reflected) != nil ||
				reflected.AccessKeyID != permanentKey || reflected.Body != fmt.Sprintf(`{"n":%d}`, i) {
				return fmt.Errorf("got %d %s", recorder.Code, recorder.Body.String())
			}
			return nil
		}},
		{"upstream, key revoked for this request only", func(int) *http.Request {
			r := signedUpstreamRequest(http.MethodGet, backendURL, "", permanentKey, permanent, testNow)
			r.Header.Set(HeaderRevokedAccessKeys, permanentKey)
			return r
		}, expect(http.StatusForbidden, codeInvalidClientTokenID)},
		{"upstream, wrong secret", func(int) *http.Request {
			return signedUpstreamRequest(http.MethodGet, backendURL, "", permanentKey,
				credential{secretAccessKey: "testWrongSecretAccessKey0000000000000001"}, testNow)
		}, expect(http.StatusForbidden, codeSignatureDoesNotMatch)},
		{"STS, AssumeRole", func(int) *http.Request {
			return assumeRoleRequest(assumeRoleForm(), permanentKey, permanent, testNow)
		}, func(_ int, recorder *httptest.ResponseRecorder) error {
			if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), "<AccessKeyId>"+issuedKeyPrefix) {
				return fmt.Errorf("got %d %s", recorder.Code, recorder.Body.String())
			}
			return nil
		}},
		{"STS, unavailable role", func(int) *http.Request {
			return assumeRoleRequest(assumeRoleFormFor(roleUnavailable), permanentKey, permanent, testNow)
		}, expect(http.StatusServiceUnavailable, "ServiceUnavailable")},
	}
	bedrockChecks := []check{
		{"Bedrock, safe content", func(i int) *http.Request {
			return bedrockApplyRequest(fmt.Sprintf("safe request %d", i))
		}, func(i int, recorder *httptest.ResponseRecorder) error {
			var resp applyGuardrailResponse
			if recorder.Code != http.StatusOK || json.Unmarshal(recorder.Body.Bytes(), &resp) != nil ||
				resp.Action != "NONE" || resp.Outputs[0]["text"] != fmt.Sprintf("safe request %d", i) {
				return fmt.Errorf("got %d %s", recorder.Code, recorder.Body.String())
			}
			return nil
		}},
		{"Bedrock, blocked content", func(int) *http.Request {
			return bedrockApplyRequest("this is about violence")
		}, func(_ int, recorder *httptest.ResponseRecorder) error {
			if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), "GUARDRAIL_INTERVENED") {
				return fmt.Errorf("got %d %s", recorder.Code, recorder.Body.String())
			}
			return nil
		}},
		{"Bedrock, simulated error", func(int) *http.Request {
			return bedrockApplyRequest("please simulate an error")
		}, expect(http.StatusInternalServerError, "InternalServerException")},
	}

	targets := []struct {
		handler http.Handler
		checks  []check
	}{{handler, checks}, {bedrock, bedrockChecks}}

	const rounds = 40
	var wg sync.WaitGroup
	failures := make(chan string, rounds*(len(checks)+len(bedrockChecks)))
	for i := range rounds {
		for _, target := range targets {
			for _, c := range target.checks {
				wg.Go(func() {
					if err := c.verify(i, serve(target.handler, c.build(i))); err != nil {
						failures <- fmt.Sprintf("%s, request %d: %v", c.name, i, err)
					}
				})
			}
		}
	}
	wg.Wait()
	close(failures)
	for failure := range failures {
		t.Error(failure)
	}
}
