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
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
)

type recordJSON struct {
	Time          time.Time `json:"time"`
	LogType       string    `json:"logType"`
	ID            string    `json:"id"`
	GatewayID     string    `json:"gatewayId"`
	CorrelationID string    `json:"correlationId"`
	Action        string    `json:"action"`
	ResourceType  string    `json:"resourceType"`
	ResourceID    string    `json:"resourceId"`
	ResourceName  string    `json:"resourceName"`
	Outcome       string    `json:"outcome"`
	PerformedBy   string    `json:"performedBy"`
}

const (
	OutcomeSucceeded = "succeeded"
	OutcomeFailed    = "failed"
)

// AuditService records changes made through the management API as a JSON line
// on every sink.
type AuditService struct {
	enabled             bool
	maskSensitiveFields bool
	gatewayID           string
	sinks               []sink
	logger              *slog.Logger
}

// NewAuditService builds the configured sinks; an error means startup must fail.
func NewAuditService(gatewayID string, cfg config.AuditConfig, logger *slog.Logger) (*AuditService, error) {
	s := &AuditService{enabled: cfg.Enabled, maskSensitiveFields: cfg.MaskSensitiveFields, gatewayID: gatewayID, logger: logger}
	if !cfg.Enabled {
		return s, nil
	}
	sinks, err := newSinks(cfg, logger)
	if err != nil {
		return nil, err
	}
	s.sinks = sinks
	return s, nil
}

const sinkCleanupTimeout = 5 * time.Second

const maskedValue = "****"

// sensitiveFields are the record fields that identify a person;
// mask_sensitive_fields hides them in the output lines.
var sensitiveFields = []func(*recordJSON) *string{
	func(r *recordJSON) *string { return &r.PerformedBy },
}

// maskSensitiveFields leaves an empty value empty, so "****" always stands for
// a real one.
func maskSensitiveFields(r *recordJSON) {
	for _, field := range sensitiveFields {
		if value := field(r); *value != "" {
			*value = maskedValue
		}
	}
}

// newSinks closes the sinks already built when a later one fails, so a failed
// startup leaks no file descriptor or goroutine.
func newSinks(cfg config.AuditConfig, logger *slog.Logger) ([]sink, error) {
	outputs, err := config.NormalizeAuditOutputs(cfg.Outputs)
	if err != nil {
		return nil, err
	}

	sinks := make([]sink, 0, len(outputs))
	closeAll := func() {
		ctx, cancel := context.WithTimeout(context.Background(), sinkCleanupTimeout)
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
		}
		if err != nil {
			closeAll()
			return nil, fmt.Errorf("controller.audit.%s: %w", name, err)
		}
		sinks = append(sinks, s)
	}
	return sinks, nil
}

// Record never fails the request: each sink handles its own failures.
func (s *AuditService) Record(action, resourceUUID, resourceName, resourceType, outcome, performedBy, correlationID string) {
	if s == nil || !s.enabled || len(s.sinks) == 0 {
		return
	}
	record := recordJSON{
		Time:          time.Now().UTC(),
		LogType:       "audit",
		ID:            uuid.New().String(),
		GatewayID:     s.gatewayID,
		CorrelationID: correlationID,
		Action:        action,
		ResourceType:  resourceType,
		ResourceID:    resourceUUID,
		ResourceName:  resourceName,
		Outcome:       outcome,
		PerformedBy:   performedBy,
	}
	if s.maskSensitiveFields {
		maskSensitiveFields(&record)
	}
	line, err := json.Marshal(record)
	if err != nil {
		s.logger.Error("Failed to marshal audit record", slog.Any("error", err))
		return
	}
	for _, sk := range s.sinks {
		sk.Write(line)
	}
}

// Close flushes and closes every sink; errors are joined. Safe on a nil service.
func (s *AuditService) Close(ctx context.Context) error {
	if s == nil {
		return nil
	}
	var errs []error
	for _, sk := range s.sinks {
		if err := sk.Close(ctx); err != nil {
			errs = append(errs, fmt.Errorf("closing the %s audit output: %w", sk.Name(), err))
		}
	}
	return errors.Join(errs...)
}
