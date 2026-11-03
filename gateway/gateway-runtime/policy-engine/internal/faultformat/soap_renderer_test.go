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

package faultformat

import (
	"encoding/xml"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

// soap11Doc and soap12Doc parse a rendered fault back, so the assertions are on what a SOAP
// stack reads rather than on byte layout.
type soap11Doc struct {
	XMLName xml.Name `xml:"http://schemas.xmlsoap.org/soap/envelope/ Envelope"`
	Fault   struct {
		Code   string     `xml:"faultcode"`
		String string     `xml:"faultstring"`
		Detail *detailDoc `xml:"detail"`
	} `xml:"http://schemas.xmlsoap.org/soap/envelope/ Body>Fault"`
}

type soap12Doc struct {
	XMLName xml.Name `xml:"http://www.w3.org/2003/05/soap-envelope Envelope"`
	Fault   struct {
		Code   string     `xml:"http://www.w3.org/2003/05/soap-envelope Code>Value"`
		Reason string     `xml:"http://www.w3.org/2003/05/soap-envelope Reason>Text"`
		Detail *detailDoc `xml:"http://www.w3.org/2003/05/soap-envelope Detail"`
	} `xml:"http://www.w3.org/2003/05/soap-envelope Body>Fault"`
}

type detailDoc struct {
	Error *struct {
		Code    string `xml:"urn:wso2:api-platform:gateway:fault code"`
		Type    string `xml:"urn:wso2:api-platform:gateway:fault type"`
		ErrorID string `xml:"urn:wso2:api-platform:gateway:fault errorId"`
	} `xml:"urn:wso2:api-platform:gateway:fault error"`
}

func TestRender_SOAP11(t *testing.T) {
	body, ct, ok := NewRegistry().Render(ShapeSOAP11, RenderInput{
		Err:    policy.FaultDetails{Code: "900901", Type: "authentication", Message: "Invalid or expired credentials."},
		Status: 401,
	})
	require.True(t, ok)
	assert.Equal(t, "text/xml; charset=utf-8", ct)

	var doc soap11Doc
	require.NoError(t, xml.Unmarshal(body, &doc), string(body))
	assert.Equal(t, "soapenv:Client", doc.Fault.Code)
	assert.Equal(t, "Invalid or expired credentials.", doc.Fault.String)
	require.NotNil(t, doc.Fault.Detail)
	require.NotNil(t, doc.Fault.Detail.Error)
	assert.Equal(t, "900901", doc.Fault.Detail.Error.Code)
	assert.Equal(t, "authentication", doc.Fault.Detail.Error.Type)
}

func TestRender_SOAP12(t *testing.T) {
	body, ct, ok := NewRegistry().Render(ShapeSOAP12, RenderInput{
		Err:     policy.FaultDetails{Message: "Upstream unavailable"},
		Status:  503,
		ErrorID: "e-123",
	})
	require.True(t, ok)
	assert.Equal(t, "application/soap+xml; charset=utf-8", ct)

	var doc soap12Doc
	require.NoError(t, xml.Unmarshal(body, &doc), string(body))
	assert.Equal(t, "env:Receiver", doc.Fault.Code)
	assert.Equal(t, "Upstream unavailable", doc.Fault.Reason)
	require.NotNil(t, doc.Fault.Detail)
	require.NotNil(t, doc.Fault.Detail.Error)
	assert.Equal(t, "e-123", doc.Fault.Detail.Error.ErrorID)
}

// The 4xx / other split is the fault code's whole meaning to a SOAP client.
func TestRender_SOAPFaultCodeFollowsStatus(t *testing.T) {
	cases := []struct {
		status int
		soap11 string
		soap12 string
	}{
		{400, "soapenv:Client", "env:Sender"},
		{429, "soapenv:Client", "env:Sender"},
		{500, "soapenv:Server", "env:Receiver"},
		{504, "soapenv:Server", "env:Receiver"},
		{0, "soapenv:Server", "env:Receiver"},
	}
	r := NewRegistry()
	for _, tc := range cases {
		b11, _, _ := r.Render(ShapeSOAP11, RenderInput{Status: tc.status})
		var d11 soap11Doc
		require.NoError(t, xml.Unmarshal(b11, &d11))
		assert.Equal(t, tc.soap11, d11.Fault.Code, "status %d", tc.status)

		b12, _, _ := r.Render(ShapeSOAP12, RenderInput{Status: tc.status})
		var d12 soap12Doc
		require.NoError(t, xml.Unmarshal(b12, &d12))
		assert.Equal(t, tc.soap12, d12.Fault.Code, "status %d", tc.status)
	}
}

// A sparse error still renders a complete fault with a usable message, and no empty detail.
func TestRender_SOAPSparseError(t *testing.T) {
	body, _, ok := NewRegistry().Render(ShapeSOAP11, RenderInput{Status: 500})
	require.True(t, ok)
	var doc soap11Doc
	require.NoError(t, xml.Unmarshal(body, &doc))
	assert.Equal(t, "An unexpected error occurred.", doc.Fault.String)
	assert.Nil(t, doc.Fault.Detail)
	assert.NotContains(t, string(body), "<detail>")
}

// A message carrying markup cannot break out of its element in either version.
func TestRender_SOAPEscapesMessage(t *testing.T) {
	msg := `</faultstring><injected>x</injected>&`
	for _, shape := range []ShapeID{ShapeSOAP11, ShapeSOAP12} {
		body, _, _ := NewRegistry().Render(shape, RenderInput{Err: policy.FaultDetails{Message: msg, Code: "<c>"}, Status: 400})
		assert.NotContains(t, string(body), "<injected>", shape)
		if shape == ShapeSOAP11 {
			var doc soap11Doc
			require.NoError(t, xml.Unmarshal(body, &doc))
			assert.Equal(t, msg, doc.Fault.String)
			assert.Equal(t, "<c>", doc.Fault.Detail.Error.Code)
		} else {
			var doc soap12Doc
			require.NoError(t, xml.Unmarshal(body, &doc))
			assert.Equal(t, msg, doc.Fault.Reason)
		}
	}
}

// Description is never rendered, in SOAP as in every other shape.
func TestRender_SOAPOmitsDescription(t *testing.T) {
	for _, shape := range []ShapeID{ShapeSOAP11, ShapeSOAP12} {
		body, _, _ := NewRegistry().Render(shape, RenderInput{Err: policy.FaultDetails{Message: "Blocked", Description: "secret-content"}, Status: 422})
		assert.NotContains(t, string(body), "secret-content", shape)
	}
}

// End to end through ShouldFormat: a policy-described rejection on a SoapApi route becomes a
// fault in the caller's version, and a policy-authored body is still left alone.
func TestShouldFormat_SoapApi(t *testing.T) {
	r := NewRegistry()
	base := Input{
		FormatterEnabled: SupportedKinds().Enabled(policy.APIKindSoapApi),
		PolicyDescribed:  true,
		Err:              policy.FaultDetails{Message: "Invalid or expired credentials."},
		Status:           401,
		APIKind:          policy.APIKindSoapApi,
		RequestMethod:    "POST",
	}

	in11 := base
	in11.ContentType = "text/xml; charset=utf-8"
	d := ShouldFormat(r, in11)
	require.True(t, d.Format, d.Reason)
	assert.Equal(t, "text/xml; charset=utf-8", d.ContentType)

	in12 := base
	in12.ContentType = "application/soap+xml"
	d = ShouldFormat(r, in12)
	require.True(t, d.Format, d.Reason)
	assert.Equal(t, "application/soap+xml; charset=utf-8", d.ContentType)

	authored := in11
	authored.BodyAuthored = true
	assert.False(t, ShouldFormat(r, authored).Format)
}
