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
	"errors"
	"fmt"
	"net/http"
	"strings"

	"platform-api/src/internal/constants"

	"github.com/go-playground/validator/v10"
)

// makeError creates a standardized error response tuple
func makeError(status int, message string) (int, interface{}) {
	return status, NewErrorResponse(status, http.StatusText(status), message)
}

// FormatValidationError converts validator errors to user-friendly messages (public API)
func FormatValidationError(err error) string {
	var validationErrors validator.ValidationErrors
	if !errors.As(err, &validationErrors) {
		return err.Error() // Not a validation error, return as-is
	}
	return formatValidationError(validationErrors)
}

// formatValidationError converts ValidationErrors to user-friendly messages (internal)
func formatValidationError(validationErrors validator.ValidationErrors) string {
	var messages []string
	for _, fieldError := range validationErrors {
		fieldName := getUserFriendlyFieldName(fieldError.Field())
		message := getValidationErrorMessage(fieldName, fieldError.Tag(), fieldError.Param())
		messages = append(messages, message)
	}
	return strings.Join(messages, "; ")
}

// getUserFriendlyFieldName maps struct field names to user-friendly field names
func getUserFriendlyFieldName(fieldName string) string {
	fieldMap := map[string]string{
		"Name":           "name",
		"APIUrl":         "API URL",
		"Hostname":       "hostname",
		"APIKey":         "API key",
		"HeaderKeyName":  "header key name",
		"Visibility":     "visibility",
		"Description":    "description",
		"Identifier":     "identifier",
		"APIID":          "API ID",
		"Provider":       "provider",
		"APIName":        "API name",
		"APIHandle":      "API handle",
		"APIDescription": "API description",
		"APIVersion":     "API version",
		"APIType":        "API type",
		"APIStatus":      "API status",
		"ProductionURL":  "production URL",
	}

	if friendly, exists := fieldMap[fieldName]; exists {
		return friendly
	}
	return strings.ToLower(fieldName)
}

// getValidationErrorMessage creates user-friendly validation error messages
func getValidationErrorMessage(fieldName, tag, param string) string {
	switch tag {
	case "required":
		return fmt.Sprintf("%s is required", fieldName)
	case "min":
		return fmt.Sprintf("%s must be at least %s characters long", fieldName, param)
	case "max":
		return fmt.Sprintf("%s must not exceed %s characters", fieldName, param)
	case "url":
		return fmt.Sprintf("%s must be a valid URL", fieldName)
	case "hostname":
		return fmt.Sprintf("%s must be a valid hostname", fieldName)
	case "oneof":
		return fmt.Sprintf("%s must be one of: %s", fieldName, strings.ReplaceAll(param, " ", ", "))
	default:
		return fmt.Sprintf("%s is invalid", fieldName)
	}
}

// GetErrorResponse maps domain errors and validation errors to HTTP status and error response
func GetErrorResponse(err error) (int, interface{}) {
	// First check if it's a validation error
	var validationErrors validator.ValidationErrors
	if errors.As(err, &validationErrors) {
		userFriendlyMessage := formatValidationError(validationErrors)
		return makeError(http.StatusBadRequest, userFriendlyMessage)
	}

	// Handle domain/business logic errors
	switch {
	// DevPortal errors
	case errors.Is(err, constants.ErrDevPortalNotFound):
		return makeError(http.StatusNotFound, "DevPortal not found")
	case errors.Is(err, constants.ErrDevPortalIdentifierExists):
		return makeError(http.StatusConflict, "DevPortal with this identifier already exists for this organization")
	case errors.Is(err, constants.ErrDevPortalAPIUrlExists):
		return makeError(http.StatusConflict, "DevPortal with this API URL already exists for this organization")
	case errors.Is(err, constants.ErrDevPortalHostnameExists):
		return makeError(http.StatusConflict, "DevPortal with this hostname already exists for this organization")
	case errors.Is(err, constants.ErrDevPortalDefaultAlreadyExists):
		return makeError(http.StatusConflict, "Default DevPortal already exists for this organization")
	case errors.Is(err, constants.ErrDevPortalCannotDeleteDefault):
		return makeError(http.StatusBadRequest, "Cannot delete default DevPortal")
	case errors.Is(err, constants.ErrDevPortalCannotDeactivateDefault):
		return makeError(http.StatusBadRequest, "Cannot deactivate default DevPortal")
	case errors.Is(err, constants.ErrDevPortalBackendUnreachable):
		return makeError(http.StatusServiceUnavailable, "DevPortal backend is currently unreachable. Please try again later or contact administrator.")
	case errors.Is(err, constants.ErrDevPortalSyncFailed):
		return makeError(http.StatusBadGateway, "Failed to sync organization to DevPortal. Please check DevPortal configuration.")
	case errors.Is(err, constants.ErrDevPortalAuthenticationFailed):
		return makeError(http.StatusUnauthorized, "DevPortal authentication failed. Please check API key and try again.")
	case errors.Is(err, constants.ErrDevPortalForbidden):
		return makeError(http.StatusForbidden, "DevPortal access forbidden. Please check permissions.")
	case errors.Is(err, constants.ErrDevPortalConnectivityFailed):
		return makeError(http.StatusBadRequest, "DevPortal endpoint is not valid or reachable. Please check the API URL and try again.")
	case errors.Is(err, constants.ErrDevPortalNameRequired):
		return makeError(http.StatusBadRequest, "DevPortal name is required")
	case errors.Is(err, constants.ErrDevPortalIdentifierRequired):
		return makeError(http.StatusBadRequest, "DevPortal identifier is required")
	case errors.Is(err, constants.ErrDevPortalAPIUrlRequired):
		return makeError(http.StatusBadRequest, "DevPortal API URL is required")
	case errors.Is(err, constants.ErrDevPortalHostnameRequired):
		return makeError(http.StatusBadRequest, "DevPortal hostname is required")
	case errors.Is(err, constants.ErrDevPortalAPIKeyRequired):
		return makeError(http.StatusBadRequest, "DevPortal API key is required")
	case errors.Is(err, constants.ErrDevPortalHeaderKeyNameRequired):
		return makeError(http.StatusBadRequest, "DevPortal header key name is required")
	case errors.Is(err, constants.ErrDevPortalAlreadyExists):
		return makeError(http.StatusConflict, "DevPortal with these attributes already exists for this organization")
	case errors.Is(err, constants.ErrDevPortalOrganizationConflict):
		return makeError(http.StatusConflict, "Organization conflict in DevPortal: an organization with the same organization ID exists in devportal, but differs from the one being synced")

	// Organization errors
	case errors.Is(err, constants.ErrOrganizationNotFound):
		return makeError(http.StatusNotFound, "Organization not found")
	case errors.Is(err, constants.ErrOrganizationExists):
		return makeError(http.StatusConflict, "Organization already exists with the given UUID")
	case errors.Is(err, constants.ErrInvalidHandle):
		return makeError(http.StatusBadRequest, "Invalid handle format")
	case errors.Is(err, constants.ErrMultipleOrganizations):
		return makeError(http.StatusInternalServerError, "Multiple organizations found")

	// Project errors
	case errors.Is(err, constants.ErrProjectExists):
		return makeError(http.StatusConflict, "Project already exists in organization")
	case errors.Is(err, constants.ErrProjectNotFound):
		return makeError(http.StatusNotFound, "Project not found")
	case errors.Is(err, constants.ErrInvalidProjectName):
		return makeError(http.StatusBadRequest, "Invalid project name")
	case errors.Is(err, constants.ErrOrganizationMustHAveAtLeastOneProject):
		return makeError(http.StatusBadRequest, "Organization must have at least one project")
	case errors.Is(err, constants.ErrProjectHasAssociatedAPIs):
		return makeError(http.StatusBadRequest, "Project has associated APIs")

	// API errors
	case errors.Is(err, constants.ErrAPINotFound):
		return makeError(http.StatusNotFound, "API not found")
	case errors.Is(err, constants.ErrAPIAlreadyExists):
		return makeError(http.StatusConflict, "API already exists in project")
	case errors.Is(err, constants.ErrInvalidAPIContext):
		return makeError(http.StatusBadRequest, "Invalid API context format")
	case errors.Is(err, constants.ErrInvalidAPIVersion):
		return makeError(http.StatusBadRequest, "Invalid API version format")
	case errors.Is(err, constants.ErrInvalidAPIName):
		return makeError(http.StatusBadRequest, "Invalid API name format")
	case errors.Is(err, constants.ErrInvalidLifecycleState):
		return makeError(http.StatusBadRequest, "Invalid lifecycle state")
	case errors.Is(err, constants.ErrInvalidAPIType):
		return makeError(http.StatusBadRequest, "Invalid API type")
	case errors.Is(err, constants.ErrInvalidTransport):
		return makeError(http.StatusBadRequest, "Invalid transport protocol")
	case errors.Is(err, constants.ErrInvalidAPIDeployment):
		return makeError(http.StatusBadRequest, "Invalid API deployment")

	// Gateway errors
	case errors.Is(err, constants.ErrGatewayNotFound):
		return makeError(http.StatusNotFound, "Gateway not found")

	// DevPortal sync errors
	case errors.Is(err, constants.ErrApiPortalSync):
		return makeError(http.StatusBadGateway, "Failed to synchronize with Dev portal")

	// API Publication errors
	case errors.Is(err, constants.ErrAPIPublicationNotFound):
		return makeError(http.StatusNotFound, "API publication not found")

	// Default case for unknown errors
	default:
		return makeError(http.StatusInternalServerError, "Internal Server Error")
	}
}
