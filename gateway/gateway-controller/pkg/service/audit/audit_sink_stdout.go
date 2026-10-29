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

package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
)

// StdoutSink writes NDJSON audit events to stdout. Thread-safe.
// Stdout also carries the application logs, so every line is tagged
// "logType":"audit" for log collectors to route on.
type StdoutSink struct {
	mu  sync.Mutex
	enc *json.Encoder
}

// NewStdoutSink constructs a StdoutSink.
func NewStdoutSink() *StdoutSink {
	return &StdoutSink{enc: json.NewEncoder(os.Stdout)}
}

// stdoutLine is an AuditEvent with the logType tag added to its JSON.
type stdoutLine struct {
	LogType string `json:"logType"`
	*AuditEvent
}

func (s *StdoutSink) Write(_ context.Context, e *AuditEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.enc.Encode(stdoutLine{LogType: "audit", AuditEvent: e}); err != nil {
		return fmt.Errorf("StdoutSink: encode: %w", err)
	}
	return nil
}
