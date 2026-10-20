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

package publishers

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testTraceID = "4bf92f3577b34da6a3ce929d0e0e4736"
	testSpanID  = "00f067aa0ba902b7"
)

// The OTLP log record carries the trace context as the record's own traceId and spanId,
// so a backend links the record to its trace (#3738).
func TestOTelLogRecord_CarriesTraceContext(t *testing.T) {
	o := &OTel{cfg: testOTelConfig("http://collector/v1/logs")}
	event := restEvent()
	event.MetaInfo.TraceID = testTraceID
	event.MetaInfo.SpanID = testSpanID

	data, err := json.Marshal(o.buildRecord(event))
	require.NoError(t, err)
	var record map[string]any
	require.NoError(t, json.Unmarshal(data, &record))
	assert.Equal(t, testTraceID, record["traceId"], "OTLP/JSON encodes the trace id as hex")
	assert.Equal(t, testSpanID, record["spanId"])
}

func TestOTelLogRecord_OmitsAbsentTraceContext(t *testing.T) {
	o := &OTel{cfg: testOTelConfig("http://collector/v1/logs")}
	data, err := json.Marshal(o.buildRecord(restEvent()))
	require.NoError(t, err)
	assert.NotContains(t, string(data), "traceId")
	assert.NotContains(t, string(data), "spanId")
}

func TestTrafficLog_CarriesTraceID(t *testing.T) {
	l, err := NewLog(bothFlowsConfig())
	require.NoError(t, err)
	var buf bytes.Buffer
	useWriterSink(l, &buf)

	event := createBaseEvent()
	event.MetaInfo.TraceID = testTraceID
	l.Publish(event)

	var line map[string]any
	require.NoError(t, json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &line))
	assert.Equal(t, testTraceID, line["traceId"])
}
