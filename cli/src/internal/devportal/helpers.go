/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package devportal

import (
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/wso2/api-platform/cli/internal/config"
)

// APIVersion is the Developer Portal REST API version segment used in every
// resource path. Bump this in one place when the API version changes.
const APIVersion = "v0.9"

// ResourcePath builds a Developer Portal resource path of the form
// /api/{version}/{resource}. Every devportal endpoint — including the
// organization lifecycle endpoints (organizations, organizations/{orgId}) — is
// served under this /api/{version} prefix.
//
// The organization that scopes a request is resolved server-side from the
// caller's credentials, so it is no longer part of the path (only the
// organization's own id remains, as the {orgId} segment of organizations/{orgId}).
// The resource is appended as-is, so callers escape any path segments they
// interpolate (for example "apis/"+url.PathEscape(apiID)) and may include a
// trailing query string (for example "api-keys?apiId=x").
func ResourcePath(resource string) string {
	return fmt.Sprintf("/api/%s/%s", APIVersion, strings.TrimPrefix(resource, "/"))
}

// ResolveDevPortal resolves the DevPortal to use from either explicit flags
// or the active DevPortal in the resolved platform.
func ResolveDevPortal(cfg *config.Config, selectedName, selectedPlatform string) (*config.DevPortal, string, error) {
	selectedName = strings.TrimSpace(selectedName)
	selectedPlatform = strings.TrimSpace(selectedPlatform)

	if selectedName != "" {
		resolvedPlatform := config.DefaultPlatform
		if selectedPlatform != "" {
			resolvedPlatform = cfg.ResolvePlatform(selectedPlatform)
		}

		devPortal, err := cfg.GetDevPortalFromPlatform(resolvedPlatform, selectedName)
		if err != nil {
			return nil, "", err
		}
		return devPortal, resolvedPlatform, nil
	}

	resolvedPlatform := cfg.ResolvePlatform(selectedPlatform)
	devPortal, err := cfg.GetActiveDevPortalFromPlatform(resolvedPlatform)
	if err != nil {
		if selectedPlatform != "" {
			return nil, "", err
		}
		return nil, "", fmt.Errorf("no active devportal set for platform '%s'", resolvedPlatform)
	}

	return devPortal, resolvedPlatform, nil
}

// ShouldSuggestInsecure reports whether an error looks like a TLS certificate
// verification failure where retrying with --insecure would help.
func ShouldSuggestInsecure(err error) bool {
	var unknownAuthorityErr x509.UnknownAuthorityError
	var hostnameError x509.HostnameError
	var certificateInvalidError x509.CertificateInvalidError
	const certificateInvalidErrorString = "tls: failed to verify certificate: x509"

	return errors.As(err, &unknownAuthorityErr) ||
		errors.As(err, &hostnameError) ||
		errors.As(err, &certificateInvalidError) ||
		strings.Contains(err.Error(), certificateInvalidErrorString)
}

// ResolveArtifactPath validates a DevPortal artifact zip path and falls back
// to ./devportal.zip when no explicit file path is provided.
func ResolveArtifactPath(requestedPath string) (string, error) {
	requestedPath = strings.TrimSpace(requestedPath)
	if requestedPath == "" {
		requestedPath = "./devportal.zip"
	}

	artifactPath, err := filepath.Abs(requestedPath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve artifact path: %w", err)
	}

	artifactInfo, err := os.Stat(artifactPath)
	if err != nil {
		if os.IsNotExist(err) {
			if strings.TrimSpace(requestedPath) == "./devportal.zip" || strings.TrimSpace(requestedPath) == "devportal.zip" {
				return "", fmt.Errorf("artifact file not found: %s. Provide --file or place devportal.zip in the current directory", artifactPath)
			}
			return "", fmt.Errorf("artifact file not found: %s", artifactPath)
		}
		return "", fmt.Errorf("failed to inspect artifact file: %w", err)
	}
	if artifactInfo.IsDir() {
		return "", fmt.Errorf("artifact path must be a zip file, got directory: %s", artifactPath)
	}
	if strings.ToLower(filepath.Ext(artifactPath)) != ".zip" {
		return "", fmt.Errorf("artifact file must be a .zip file: %s", artifactPath)
	}

	return artifactPath, nil
}

// ReadJSONFile reads a JSON payload file from disk after validating that the
// path exists and points to a regular file.
func ReadJSONFile(filePath string) ([]byte, error) {
	resolvedPath, err := filepath.Abs(strings.TrimSpace(filePath))
	if err != nil {
		return nil, fmt.Errorf("failed to resolve file path: %w", err)
	}

	info, err := os.Stat(resolvedPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("file not found: %s", resolvedPath)
		}
		return nil, fmt.Errorf("failed to inspect file: %w", err)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("file path must point to a JSON file, got directory: %s", resolvedPath)
	}

	content, err := os.ReadFile(resolvedPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	return content, nil
}

// WrapRequestError formats DevPortal request failures and adds a hint about
// --insecure when the error is caused by certificate verification.
func WrapRequestError(action string, err error, insecure bool) error {
	if ShouldSuggestInsecure(err) && !insecure {
		return fmt.Errorf("failed to %s: %w\nhint: retry with --insecure if you are using a self-signed or local development certificate", action, err)
	}
	return fmt.Errorf("failed to %s: %w", action, err)
}

// PrintJSONResponse prints a DevPortal HTTP response as pretty JSON when
// possible and falls back to the raw trimmed body otherwise.
func PrintJSONResponse(resp *http.Response) error {
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" {
		return nil
	}

	var jsonBody interface{}
	if err := json.Unmarshal(body, &jsonBody); err == nil {
		prettyBody, marshalErr := json.MarshalIndent(jsonBody, "", "  ")
		if marshalErr == nil {
			fmt.Println(string(prettyBody))
			return nil
		}
	}

	fmt.Println(trimmed)
	return nil
}
