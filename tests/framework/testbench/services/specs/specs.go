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

// Package specs serves OpenAPI documents for the control plane to import by URL, over HTTP
// and HTTPS:
//
//	http://testbench:3024/specs/<name>
//	https://testbench:3025/specs/<name>
//	https://testbench:3025/specs/large?bytes=<n>
//
// <name> is one of the embedded fixtures, which the suites also upload as files, so a spec
// imported by URL and one imported by file are the same bytes. large generates a valid
// document of exactly n bytes, for the control plane's size limit.
package specs

import (
	"crypto/tls"
	"embed"
	"fmt"
	"net/http"
	"path"
	"strconv"
	"strings"

	"github.com/wso2/api-platform/tests/framework/testbench"
)

// Port is the container port the service serves HTTP on.
const Port = 3024

// TLSPort is the container port the service serves HTTPS on.
const TLSPort = 3025

// EnvTLSCert and EnvTLSKey name the variables holding the PEM certificate and key the service
// serves HTTPS with on TLSPort. When both are unset only the plain HTTP port is served.
const (
	EnvTLSCert = "TESTBENCH_SPECS_TLS_CERT"
	EnvTLSKey  = "TESTBENCH_SPECS_TLS_KEY"
)

// maxGeneratedBytes bounds a generated document, far above any limit a suite configures.
const maxGeneratedBytes = 16 << 20

//go:embed fixtures
var fixtures embed.FS

// Fixture returns an embedded document by file name.
func Fixture(name string) ([]byte, error) {
	if name != path.Base(name) || strings.HasPrefix(name, ".") {
		return nil, fmt.Errorf("specs: invalid fixture name %q", name)
	}
	data, err := fixtures.ReadFile("fixtures/" + name)
	if err != nil {
		return nil, fmt.Errorf("specs: unknown fixture %q", name)
	}
	return data, nil
}

// Generate returns a valid OpenAPI 3.0 document of exactly n bytes, padded in its description.
func Generate(n int) ([]byte, error) {
	const head = "openapi: 3.0.3\ninfo:\n  title: Generated\n  version: 1.0.0\n  description: "
	const tail = "\npaths:\n  /items:\n    get:\n      operationId: listItems\n      responses:\n        '200':\n          description: OK\n"
	pad := n - len(head) - len(tail)
	if pad < 1 || n > maxGeneratedBytes {
		return nil, fmt.Errorf("specs: cannot generate a %d-byte document", n)
	}
	return []byte(head + strings.Repeat("x", pad) + tail), nil
}

// Service serves the fixtures over HTTP.
type Service struct{}

// New returns the service.
func New() *Service { return &Service{} }

// Name returns the service registration name.
func (s *Service) Name() string { return "specs" }

// Port returns the service's listening port.
func (s *Service) Port() int { return Port }

// Stateful reports that every response derives from the request alone.
func (s *Service) Stateful() bool { return false }

// Handler returns the service HTTP handler.
func (s *Service) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /specs/large", func(w http.ResponseWriter, r *http.Request) {
		n, err := strconv.Atoi(r.URL.Query().Get("bytes"))
		if err != nil {
			http.Error(w, "bytes must be an integer", http.StatusBadRequest)
			return
		}
		data, err := Generate(n)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		write(w, r, "application/yaml", data)
	})
	mux.HandleFunc("GET /specs/{name}", func(w http.ResponseWriter, r *http.Request) {
		data, err := Fixture(r.PathValue("name"))
		if err != nil {
			http.NotFound(w, r)
			return
		}
		write(w, r, contentType(r.PathValue("name")), data)
	})
	return testbench.NormalizeMethod(mux)
}

func contentType(name string) string {
	switch path.Ext(name) {
	case ".yaml", ".yml":
		return "application/yaml"
	case ".json":
		return "application/json"
	default:
		return "text/plain"
	}
}

func write(w http.ResponseWriter, r *http.Request, contentType string, data []byte) {
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	if _, err := w.Write(data); err != nil {
		testbench.ServiceLogger(r.Context()).Warn("specs response failed", "error", err)
	}
}

// OverTLS returns the service served over HTTPS on TLSPort with the supplied PEM certificate
// and key.
func (s *Service) OverTLS(certPEM, keyPEM []byte) (testbench.Service, error) {
	certificate, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, fmt.Errorf("specs: loading the TLS key pair: %w", err)
	}
	return &secure{
		Service:   s,
		tlsConfig: &tls.Config{Certificates: []tls.Certificate{certificate}, MinVersion: tls.VersionTLS12},
	}, nil
}

// secure is the service registered on TLSPort; everything but its name, port and TLS
// configuration is the plain service's.
type secure struct {
	*Service
	tlsConfig *tls.Config
}

func (s *secure) Name() string           { return "specs-tls" }
func (s *secure) Port() int              { return TLSPort }
func (s *secure) TLSConfig() *tls.Config { return s.tlsConfig }
