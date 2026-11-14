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

type auditIdentity interface {
	DisplayIdentity(internalUserID string) (string, error)
}

const auditMaskedValue = "****"

type auditRecordJSON struct {
	Time           time.Time `json:"time"`
	LogType        string    `json:"logType"`
	ID             string    `json:"id"`
	Action         string    `json:"action"`
	ResourceType   string    `json:"resourceType"`
	ResourceID     string    `json:"resourceId"`
	OrganizationID string    `json:"organizationId"`
	PerformedBy    string    `json:"performedBy"`
}

// auditSensitiveFields are the record fields that identify a person;
// mask_sensitive_fields hides them in the output lines.
var auditSensitiveFields = []func(*auditRecordJSON) *string{
	func(r *auditRecordJSON) *string { return &r.PerformedBy },
}

// maskAuditSensitiveFields leaves an empty value empty, so "****" always
// stands for a real one.
func maskAuditSensitiveFields(r *auditRecordJSON) {
	for _, field := range auditSensitiveFields {
		if value := field(r); *value != "" {
			*value = auditMaskedValue
		}
	}
}

// AuditService implements repository.AuditRepository, so the services that
// already record through it are unchanged.
type AuditService struct {
	enabled             bool
	maskSensitiveFields bool
	repo                auditTable
	identity            auditIdentity
	sinks               []auditSink
	logger              *slog.Logger
}

// NewAuditService builds the configured sinks; an error means startup must fail.
func NewAuditService(repo auditTable, identity auditIdentity, cfg config.Audit, logger *slog.Logger) (*AuditService, error) {
	s := &AuditService{enabled: cfg.Enabled, maskSensitiveFields: cfg.MaskSensitiveFields, repo: repo, identity: identity, logger: logger}
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

const auditSinkCleanupTimeout = 5 * time.Second

// newAuditSinks closes the sinks already built when a later one fails, so a
// failed startup leaks no file descriptor or goroutine.
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
		}
		if err != nil {
			closeAll()
			return nil, fmt.Errorf("audit.%s: %w", name, err)
		}
		sinks = append(sinks, s)
	}
	return sinks, nil
}

// Record returns only the table error: a sink failure never fails the request.
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
	record := auditRecordJSON{
		Time:           performedAt,
		LogType:        "audit",
		ID:             id,
		Action:         action,
		ResourceType:   resourceType,
		ResourceID:     resourceUUID,
		OrganizationID: orgUUID,
		PerformedBy:    s.actorIdentity(performedBy),
	}
	if s.maskSensitiveFields {
		maskAuditSensitiveFields(&record)
	}
	line, jsonErr := json.Marshal(record)
	if jsonErr != nil {
		s.logger.Error("Failed to marshal audit record", "error", jsonErr)
		return err
	}
	for _, sk := range s.sinks {
		sk.Write(line)
	}
	return err
}

// actorIdentity returns the identity an output line shows for the internal
// user id that the table keeps: the token's sub, or the username for file-based
// users. The internal id must never leave the process, so a failed lookup
// shows no user; the table row still holds the id.
func (s *AuditService) actorIdentity(internalUserID string) string {
	identity, err := s.identity.DisplayIdentity(internalUserID)
	if err != nil {
		s.logger.Warn("Failed to resolve the audit actor; leaving it empty in the output", "error", err)
		return ""
	}
	return identity
}

func (s *AuditService) Close(ctx context.Context) error {
	var errs []error
	for _, sk := range s.sinks {
		if err := sk.Close(ctx); err != nil {
			errs = append(errs, fmt.Errorf("closing the %s audit output: %w", sk.Name(), err))
		}
	}
	return errors.Join(errs...)
}
