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

package kernel

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/faultformat"
	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

// soapCtx is an execution context for a SoapApi route whose request carried contentType,
// using the shipped default kind set rather than enabling the kind by hand.
func soapCtx(t *testing.T, contentType string) *PolicyExecutionContext {
	t.Helper()
	ec := faultExecCtx(t, nil, nil)
	ec.server.errorFormatterKinds = faultformat.SupportedKinds()
	ec.sharedCtx.APIKind = policy.APIKindSoapApi
	if contentType != "" {
		ec.requestHeaderCtx.Headers = policy.NewHeaders(map[string][]string{"content-type": {contentType}})
	}
	return ec
}

// A policy-described rejection on a SoapApi route reaches the caller as a Fault in the version
// it spoke, with the status untouched.
func TestErrorFormat_SoapApiRendersAFaultInTheCallersVersion(t *testing.T) {
	cases := []struct {
		contentType string
		wantCT      string
		wantNS      string
	}{
		{"text/xml; charset=utf-8", "text/xml; charset=utf-8", "http://schemas.xmlsoap.org/soap/envelope/"},
		{"application/soap+xml; charset=utf-8", "application/soap+xml; charset=utf-8", "http://www.w3.org/2003/05/soap-envelope"},
		{"", "text/xml; charset=utf-8", "http://schemas.xmlsoap.org/soap/envelope/"}, // GET ?wsdl
	}
	for _, tc := range cases {
		ec := soapCtx(t, tc.contentType)
		out := ec.runFaultPoliciesOnRejection(context.Background(), policy.ImmediateResponse{
			StatusCode: 401,
			// A fallback body alongside the description, as a migrated policy sends it.
			Body:  []byte(`{"error":"unauthorized"}`),
			Fault: &policy.FaultDetails{Code: "900901", Message: "Invalid or expired credentials."},
		})

		assert.Equal(t, 401, out.StatusCode, "formatting must never change the status")
		assert.Equal(t, tc.wantCT, out.Headers[contentTypeHeader], tc.contentType)
		require.NotNil(t, out.Body)
		assert.True(t, strings.Contains(string(out.Body), tc.wantNS), "%q: %s", tc.contentType, out.Body)
		assert.Contains(t, string(out.Body), "Invalid or expired credentials.")
	}
}

// A policy that writes its own body and describes nothing keeps it — the override path is the
// same on a SoapApi route as everywhere else.
func TestErrorFormat_SoapApiAuthoredBodyIsLeftAlone(t *testing.T) {
	ec := soapCtx(t, "text/xml")
	custom := []byte(`<soapenv:Envelope/>`)

	out := ec.runFaultPoliciesOnRejection(context.Background(), policy.ImmediateResponse{
		StatusCode: 403,
		Body:       custom,
	})

	assert.Equal(t, custom, out.Body)
	assert.Empty(t, out.Headers[contentTypeHeader])
}
