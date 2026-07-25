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

package utils

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/dto"
	"github.com/wso2/api-platform/platform-api/internal/model"

	"net/http"

	commonconstants "github.com/wso2/api-platform/common/constants"
)

// GatewayArtifactsZipEntry is the file name, inside the multipart "artifacts" zip, that holds
// the JSON array of ImportGatewayArtifactRequest. The gateway writes this exact name when
// building the push zip.
const GatewayArtifactsZipEntry = "artifacts.json"

// gatewayArtifactsFormField is the multipart/form-data field that carries the artifacts zip.
const gatewayArtifactsFormField = "artifacts"

const (
	// maxArtifactsZipBytes caps the uploaded artifacts zip so a large upload cannot exhaust
	// memory during the read.
	maxArtifactsZipBytes = 32 << 20 // 32 MiB
	// maxArtifactsJSONBytes caps the decompressed artifacts.json entry, bounding memory use
	// against a zip bomb (a small zip that decompresses to a huge payload).
	maxArtifactsJSONBytes = 64 << 20 // 64 MiB
)

// ParseGatewayArtifactsRequest reads the multipart/form-data import-gateway-artifacts request:
// it extracts the "artifacts" zip part, decodes its artifacts.json entry into the list of
// import requests, and validates that the list is non-empty. It returns a descriptive error
// (suitable for a 400 response) when the part is missing, unreadable, not a valid zip, or empty.
func ParseGatewayArtifactsRequest(r *http.Request) ([]dto.ImportGatewayArtifactRequest, error) {
	f, fileHeader, err := r.FormFile(gatewayArtifactsFormField)
	if err != nil {
		return nil, fmt.Errorf("missing '%s' zip file in multipart form: %w", gatewayArtifactsFormField, err)
	}
	defer f.Close()
	if fileHeader.Size > maxArtifactsZipBytes {
		return nil, fmt.Errorf("'%s' file exceeds the maximum allowed size of %d bytes", gatewayArtifactsFormField, maxArtifactsZipBytes)
	}
	// Bound the read in case the reported size is unreliable.
	zipBytes, err := io.ReadAll(io.LimitReader(f, maxArtifactsZipBytes+1))
	if err != nil {
		return nil, fmt.Errorf("failed to read '%s' file: %w", gatewayArtifactsFormField, err)
	}
	if int64(len(zipBytes)) > maxArtifactsZipBytes {
		return nil, fmt.Errorf("'%s' file exceeds the maximum allowed size of %d bytes", gatewayArtifactsFormField, maxArtifactsZipBytes)
	}

	reqs, err := UnzipGatewayArtifacts(zipBytes)
	if err != nil {
		return nil, fmt.Errorf("invalid artifacts zip: %w", err)
	}
	if len(reqs) == 0 {
		return nil, fmt.Errorf("artifacts zip contains no artifacts")
	}
	return reqs, nil
}

// UnzipGatewayArtifacts reads the artifacts.json entry from the pushed zip and decodes it
// into the list of import requests.
func UnzipGatewayArtifacts(zipBytes []byte) ([]dto.ImportGatewayArtifactRequest, error) {
	zr, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	if err != nil {
		return nil, fmt.Errorf("failed to open zip: %w", err)
	}
	for _, file := range zr.File {
		if file.Name != GatewayArtifactsZipEntry {
			continue
		}
		if file.UncompressedSize64 > maxArtifactsJSONBytes {
			return nil, fmt.Errorf("%s exceeds the maximum allowed size of %d bytes", file.Name, maxArtifactsJSONBytes)
		}
		rc, err := file.Open()
		if err != nil {
			return nil, fmt.Errorf("failed to open %s: %w", file.Name, err)
		}
		defer rc.Close()
		// Bound the decompressed read to guard against a zip bomb / unreliable header.
		data, err := io.ReadAll(io.LimitReader(rc, maxArtifactsJSONBytes+1))
		if err != nil {
			return nil, fmt.Errorf("failed to read %s: %w", file.Name, err)
		}
		if int64(len(data)) > maxArtifactsJSONBytes {
			return nil, fmt.Errorf("%s exceeds the maximum allowed size of %d bytes", file.Name, maxArtifactsJSONBytes)
		}
		var reqs []dto.ImportGatewayArtifactRequest
		if err := json.Unmarshal(data, &reqs); err != nil {
			return nil, fmt.Errorf("failed to decode %s: %w", file.Name, err)
		}
		return reqs, nil
	}
	return nil, fmt.Errorf("zip does not contain %s", GatewayArtifactsZipEntry)
}

// artifactImportOrder is the dependency order in which gateway-pushed artifacts are created
// in the control plane: a kind must be created after the kinds it references. LLM providers
// reference templates, and LLM proxies reference providers, so templates come before
// providers before proxies. Kinds not listed sort last (stable), which is safe because they
// have no cross-kind dependencies among the supported set.
var artifactImportOrder = map[string]int{
	constants.LLMProviderTemplate: 0,
	constants.LLMProvider:         1,
	constants.LLMProxy:            2,
	constants.MCPProxy:            3,
	constants.RestApi:             4,
	constants.WebSubApi:           5,
	constants.WebBrokerApi:        6,
}

// ArtifactImportRank returns the creation-order rank for a kind; unknown kinds sort last.
func ArtifactImportRank(kind string) int {
	if rank, ok := artifactImportOrder[kind]; ok {
		return rank
	}
	return len(artifactImportOrder)
}

// ResolveImportProject extracts the project handle from the k8s-shaped metadata.
// The project is identified by the domain-prefixed project-id annotation or the deprecated bare label as a fallback.
func ResolveImportProject(md dto.ArtifactImportMetadata) string {
	projectHandle := md.Annotations[commonconstants.AnnotationProjectID]
	if projectHandle == "" {
		projectHandle = md.Labels[commonconstants.DeprecatedLabelProjectID]
	}
	return projectHandle
}

// IsNewerDeployment reports whether incoming supersedes the working copy's current
// deployment time. A nil current (no dated push yet) is always superseded. A nil incoming
// cannot establish ordering, so it is treated as not newer (stale) to avoid letting an
// undated push clobber a working copy set from a dated one.
func IsNewerDeployment(incoming, current *time.Time) bool {
	if incoming == nil {
		return false
	}
	if current == nil {
		return true
	}
	return incoming.After(*current)
}

// MetadataWriteMode is how an importer should treat the control plane's working copy
// of an artifact for a given DP->CP push.
type MetadataWriteMode int

const (
	// WriteFullMetadata persist both metadata and gateway-specific data. Used for a new
	// artifact, or a gateway-originated artifact whose push is the newest by deployment time.
	WriteFullMetadata MetadataWriteMode = iota
	// WriteGatewaySpecificOnly persist only gateway-specific data (e.g. upstreams), never
	// metadata. Used when the artifact is control-plane-owned (the CP owns its metadata).
	WriteGatewaySpecificOnly
	// SkipWorkingCopy leave the working copy untouched. Used when a gateway-originated push
	// is stale — its deployment time is not newer than the working copy's, so a more recent
	// deployment already defines the metadata. The per-gateway deployment status is still
	// recorded by the orchestrator; only the shared working copy is left alone.
	SkipWorkingCopy
)

// DecideMetadataWrite implements the DP->CP last-in-wins rule for the control plane's
// working copy of an artifact. Metadata ownership is no longer tied to a per-gateway
// flag: any gateway may define the working copy, and the most recent deployment (by the
// deployment time the gateway sends, in UTC) wins.
//
//   - new artifact                         -> WriteFullMetadata
//   - existing, origin control_plane       -> WriteGatewaySpecificOnly (the CP owns metadata)
//   - existing, origin gateway_api, newer  -> WriteFullMetadata (this push wins)
//   - existing, origin gateway_api, stale  -> SkipWorkingCopy (an out-of-order/older push)
func DecideMetadataWrite(isNew bool, origin string, currentDeployedAt, incomingDeployedAt *time.Time) MetadataWriteMode {
	if isNew {
		return WriteFullMetadata
	}
	if origin == constants.OriginCP {
		return WriteGatewaySpecificOnly
	}
	if IsNewerDeployment(incomingDeployedAt, currentDeployedAt) {
		return WriteFullMetadata
	}
	return SkipWorkingCopy
}

// Pushed artifact status strings.
const (
	ImportStatusDeployed   = "deployed"
	ImportStatusPending    = "pending"
	ImportStatusFailed     = "failed"
	ImportStatusUndeployed = "undeployed"
)

// MapDeploymentStatus maps a pushed status string to a deployment_status value.
func MapDeploymentStatus(status string) model.DeploymentStatus {
	switch status {
	case ImportStatusDeployed:
		return model.DeploymentStatusDeployed
	case ImportStatusPending:
		return model.DeploymentStatusDeploying
	case ImportStatusFailed:
		return model.DeploymentStatusFailed
	case ImportStatusUndeployed:
		return model.DeploymentStatusUndeployed
	default:
		return model.DeploymentStatusDeployed
	}
}

// ImportVersion resolves the artifact version from the CR spec (version lives in the
// spec, not the k8s metadata), defaulting to "1.0.0" when absent.
func ImportVersion(cfg dto.ArtifactImportConfig) string {
	if v := SpecString(cfg.Spec, "version"); v != "" {
		return v
	}
	return "1.0.0"
}

// ImportHandle returns the URL-safe identifier for the artifact. metadata.name is
// treated as the handle (identifier) in the k8s-shaped descriptor.
func ImportHandle(cfg dto.ArtifactImportConfig) string {
	return cfg.Metadata.Name
}

// ImportDisplayName returns the human-friendly name, falling back to the handle.
func ImportDisplayName(cfg dto.ArtifactImportConfig) string {
	if dn := SpecString(cfg.Spec, "displayName"); dn != "" {
		return dn
	}
	return cfg.Metadata.Name
}

// SpecString safely extracts a string value from the generic spec map.
func SpecString(spec map[string]interface{}, key string) string {
	if spec == nil {
		return ""
	}
	if v, ok := spec[key].(string); ok {
		return v
	}
	return ""
}

// DecodeSpec re-marshals the generic spec map into a kind-specific struct via JSON.
// Unknown fields are ignored, matching fields are populated.
func DecodeSpec(spec map[string]interface{}, out interface{}) error {
	if len(spec) == 0 {
		return nil
	}
	b, err := json.Marshal(spec)
	if err != nil {
		return fmt.Errorf("failed to marshal spec: %w", err)
	}
	if err := json.Unmarshal(b, out); err != nil {
		return fmt.Errorf("failed to decode spec: %w", err)
	}
	return nil
}

// StringProperty reads a string value from an import request Properties bag, returning "" when
// the bag is nil, the key is absent, or the value is not a string.
func StringProperty(props map[string]interface{}, key string) string {
	if props == nil {
		return ""
	}
	if v, ok := props[key].(string); ok {
		return v
	}
	return ""
}

// RevisionMetadata builds the deployment metadata map carrying the gateway revision, or nil when
// no revision was supplied (so pre-revision pushes store no metadata, as before).
func RevisionMetadata(revision string) map[string]any {
	if revision == "" {
		return nil
	}
	return map[string]any{"gatewayRevision": revision}
}

// TemplateVersionNewer reports whether template version a is strictly higher than b.
// versions are v<major>.<minor> (e.g. v1.0, v2.3)
func TemplateVersionNewer(a, b string) bool {
	aMaj, aMin, aOK := parseTemplateVersion(a)
	bMaj, bMin, bOK := parseTemplateVersion(b)
	if aOK && bOK {
		if aMaj != bMaj {
			return aMaj > bMaj
		}
		return aMin > bMin
	}
	return a > b
}

func parseTemplateVersion(v string) (major, minor int, ok bool) {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "v")
	v = strings.TrimPrefix(v, "V")
	majStr, minStr, found := strings.Cut(v, ".")
	if !found {
		return 0, 0, false
	}
	major, err1 := strconv.Atoi(majStr)
	minor, err2 := strconv.Atoi(minStr)
	if err1 != nil || err2 != nil {
		return 0, 0, false
	}
	return major, minor, true
}
