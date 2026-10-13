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
		cfg.SDL, cfg.IntrospectionMode, _ = i.resolveSchema(cfg.Upstream.Main)
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
		// the configuration, the same as at create time — but unlike create, a
		// failed resolution here keeps the previously-stored schema instead of
		// blanking it out (mirrors GraphQLAPIService.Update's same posture): a
		// transient upstream issue during a metadata-only re-import must not
		// destroy a schema that was working before this push.
		if sdl, mode, ok := i.resolveSchema(cfg.Upstream.Main); ok {
			cfg.SDL, cfg.IntrospectionMode = sdl, mode
		} else {
			cfg.SDL, cfg.IntrospectionMode = existing.Configuration.SDL, existing.Configuration.IntrospectionMode
		}
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

// resolveSchema derives SDL/introspectionMode via the same introspection path
// CP-native create/update uses (fetchAndConvertGraphQLSchema, graphql_introspection.go),
// since the gateway-pushed spec never carries the schema. Best-effort, mirroring
// mcpProxyImporter.fetchCapabilities: an unreachable or misbehaving upstream must
// not fail the whole import — ok reports whether resolution actually succeeded,
// so a caller updating an existing artifact can keep its previously-stored
// schema instead of blanking it out, the same way GraphQLAPIService.Update does.
func (i *graphqlAPIImporter) resolveSchema(upstreamMain *model.UpstreamEndpoint) (sdl, introspectionMode string, ok bool) {
	if upstreamMain == nil || upstreamMain.URL == "" {
		return "", "", false
	}
	derived, err := fetchAndConvertGraphQLSchema(upstreamMain.URL)
	if err != nil {
		return "", "", false
	}
	return derived, "ENDPOINT", true
}
