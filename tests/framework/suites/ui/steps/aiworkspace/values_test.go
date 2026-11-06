/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.  You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package aiworkspace

import (
	"context"
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/tests/framework/core/catalog/shared"
	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
)

func TestPlatformAPIReadinessClientVerifiesCertificates(t *testing.T) {
	crypto := shared.ControlPlaneCrypto()
	certificate, err := tls.X509KeyPair(crypto["certs/cert.pem"], crypto["certs/key.pem"])
	require.NoError(t, err)

	trusted := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	trusted.TLS = &tls.Config{Certificates: []tls.Certificate{certificate}}
	trusted.StartTLS()
	defer trusted.Close()

	client, err := newPlatformAPIReadinessClient()
	require.NoError(t, err)
	response, err := client.Do(context.Background(), httpx.Request{
		Method: http.MethodGet,
		URL:    trusted.URL,
	}, 0, 0)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, response.StatusCode)

	untrusted := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer untrusted.Close()
	_, err = client.Do(context.Background(), httpx.Request{
		Method: http.MethodGet,
		URL:    untrusted.URL,
	}, 0, 0)
	require.Error(t, err)
}
