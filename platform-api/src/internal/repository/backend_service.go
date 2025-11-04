/*
 *  Copyright (c) 2025, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
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

package repository

import (
	"database/sql"
	"errors"
	"time"

	"platform-api/src/internal/database"
	"platform-api/src/internal/model"
)

// BackendServiceRepo implements BackendServiceRepository
type BackendServiceRepo struct {
	db *database.DB
}

// NewBackendServiceRepo creates a new backend service repository
func NewBackendServiceRepo(db *database.DB) BackendServiceRepository {
	return &BackendServiceRepo{db: db}
}

// CreateBackendService creates a new independent backend service
func (r *BackendServiceRepo) CreateBackendService(service *model.BackendService) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	service.CreatedAt = time.Now()
	service.UpdatedAt = time.Now()

	// Insert main backend service record
	serviceQuery := `
		INSERT INTO backend_services (uuid, organization_uuid, name, description, 
			timeout_connect_ms, timeout_read_ms, timeout_write_ms, retries, 
			loadBalance_algorithm, loadBalance_failover, circuit_breaker_enabled,
			max_connections, max_pending_requests, max_requests, max_retries, 
			created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	// Extract configuration values for database insertion
	timeoutConnect, timeoutRead, timeoutWrite, lbAlgorithm, lbFailover, cbEnabled, maxConnections, maxPendingRequests,
		maxRequests, maxRetries := r.extractConfigurationForDatabase(service)

	_, err = tx.Exec(serviceQuery, service.ID, service.OrganizationID, service.Name, service.Description,
		timeoutConnect, timeoutRead, timeoutWrite, service.Retries, lbAlgorithm, lbFailover,
		cbEnabled, maxConnections, maxPendingRequests, maxRequests, maxRetries,
		service.CreatedAt, service.UpdatedAt)
	if err != nil {
		return err
	}

	// Insert endpoints using service UUID
	for _, endpoint := range service.Endpoints {
		if err := r.insertBackendEndpointByUUID(tx, service.ID, &endpoint); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// GetBackendServiceByUUID retrieves a backend service by its UUID
func (r *BackendServiceRepo) GetBackendServiceByUUID(serviceId string) (*model.BackendService, error) {
	service := &model.BackendService{}

	query := `
		SELECT uuid, organization_uuid, name, description, timeout_connect_ms, timeout_read_ms, 
			timeout_write_ms, retries, loadBalance_algorithm, loadBalance_failover,
			circuit_breaker_enabled, max_connections, max_pending_requests, max_requests, 
			max_retries, created_at, updated_at
		FROM backend_services WHERE uuid = ?
	`

	var timeoutConnect, timeoutRead, timeoutWrite sql.NullInt64
	var lbAlgorithm sql.NullString
	var lbFailover sql.NullBool
	var cbEnabled sql.NullBool
	var maxConnections, maxPendingRequests, maxRequests, maxRetries sql.NullInt64

	err := r.db.QueryRow(query, serviceId).Scan(
		&service.ID, &service.OrganizationID, &service.Name, &service.Description,
		&timeoutConnect, &timeoutRead, &timeoutWrite, &service.Retries,
		&lbAlgorithm, &lbFailover, &cbEnabled, &maxConnections,
		&maxPendingRequests, &maxRequests, &maxRetries,
		&service.CreatedAt, &service.UpdatedAt)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	// Build configurations from database values
	r.buildConfigurationFromDatabase(service, timeoutConnect, timeoutRead, timeoutWrite, lbAlgorithm, lbFailover,
		cbEnabled, maxConnections, maxPendingRequests, maxRequests, maxRetries)

	// Load endpoints
	if endpoints, err := r.loadBackendEndpointsByServiceUUID(service.ID); err != nil {
		return nil, err
	} else {
		service.Endpoints = endpoints
	}

	return service, nil
}

// GetBackendServicesByOrganizationID retrieves all backend services for an organization
func (r *BackendServiceRepo) GetBackendServicesByOrganizationID(orgID string) ([]*model.BackendService, error) {
	query := `
		SELECT uuid, organization_uuid, name, description, timeout_connect_ms, timeout_read_ms, 
			timeout_write_ms, retries, loadBalance_algorithm, loadBalance_failover,
			circuit_breaker_enabled, max_connections, max_pending_requests, max_requests, 
			max_retries, created_at, updated_at
		FROM backend_services WHERE organization_uuid = ? ORDER BY created_at DESC
	`

	rows, err := r.db.Query(query, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var services []*model.BackendService
	for rows.Next() {
		service := &model.BackendService{}
		var timeoutConnect, timeoutRead, timeoutWrite sql.NullInt64
		var lbAlgorithm sql.NullString
		var lbFailover sql.NullBool
		var cbEnabled sql.NullBool
		var maxConnections, maxPendingRequests, maxRequests, maxRetries sql.NullInt64

		err := rows.Scan(&service.ID, &service.OrganizationID, &service.Name, &service.Description,
			&timeoutConnect, &timeoutRead, &timeoutWrite, &service.Retries,
			&lbAlgorithm, &lbFailover, &cbEnabled, &maxConnections,
			&maxPendingRequests, &maxRequests, &maxRetries,
			&service.CreatedAt, &service.UpdatedAt)
		if err != nil {
			return nil, err
		}

		// Build configurations from database values
		r.buildConfigurationFromDatabase(service, timeoutConnect, timeoutRead, timeoutWrite, lbAlgorithm, lbFailover,
			cbEnabled, maxConnections, maxPendingRequests, maxRequests, maxRetries)

		// Load endpoints
		if endpoints, err := r.loadBackendEndpointsByServiceUUID(service.ID); err != nil {
			return nil, err
		} else {
			service.Endpoints = endpoints
		}

		services = append(services, service)
	}

	return services, rows.Err()
}

// GetBackendServiceByNameAndOrgID retrieves a backend service by name within an organization
func (r *BackendServiceRepo) GetBackendServiceByNameAndOrgID(name, orgID string) (*model.BackendService, error) {
	service := &model.BackendService{}

	query := `
		SELECT uuid, organization_uuid, name, description, timeout_connect_ms, timeout_read_ms, 
			timeout_write_ms, retries, loadBalance_algorithm, loadBalance_failover,
			circuit_breaker_enabled, max_connections, max_pending_requests, max_requests, 
			max_retries, created_at, updated_at
		FROM backend_services WHERE name = ? AND organization_uuid = ?
	`

	var timeoutConnect, timeoutRead, timeoutWrite sql.NullInt64
	var lbAlgorithm sql.NullString
	var lbFailover sql.NullBool
	var cbEnabled sql.NullBool
	var maxConnections, maxPendingRequests, maxRequests, maxRetries sql.NullInt64

	err := r.db.QueryRow(query, name, orgID).Scan(
		&service.ID, &service.OrganizationID, &service.Name, &service.Description,
		&timeoutConnect, &timeoutRead, &timeoutWrite, &service.Retries,
		&lbAlgorithm, &lbFailover, &cbEnabled, &maxConnections,
		&maxPendingRequests, &maxRequests, &maxRetries,
		&service.CreatedAt, &service.UpdatedAt)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	// Build configurations from database values
	r.buildConfigurationFromDatabase(service, timeoutConnect, timeoutRead, timeoutWrite, lbAlgorithm, lbFailover,
		cbEnabled, maxConnections, maxPendingRequests, maxRequests, maxRetries)

	// Load endpoints
	if endpoints, err := r.loadBackendEndpointsByServiceUUID(service.ID); err != nil {
		return nil, err
	} else {
		service.Endpoints = endpoints
	}

	return service, nil
}

// UpdateBackendService updates an existing backend service
func (r *BackendServiceRepo) UpdateBackendService(service *model.BackendService) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	service.UpdatedAt = time.Now()

	// Extract configuration values for database update
	timeoutConnect, timeoutRead, timeoutWrite, lbAlgorithm, lbFailover, cbEnabled, maxConnections, maxPendingRequests,
		maxRequests, maxRetries := r.extractConfigurationForDatabase(service)

	// Update main backend service record
	query := `
		UPDATE backend_services SET 
			name = ?, description = ?, timeout_connect_ms = ?, timeout_read_ms = ?, timeout_write_ms = ?, 
			retries = ?, loadBalance_algorithm = ?, loadBalance_failover = ?, 
			circuit_breaker_enabled = ?, max_connections = ?, max_pending_requests = ?, 
			max_requests = ?, max_retries = ?, updated_at = ?
		WHERE uuid = ?
	`
	_, err = tx.Exec(query, service.Name, service.Description, timeoutConnect, timeoutRead, timeoutWrite,
		service.Retries, lbAlgorithm, lbFailover, cbEnabled, maxConnections,
		maxPendingRequests, maxRequests, maxRetries, service.UpdatedAt, service.ID)
	if err != nil {
		return err
	}

	// Delete existing endpoints and re-insert using service UUID
	deleteEndpointsQuery := `DELETE FROM backend_endpoints WHERE backend_service_uuid = ?`
	_, err = tx.Exec(deleteEndpointsQuery, service.ID)
	if err != nil {
		return err
	}

	// Insert new endpoints
	for _, endpoint := range service.Endpoints {
		if err := r.insertBackendEndpointByUUID(tx, service.ID, &endpoint); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// DeleteBackendService deletes a backend service and all its endpoints
func (r *BackendServiceRepo) DeleteBackendService(serviceId string) error {
	// Due to foreign key constraints with CASCADE, deleting the main backend service record
	// will automatically delete all related endpoints and associations
	query := `DELETE FROM backend_services WHERE uuid = ?`
	_, err := r.db.Exec(query, serviceId)
	return err
}

// AssociateBackendServiceWithAPI creates an association between an API and a backend service
func (r *BackendServiceRepo) AssociateBackendServiceWithAPI(apiId, backendServiceId string, isDefault bool) error {
	// First check if the association already exists
	var existingCount int
	checkQuery := `SELECT COUNT(*) FROM api_backend_services WHERE api_uuid = ? AND backend_service_uuid = ?`
	err := r.db.QueryRow(checkQuery, apiId, backendServiceId).Scan(&existingCount)
	if err != nil {
		return err
	}

	if existingCount > 0 {
		// Association exists, update it
		updateQuery := `UPDATE api_backend_services SET is_default = ? WHERE api_uuid = ? AND backend_service_uuid = ?`
		_, err = r.db.Exec(updateQuery, isDefault, apiId, backendServiceId)
	} else {
		// Association doesn't exist, insert it
		insertQuery := `INSERT INTO api_backend_services (api_uuid, backend_service_uuid, is_default) VALUES (?, ?, ?)`
		_, err = r.db.Exec(insertQuery, apiId, backendServiceId, isDefault)
	}

	return err
}

// DisassociateBackendServiceFromAPI removes the association between an API and a backend service
func (r *BackendServiceRepo) DisassociateBackendServiceFromAPI(apiId, backendServiceId string) error {
	query := `DELETE FROM api_backend_services WHERE api_uuid = ? AND backend_service_uuid = ?`
	_, err := r.db.Exec(query, apiId, backendServiceId)
	return err
}

// GetBackendServicesByAPIID retrieves all backend services associated with an API
func (r *BackendServiceRepo) GetBackendServicesByAPIID(apiId string) ([]*model.BackendService, error) {
	query := `
		SELECT bs.uuid, bs.organization_uuid, bs.name, bs.description, bs.timeout_connect_ms, bs.timeout_read_ms, 
			bs.timeout_write_ms, bs.retries, bs.loadBalance_algorithm, bs.loadBalance_failover,
			bs.circuit_breaker_enabled, bs.max_connections, bs.max_pending_requests, bs.max_requests, 
			bs.max_retries, bs.created_at, bs.updated_at
		FROM backend_services bs
		JOIN api_backend_services abs ON bs.uuid = abs.backend_service_uuid
		WHERE abs.api_uuid = ?
		ORDER BY bs.created_at DESC
	`

	rows, err := r.db.Query(query, apiId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var services []*model.BackendService
	for rows.Next() {
		service := &model.BackendService{}
		var timeoutConnect, timeoutRead, timeoutWrite sql.NullInt64
		var lbAlgorithm sql.NullString
		var lbFailover sql.NullBool
		var cbEnabled sql.NullBool
		var maxConnections, maxPendingRequests, maxRequests, maxRetries sql.NullInt64

		err := rows.Scan(&service.ID, &service.OrganizationID, &service.Name, &service.Description,
			&timeoutConnect, &timeoutRead, &timeoutWrite, &service.Retries,
			&lbAlgorithm, &lbFailover, &cbEnabled, &maxConnections,
			&maxPendingRequests, &maxRequests, &maxRetries,
			&service.CreatedAt, &service.UpdatedAt)
		if err != nil {
			return nil, err
		}

		// Build configurations from database values
		r.buildConfigurationFromDatabase(service, timeoutConnect, timeoutRead, timeoutWrite, lbAlgorithm, lbFailover,
			cbEnabled, maxConnections, maxPendingRequests, maxRequests, maxRetries)

		// Load endpoints
		if endpoints, err := r.loadBackendEndpointsByServiceUUID(service.ID); err != nil {
			return nil, err
		} else {
			service.Endpoints = endpoints
		}

		services = append(services, service)
	}

	return services, rows.Err()
}

// GetAPIsByBackendServiceID retrieves all API IDs that use a specific backend service
func (r *BackendServiceRepo) GetAPIsByBackendServiceID(backendServiceId string) ([]string, error) {
	query := `SELECT api_uuid FROM api_backend_services WHERE backend_service_uuid = ?`

	rows, err := r.db.Query(query, backendServiceId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var apiIds []string
	for rows.Next() {
		var apiId string
		if err := rows.Scan(&apiId); err != nil {
			return nil, err
		}
		apiIds = append(apiIds, apiId)
	}

	return apiIds, rows.Err()
}

// Helper methods

func (r *BackendServiceRepo) insertBackendEndpointByUUID(tx *sql.Tx, serviceUUID string, endpoint *model.BackendEndpoint) error {
	var hcEnabled *bool
	var hcInterval, hcTimeout, unhealthyThreshold, healthyThreshold *int
	if endpoint.HealthCheck != nil {
		hcEnabled = &endpoint.HealthCheck.Enabled
		hcInterval = &endpoint.HealthCheck.Interval
		hcTimeout = &endpoint.HealthCheck.Timeout
		unhealthyThreshold = &endpoint.HealthCheck.UnhealthyThreshold
		healthyThreshold = &endpoint.HealthCheck.HealthyThreshold
	}

	var mtlsEnabled *bool
	var enforceIfClientCertPresent, verifyClient *bool
	var clientCert, clientKey, caCert *string
	if endpoint.MTLS != nil {
		mtlsEnabled = &endpoint.MTLS.Enabled
		enforceIfClientCertPresent = &endpoint.MTLS.EnforceIfClientCertPresent
		verifyClient = &endpoint.MTLS.VerifyClient
		clientCert = &endpoint.MTLS.ClientCert
		clientKey = &endpoint.MTLS.ClientKey
		caCert = &endpoint.MTLS.CACert
	}

	// Insert endpoint using service UUID
	endpointQuery := `
		INSERT INTO backend_endpoints (backend_service_uuid, url, description, healthcheck_enabled,
			healthcheck_interval_seconds, healthcheck_timeout_seconds, unhealthy_threshold,
			healthy_threshold, weight, mtls_enabled, enforce_if_client_cert_present, verify_client,
			client_cert, client_key, ca_cert)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err := tx.Exec(endpointQuery, serviceUUID, endpoint.URL, endpoint.Description,
		hcEnabled, hcInterval, hcTimeout, unhealthyThreshold, healthyThreshold, endpoint.Weight,
		mtlsEnabled, enforceIfClientCertPresent, verifyClient, clientCert, clientKey, caCert)
	return err
}

func (r *BackendServiceRepo) loadBackendEndpointsByServiceUUID(serviceUUID string) ([]model.BackendEndpoint, error) {
	query := `
		SELECT be.url, be.description, be.healthcheck_enabled, be.healthcheck_interval_seconds,
			be.healthcheck_timeout_seconds, be.unhealthy_threshold, be.healthy_threshold, be.weight,
			be.mtls_enabled, be.enforce_if_client_cert_present, be.verify_client, be.client_cert, be.client_key, be.ca_cert
		FROM backend_endpoints be
		WHERE be.backend_service_uuid = ?
	`
	rows, err := r.db.Query(query, serviceUUID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var endpoints []model.BackendEndpoint
	for rows.Next() {
		endpoint := model.BackendEndpoint{}

		var hcEnabled sql.NullBool
		var hcInterval, hcTimeout, unhealthyThreshold, healthyThreshold sql.NullInt64
		var mtlsEnabled sql.NullBool
		var enforceIfClientCertPresent, verifyClient sql.NullBool
		var clientCert, clientKey, caCert sql.NullString

		err := rows.Scan(&endpoint.URL, &endpoint.Description, &hcEnabled, &hcInterval,
			&hcTimeout, &unhealthyThreshold, &healthyThreshold, &endpoint.Weight,
			&mtlsEnabled, &enforceIfClientCertPresent, &verifyClient, &clientCert, &clientKey, &caCert)
		if err != nil {
			return nil, err
		}

		// Build health check config
		if hcEnabled.Valid {
			endpoint.HealthCheck = &model.HealthCheckConfig{
				Enabled:            hcEnabled.Bool,
				Interval:           int(hcInterval.Int64),
				Timeout:            int(hcTimeout.Int64),
				UnhealthyThreshold: int(unhealthyThreshold.Int64),
				HealthyThreshold:   int(healthyThreshold.Int64),
			}
		}

		// Build MTLS config
		if mtlsEnabled.Valid {
			endpoint.MTLS = &model.MTLSConfig{
				Enabled:                    mtlsEnabled.Bool,
				EnforceIfClientCertPresent: enforceIfClientCertPresent.Bool,
				VerifyClient:               verifyClient.Bool,
				ClientCert:                 clientCert.String,
				ClientKey:                  clientKey.String,
				CACert:                     caCert.String,
			}
		}

		endpoints = append(endpoints, endpoint)
	}

	return endpoints, rows.Err()
}

// Helper methods to extract duplicated configuration building logic

// extractConfigurationForDatabase extracts nullable pointers from model configurations for database insertion
func (r *BackendServiceRepo) extractConfigurationForDatabase(service *model.BackendService) (
	timeoutConnect, timeoutRead, timeoutWrite *int,
	lbAlgorithm *string, lbFailover *bool,
	cbEnabled *bool, maxConnections, maxPendingRequests, maxRequests, maxRetries *int) {

	// Extract timeout values
	if service.Timeout != nil {
		timeoutConnect = &service.Timeout.Connect
		timeoutRead = &service.Timeout.Read
		timeoutWrite = &service.Timeout.Write
	}

	// Extract load balance values
	if service.LoadBalance != nil {
		lbAlgorithm = &service.LoadBalance.Algorithm
		lbFailover = &service.LoadBalance.Failover
	}

	// Extract circuit breaker values
	if service.CircuitBreaker != nil {
		cbEnabled = &service.CircuitBreaker.Enabled
		maxConnections = &service.CircuitBreaker.MaxConnections
		maxPendingRequests = &service.CircuitBreaker.MaxPendingRequests
		maxRequests = &service.CircuitBreaker.MaxRequests
		maxRetries = &service.CircuitBreaker.MaxRetries
	}

	return
}

// buildConfigurationFromDatabase builds model configurations from nullable database values
func (r *BackendServiceRepo) buildConfigurationFromDatabase(service *model.BackendService,
	timeoutConnect, timeoutRead, timeoutWrite sql.NullInt64,
	lbAlgorithm sql.NullString, lbFailover sql.NullBool,
	cbEnabled sql.NullBool, maxConnections, maxPendingRequests, maxRequests, maxRetries sql.NullInt64) {

	// Build timeout config
	if timeoutConnect.Valid || timeoutRead.Valid || timeoutWrite.Valid {
		service.Timeout = &model.TimeoutConfig{
			Connect: int(timeoutConnect.Int64),
			Read:    int(timeoutRead.Int64),
			Write:   int(timeoutWrite.Int64),
		}
	}

	// Build load balance config
	if lbAlgorithm.Valid || lbFailover.Valid {
		service.LoadBalance = &model.LoadBalanceConfig{
			Algorithm: lbAlgorithm.String,
			Failover:  lbFailover.Bool,
		}
	}

	// Build circuit breaker config
	if cbEnabled.Valid {
		service.CircuitBreaker = &model.CircuitBreakerConfig{
			Enabled:            cbEnabled.Bool,
			MaxConnections:     int(maxConnections.Int64),
			MaxPendingRequests: int(maxPendingRequests.Int64),
			MaxRequests:        int(maxRequests.Int64),
			MaxRetries:         int(maxRetries.Int64),
		}
	}
}
