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
	"fmt"
	"log/slog"
	"time"

	"github.com/wso2/api-platform/platform-api/config"
)

// auditSinkCleanupTimeout bounds closing the sinks already built when a later one
// fails to build, so a stuck sink cannot hide the error that caused it.
const auditSinkCleanupTimeout = 5 * time.Second

// newAuditSinks builds the sinks named by audit.outputs.
//
// It fails closed: a sink that cannot be built returns an error and the caller
// refuses to start, rather than running without an output the operator asked for.
// Sinks already built are closed first, so a failure part way through the list
// leaks no file descriptor or goroutine.
func newAuditSinks(cfg config.Audit, logger *slog.Logger) ([]auditSink, error) {
	outputs, err := config.NormalizeAuditOutputs(cfg.Outputs)
	if err != nil {
		return nil, err
	}

	sinks := make([]auditSink, 0, len(outputs))
	closeAll := func() {
		ctx, cancel := context.WithTimeout(context.Background(), auditSinkCleanupTimeout)
		defer cancel()
		for _, s := range sinks {
			_ = s.Close(ctx)
		}
	}

	for _, name := range outputs {
		switch name {
		case config.AuditOutputStdout:
			sinks = append(sinks, newAuditStdoutSink(logger))
		case config.AuditOutputFile:
			s, err := newAuditFileSink(cfg.File, logger)
			if err != nil {
				closeAll()
				return nil, fmt.Errorf("audit.file: %w", err)
			}
			sinks = append(sinks, s)
		default:
			// Unreachable: NormalizeAuditOutputs rejects unknown names.
			closeAll()
			return nil, fmt.Errorf("unsupported audit.outputs entry %q", name)
		}
	}
	return sinks, nil
}
