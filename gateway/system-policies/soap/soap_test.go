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

package soap

import (
	"context"
	"encoding/binary"
	"strings"
	"testing"
	"unicode/utf16"

	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

// calculatorWSDL is shaped like the local test backend's: a SOAP 1.1 and a SOAP 1.2 port
// below the upstream base path, a port elsewhere on the backend, an absolute schema import
// from the backend, a relative one, and a third-party import. The backend names itself
// localhost, not the name the gateway reaches it by.
const calculatorWSDL = `<?xml version="1.0" encoding="UTF-8"?>
<wsdl:definitions xmlns:wsdl="http://schemas.xmlsoap.org/wsdl/"
    xmlns:soap="http://schemas.xmlsoap.org/wsdl/soap/"
    xmlns:soap12="http://schemas.xmlsoap.org/wsdl/soap12/"
    xmlns:xsd="http://www.w3.org/2001/XMLSchema"
    xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
    targetNamespace="http://example.com/calculator">
  <wsdl:types>
    <xsd:schema xsi:schemaLocation="http://localhost:3002/calculator/v1?xsd=keep">
      <xsd:import namespace="urn:types" schemaLocation="http://localhost:3002/calculator/v1?xsd=1"/>
      <xsd:import namespace="urn:local" schemaLocation="types.xsd"/>
      <xsd:import namespace="http://www.w3.org/XML/1998/namespace" schemaLocation="http://www.w3.org/2001/xml.xsd"/>
    </xsd:schema>
  </wsdl:types>
  <wsdl:service name="CalculatorService">
    <wsdl:port name="Soap11" binding="tns:Soap11">
      <soap:address location="http://localhost:3002/calculator/v1/soap11"/>
    </wsdl:port>
    <wsdl:port name="Soap12" binding="tns:Soap12">
      <soap12:address location='http://localhost:3002/calculator/v1/soap12'/>
    </wsdl:port>
    <wsdl:port name="Elsewhere" binding="tns:Soap11">
      <soap:address location="http://localhost:3002/other/soap11"/>
    </wsdl:port>
  </wsdl:service>
</wsdl:definitions>`

// wsdlWithAddress is a minimal WSDL 1.1 document with one SOAP 1.1 endpoint address.
func wsdlWithAddress(location string) string {
	return `<definitions xmlns="http://schemas.xmlsoap.org/wsdl/" xmlns:soap="http://schemas.xmlsoap.org/wsdl/soap/">
<service name="S"><port name="P" binding="tns:B"><soap:address location="` + location + `"/></port></service></definitions>`
}

func rewriteParams(upstreamURL, vhost string) map[string]any {
	return map[string]any{ParamWSDLRewrite: map[string]any{
		ParamGatewayPath: "/calc/v1.0",
		ParamUpstreamURL: upstreamURL,
		ParamVhost:       vhost,
	}}
}

func respond(t *testing.T, params map[string]any, status int, body, scheme, authority string) policy.ResponseAction {
	t.Helper()
	instance, err := GetPolicy(policy.PolicyMetadata{}, params)
	if err != nil {
		t.Fatalf("GetPolicy: %v", err)
	}
	p := instance.(*SOAPSystemPolicy)
	var responseBody *policy.Body
	if body != "" {
		responseBody = &policy.Body{Content: []byte(body), Present: true}
	}
	return p.OnResponseBody(context.Background(), &policy.ResponseContext{
		SharedContext:   &policy.SharedContext{APIId: "calc"},
		RequestPath:     "/calc/v1.0",
		RequestMethod:   "GET",
		ResponseHeaders: policy.NewHeaders(nil),
		ResponseBody:    responseBody,
		ResponseStatus:  status,
		Downstream: &policy.DownstreamContext{Request: &policy.DownstreamRequest{
			Headers:   policy.NewHeaders(nil),
			Path:      "/calc/v1.0",
			Method:    "GET",
			Scheme:    scheme,
			Authority: authority,
		}},
	}, params)
}

func rewrittenBody(t *testing.T, action policy.ResponseAction) string {
	t.Helper()
	mods, ok := action.(policy.DownstreamResponseModifications)
	if !ok {
		t.Fatalf("expected DownstreamResponseModifications, got %T", action)
	}
	if mods.Body == nil {
		t.Fatal("expected the body to be rewritten, got it forwarded unchanged")
	}
	return string(mods.Body)
}

func requireUnchanged(t *testing.T, action policy.ResponseAction) {
	t.Helper()
	mods, ok := action.(policy.DownstreamResponseModifications)
	if !ok {
		t.Fatalf("expected DownstreamResponseModifications, got %T", action)
	}
	if mods.Body != nil || len(mods.HeadersToSet) > 0 || len(mods.HeadersToRemove) > 0 {
		t.Fatalf("expected the response forwarded unchanged, got %+v", mods)
	}
}

func requireRefused(t *testing.T, action policy.ResponseAction) {
	t.Helper()
	imm, ok := action.(policy.ImmediateResponse)
	if !ok {
		t.Fatalf("expected an ImmediateResponse refusal, got %T", action)
	}
	if imm.StatusCode != 502 || imm.Fault == nil {
		t.Fatalf("expected a described 502, got %d (fault %v)", imm.StatusCode, imm.Fault)
	}
}

func TestRewritesEndpointAddressesOnly(t *testing.T) {
	out := rewrittenBody(t, respond(t, rewriteParams("http://host.docker.internal:3002/calculator/v1", "soap.example.com"),
		200, calculatorWSDL, "https", "gw.internal:8443"))

	for _, want := range []string{
		`<soap:address location="https://soap.example.com:8443/calc/v1.0/soap11"/>`,
		`<soap12:address location='https://soap.example.com:8443/calc/v1.0/soap12'/>`,
		// Not under the upstream's base path: the API's context, never the backend.
		`<soap:address location="https://soap.example.com:8443/calc/v1.0"/>`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %s in:\n%s", want, out)
		}
	}
	for _, kept := range []string{
		`schemaLocation="http://localhost:3002/calculator/v1?xsd=1"`, // imports are not rewritten
		`schemaLocation="types.xsd"`,
		`schemaLocation="http://www.w3.org/2001/xml.xsd"`,
		`xsi:schemaLocation="http://localhost:3002/calculator/v1?xsd=keep"`,
		`targetNamespace="http://example.com/calculator"`,
	} {
		if !strings.Contains(out, kept) {
			t.Errorf("expected %s to be left as is in:\n%s", kept, out)
		}
	}
	if strings.Contains(out, `address location="http://localhost:3002`) {
		t.Errorf("a backend endpoint address survived:\n%s", out)
	}
}

// Only the address values change; every other byte the backend wrote is kept.
func TestRewriteKeepsTheRestOfTheDocument(t *testing.T) {
	out := rewrittenBody(t, respond(t, rewriteParams("http://backend:3002/calculator/v1", ""),
		200, calculatorWSDL, "http", "localhost:8080"))
	restored := strings.NewReplacer(
		`"http://localhost:8080/calc/v1.0/soap11"`, `"http://localhost:3002/calculator/v1/soap11"`,
		`'http://localhost:8080/calc/v1.0/soap12'`, `'http://localhost:3002/calculator/v1/soap12'`,
		`"http://localhost:8080/calc/v1.0"`, `"http://localhost:3002/other/soap11"`,
	).Replace(out)
	if restored != calculatorWSDL {
		t.Fatalf("document changed beyond the rewritten values:\n%s", out)
	}
}

func TestRewriteAdvertisesVhostAndDropsStandardPort(t *testing.T) {
	out := rewrittenBody(t, respond(t, rewriteParams("http://backend:3002/calculator/v1", "soap.example.com"),
		200, calculatorWSDL, "https", "10.0.0.7:443"))
	if !strings.Contains(out, `location="https://soap.example.com/calc/v1.0/soap11"`) {
		t.Fatalf("expected the vhost without the standard port:\n%s", out)
	}
}

// A wildcard vhost names no dialable host, so the request's own authority is used.
func TestRewriteWithWildcardVhostUsesTheRequestAuthority(t *testing.T) {
	out := rewrittenBody(t, respond(t, rewriteParams("http://backend:3002/calculator/v1", "*.example.com"),
		200, calculatorWSDL, "http", "calc.example.com:8080"))
	if !strings.Contains(out, `location="http://calc.example.com:8080/calc/v1.0/soap11"`) {
		t.Fatalf("expected the request authority:\n%s", out)
	}
}

// Every endpoint address is replaced, whatever it names: the part below the upstream's base
// path is kept, and anything else becomes the API's context, as WSO2 API Manager does.
func TestEveryEndpointAddressPointsAtTheGateway(t *testing.T) {
	cases := []struct {
		name, upstream, location, want string
	}{
		{"below the base path", "http://backend:3002/calculator/v1", "http://localhost:3002/calculator/v1/soap11", "http://gw:8080/calc/v1.0/soap11"},
		{"the base path itself", "http://backend:3002/calculator/v1", "http://localhost:3002/calculator/v1", "http://gw:8080/calc/v1.0"},
		{"query is kept", "http://backend:3002/calculator/v1", "http://localhost:3002/calculator/v1/soap11?tenant=a", "http://gw:8080/calc/v1.0/soap11?tenant=a"},
		{"another path", "http://backend:3002/calculator/v1", "http://localhost:3002/other/soap11?x=1", "http://gw:8080/calc/v1.0"},
		{"another host", "http://backend:3002/calculator/v1", "https://internal.corp:9443/svc", "http://gw:8080/calc/v1.0"},
		{"relative", "http://backend:3002/calculator/v1", "soap11", "http://gw:8080/calc/v1.0"},
		{"placeholder", "http://backend:3002/calculator/v1", "REPLACE_WITH_ACTUAL_URL", "http://gw:8080/calc/v1.0"},
		{"empty", "http://backend:3002/calculator/v1", "", "http://gw:8080/calc/v1.0"},
		{"root upstream keeps the whole path", "http://backend:3002", "http://localhost:9000/svc", "http://gw:8080/calc/v1.0/svc"},
		{"escaped upstream base path", "http://backend:3002/my%20svc", "http://localhost:3002/my%20svc/soap11", "http://gw:8080/calc/v1.0/soap11"},
		{"escaped suffix stays escaped", "http://backend:3002/svc", "http://localhost:3002/svc/a%20b", "http://gw:8080/calc/v1.0/a%20b"},
		// The route consumes the slash after the context for such an upstream, so these
		// reach /calculator/v1/soap11 and /calculator/v1/ (see the controller's routing test).
		{"trailing-slash upstream, below it", "http://backend:3002/calculator/v1/", "http://localhost:3002/calculator/v1/soap11", "http://gw:8080/calc/v1.0/soap11"},
		{"trailing-slash upstream, itself", "http://backend:3002/calculator/v1/", "http://localhost:3002/calculator/v1/", "http://gw:8080/calc/v1.0/"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := rewrittenBody(t, respond(t, rewriteParams(tc.upstream, ""), 200, wsdlWithAddress(tc.location), "http", "gw:8080"))
			if want := `location="` + tc.want + `"`; !strings.Contains(out, want) {
				t.Fatalf("expected %s in:\n%s", want, out)
			}
		})
	}
}

func TestRewritesWSDL20Endpoints(t *testing.T) {
	doc := `<description xmlns="http://www.w3.org/ns/wsdl">
  <service name="Calc" interface="tns:Calc">
    <endpoint name="ep" binding="tns:b" address="http://backend:3002/calculator/v1/soap12"/>
  </service>
</description>`
	out := rewrittenBody(t, respond(t, rewriteParams("http://backend:3002/calculator/v1", ""), 200, doc, "http", "gw:8080"))
	if !strings.Contains(out, `address="http://gw:8080/calc/v1.0/soap12"`) {
		t.Fatalf("WSDL 2.0 endpoint not rewritten:\n%s", out)
	}
}

// A replacement value is escaped for the attribute it lands in.
func TestRewriteEscapesTheReplacement(t *testing.T) {
	doc := wsdlWithAddress("http://backend:3002/calculator/v1/soap11?a=1&amp;b=2")
	out := rewrittenBody(t, respond(t, rewriteParams("http://backend:3002/calculator/v1", ""), 200, doc, "http", "gw:8080"))
	if !strings.Contains(out, `location="http://gw:8080/calc/v1.0/soap11?a=1&amp;b=2"`) {
		t.Fatalf("expected the query escaped in the attribute:\n%s", out)
	}
}

// The address is found attribute by attribute, so a look-alike inside another attribute's
// quoted value is neither replaced nor mistaken for the real one.
func TestRewriteFindsTheRealAttribute(t *testing.T) {
	doc := `<definitions xmlns="http://schemas.xmlsoap.org/wsdl/" xmlns:soap="http://schemas.xmlsoap.org/wsdl/soap/">
<service name="S"><port name="P" binding="tns:B"><soap:address note='a location="x"' location="http://backend:3002/calculator/v1/soap11"/></port></service></definitions>`
	out := rewrittenBody(t, respond(t, rewriteParams("http://backend:3002/calculator/v1", ""), 200, doc, "http", "gw:8080"))
	want := `<soap:address note='a location="x"' location="http://gw:8080/calc/v1.0/soap11"/>`
	if !strings.Contains(out, want) {
		t.Fatalf("expected %s in:\n%s", want, out)
	}
}

// A WSDL declared in a single-byte, ASCII-compatible encoding is rewritten as it stands.
func TestRewritesASCIICompatibleEncodings(t *testing.T) {
	for _, charset := range []string{"windows-1252", "ISO-8859-15", "us-ascii"} {
		t.Run(charset, func(t *testing.T) {
			doc := `<?xml version="1.0" encoding="` + charset + `"?>` + wsdlWithAddress("http://backend:3002/calculator/v1/soap11")
			out := rewrittenBody(t, respond(t, rewriteParams("http://backend:3002/calculator/v1", ""), 200, doc, "http", "gw:8080"))
			if !strings.Contains(out, `location="http://gw:8080/calc/v1.0/soap11"`) {
				t.Fatalf("not rewritten:\n%s", out)
			}
		})
	}
}

// Non-ASCII text in a single-byte encoding is decoded, not rejected as invalid UTF-8, and
// the rewritten description is written back in the encoding it declares.
func TestRewritesSingleByteEncodedText(t *testing.T) {
	cases := []struct{ charset, raw string }{
		{"ISO-8859-1", "caf\xe9"},      // é
		{"windows-1252", "\x80 price"}, // €
		{"iso-8859-15", "\xa4 price"},  // € in Latin-9
	}
	for _, tc := range cases {
		t.Run(tc.charset, func(t *testing.T) {
			doc := `<?xml version="1.0" encoding="` + tc.charset + `"?>` +
				`<definitions xmlns="http://schemas.xmlsoap.org/wsdl/" xmlns:soap="http://schemas.xmlsoap.org/wsdl/soap/">` +
				`<documentation>` + tc.raw + `</documentation>` +
				`<service name="S"><port name="P" binding="tns:B"><soap:address location="http://backend:3002/calculator/v1/soap11"/></port></service></definitions>`
			out := rewrittenBody(t, respond(t, rewriteParams("http://backend:3002/calculator/v1", ""), 200, doc, "http", "gw:8080"))
			if !strings.Contains(out, `location="http://gw:8080/calc/v1.0/soap11"`) {
				t.Fatalf("not rewritten:\n%q", out)
			}
			if !strings.Contains(out, "<documentation>"+tc.raw+"</documentation>") {
				t.Fatalf("text not kept in its declared encoding:\n%q", out)
			}
		})
	}
}

func TestLeavesOtherResponsesUntouched(t *testing.T) {
	params := rewriteParams("http://backend:3002/calculator/v1", "")
	cases := map[string]struct {
		status int
		body   string
	}{
		"error status":                         {500, calculatorWSDL},
		"empty body":                           {200, ""},
		"json body":                            {200, `{"wsdl":"http://backend:3002/calculator/v1"}`},
		"not a description":                    {200, `<html><a href="http://backend:3002/calculator/v1">x</a></html>`},
		"html with a doctype":                  {200, `<!DOCTYPE html><html><body><a href="http://backend:3002/calculator/v1">x</a></body></html>`},
		"html with a doctype, not well-formed": {200, `<!DOCTYPE html><html lang=en><body>services</body></html>`},
		"html not well-formed":                 {200, `<html lang=en><body>services</body></html>`},
		"schema":                               {200, `<xsd:schema xmlns:xsd="http://www.w3.org/2001/XMLSchema"><xsd:import schemaLocation="http://backend:3002/calculator/v1?xsd=2"/></xsd:schema>`},
		"no endpoint address":                  {200, `<definitions xmlns="http://schemas.xmlsoap.org/wsdl/"/>`},
		"unparseable non-xml":                  {200, `<<<`},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			requireUnchanged(t, respond(t, params, tc.status, tc.body, "http", "gw:8080"))
		})
	}
}

// A service description that cannot be rewritten safely is refused, never forwarded with
// the backend's URLs in it.
func TestRefusesWhatItCannotRewriteSafely(t *testing.T) {
	params := rewriteParams("http://backend:3002/calculator/v1", "")
	t.Run("doctype", func(t *testing.T) {
		doc := `<!DOCTYPE d [<!ENTITY x "y">]>` + calculatorWSDL[strings.Index(calculatorWSDL, "<wsdl:definitions"):]
		requireRefused(t, respond(t, params, 200, doc, "http", "gw:8080"))
	})
	// An entity used in the root's attributes keeps encoding/xml from reading the root at
	// all, while an entity-aware client reads the description, backend address included.
	t.Run("doctype entity in the root", func(t *testing.T) {
		doc := `<!DOCTYPE definitions [<!ENTITY ns "urn:example">]>` + strings.Replace(
			wsdlWithAddress("http://backend:3002/calculator/v1/soap11"), "<definitions ", `<definitions targetNamespace="&ns;" `, 1)
		requireRefused(t, respond(t, params, 200, doc, "http", "gw:8080"))
	})
	t.Run("byte undefined in the declared code page", func(t *testing.T) {
		doc := `<?xml version="1.0" encoding="windows-1252"?>` + strings.Replace(
			wsdlWithAddress("http://backend:3002/calculator/v1/soap11"), "<service ", "<documentation>\x81</documentation><service ", 1)
		requireRefused(t, respond(t, params, 200, doc, "http", "gw:8080"))
	})
	t.Run("too large", func(t *testing.T) {
		small := map[string]any{ParamWSDLRewrite: map[string]any{
			ParamGatewayPath: "/calc/v1.0", ParamUpstreamURL: "http://backend:3002/calculator/v1", ParamMaxResponseBytes: float64(1024),
		}}
		requireRefused(t, respond(t, small, 200, calculatorWSDL+strings.Repeat(" ", 2048), "http", "gw:8080"))
		// The ceiling is for service descriptions; a larger document of another kind passes.
		requireUnchanged(t, respond(t, small, 200, `<data>`+strings.Repeat("x", 2048)+`</data>`, "http", "gw:8080"))
	})
	t.Run("too deep", func(t *testing.T) {
		doc := `<definitions xmlns="http://schemas.xmlsoap.org/wsdl/">` + strings.Repeat("<a>", maxDepth+1)
		requireRefused(t, respond(t, params, 200, doc, "http", "gw:8080"))
	})
	t.Run("unsupported declared encoding", func(t *testing.T) {
		doc := `<?xml version="1.0" encoding="Shift_JIS"?>` + wsdlWithAddress("http://backend:3002/calculator/v1/soap11")
		requireRefused(t, respond(t, params, 200, doc, "http", "gw:8080"))
	})
	t.Run("utf-16", func(t *testing.T) {
		for name, bom := range map[string][]byte{"little endian": {0xff, 0xfe}, "big endian": {0xfe, 0xff}} {
			t.Run(name, func(t *testing.T) {
				requireRefused(t, respond(t, params, 200, encodeUTF16(bom, wsdlWithAddress("http://backend:3002/calculator/v1/soap11")), "http", "gw:8080"))
			})
		}
	})
	t.Run("malformed after the root", func(t *testing.T) {
		requireRefused(t, respond(t, params, 200, calculatorWSDL[:len(calculatorWSDL)-30]+"<<", "http", "gw:8080"))
	})
	t.Run("no usable authority", func(t *testing.T) {
		requireRefused(t, respond(t, params, 200, calculatorWSDL, "http", ""))
	})
	t.Run("unusable scheme", func(t *testing.T) {
		requireRefused(t, respond(t, params, 200, calculatorWSDL, "", "gw:8080"))
	})
	t.Run("invalid configuration", func(t *testing.T) {
		bad := map[string]any{ParamWSDLRewrite: map[string]any{ParamGatewayPath: "calc", ParamUpstreamURL: "backend"}}
		requireRefused(t, respond(t, bad, 200, calculatorWSDL, "http", "gw:8080"))
	})
}

// A rewritten description drops the backend's validators and is not cached, since its URLs
// depend on how the request reached the gateway.
func TestRewrittenResponseHeaders(t *testing.T) {
	mods := respond(t, rewriteParams("http://backend:3002/calculator/v1", ""), 200, calculatorWSDL, "http", "gw:8080").(policy.DownstreamResponseModifications)
	if mods.HeadersToSet["cache-control"] != "no-store" {
		t.Errorf("cache-control = %q", mods.HeadersToSet["cache-control"])
	}
	removed := strings.Join(mods.HeadersToRemove, ",")
	for _, h := range []string{"etag", "last-modified"} {
		if !strings.Contains(removed, h) {
			t.Errorf("%s not removed", h)
		}
	}
}

// Only an instance that rewrites buffers the response body.
func TestModeBuffersOnlyWhenRewriting(t *testing.T) {
	plain, _ := GetPolicy(policy.PolicyMetadata{}, nil)
	if plain.Mode().ResponseBodyMode != policy.BodyModeSkip {
		t.Error("an instance with nothing to do must not buffer the response")
	}
	rewriting, _ := GetPolicy(policy.PolicyMetadata{}, rewriteParams("http://backend:3002/calculator/v1", ""))
	if rewriting.Mode().ResponseBodyMode != policy.BodyModeBuffer {
		t.Error("a rewriting instance must buffer the response")
	}
	if m := rewriting.Mode(); m.RequestHeaderMode != policy.HeaderModeSkip || m.RequestBodyMode != policy.BodyModeSkip {
		t.Error("the rewrite needs no request phase")
	}
}

// The controller spells these names on its side (constants.SOAP_POLICY_PARAM_* and
// SOAP_SYSTEM_POLICY_NAME), and neither module can import the other, so both sides assert
// the literals.
func TestParameterNamesMatchTheController(t *testing.T) {
	for got, want := range map[string]string{
		ParamWSDLRewrite:      "wsdlRewrite",
		ParamGatewayPath:      "gatewayPath",
		ParamUpstreamURL:      "upstreamUrl",
		ParamVhost:            "vhost",
		ParamMaxResponseBytes: "maxResponseBytes",
	} {
		if got != want {
			t.Errorf("parameter name %q, controller writes %q", got, want)
		}
	}
}

// encodeUTF16 encodes s as UTF-16 in the byte order its byte order mark names, mark included.
func encodeUTF16(bom []byte, s string) string {
	out := append([]byte{}, bom...)
	var order binary.AppendByteOrder = binary.LittleEndian
	if bom[0] == 0xfe {
		order = binary.BigEndian
	}
	for _, u := range utf16.Encode([]rune(s)) {
		out = order.AppendUint16(out, u)
	}
	return string(out)
}
