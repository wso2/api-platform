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

// Package soapbackend provides a SOAP calculator upstream that serves SOAP 1.1 and SOAP 1.2
// envelopes and a WSDL describing them.
package soapbackend

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/wso2/api-platform/tests/framework/testbench"
)

// Port is the container port used by the testbench.
const Port = 3027

const (
	soap11Namespace = "http://schemas.xmlsoap.org/soap/envelope/"
	soap12Namespace = "http://www.w3.org/2003/05/soap-envelope"
	// Namespace is the target namespace of the calculator's messages.
	Namespace = "http://example.com/soap/calculator/v1"
	// PathHeader echoes the request path the backend received, so a scenario can assert how
	// the gateway rewrote it.
	PathHeader = "X-Backend-Path"
)

// Service implements the stateless SOAP backend testbench service.
type Service struct{}

// New creates a soapbackend service.
func New() *Service { return &Service{} }

// Name returns the service registration name.
func (s *Service) Name() string { return "soap-backend" }

// Port returns the service's listening port.
func (s *Service) Port() int { return Port }

// Stateful reports whether the service keeps request-specific state.
func (s *Service) Stateful() bool { return false }

// Handler returns the soapbackend service's HTTP handler.
//
// Under any base path:
//   - POST <base>/soap11 and <base>/soap12 run the Add, Subtract or Divide operation named by
//     the body's first element. Divide by zero, and any request that is not a calculator
//     envelope of the matching SOAP version, answer 500 with a SOAP Fault.
//   - GET <base>?wsdl returns a WSDL 1.1 document whose soap:address and soap12:address point
//     at <base>/soap11 and <base>/soap12 on this backend. charset=iso-8859-1 serves it encoded
//     in ISO-8859-1 with a non-ASCII character, and doctype=true prefixes an internal DTD.
//   - Any other GET returns an HTML page.
//
// Every response carries the received path in PathHeader.
func (s *Service) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", s.health)
	mux.HandleFunc("/", s.serve)
	return testbench.NormalizeMethod(mux)
}

func (s *Service) health(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]string{
		"status":  "ok",
		"service": "soap-backend",
	}); err != nil {
		log.Printf("soapbackend: failed to write health response: %v", err)
	}
}

func (s *Service) serve(w http.ResponseWriter, r *http.Request) {
	w.Header().Set(PathHeader, r.URL.Path)
	switch r.Method {
	case http.MethodPost:
		s.invoke(w, r)
	case http.MethodGet:
		if _, ok := r.URL.Query()["wsdl"]; ok {
			s.wsdl(w, r)
			return
		}
		writeBody(w, http.StatusOK, "text/html; charset=utf-8",
			[]byte("<!DOCTYPE html><html lang=en><body>Calculator service</body></html>"))
	default:
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	}
}

// soapVersion describes one SOAP version's envelope namespace, media type and fault shape.
type soapVersion struct {
	namespace   string
	contentType string
	fault       func(reason string) string
}

var soap11 = soapVersion{
	namespace:   soap11Namespace,
	contentType: "text/xml; charset=utf-8",
	fault: func(reason string) string {
		return "<soap:Fault><faultcode>soap:Client</faultcode><faultstring>" + reason +
			"</faultstring></soap:Fault>"
	},
}

var soap12 = soapVersion{
	namespace:   soap12Namespace,
	contentType: "application/soap+xml; charset=utf-8",
	fault: func(reason string) string {
		return "<soap:Fault><soap:Code><soap:Value>soap:Sender</soap:Value></soap:Code>" +
			"<soap:Reason><soap:Text xml:lang=\"en\">" + reason + "</soap:Text></soap:Reason></soap:Fault>"
	},
}

func (s *Service) invoke(w http.ResponseWriter, r *http.Request) {
	var version soapVersion
	switch {
	case strings.HasSuffix(r.URL.Path, "/soap11"):
		version = soap11
	case strings.HasSuffix(r.URL.Path, "/soap12"):
		version = soap12
	default:
		http.NotFound(w, r)
		return
	}
	body, err := io.ReadAll(r.Body)
	defer func() { _ = r.Body.Close() }()
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			http.Error(w, "Request Entity Too Large", http.StatusRequestEntityTooLarge)
			return
		}
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	result, err := calculate(body, version.namespace)
	if err != nil {
		writeEnvelope(w, version, http.StatusInternalServerError, version.fault(err.Error()))
		return
	}
	writeEnvelope(w, version, http.StatusOK, result)
}

// operation is the calculator request inside the SOAP Body.
type operation struct {
	XMLName xml.Name
	A       string `xml:"a"`
	B       string `xml:"b"`
}

// envelope is the minimal SOAP envelope the calculator reads.
type envelope struct {
	XMLName xml.Name
	Body    struct {
		Operation operation `xml:",any"`
	} `xml:"Body"`
}

// calculate runs the operation in a SOAP envelope of the given namespace and returns the
// response element.
func calculate(body []byte, namespace string) (string, error) {
	var env envelope
	if err := xml.Unmarshal(body, &env); err != nil {
		return "", errors.New("malformed SOAP envelope")
	}
	if env.XMLName.Local != "Envelope" || env.XMLName.Space != namespace {
		return "", errors.New("unexpected SOAP envelope version")
	}
	op := env.Body.Operation
	a, errA := strconv.Atoi(strings.TrimSpace(op.A))
	b, errB := strconv.Atoi(strings.TrimSpace(op.B))
	if errA != nil || errB != nil {
		return "", errors.New("operands must be integers")
	}
	var result int
	switch op.XMLName.Local {
	case "Add":
		result = a + b
	case "Subtract":
		result = a - b
	case "Divide":
		if b == 0 {
			return "", errors.New("division by zero")
		}
		result = a / b
	default:
		return "", errors.New("unknown operation")
	}
	return fmt.Sprintf(`<cal:%sResponse xmlns:cal="%s"><cal:result>%d</cal:result></cal:%sResponse>`,
		op.XMLName.Local, Namespace, result, op.XMLName.Local), nil
}

func writeEnvelope(w http.ResponseWriter, version soapVersion, status int, content string) {
	writeBody(w, status, version.contentType, []byte(`<?xml version="1.0" encoding="UTF-8"?>`+
		`<soap:Envelope xmlns:soap="`+version.namespace+`"><soap:Body>`+content+`</soap:Body></soap:Envelope>`))
}

// wsdlTemplate is the calculator's WSDL. The first verb is the XML declaration's encoding,
// the second an optional DOCTYPE, the third the service documentation, and the last two the
// SOAP 1.1 and SOAP 1.2 endpoint addresses.
const wsdlTemplate = `<?xml version="1.0" encoding="%s"?>
%s<wsdl:definitions xmlns:wsdl="http://schemas.xmlsoap.org/wsdl/"
    xmlns:soap="http://schemas.xmlsoap.org/wsdl/soap/"
    xmlns:soap12="http://schemas.xmlsoap.org/wsdl/soap12/"
    xmlns:tns="` + Namespace + `" targetNamespace="` + Namespace + `" name="Calculator">
  <wsdl:portType name="CalculatorPortType">
    <wsdl:operation name="Add"/>
  </wsdl:portType>
  <wsdl:binding name="CalculatorSoap11" type="tns:CalculatorPortType">
    <soap:binding transport="http://schemas.xmlsoap.org/soap/http"/>
  </wsdl:binding>
  <wsdl:binding name="CalculatorSoap12" type="tns:CalculatorPortType">
    <soap12:binding transport="http://schemas.xmlsoap.org/soap/http"/>
  </wsdl:binding>
  <wsdl:service name="CalculatorService">
    <wsdl:documentation>%s</wsdl:documentation>
    <wsdl:port name="CalculatorSoap11Port" binding="tns:CalculatorSoap11">
      <soap:address location="%s"/>
    </wsdl:port>
    <wsdl:port name="CalculatorSoap12Port" binding="tns:CalculatorSoap12">
      <soap12:address location="%s"/>
    </wsdl:port>
  </wsdl:service>
</wsdl:definitions>
`

func (s *Service) wsdl(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	base := "http://" + r.Host + strings.TrimSuffix(r.URL.Path, "/")
	doctype := ""
	if query.Get("doctype") == "true" {
		doctype = `<!DOCTYPE wsdl:definitions [<!ENTITY calc "Calculator">]>` + "\n"
	}
	switch strings.ToLower(query.Get("charset")) {
	case "":
		doc := fmt.Sprintf(wsdlTemplate, "UTF-8", doctype, "Calculadora numérica", base+"/soap11", base+"/soap12")
		writeBody(w, http.StatusOK, "text/xml; charset=utf-8", []byte(doc))
	case "iso-8859-1":
		doc := fmt.Sprintf(wsdlTemplate, "ISO-8859-1", doctype, "Calculadora num\xe9rica", base+"/soap11", base+"/soap12")
		writeBody(w, http.StatusOK, "text/xml; charset=iso-8859-1", []byte(doc))
	default:
		http.Error(w, "unsupported charset", http.StatusBadRequest)
	}
}

func writeBody(w http.ResponseWriter, status int, contentType string, body []byte) {
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(status)
	if _, err := io.Copy(w, bytes.NewReader(body)); err != nil {
		log.Printf("soapbackend: failed to write response: %v", err)
	}
}
