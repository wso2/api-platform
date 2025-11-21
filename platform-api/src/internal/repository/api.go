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
	"fmt"
	"time"

	"platform-api/src/internal/database"
	"platform-api/src/internal/model"
)

// APIRepo implements APIRepository
type APIRepo struct {
	db                 *database.DB
	backendServiceRepo BackendServiceRepository
}

// NewAPIRepo creates a new API repository
func NewAPIRepo(db *database.DB) APIRepository {
	return &APIRepo{
		db:                 db,
		backendServiceRepo: NewBackendServiceRepo(db),
	}
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
			project_uuid, organization_uuid, lifecycle_status, has_thumbnail, is_default_version, is_revision, 
			revisioned_api_id, revision_id, type, transport, security_enabled, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	securityEnabled := api.Security != nil && api.Security.Enabled

	_, err = tx.Exec(apiQuery, api.ID, api.Name, api.DisplayName, api.Description,
		api.Context, api.Version, api.Provider, api.ProjectID, api.OrganizationID, api.LifeCycleStatus,
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

	// Insert Rate Limiting configuration
	if api.APIRateLimiting != nil {
		if err := r.insertRateLimitingConfig(tx, api.ID, api.APIRateLimiting); err != nil {
			return err
		}
	}

	// Insert Operations
	for _, operation := range api.Operations {
		if err := r.insertOperation(tx, api.ID, api.OrganizationID, &operation); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// GetAPIByUUID retrieves an API by ID with all its configurations
func (r *APIRepo) GetAPIByUUID(apiId string) (*model.API, error) {
	api := &model.API{}

	query := `
		SELECT uuid, name, display_name, description, context, version, provider,
			project_uuid, organization_uuid, lifecycle_status, has_thumbnail, is_default_version, is_revision,
			revisioned_api_id, revision_id, type, transport, security_enabled, created_at, updated_at
		FROM apis WHERE uuid = ?
	`

	var transportJSON string
	var securityEnabled bool
	err := r.db.QueryRow(query, apiId).Scan(
		&api.ID, &api.Name, &api.DisplayName, &api.Description, &api.Context,
		&api.Version, &api.Provider, &api.ProjectID, &api.OrganizationID, &api.LifeCycleStatus,
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
			project_uuid, organization_uuid, lifecycle_status, has_thumbnail, is_default_version, is_revision,
			revisioned_api_id, revision_id, type, transport, security_enabled, created_at, updated_at
		FROM apis WHERE project_uuid = ? ORDER BY created_at DESC
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
			&api.Context, &api.Version, &api.Provider, &api.ProjectID, &api.OrganizationID,
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

// GetAPIsByOrganizationID retrieves all APIs for an organization with optional project filter
func (r *APIRepo) GetAPIsByOrganizationID(orgID string, projectID *string) ([]*model.API, error) {
	var query string
	var args []interface{}

	if projectID != nil && *projectID != "" {
		// Filter by specific project within the organization
		query = `
			SELECT uuid, name, display_name, description, context, version, provider,
				project_uuid, organization_uuid, lifecycle_status, has_thumbnail, is_default_version, is_revision,
				revisioned_api_id, revision_id, type, transport, security_enabled, created_at, updated_at
			FROM apis
			WHERE organization_uuid = ? AND project_uuid = ?
			ORDER BY created_at DESC
		`
		args = []interface{}{orgID, *projectID}
	} else {
		// Get all APIs for the organization
		query = `
			SELECT uuid, name, display_name, description, context, version, provider,
				project_uuid, organization_uuid, lifecycle_status, has_thumbnail, is_default_version, is_revision,
				revisioned_api_id, revision_id, type, transport, security_enabled, created_at, updated_at
			FROM apis
			WHERE organization_uuid = ?
			ORDER BY created_at DESC
		`
		args = []interface{}{orgID}
	}

	rows, err := r.db.Query(query, args...)
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
			&api.Context, &api.Version, &api.Provider, &api.ProjectID, &api.OrganizationID,
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

// GetDeployedAPIsByGatewayID retrieves all APIs deployed to a specific gateway
func (r *APIRepo) GetDeployedAPIsByGatewayID(gatewayID, organizationID string) ([]*model.API, error) {
	query := `
		SELECT a.uuid, a.name, a.display_name, a.description, a.context, a.version, a.provider,
		       a.project_uuid, a.organization_uuid, a.type, a.created_at, a.updated_at
		FROM apis a
		INNER JOIN api_deployments ad ON a.uuid = ad.api_uuid
		WHERE ad.gateway_uuid = ? AND a.organization_uuid = ?
		ORDER BY a.created_at DESC
	`

	rows, err := r.db.Query(query, gatewayID, organizationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var apis []*model.API
	for rows.Next() {
		api := &model.API{}
		err := rows.Scan(&api.ID, &api.Name, &api.DisplayName, &api.Description,
			&api.Context, &api.Version, &api.Provider, &api.ProjectID, &api.OrganizationID,
			&api.Type, &api.CreatedAt, &api.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan API row: %w", err)
		}
		apis = append(apis, api)
	}

	return apis, nil
}

// GetAPIsByGatewayID retrieves all APIs associated with a specific gateway
func (r *APIRepo) GetAPIsByGatewayID(gatewayID, organizationID string) ([]*model.API, error) {
	query := `
		SELECT a.uuid, a.name, a.display_name, a.description, a.context, a.version, a.provider,
			a.project_uuid, a.organization_uuid, a.type, a.created_at, a.updated_at
		FROM apis a
		INNER JOIN api_associations aa ON a.uuid = aa.api_uuid
		WHERE aa.resource_uuid = ? AND aa.association_type = 'gateway' AND a.organization_uuid = ?
		ORDER BY a.created_at DESC
	`

	rows, err := r.db.Query(query, gatewayID, organizationID)
	if err != nil {
		return nil, fmt.Errorf("failed to query APIs associated with gateway: %w", err)
	}
	defer rows.Close()

	var apis []*model.API
	for rows.Next() {
		api := &model.API{}
		err := rows.Scan(&api.ID, &api.Name, &api.DisplayName, &api.Description,
			&api.Context, &api.Version, &api.Provider, &api.ProjectID, &api.OrganizationID,
			&api.Type, &api.CreatedAt, &api.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan API row: %w", err)
		}
		apis = append(apis, api)
	}

	return apis, nil
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

	if api.APIRateLimiting != nil {
		if err := r.insertRateLimitingConfig(tx, api.ID, api.APIRateLimiting); err != nil {
			return err
		}
	}

	// Re-insert operations
	for _, operation := range api.Operations {
		if err := r.insertOperation(tx, api.ID, api.OrganizationID, &operation); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// DeleteAPI removes an API and all its configurations
func (r *APIRepo) DeleteAPI(apiId string) error {
	// Due to foreign key constraints with CASCADE, deleting the main API record
	// will automatically delete all related configurations
	query := `DELETE FROM apis WHERE uuid = ?`
	_, err := r.db.Exec(query, apiId)
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

	// Load Backend Services associated with this API
	if backendServices, err := r.backendServiceRepo.GetBackendServicesByAPIID(api.ID); err != nil {
		return err
	} else if backendServices != nil {
		// Convert from []*model.BackendService to []model.BackendService
		api.BackendServices = make([]model.BackendService, len(backendServices))
		for i, bs := range backendServices {
			api.BackendServices[i] = *bs
		}
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
func (r *APIRepo) insertMTLSConfig(tx *sql.Tx, apiId string, mtls *model.MTLSConfig) error {
	query := `
		INSERT INTO api_mtls_config (api_uuid, enabled, enforce_if_client_cert_present,
			verify_client, client_cert, client_key, ca_cert)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`
	_, err := tx.Exec(query, apiId, mtls.Enabled, mtls.EnforceIfClientCertPresent,
		mtls.VerifyClient, mtls.ClientCert, mtls.ClientKey, mtls.CACert)
	return err
}

func (r *APIRepo) loadMTLSConfig(apiId string) (*model.MTLSConfig, error) {
	mtls := &model.MTLSConfig{}
	query := `
		SELECT enabled, enforce_if_client_cert_present, verify_client,
			client_cert, client_key, ca_cert
		FROM api_mtls_config WHERE api_uuid = ?
	`
	err := r.db.QueryRow(query, apiId).Scan(&mtls.Enabled,
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
func (r *APIRepo) insertSecurityConfig(tx *sql.Tx, apiId string, security *model.SecurityConfig) error {
	// Insert API Key security if present
	if security.APIKey != nil {
		apiKeyQuery := `
			INSERT INTO api_key_security (api_uuid, enabled, header, query, cookie)
			VALUES (?, ?, ?, ?, ?)
		`
		_, err := tx.Exec(apiKeyQuery, apiId, security.APIKey.Enabled,
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
		_, err := tx.Exec(oauth2Query, apiId, true, authCodeEnabled, authCodeCallback,
			implicitEnabled, implicitCallback, passwordEnabled, clientCredEnabled, string(scopesJSON))
		if err != nil {
			return err
		}
	}

	return nil
}

func (r *APIRepo) loadSecurityConfig(apiId string) (*model.SecurityConfig, error) {
	security := &model.SecurityConfig{Enabled: true}

	// Load API Key security
	apiKey := &model.APIKeySecurity{}
	apiKeyQuery := `
		SELECT enabled, header, query, cookie 
		FROM api_key_security WHERE api_uuid = ?
	`
	err := r.db.QueryRow(apiKeyQuery, apiId).Scan(&apiKey.Enabled,
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
	err = r.db.QueryRow(oauth2Query, apiId).Scan(&enabled, &authCodeEnabled, &authCodeCallback,
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
func (r *APIRepo) insertCORSConfig(tx *sql.Tx, apiId string, cors *model.CORSConfig) error {
	query := `
		INSERT INTO api_cors_config (api_uuid, enabled, allow_origins, allow_methods,
			allow_headers, expose_headers, max_age, allow_credentials)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err := tx.Exec(query, apiId, cors.Enabled, cors.AllowOrigins,
		cors.AllowMethods, cors.AllowHeaders, cors.ExposeHeaders,
		cors.MaxAge, cors.AllowCredentials)
	return err
}

func (r *APIRepo) loadCORSConfig(apiId string) (*model.CORSConfig, error) {
	cors := &model.CORSConfig{}
	query := `
		SELECT enabled, allow_origins, allow_methods, allow_headers,
			expose_headers, max_age, allow_credentials
		FROM api_cors_config WHERE api_uuid = ?
	`
	err := r.db.QueryRow(query, apiId).Scan(&cors.Enabled, &cors.AllowOrigins,
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

// Helper methods for Rate Limiting configuration
func (r *APIRepo) insertRateLimitingConfig(tx *sql.Tx, apiId string, rateLimiting *model.RateLimitingConfig) error {
	query := `
		INSERT INTO api_rate_limiting (api_uuid, enabled, rate_limit_count,
			rate_limit_time_unit, stop_on_quota_reach)
		VALUES (?, ?, ?, ?, ?)
	`
	_, err := tx.Exec(query, apiId, rateLimiting.Enabled, rateLimiting.RateLimitCount,
		rateLimiting.RateLimitTimeUnit, rateLimiting.StopOnQuotaReach)
	return err
}

func (r *APIRepo) loadRateLimitingConfig(apiId string) (*model.RateLimitingConfig, error) {
	rateLimiting := &model.RateLimitingConfig{}
	query := `
		SELECT enabled, rate_limit_count, rate_limit_time_unit, stop_on_quota_reach
		FROM api_rate_limiting WHERE api_uuid = ?
	`
	err := r.db.QueryRow(query, apiId).Scan(&rateLimiting.Enabled,
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
func (r *APIRepo) insertOperation(tx *sql.Tx, apiId string, organizationId string, operation *model.Operation) error {
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
	result, err := tx.Exec(opQuery, apiId, operation.Name, operation.Description,
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
		// Look up backend service UUID by name and organization ID
		var backendServiceUUID string
		lookupQuery := `SELECT uuid FROM backend_services WHERE name = ? AND organization_uuid = ?`
		err = tx.QueryRow(lookupQuery, backendRouting.Name, organizationId).Scan(&backendServiceUUID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("backend service with name '%s' not found in organization", backendRouting.Name)
			}
			return fmt.Errorf("failed to lookup backend service UUID: %w", err)
		}

		bsQuery := `
			INSERT INTO operation_backend_services (operation_id, backend_service_uuid, weight)
			VALUES (?, ?, ?)
		`
		_, err = tx.Exec(bsQuery, operationID, backendServiceUUID, backendRouting.Weight)
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

func (r *APIRepo) loadOperations(apiId string) ([]model.Operation, error) {
	query := `
		SELECT id, name, description, method, path, authentication_required, scopes 
		FROM api_operations WHERE api_uuid = ?
	`
	rows, err := r.db.Query(query, apiId)
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
	query := `
		SELECT bs.name, obs.weight
		FROM operation_backend_services obs
		JOIN backend_services bs ON bs.uuid = obs.backend_service_uuid
		WHERE obs.operation_id = ?
	`
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
func (r *APIRepo) deleteAPIConfigurations(tx *sql.Tx, apiId string) error {
	// Delete in reverse order of dependencies
	queries := []string{
		`DELETE FROM policies WHERE operation_id IN (SELECT id FROM api_operations WHERE api_uuid = ?)`,
		`DELETE FROM operation_backend_services WHERE operation_id IN (SELECT id FROM api_operations WHERE api_uuid = ?)`,
		`DELETE FROM api_operations WHERE api_uuid = ?`,
		`DELETE FROM api_backend_services WHERE api_uuid = ?`, // Remove API-backend service associations
		`DELETE FROM api_rate_limiting WHERE api_uuid = ?`,
		`DELETE FROM api_cors_config WHERE api_uuid = ?`,
		`DELETE FROM oauth2_security WHERE api_uuid = ?`,
		`DELETE FROM api_key_security WHERE api_uuid = ?`,
		`DELETE FROM api_mtls_config WHERE api_uuid = ?`,
	}

	for _, query := range queries {
		if _, err := tx.Exec(query, apiId); err != nil {
			return err
		}
	}

	return nil
}

// CreateDeployment inserts a new API deployment record
func (r *APIRepo) CreateDeployment(deployment *model.APIDeployment) error {
	deployment.CreatedAt = time.Now()

	query := `
		INSERT INTO api_deployments (api_uuid, organization_uuid, gateway_uuid, created_at)
		VALUES (?, ?, ?, ?)
	`

	result, err := r.db.Exec(query, deployment.ApiID, deployment.OrganizationID,
		deployment.GatewayID, deployment.CreatedAt)
	if err != nil {
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return err
	}

	deployment.ID = int(id)
	return nil
}

// GetDeploymentsByAPIUUID retrieves all deployment records for an API
func (r *APIRepo) GetDeploymentsByAPIUUID(apiId string) ([]*model.APIDeployment, error) {
	query := `
		SELECT id, api_uuid, organization_uuid, gateway_uuid, created_at
		FROM api_deployments
		WHERE api_uuid = ?
		ORDER BY created_at DESC
	`

	rows, err := r.db.Query(query, apiId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var deployments []*model.APIDeployment
	for rows.Next() {
		var deployment model.APIDeployment
		err := rows.Scan(&deployment.ID, &deployment.ApiID, &deployment.OrganizationID,
			&deployment.GatewayID, &deployment.CreatedAt)
		if err != nil {
			return nil, err
		}
		deployments = append(deployments, &deployment)
	}

	return deployments, rows.Err()
}

// CreateAPIAssociation creates an association between an API and resource (e.g., gateway or dev portal)
func (r *APIRepo) CreateAPIAssociation(association *model.APIAssociation) error {
	query := `
		INSERT INTO api_associations (api_uuid, organization_uuid, resource_uuid, association_type, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`
	result, err := r.db.Exec(query, association.ApiID, association.OrganizationID, association.ResourceID,
		association.AssociationType, association.CreatedAt, association.UpdatedAt)
	if err != nil {
		return err
	}

	// Get the auto-generated ID
	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	association.ID = int(id)

	return nil
}

// UpdateAPIAssociation updates the updated_at timestamp for an existing API resource association
func (r *APIRepo) UpdateAPIAssociation(apiId, resourceId, associationType, orgId string) error {
	query := `
		UPDATE api_associations 
		SET updated_at = ?
		WHERE api_uuid = ? AND resource_uuid = ? AND association_type = ? AND organization_uuid = ?
	`
	_, err := r.db.Exec(query, time.Now(), apiId, resourceId, associationType, orgId)
	return err
}

// GetAPIAssociations retrieves all resource associations for an API of a specific type
func (r *APIRepo) GetAPIAssociations(apiId, associationType, orgId string) ([]*model.APIAssociation, error) {
	query := `
		SELECT id, api_uuid, organization_uuid, resource_uuid, association_type, created_at, updated_at
		FROM api_associations
		WHERE api_uuid = ? AND association_type = ? AND organization_uuid = ?
	`
	rows, err := r.db.Query(query, apiId, associationType, orgId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var associations []*model.APIAssociation
	for rows.Next() {
		var association model.APIAssociation
		err := rows.Scan(&association.ID, &association.ApiID, &association.OrganizationID,
			&association.ResourceID, &association.AssociationType, &association.CreatedAt, &association.UpdatedAt)
		if err != nil {
			return nil, err
		}
		associations = append(associations, &association)
	}

	return associations, rows.Err()
}

// GetAPIGatewaysWithDetails retrieves all gateways associated with an API including deployment details
func (r *APIRepo) GetAPIGatewaysWithDetails(apiId, organizationId string) ([]*model.APIGatewayWithDetails, error) {
	query := `
		SELECT 
			g.uuid as id,
			g.organization_uuid as organization_id,
			g.name,
			g.display_name,
			g.description,
			g.vhost,
			g.is_critical,
			g.gateway_functionality_type as functionality_type,
			g.is_active,
			g.created_at,
			g.updated_at,
			aa.created_at as associated_at,
			aa.updated_at as association_updated_at,
			CASE WHEN ad.id IS NOT NULL THEN 1 ELSE 0 END as is_deployed,
			ad.created_at as deployed_at
		FROM gateways g
		INNER JOIN api_associations aa ON g.uuid = aa.resource_uuid AND association_type = 'gateway'
		LEFT JOIN api_deployments ad ON g.uuid = ad.gateway_uuid AND ad.api_uuid = ?
		WHERE aa.api_uuid = ? AND g.organization_uuid = ?
		ORDER BY aa.created_at DESC
	`

	rows, err := r.db.Query(query, apiId, apiId, organizationId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var gateways []*model.APIGatewayWithDetails
	for rows.Next() {
		gateway := &model.APIGatewayWithDetails{}
		var deployedAt *time.Time

		err := rows.Scan(
			&gateway.ID,
			&gateway.OrganizationID,
			&gateway.Name,
			&gateway.DisplayName,
			&gateway.Description,
			&gateway.Vhost,
			&gateway.IsCritical,
			&gateway.FunctionalityType,
			&gateway.IsActive,
			&gateway.CreatedAt,
			&gateway.UpdatedAt,
			&gateway.AssociatedAt,
			&gateway.AssociationUpdatedAt,
			&gateway.IsDeployed,
			&deployedAt,
		)
		if err != nil {
			return nil, err
		}

		gateway.DeployedAt = deployedAt
		// For now, we don't have revision information in api_deployments table
		// This can be enhanced when revision support is added
		gateway.DeployedRevision = nil

		gateways = append(gateways, gateway)
	}

	return gateways, rows.Err()
}
