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

package soapbackend

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func soapRequest(namespace, operation, a, b string) string {
	return `<soap:Envelope xmlns:soap="` + namespace + `" xmlns:cal="` + Namespace + `"><soap:Body>` +
		`<cal:` + operation + `><cal:a>` + a + `</cal:a><cal:b>` + b + `</cal:b></cal:` + operation + `>` +
		`</soap:Body></soap:Envelope>`
}

func serve(method, target, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	recorder := httptest.NewRecorder()
	New().Handler().ServeHTTP(recorder, req)
	return recorder
}

func TestOperations(t *testing.T) {
	for _, tc := range []struct {
		name, path, namespace, operation, a, b, contentType, want string
	}{
		{"soap 1.1 add", "/calculator/v1/soap11", soap11Namespace, "Add", "7", "5", "text/xml; charset=utf-8", "<cal:result>12</cal:result>"},
		{"soap 1.2 add", "/calculator/v1/soap12", soap12Namespace, "Add", "7", "5", "application/soap+xml; charset=utf-8", "<cal:result>12</cal:result>"},
		{"subtract", "/soap11", soap11Namespace, "Subtract", "7", "5", "text/xml; charset=utf-8", "<cal:SubtractResponse"},
		{"negative result", "/soap11", soap11Namespace, "Subtract", "5", "7", "text/xml; charset=utf-8", "<cal:result>-2</cal:result>"},
		{"divide", "/soap12", soap12Namespace, "Divide", "9", "3", "application/soap+xml; charset=utf-8", "<cal:result>3</cal:result>"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			recorder := serve(http.MethodPost, tc.path, soapRequest(tc.namespace, tc.operation, tc.a, tc.b))
			require.Equal(t, http.StatusOK, recorder.Code)
			require.Equal(t, tc.contentType, recorder.Header().Get("Content-Type"))
			require.Equal(t, tc.path, recorder.Header().Get(PathHeader))
			require.Contains(t, recorder.Body.String(), tc.want)
			require.Contains(t, recorder.Body.String(), `xmlns:soap="`+tc.namespace+`"`)
		})
	}
}

func TestFaults(t *testing.T) {
	for _, tc := range []struct {
		name, path, body, want string
	}{
		{"soap 1.1 division by zero", "/soap11", soapRequest(soap11Namespace, "Divide", "1", "0"), "<faultstring>division by zero</faultstring>"},
		{"soap 1.2 division by zero", "/soap12", soapRequest(soap12Namespace, "Divide", "1", "0"), "<soap:Value>soap:Sender</soap:Value>"},
		{"wrong envelope version", "/soap11", soapRequest(soap12Namespace, "Add", "1", "2"), "unexpected SOAP envelope version"},
		{"unknown operation", "/soap11", soapRequest(soap11Namespace, "Multiply", "1", "2"), "unknown operation"},
		{"non-integer operand", "/soap11", soapRequest(soap11Namespace, "Add", "x", "2"), "operands must be integers"},
		{"missing operand", "/soap11", soapRequest(soap11Namespace, "Add", "", "2"), "operands must be integers"},
		{"malformed body", "/soap11", "<not-closed", "malformed SOAP envelope"},
		{"empty body", "/soap12", "", "malformed SOAP envelope"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			recorder := serve(http.MethodPost, tc.path, tc.body)
			require.Equal(t, http.StatusInternalServerError, recorder.Code)
			require.Contains(t, recorder.Body.String(), "<soap:Fault>")
			require.Contains(t, recorder.Body.String(), tc.want)
		})
	}
}

func TestPostOutsideAnEndpointIsNotFound(t *testing.T) {
	recorder := serve(http.MethodPost, "/calculator/v1", soapRequest(soap11Namespace, "Add", "1", "2"))
	require.Equal(t, http.StatusNotFound, recorder.Code)
}

func TestWSDLAddressesPointAtTheBackend(t *testing.T) {
	recorder := serve(http.MethodGet, "http://testbench:3027/calculator/v1/?wsdl", "")
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "text/xml; charset=utf-8", recorder.Header().Get("Content-Type"))
	body := recorder.Body.String()
	require.True(t, strings.HasPrefix(body, `<?xml version="1.0" encoding="UTF-8"?>`))
	require.Contains(t, body, `<soap:address location="http://testbench:3027/calculator/v1/soap11"/>`)
	require.Contains(t, body, `<soap12:address location="http://testbench:3027/calculator/v1/soap12"/>`)
	require.Contains(t, body, "Calculadora numérica")
	require.NotContains(t, body, "<!DOCTYPE")
}

func TestWSDLInISO88591(t *testing.T) {
	recorder := serve(http.MethodGet, "/calculator/v1?wsdl&charset=ISO-8859-1", "")
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "text/xml; charset=iso-8859-1", recorder.Header().Get("Content-Type"))
	body := recorder.Body.String()
	require.Contains(t, body, `encoding="ISO-8859-1"`)
	require.Contains(t, body, "Calculadora num\xe9rica")
}

func TestWSDLWithDoctype(t *testing.T) {
	recorder := serve(http.MethodGet, "/calculator/v1?wsdl&doctype=true", "")
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `<!DOCTYPE wsdl:definitions [<!ENTITY calc "Calculator">]>`)
}

func TestWSDLWithUnsupportedCharsetIsRejected(t *testing.T) {
	recorder := serve(http.MethodGet, "/calculator/v1?wsdl&charset=utf-16", "")
	require.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestOtherGetReturnsHTML(t *testing.T) {
	recorder := serve(http.MethodGet, "/calculator/v1/docs", "")
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "text/html; charset=utf-8", recorder.Header().Get("Content-Type"))
	require.True(t, strings.HasPrefix(recorder.Body.String(), "<!DOCTYPE html>"))
	require.Equal(t, "/calculator/v1/docs", recorder.Header().Get(PathHeader))
}

func TestUnsupportedMethod(t *testing.T) {
	recorder := serve(http.MethodPut, "/soap11", "")
	require.Equal(t, http.StatusMethodNotAllowed, recorder.Code)
	require.Equal(t, "GET, POST", recorder.Header().Get("Allow"))
}

func TestLowercaseMethodIsNormalized(t *testing.T) {
	recorder := serve("post", "/soap11", soapRequest(soap11Namespace, "Add", "1", "2"))
	require.Equal(t, http.StatusOK, recorder.Code)
}

func TestHealth(t *testing.T) {
	recorder := serve(http.MethodGet, "/health", "")
	require.Equal(t, http.StatusOK, recorder.Code)
	require.JSONEq(t, `{"status":"ok","service":"soap-backend"}`, recorder.Body.String())
}

func TestServiceMetadata(t *testing.T) {
	svc := New()
	require.Equal(t, "soap-backend", svc.Name())
	require.Equal(t, Port, svc.Port())
	require.False(t, svc.Stateful())
}

func TestConcurrentRequests(t *testing.T) {
	handler := New().Handler()
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodPost, "/soap11", strings.NewReader(soapRequest(soap11Namespace, "Add", "2", "3")))
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, req)
			assert.Equal(t, http.StatusOK, recorder.Code)
			assert.Contains(t, recorder.Body.String(), "<cal:result>5</cal:result>")
		}()
	}
	wg.Wait()
}
