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

package certmetrics

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/metrics"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
)

// fakeStore is an in-memory Store for tests: its certificate list can be
// mutated between calls to Refresh/Sweep to simulate uploads and deletes.
type fakeStore struct {
	certs []*models.StoredCertificate
}

func (f *fakeStore) ListCertificates() ([]*models.StoredCertificate, error) {
	return f.certs, nil
}

func setupTestRegistry(t *testing.T) {
	t.Helper()
	metrics.SetEnabled(true)
	metrics.Init()
}

func gaugeValue(t *testing.T, family, matchLabelValue string) (float64, bool) {
	t.Helper()
	mfs, err := metrics.GetRegistry().Gather()
	if err != nil {
		t.Fatalf("failed to gather metrics: %v", err)
	}
	for _, mf := range mfs {
		if mf.GetName() != family {
			continue
		}
		for _, m := range mf.GetMetric() {
			for _, l := range m.GetLabel() {
				if l.GetValue() == matchLabelValue {
					return m.GetGauge().GetValue(), true
				}
			}
		}
	}
	return 0, false
}

func countSeries(t *testing.T, family string) int {
	t.Helper()
	mfs, err := metrics.GetRegistry().Gather()
	if err != nil {
		t.Fatalf("failed to gather metrics: %v", err)
	}
	for _, mf := range mfs {
		if mf.GetName() == family {
			return len(mf.GetMetric())
		}
	}
	return 0
}

func hasSeriesWithLabelValue(t *testing.T, family, labelValue string) bool {
	t.Helper()
	_, ok := gaugeValue(t, family, labelValue)
	return ok
}

func TestRefresh_CountsPerUsageAndSetsExpiryGauges(t *testing.T) {
	setupTestRegistry(t)

	now := time.Now()
	store := &fakeStore{certs: []*models.StoredCertificate{
		{UUID: "u-1", Name: "upstream-a", Usage: models.CertificateUsageUpstream, NotAfter: now.Add(400 * 24 * time.Hour)},
		{UUID: "c-1", Name: "client-a", Usage: models.CertificateUsageDownstream, NotAfter: now.Add(10 * 24 * time.Hour)},
		{UUID: "c-2", Name: "client-b", Usage: models.CertificateUsageDownstream, NotAfter: now.Add(20 * 24 * time.Hour)},
		{UUID: "i-1", Name: "identity-a", Usage: models.CertificateUsageIdentity, NotAfter: now.Add(30 * 24 * time.Hour)},
	}}

	if _, err := Refresh(store); err != nil {
		t.Fatalf("Refresh returned error: %v", err)
	}

	if v, ok := gaugeValue(t, "gateway_controller_certificates_total", models.CertificateUsageDownstream); !ok || v != 2 {
		t.Fatalf("expected certificates_total{usage=downstream}=2, got %v (found=%v)", v, ok)
	}
	if v, ok := gaugeValue(t, "gateway_controller_certificates_total", models.CertificateUsageUpstream); !ok || v != 1 {
		t.Fatalf("expected certificates_total{usage=upstream}=1, got %v (found=%v)", v, ok)
	}
	if v, ok := gaugeValue(t, "gateway_controller_certificates_total", models.CertificateUsageIdentity); !ok || v != 1 {
		t.Fatalf("expected certificates_total{usage=identity}=1, got %v (found=%v)", v, ok)
	}

	if got := countSeries(t, "gateway_controller_certificate_expiry_seconds"); got != 4 {
		t.Fatalf("expected 4 certificate_expiry_seconds series, got %d", got)
	}
	if v, ok := gaugeValue(t, "gateway_controller_certificate_expiry_seconds", "client-a"); !ok {
		t.Fatalf("expected an expiry series for client-a")
	} else if int64(v) != store.certs[1].NotAfter.Unix() {
		t.Fatalf("expected client-a expiry gauge to equal its NotAfter, got %v", v)
	}
}

func TestRefresh_DropsTheSeriesOfADeletedCertificate(t *testing.T) {
	setupTestRegistry(t)

	notAfter := time.Now().Add(400 * 24 * time.Hour)
	store := &fakeStore{certs: []*models.StoredCertificate{
		{UUID: "c-1", Name: "client-a", Usage: models.CertificateUsageDownstream, NotAfter: notAfter},
		{UUID: "c-2", Name: "client-b", Usage: models.CertificateUsageDownstream, NotAfter: notAfter},
	}}
	if _, err := Refresh(store); err != nil {
		t.Fatalf("Refresh returned error: %v", err)
	}
	if !hasSeriesWithLabelValue(t, "gateway_controller_certificate_expiry_seconds", "client-b") {
		t.Fatalf("expected an expiry series for client-b before the delete")
	}

	store.certs = store.certs[:1]
	if _, err := Refresh(store); err != nil {
		t.Fatalf("Refresh returned error: %v", err)
	}

	if hasSeriesWithLabelValue(t, "gateway_controller_certificate_expiry_seconds", "client-b") {
		t.Fatalf("expected no expiry series for the deleted client-b")
	}
	if !hasSeriesWithLabelValue(t, "gateway_controller_certificate_expiry_seconds", "client-a") {
		t.Fatalf("expected the expiry series for client-a to remain")
	}
	if v, ok := gaugeValue(t, "gateway_controller_certificates_total", models.CertificateUsageDownstream); !ok || v != 1 {
		t.Fatalf("expected certificates_total{usage=downstream}=1, got %v (found=%v)", v, ok)
	}
}

func TestSweep_LogsOneWarnPerExpiringCertificate(t *testing.T) {
	setupTestRegistry(t)

	now := time.Now()
	store := &fakeStore{certs: []*models.StoredCertificate{
		{UUID: "c-1", Name: "expiring-soon", Usage: models.CertificateUsageDownstream, NotAfter: now.Add(5 * 24 * time.Hour)},
		{UUID: "c-2", Name: "not-expiring", Usage: models.CertificateUsageDownstream, NotAfter: now.Add(400 * 24 * time.Hour)},
		{UUID: "i-1", Name: "identity-expiring", Usage: models.CertificateUsageIdentity, NotAfter: now.Add(1 * time.Hour)},
	}}

	var buf bytes.Buffer
	log := slog.New(slog.NewTextHandler(&buf, nil))

	// With the context already cancelled, Sweep runs once and returns.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	Sweep(ctx, store, log)

	output := buf.String()
	if got := strings.Count(output, "CERT_EXPIRES_SOON"); got != 2 {
		t.Fatalf("expected exactly 2 CERT_EXPIRES_SOON warnings, got %d in log:\n%s", got, output)
	}
	if !strings.Contains(output, "expiring-soon") {
		t.Fatalf("expected a warning naming expiring-soon, got:\n%s", output)
	}
	if !strings.Contains(output, "identity-expiring") {
		t.Fatalf("expected a warning naming identity-expiring, got:\n%s", output)
	}
	if strings.Contains(output, "not-expiring") {
		t.Fatalf("did not expect a warning naming not-expiring, got:\n%s", output)
	}
}
