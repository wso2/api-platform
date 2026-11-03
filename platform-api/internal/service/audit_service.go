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
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/wso2/api-platform/platform-api/config"
)

type auditTable interface {
	Record(id string, performedAt time.Time, action, resourceUUID, resourceType, orgUUID, performedBy string) error
}

// auditRecordJSON is the line written for a record. level, msg and logType are
// fixed, so a collector can tell audit lines apart from platform-api's own JSON
// logs.
type auditRecordJSON struct {
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

// AuditService records audited changes. It implements repository.AuditRepository.
type AuditService struct {
	enabled bool
	repo    auditTable
	sinks   []auditSink
	logger  *slog.Logger
}

// NewAuditService builds the sinks named by cfg.Outputs when auditing is enabled. A
// sink that cannot be built is an error, and the caller refuses to start.
func NewAuditService(repo auditTable, cfg config.Audit, logger *slog.Logger) (*AuditService, error) {
	s := &AuditService{enabled: cfg.Enabled, repo: repo, logger: logger}
	if !cfg.Enabled {
		return s, nil
	}
	sinks, err := newAuditSinks(cfg, logger)
	if err != nil {
		return nil, err
	}
	s.sinks = sinks
	return s, nil
}

// auditSinkCleanupTimeout bounds closing the sinks already built when a later one fails
// to build, so a stuck sink cannot hide the error that caused it.
const auditSinkCleanupTimeout = 5 * time.Second

// newAuditSinks builds one sink per name in cfg.Outputs.
//
// It fails closed: a sink that cannot be built returns an error and the caller
// refuses to start, rather than running without an output the operator asked
// for. Sinks already built are closed first, so a failure part way through the
// list leaks no file descriptor or goroutine.
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
		var s auditSink
		var err error
		switch name {
		case config.AuditOutputStdout:
			s = newAuditStdoutSink(logger)
		case config.AuditOutputFile:
			s, err = newAuditFileSink(cfg.File, logger)
		case config.AuditOutputHTTP:
			s, err = newAuditHTTPSink(cfg.HTTP, logger)
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

// Record writes the table row, then the same record as one JSON line to every
// sink. Only the table error is returned: each sink handles its own failures,
// so one failing sink never stops the others and never fails the request.
func (s *AuditService) Record(action, resourceUUID, resourceType, orgUUID, performedBy string) error {
	if !s.enabled {
		return nil
	}
	id := uuid.New().String()
	// Microseconds: the finest precision every supported database stores, so
	// the row and the line hold the same time.
	performedAt := time.Now().UTC().Truncate(time.Microsecond)

	err := s.repo.Record(id, performedAt, action, resourceUUID, resourceType, orgUUID, performedBy)
	if len(s.sinks) == 0 {
		return err
	}
	line, jsonErr := json.Marshal(auditRecordJSON{
		Time:           performedAt,
		Level:          "INFO",
		Msg:            "audit",
		LogType:        "audit",
		ID:             id,
		Action:         action,
		ResourceType:   resourceType,
		ResourceID:     resourceUUID,
		OrganizationID: orgUUID,
		PerformedBy:    performedBy,
	})
	if jsonErr != nil {
		s.logger.Error("Failed to marshal audit record", "error", jsonErr)
		return err
	}
	for _, sk := range s.sinks {
		sk.Write(line)
	}
	return err
}

// Close closes every sink, flushing any that buffer. Errors are joined so one
// stuck sink does not stop the others from closing.
func (s *AuditService) Close(ctx context.Context) error {
	var errs []error
	for _, sk := range s.sinks {
		if err := sk.Close(ctx); err != nil {
			errs = append(errs, fmt.Errorf("closing the %s audit output: %w", sk.Name(), err))
		}
	}
	return errors.Join(errs...)
}
