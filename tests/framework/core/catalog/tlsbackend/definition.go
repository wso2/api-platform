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
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package tlsbackend

import (
	"fmt"

	"github.com/wso2/api-platform/tests/framework/core/catalog/testbench"
	"github.com/wso2/api-platform/tests/framework/core/components"
	"github.com/wso2/api-platform/tests/framework/core/util/testpki"
	service "github.com/wso2/api-platform/tests/framework/testbench/services/tlsbackend"
)

// Name is the component name a suite file references.
const Name = "tls-backend"

// backends lists the TLS backends the gateway dials as https://tls-backend:<port>.
//
// A accepts client certificates from partner A's authority, B from partner B's. The
// wrong-host backend trusts partner A's and serves a certificate for another name. A and B
// serve certificates from separate backend authorities. The optional backend requests a
// client certificate from partner A's authority without requiring one and always answers 200.
//
// The keys are throwaway test material, generated for this test process and handed to the
// container through its environment.
func backends(set *testpki.Set) ([]service.Backend, error) {
	var out []service.Backend
	for _, b := range []struct {
		name, server, clientCA string
		port                   int
	}{
		{"a", "backend-server-a", "ca-a", service.PortA},
		{"b", "backend-server-b", "ca-b", service.PortB},
		{"wronghost", "backend-server-wronghost", "ca-a", service.PortWrongHost},
		{"optional", "backend-server-a", "ca-a", service.PortOptional},
	} {
		server, err := set.Get(b.server)
		if err != nil {
			return nil, err
		}
		authority, err := set.Get(b.clientCA)
		if err != nil {
			return nil, err
		}
		out = append(out, service.Backend{
			Name: b.name, Port: b.port, Optional: b.name == "optional",
			Certificate: string(server.CertPEM), PrivateKey: string(server.KeyPEM),
			ClientCAs: string(authority.CertPEM),
		})
	}
	return out, nil
}

// TLSBackend returns the TLS backend component: the testbench image serving only the
// backends in its environment. It is stateless, so every block shares one instance.
func TLSBackend() *components.Definition {
	set, err := testpki.Default()
	if err != nil {
		panic(fmt.Sprintf("catalog: generating the test PKI: %v", err))
	}
	list, err := backends(set)
	if err != nil {
		panic(fmt.Sprintf("catalog: building the TLS backends: %v", err))
	}
	env, err := service.Encode(list)
	if err != nil {
		panic(fmt.Sprintf("catalog: encoding the TLS backends: %v", err))
	}
	return &components.Definition{
		Name:  Name,
		Image: testbench.Testbench().Image,
		Alias: testpki.TLSBackendHost,
		Env:   map[string]string{service.EnvBackends: env},
		Endpoints: []components.Endpoint{
			{Name: "a", Port: service.PortA, Scheme: "https", AwaitListening: true},
			{Name: "b", Port: service.PortB, Scheme: "https", AwaitListening: true},
			{Name: "wronghost", Port: service.PortWrongHost, Scheme: "https", AwaitListening: true},
			{Name: "optional", Port: service.PortOptional, Scheme: "https", AwaitListening: true},
		},
		Limits: components.ResourceLimits{CPUs: 0.5, MemoryMB: 128},
		Shared: true,
	}
}
