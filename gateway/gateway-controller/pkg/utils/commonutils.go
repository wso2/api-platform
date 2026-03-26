/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

package utils

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

// escapeParam escapes special characters in a parameter value to prevent
// format string injection and YAML injection attacks
func escapeParam(param string) string {
	// Escape % to prevent format string injection in fmt.Sprintf
	escaped := strings.ReplaceAll(param, "%", "%%")
	return escaped
}

// GetParamsOfPolicy renders a policy definition template with given parameters
// and unmarshals it into a map[string]any
func GetParamsOfPolicy(policyDef string, params ...string) (map[string]any, error) {
	args := make([]any, len(params))
	for i, v := range params {
		args[i] = escapeParam(v)
	}
	rendered := fmt.Sprintf(policyDef, args...)

	var m map[string]any
	if err := yaml.Unmarshal([]byte(rendered), &m); err != nil {
		return map[string]any{}, err
	}
	return m, nil
}

// APIKeyETag produces a deterministic UUID v7-formatted ETag from the unique
// (artifactUUID, name, updatedAt) tuple. Uses SHA-256 of the tuple as the source
// bytes, then stamps version=7 and RFC 4122 variant bits.
// Mirrors the same algorithm used by the platform-API so both sides agree on the value.
func APIKeyETag(artifactUUID, name string, updatedAt time.Time) string {
	h := sha256.Sum256([]byte(fmt.Sprintf("%s:%s:%d", artifactUUID, name, updatedAt.UnixNano())))
	var uid uuid.UUID
	copy(uid[:], h[:16])
	uid[6] = (uid[6] & 0x0f) | 0x70 // version = 7
	uid[8] = (uid[8] & 0x3f) | 0x80 // RFC 4122 variant
	return uid.String()
}

// GenerateUUID generates a new UUID v7 string
func GenerateUUID() (string, error) {
	u, err := uuid.NewV7()
	if err != nil {
		return "", fmt.Errorf("failed to generate UUID v7: %w", err)
	}
	return u.String(), nil
}

// GenerateDeterministicUUIDv7 generates a deterministic UUIDv7 from a deployment ID and timestamp.
// The timestamp component uses performedAt (millisecond precision) and the random bits are
// derived from a SHA-256 hash of the deploymentID. This ensures:
//   - Same (deploymentID, performedAt) always produces the same UUID
//   - UUIDs are time-ordered by performedAt (UUIDv7 property)
//   - Different deploymentIDs produce different UUIDs even at the same timestamp
func GenerateDeterministicUUIDv7(deploymentID string, performedAt time.Time) string {
	// Hash the deploymentID to get deterministic random bits
	hash := sha256.Sum256([]byte(deploymentID))

	// UUIDv7 layout (RFC 9562):
	// Octets 0-5:   48-bit Unix timestamp in milliseconds (big-endian)
	// Octet  6:     version (4 bits = 0111) | rand_a (4 bits)
	// Octet  7:     rand_a continued (8 bits)
	// Octet  8:     variant (2 bits = 10) | rand_b (6 bits)
	// Octets 9-15:  rand_b continued (56 bits)
	var b [16]byte

	// Set 48-bit millisecond timestamp
	ms := uint64(performedAt.UnixMilli())
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
