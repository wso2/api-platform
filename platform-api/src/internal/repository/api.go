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

	"platform-api/src/internal/constants"
	"platform-api/src/internal/database"
	"platform-api/src/internal/model"

	"github.com/google/uuid"
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

	// Always generate a new UUID for the API
	api.ID = uuid.New().String()
	api.CreatedAt = time.Now()
	api.UpdatedAt = time.Now()

	// Convert transport slice to JSON
	transportJSON, _ := json.Marshal(api.Transport)
	policiesJSON, err := serializePolicies(api.Policies)
	if err != nil {
		return err
	}

	// Insert main API record
	apiQuery := `
		INSERT INTO apis (uuid, handle, name, description, context, version, provider,
			project_uuid, organization_uuid, lifecycle_status, has_thumbnail, is_default_version,
			type, transport, policies, security_enabled, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	securityEnabled := api.Security != nil && api.Security.Enabled

	_, err = tx.Exec(r.db.Rebind(apiQuery), api.ID, api.Handle, api.Name, api.Description,
		api.Context, api.Version, api.Provider, api.ProjectID, api.OrganizationID, api.LifeCycleStatus,
		api.HasThumbnail, api.IsDefaultVersion, api.Type, string(transportJSON), policiesJSON, securityEnabled, api.CreatedAt, api.UpdatedAt)
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

	// Insert Channels
	for _, channel := range api.Channels {
		if err := r.insertChannel(tx, api.ID, &channel); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// GetAPIByUUID retrieves an API by UUID with all its configurations
func (r *APIRepo) GetAPIByUUID(apiUUID, orgUUID string) (*model.API, error) {
	api := &model.API{}

	query := `
		SELECT uuid, handle, name, description, context, version, provider,
			project_uuid, organization_uuid, lifecycle_status, has_thumbnail, is_default_version,
			type, transport, policies, security_enabled, created_at, updated_at
		FROM apis WHERE uuid = ? and organization_uuid = ?
	`

	var transportJSON string
	var policiesJSON sql.NullString
	var securityEnabled bool
	err := r.db.QueryRow(r.db.Rebind(query), apiUUID, orgUUID).Scan(
		&api.ID, &api.Handle, &api.Name, &api.Description, &api.Context,
		&api.Version, &api.Provider, &api.ProjectID, &api.OrganizationID, &api.LifeCycleStatus,
		&api.HasThumbnail, &api.IsDefaultVersion, &api.Type, &transportJSON, &policiesJSON,
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
	if policies, err := deserializePolicies(policiesJSON); err != nil {
		return nil, err
	} else {
		api.Policies = policies
	}

	// Load related configurations
	if err := r.loadAPIConfigurations(api); err != nil {
		return nil, err
	}

	return api, nil
}

// GetAPIMetadataByHandle retrieves minimal API information by handle and organization ID
func (r *APIRepo) GetAPIMetadataByHandle(handle, orgUUID string) (*model.APIMetadata, error) {
	metadata := &model.APIMetadata{}

	query := `SELECT uuid, handle, name, context, organization_uuid FROM apis WHERE handle = ? AND organization_uuid = ?`

	err := r.db.QueryRow(r.db.Rebind(query), handle, orgUUID).Scan(
		&metadata.ID, &metadata.Handle, &metadata.Name, &metadata.Context, &metadata.OrganizationID)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	return metadata, nil
}

// GetAPIsByProjectUUID retrieves all APIs for a project
func (r *APIRepo) GetAPIsByProjectUUID(projectUUID, orgUUID string) ([]*model.API, error) {
	query := `
		SELECT uuid, handle, name, description, context, version, provider,
			project_uuid, organization_uuid, lifecycle_status, has_thumbnail, is_default_version,
			type, transport, policies, security_enabled, created_at, updated_at
		FROM apis WHERE project_uuid = ? AND organization_uuid = ? ORDER BY created_at DESC
	`

	rows, err := r.db.Query(r.db.Rebind(query), projectUUID, orgUUID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var apis []*model.API
	for rows.Next() {
		api := &model.API{}
		var transportJSON string
		var policiesJSON sql.NullString
		var securityEnabled bool

		err := rows.Scan(&api.ID, &api.Handle, &api.Name, &api.Description,
			&api.Context, &api.Version, &api.Provider, &api.ProjectID, &api.OrganizationID,
			&api.LifeCycleStatus, &api.HasThumbnail, &api.IsDefaultVersion,
			&api.Type, &transportJSON, &policiesJSON, &securityEnabled, &api.CreatedAt, &api.UpdatedAt)
		if err != nil {
			return nil, err
		}

		// Parse transport JSON
		if transportJSON != "" {
			json.Unmarshal([]byte(transportJSON), &api.Transport)
		}
		if policies, err := deserializePolicies(policiesJSON); err != nil {
			return nil, err
		} else {
			api.Policies = policies
		}

		// Load related configurations
		if err := r.loadAPIConfigurations(api); err != nil {
			return nil, err
		}

		apis = append(apis, api)
	}

	return apis, rows.Err()
}

// GetAPIsByOrganizationUUID retrieves all APIs for an organization with optional project filter
func (r *APIRepo) GetAPIsByOrganizationUUID(orgUUID string, projectUUID *string) ([]*model.API, error) {
	var query string
	var args []interface{}

	if projectUUID != nil && *projectUUID != "" {
		// Filter by specific project within the organization
		query = `
			SELECT uuid, handle, name, description, context, version, provider,
				project_uuid, organization_uuid, lifecycle_status, has_thumbnail, is_default_version,
				type, transport, policies, security_enabled, created_at, updated_at
			FROM apis
			WHERE organization_uuid = ? AND project_uuid = ?
			ORDER BY created_at DESC
		`
		args = []interface{}{orgUUID, *projectUUID}
	} else {
		// Get all APIs for the organization
		query = `
			SELECT uuid, handle, name, description, context, version, provider,
				project_uuid, organization_uuid, lifecycle_status, has_thumbnail, is_default_version,
				type, transport, policies, security_enabled, created_at, updated_at
			FROM apis
			WHERE organization_uuid = ?
			ORDER BY created_at DESC
		`
		args = []interface{}{orgUUID}
	}

	rows, err := r.db.Query(r.db.Rebind(query), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var apis []*model.API
	for rows.Next() {
		api := &model.API{}
		var transportJSON string
		var policiesJSON sql.NullString
		var securityEnabled bool

		err := rows.Scan(&api.ID, &api.Handle, &api.Name, &api.Description,
			&api.Context, &api.Version, &api.Provider, &api.ProjectID, &api.OrganizationID,
			&api.LifeCycleStatus, &api.HasThumbnail, &api.IsDefaultVersion,
			&api.Type, &transportJSON, &policiesJSON, &securityEnabled, &api.CreatedAt, &api.UpdatedAt)
		if err != nil {
			return nil, err
		}

		// Parse transport JSON
		if transportJSON != "" {
			json.Unmarshal([]byte(transportJSON), &api.Transport)
		}
		if policies, err := deserializePolicies(policiesJSON); err != nil {
			return nil, err
		} else {
			api.Policies = policies
		}

		// Load related configurations
		if err := r.loadAPIConfigurations(api); err != nil {
			return nil, err
		}

		apis = append(apis, api)
	}

	return apis, rows.Err()
}

// GetDeployedAPIsByGatewayUUID retrieves all APIs deployed to a specific gateway
func (r *APIRepo) GetDeployedAPIsByGatewayUUID(gatewayUUID, orgUUID string) ([]*model.API, error) {
	query := `
		SELECT a.uuid, a.name, a.description, a.context, a.version, a.provider,
		       a.project_uuid, a.organization_uuid, a.type, a.created_at, a.updated_at
		FROM apis a
		INNER JOIN api_deployment_status ad ON a.uuid = ad.api_uuid
		WHERE ad.gateway_uuid = ? AND a.organization_uuid = ? AND ad.status = ?
		ORDER BY a.created_at DESC
	`

	rows, err := r.db.Query(r.db.Rebind(query), gatewayUUID, orgUUID, string(model.DeploymentStatusDeployed))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var apis []*model.API
	for rows.Next() {
		api := &model.API{}
		err := rows.Scan(&api.ID, &api.Name, &api.Description,
			&api.Context, &api.Version, &api.Provider, &api.ProjectID, &api.OrganizationID,
			&api.Type, &api.CreatedAt, &api.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan API row: %w", err)
		}
		apis = append(apis, api)
	}

	return apis, nil
}

// GetAPIsByGatewayUUID retrieves all APIs associated with a specific gateway
func (r *APIRepo) GetAPIsByGatewayUUID(gatewayUUID, orgUUID string) ([]*model.API, error) {
	query := `
		SELECT a.uuid, a.name, a.description, a.context, a.version, a.provider,
			a.project_uuid, a.organization_uuid, a.type, a.created_at, a.updated_at
		FROM apis a
		INNER JOIN api_associations aa ON a.uuid = aa.api_uuid
		WHERE aa.resource_uuid = ? AND aa.association_type = 'gateway' AND a.organization_uuid = ?
		ORDER BY a.created_at DESC
	`

	rows, err := r.db.Query(r.db.Rebind(query), gatewayUUID, orgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to query APIs associated with gateway: %w", err)
	}
	defer rows.Close()

	var apis []*model.API
	for rows.Next() {
		api := &model.API{}
		err := rows.Scan(&api.ID, &api.Name, &api.Description,
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
	policiesJSON, err := serializePolicies(api.Policies)
	if err != nil {
		return err
	}
	securityEnabled := api.Security != nil && api.Security.Enabled

	// Update main API record
	query := `
		UPDATE apis SET description = ?,
			provider = ?, lifecycle_status = ?, has_thumbnail = ?,
			is_default_version = ?, type = ?, transport = ?, policies = ?, security_enabled = ?, updated_at = ?
		WHERE uuid = ?
	`
	_, err = tx.Exec(r.db.Rebind(query), api.Description,
		api.Provider, api.LifeCycleStatus,
		api.HasThumbnail, api.IsDefaultVersion, api.Type, string(transportJSON), policiesJSON,
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

	// Re-insert channels
	for _, channel := range api.Channels {
		if err := r.insertChannel(tx, api.ID, &channel); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// DeleteAPI removes an API and all its configurations
func (r *APIRepo) DeleteAPI(apiUUID, orgUUID string) error {
	// Start transaction for atomicity
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Delete in order of dependencies (children first, parent last)
	deleteQueries := []string{
		// Delete API associations first
		`DELETE FROM api_associations WHERE api_uuid = ?`,
		// Delete API deployments
		`DELETE FROM api_deployments WHERE api_uuid = ?`,
		// Delete other related tables that reference the API
		`DELETE FROM operation_backend_services WHERE operation_id IN (SELECT id FROM api_operations WHERE api_uuid = ?)`,
		`DELETE FROM api_operations WHERE api_uuid = ?`,
		`DELETE FROM api_backend_services WHERE api_uuid = ?`,
		`DELETE FROM api_rate_limiting WHERE api_uuid = ?`,
		`DELETE FROM api_cors_config WHERE api_uuid = ?`,
		`DELETE FROM oauth2_security WHERE api_uuid = ?`,
		`DELETE FROM api_key_security WHERE api_uuid = ?`,
		`DELETE FROM api_mtls_config WHERE api_uuid = ?`,
		`DELETE FROM xhub_signature_security WHERE api_uuid = ?`,
		// Finally delete the main API record
		`DELETE FROM apis WHERE uuid = ?`,
	}

	// Execute all delete statements
	for _, query := range deleteQueries {
		if _, err := tx.Exec(r.db.Rebind(query), apiUUID); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// CheckAPIExistsByHandleInOrganization checks if an API with the given handle exists within a specific organization
func (r *APIRepo) CheckAPIExistsByHandleInOrganization(handle, orgUUID string) (bool, error) {
	query := `
		SELECT COUNT(*) FROM apis
		WHERE handle = ? AND organization_uuid = ?
	`

	var count int
	err := r.db.QueryRow(r.db.Rebind(query), handle, orgUUID).Scan(&count)
	if err != nil {
		return false, err
	}

	return count > 0, nil
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

	// Load Channels
	if channels, err := r.loadChannels(api.ID); err != nil {
		return err
	} else {
		api.Channels = channels
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
	_, err := tx.Exec(r.db.Rebind(query), apiId, mtls.Enabled, mtls.EnforceIfClientCertPresent,
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
	err := r.db.QueryRow(r.db.Rebind(query), apiId).Scan(&mtls.Enabled,
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
		_, err := tx.Exec(r.db.Rebind(apiKeyQuery), apiId, security.APIKey.Enabled,
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
		_, err := tx.Exec(r.db.Rebind(oauth2Query), apiId, true, authCodeEnabled, authCodeCallback,
			implicitEnabled, implicitCallback, passwordEnabled, clientCredEnabled, string(scopesJSON))
		if err != nil {
			return err
		}
	}

	if security.XHubSignature != nil {
		xHubQuery := `
				INSERT INTO xhub_signature_security (api_uuid, enabled, secret, algorithm, header)
				VALUES (?, ?, ?, ?, ?)
			`
		_, err := tx.Exec(r.db.Rebind(xHubQuery), apiId, security.XHubSignature.Enabled,
			security.XHubSignature.Secret, security.XHubSignature.Algorithm, security.XHubSignature.Header)
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
	err := r.db.QueryRow(r.db.Rebind(apiKeyQuery), apiId).Scan(&apiKey.Enabled,
		&apiKey.Header, &apiKey.Query, &apiKey.Cookie)
	if err == nil {
		security.APIKey = apiKey
	} else if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	// Load XHub Signature security if present
	xHub := &model.XHubSignatureSecurity{}
	xHubQuery := `
			SELECT enabled, secret, algorithm, header
			FROM xhub_signature_security WHERE api_uuid = ?
		`
	err = r.db.QueryRow(r.db.Rebind(xHubQuery), apiId).Scan(&xHub.Enabled,
		&xHub.Secret, &xHub.Algorithm, &xHub.Header)
	if err == nil {
		security.XHubSignature = xHub
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
	err = r.db.QueryRow(r.db.Rebind(oauth2Query), apiId).Scan(&enabled, &authCodeEnabled, &authCodeCallback,
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

	// Return security config only if we have API key or OAuth2 config or XHubSignature config
	if security.APIKey == nil && security.OAuth2 == nil && security.XHubSignature == nil {
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
	_, err := tx.Exec(r.db.Rebind(query), apiId, cors.Enabled, cors.AllowOrigins,
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
	err := r.db.QueryRow(r.db.Rebind(query), apiId).Scan(&cors.Enabled, &cors.AllowOrigins,
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
	_, err := tx.Exec(r.db.Rebind(query), apiId, rateLimiting.Enabled, rateLimiting.RateLimitCount,
		rateLimiting.RateLimitTimeUnit, rateLimiting.StopOnQuotaReach)
	return err
}

func (r *APIRepo) loadRateLimitingConfig(apiId string) (*model.RateLimitingConfig, error) {
	rateLimiting := &model.RateLimitingConfig{}
	query := `
		SELECT enabled, rate_limit_count, rate_limit_time_unit, stop_on_quota_reach
		FROM api_rate_limiting WHERE api_uuid = ?
	`
	err := r.db.QueryRow(r.db.Rebind(query), apiId).Scan(&rateLimiting.Enabled,
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
	var err error
	if operation.Request.Authentication != nil {
		authRequired = operation.Request.Authentication.Required
		if len(operation.Request.Authentication.Scopes) > 0 {
			scopesBytes, _ := json.Marshal(operation.Request.Authentication.Scopes)
			scopesJSON = string(scopesBytes)
		}
	}
	policiesValue, err := serializePolicies(operation.Request.Policies)
	if err != nil {
		return err
	}

	// Insert operation
	var operationID int64
	if r.db.Driver() == "postgres" || r.db.Driver() == "postgresql" {
		// PostgreSQL: use RETURNING to get the generated ID
		opQuery := `
			INSERT INTO api_operations (api_uuid, name, description, method, path, authentication_required, scopes, policies)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)
			RETURNING id
		`
		if err := tx.QueryRow(r.db.Rebind(opQuery), apiId, operation.Name, operation.Description,
			operation.Request.Method, operation.Request.Path, authRequired, scopesJSON, policiesValue).Scan(&operationID); err != nil {
			return err
		}
	} else {
		// SQLite (and other drivers that support LastInsertId)
		opQuery := `
			INSERT INTO api_operations (api_uuid, name, description, method, path, authentication_required, scopes, policies)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		`
		result, err := tx.Exec(r.db.Rebind(opQuery), apiId, operation.Name, operation.Description,
			operation.Request.Method, operation.Request.Path, authRequired, scopesJSON, policiesValue)
		if err != nil {
			return err
		}

		var lastID int64
		lastID, err = result.LastInsertId()
		if err != nil {
			return err
		}
		operationID = lastID
	}

	// Insert backend services routing
	for _, backendRouting := range operation.Request.BackendServices {
		// Look up backend service UUID by name and organization ID
		var backendServiceUUID string
		lookupQuery := `SELECT uuid FROM backend_services WHERE name = ? AND organization_uuid = ?`
		err = tx.QueryRow(r.db.Rebind(lookupQuery), backendRouting.Name, organizationId).Scan(&backendServiceUUID)
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
		if _, err = tx.Exec(r.db.Rebind(bsQuery), operationID, backendServiceUUID, backendRouting.Weight); err != nil {
			return err
		}
	}

	return nil
}

func (r *APIRepo) insertChannel(tx *sql.Tx, apiId string, channel *model.Channel) error {
	var authRequired bool
	var scopesJSON string
	if channel.Request.Authentication != nil {
		authRequired = channel.Request.Authentication.Required
		if len(channel.Request.Authentication.Scopes) > 0 {
			scopesBytes, _ := json.Marshal(channel.Request.Authentication.Scopes)
			scopesJSON = string(scopesBytes)
		}
	}
	policiesJSON, err := serializePolicies(channel.Request.Policies)
	if err != nil {
		return err
	}
	// Insert channel
	var channelID int64
	if r.db.Driver() == "postgres" || r.db.Driver() == "postgresql" {
		// PostgreSQL: use RETURNING to get the generated ID
		channelQuery := `
		INSERT INTO api_operations (api_uuid, name, description, method, path, authentication_required, scopes, policies)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		RETURNING id`
		if err := tx.QueryRow(r.db.Rebind(channelQuery), apiId, channel.Name, channel.Description,
			channel.Request.Method, channel.Request.Name, authRequired, scopesJSON, policiesJSON).Scan(&channelID); err != nil {
			return err
		}
	} else {
		// SQLite (and other drivers that support LastInsertId)
		channelQuery := `
		INSERT INTO api_operations (api_uuid, name, description, method, path, authentication_required, scopes, policies)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
		result, err := tx.Exec(r.db.Rebind(channelQuery), apiId, channel.Name, channel.Description,
			channel.Request.Method, channel.Request.Name, authRequired, scopesJSON, policiesJSON)
		if err != nil {
			return err
		}

		channelID, err = result.LastInsertId()
		if err != nil {
			return err
		}
	}

	return nil
}

func (r *APIRepo) loadChannels(apiId string) ([]model.Channel, error) {
	query := `
		SELECT id, name, description, method, path, authentication_required, scopes, policies 
		FROM api_operations WHERE api_uuid = ?
	`
	rows, err := r.db.Query(r.db.Rebind(query), apiId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var channels []model.Channel
	for rows.Next() {
		var operationID int64
		channel := model.Channel{
			Request: &model.ChannelRequest{},
		}
		var authRequired bool
		var scopesJSON string
		var policiesJSON sql.NullString

		err := rows.Scan(&operationID, &channel.Name, &channel.Description,
			&channel.Request.Method, &channel.Request.Name, &authRequired, &scopesJSON, &policiesJSON)
		if err != nil {
			return nil, err
		}

		// Build authentication config
		if authRequired || scopesJSON != "" {
			auth := &model.AuthenticationConfig{Required: authRequired}
			if scopesJSON != "" {
				json.Unmarshal([]byte(scopesJSON), &auth.Scopes)
			}
			channel.Request.Authentication = auth
		}

		policies, err := deserializePolicies(policiesJSON)
		if err != nil {
			return nil, err
		}
		if policies != nil {
			channel.Request.Policies = policies
		}

		channels = append(channels, channel)
	}

	return channels, rows.Err()
}

func (r *APIRepo) loadOperations(apiId string) ([]model.Operation, error) {
	query := `
		SELECT id, name, description, method, path, authentication_required, scopes, policies 
		FROM api_operations WHERE api_uuid = ?
	`
	rows, err := r.db.Query(r.db.Rebind(query), apiId)
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
		var policiesJSON sql.NullString

		err := rows.Scan(&operationID, &operation.Name, &operation.Description,
			&operation.Request.Method, &operation.Request.Path, &authRequired, &scopesJSON, &policiesJSON)
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

		policies, err := deserializePolicies(policiesJSON)
		if err != nil {
			return nil, err
		}
		if policies != nil {
			operation.Request.Policies = policies
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
	rows, err := r.db.Query(r.db.Rebind(query), operationID)
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

func serializePolicies(policies []model.Policy) (any, error) {
	if policies == nil {
		policies = []model.Policy{}
	}
	policiesJSON, err := json.Marshal(policies)
	if err != nil {
		return nil, err
	}

	return string(policiesJSON), nil
}

func deserializePolicies(policiesJSON sql.NullString) ([]model.Policy, error) {
	if !policiesJSON.Valid || policiesJSON.String == "" {
		return []model.Policy{}, nil
	}

	var policies []model.Policy
	if err := json.Unmarshal([]byte(policiesJSON.String), &policies); err != nil {
		return nil, err
	}

	return policies, nil
}

// Helper method to delete all API configurations (used in Update)
func (r *APIRepo) deleteAPIConfigurations(tx *sql.Tx, apiId string) error {
	// Delete in reverse order of dependencies
	queries := []string{
		`DELETE FROM operation_backend_services WHERE operation_id IN (SELECT id FROM api_operations WHERE api_uuid = ?)`,
		`DELETE FROM api_operations WHERE api_uuid = ?`,
		`DELETE FROM api_backend_services WHERE api_uuid = ?`, // Remove API-backend service associations
		`DELETE FROM api_rate_limiting WHERE api_uuid = ?`,
		`DELETE FROM api_cors_config WHERE api_uuid = ?`,
		`DELETE FROM oauth2_security WHERE api_uuid = ?`,
		`DELETE FROM api_key_security WHERE api_uuid = ?`,
		`DELETE FROM api_mtls_config WHERE api_uuid = ?`,
		`DELETE FROM xhub_signature_security WHERE api_uuid = ?`,
	}

	for _, query := range queries {
		if _, err := tx.Exec(r.db.Rebind(query), apiId); err != nil {
			return err
		}
	}

	return nil
}

// CreateDeploymentWithLimitEnforcement atomically creates a deployment with hard limit enforcement
// If deployment count >= hardLimit, deletes oldest 5 ARCHIVED deployments before inserting new one
// This entire operation is wrapped in a single transaction to ensure atomicity
// and to leverage row-level locks during deletion to reduce race conditions.
func (r *APIRepo) CreateDeploymentWithLimitEnforcement(deployment *model.APIDeployment, hardLimit int) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Generate UUID for deployment if not already set
	if deployment.DeploymentID == "" {
		deployment.DeploymentID = uuid.New().String()
	}
	deployment.CreatedAt = time.Now()

	// Status must be provided and should be DEPLOYED for new deployments
	if deployment.Status == nil {
		deployed := model.DeploymentStatusDeployed
		deployment.Status = &deployed
	}

	updatedAt := time.Now()
	deployment.UpdatedAt = &updatedAt

	// 1. Count total deployments for this API+Gateway
	var count int
	countQuery := `
		SELECT COUNT(*)
		FROM api_deployments
		WHERE api_uuid = ? AND gateway_uuid = ? AND organization_uuid = ?
	`
	err = tx.QueryRow(r.db.Rebind(countQuery), deployment.ApiID, deployment.GatewayID, deployment.OrganizationID).Scan(&count)
	if err != nil {
		return err
	}

	// 2. If at/over hard limit, delete oldest 5 ARCHIVED deployments
	if count >= hardLimit {
		// Get oldest 5 ARCHIVED deployment IDs (LEFT JOIN WHERE status IS NULL)
		getOldestQuery := `
			SELECT d.deployment_id
			FROM api_deployments d
			LEFT JOIN api_deployment_status s ON d.deployment_id = s.deployment_id
				AND d.api_uuid = s.api_uuid
				AND d.organization_uuid = s.organization_uuid
				AND d.gateway_uuid = s.gateway_uuid
			WHERE d.api_uuid = ? AND d.gateway_uuid = ? AND d.organization_uuid = ?
				AND s.deployment_id IS NULL
			ORDER BY d.created_at ASC
			LIMIT 5
		`

		rows, err := tx.Query(r.db.Rebind(getOldestQuery), deployment.ApiID, deployment.GatewayID, deployment.OrganizationID)
		if err != nil {
			return err
		}

		var idsToDelete []string
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return err
			}
			idsToDelete = append(idsToDelete, id)
		}
		rows.Close()

		// Check for iteration errors
		if err := rows.Err(); err != nil {
			return err
		}

		// Delete one-by-one to use row-level locks (prevents over-deletion in concurrent scenarios)
		deleteQuery := `DELETE FROM api_deployments WHERE deployment_id = ?`
		for _, id := range idsToDelete {
			_, err := tx.Exec(r.db.Rebind(deleteQuery), id)
			if err != nil {
				return err
			}
		}
	}

	// 3. Insert new deployment artifact
	deploymentQuery := `
		INSERT INTO api_deployments (deployment_id, name, api_uuid, organization_uuid, gateway_uuid, base_deployment_id, content, metadata, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	var baseDeploymentID interface{}
	if deployment.BaseDeploymentID != nil {
		baseDeploymentID = *deployment.BaseDeploymentID
	}

	var metadataJSON string
	if len(deployment.Metadata) > 0 {
		metadataBytes, err := json.Marshal(deployment.Metadata)
		if err != nil {
			return fmt.Errorf("failed to marshal deployment metadata: %w", err)
		}
		metadataJSON = string(metadataBytes)
	}

	_, err = tx.Exec(r.db.Rebind(deploymentQuery), deployment.DeploymentID, deployment.Name, deployment.ApiID, deployment.OrganizationID,
		deployment.GatewayID, baseDeploymentID, deployment.Content, metadataJSON, deployment.CreatedAt)
	if err != nil {
		return err
	}

	// 4. Insert or update deployment status (UPSERT)
	var statusQuery string
	if r.db.Driver() == "postgres" || r.db.Driver() == "postgresql" {
		statusQuery = `
			INSERT INTO api_deployment_status (api_uuid, organization_uuid, gateway_uuid, deployment_id, status, updated_at)
			VALUES (?, ?, ?, ?, ?, ?)
			ON CONFLICT (api_uuid, organization_uuid, gateway_uuid)
			DO UPDATE SET deployment_id = EXCLUDED.deployment_id, status = EXCLUDED.status, updated_at = EXCLUDED.updated_at
		`
	} else {
		statusQuery = `
			REPLACE INTO api_deployment_status (api_uuid, organization_uuid, gateway_uuid, deployment_id, status, updated_at)
			VALUES (?, ?, ?, ?, ?, ?)
		`
	}

	// Status and UpdatedAt are guaranteed to be non-nil by initialization at function start
	_, err = tx.Exec(r.db.Rebind(statusQuery),
		deployment.ApiID,
		deployment.OrganizationID,
		deployment.GatewayID,
		deployment.DeploymentID,
		*deployment.Status,
		*deployment.UpdatedAt,
	)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// GetDeploymentWithContent retrieves a deployment including its content (for rollback/base deployment scenarios)
func (r *APIRepo) GetDeploymentWithContent(deploymentID, apiID, orgID string) (*model.APIDeployment, error) {
	deployment := &model.APIDeployment{}

	query := `
		SELECT deployment_id, name, api_uuid, organization_uuid, gateway_uuid, base_deployment_id, content, metadata, created_at
		FROM api_deployments
		WHERE deployment_id = ? AND api_uuid = ? AND organization_uuid = ?
	`

	var baseDeploymentID sql.NullString
	var metadataJSON string

	err := r.db.QueryRow(r.db.Rebind(query), deploymentID, apiID, orgID).Scan(
		&deployment.DeploymentID, &deployment.Name, &deployment.ApiID, &deployment.OrganizationID,
		&deployment.GatewayID, &baseDeploymentID, &deployment.Content, &metadataJSON, &deployment.CreatedAt)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, constants.ErrDeploymentNotFound
		}
		return nil, err
	}

	if baseDeploymentID.Valid {
		deployment.BaseDeploymentID = &baseDeploymentID.String
	}

	if metadataJSON != "" {
		var metadata map[string]interface{}
		if err := json.Unmarshal([]byte(metadataJSON), &metadata); err == nil {
			deployment.Metadata = metadata
		} else {
			return nil, fmt.Errorf("failed to unmarshal deployment metadata: %w", err)
		}
	}

	return deployment, nil
}

// DeleteDeployment deletes a deployment record
func (r *APIRepo) DeleteDeployment(deploymentID, apiID, orgID string) error {
	query := `DELETE FROM api_deployments WHERE deployment_id = ? AND api_uuid = ? AND organization_uuid = ?`

	result, err := r.db.Exec(r.db.Rebind(query), deploymentID, apiID, orgID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return constants.ErrDeploymentNotFound
	}

	return nil
}

// GetCurrentDeploymentByGateway retrieves the currently DEPLOYED deployment for an API on a gateway
// Returns only deployments with DEPLOYED status (filters out UNDEPLOYED/suspended deployments)
func (r *APIRepo) GetCurrentDeploymentByGateway(apiUUID, gatewayID, orgID string) (*model.APIDeployment, error) {
	deployment := &model.APIDeployment{}

	query := `
		SELECT 
			d.deployment_id, d.name, d.api_uuid, d.organization_uuid, d.gateway_uuid, 
			d.base_deployment_id, d.content, d.metadata, d.created_at,
			s.status, s.updated_at AS status_updated_at
		FROM api_deployments d
		INNER JOIN api_deployment_status s 
			ON d.deployment_id = s.deployment_id
			AND d.api_uuid = s.api_uuid
			AND d.organization_uuid = s.organization_uuid
			AND d.gateway_uuid = s.gateway_uuid
		WHERE d.api_uuid = ? AND d.gateway_uuid = ? AND d.organization_uuid = ?
			AND s.status = ?
		ORDER BY d.created_at DESC
		LIMIT 1
	`

	var baseDeploymentID sql.NullString
	var metadataJSON string
	var statusStr string
	var updatedAt time.Time

	err := r.db.QueryRow(r.db.Rebind(query), apiUUID, gatewayID, orgID, string(model.DeploymentStatusDeployed)).Scan(
		&deployment.DeploymentID, &deployment.Name, &deployment.ApiID, &deployment.OrganizationID,
		&deployment.GatewayID, &baseDeploymentID, &deployment.Content, &metadataJSON, &deployment.CreatedAt,
		&statusStr, &updatedAt)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	if baseDeploymentID.Valid {
		deployment.BaseDeploymentID = &baseDeploymentID.String
	}

	if metadataJSON != "" {
		var metadata map[string]interface{}
		if err := json.Unmarshal([]byte(metadataJSON), &metadata); err == nil {
			deployment.Metadata = metadata
		} else {
			return nil, fmt.Errorf("failed to unmarshal deployment metadata: %w", err)
		}
	}

	// Populate status fields
	status := model.DeploymentStatus(statusStr)
	deployment.Status = &status
	deployment.UpdatedAt = &updatedAt

	return deployment, nil
}

// SetCurrentDeployment inserts or updates the deployment status record to set the current deployment for an API on a gateway
func (r *APIRepo) SetCurrentDeployment(apiUUID, orgUUID, gatewayID, deploymentID string, status model.DeploymentStatus) (time.Time, error) {
	updatedAt := time.Now()

	if r.db.Driver() == "postgres" || r.db.Driver() == "postgresql" {
		// PostgreSQL: Use ON CONFLICT
		query := `
			INSERT INTO api_deployment_status (api_uuid, organization_uuid, gateway_uuid, deployment_id, status, updated_at)
			VALUES (?, ?, ?, ?, ?, ?)
			ON CONFLICT (api_uuid, organization_uuid, gateway_uuid)
			DO UPDATE SET deployment_id = ?, status = ?, updated_at = ?
		`
		_, err := r.db.Exec(r.db.Rebind(query),
			apiUUID, orgUUID, gatewayID, deploymentID, status, updatedAt,
			deploymentID, status, updatedAt)
		return updatedAt, err
	} else {
		// SQLite: Use REPLACE
		query := `
			REPLACE INTO api_deployment_status (api_uuid, organization_uuid, gateway_uuid, deployment_id, status, updated_at)
			VALUES (?, ?, ?, ?, ?, ?)
		`
		_, err := r.db.Exec(r.db.Rebind(query),
			apiUUID, orgUUID, gatewayID, deploymentID, status, updatedAt)
		return updatedAt, err
	}
}

// GetDeploymentStatus retrieves the current deployment status for an API on a gateway (lightweight - no content)
func (r *APIRepo) GetDeploymentStatus(apiUUID, orgUUID, gatewayID string) (string, model.DeploymentStatus, *time.Time, error) {
	query := `
		SELECT deployment_id, status, updated_at
		FROM api_deployment_status
		WHERE api_uuid = ? AND organization_uuid = ? AND gateway_uuid = ?
	`

	var deploymentID string
	var statusStr string
	var updatedAt time.Time

	err := r.db.QueryRow(r.db.Rebind(query), apiUUID, orgUUID, gatewayID).Scan(
		&deploymentID, &statusStr, &updatedAt)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// No status row means no active deployment (all ARCHIVED)
			return "", "", nil, nil
		}
		return "", "", nil, err
	}

	return deploymentID, model.DeploymentStatus(statusStr), &updatedAt, nil
}

// DeleteDeploymentStatus deletes the status entry for an API on a gateway
func (r *APIRepo) DeleteDeploymentStatus(apiUUID, orgUUID, gatewayID string) error {
	query := `
		DELETE FROM api_deployment_status
		WHERE api_uuid = ? AND organization_uuid = ? AND gateway_uuid = ?
	`

	_, err := r.db.Exec(r.db.Rebind(query), apiUUID, orgUUID, gatewayID)
	return err
}

// GetDeploymentWithState retrieves a deployment with its lifecycle state populated (without content - lightweight)
func (r *APIRepo) GetDeploymentWithState(deploymentID, apiUUID, orgUUID string) (*model.APIDeployment, error) {
	deployment := &model.APIDeployment{}

	query := `
		SELECT 
			d.deployment_id, d.name, d.api_uuid, d.organization_uuid, d.gateway_uuid, 
			d.base_deployment_id, d.metadata, d.created_at,
			s.status, s.updated_at AS status_updated_at
		FROM api_deployments d
		LEFT JOIN api_deployment_status s 
			ON d.deployment_id = s.deployment_id
			AND d.api_uuid = s.api_uuid
			AND d.organization_uuid = s.organization_uuid
			AND d.gateway_uuid = s.gateway_uuid
		WHERE d.deployment_id = ? AND d.api_uuid = ? AND d.organization_uuid = ?
	`

	var baseDeploymentID sql.NullString
	var metadataJSON string
	var statusStr sql.NullString
	var updatedAtVal sql.NullTime

	err := r.db.QueryRow(r.db.Rebind(query), deploymentID, apiUUID, orgUUID).Scan(
		&deployment.DeploymentID, &deployment.Name, &deployment.ApiID, &deployment.OrganizationID, &deployment.GatewayID,
		&baseDeploymentID, &metadataJSON, &deployment.CreatedAt,
		&statusStr, &updatedAtVal)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, constants.ErrDeploymentNotFound
		}
		return nil, err
	}

	// Set nullable fields
	if baseDeploymentID.Valid {
		deployment.BaseDeploymentID = &baseDeploymentID.String
	}

	if metadataJSON != "" {
		var metadata map[string]interface{}
		if err := json.Unmarshal([]byte(metadataJSON), &metadata); err == nil {
			deployment.Metadata = metadata
		} else {
			return nil, fmt.Errorf("failed to unmarshal deployment metadata: %w", err)
		}
	}

	// Populate status fields from JOIN (nil if ARCHIVED)
	if statusStr.Valid {
		st := model.DeploymentStatus(statusStr.String)
		deployment.Status = &st
		if updatedAtVal.Valid {
			deployment.UpdatedAt = &updatedAtVal.Time
		}
	} else {
		// ARCHIVED state - Status and UpdatedAt remain nil
		archived := model.DeploymentStatusArchived
		deployment.Status = &archived
	}

	return deployment, nil
}

// GetDeploymentsWithState retrieves deployments with their lifecycle states.
// It enforces a soft limit of N records per Gateway, ensuring that the
// currently DEPLOYED or UNDEPLOYED record is always included regardless of its age.
func (r *APIRepo) GetDeploymentsWithState(apiUUID, orgUUID string, gatewayID *string, status *string, maxPerAPIGW int) ([]*model.APIDeployment, error) {

	// 1. Validation Logic
	if status != nil {
		validStatuses := map[string]bool{
			string(model.DeploymentStatusDeployed):   true,
			string(model.DeploymentStatusUndeployed): true,
			string(model.DeploymentStatusArchived):   true,
		}
		if !validStatuses[*status] {
			return nil, fmt.Errorf("invalid deployment status: %s", *status)
		}
	}

	var args []interface{}

	// 2. Build the CTE (Common Table Expression)
	// We rank within the CTE to ensure each Gateway gets its own "Top N" bucket.
	// Order Priority:
	//   1. Records with an active status (Deployed/Undeployed)
	//   2. Creation date (Newest first)
	query := `
        WITH AnnotatedDeployments AS (
            SELECT 
                d.deployment_id, d.name, d.api_uuid, d.organization_uuid, d.gateway_uuid,
                d.base_deployment_id, d.metadata, d.created_at,
                s.status as current_status,
                s.updated_at as status_updated_at,
                ROW_NUMBER() OVER (
                    PARTITION BY d.gateway_uuid 
                    ORDER BY 
                        (CASE WHEN s.status IS NOT NULL THEN 0 ELSE 1 END) ASC, 
                        d.created_at DESC
                ) as rank_idx
            FROM api_deployments d
            LEFT JOIN api_deployment_status s 
                ON d.deployment_id = s.deployment_id
                AND d.gateway_uuid = s.gateway_uuid
				AND d.api_uuid = s.api_uuid
				AND d.organization_uuid = s.organization_uuid
            WHERE d.api_uuid = ? AND d.organization_uuid = ?
    `
	args = append(args, apiUUID, orgUUID)

	if gatewayID != nil {
		query += " AND d.gateway_uuid = ?"
		args = append(args, *gatewayID)
	}

	// 3. Close CTE and start Outer Selection
	query += `
        )
        SELECT 
            deployment_id, name, api_uuid, organization_uuid, gateway_uuid,
            base_deployment_id, metadata, created_at,
            current_status, status_updated_at
        FROM AnnotatedDeployments
        WHERE rank_idx <= ?
    `
	args = append(args, maxPerAPIGW)

	// 4. Apply Status Filters on the Ranked Set
	if status != nil {
		if *status == string(model.DeploymentStatusArchived) {
			// ARCHIVED means no entry exists in the status table for this artifact
			query += " AND current_status IS NULL"
		} else {
			// DEPLOYED or UNDEPLOYED must match the status column exactly
			query += " AND current_status = ?"
			args = append(args, *status)
		}
	}

	// Final sorting for the application layer
	query += " ORDER BY gateway_uuid ASC, rank_idx ASC"

	// 5. Execution
	rows, err := r.db.Query(r.db.Rebind(query), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var deployments []*model.APIDeployment
	for rows.Next() {
		deployment := &model.APIDeployment{}
		var baseDeploymentID sql.NullString
		var metadataJSON string
		var statusStr sql.NullString
		var updatedAtVal sql.NullTime

		err := rows.Scan(
			&deployment.DeploymentID, &deployment.Name, &deployment.ApiID,
			&deployment.OrganizationID, &deployment.GatewayID,
			&baseDeploymentID, &metadataJSON, &deployment.CreatedAt,
			&statusStr, &updatedAtVal)

		if err != nil {
			return nil, err
		}

		// Handle Nullable BaseDeploymentID
		if baseDeploymentID.Valid {
			deployment.BaseDeploymentID = &baseDeploymentID.String
		}

		// Handle Metadata
		if metadataJSON != "" {
			var metadata map[string]interface{}
			if err := json.Unmarshal([]byte(metadataJSON), &metadata); err == nil {
				deployment.Metadata = metadata
			} else {
				return nil, fmt.Errorf("failed to unmarshal deployment metadata: %w", err)
			}
		}

		// Map Database Status to Model Status
		if statusStr.Valid {
			st := model.DeploymentStatus(statusStr.String)
			deployment.Status = &st
			if updatedAtVal.Valid {
				deployment.UpdatedAt = &updatedAtVal.Time
			}
		} else {
			// If the JOIN resulted in NULL, the record is ARCHIVED
			archived := model.DeploymentStatusArchived
			deployment.Status = &archived
			// For Archived, UpdatedAt usually defaults to nil
		}

		deployments = append(deployments, deployment)
	}

	// Check if the loop stopped because of an error rather than reaching the end
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error during deployment rows iteration: %w", err)
	}

	return deployments, nil
}

// CreateAPIAssociation creates an association between an API and resource (e.g., gateway or dev portal)
func (r *APIRepo) CreateAPIAssociation(association *model.APIAssociation) error {
	if r.db.Driver() == "postgres" || r.db.Driver() == "postgresql" {
		// PostgreSQL: use RETURNING to get the generated ID
		query := `
			INSERT INTO api_associations (api_uuid, organization_uuid, resource_uuid, association_type, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?)
			RETURNING id
		`
		if err := r.db.QueryRow(r.db.Rebind(query), association.ApiID, association.OrganizationID, association.ResourceID,
			association.AssociationType, association.CreatedAt, association.UpdatedAt).Scan(&association.ID); err != nil {
			return err
		}
	} else {
		// SQLite: use LastInsertId
		query := `
			INSERT INTO api_associations (api_uuid, organization_uuid, resource_uuid, association_type, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?)
		`
		result, err := r.db.Exec(r.db.Rebind(query), association.ApiID, association.OrganizationID, association.ResourceID,
			association.AssociationType, association.CreatedAt, association.UpdatedAt)
		if err != nil {
			return err
		}

		lastID, err := result.LastInsertId()
		if err != nil {
			return err
		}
		association.ID = lastID
	}

	return nil
}

// UpdateAPIAssociation updates the updated_at timestamp for an existing API resource association
func (r *APIRepo) UpdateAPIAssociation(apiUUID, resourceId, associationType, orgUUID string) error {
	query := `
		UPDATE api_associations 
		SET updated_at = ?
		WHERE api_uuid = ? AND resource_uuid = ? AND association_type = ? AND organization_uuid = ?
	`
	_, err := r.db.Exec(r.db.Rebind(query), time.Now(), apiUUID, resourceId, associationType, orgUUID)
	return err
}

// GetAPIAssociations retrieves all resource associations for an API of a specific type
func (r *APIRepo) GetAPIAssociations(apiUUID, associationType, orgUUID string) ([]*model.APIAssociation, error) {
	query := `
		SELECT id, api_uuid, organization_uuid, resource_uuid, association_type, created_at, updated_at
		FROM api_associations
		WHERE api_uuid = ? AND association_type = ? AND organization_uuid = ?
	`
	rows, err := r.db.Query(r.db.Rebind(query), apiUUID, associationType, orgUUID)
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
func (r *APIRepo) GetAPIGatewaysWithDetails(apiUUID, orgUUID string) ([]*model.APIGatewayWithDetails, error) {
	query := `
		SELECT 
			g.uuid as id,
			g.organization_uuid as organization_id,
			g.name,
			g.display_name,
			g.description,
			g.properties,
			g.vhost,
			g.is_critical,
			g.gateway_functionality_type as functionality_type,
			g.is_active,
			g.created_at,
			g.updated_at,
			aa.created_at as associated_at,
			aa.updated_at as association_updated_at,
			CASE WHEN ad.deployment_id IS NOT NULL THEN 1 ELSE 0 END as is_deployed,
			ad.deployment_id,
			ad.updated_at as deployed_at
		FROM gateways g
		INNER JOIN api_associations aa ON g.uuid = aa.resource_uuid AND aa.association_type = 'gateway'
		LEFT JOIN api_deployment_status ad ON g.uuid = ad.gateway_uuid AND ad.api_uuid = ? AND ad.status = ?
		WHERE aa.api_uuid = ? AND g.organization_uuid = ?
		ORDER BY aa.created_at DESC
	`

	rows, err := r.db.Query(r.db.Rebind(query), apiUUID, string(model.DeploymentStatusDeployed), apiUUID, orgUUID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var gateways []*model.APIGatewayWithDetails
	for rows.Next() {
		gateway := &model.APIGatewayWithDetails{}
		var propertiesJSON string
		var deployedAt sql.NullTime
		var deploymentId sql.NullString

		err := rows.Scan(
			&gateway.ID,
			&gateway.OrganizationID,
			&gateway.Name,
			&gateway.DisplayName,
			&gateway.Description,
			&propertiesJSON,
			&gateway.Vhost,
			&gateway.IsCritical,
			&gateway.FunctionalityType,
			&gateway.IsActive,
			&gateway.CreatedAt,
			&gateway.UpdatedAt,
			&gateway.AssociatedAt,
			&gateway.AssociationUpdatedAt,
			&gateway.IsDeployed,
			&deploymentId,
			&deployedAt,
		)
		if err != nil {
			return nil, err
		}

		if propertiesJSON != "" && propertiesJSON != "{}" {
			if err := json.Unmarshal([]byte(propertiesJSON), &gateway.Properties); err != nil {
				return nil, fmt.Errorf("failed to unmarshal gateway properties: %w", err)
			}
		}

		if deploymentId.Valid {
			gateway.DeploymentID = &deploymentId.String
		}
		if deployedAt.Valid {
			gateway.DeployedAt = &deployedAt.Time
		}
		gateways = append(gateways, gateway)
	}

	return gateways, rows.Err()
}

// CheckAPIExistsByNameAndVersionInOrganization checks if an API with the given name and version exists within a specific organization
// excludeHandle: if provided, excludes the API with this handle from the check (useful for updates)
func (r *APIRepo) CheckAPIExistsByNameAndVersionInOrganization(name, version, orgUUID, excludeHandle string) (bool, error) {
	var query string
	var args []interface{}

	if excludeHandle != "" {
		query = `
			SELECT COUNT(*) FROM apis
			WHERE name = ? AND version = ? AND organization_uuid = ? AND handle != ?
		`
		args = []interface{}{name, version, orgUUID, excludeHandle}
	} else {
		query = `
			SELECT COUNT(*) FROM apis
			WHERE name = ? AND version = ? AND organization_uuid = ?
		`
		args = []interface{}{name, version, orgUUID}
	}

	var count int
	err := r.db.QueryRow(r.db.Rebind(query), args...).Scan(&count)
	if err != nil {
		return false, err
	}

	return count > 0, nil
}
