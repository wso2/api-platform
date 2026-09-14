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
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/config"
	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/repository"
	"github.com/wso2/api-platform/platform-api/internal/utils"
)

// BuildService stores and serves builds for EVERY artifact kind.
//
// Builds hang off artifact_uuid rather than any kind's own table, so everything
// about storing, listing, limiting and removing them is already common. The one
// part that is not — turning an artifact into the definition a build holds — is
// reached through ArtifactDefinition, so a new kind becomes a definition rather
// than another copy of this file.
//
// Each kind keeps its own REST path (/rest-apis/…/builds, /mcp-proxies/…/builds
// and so on). Only the implementation is shared.
type BuildService struct {
	artifactRepo   repository.ArtifactRepository
	deploymentRepo repository.DeploymentRepository
	definitions    ArtifactDefinitions
	cfg            *config.Server
	slogger        *slog.Logger
}

// NewBuildService creates the shared build service.
func NewBuildService(
	artifactRepo repository.ArtifactRepository,
	deploymentRepo repository.DeploymentRepository,
	definitions ArtifactDefinitions,
	cfg *config.Server,
	slogger *slog.Logger,
) *BuildService {
	return &BuildService{
		artifactRepo:   artifactRepo,
		deploymentRepo: deploymentRepo,
		definitions:    definitions,
		cfg:            cfg,
		slogger:        slogger,
	}
}

// resolve finds the artifact and the definition that can render its kind.
//
// The artifact row carries the kind (as the registry's alias, which is the key
// ArtifactDefinitions is indexed by), so one lookup answers both "does this exist"
// and "how is it rendered".
func (s *BuildService) resolve(artifactUUID, orgUUID string) (*model.Artifact, ArtifactDefinition, error) {
	artifact, err := s.artifactRepo.GetByUUID(artifactUUID, orgUUID)
	if err != nil {
		return nil, nil, err
	}
	if artifact == nil {
		return nil, nil, apperror.ArtifactNotFound.New()
	}
	definition, err := s.definitions.For(artifact.Type)
	if err != nil {
		return nil, nil, err
	}
	return artifact, definition, nil
}

// Render turns an artifact's current definition into a build that has not been
// stored yet, and hands back the struct it was rendered from alongside it.
//
// The struct is returned so a deploy can apply its own overrides and translate for
// the target gateway without re-parsing what it has just written — and so those
// overrides never reach the build, whose content is marshalled HERE, before any
// caller sees the struct. A build is the definition as it stood, not one
// deployment's customization of it.
//
// Storing is the caller's to do: preparing a build stores it alone, while a deploy
// from `current` stores it on the transaction that records the deployment, so the
// two commit together.
func (s *BuildService) Render(artifactUUID, orgUUID, createdBy string,
	metadata map[string]interface{}) (*model.Build, any, error) {

	artifact, definition, err := s.resolve(artifactUUID, orgUUID)
	if err != nil {
		return nil, nil, err
	}
	snapshot, err := definition.Current(artifact)
	if err != nil {
		return nil, nil, err
	}
	// DP-originated artifacts are read-only in the control plane, so there is
	// nothing here to snapshot and deploy.
	if err := ensureOriginMutable(snapshot.Origin); err != nil {
		return nil, nil, err
	}
	contentBytes, err := yaml.Marshal(snapshot.Definition)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal %s deployment YAML: %w", artifact.Type, err)
	}
	return &model.Build{
		ArtifactID:     artifactUUID,
		OrganizationID: orgUUID,
		Content:        contentBytes,
		DataVersion:    snapshot.DataVersion,
		Metadata:       metadata,
		CreatedBy:      createdBy,
	}, snapshot.Definition, nil
}

// Create renders the artifact's current definition into an immutable snapshot and
// stores it, without deploying it anywhere.
func (s *BuildService) Create(artifactUUID, orgUUID, createdBy, description string,
	metadata map[string]interface{}) (*api.BuildResponse, error) {

	build, _, err := s.Render(artifactUUID, orgUUID, createdBy, metadata)
	if err != nil {
		return nil, err
	}
	build.Description = description
	if err := s.deploymentRepo.CreateBuildWithLimitEnforcement(build, s.cfg.Deployments.MaxBuildsPerAPI); err != nil {
		return nil, s.LimitError(err)
	}
	s.slogger.Debug("Build created", "buildID", build.BuildID, "artifactUUID", artifactUUID)
	return toAPIBuildResponse(build), nil
}

// Get returns one of an artifact's builds.
func (s *BuildService) Get(artifactUUID, buildID, orgUUID string) (*api.BuildResponse, error) {
	build, err := s.deploymentRepo.GetBuild(buildID, artifactUUID, orgUUID)
	if err != nil {
		return nil, err
	}
	if build == nil {
		return nil, apperror.BuildNotFound.New()
	}
	return toAPIBuildResponse(build), nil
}

// List returns an artifact's builds, newest first.
func (s *BuildService) List(artifactUUID, orgUUID string, limit int) (*api.BuildListResponse, error) {
	if _, _, err := s.resolve(artifactUUID, orgUUID); err != nil {
		return nil, err
	}
	builds, err := s.deploymentRepo.GetBuilds(artifactUUID, orgUUID, limit)
	if err != nil {
		return nil, err
	}
	list := make([]api.BuildResponse, 0, len(builds))
	for _, build := range builds {
		list = append(list, *toAPIBuildResponse(build))
	}
	return &api.BuildListResponse{Count: len(list), List: list}, nil
}

// Delete removes one of an artifact's builds.
//
// A build a gateway is serving is not deleted: taking the snapshot out from under
// it would leave the deployment with nothing to trace back to or promote onward,
// and the definition as it stood cannot be rendered again. Which deployment to give
// up is the caller's decision, so the conflict is reported rather than resolved.
func (s *BuildService) Delete(artifactUUID, buildID, orgUUID string) error {
	if _, _, err := s.resolve(artifactUUID, orgUUID); err != nil {
		return err
	}
	if err := s.deploymentRepo.DeleteBuild(buildID, artifactUUID, orgUUID); err != nil {
		switch {
		case errors.Is(err, repository.ErrBuildNotFound):
			return apperror.BuildNotFound.New()
		case errors.Is(err, repository.ErrBuildInUse):
			return apperror.BuildInUse.New()
		}
		return err
	}
	s.slogger.Debug("Build deleted", "buildID", buildID, "artifactUUID", artifactUUID)
	return nil
}

// LimitError turns the repository's "nothing free to remove" signal into the
// conflict a caller can act on, naming the limit they are up against. Any other
// error is passed through untouched. Deploy paths that store a build of their own
// use it too, so being at the limit reads the same however it is reached.
func (s *BuildService) LimitError(err error) error {
	if errors.Is(err, repository.ErrBuildLimitReached) {
		return apperror.BuildLimitReached.New(s.cfg.Deployments.MaxBuildsPerAPI)
	}
	return err
}

// DeploySource is what a deploy is about to put on a gateway: the definition to
// translate and override, the data version to translate FROM, and how the
// deployment records the build it runs.
//
// NewBuild is set only when the deploy rendered the artifact itself (base
// "current"). It is deliberately NOT stored here — the caller stores it on the
// transaction that records the deployment, so a deployment always has the build it
// runs and a failed deploy leaves no build behind.
type DeploySource struct {
	Definition  any
	DataVersion string
	NewBuild    *model.Build
	BuildUUID   *string
	BuildID     *string
}

// ValidateDeployBase checks the two fields that say WHAT a deploy ships.
//
// `base` is `current` (snapshot the artifact as it stands) or `build` (ship one
// prepared earlier, named by buildId). buildId is required with one and meaningless
// with the other; rejecting it where it cannot apply keeps a request from looking
// like it asked for something it did not get.
//
// The kind supplies its own validation error so the message names the right thing.
func ValidateDeployBase(base string, buildID *string, invalid apperror.Def) (string, string, error) {
	base = strings.TrimSpace(base)
	if base == "" {
		return "", "", invalid.New("Base is required (use 'current' or 'build').")
	}
	if base != deployBaseCurrent && base != deployBaseBuild {
		return "", "", invalid.New("Base must be 'current' or 'build'.")
	}
	requested := strings.TrimSpace(utils.ValueOrEmpty(buildID))
	if base == deployBaseBuild && requested == "" {
		return "", "", invalid.New("A buildId is required when base is 'build'.")
	}
	if base == deployBaseCurrent && requested != "" {
		return "", "", invalid.New("A buildId applies only when base is 'build'.")
	}
	return base, requested, nil
}

// SourceForDeploy resolves what a deploy ships, for any artifact kind.
//
// `build` loads the named snapshot and decodes it through the kind's own
// definition; `current` renders the artifact now and hands back an unstored build
// for the caller to commit alongside the deployment. Either way the caller gets one
// shape back, so the deploy paths stop differing on this.
func (s *BuildService) SourceForDeploy(artifactUUID, orgUUID, createdBy, base, requestedBuild string) (*DeploySource, error) {
	if base == deployBaseBuild {
		stored, err := s.deploymentRepo.GetBuild(requestedBuild, artifactUUID, orgUUID)
		if err != nil {
			return nil, fmt.Errorf("failed to get build: %w", err)
		}
		if stored == nil {
			return nil, apperror.BuildNotFound.New()
		}
		_, definition, err := s.resolve(artifactUUID, orgUUID)
		if err != nil {
			return nil, err
		}
		decoded, err := definition.Decode(stored.Content)
		if err != nil {
			return nil, err
		}
		return &DeploySource{
			Definition:  decoded,
			DataVersion: stored.DataVersion,
			// Record which build this deployment runs, so it can be traced back to
			// the snapshot it came from.
			BuildUUID: &stored.UUID,
			BuildID:   &stored.BuildID,
		}, nil
	}

	newBuild, definition, err := s.Render(artifactUUID, orgUUID, createdBy, nil)
	if err != nil {
		return nil, err
	}
	return &DeploySource{
		Definition:  definition,
		DataVersion: newBuild.DataVersion,
		NewBuild:    newBuild,
	}, nil
}
