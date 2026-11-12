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
	"sort"
	"strings"

	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/repository"
)

// SecretRefContext names the artifact whose configuration references secrets.
// A {{ secret "handle" }} reference resolves only to a secret that is scoped to
// the whole organization, to ProjectUUID, or to ArtifactUUID. Leave a field empty
// when it does not apply (a new artifact has no UUID yet; an LLM provider has no
// project): an empty field matches no project or artifact scope, so the check
// fails closed for secrets that are not org-wide.
type SecretRefContext struct {
	ProjectUUID  string
	ArtifactUUID string
}

// secretRefContextFor builds a SecretRefContext from an optional project UUID.
func secretRefContextFor(projectUUID *string, artifactUUID string) SecretRefContext {
	ctx := SecretRefContext{ArtifactUUID: artifactUUID}
	if projectUUID != nil {
		ctx.ProjectUUID = *projectUUID
	}
	return ctx
}

// scopeArtifactKinds are the artifact kinds a secret can be scoped to: the kinds
// whose configuration the Platform API validates for {{ secret }} references.
var scopeArtifactKinds = []string{
	constants.RestApi,
	constants.GraphQLApi,
	constants.LLMProvider,
	constants.LLMProxy,
	constants.MCPProxy,
	constants.AgentProxy,
}

// SecretScopeResolver turns the handles in a scope ("project:<handle>",
// "artifact:<kind>:<handle>") into the UUIDs secret_scopes stores, and back.
type SecretScopeResolver interface {
	ProjectUUID(orgID, handle string) (string, error)
	ProjectHandle(orgID, uuid string) (string, error)
	ArtifactUUID(orgID, kind, handle string) (string, error)
	ArtifactKindAndHandle(orgID, uuid string) (kind, handle string, err error)
	// ArtifactProjectUUID returns the UUID of the artifact's project, or "" when
	// its kind has no project or it belongs to none.
	ArtifactProjectUUID(orgID, artifactUUID string) (string, error)
}

// secretScopeArtifacts is the part of the artifact repository the resolver uses.
// *repository.ArtifactRepo satisfies it.
type secretScopeArtifacts interface {
	GetAPIMetadataByHandleAndKind(handle, kind, orgUUID string) (*model.APIMetadata, error)
	GetMetadataByUUIDs(uuids []string, orgUUID string) (map[string]*model.APIMetadata, error)
	GetProjectHandle(kind, handle, orgUUID string) (projectHandle string, found bool, err error)
}

// repoSecretScopeResolver is the SecretScopeResolver backed by the project and
// artifact repositories.
type repoSecretScopeResolver struct {
	projects  repository.ProjectRepository
	artifacts secretScopeArtifacts
}

// NewSecretScopeResolver returns a SecretScopeResolver over the given repositories.
func NewSecretScopeResolver(projects repository.ProjectRepository, artifacts secretScopeArtifacts) SecretScopeResolver {
	return &repoSecretScopeResolver{projects: projects, artifacts: artifacts}
}

func (r *repoSecretScopeResolver) ProjectUUID(orgID, handle string) (string, error) {
	p, err := r.projects.GetProjectByHandleAndOrgID(handle, orgID)
	if err != nil {
		return "", err
	}
	if p == nil {
		return "", apperror.ProjectNotFound.New()
	}
	return p.ID, nil
}

func (r *repoSecretScopeResolver) ProjectHandle(orgID, uuid string) (string, error) {
	p, err := r.projects.GetProjectByUUIDAndOrgID(uuid, orgID)
	if err != nil {
		return "", err
	}
	if p == nil {
		return "", apperror.ProjectNotFound.New()
	}
	return p.Handle, nil
}

func (r *repoSecretScopeResolver) ArtifactUUID(orgID, kind, handle string) (string, error) {
	m, err := r.artifacts.GetAPIMetadataByHandleAndKind(handle, kind, orgID)
	if err != nil {
		return "", err
	}
	if m == nil {
		return "", errSecretScopeArtifactNotFound
	}
	return m.ID, nil
}

func (r *repoSecretScopeResolver) ArtifactKindAndHandle(orgID, uuid string) (string, string, error) {
	byUUID, err := r.artifacts.GetMetadataByUUIDs([]string{uuid}, orgID)
	if err != nil {
		return "", "", err
	}
	m, ok := byUUID[uuid]
	if !ok || m == nil {
		return "", "", errSecretScopeArtifactNotFound
	}
	return m.Kind, m.Handle, nil
}

func (r *repoSecretScopeResolver) ArtifactProjectUUID(orgID, artifactUUID string) (string, error) {
	kind, handle, err := r.ArtifactKindAndHandle(orgID, artifactUUID)
	if err != nil {
		if errors.Is(err, errSecretScopeArtifactNotFound) {
			return "", nil
		}
		return "", err
	}
	projectHandle, found, err := r.artifacts.GetProjectHandle(kind, handle, orgID)
	if err != nil {
		if errors.Is(err, repository.ErrUnknownArtifactKind) {
			return "", nil
		}
		return "", err
	}
	if !found || projectHandle == "" {
		return "", nil
	}
	return r.ProjectUUID(orgID, projectHandle)
}

var errSecretScopeArtifactNotFound = errors.New("artifact not found")

// parseSecretScopes turns the "scopes" form values into secret_scopes rows.
// Each value is "org", "project:<handle>" or "artifact:<kind>:<handle>". "org"
// cannot be combined with a narrower scope, since it already admits every
// artifact in the organization. Duplicates collapse.
func (s *SecretService) parseSecretScopes(orgID string, raw []string) ([]model.SecretScope, error) {
	var (
		out  []model.SecretScope
		seen = map[string]bool{}
		org  bool
	)
	for _, v := range raw {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		parts := strings.SplitN(v, ":", 3)
		var sc model.SecretScope
		switch {
		case len(parts) == 1 && parts[0] == model.SecretScopeTypeOrg:
			org = true
			sc = model.SecretScope{Scope: model.SecretScopeTypeOrg, ScopeValue: orgID}
		case len(parts) == 2 && parts[0] == model.SecretScopeTypeProject && parts[1] != "":
			if s.scopeResolver == nil {
				return nil, errSecretScopesUnsupported
			}
			id, err := s.scopeResolver.ProjectUUID(orgID, parts[1])
			if err != nil {
				return nil, invalidSecretScope(v, err)
			}
			sc = model.SecretScope{Scope: model.SecretScopeTypeProject, ScopeValue: id}
		case len(parts) == 3 && parts[0] == model.SecretScopeTypeArtifact && parts[1] != "" && parts[2] != "":
			if s.scopeResolver == nil {
				return nil, errSecretScopesUnsupported
			}
			if !isScopeArtifactKind(parts[1]) {
				return nil, apperror.ValidationFailed.New(fmt.Sprintf(
					"Scope %q names an unsupported artifact kind. Use one of: %s.", v, strings.Join(scopeArtifactKinds, ", ")))
			}
			id, err := s.scopeResolver.ArtifactUUID(orgID, parts[1], parts[2])
			if err != nil {
				return nil, invalidSecretScope(v, err)
			}
			sc = model.SecretScope{Scope: model.SecretScopeTypeArtifact, ScopeValue: id}
		default:
			return nil, apperror.ValidationFailed.New(fmt.Sprintf(
				"Scope %q is not valid. Use \"org\", \"project:<project id>\" or \"artifact:<kind>:<artifact id>\".", v))
		}
		key := sc.Scope + "\x00" + sc.ScopeValue
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, sc)
	}
	if org && len(out) > 1 {
		return nil, apperror.ValidationFailed.New(
			"Scope \"org\" already admits every artifact in the organization; it cannot be combined with a project or artifact scope.")
	}
	return out, nil
}

var errSecretScopesUnsupported = apperror.ValidationFailed.New("Project and artifact scopes are not available on this server.")

func invalidSecretScope(v string, err error) error {
	if apperror.ProjectNotFound.Is(err) || errors.Is(err, errSecretScopeArtifactNotFound) ||
		errors.Is(err, repository.ErrUnknownArtifactKind) {
		return apperror.ValidationFailed.New(fmt.Sprintf("Scope %q names a project or artifact that does not exist in this organization.", v))
	}
	return fmt.Errorf("failed to resolve secret scope %q: %w", v, err)
}

func isScopeArtifactKind(kind string) bool {
	for _, k := range scopeArtifactKinds {
		if k == kind {
			return true
		}
	}
	return false
}

// scopeAdmits reports whether a secret with the given scopes may be referenced
// from the artifact ctx describes. A secret with no scope rows predates scoping
// (Create has always written an org scope, and the migration verifier reports
// any secret without one); it stays org-wide, as it was before scopes were
// enforced, so existing references keep working.
func scopeAdmits(orgID string, scopes []model.SecretScope, ctx SecretRefContext) bool {
	if len(scopes) == 0 {
		return true
	}
	for _, sc := range scopes {
		switch sc.Scope {
		case model.SecretScopeTypeOrg:
			if sc.ScopeValue == orgID {
				return true
			}
		case model.SecretScopeTypeProject:
			if ctx.ProjectUUID != "" && sc.ScopeValue == ctx.ProjectUUID {
				return true
			}
		case model.SecretScopeTypeArtifact:
			if ctx.ArtifactUUID != "" && sc.ScopeValue == ctx.ArtifactUUID {
				return true
			}
		}
	}
	return false
}

// describeSecretScopes renders stored scopes back to the handles a caller uses.
// A project or artifact that no longer resolves is shown by its UUID.
func (s *SecretService) describeSecretScopes(orgID string, scopes []model.SecretScope) []string {
	out := make([]string, 0, len(scopes))
	for _, sc := range scopes {
		switch sc.Scope {
		case model.SecretScopeTypeOrg:
			out = append(out, model.SecretScopeTypeOrg)
		case model.SecretScopeTypeProject:
			h := sc.ScopeValue
			if s.scopeResolver != nil {
				if v, err := s.scopeResolver.ProjectHandle(orgID, sc.ScopeValue); err == nil {
					h = v
				}
			}
			out = append(out, model.SecretScopeTypeProject+":"+h)
		case model.SecretScopeTypeArtifact:
			v := sc.ScopeValue
			if s.scopeResolver != nil {
				if kind, h, err := s.scopeResolver.ArtifactKindAndHandle(orgID, sc.ScopeValue); err == nil {
					v = kind + ":" + h
				}
			}
			out = append(out, model.SecretScopeTypeArtifact+":"+v)
		}
	}
	sort.Strings(out)
	return out
}
