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

package webhook

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReceiveRejectsOversizedDelivery(t *testing.T) {
	service := New()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/webhook",
		bytes.NewReader(bytes.Repeat([]byte("x"), maxDeliveryBodyBytes+1)))

	service.receive("block", recorder, request)

	require.Equal(t, http.StatusRequestEntityTooLarge, recorder.Code)
	require.Empty(t, service.partitions)
}

func TestReceiveRecordsValidDelivery(t *testing.T) {
	service := New()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewBufferString(`{"event_type":"created"}`))

	service.receive("block", recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Len(t, service.partitions["block"].deliveries, 1)
}

func TestReceiveRetainsOnlyTheNewestDeliveries(t *testing.T) {
	service := New()
	for i := 0; i < maxRetainedDeliveries+1; i++ {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/webhook",
			bytes.NewBufferString(fmt.Sprintf(`{"event_type":"delivery","sequence":%d}`, i)))
		service.receive("block", recorder, request)
	}

	service.partitions["block"].mu.RLock()
	deliveries := append([]delivery(nil), service.partitions["block"].deliveries...)
	service.partitions["block"].mu.RUnlock()
	require.Len(t, deliveries, maxRetainedDeliveries)
	require.JSONEq(t, `{"event_type":"delivery","sequence":1}`, string(deliveries[0].Body))
	require.JSONEq(t, fmt.Sprintf(`{"event_type":"delivery","sequence":%d}`, maxRetainedDeliveries),
		string(deliveries[len(deliveries)-1].Body))
}
