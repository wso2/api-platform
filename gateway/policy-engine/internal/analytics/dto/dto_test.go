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

package dto

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// =============================================================================
// API Tests
// =============================================================================

func TestAPI_GetSetAPIID(t *testing.T) {
	api := &API{}
	assert.Equal(t, "", api.GetAPIID())

	api.SetAPIID("api-123")
	assert.Equal(t, "api-123", api.GetAPIID())
}

func TestAPI_GetSetAPIType(t *testing.T) {
	api := &API{}
	assert.Equal(t, "", api.GetAPIType())

	api.SetAPIType("REST")
	assert.Equal(t, "REST", api.GetAPIType())
}

func TestAPI_GetSetAPIName(t *testing.T) {
	api := &API{}
	assert.Equal(t, "", api.GetAPIName())

	api.SetAPIName("PetStore")
	assert.Equal(t, "PetStore", api.GetAPIName())
}

func TestAPI_GetSetAPIVersion(t *testing.T) {
	api := &API{}
	assert.Equal(t, "", api.GetAPIVersion())

	api.SetAPIVersion("v1.0.0")
	assert.Equal(t, "v1.0.0", api.GetAPIVersion())
}

func TestAPI_GetSetAPICreator(t *testing.T) {
	api := &API{}
	assert.Equal(t, "", api.GetAPICreator())

	api.SetAPICreator("admin")
	assert.Equal(t, "admin", api.GetAPICreator())
}

func TestAPI_GetSetAPICreatorTenantDomain(t *testing.T) {
	api := &API{}
	assert.Equal(t, "", api.GetAPICreatorTenantDomain())

	api.SetAPICreatorTenantDomain("carbon.super")
	assert.Equal(t, "carbon.super", api.GetAPICreatorTenantDomain())
}

// =============================================================================
// Application Tests
// =============================================================================

func TestApplication_GetSetKeyType(t *testing.T) {
	app := &Application{}
	assert.Equal(t, "", app.GetKeyType())

	app.SetKeyType("PRODUCTION")
	assert.Equal(t, "PRODUCTION", app.GetKeyType())
}

func TestApplication_GetSetApplicationID(t *testing.T) {
	app := &Application{}
	assert.Equal(t, "", app.GetApplicationID())

	app.SetApplicationID("app-456")
	assert.Equal(t, "app-456", app.GetApplicationID())
}

func TestApplication_GetSetApplicationName(t *testing.T) {
	app := &Application{}
	assert.Equal(t, "", app.GetApplicationName())

	app.SetApplicationName("MyApp")
	assert.Equal(t, "MyApp", app.GetApplicationName())
}

func TestApplication_GetSetApplicationOwner(t *testing.T) {
	app := &Application{}
	assert.Equal(t, "", app.GetApplicationOwner())

	app.SetApplicationOwner("john@example.com")
	assert.Equal(t, "john@example.com", app.GetApplicationOwner())
}

// =============================================================================
// Latencies Tests
// =============================================================================

func TestLatencies_GetSetResponseLatency(t *testing.T) {
	lat := &Latencies{}
	assert.Equal(t, int64(0), lat.GetResponseLatency())

	lat.SetResponseLatency(150)
	assert.Equal(t, int64(150), lat.GetResponseLatency())
}

func TestLatencies_GetSetBackendLatency(t *testing.T) {
	lat := &Latencies{}
	assert.Equal(t, int64(0), lat.GetBackendLatency())

	lat.SetBackendLatency(100)
	assert.Equal(t, int64(100), lat.GetBackendLatency())
}

func TestLatencies_GetSetRequestMediationLatency(t *testing.T) {
	lat := &Latencies{}
	assert.Equal(t, int64(0), lat.GetRequestMediationLatency())

	lat.SetRequestMediationLatency(25)
	assert.Equal(t, int64(25), lat.GetRequestMediationLatency())
}

func TestLatencies_GetSetResponseMediationLatency(t *testing.T) {
	lat := &Latencies{}
	assert.Equal(t, int64(0), lat.GetResponseMediationLatency())

	lat.SetResponseMediationLatency(30)
	assert.Equal(t, int64(30), lat.GetResponseMediationLatency())
}

func TestLatencies_GetSetDuration(t *testing.T) {
	lat := &Latencies{}
	assert.Equal(t, int64(0), lat.GetDuration())

	lat.SetDuration(200)
	assert.Equal(t, int64(200), lat.GetDuration())
}

// =============================================================================
// AIMetadata Tests
// =============================================================================

func TestAIMetadata_GetSetModel(t *testing.T) {
	ai := &AIMetadata{}
	assert.Equal(t, "", ai.GetModel())

	ai.SetModel("gpt-4")
	assert.Equal(t, "gpt-4", ai.GetModel())
}

func TestAIMetadata_GetSetVendorName(t *testing.T) {
	ai := &AIMetadata{}
	assert.Equal(t, "", ai.GetVendorName())

	ai.SetVendorName("OpenAI")
	assert.Equal(t, "OpenAI", ai.GetVendorName())
}

func TestAIMetadata_GetSetVendorVersion(t *testing.T) {
	ai := &AIMetadata{}
	assert.Equal(t, "", ai.GetVendorVersion())

	ai.SetVendorVersion("v1.0.0")
	assert.Equal(t, "v1.0.0", ai.GetVendorVersion())
}

// =============================================================================
// Target Tests
// =============================================================================

func TestTarget_GetSetTargetResponseCode(t *testing.T) {
	target := &Target{}
	assert.Equal(t, 0, target.GetTargetResponseCode())

	target.SetTargetResponseCode(200)
	assert.Equal(t, 200, target.GetTargetResponseCode())
}

func TestTarget_IsSetResponseCacheHit(t *testing.T) {
	target := &Target{}
	assert.False(t, target.IsResponseCacheHit())

	target.SetResponseCacheHit(true)
	assert.True(t, target.IsResponseCacheHit())

	target.SetResponseCacheHit(false)
	assert.False(t, target.IsResponseCacheHit())
}

func TestTarget_GetSetDestination(t *testing.T) {
	target := &Target{}
	assert.Equal(t, "", target.GetDestination())

	target.SetDestination("https://backend.example.com")
	assert.Equal(t, "https://backend.example.com", target.GetDestination())
}

// =============================================================================
// Operation Tests
// =============================================================================

func TestOperation_GetSetAPIMethod(t *testing.T) {
	op := &Operation{}
	assert.Equal(t, "", op.GetAPIMethod())

	op.SetAPIMethod("GET")
	assert.Equal(t, "GET", op.GetAPIMethod())
}

func TestOperation_GetSetAPIResourceTemplate(t *testing.T) {
	op := &Operation{}
	assert.Equal(t, "", op.GetAPIResourceTemplate())

	op.SetAPIResourceTemplate("/pets/{petId}")
	assert.Equal(t, "/pets/{petId}", op.GetAPIResourceTemplate())
}

// =============================================================================
// Event Tests
// =============================================================================

func TestEvent_StructFields(t *testing.T) {
	event := &Event{
		ProxyResponseCode: 200,
		UserAgentHeader:   "Mozilla/5.0",
		UserName:          "testuser",
		UserIP:            "192.168.1.1",
		ErrorType:         "",
		Properties:        map[string]interface{}{"key": "value"},
	}

	assert.Equal(t, 200, event.ProxyResponseCode)
	assert.Equal(t, "Mozilla/5.0", event.UserAgentHeader)
	assert.Equal(t, "testuser", event.UserName)
	assert.Equal(t, "192.168.1.1", event.UserIP)
	assert.Equal(t, "", event.ErrorType)
	assert.Equal(t, "value", event.Properties["key"])
}

func TestEvent_NestedStructs(t *testing.T) {
	api := &ExtendedAPI{}
	api.SetAPIName("TestAPI")

	op := &Operation{}
	op.SetAPIMethod("POST")

	target := &Target{}
	target.SetTargetResponseCode(201)

	app := &Application{}
	app.SetApplicationName("TestApp")

	lat := &Latencies{}
	lat.SetResponseLatency(50)

	event := &Event{
		API:         api,
		Operation:   op,
		Target:      target,
		Application: app,
		Latencies:   lat,
	}

	assert.Equal(t, "TestAPI", event.API.GetAPIName())
	assert.Equal(t, "POST", event.Operation.GetAPIMethod())
	assert.Equal(t, 201, event.Target.GetTargetResponseCode())
	assert.Equal(t, "TestApp", event.Application.GetApplicationName())
	assert.Equal(t, int64(50), event.Latencies.GetResponseLatency())
}

// =============================================================================
// MetaInfo Tests
// =============================================================================

func TestMetaInfo_StructFields(t *testing.T) {
	meta := &MetaInfo{
		CorrelationID: "corr-123",
		RegionID:      "us-west-1",
		GatewayType:   "APK",
	}

	assert.Equal(t, "corr-123", meta.CorrelationID)
	assert.Equal(t, "us-west-1", meta.RegionID)
	assert.Equal(t, "APK", meta.GatewayType)
}

// =============================================================================
// Error Tests
// =============================================================================

func TestError_StructFields(t *testing.T) {
	err := &Error{
		ErrorCode:    401,
		ErrorMessage: AuthenticationFailure,
	}

	assert.Equal(t, 401, err.ErrorCode)
	assert.Equal(t, AuthenticationFailure, err.ErrorMessage)
}

// =============================================================================
// ExtendedAPI Tests
// =============================================================================

func TestExtendedAPI_GetSetOrganizationID(t *testing.T) {
	api := &ExtendedAPI{}
	assert.Equal(t, "", api.GetOrganizationID())

	api.SetOrganizationID("org-123")
	assert.Equal(t, "org-123", api.GetOrganizationID())
}

func TestExtendedAPI_GetSetEnvironmentID(t *testing.T) {
	api := &ExtendedAPI{}
	assert.Equal(t, "", api.GetEnvironmentID())

	api.SetEnvironmentID("env-456")
	assert.Equal(t, "env-456", api.GetEnvironmentID())
}

func TestExtendedAPI_GetSetAPIContext(t *testing.T) {
	api := &ExtendedAPI{}
	assert.Equal(t, "", api.GetAPIContext())

	api.SetAPIContext("/petstore/v1")
	assert.Equal(t, "/petstore/v1", api.GetAPIContext())
}

func TestExtendedAPI_InheritsAPIMethods(t *testing.T) {
	api := &ExtendedAPI{}

	// Test inherited methods from API
	api.SetAPIID("api-789")
	api.SetAPIName("InheritedAPI")
	api.SetAPIVersion("v2.0")
	api.SetAPIType("GraphQL")
	api.SetAPICreator("admin")
	api.SetAPICreatorTenantDomain("tenant1")

	assert.Equal(t, "api-789", api.GetAPIID())
	assert.Equal(t, "InheritedAPI", api.GetAPIName())
	assert.Equal(t, "v2.0", api.GetAPIVersion())
	assert.Equal(t, "GraphQL", api.GetAPIType())
	assert.Equal(t, "admin", api.GetAPICreator())
	assert.Equal(t, "tenant1", api.GetAPICreatorTenantDomain())
}

// =============================================================================
// FaultSubCategory Tests
// =============================================================================

func TestFaultSubCategory_Constants(t *testing.T) {
	// Test Other category subcategories
	assert.Equal(t, FaultSubCategory("MEDIATION_ERROR"), OtherMediationError)
	assert.Equal(t, FaultSubCategory("RESOURCE_NOT_FOUND"), OtherResourceNotFound)
	assert.Equal(t, FaultSubCategory("METHOD_NOT_ALLOWED"), OtherMethodNotAllowed)
	assert.Equal(t, FaultSubCategory("UNCLASSIFIED"), OtherUnclassified)

	// Test Throttling subcategories
	assert.Equal(t, FaultSubCategory("API_LEVEL_LIMIT_EXCEEDED"), ThrottlingAPILimitExceeded)
	assert.Equal(t, FaultSubCategory("HARD_LIMIT_EXCEEDED"), ThrottlingHardLimitExceeded)
	assert.Equal(t, FaultSubCategory("APPLICATION_LEVEL_LIMIT_EXCEEDED"), ThrottlingApplicationLimitExceeded)
	assert.Equal(t, FaultSubCategory("SUBSCRIPTION_LIMIT_EXCEEDED"), ThrottlingSubscriptionLimitExceeded)
	assert.Equal(t, FaultSubCategory("BLOCKED"), ThrottlingBlocked)

	// Test Authentication subcategories
	assert.Equal(t, FaultSubCategory("AUTHENTICATION_FAILURE"), AuthenticationFailure)
	assert.Equal(t, FaultSubCategory("AUTHORIZATION_FAILURE"), AuthenticationAuthorizationFailure)
	assert.Equal(t, FaultSubCategory("SUBSCRIPTION_VALIDATION_FAILURE"), AuthenticationSubscriptionValidationFailure)

	// Test TargetConnectivity subcategories
	assert.Equal(t, FaultSubCategory("CONNECTION_TIMEOUT"), TargetConnectivityConnectionTimeout)
	assert.Equal(t, FaultSubCategory("CONNECTION_SUSPENDED"), TargetConnectivityConnectionSuspended)
}
