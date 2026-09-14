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

// Package tlsauth provides the peer-identity authorization check shared by
// every xDS gRPC server in gateway-controller (pkg/xds, pkg/policyxds).
// Mutual TLS alone proves a connecting client's certificate chains to a
// trusted CA; it does not prove that client is entitled to this particular
// snapshot. go-control-plane-xds-security.md directive 2 requires an
// explicit accept/reject decision against a known-identity allowlist on
// every stream, in addition to the TLS handshake itself.
package tlsauth

import (
	"context"
	"crypto/x509"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

// PeerIdentity returns the identity a verified client certificate presents:
// the first SAN URI (e.g. a SPIFFE ID) if present, otherwise the
// certificate's Subject CommonName.
func PeerIdentity(cert *x509.Certificate) string {
	if len(cert.URIs) > 0 {
		return cert.URIs[0].String()
	}
	return cert.Subject.CommonName
}

// AllowedSet converts a config allowlist slice into the map VerifyStreamPeer
// expects.
func AllowedSet(identities []string) map[string]bool {
	set := make(map[string]bool, len(identities))
	for _, id := range identities {
		set[id] = true
	}
	return set
}

// VerifyStreamPeer checks that a streaming RPC's authenticated context
// carries a client certificate whose identity (see PeerIdentity) is in
// allowed. Returns a gRPC status error suitable for returning directly from
// an xDS server.Callbacks.OnStreamOpen implementation; any client that
// clears the mTLS handshake but isn't in allowed is rejected here, not
// merely logged.
func VerifyStreamPeer(ctx context.Context, allowed map[string]bool) error {
	p, ok := peer.FromContext(ctx)
	if !ok {
		return status.Error(codes.Unauthenticated, "no peer information")
	}
	tlsInfo, isTLS := p.AuthInfo.(credentials.TLSInfo)
	if !isTLS || len(tlsInfo.State.PeerCertificates) == 0 {
		return status.Error(codes.Unauthenticated, "no client certificate presented")
	}
	identity := PeerIdentity(tlsInfo.State.PeerCertificates[0])
	if !allowed[identity] {
		return status.Error(codes.PermissionDenied, "peer identity not authorized for this xDS snapshot")
	}
	return nil
}
