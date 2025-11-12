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

package devportal_client

import (
	"errors"
	"fmt"
)

// DevPortalError represents an error from DevPortal operations
type DevPortalError struct {
	Code       int    // HTTP status code from DevPortal
	Message    string // Human-readable error message
	Retryable  bool   // Whether the error should trigger a retry
	Underlying error  // Underlying error if any
}

// Error implements the error interface for DevPortalError
func (e *DevPortalError) Error() string {
	if e.Code > 0 {
		return fmt.Sprintf("devportal error (%d): %s", e.Code, e.Message)
	}
	return fmt.Sprintf("devportal error: %s", e.Message)
}

// Unwrap returns the underlying error for error unwrapping
func (e *DevPortalError) Unwrap() error {
	return e.Underlying
}

// NewDevPortalError creates a new DevPortalError
func NewDevPortalError(code int, message string, retryable bool, underlying error) *DevPortalError {
	return &DevPortalError{
		Code:       code,
		Message:    message,
		Retryable:  retryable,
		Underlying: underlying,
	}
}

// IsRetryable checks if the error should trigger a retry
func (e *DevPortalError) IsRetryable() bool {
	return e.Retryable
}

// Organization-related errors
var (
	ErrOrganizationAlreadyExists  = errors.New("organization already exists")
	ErrOrganizationNotFound       = errors.New("organization not found")
	ErrOrganizationCreationFailed = errors.New("organization creation failed")
	ErrOrganizationUpdateFailed   = errors.New("organization update failed")
	ErrOrganizationDeletionFailed = errors.New("organization deletion failed")
)

// API-related errors
var (
	ErrAPIAlreadyExists   = errors.New("API already exists")
	ErrAPINotFound        = errors.New("API not found")
	ErrAPICreationFailed  = errors.New("API creation failed")
	ErrAPIUpdateFailed    = errors.New("API update failed")
	ErrAPIDeletionFailed  = errors.New("API deletion failed")
	ErrAPIPublishFailed   = errors.New("API publish failed")
	ErrAPIUnpublishFailed = errors.New("API unpublish failed")
)

// Subscription Policy-related errors
var (
	ErrSubscriptionPolicyAlreadyExists  = errors.New("subscription policy already exists")
	ErrSubscriptionPolicyNotFound       = errors.New("subscription policy not found")
	ErrSubscriptionPolicyCreationFailed = errors.New("subscription policy creation failed")
	ErrSubscriptionPolicyUpdateFailed   = errors.New("subscription policy update failed")
	ErrSubscriptionPolicyDeletionFailed = errors.New("subscription policy deletion failed")
)

// DevPortal connection and general errors
var (
	ErrDevPortalConnectionFailed     = errors.New("devportal connection failed")
	ErrDevPortalTimeout              = errors.New("devportal request timeout")
	ErrDevPortalAuthenticationFailed = errors.New("devportal authentication failed")
	ErrDevPortalInvalidRequest       = errors.New("devportal invalid request")
	ErrDevPortalServerError          = errors.New("devportal server error")
	ErrDevPortalServiceUnavailable   = errors.New("devportal service unavailable")
)

// Template-related errors
var (
	ErrTemplateNotFound       = errors.New("template not found")
	ErrTemplateUploadFailed   = errors.New("template upload failed")
	ErrTemplateUpdateFailed   = errors.New("template update failed")
	ErrTemplateDeletionFailed = errors.New("template deletion failed")
)

// Multipart/form-data errors
var (
	ErrMultipartCreationFailed = errors.New("multipart creation failed")
	ErrFormFieldCreationFailed = errors.New("form field creation failed")
	ErrFileWriteFailed         = errors.New("file write failed")
)

// Label-related errors
var (
	ErrLabelNotFound       = errors.New("label not found")
	ErrLabelCreationFailed = errors.New("label creation failed")
	ErrLabelUpdateFailed   = errors.New("label update failed")
	ErrLabelDeletionFailed = errors.New("label deletion failed")
)

// View-related errors
var (
	ErrViewNotFound       = errors.New("view not found")
	ErrViewCreationFailed = errors.New("view creation failed")
	ErrViewUpdateFailed   = errors.New("view update failed")
	ErrViewDeletionFailed = errors.New("view deletion failed")
)
