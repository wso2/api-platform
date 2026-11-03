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

// Package audit writes audit records to the outputs named by
// [platform_api.audit] outputs: stdout, a rotating file, or an HTTP receiver.
// The audit table row is written by the caller; this package only handles the
// copies that leave the process.
package audit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/wso2/api-platform/platform-api/config"
)

// Record is one audit record, with the same values as its audit table row.
type Record struct {
	ID             string
	Time           time.Time
	Action         string
	ResourceType   string
	ResourceID     string
	OrganizationID string
	PerformedBy    string
}

// recordJSON is the line written for a Record. level, msg and logType are fixed,
// so a collector can tell audit lines apart from platform-api's own JSON logs.
type recordJSON struct {
	Time           time.Time `json:"time"`
	Level          string    `json:"level"`
	Msg            string    `json:"msg"`
	LogType        string    `json:"logType"`
	ID             string    `json:"id"`
	Action         string    `json:"action"`
	ResourceType   string    `json:"resourceType"`
	ResourceID     string    `json:"resourceId"`
	OrganizationID string    `json:"organizationId"`
	PerformedBy    string    `json:"performedBy"`
}

// Writer writes each record to every configured sink.
type Writer struct {
	sinks  []sink
	logger *slog.Logger
}

// NewWriter builds the sinks named by cfg.Outputs. A sink that cannot be built
// is an error, and the caller refuses to start.
func NewWriter(cfg config.Audit, logger *slog.Logger) (*Writer, error) {
	sinks, err := newSinks(cfg, logger)
	if err != nil {
		return nil, err
	}
	return &Writer{sinks: sinks, logger: logger}, nil
}

// cleanupTimeout bounds closing the sinks already built when a later one fails
// to build, so a stuck sink cannot hide the error that caused it.
const cleanupTimeout = 5 * time.Second

// newSinks builds one sink per name in cfg.Outputs.
//
// It fails closed: a sink that cannot be built returns an error and the caller
// refuses to start, rather than running without an output the operator asked
// for. Sinks already built are closed first, so a failure part way through the
// list leaks no file descriptor or goroutine.
func newSinks(cfg config.Audit, logger *slog.Logger) ([]sink, error) {
	outputs, err := config.NormalizeAuditOutputs(cfg.Outputs)
	if err != nil {
		return nil, err
	}

	sinks := make([]sink, 0, len(outputs))
	closeAll := func() {
		ctx, cancel := context.WithTimeout(context.Background(), cleanupTimeout)
		defer cancel()
		for _, s := range sinks {
			_ = s.Close(ctx)
		}
	}

	for _, name := range outputs {
		var s sink
		var err error
		switch name {
		case config.AuditOutputStdout:
			s = newStdoutSink(logger)
		case config.AuditOutputFile:
			s, err = newFileSink(cfg.File, logger)
		case config.AuditOutputHTTP:
			s, err = newHTTPSink(cfg.HTTP, logger)
		default:
			err = fmt.Errorf("unsupported output %q", name) // unreachable: NormalizeAuditOutputs rejects it
		}
		if err != nil {
			closeAll()
			return nil, fmt.Errorf("audit.%s: %w", name, err)
		}
		sinks = append(sinks, s)
	}
	return sinks, nil
}

// Write serializes the record once and hands the line to every sink. Each sink
// handles its own failures, so one failing sink never stops the others and
// never reaches the caller.
func (w *Writer) Write(r Record) {
	if len(w.sinks) == 0 {
		return
	}
	line, err := json.Marshal(recordJSON{
		Time:           r.Time,
		Level:          "INFO",
		Msg:            "audit",
		LogType:        "audit",
		ID:             r.ID,
		Action:         r.Action,
		ResourceType:   r.ResourceType,
		ResourceID:     r.ResourceID,
		OrganizationID: r.OrganizationID,
		PerformedBy:    r.PerformedBy,
	})
	if err != nil {
		w.logger.Error("Failed to marshal audit record", "error", err)
		return
	}
	for _, s := range w.sinks {
		s.Write(line)
	}
}

// Close closes every sink, flushing any that buffer. Errors are joined so one
// stuck sink does not stop the others from closing.
func (w *Writer) Close(ctx context.Context) error {
	var errs []error
	for _, s := range w.sinks {
		if err := s.Close(ctx); err != nil {
			errs = append(errs, fmt.Errorf("closing the %s audit output: %w", s.Name(), err))
		}
	}
	return errors.Join(errs...)
}
