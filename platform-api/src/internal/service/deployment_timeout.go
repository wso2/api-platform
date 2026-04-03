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
	"log/slog"
	"time"

	"platform-api/src/internal/model"
	"platform-api/src/internal/repository"
)

// DeploymentTimeoutConfig holds configuration for the timeout background job
type DeploymentTimeoutConfig struct {
	Enabled  bool          // Whether the timeout job runs
	Interval time.Duration // How often the job runs (default: 1 minute)
	Timeout  time.Duration // How long before a transitional status is considered stale (default: 5 minutes)
}

// DeploymentTimeoutService runs a background goroutine that marks stale
// DEPLOYING/UNDEPLOYING entries as either resolved or failed based on config.
type DeploymentTimeoutService struct {
	deploymentRepo repository.DeploymentRepository
	config         DeploymentTimeoutConfig
	slogger        *slog.Logger
}

// NewDeploymentTimeoutService creates a new timeout service
func NewDeploymentTimeoutService(
	deploymentRepo repository.DeploymentRepository,
	config DeploymentTimeoutConfig,
	slogger *slog.Logger,
) *DeploymentTimeoutService {
	return &DeploymentTimeoutService{
		deploymentRepo: deploymentRepo,
		config:         config,
		slogger:        slogger,
	}
}

// Start begins the background timeout job. It blocks until ctx is cancelled.
func (s *DeploymentTimeoutService) Start(ctx context.Context) {
	if !s.config.Enabled {
		s.slogger.Info("Deployment timeout job disabled")
		return
	}

	interval := s.config.Interval
	if interval <= 0 {
		interval = 1 * time.Minute
	}
	timeout := s.config.Timeout
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}

	s.slogger.Info("Deployment timeout job started",
		slog.String("interval", interval.String()), slog.String("timeout", timeout.String()))

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.slogger.Info("Deployment timeout job stopped")
			return
		case <-ticker.C:
			s.processStaleStatuses(timeout)
		}
	}
}

func (s *DeploymentTimeoutService) processStaleStatuses(timeout time.Duration) {
	stale, err := s.deploymentRepo.GetStaleTransitionalStatuses(timeout)
	if err != nil {
		s.slogger.Error("Failed to query stale deployment statuses", "error", err)
		return
	}

	if len(stale) == 0 {
		return
	}

	s.slogger.Info("Processing stale deployment statuses", "count", len(stale))

	for _, entry := range stale {
		newStatus := model.DeploymentStatusFailed
		statusReason := model.DeploymentErrorTimeout

		rowsAffected, err := s.deploymentRepo.UpdateStatusWithPerformedAtGuard(
			entry.ArtifactUUID, entry.OrganizationUUID, entry.GatewayUUID,
			newStatus, statusReason, entry.PerformedAt,
			[]model.DeploymentStatus{entry.Status},
		)
		if err != nil {
			s.slogger.Error("Failed to resolve stale deployment status",
				"artifactUUID", entry.ArtifactUUID,
				"gatewayUUID", entry.GatewayUUID,
				"error", err)
			continue
		}
		if rowsAffected > 0 {
			s.slogger.Info("Resolved stale deployment status",
				"artifactUUID", entry.ArtifactUUID,
				"gatewayUUID", entry.GatewayUUID,
				"oldStatus", entry.Status,
				"newStatus", newStatus)
		}
	}
}
