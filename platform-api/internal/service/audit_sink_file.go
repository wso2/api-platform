/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 */

package service

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// FileSink appends each event as a single JSON line (NDJSON) to a file.
const auditLogFileName = "audit.log"

// Thread-safe via an internal mutex around the shared *json.Encoder.
//
// FileSink additionally implements io.Closer; it is not part of the AuditSink
// interface.
type FileSink struct {
	mu  sync.Mutex
	f   *os.File
	enc *json.Encoder
}

// NewFileSink opens dir/audit.log for appending (creating dir and the file if
// absent) and returns a FileSink ready to receive events. The caller must call
// Close when done.
func NewFileSink(dir string) (*FileSink, error) {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, fmt.Errorf("FileSink: create directory %q: %w", dir, err)
	}
	path := filepath.Join(dir, auditLogFileName)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return nil, fmt.Errorf("FileSink: open %q: %w", path, err)
	}
	return &FileSink{f: f, enc: json.NewEncoder(f)}, nil
}

// Write appends the event as a single JSON object followed by a newline.
func (s *FileSink) Write(_ context.Context, e *AuditEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.enc.Encode(e); err != nil {
		return fmt.Errorf("FileSink: encode: %w", err)
	}
	return nil
}

// Close flushes and closes the underlying file. Safe to call concurrently
// with in-flight Writes — it blocks until any Write holding the mutex returns.
func (s *FileSink) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.f.Close()
}
