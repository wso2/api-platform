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
)

var (
	// ErrNotFound is returned when a certificate does not exist.
	ErrNotFound = errors.New("certificate not found")

	// ErrCertStoreNotConfigured is returned when the router was started without
	// router.upstream.tls.customCertsPath, so there is no cert store to reload.
	ErrCertStoreNotConfigured = errors.New("certificate store not configured")

	// ErrMissingFields is returned when name or certificate is blank.
	ErrMissingFields = errors.New("name and certificate are required fields")

	// ErrIDGeneration is returned when a certificate ID could not be minted.
	ErrIDGeneration = errors.New("failed to generate certificate ID")
)

// Persist operation names carried by PersistError.
const (
	OpSave   = "save"
	OpDelete = "delete"
	OpList   = "list"
	OpLoad   = "load"
)

// SyncStage identifies which half of the cert-store sync failed.
type SyncStage string

const (
	// StageReload is a failure to re-read the trust store from the database.
	StageReload SyncStage = "reload"
	// StageSnapshot is a failure to push the reloaded bundle to the router.
	StageSnapshot SyncStage = "snapshot"
)

// InvalidCertificateError means the supplied PEM data is not a usable
// certificate chain.
type InvalidCertificateError struct {
	Cause error
}

func (e *InvalidCertificateError) Error() string {
	return fmt.Sprintf("invalid certificate: %v", e.Cause)
}

func (e *InvalidCertificateError) Unwrap() error { return e.Cause }

// MetadataError means the PEM decoded but x509 metadata could not be extracted.
type MetadataError struct {
	Cause error
}

func (e *MetadataError) Error() string {
	return fmt.Sprintf("failed to parse certificate metadata: %v", e.Cause)
}

func (e *MetadataError) Unwrap() error { return e.Cause }

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

// SyncError reports a failure to propagate a committed database change to the
// router. Persisted is true whenever the database write already succeeded, so a
// caller knows the certificate table and the router have diverged.
type SyncError struct {
	Stage     SyncStage
	Persisted bool
	Cause     error
}

func (e *SyncError) Error() string {
	return fmt.Sprintf("certificate store %s failed: %v", e.Stage, e.Cause)
}

func (e *SyncError) Unwrap() error { return e.Cause }
