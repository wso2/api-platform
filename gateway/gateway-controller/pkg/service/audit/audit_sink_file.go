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
	"path/filepath"
	"sync"
)

// FileSink appends NDJSON audit events to a file. Thread-safe.
const logFileName = "audit.log"

// Implements io.Closer — call Close() on shutdown to flush the last write.
type FileSink struct {
	mu  sync.Mutex
	enc *json.Encoder
	f   *os.File
}

// NewFileSink opens (or creates) dir/audit.log for append-only writing,
// creating dir if it does not exist.
func NewFileSink(dir string) (*FileSink, error) {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, fmt.Errorf("failed to create audit log directory %s: %w", dir, err)
	}
	path := filepath.Join(dir, logFileName)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return nil, fmt.Errorf("failed to open audit log file %s: %w", path, err)
	}
	return &FileSink{f: f, enc: json.NewEncoder(f)}, nil
}

func (s *FileSink) Write(_ context.Context, e *AuditEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.enc.Encode(e); err != nil {
		return fmt.Errorf("FileSink: encode: %w", err)
	}
	return nil
}

func (s *FileSink) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.f.Close()
}
