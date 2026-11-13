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

package certificate

import (
	"errors"
	"fmt"
	"strings"
)

var (
	// ErrNotFound is returned when a certificate does not exist.
	ErrNotFound = errors.New("certificate not found")

	// ErrCertStoreNotConfigured is returned when there is no certificate store
	// or snapshot manager to sync with. The controller always builds both, so
	// in production this cannot happen; it means a hand-constructed server
	// without an xDS layer, as in tests.
	ErrCertStoreNotConfigured = errors.New("certificate store not configured")

	// ErrMissingID is returned when a delete names no certificate.
	ErrMissingID = errors.New("certificate ID is required")

	// ErrIDGeneration is returned when a certificate ID could not be minted.
	ErrIDGeneration = errors.New("failed to generate certificate ID")

	// ErrIdentityStorageUnavailable is returned when a gateway identity is
	// uploaded or rotated with no encryption provider configured, so its
	// private key is never stored in clear.
	ErrIdentityStorageUnavailable = errors.New("gateway identity storage is not available")
)

// Top-level messages of a ValidationError, as the REST API reports them.
const (
	MsgUploadInvalid      = "certificate upload is invalid"
	MsgInvalidUsageFilter = "invalid usage filter"
)

// Persist operation names carried by PersistError.
const (
	OpSave   = "save"
	OpUpdate = "update"
	OpDelete = "delete"
	OpList   = "list"
	OpLoad   = "load"
)

// SyncStage identifies which part of propagating a change failed.
type SyncStage string

const (
	// StageReload is a failure to re-read the certificate store.
	StageReload SyncStage = "reload"
	// StageSnapshot is a failure to push the store to the router over SDS.
	StageSnapshot SyncStage = "snapshot"
	// StagePolicyEngine is a failure to republish the client certificate
	// authority pool to the policy engine.
	StagePolicyEngine SyncStage = "policy-engine"
)

// FieldError is one request-level problem, named by the field it is about.
type FieldError struct {
	Field   string
	Message string
}

// ValidationError reports every problem found in a request at once.
type ValidationError struct {
	Message string
	Fields  []FieldError
}

func (e *ValidationError) Error() string {
	parts := make([]string, 0, len(e.Fields))
	for _, f := range e.Fields {
		parts = append(parts, f.Field+": "+f.Message)
	}
	return fmt.Sprintf("%s: %s", e.Message, strings.Join(parts, "; "))
}

// DefaultIdentityExistsError is returned when a second role: default gateway
// identity is uploaded.
type DefaultIdentityExistsError struct {
	Name string
}

func (e *DefaultIdentityExistsError) Error() string {
	return fmt.Sprintf("gateway identity %s already has role: default; delete it before uploading another default identity", e.Name)
}

// EncryptionError wraps a failure to encrypt a gateway identity's private key.
type EncryptionError struct {
	Cause error
}

func (e *EncryptionError) Error() string {
	return fmt.Sprintf("failed to encrypt private key: %v", e.Cause)
}

func (e *EncryptionError) Unwrap() error { return e.Cause }

// Reference is one deployed API's use of a certificate.
type Reference struct {
	Field string // e.g. spec.upstreamDefinitions[0].tls.trustedCAs[0]
	API   string // the API's handle
}

// InUseError is returned when a delete is refused because deployed APIs still
// depend on the certificate.
type InUseError struct {
	Message    string
	References []Reference
}

func (e *InUseError) Error() string { return e.Message }

// ReferenceCheckError is returned when a delete is refused because the
// deployed configurations could not be read to check for references.
type ReferenceCheckError struct {
	Message string
	Cause   error
}

func (e *ReferenceCheckError) Error() string { return fmt.Sprintf("%s: %v", e.Message, e.Cause) }

func (e *ReferenceCheckError) Unwrap() error { return e.Cause }

// PersistError wraps a storage failure and names the operation that failed, so
// a caller can tell a save failure from an ID-generation failure — both are
// internal errors but they are reported differently.
type PersistError struct {
	Op    string
	Cause error
}

func (e *PersistError) Error() string {
	return fmt.Sprintf("certificate %s failed: %v", e.Op, e.Cause)
}

func (e *PersistError) Unwrap() error { return e.Cause }

// SyncError reports a failure to propagate a change to the router or the
// policy engine. Persisted is true whenever the database write already
// succeeded, so a caller knows the certificate table and the data plane have
// diverged.
type SyncError struct {
	Stage     SyncStage
	Persisted bool
	Cause     error
}

func (e *SyncError) Error() string {
	return fmt.Sprintf("certificate store %s failed: %v", e.Stage, e.Cause)
}

func (e *SyncError) Unwrap() error { return e.Cause }
