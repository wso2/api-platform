/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the
 * License at http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

// Package tlsutil provides the BFF listener's TLS certificate, loaded from a
// user-mounted cert/key pair. There is no self-signed fallback — generate a
// pair with the quickstart setup script (or your own tooling) and mount it.
package tlsutil

import (
	"crypto/tls"
)

// CertFromFiles loads a TLS certificate from PEM cert/key files on disk.
func CertFromFiles(certFile, keyFile string) (tls.Certificate, error) {
	return tls.LoadX509KeyPair(certFile, keyFile)
}
