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
	"encoding/json"
	"errors"
	"time"

	"platform-api/src/internal/database"
	"platform-api/src/internal/model"
)

// APIRepo implements APIRepository
type APIRepo struct {
	db *database.DB
}

// CreateAPI inserts a new API with all its configurations
func (r *APIRepo) CreateAPI(api *model.API) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	api.CreatedAt = time.Now()
	api.UpdatedAt = time.Now()

	// Convert transport slice to JSON
	transportJSON, _ := json.Marshal(api.Transport)

	// Insert main API record
	apiQuery := `
		INSERT INTO apis (uuid, name, display_name, description, context, version, provider, 
			project_id, lifecycle_status, has_thumbnail, is_default_version, is_revision, 
			revisioned_api_id, revision_id, type, transport, security_enabled, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	securityEnabled := api.Security != nil && api.Security.Enabled

	_, err = tx.Exec(apiQuery, api.ID, api.Name, api.DisplayName, api.Description,
		api.Context, api.Version, api.Provider, api.ProjectID, api.LifeCycleStatus,
		api.HasThumbnail, api.IsDefaultVersion, api.IsRevision, api.RevisionedAPIID,
		api.RevisionID, api.Type, string(transportJSON), securityEnabled, api.CreatedAt, api.UpdatedAt)
	if err != nil {
		return err
	}

	// Insert MTLS configuration
	if api.MTLS != nil {
		if err := r.insertMTLSConfig(tx, api.ID, api.MTLS); err != nil {
			return err
		}
	}

	// Insert Security configuration
	if api.Security != nil {
		if err := r.insertSecurityConfig(tx, api.ID, api.Security); err != nil {
			return err
		}
	}

	// Insert CORS configuration
	if api.CORS != nil {
		if err := r.insertCORSConfig(tx, api.ID, api.CORS); err != nil {
			return err
		}
	}

	// Insert Backend Services
	for _, backendService := range api.BackendServices {
		if err := r.insertBackendService(tx, api.ID, &backendService); err != nil {
			return err
		}
	}

	// Insert API Rate Limiting
	if api.APIRateLimiting != nil {
		if err := r.insertRateLimitingConfig(tx, api.ID, api.APIRateLimiting); err != nil {
			return err
		}
	}

	// Insert Operations
	for _, operation := range api.Operations {
		if err := r.insertOperation(tx, api.ID, &operation); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// GetAPIByUUID retrieves an API by UUID with all its configurations
func (r *APIRepo) GetAPIByUUID(uuid string) (*model.API, error) {
	api := &model.API{}

	query := `
		SELECT uuid, name, display_name, description, context, version, provider,
			project_id, lifecycle_status, has_thumbnail, is_default_version, is_revision,
			revisioned_api_id, revision_id, type, transport, security_enabled, created_at, updated_at
		FROM apis WHERE uuid = ?
	`

	var transportJSON string
	var securityEnabled bool
	err := r.db.QueryRow(query, uuid).Scan(
		&api.ID, &api.Name, &api.DisplayName, &api.Description, &api.Context,
		&api.Version, &api.Provider, &api.ProjectID, &api.LifeCycleStatus,
		&api.HasThumbnail, &api.IsDefaultVersion, &api.IsRevision,
		&api.RevisionedAPIID, &api.RevisionID, &api.Type, &transportJSON,
		&securityEnabled, &api.CreatedAt, &api.UpdatedAt)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	// Parse transport JSON
	if transportJSON != "" {
		json.Unmarshal([]byte(transportJSON), &api.Transport)
	}

	// Load related configurations
	if err := r.loadAPIConfigurations(api); err != nil {
		return nil, err
	}

	return api, nil
}

// GetAPIsByProjectID retrieves all APIs for a project
func (r *APIRepo) GetAPIsByProjectID(projectID string) ([]*model.API, error) {
	query := `
		SELECT uuid, name, display_name, description, context, version, provider,
			project_id, lifecycle_status, has_thumbnail, is_default_version, is_revision,
			revisioned_api_id, revision_id, type, transport, security_enabled, created_at, updated_at
		FROM apis WHERE project_id = ? ORDER BY created_at DESC
	`

	rows, err := r.db.Query(query, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var apis []*model.API
	for rows.Next() {
		api := &model.API{}
		var transportJSON string
		var securityEnabled bool

		err := rows.Scan(&api.ID, &api.Name, &api.DisplayName, &api.Description,
			&api.Context, &api.Version, &api.Provider, &api.ProjectID,
			&api.LifeCycleStatus, &api.HasThumbnail, &api.IsDefaultVersion,
			&api.IsRevision, &api.RevisionedAPIID, &api.RevisionID, &api.Type,
			&transportJSON, &securityEnabled, &api.CreatedAt, &api.UpdatedAt)
		if err != nil {
			return nil, err
		}

		// Parse transport JSON
		if transportJSON != "" {
			json.Unmarshal([]byte(transportJSON), &api.Transport)
		}

		// Load related configurations
		if err := r.loadAPIConfigurations(api); err != nil {
			return nil, err
		}

		apis = append(apis, api)
	}

	return apis, rows.Err()
}

// UpdateAPI modifies an existing API
func (r *APIRepo) UpdateAPI(api *model.API) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	api.UpdatedAt = time.Now()

	// Convert transport slice to JSON
	transportJSON, _ := json.Marshal(api.Transport)
	securityEnabled := api.Security != nil && api.Security.Enabled

	// Update main API record
	query := `
		UPDATE apis SET display_name = ?, description = ?,
			provider = ?, lifecycle_status = ?, has_thumbnail = ?,
			is_default_version = ?, is_revision = ?, revisioned_api_id = ?,
			revision_id = ?, type = ?, transport = ?, security_enabled = ?, updated_at = ?
		WHERE uuid = ?
	`
	_, err = tx.Exec(query, api.DisplayName, api.Description,
		api.Provider, api.LifeCycleStatus,
		api.HasThumbnail, api.IsDefaultVersion, api.IsRevision,
		api.RevisionedAPIID, api.RevisionID, api.Type, string(transportJSON),
		securityEnabled, api.UpdatedAt, api.ID)
	if err != nil {
		return err
	}

	// Delete existing configurations and re-insert
	if err := r.deleteAPIConfigurations(tx, api.ID); err != nil {
		return err
	}

	// Re-insert configurations
	if api.MTLS != nil {
		if err := r.insertMTLSConfig(tx, api.ID, api.MTLS); err != nil {
			return err
		}
	}

	if api.Security != nil {
		if err := r.insertSecurityConfig(tx, api.ID, api.Security); err != nil {
			return err
		}
	}

	if api.CORS != nil {
		if err := r.insertCORSConfig(tx, api.ID, api.CORS); err != nil {
			return err
		}
	}

	for _, backendService := range api.BackendServices {
		if err := r.insertBackendService(tx, api.ID, &backendService); err != nil {
			return err
		}
	}

	if api.APIRateLimiting != nil {
		if err := r.insertRateLimitingConfig(tx, api.ID, api.APIRateLimiting); err != nil {
			return err
		}
	}

	for _, operation := range api.Operations {
		if err := r.insertOperation(tx, api.ID, &operation); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// DeleteAPI removes an API and all its configurations
func (r *APIRepo) DeleteAPI(uuid string) error {
	// Due to foreign key constraints with CASCADE, deleting the main API record
	// will automatically delete all related configurations
	query := `DELETE FROM apis WHERE uuid = ?`
	_, err := r.db.Exec(query, uuid)
	return err
}

// Helper methods for loading configurations

func (r *APIRepo) loadAPIConfigurations(api *model.API) error {
	// Load MTLS configuration
	if mtls, err := r.loadMTLSConfig(api.ID); err != nil {
		return err
	} else if mtls != nil {
		api.MTLS = mtls
	}

	// Load Security configuration
	if security, err := r.loadSecurityConfig(api.ID); err != nil {
		return err
	} else if security != nil {
		api.Security = security
	}

	// Load CORS configuration
	if cors, err := r.loadCORSConfig(api.ID); err != nil {
		return err
	} else if cors != nil {
		api.CORS = cors
	}

	// Load Backend Services
	if backendServices, err := r.loadBackendServices(api.ID); err != nil {
		return err
	} else {
		api.BackendServices = backendServices
	}

	// Load Rate Limiting configuration
	if rateLimiting, err := r.loadRateLimitingConfig(api.ID); err != nil {
		return err
	} else if rateLimiting != nil {
		api.APIRateLimiting = rateLimiting
	}

	// Load Operations
	if operations, err := r.loadOperations(api.ID); err != nil {
		return err
	} else {
		api.Operations = operations
	}

	return nil
}

// Helper methods for MTLS configuration
func (r *APIRepo) insertMTLSConfig(tx *sql.Tx, apiUUID string, mtls *model.MTLSConfig) error {
	query := `
		INSERT INTO api_mtls_config (api_uuid, enabled, enforce_if_client_cert_present,
			verify_client, client_cert, client_key, ca_cert)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`
	_, err := tx.Exec(query, apiUUID, mtls.Enabled, mtls.EnforceIfClientCertPresent,
		mtls.VerifyClient, mtls.ClientCert, mtls.ClientKey, mtls.CACert)
	return err
}

func (r *APIRepo) loadMTLSConfig(apiUUID string) (*model.MTLSConfig, error) {
	mtls := &model.MTLSConfig{}
	query := `
		SELECT enabled, enforce_if_client_cert_present, verify_client,
			client_cert, client_key, ca_cert
		FROM api_mtls_config WHERE api_uuid = ?
	`
	err := r.db.QueryRow(query, apiUUID).Scan(&mtls.Enabled,
		&mtls.EnforceIfClientCertPresent, &mtls.VerifyClient,
		&mtls.ClientCert, &mtls.ClientKey, &mtls.CACert)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return mtls, nil
}

// Helper methods for Security configuration
func (r *APIRepo) insertSecurityConfig(tx *sql.Tx, apiUUID string, security *model.SecurityConfig) error {
	// Insert API Key security if present
	if security.APIKey != nil {
		apiKeyQuery := `
			INSERT INTO api_key_security (api_uuid, enabled, header, query, cookie)
			VALUES (?, ?, ?, ?, ?)
		`
		_, err := tx.Exec(apiKeyQuery, apiUUID, security.APIKey.Enabled,
			security.APIKey.Header, security.APIKey.Query, security.APIKey.Cookie)
		if err != nil {
			return err
		}
	}

	// Insert OAuth2 security if present
	if security.OAuth2 != nil {
		scopesJSON, _ := json.Marshal(security.OAuth2.Scopes)

		var authCodeEnabled bool
		var authCodeCallback string
		var implicitEnabled bool
		var implicitCallback string
		var passwordEnabled bool
		var clientCredEnabled bool

		if security.OAuth2.GrantTypes != nil {
			if security.OAuth2.GrantTypes.AuthorizationCode != nil {
				authCodeEnabled = security.OAuth2.GrantTypes.AuthorizationCode.Enabled
				authCodeCallback = security.OAuth2.GrantTypes.AuthorizationCode.CallbackURL
			}
			if security.OAuth2.GrantTypes.Implicit != nil {
				implicitEnabled = security.OAuth2.GrantTypes.Implicit.Enabled
				implicitCallback = security.OAuth2.GrantTypes.Implicit.CallbackURL
			}
			if security.OAuth2.GrantTypes.Password != nil {
				passwordEnabled = security.OAuth2.GrantTypes.Password.Enabled
			}
			if security.OAuth2.GrantTypes.ClientCredentials != nil {
				clientCredEnabled = security.OAuth2.GrantTypes.ClientCredentials.Enabled
			}
		}

		oauth2Query := `
			INSERT INTO oauth2_security (api_uuid, enabled, authorization_code_enabled,
				authorization_code_callback_url, implicit_enabled, implicit_callback_url,
				password_enabled, client_credentials_enabled, scopes)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		`
		_, err := tx.Exec(oauth2Query, apiUUID, true, authCodeEnabled, authCodeCallback,
			implicitEnabled, implicitCallback, passwordEnabled, clientCredEnabled, string(scopesJSON))
		if err != nil {
			return err
		}
	}

	return nil
}

func (r *APIRepo) loadSecurityConfig(apiUUID string) (*model.SecurityConfig, error) {
	security := &model.SecurityConfig{Enabled: true}

	// Load API Key security
	apiKey := &model.APIKeySecurity{}
	apiKeyQuery := `
		SELECT enabled, header, query, cookie 
		FROM api_key_security WHERE api_uuid = ?
	`
	err := r.db.QueryRow(apiKeyQuery, apiUUID).Scan(&apiKey.Enabled,
		&apiKey.Header, &apiKey.Query, &apiKey.Cookie)
	if err == nil {
		security.APIKey = apiKey
	} else if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	// Load OAuth2 security
	oauth2 := &model.OAuth2Security{}
	var scopesJSON string
	var authCodeEnabled, implicitEnabled, passwordEnabled, clientCredEnabled bool
	var authCodeCallback, implicitCallback string

	oauth2Query := `
		SELECT enabled, authorization_code_enabled, authorization_code_callback_url,
			implicit_enabled, implicit_callback_url, password_enabled, client_credentials_enabled, scopes
		FROM oauth2_security WHERE api_uuid = ?
	`
	var enabled bool
	err = r.db.QueryRow(oauth2Query, apiUUID).Scan(&enabled, &authCodeEnabled, &authCodeCallback,
		&implicitEnabled, &implicitCallback, &passwordEnabled, &clientCredEnabled, &scopesJSON)
	if err == nil {
		if scopesJSON != "" {
			json.Unmarshal([]byte(scopesJSON), &oauth2.Scopes)
		}

		// Build grant types
		grantTypes := &model.OAuth2GrantTypes{}
		if authCodeEnabled {
			grantTypes.AuthorizationCode = &model.AuthorizationCodeGrant{
				Enabled:     authCodeEnabled,
				CallbackURL: authCodeCallback,
			}
		}
		if implicitEnabled {
			grantTypes.Implicit = &model.ImplicitGrant{
				Enabled:     implicitEnabled,
				CallbackURL: implicitCallback,
			}
		}
		if passwordEnabled {
			grantTypes.Password = &model.PasswordGrant{Enabled: passwordEnabled}
		}
		if clientCredEnabled {
			grantTypes.ClientCredentials = &model.ClientCredentialsGrant{Enabled: clientCredEnabled}
		}
		oauth2.GrantTypes = grantTypes
		security.OAuth2 = oauth2
	} else if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	// Return security config only if we have API key or OAuth2 config
	if security.APIKey == nil && security.OAuth2 == nil {
		return nil, nil
	}

	return security, nil
}

// Helper methods for CORS configuration
func (r *APIRepo) insertCORSConfig(tx *sql.Tx, apiUUID string, cors *model.CORSConfig) error {
	query := `
		INSERT INTO api_cors_config (api_uuid, enabled, allow_origins, allow_methods,
			allow_headers, expose_headers, max_age, allow_credentials)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err := tx.Exec(query, apiUUID, cors.Enabled, cors.AllowOrigins,
		cors.AllowMethods, cors.AllowHeaders, cors.ExposeHeaders,
		cors.MaxAge, cors.AllowCredentials)
	return err
}

func (r *APIRepo) loadCORSConfig(apiUUID string) (*model.CORSConfig, error) {
	cors := &model.CORSConfig{}
	query := `
		SELECT enabled, allow_origins, allow_methods, allow_headers,
			expose_headers, max_age, allow_credentials
		FROM api_cors_config WHERE api_uuid = ?
	`
	err := r.db.QueryRow(query, apiUUID).Scan(&cors.Enabled, &cors.AllowOrigins,
		&cors.AllowMethods, &cors.AllowHeaders, &cors.ExposeHeaders,
		&cors.MaxAge, &cors.AllowCredentials)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return cors, nil
}

// Helper methods for Backend Services
func (r *APIRepo) insertBackendService(tx *sql.Tx, apiUUID string, service *model.BackendService) error {
	// Build timeout and load balance values
	var timeoutConnect, timeoutRead, timeoutWrite *int
	if service.Timeout != nil {
		timeoutConnect = &service.Timeout.Connect
		timeoutRead = &service.Timeout.Read
		timeoutWrite = &service.Timeout.Write
	}

	var lbAlgorithm *string
	var lbFailover *bool
	if service.LoadBalance != nil {
		lbAlgorithm = &service.LoadBalance.Algorithm
		lbFailover = &service.LoadBalance.Failover
	}

	var cbEnabled *bool
	var maxConnections, maxPendingRequests, maxRequests, maxRetries *int
	if service.CircuitBreaker != nil {
		cbEnabled = &service.CircuitBreaker.Enabled
		maxConnections = &service.CircuitBreaker.MaxConnections
		maxPendingRequests = &service.CircuitBreaker.MaxPendingRequests
		maxRequests = &service.CircuitBreaker.MaxRequests
		maxRetries = &service.CircuitBreaker.MaxRetries
	}

	// Insert backend service
	serviceQuery := `
		INSERT INTO backend_services (api_uuid, name, is_default, timeout_connect_ms, timeout_read_ms, 
			timeout_write_ms, retries, loadBalanace_algorithm, loadBalanace_failover, circuit_breaker_enabled,
			max_connections, max_pending_requests, max_requests, max_retries)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	result, err := tx.Exec(serviceQuery, apiUUID, service.Name, service.IsDefault,
		timeoutConnect, timeoutRead, timeoutWrite, service.Retries, lbAlgorithm, lbFailover,
		cbEnabled, maxConnections, maxPendingRequests, maxRequests, maxRetries)
	if err != nil {
		return err
	}

	serviceID, err := result.LastInsertId()
	if err != nil {
		return err
	}

	// Insert endpoints
	for _, endpoint := range service.Endpoints {
		if err := r.insertBackendEndpoint(tx, serviceID, &endpoint); err != nil {
			return err
		}
	}

	return nil
}

func (r *APIRepo) insertBackendEndpoint(tx *sql.Tx, serviceID int64, endpoint *model.BackendEndpoint) error {
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

	// Insert endpoint
	endpointQuery := `
		INSERT INTO backend_endpoints (backend_service_id, url, description, healthcheck_enabled,
			healthcheck_interval_seconds, healthcheck_timeout_seconds, unhealthy_threshold,
			healthy_threshold, weight, mtls_enabled, enforce_if_client_cert_present, verify_client,
			client_cert, client_key, ca_cert)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err := tx.Exec(endpointQuery, serviceID, endpoint.URL, endpoint.Description,
		hcEnabled, hcInterval, hcTimeout, unhealthyThreshold, healthyThreshold, endpoint.Weight,
		mtlsEnabled, enforceIfClientCertPresent, verifyClient, clientCert, clientKey, caCert)
	return err
}

func (r *APIRepo) loadBackendServices(apiUUID string) ([]model.BackendService, error) {
	query := `
		SELECT id, name, is_default, timeout_connect_ms, timeout_read_ms, timeout_write_ms, retries,
			loadBalanace_algorithm, loadBalanace_failover, circuit_breaker_enabled, max_connections,
			max_pending_requests, max_requests, max_retries
		FROM backend_services WHERE api_uuid = ?
	`
	rows, err := r.db.Query(query, apiUUID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var services []model.BackendService
	for rows.Next() {
		var serviceID int64
		service := model.BackendService{}

		var timeoutConnect, timeoutRead, timeoutWrite sql.NullInt64
		var lbAlgorithm sql.NullString
		var lbFailover sql.NullBool
		var cbEnabled sql.NullBool
		var maxConnections, maxPendingRequests, maxRequests, maxRetries sql.NullInt64

		err := rows.Scan(&serviceID, &service.Name, &service.IsDefault,
			&timeoutConnect, &timeoutRead, &timeoutWrite, &service.Retries,
			&lbAlgorithm, &lbFailover, &cbEnabled, &maxConnections,
			&maxPendingRequests, &maxRequests, &maxRetries)
		if err != nil {
			return nil, err
		}

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

		// Load endpoints
		if endpoints, err := r.loadBackendEndpoints(serviceID); err != nil {
			return nil, err
		} else {
			service.Endpoints = endpoints
		}

		services = append(services, service)
	}

	return services, rows.Err()
}

func (r *APIRepo) loadBackendEndpoints(serviceID int64) ([]model.BackendEndpoint, error) {
	query := `
		SELECT url, description, healthcheck_enabled, healthcheck_interval_seconds,
			healthcheck_timeout_seconds, unhealthy_threshold, healthy_threshold, weight,
			mtls_enabled, enforce_if_client_cert_present, verify_client, client_cert, client_key, ca_cert
		FROM backend_endpoints WHERE backend_service_id = ?
	`
	rows, err := r.db.Query(query, serviceID)
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

// Helper methods for Rate Limiting configuration
func (r *APIRepo) insertRateLimitingConfig(tx *sql.Tx, apiUUID string, rateLimiting *model.RateLimitingConfig) error {
	query := `
		INSERT INTO api_rate_limiting (api_uuid, enabled, rate_limit_count,
			rate_limit_time_unit, stop_on_quota_reach)
		VALUES (?, ?, ?, ?, ?)
	`
	_, err := tx.Exec(query, apiUUID, rateLimiting.Enabled, rateLimiting.RateLimitCount,
		rateLimiting.RateLimitTimeUnit, rateLimiting.StopOnQuotaReach)
	return err
}

func (r *APIRepo) loadRateLimitingConfig(apiUUID string) (*model.RateLimitingConfig, error) {
	rateLimiting := &model.RateLimitingConfig{}
	query := `
		SELECT enabled, rate_limit_count, rate_limit_time_unit, stop_on_quota_reach
		FROM api_rate_limiting WHERE api_uuid = ?
	`
	err := r.db.QueryRow(query, apiUUID).Scan(&rateLimiting.Enabled,
		&rateLimiting.RateLimitCount, &rateLimiting.RateLimitTimeUnit,
		&rateLimiting.StopOnQuotaReach)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return rateLimiting, nil
}

// Helper methods for Operations
func (r *APIRepo) insertOperation(tx *sql.Tx, apiUUID string, operation *model.Operation) error {
	var authRequired bool
	var scopesJSON string
	if operation.Request.Authentication != nil {
		authRequired = operation.Request.Authentication.Required
		if len(operation.Request.Authentication.Scopes) > 0 {
			scopesBytes, _ := json.Marshal(operation.Request.Authentication.Scopes)
			scopesJSON = string(scopesBytes)
		}
	}

	// Insert operation
	opQuery := `
		INSERT INTO api_operations (api_uuid, name, description, method, path, authentication_required, scopes)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`
	result, err := tx.Exec(opQuery, apiUUID, operation.Name, operation.Description,
		operation.Request.Method, operation.Request.Path, authRequired, scopesJSON)
	if err != nil {
		return err
	}

	operationID, err := result.LastInsertId()
	if err != nil {
		return err
	}

	// Insert backend services routing
	for _, backendRouting := range operation.Request.BackendServices {
		bsQuery := `
			INSERT INTO operation_backend_services (operation_id, backend_service_name, weight)
			VALUES (?, ?, ?)
		`
		_, err = tx.Exec(bsQuery, operationID, backendRouting.Name, backendRouting.Weight)
		if err != nil {
			return err
		}
	}

	// Insert request policies
	for _, policy := range operation.Request.RequestPolicies {
		if err := r.insertPolicy(tx, operationID, "REQUEST", &policy); err != nil {
			return err
		}
	}

	// Insert response policies
	for _, policy := range operation.Request.ResponsePolicies {
		if err := r.insertPolicy(tx, operationID, "RESPONSE", &policy); err != nil {
			return err
		}
	}

	return nil
}

func (r *APIRepo) insertPolicy(tx *sql.Tx, operationID int64, flowDirection string, policy *model.Policy) error {
	paramsJSON, _ := json.Marshal(policy.Params)
	policyQuery := `
		INSERT INTO policies (operation_id, flow_direction, name, params)
		VALUES (?, ?, ?, ?)
	`
	_, err := tx.Exec(policyQuery, operationID, flowDirection, policy.Name, string(paramsJSON))
	return err
}

func (r *APIRepo) loadOperations(apiUUID string) ([]model.Operation, error) {
	query := `
		SELECT id, name, description, method, path, authentication_required, scopes 
		FROM api_operations WHERE api_uuid = ?
	`
	rows, err := r.db.Query(query, apiUUID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var operations []model.Operation
	for rows.Next() {
		var operationID int64
		operation := model.Operation{
			Request: &model.OperationRequest{},
		}
		var authRequired bool
		var scopesJSON string

		err := rows.Scan(&operationID, &operation.Name, &operation.Description,
			&operation.Request.Method, &operation.Request.Path, &authRequired, &scopesJSON)
		if err != nil {
			return nil, err
		}

		// Build authentication config
		if authRequired || scopesJSON != "" {
			auth := &model.AuthenticationConfig{Required: authRequired}
			if scopesJSON != "" {
				json.Unmarshal([]byte(scopesJSON), &auth.Scopes)
			}
			operation.Request.Authentication = auth
		}

		// Load backend services routing
		if backendServices, err := r.loadOperationBackendServices(operationID); err != nil {
			return nil, err
		} else {
			operation.Request.BackendServices = backendServices
		}

		// Load request policies
		if reqPolicies, err := r.loadPolicies(operationID, "REQUEST"); err != nil {
			return nil, err
		} else {
			operation.Request.RequestPolicies = reqPolicies
		}

		// Load response policies
		if resPolicies, err := r.loadPolicies(operationID, "RESPONSE"); err != nil {
			return nil, err
		} else {
			operation.Request.ResponsePolicies = resPolicies
		}

		operations = append(operations, operation)
	}

	return operations, rows.Err()
}

func (r *APIRepo) loadOperationBackendServices(operationID int64) ([]model.BackendRouting, error) {
	query := `SELECT backend_service_name, weight FROM operation_backend_services WHERE operation_id = ?`
	rows, err := r.db.Query(query, operationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var backendServices []model.BackendRouting
	for rows.Next() {
		bs := model.BackendRouting{}
		err := rows.Scan(&bs.Name, &bs.Weight)
		if err != nil {
			return nil, err
		}
		backendServices = append(backendServices, bs)
	}

	return backendServices, rows.Err()
}

func (r *APIRepo) loadPolicies(operationID int64, flowDirection string) ([]model.Policy, error) {
	query := `SELECT name, params FROM policies WHERE operation_id = ? AND flow_direction = ?`
	rows, err := r.db.Query(query, operationID, flowDirection)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var policies []model.Policy
	for rows.Next() {
		policy := model.Policy{}
		var paramsJSON string
		err := rows.Scan(&policy.Name, &paramsJSON)
		if err != nil {
			return nil, err
		}

		if paramsJSON != "" {
			json.Unmarshal([]byte(paramsJSON), &policy.Params)
		}

		policies = append(policies, policy)
	}

	return policies, rows.Err()
}

// Helper method to delete all API configurations (used in Update)
func (r *APIRepo) deleteAPIConfigurations(tx *sql.Tx, apiUUID string) error {
	// Delete in reverse order of dependencies
	queries := []string{
		`DELETE FROM policies WHERE operation_id IN (SELECT id FROM api_operations WHERE api_uuid = ?)`,
		`DELETE FROM operation_backend_services WHERE operation_id IN (SELECT id FROM api_operations WHERE api_uuid = ?)`,
		`DELETE FROM api_operations WHERE api_uuid = ?`,
		`DELETE FROM api_rate_limiting WHERE api_uuid = ?`,
		`DELETE FROM backend_endpoints WHERE backend_service_id IN (SELECT id FROM backend_services WHERE api_uuid = ?)`,
		`DELETE FROM backend_services WHERE api_uuid = ?`,
		`DELETE FROM api_cors_config WHERE api_uuid = ?`,
		`DELETE FROM oauth2_security WHERE api_uuid = ?`,
		`DELETE FROM api_key_security WHERE api_uuid = ?`,
		`DELETE FROM api_mtls_config WHERE api_uuid = ?`,
	}

	for _, query := range queries {
		if _, err := tx.Exec(query, apiUUID); err != nil {
			return err
		}
	}

	return nil
}

// NewAPIRepo creates a new API repository
func NewAPIRepo(db *database.DB) APIRepository {
	return &APIRepo{db: db}
}
