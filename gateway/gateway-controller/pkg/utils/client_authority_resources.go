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

package utils

import (
	"encoding/pem"
	"fmt"
	"reflect"
	"strings"
	"sync"

	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/lazyresourcexds"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
)

// ClientAuthorityPublisher publishes the client certificate authority pool to
// the policy engine as one lazy resource per usage: downstream row, keyed by the
// row's name. Each resource holds "certificates" (one PEM per certificate),
// "role", and "match" only for a relay row stored with narrowing. A pool
// change reaches every API through this one push.
type ClientAuthorityPublisher struct {
	certificates config.MtlsAuthCertificateStore
	resources    *lazyresourcexds.LazyResourceStateManager

	// mu serialises Publish so the last one to run always leaves the latest
	// committed pool published.
	mu sync.Mutex
}

// NewClientAuthorityPublisher creates a publisher reading the pool from
// certificates and publishing it through resources.
func NewClientAuthorityPublisher(certificates config.MtlsAuthCertificateStore, resources *lazyresourcexds.LazyResourceStateManager) *ClientAuthorityPublisher {
	return &ClientAuthorityPublisher{certificates: certificates, resources: resources}
}

// Publish makes the published resources match the usage: downstream rows in the
// database. It stores before it removes, so no intermediate snapshot lacks a
// row that still exists.
func (p *ClientAuthorityPublisher) Publish(correlationID string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	rows, err := p.certificates.ListCertificatesByUsage(models.CertificateUsageDownstream)
	if err != nil {
		return fmt.Errorf("listing client certificate authorities: %w", err)
	}

	published := p.resources.GetResourcesByType(LazyResourceTypeClientCertificateAuthority)
	current := make(map[string]struct{}, len(rows))
	for _, row := range rows {
		current[row.Name] = struct{}{}
		resource := clientAuthorityResource(row)
		if existing, ok := published[row.Name]; ok && reflect.DeepEqual(existing.Resource, resource.Resource) {
			continue
		}
		if err := p.resources.StoreResource(resource, correlationID); err != nil {
			return fmt.Errorf("publishing client certificate authority %q: %w", row.Name, err)
		}
	}

	for name := range published {
		if _, ok := current[name]; ok {
			continue
		}
		if err := p.resources.RemoveResourceByIDAndType(name, LazyResourceTypeClientCertificateAuthority, correlationID); err != nil {
			return fmt.Errorf("removing client certificate authority %q: %w", name, err)
		}
	}
	return nil
}

// clientAuthorityResource builds the published resource for one usage:
// downstream row.
func clientAuthorityResource(row *models.StoredCertificate) *storage.LazyResource {
	role := row.EffectiveRole()
	body := map[string]interface{}{
		"certificates": splitCertificatePEMs(row.Certificate),
		"role":         role,
	}
	if role == models.CertificateRoleRelay && row.Match != nil {
		match := map[string]interface{}{}
		if len(row.Match.URISANs) > 0 {
			match["uriSANs"] = row.Match.URISANs
		}
		if len(row.Match.DNSSANs) > 0 {
			match["dnsSANs"] = row.Match.DNSSANs
		}
		if len(match) > 0 {
			body["match"] = match
		}
	}
	return &storage.LazyResource{
		ID:           row.Name,
		ResourceType: LazyResourceTypeClientCertificateAuthority,
		Resource:     body,
	}
}

// splitCertificatePEMs re-encodes each CERTIFICATE PEM block in data as its
// own string, in order, dropping other blocks and stray bytes.
func splitCertificatePEMs(data []byte) []string {
	out := []string{}
	rest := data
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			return out
		}
		if block.Type != "CERTIFICATE" {
			continue
		}
		out = append(out, strings.TrimRight(string(pem.EncodeToMemory(block)), "\n"))
	}
}
