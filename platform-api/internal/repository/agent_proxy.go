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

package repository

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/database"
	"github.com/wso2/api-platform/platform-api/internal/gatewaytranslator"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/utils"
)

// agentProxyColumns is the projection every Agent proxy read path uses. It names
// protocol explicitly: the column is the sole discriminator, so a projection that
// omitted it would leave model.AgentProxy.Protocol empty and force callers to
// guess the variant from a JSON key.
const agentProxyColumns = `
	uuid, organization_uuid, project_uuid, handle, display_name, version, protocol,
	description, configuration, origin, data_version,
	created_by, created_at, updated_by, updated_at`

// ErrAgentProxyProtocolMismatch reports that a row's protocol column and its
// stored configuration document disagree — an internal consistency error, never
// something to resolve by inferring the protocol from the document.
var ErrAgentProxyProtocolMismatch = errors.New("agent proxy protocol does not match its stored configuration")

// ErrAgentProxyProtocolImmutable reports an attempt to change an Agent proxy's
// protocol, which is fixed at creation.
var ErrAgentProxyProtocolImmutable = errors.New("agent proxy protocol is immutable")

// ErrAgentProxyProjectOrgMismatch reports that the referenced project belongs to a
// different organization. The independent project_uuid and organization_uuid
// foreign keys each hold on their own, so only this check enforces that the two
// agree.
var ErrAgentProxyProjectOrgMismatch = errors.New("project does not belong to the organization")

// AgentProxyListOptions carries the org-scoped list and count inputs. Protocol is
// optional; empty means "every protocol". Both the list and the count query build
// their predicate from the same helper so a filtered page can never disagree with
// the total it is paginated against.
type AgentProxyListOptions struct {
	Limit    int
	Offset   int
	Protocol model.AgentProxyProtocol
}

// AgentProxyRepo handles database operations for Agent proxies.
type AgentProxyRepo struct {
	db           *database.DB
	artifactRepo *ArtifactRepo
}

// NewAgentProxyRepo creates a new AgentProxyRepo instance.
func NewAgentProxyRepo(db *database.DB) *AgentProxyRepo {
	return &AgentProxyRepo{db: db, artifactRepo: NewArtifactRepo(db)}
}

// Create inserts a new Agent proxy and its parent artifact row in one transaction.
func (r *AgentProxyRepo) Create(p *model.AgentProxy) error {
	if err := validateAgentProxyProtocolConfiguration(p); err != nil {
		return err
	}

	uuidStr, err := utils.GenerateUUID()
	if err != nil {
		return fmt.Errorf("failed to generate agent proxy ID: %w", err)
	}
	p.UUID = uuidStr
	now := time.Now().UTC()
	p.CreatedAt = now
	p.UpdatedAt = now

	configurationJSON, err := serializeAgentProxyConfiguration(p.Configuration)
	if err != nil {
		return fmt.Errorf("failed to serialize configuration: %w", err)
	}

	if p.Origin == "" {
		p.Origin = constants.OriginCP
	}
	if p.DataVersion == "" {
		p.DataVersion = string(gatewaytranslator.ComputeDataVersion(constants.AgentProxy, constants.GatewayApiVersion))
	}

	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck

	if err := r.assertProjectInOrg(tx, p.ProjectUUID, p.OrganizationUUID); err != nil {
		return err
	}

	// The artifact row carries the control-plane kind. The gateway's own kind
	// stays Agent and is only produced when a deployment artifact is built.
	if err := r.artifactRepo.Create(tx, &model.Artifact{
		UUID:             p.UUID,
		Type:             constants.AgentProxy,
		OrganizationUUID: p.OrganizationUUID,
	}); err != nil {
		return fmt.Errorf("failed to create artifact: %w", err)
	}

	query := `
		INSERT INTO agent_proxies (
			uuid, organization_uuid, project_uuid, handle, display_name, version, protocol,
			description, configuration, origin, data_version,
			created_by, created_at, updated_by, updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	if _, err := tx.Exec(r.db.Rebind(query),
		p.UUID, p.OrganizationUUID, p.ProjectUUID, p.Handle, p.Name, p.Version, string(p.Protocol),
		p.Description, configurationJSON, p.Origin, p.DataVersion,
		p.CreatedBy, p.CreatedAt, p.UpdatedBy, p.UpdatedAt,
	); err != nil {
		return fmt.Errorf("failed to create agent proxy: %w", err)
	}

	// Upstream auth is a {{ secret "handle" }} placeholder, so the artifact's
	// secret references have to be recorded here — otherwise artifact_secret_refs
	// is silently incomplete for this kind and a secret an Agent proxy depends on
	// looks unreferenced at deletion time.
	if err := upsertArtifactSecretRefs(tx, r.db, p.OrganizationUUID, p.UUID, configurationJSON); err != nil {
		return fmt.Errorf("failed to upsert artifact secret refs: %w", err)
	}

	if err := insertArtifactGatewayAssociations(tx, r.db, p.UUID, p.OrganizationUUID, p.CreatedBy, p.AssociatedGateways, now); err != nil {
		return err
	}

	return tx.Commit()
}

// GetByHandle retrieves an Agent proxy by its public handle within an organization.
// A missing row is (nil, nil), matching the other artifact repositories.
func (r *AgentProxyRepo) GetByHandle(handle, orgUUID string) (*model.AgentProxy, error) {
	query := `SELECT` + agentProxyColumns + `
		FROM agent_proxies
		WHERE handle = ? AND organization_uuid = ?`
	p, err := scanAgentProxyRow(r.db.QueryRow(r.db.Rebind(query), handle, orgUUID))
	if err != nil || p == nil {
		return nil, err
	}

	associations, err := loadArtifactGatewayAssociations(r.db, p.UUID, orgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to load gateway associations for agent proxy %s: %w", p.Handle, err)
	}
	p.AssociatedGateways = associations
	return p, nil
}

// GetByUUID retrieves an Agent proxy by its artifact UUID within an organization.
func (r *AgentProxyRepo) GetByUUID(uuid, orgUUID string) (*model.AgentProxy, error) {
	query := `SELECT` + agentProxyColumns + `
		FROM agent_proxies
		WHERE uuid = ? AND organization_uuid = ?`
	return scanAgentProxyRow(r.db.QueryRow(r.db.Rebind(query), uuid, orgUUID))
}

// List retrieves the Agent proxies of an organization, optionally restricted to
// one protocol.
func (r *AgentProxyRepo) List(orgUUID string, opts AgentProxyListOptions) ([]*model.AgentProxy, error) {
	filter, filterArgs := agentProxyProtocolPredicate(opts.Protocol)
	pageClause, pageArgs := r.db.PaginationClause(opts.Limit, opts.Offset)
	query := `SELECT` + agentProxyColumns + `
		FROM agent_proxies
		WHERE organization_uuid = ?` + filter + `
		ORDER BY created_at DESC
		` + pageClause

	args := append([]any{orgUUID}, filterArgs...)
	args = append(args, pageArgs...)
	return r.queryAgentProxies(query, args...)
}

// Count returns the number of Agent proxies in an organization under the same
// filter List applies.
func (r *AgentProxyRepo) Count(orgUUID string, opts AgentProxyListOptions) (int, error) {
	filter, filterArgs := agentProxyProtocolPredicate(opts.Protocol)
	query := `SELECT COUNT(*) FROM agent_proxies WHERE organization_uuid = ?` + filter

	var count int
	args := append([]any{orgUUID}, filterArgs...)
	if err := r.db.QueryRow(r.db.Rebind(query), args...).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

// ListByProject retrieves every Agent proxy in one project.
func (r *AgentProxyRepo) ListByProject(orgUUID, projectUUID string) ([]*model.AgentProxy, error) {
	query := `SELECT` + agentProxyColumns + `
		FROM agent_proxies
		WHERE organization_uuid = ? AND project_uuid = ?
		ORDER BY created_at DESC`
	return r.queryAgentProxies(query, orgUUID, projectUUID)
}

// CountByProject returns the number of Agent proxies in one project. Project
// deletion is guarded on this count.
func (r *AgentProxyRepo) CountByProject(orgUUID, projectUUID string) (int, error) {
	var count int
	query := `SELECT COUNT(*) FROM agent_proxies WHERE organization_uuid = ? AND project_uuid = ?`
	if err := r.db.QueryRow(r.db.Rebind(query), orgUUID, projectUUID).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

// Update replaces an Agent proxy's writable columns and its configuration
// document. The protocol column is never written: it is loaded, compared, and the
// update is refused if the caller is trying to change it.
func (r *AgentProxyRepo) Update(p *model.AgentProxy) error {
	if err := validateAgentProxyProtocolConfiguration(p); err != nil {
		return err
	}

	now := time.Now().UTC()
	p.UpdatedAt = now

	configurationJSON, err := serializeAgentProxyConfiguration(p.Configuration)
	if err != nil {
		return fmt.Errorf("failed to serialize configuration: %w", err)
	}

	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck

	// Read the persisted protocol and data version up front. An edit that does not
	// carry DataVersion forward keeps the stored value rather than recomputing it.
	var proxyUUID, storedProtocol, existingDataVersion string
	lookup := `
		SELECT uuid, protocol, data_version FROM agent_proxies
		WHERE handle = ? AND organization_uuid = ?`
	if err := tx.QueryRow(r.db.Rebind(lookup), p.Handle, p.OrganizationUUID).
		Scan(&proxyUUID, &storedProtocol, &existingDataVersion); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return sql.ErrNoRows
		}
		return err
	}

	if string(p.Protocol) != storedProtocol {
		return fmt.Errorf("%w: stored %q, requested %q", ErrAgentProxyProtocolImmutable, storedProtocol, p.Protocol)
	}

	// project_uuid is not rewritten here — an artifact stays in the project it was
	// created in — but a caller-supplied project must still belong to this
	// organization, so a cross-tenant project handle is refused rather than ignored.
	if p.ProjectUUID != "" {
		if err := r.assertProjectInOrg(tx, p.ProjectUUID, p.OrganizationUUID); err != nil {
			return err
		}
	}

	if p.DataVersion == "" {
		if existingDataVersion != "" {
			p.DataVersion = existingDataVersion
		} else {
			p.DataVersion = string(gatewaytranslator.ComputeDataVersion(constants.AgentProxy, constants.GatewayApiVersion))
		}
	}

	update := `
		UPDATE agent_proxies
		SET display_name = ?, version = ?, description = ?, configuration = ?,
			updated_by = ?, data_version = ?, updated_at = ?
		WHERE uuid = ?`
	result, err := tx.Exec(r.db.Rebind(update),
		p.Name, p.Version, p.Description, configurationJSON,
		p.UpdatedBy, p.DataVersion, now,
		proxyUUID,
	)
	if err != nil {
		return fmt.Errorf("failed to update agent proxy: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return sql.ErrNoRows
	}

	if err := upsertArtifactSecretRefs(tx, r.db, p.OrganizationUUID, proxyUUID, configurationJSON); err != nil {
		return fmt.Errorf("failed to upsert artifact secret refs: %w", err)
	}

	if p.ReplaceAssociatedGateways {
		if err := replaceArtifactGatewayAssociations(tx, r.db, proxyUUID, p.OrganizationUUID, p.UpdatedBy, p.AssociatedGateways, now); err != nil {
			return err
		}
	}

	p.UUID = proxyUUID
	return tx.Commit()
}

// Delete removes an Agent proxy and its parent artifact row in one transaction.
func (r *AgentProxyRepo) Delete(handle, orgUUID string) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck

	var proxyUUID string
	lookup := `SELECT uuid FROM agent_proxies WHERE handle = ? AND organization_uuid = ?`
	if err := tx.QueryRow(r.db.Rebind(lookup), handle, orgUUID).Scan(&proxyUUID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return sql.ErrNoRows
		}
		return err
	}

	if _, err := tx.Exec(r.db.Rebind(`DELETE FROM agent_proxies WHERE uuid = ?`), proxyUUID); err != nil {
		return err
	}
	if err := r.artifactRepo.Delete(tx, proxyUUID); err != nil {
		return err
	}
	return tx.Commit()
}

// Exists reports whether an Agent proxy with this handle exists in the organization.
func (r *AgentProxyRepo) Exists(handle, orgUUID string) (bool, error) {
	return r.artifactRepo.Exists(constants.AgentProxy, handle, orgUUID)
}

// EnsureGatewayAssociation creates a gateway association for the Agent proxy if
// one does not already exist and resolves the metadata to use for the deployment.
// See ensureArtifactGatewayAssociation for the full semantics.
func (r *AgentProxyRepo) EnsureGatewayAssociation(proxyUUID, gatewayUUID, orgUUID, createdBy, deployMetadata string, metadataProvided bool) (string, error) {
	return ensureArtifactGatewayAssociation(r.db, proxyUUID, gatewayUUID, orgUUID, createdBy, deployMetadata, metadataProvided)
}

// assertProjectInOrg refuses a project owned by another organization. Both FKs are
// satisfied independently by a cross-tenant pair, so nothing in the schema catches it.
func (r *AgentProxyRepo) assertProjectInOrg(tx *sql.Tx, projectUUID, orgUUID string) error {
	var owner string
	query := `SELECT organization_uuid FROM projects WHERE uuid = ?`
	if err := tx.QueryRow(r.db.Rebind(query), projectUUID).Scan(&owner); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("%w: project %s not found", ErrAgentProxyProjectOrgMismatch, projectUUID)
		}
		return err
	}
	if owner != orgUUID {
		return ErrAgentProxyProjectOrgMismatch
	}
	return nil
}

func (r *AgentProxyRepo) queryAgentProxies(query string, args ...any) ([]*model.AgentProxy, error) {
	rows, err := r.db.Query(r.db.Rebind(query), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []*model.AgentProxy
	for rows.Next() {
		p, err := scanAgentProxy(rows)
		if err != nil {
			return nil, err
		}
		res = append(res, p)
	}
	return res, rows.Err()
}

// agentProxyProtocolPredicate builds the optional protocol filter shared by the
// list and count queries, as a bound parameter rather than interpolated text.
func agentProxyProtocolPredicate(protocol model.AgentProxyProtocol) (string, []any) {
	if protocol == "" {
		return "", nil
	}
	return ` AND protocol = ?`, []any{string(protocol)}
}

// scanAgentProxyRow scans a single-row query, translating "no rows" into (nil, nil).
func scanAgentProxyRow(row *sql.Row) (*model.AgentProxy, error) {
	p, err := scanAgentProxy(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return p, nil
}

// scanAgentProxy reads one agent_proxies row in agentProxyColumns order.
func scanAgentProxy(row rowScanner) (*model.AgentProxy, error) {
	var p model.AgentProxy
	var protocol string
	var description, createdBy, updatedBy sql.NullString
	var configurationJSON []byte

	if err := row.Scan(
		&p.UUID, &p.OrganizationUUID, &p.ProjectUUID, &p.Handle, &p.Name, &p.Version, &protocol,
		&description, &configurationJSON, &p.Origin, &p.DataVersion,
		&createdBy, &p.CreatedAt, &updatedBy, &p.UpdatedAt,
	); err != nil {
		return nil, err
	}

	p.Protocol = model.AgentProxyProtocol(protocol)
	p.Description = description.String
	p.CreatedBy = createdBy.String
	p.UpdatedBy = updatedBy.String

	config, err := deserializeAgentProxyConfiguration(configurationJSON)
	if err != nil {
		return nil, fmt.Errorf("unmarshal configuration for agent proxy %s: %w", p.Handle, err)
	}
	p.Configuration = *config

	if err := validateAgentProxyProtocolConfiguration(&p); err != nil {
		return nil, fmt.Errorf("agent proxy %s: %w", p.Handle, err)
	}
	return &p, nil
}

// validateAgentProxyProtocolConfiguration asserts that the protocol value selects
// exactly the configuration variant that is present. It runs on every write and
// on every read, so a row whose column and document have drifted apart surfaces
// as an error instead of quietly deploying as something else.
func validateAgentProxyProtocolConfiguration(p *model.AgentProxy) error {
	if p == nil {
		return fmt.Errorf("agent proxy is nil")
	}
	if p.Protocol == "" {
		return fmt.Errorf("%w: protocol is empty", ErrAgentProxyProtocolMismatch)
	}
	if !model.IsSupportedAgentProxyProtocol(p.Protocol) {
		return fmt.Errorf("%w: unsupported protocol %q", ErrAgentProxyProtocolMismatch, p.Protocol)
	}
	if p.Protocol == model.AgentProxyProtocolA2A && p.Configuration.A2A == nil {
		return fmt.Errorf("%w: protocol %q without an a2a configuration block", ErrAgentProxyProtocolMismatch, p.Protocol)
	}
	return nil
}

func serializeAgentProxyConfiguration(config model.AgentProxyConfiguration) ([]byte, error) {
	return json.Marshal(config)
}

// agentProxyConfigurationEnvelope captures a stray protocol key alongside the
// configuration document. The discriminator lives in its own column, so a
// document that also carries one is a consistency error rather than a fallback
// source to read it from.
//
// Protocol is a json.RawMessage rather than a *string because the rule is about
// the key existing at all, not about it decoding to something useful: a
// RawMessage field is left nil when the key is absent and is assigned the literal
// bytes when it is present — including "protocol": null, which a *string would
// decode to nil and wave through.
type agentProxyConfigurationEnvelope struct {
	model.AgentProxyConfiguration
	Protocol json.RawMessage `json:"protocol,omitempty"`
}

func deserializeAgentProxyConfiguration(configJSON []byte) (*model.AgentProxyConfiguration, error) {
	if len(configJSON) == 0 {
		return nil, fmt.Errorf("null configuration")
	}
	var envelope agentProxyConfigurationEnvelope
	if err := json.Unmarshal(configJSON, &envelope); err != nil {
		return nil, err
	}
	if envelope.Protocol != nil {
		return nil, fmt.Errorf("%w: configuration document carries a protocol discriminator", ErrAgentProxyProtocolMismatch)
	}
	config := envelope.AgentProxyConfiguration
	return &config, nil
}
