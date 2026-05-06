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
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"platform-api/src/internal/model"

	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

// addFileToZip adds a single file to the zip writer.
// On error it attempts to close the zip writer before returning.
func addFileToZip(zipWriter *zip.Writer, fileName string, content []byte) error {
	fileWriter, err := zipWriter.Create(fileName)
	if err != nil {
		if closeErr := zipWriter.Close(); closeErr != nil {
			return fmt.Errorf("failed to create file in zip: %w (close error: %v)", err, closeErr)
		}
		return fmt.Errorf("failed to create file in zip: %w", err)
	}

	if _, err = fileWriter.Write(content); err != nil {
		if closeErr := zipWriter.Close(); closeErr != nil {
			return fmt.Errorf("failed to write file content: %w (close error: %v)", err, closeErr)
		}
		return fmt.Errorf("failed to write file content: %w", err)
	}
	return nil
}

// CreateAPIYamlZip creates a ZIP file containing API YAML files
func CreateAPIYamlZip(apiYamlMap map[string]string) ([]byte, error) {
	var buf bytes.Buffer
	zipWriter := zip.NewWriter(&buf)

	for apiID, yamlContent := range apiYamlMap {
		fileName := fmt.Sprintf("api-%s.yaml", apiID)
		if err := addFileToZip(zipWriter, fileName, []byte(yamlContent)); err != nil {
			return nil, err
		}
	}

	if err := zipWriter.Close(); err != nil {
		return nil, fmt.Errorf("failed to close zip writer: %w", err)
	}

	return buf.Bytes(), nil
}

// CreateLLMProviderYamlZip creates a ZIP file containing LLM provider YAML files
func CreateLLMProviderYamlZip(providerYamlMap map[string]string) ([]byte, error) {
	var buf bytes.Buffer
	zipWriter := zip.NewWriter(&buf)

	for providerID, yamlContent := range providerYamlMap {
		fileName := fmt.Sprintf("llm-provider-%s.yaml", providerID)
		if err := addFileToZip(zipWriter, fileName, []byte(yamlContent)); err != nil {
			return nil, err
		}
	}

	if err := zipWriter.Close(); err != nil {
		return nil, fmt.Errorf("failed to close zip writer: %w", err)
	}

	return buf.Bytes(), nil
}

// CreateLLMProxyYamlZip creates a ZIP file containing LLM proxy YAML files
func CreateLLMProxyYamlZip(proxyYamlMap map[string]string) ([]byte, error) {
	var buf bytes.Buffer
	zipWriter := zip.NewWriter(&buf)

	for proxyID, yamlContent := range proxyYamlMap {
		fileName := fmt.Sprintf("llm-proxy-%s.yaml", proxyID)
		if err := addFileToZip(zipWriter, fileName, []byte(yamlContent)); err != nil {
			return nil, err
		}
	}

	if err := zipWriter.Close(); err != nil {
		return nil, fmt.Errorf("failed to close zip writer: %w", err)
	}

	return buf.Bytes(), nil
}

// CreateBatchDeploymentTarGz creates a TAR.GZ archive containing deployment YAML files
// organized in directories by deployment ID. The filename prefix is determined
// by the artifact kind: api-, llm-provider-, or llm-proxy-.
// Structure:
//
//	batch.tar.gz
//	├── dep-789/
//	│   └── api-{artifactID}.yaml
//	├── dep-456/
//	│   └── llm-provider-{artifactID}.yaml
//	└── dep-111/
//	    └── llm-proxy-{artifactID}.yaml
//
// TAR.GZ is used here (over ZIP) because gzip compresses the entire stream as one unit,
// exploiting the high structural similarity across YAML files in a batch.
func CreateBatchDeploymentTarGz(deploymentContentMap map[string]*model.DeploymentContent) ([]byte, error) {
	var buf bytes.Buffer
	gzWriter, err := gzip.NewWriterLevel(&buf, gzip.BestSpeed)
	if err != nil {
		return nil, fmt.Errorf("failed to create gzip writer: %w", err)
	}
	tarWriter := tar.NewWriter(gzWriter)

	for deploymentID, dc := range deploymentContentMap {
		var prefix string
		switch dc.Kind {
		case "LlmProvider":
			prefix = "llm-provider"
		case "LlmProxy":
			prefix = "llm-proxy"
		case "Mcp":
			prefix = "mcp-proxy"
		default: // RestApi and any future kinds
			prefix = "api"
		}
		fileName := fmt.Sprintf("%s/%s-%s.yaml", deploymentID, prefix, dc.ArtifactID)
		hdr := &tar.Header{
			Name:    fileName,
			Mode:    0644,
			Size:    int64(len(dc.Content)),
			ModTime: time.Now(),
		}
		if err := tarWriter.WriteHeader(hdr); err != nil {
			return nil, fmt.Errorf("failed to write tar header for %s: %w", fileName, err)
		}
		if _, err := tarWriter.Write(dc.Content); err != nil {
			return nil, fmt.Errorf("failed to write tar content for %s: %w", fileName, err)
		}
	}

	if err := tarWriter.Close(); err != nil {
		return nil, fmt.Errorf("failed to close tar writer: %w", err)
	}
	if err := gzWriter.Close(); err != nil {
		return nil, fmt.Errorf("failed to close gzip writer: %w", err)
	}

	return buf.Bytes(), nil
}

// CreateWebSubAPIYamlZip creates a ZIP file containing WebSub API YAML files
func CreateWebSubAPIYamlZip(apiYamlMap map[string]string) ([]byte, error) {
	var buf bytes.Buffer
	zipWriter := zip.NewWriter(&buf)

	for apiID, yamlContent := range apiYamlMap {
		fileName := fmt.Sprintf("websub-api-%s.yaml", apiID)
		if err := addFileToZip(zipWriter, fileName, []byte(yamlContent)); err != nil {
			return nil, err
		}
	}

	if err := zipWriter.Close(); err != nil {
		return nil, fmt.Errorf("failed to close zip writer: %w", err)
	}

	return buf.Bytes(), nil
}

// CreateMCPProxyYamlZip creates a ZIP file containing MCP proxy YAML files
func CreateMCPProxyYamlZip(proxyYamlMap map[string]string) ([]byte, error) {
	var buf bytes.Buffer
	zipWriter := zip.NewWriter(&buf)

	for proxyID, yamlContent := range proxyYamlMap {
		fileName := fmt.Sprintf("mcp-proxy-%s.yaml", proxyID)
		fileWriter, err := zipWriter.Create(fileName)
		if err != nil {
			if closeErr := zipWriter.Close(); closeErr != nil {
				return nil, fmt.Errorf("failed to create file in zip: %w (close error: %v)", err, closeErr)
			}
			return nil, fmt.Errorf("failed to create file in zip: %w", err)
		}

		_, err = fileWriter.Write([]byte(yamlContent))
		if err != nil {
			if closeErr := zipWriter.Close(); closeErr != nil {
				return nil, fmt.Errorf("failed to write file content: %w (close error: %v)", err, closeErr)
			}
			return nil, fmt.Errorf("failed to write file content: %w", err)
		}
	}

	err := zipWriter.Close()
	if err != nil {
		return nil, fmt.Errorf("failed to close zip writer: %w", err)
	}

	return buf.Bytes(), nil
}

// OpenAPIUUIDToString converts an OpenAPI UUID to string.
func OpenAPIUUIDToString(id openapi_types.UUID) string {
	return uuid.UUID(id).String()
}

// ParseOpenAPIUUID parses a UUID string into an OpenAPI UUID pointer.
func ParseOpenAPIUUID(id string) (*openapi_types.UUID, error) {
	parsed, err := uuid.Parse(id)
	if err != nil {
		return nil, err
	}
	openapiUUID := openapi_types.UUID(parsed)
	return &openapiUUID, nil
}

// ParseOptionalOpenAPIUUID parses an optional UUID string pointer into an OpenAPI UUID pointer.
// Returns nil when input is nil, empty, or invalid.
func ParseOptionalOpenAPIUUID(id *string) *openapi_types.UUID {
	if id == nil || *id == "" {
		return nil
	}

	parsed, err := ParseOpenAPIUUID(*id)
	if err != nil {
		return nil
	}

	return parsed
}

// ParseOpenAPIUUIDOrZero parses a UUID string into an OpenAPI UUID value.
// Returns zero UUID when input is invalid.
func ParseOpenAPIUUIDOrZero(id string) openapi_types.UUID {
	parsed, err := ParseOpenAPIUUID(id)
	if err != nil || parsed == nil {
		return openapi_types.UUID{}
	}

	return *parsed
}

// StringPtrIfNotEmpty returns a pointer for non-empty strings.
func StringPtrIfNotEmpty(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return &value
}

// defaultStringPtr returns the string value if not nil, otherwise empty string.
func defaultStringPtr(value *string) string {
	if value == nil {
		return ""
	}

	return *value
}

// stringSlicePtr returns a pointer to a non-empty string slice or nil for an empty slice.
func stringSlicePtr(values []string) *[]string {
	if len(values) == 0 {
		return nil
	}

	return &values
}

// stringSliceValue returns the slice value or nil slice if the pointer is nil.
func stringSliceValue(ptr *[]string) []string {
	if ptr == nil {
		return nil
	}
	return *ptr
}

// TimePtrIfNotZero returns a pointer for non-zero timestamps.
func TimePtrIfNotZero(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	return &value
}

// BoolPtr returns a pointer to the provided boolean.
func BoolPtr(value bool) *bool {
	return &value
}

func MapPtrIfNotEmpty(m map[string]interface{}) *map[string]interface{} {
	if len(m) == 0 {
		return nil
	}
	return &m
}

// MapValueOrEmpty returns the map value when non-nil or an empty map otherwise.
func MapValueOrEmpty(m *map[string]interface{}) map[string]interface{} {
	if m == nil {
		return map[string]interface{}{}
	}

	return *m
}

// StringPtrValue returns the value of a string pointer or empty string if nil
func StringPtrValue(ptr *string) string {
	if ptr == nil {
		return ""
	}
	return *ptr
}

// ValueOrEmpty returns the string value or empty string if nil
func ValueOrEmpty(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// DefaultStringPtr returns the string value if not nil/empty, otherwise the default
func DefaultStringPtr(v *string, def string) string {
	if v == nil {
		return def
	}
	if strings.TrimSpace(*v) == "" {
		return def
	}
	return *v
}

// TimePtr returns a pointer to the given time
func TimePtr(t time.Time) *time.Time {
	return &t
}

// GenerateUUID generates a new UUID v7 string
func GenerateUUID() (string, error) {
	u, err := uuid.NewV7()
	if err != nil {
		return "", fmt.Errorf("failed to generate UUID v7: %w", err)
	}
	return u.String(), nil
}

// ValidateURL validates a URL with additional checks
func ValidateURL(rawURL string) error {
	if rawURL == "" {
		return errors.New("URL is required")
	}

	parsedURL, err := url.ParseRequestURI(rawURL)
	if err != nil {
		return errors.New("Invalid URL format")
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return errors.New("URL must use http or https")
	}

	if parsedURL.Host == "" {
		return errors.New("URL must include a valid host")
	}

	if parsedURL.User != nil {
		return errors.New("URL must not include user credentials")
	}

	if parsedURL.Fragment != "" {
		return errors.New("URL must not include a fragment")
	}

	if parsedURL.Port() != "" {
		port, err := strconv.Atoi(parsedURL.Port())
		if err != nil || port < 1 || port > 65535 {
			return errors.New("URL must include a valid port")
		}
	}

	if hasTraversalSegments(parsedURL.EscapedPath()) {
		return errors.New("URL path must not contain traversal segments")
	}

	return nil
}

// CheckURLReachability verifies that the given URL is reachable by sending an HTTP HEAD request.
// It returns an error if the request fails or cannot complete within the given timeout.
func CheckURLReachability(rawURL string, timeout time.Duration) error {
	client := &http.Client{
		Timeout: timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return errors.New("too many redirects")
			}
			return nil
		},
	}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodHead, rawURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request")
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("URL is not reachable")
	}
	defer resp.Body.Close() //nolint:errcheck

	return nil
}

// APIKeyETag derives a stable UUID v7-formatted ETag from the unique
// (artifactUUID, name, updatedAt) tuple by SHA-256 hashing them, taking the
// first 16 bytes, then stamping the UUID version (7) and RFC 4122 variant bits.
// updatedAt is included to capture the current state of the artifact, ensuring
// the ETag changes on every update and remains unique as the PK in the event table.
func APIKeyETag(artifactUUID, name string, updatedAt time.Time) string {
	h := sha256.Sum256([]byte(fmt.Sprintf("%s:%s:%d", artifactUUID, name, updatedAt.UnixNano())))
	var uid uuid.UUID
	copy(uid[:], h[:16])
	uid[6] = (uid[6] & 0x0f) | 0x70 // version = 7
	uid[8] = (uid[8] & 0x3f) | 0x80 // RFC 4122 variant
	return uid.String()
}

func hasTraversalSegments(escapedPath string) bool {
	for segment := range strings.SplitSeq(escapedPath, "/") {
		if segment == "" {
			continue
		}

		unescapedSegment, err := url.PathUnescape(segment)
		if err != nil {
			return true
		}

		if unescapedSegment == "." || unescapedSegment == ".." || strings.Contains(unescapedSegment, `\`) {
			return true
		}
	}

	return false
}

// GenerateDeterministicUUIDv7 generates a deterministic UUIDv7 from an entity ID and timestamp.
// The timestamp component uses the provided time (millisecond precision) and the random bits are
// derived from a SHA-256 hash of the entityID. This ensures:
//   - Same (entityID, timestamp) always produces the same UUID
//   - UUIDs are time-ordered (UUIDv7 property)
//   - Different entityIDs produce different UUIDs even at the same timestamp
func GenerateDeterministicUUIDv7(entityID string, ts time.Time) string {
	hash := sha256.Sum256([]byte(entityID))

	var b [16]byte

	// Set 48-bit millisecond timestamp
	ms := uint64(ts.UnixMilli())
	binary.BigEndian.PutUint16(b[0:2], uint16(ms>>32))
	binary.BigEndian.PutUint32(b[2:6], uint32(ms))

	// Fill remaining bytes from hash (deterministic "random" bits)
	copy(b[6:], hash[:10])

	// Set version 7 (bits 48-51 = 0111)
	b[6] = (b[6] & 0x0F) | 0x70

	// Set variant (bits 64-65 = 10)
	b[8] = (b[8] & 0x3F) | 0x80

	u, _ := uuid.FromBytes(b[:])
	return u.String()
}
