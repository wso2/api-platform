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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/wso2/api-platform/common/eventhub"
)

// certificateSyncCalls records, in order, what a certificate event drove.
type certificateSyncCalls struct {
	calls []string
}

type fakeClientAuthorityPublisher struct {
	record *certificateSyncCalls
	err    error
}

func (f *fakeClientAuthorityPublisher) Publish(correlationID string) error {
	if f.record != nil {
		f.record.calls = append(f.record.calls, "publish:"+correlationID)
	}
	return f.err
}

type fakeCertificateSnapshot struct {
	record    *certificateSyncCalls
	reloadErr error
}

func (f *fakeCertificateSnapshot) ReloadCertificates() error {
	f.record.calls = append(f.record.calls, "reload")
	return f.reloadErr
}

func (f *fakeCertificateSnapshot) UpdateSnapshot(_ context.Context, correlationID string) error {
	f.record.calls = append(f.record.calls, "snapshot:"+correlationID)
	return nil
}

func certificateSyncListener(t *testing.T, record *certificateSyncCalls, reloadErr, publishErr error) *EventListener {
	t.Helper()
	return &EventListener{
		logger:            newTestLogger(),
		db:                setupSQLiteDBForEventListenerTests(t),
		certificates:      &fakeCertificateSnapshot{record: record, reloadErr: reloadErr},
		clientAuthorities: &fakeClientAuthorityPublisher{record: record, err: publishErr},
	}
}

func TestHandleEvent_Certificate_ReloadsPublishesAndSnapshots(t *testing.T) {
	for _, action := range []string{"CREATE", "UPDATE", "DELETE"} {
		t.Run(action, func(t *testing.T) {
			record := &certificateSyncCalls{}
			listener := certificateSyncListener(t, record, nil, nil)

			listener.handleEvent(eventhub.Event{
				EventType: eventhub.EventTypeCertificate,
				Action:    action,
				EntityID:  "cert-1",
				EventID:   "corr-1",
			})

			assert.Equal(t, []string{"reload", "publish:corr-1", "snapshot:corr-1"}, record.calls)
		})
	}
}

func TestHandleEvent_Certificate_ReloadFailureStopsTheSync(t *testing.T) {
	record := &certificateSyncCalls{}
	listener := certificateSyncListener(t, record, errors.New("database unavailable"), nil)

	listener.handleEvent(eventhub.Event{EventType: eventhub.EventTypeCertificate, Action: "CREATE", EntityID: "cert-1", EventID: "corr-1"})

	assert.Equal(t, []string{"reload"}, record.calls, "a store that did not reload must not be published or snapshotted")
}

func TestHandleEvent_Certificate_PublishFailureSkipsTheSnapshot(t *testing.T) {
	record := &certificateSyncCalls{}
	listener := certificateSyncListener(t, record, nil, errors.New("lazy resource store unavailable"))

	listener.handleEvent(eventhub.Event{EventType: eventhub.EventTypeCertificate, Action: "DELETE", EntityID: "cert-1", EventID: "corr-1"})

	assert.Equal(t, []string{"reload", "publish:corr-1"}, record.calls)
}

func TestHandleEvent_Certificate_UnknownActionDoesNothing(t *testing.T) {
	record := &certificateSyncCalls{}
	listener := certificateSyncListener(t, record, nil, nil)

	listener.handleEvent(eventhub.Event{EventType: eventhub.EventTypeCertificate, Action: "ROTATE", EntityID: "cert-1"})

	assert.Empty(t, record.calls)
}
