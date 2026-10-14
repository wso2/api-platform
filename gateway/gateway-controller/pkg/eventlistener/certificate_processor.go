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

package eventlistener

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/wso2/api-platform/common/eventhub"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/certmetrics"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/xds"
)

// ClientAuthorityPublisher republishes the usage: downstream certificate pool to
// the policy engine from the database.
type ClientAuthorityPublisher interface {
	Publish(correlationID string) error
}

// certificateSnapshot is what a certificate event rebuilds on this replica:
// the certificate store the translator reads, then the xDS snapshot built
// from it.
type certificateSnapshot interface {
	ReloadCertificates() error
	UpdateSnapshot(ctx context.Context, correlationID string) error
}

// snapshotManagerCertificates adapts the xDS snapshot manager to
// certificateSnapshot.
type snapshotManagerCertificates struct {
	snapshotManager *xds.SnapshotManager
}

func (s snapshotManagerCertificates) ReloadCertificates() error {
	translator := s.snapshotManager.GetTranslator()
	if translator == nil || translator.GetCertStore() == nil {
		return errors.New("certificate store not configured")
	}
	return translator.GetCertStore().Reload()
}

func (s snapshotManagerCertificates) UpdateSnapshot(ctx context.Context, correlationID string) error {
	return s.snapshotManager.UpdateSnapshot(ctx, correlationID)
}

// processCertificateEvent brings this replica in line with a certificate
// written on any replica. The event names the row but carries no certificate
// material, so every action rebuilds from the database: the certificate
// store, the client authority pool the policy engine reads, and the xDS
// snapshot that carries the listener's client CA, gateway identities and
// upstream trust.
func (l *EventListener) processCertificateEvent(event eventhub.Event) {
	switch event.Action {
	case "CREATE", "UPDATE", "DELETE":
	default:
		l.logger.Warn("Unknown certificate event action",
			slog.String("action", event.Action),
			slog.String("entity_id", event.EntityID))
		return
	}

	log := l.logger.With(
		slog.String("certificate_id", event.EntityID),
		slog.String("action", event.Action),
		slog.String("event_id", event.EventID))

	if l.certificates == nil {
		log.Error("Cannot apply certificate event: certificate store not configured")
		return
	}
	if err := l.certificates.ReloadCertificates(); err != nil {
		log.Error("Failed to reload certificates for replica sync", slog.Any("error", err))
		return
	}

	if err := l.clientAuthorities.Publish(event.EventID); err != nil {
		log.Error("Failed to publish client certificate authorities for replica sync", slog.Any("error", err))
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := l.certificates.UpdateSnapshot(ctx, event.EventID); err != nil {
		log.Error("Failed to update xDS snapshot for replica sync", slog.Any("error", err))
		return
	}

	if _, err := certmetrics.Refresh(l.db); err != nil {
		log.Warn("Failed to refresh certificate metrics for replica sync", slog.Any("error", err))
	}

	log.Info("Certificate change applied from replica sync")
}
