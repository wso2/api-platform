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
	"log/slog"
	"os"
	"sync"

	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
)

type stdoutSink struct {
	mu       sync.Mutex
	logger   *slog.Logger
	throttle errThrottle
	buf      []byte
}

func newStdoutSink(logger *slog.Logger) *stdoutSink {
	return &stdoutSink{logger: logger}
}

func (s *stdoutSink) Name() string { return config.AuditOutputStdout }

func (s *stdoutSink) Write(line []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.buf = append(append(s.buf[:0], line...), '\n')
	if _, err := os.Stdout.Write(s.buf); err != nil {
		s.throttle.logError(s.logger, "Failed to write audit record; dropping it", s.Name(), err)
	}
}

func (s *stdoutSink) Close(context.Context) error { return nil }
