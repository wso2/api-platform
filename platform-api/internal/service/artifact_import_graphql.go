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
	"fmt"

	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/repository"
	"github.com/wso2/api-platform/platform-api/internal/utils"
)

// graphqlAPIImporter imports GraphQL API artifacts (project-scoped).
type graphqlAPIImporter struct {
	graphqlAPIRepo repository.GraphQLAPIRepository
	artifactRepo   repository.ArtifactRepository
}

func newGraphQLAPIImporter(graphqlAPIRepo repository.GraphQLAPIRepository, artifactRepo repository.ArtifactRepository) *graphqlAPIImporter {
	return &graphqlAPIImporter{graphqlAPIRepo: graphqlAPIRepo, artifactRepo: artifactRepo}
}

func (i *graphqlAPIImporter) Kind() string          { return constants.GraphQLApi }
func (i *graphqlAPIImporter) RequiresProject() bool { return true }

func (i *graphqlAPIImporter) Import(ctx *ImportContext) (*ImportResult, error) {
	version := utils.ImportVersion(ctx.Configuration)

	// The gateway pushes the artifact spec in the same shape the control plane emits
	// when generating a deployment (context + upstream only — see
	// generateGraphQLAPIDeploymentYAML in graphql_deployment.go). It never carries the
	// schema, so SDL/introspectionMode come back empty from the decode and are
	// resolved separately below, mirroring mcpProxyImporter's out-of-band capability
	// fetch.
	var cfg model.GraphQLAPIConfig
	if err := utils.DecodeSpec(ctx.Configuration.Spec, &cfg); err != nil {
		return nil, err
	}

	if ctx.Existing == nil {
		i.resolveSchema(&cfg)
		projectID := ctx.ProjectID
		graphqlAPI := &model.GraphQLAPI{
			ID:             ctx.ID,
			Handle:         utils.ImportHandle(ctx.Configuration),
			Name:           utils.ImportDisplayName(ctx.Configuration),
			Kind:           constants.GraphQLApi,
			Version:        version,
			ProjectID:      projectID,
			OrganizationID: ctx.OrgID,
			Origin:         constants.OriginDP,
			Configuration:  cfg,
		}
		if err := i.graphqlAPIRepo.Create(graphqlAPI); err != nil {
			return nil, fmt.Errorf("failed to create GraphQL API from gateway import: %w", err)
		}
		return &ImportResult{ID: graphqlAPI.ID, DeployedVersion: version, Deployable: true}, nil
	}

	existing, err := i.graphqlAPIRepo.GetByUUID(ctx.ID, ctx.OrgID)
	if err != nil {
		return nil, fmt.Errorf("failed to load existing GraphQL API: %w", err)
	}
	if existing == nil {
		return &ImportResult{ID: ctx.ID, DeployedVersion: version, Deployable: true}, nil
	}

	switch ctx.MetadataMode {
	case utils.SkipWorkingCopy:
		// Stale, out-of-order push: a newer deployment already defines the working copy.
		return &ImportResult{ID: ctx.ID, DeployedVersion: version, Deployable: true}, nil
	case utils.WriteFullMetadata:
		existing.Name = utils.ImportDisplayName(ctx.Configuration)
		existing.Version = version
		existing.ProjectID = ctx.ProjectID
		// Refresh the schema from the (possibly new) upstream alongside the rest of
		// the configuration, the same as at create time.
		i.resolveSchema(&cfg)
		existing.Configuration = cfg
	case utils.WriteGatewaySpecificOnly:
		// CP-owned: only the upstream is gateway-specific data; SDL/name/etc. are not
		// touched.
		existing.Configuration.Upstream = cfg.Upstream
	}
	if err := i.graphqlAPIRepo.Update(existing); err != nil {
		return nil, fmt.Errorf("failed to update GraphQL API from gateway import: %w", err)
	}
	return &ImportResult{ID: ctx.ID, DeployedVersion: version, Deployable: true}, nil
}

// resolveSchema derives cfg.SDL/IntrospectionMode via the same introspection path
// CP-native create uses (fetchAndConvertGraphQLSchema, graphql_introspection.go),
// since the gateway-pushed spec never carries the schema. Best-effort, mirroring
// mcpProxyImporter.fetchCapabilities: an unreachable or misbehaving upstream must
// not fail the whole import, so a failure just leaves SDL empty rather than
// surfacing the specific reason (matches GraphQLAPIService.resolveSchema's own
// sterile-failure posture).
func (i *graphqlAPIImporter) resolveSchema(cfg *model.GraphQLAPIConfig) {
	if cfg.Upstream.Main == nil || cfg.Upstream.Main.URL == "" {
		return
	}
	derived, err := fetchAndConvertGraphQLSchema(cfg.Upstream.Main.URL)
	if err != nil {
		return
	}
	cfg.SDL = derived
	cfg.IntrospectionMode = "ENDPOINT"
}
