package apikey

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	policy "github.com/wso2/api-platform/sdk/gateway/policy/v1alpha"
)

// Helper function to generate realistic 32-byte random API key with apip_ prefix
func generateRealistic32ByteAPIKey() string {
	// Generate 32 random bytes (as per constants.APIKeyLen = 32)
	randomBytes := make([]byte, 32)
	_, err := rand.Read(randomBytes)
	if err != nil {
		return ""
	}
	return "apip_" + hex.EncodeToString(randomBytes)
}

// Helper function to generate 22-character URL-safe unique identifier (matches new implementation)
func generateShortUniqueID() string {
	// Generate 16 random bytes (128 bits of entropy)
	randomBytes := make([]byte, 16)
	_, err := rand.Read(randomBytes)
	if err != nil {
		return "defaulttestid123456" // fallback for tests
	}

	// Encode as base64url without padding and replace underscores with tildes
	id := base64.RawURLEncoding.EncodeToString(randomBytes)
	id = strings.ReplaceAll(id, "_", "~")

	return id
}

// Helper function to hash API key using SHA256 with salt (mimicking the real implementation)
func hashAPIKeyWithSHA256(plainAPIKey string) string {
	// Generate a fixed salt for testing (in real implementation this would be random)
	salt := []byte("testsalt12345678") // 16 bytes fixed salt for predictable tests

	// Generate hash using SHA-256
	hasher := sha256.New()
	hasher.Write([]byte(plainAPIKey))
	hasher.Write(salt)
	hash := hasher.Sum(nil)

	// Encode salt and hash using hex
	saltHex := hex.EncodeToString(salt)
	hashHex := hex.EncodeToString(hash)

	// Format: $sha256$<salt_hex>$<hash_hex>
	return fmt.Sprintf("$sha256$%s$%s", saltHex, hashHex)
}

// TestAPIKeyPolicy_ValidKeyInHeader tests successful API key authentication from header
func TestAPIKeyPolicy_ValidKeyInHeader(t *testing.T) {
	// Generate realistic API key (apip_ + 64 hex chars)
	plainAPIKey := generateRealistic32ByteAPIKey()

	// Create API key with new short ID format (22 characters, no underscores)
	id := generateShortUniqueID()
	// Setup mock API key store with valid key
	setupValidAPIKeyStore(t, "test-api-id", plainAPIKey, id)

	// Create request context with valid API key in header using consistent helper
	apiKeyValue := createTestAPIKeyValue(plainAPIKey, id)
	headers := map[string][]string{
		"X-API-Key": {apiKeyValue},
	}
	ctx := createMockRequestContext("/api/test", headers)
	ctx.APIId = "test-api-id"
	ctx.APIName = "test-api"
	ctx.APIVersion = "v1.0.0"
	ctx.OperationPath = "/test"
	ctx.Method = "GET"

	// Create params for header-based API key authentication
	params := map[string]interface{}{
		"key": "X-API-Key",
		"in":  "header",
	}

	p, err := GetPolicy(policy.PolicyMetadata{}, params)
	if err != nil {
		t.Fatalf("Failed to create policy: %v", err)
	}

	// Execute policy
	action := p.OnRequest(ctx, params)

	// Verify successful authentication
	if ctx.Metadata[MetadataKeyAuthSuccess] != true {
		t.Errorf("Expected auth.success to be true, got %v", ctx.Metadata[MetadataKeyAuthSuccess])
	}

	if ctx.Metadata[MetadataKeyAuthMethod] != "api-key" {
		t.Errorf("Expected auth.method to be 'api-key', got %v", ctx.Metadata[MetadataKeyAuthMethod])
	}

	// Verify it's an UpstreamRequestModifications action
	_, ok := action.(policy.UpstreamRequestModifications)
	if !ok {
		t.Fatalf("Expected UpstreamRequestModifications, got %T", action)
	}
}

// TestAPIKeyPolicy_ValidKeyInQuery tests successful API key authentication from query parameter
func TestAPIKeyPolicy_ValidKeyInQuery(t *testing.T) {
	// Generate realistic API key (apip_ + 64 hex chars)
	plainAPIKey := generateRealistic32ByteAPIKey()

	// Create request context with valid API key in query parameter
	id := generateShortUniqueID()
	// Setup mock API key store with valid key
	setupValidAPIKeyStore(t, "test-api-id", plainAPIKey, id)

	// Create request context with valid API key in header using consistent helper
	apiKeyValue := createTestAPIKeyValue(plainAPIKey, id)
	ctx := createMockRequestContext("/api/test?api_key="+apiKeyValue+"&other=value", map[string][]string{})
	ctx.APIId = "test-api-id"
	ctx.APIName = "test-api"
	ctx.APIVersion = "v1.0.0"
	ctx.OperationPath = "/test"
	ctx.Method = "GET"

	// Create params for query-based API key authentication
	params := map[string]interface{}{
		"key": "api_key",
		"in":  "query",
	}

	p, err := GetPolicy(policy.PolicyMetadata{}, params)
	if err != nil {
		t.Fatalf("Failed to create policy: %v", err)
	}

	// Execute policy
	action := p.OnRequest(ctx, params)

	// Verify successful authentication
	if ctx.Metadata[MetadataKeyAuthSuccess] != true {
		t.Errorf("Expected auth.success to be true, got %v", ctx.Metadata[MetadataKeyAuthSuccess])
	}

	if ctx.Metadata[MetadataKeyAuthMethod] != "api-key" {
		t.Errorf("Expected auth.method to be 'api-key', got %v", ctx.Metadata[MetadataKeyAuthMethod])
	}

	// Verify it's an UpstreamRequestModifications action
	_, ok := action.(policy.UpstreamRequestModifications)
	if !ok {
		t.Fatalf("Expected UpstreamRequestModifications, got %T", action)
	}
}

// TestAPIKeyPolicy_ValidKeyWithPrefix tests successful API key authentication with prefix stripping
func TestAPIKeyPolicy_ValidKeyWithPrefix(t *testing.T) {
	// Generate realistic API key (apip_ + 64 hex chars)
	plainAPIKey := generateRealistic32ByteAPIKey()

	// Create request context with valid API key in query parameter
	id := generateShortUniqueID()
	// Setup mock API key store with valid key
	setupValidAPIKeyStore(t, "test-api-id", plainAPIKey, id)

	// Create request context with valid API key in header using consistent helper
	apiKeyValue := createTestAPIKeyValue(plainAPIKey, id)
	headers := map[string][]string{
		"Authorization": {"Bearer " + apiKeyValue},
	}
	ctx := createMockRequestContext("/api/test", headers)
	ctx.APIId = "test-api-id"
	ctx.APIName = "test-api"
	ctx.APIVersion = "v1.0.0"
	ctx.OperationPath = "/test"
	ctx.Method = "GET"

	// Create params with prefix stripping
	params := map[string]interface{}{
		"key":          "Authorization",
		"in":           "header",
		"value-prefix": "Bearer ",
	}

	p, err := GetPolicy(policy.PolicyMetadata{}, params)
	if err != nil {
		t.Fatalf("Failed to create policy: %v", err)
	}

	// Execute policy
	action := p.OnRequest(ctx, params)

	// Verify successful authentication
	if ctx.Metadata[MetadataKeyAuthSuccess] != true {
		t.Errorf("Expected auth.success to be true, got %v", ctx.Metadata[MetadataKeyAuthSuccess])
	}

	if ctx.Metadata[MetadataKeyAuthMethod] != "api-key" {
		t.Errorf("Expected auth.method to be 'api-key', got %v", ctx.Metadata[MetadataKeyAuthMethod])
	}

	// Verify it's an UpstreamRequestModifications action
	_, ok := action.(policy.UpstreamRequestModifications)
	if !ok {
		t.Fatalf("Expected UpstreamRequestModifications, got %T", action)
	}
}

// TestAPIKeyPolicy_MissingKeyParameter tests failure when 'key' parameter is missing
func TestAPIKeyPolicy_MissingKeyParameter(t *testing.T) {
	ctx := createMockRequestContext("/api/test", map[string][]string{})
	ctx.APIId = "test-api-id"
	ctx.APIName = "test-api"
	ctx.APIVersion = "v1.0.0"
	ctx.OperationPath = "/test"
	ctx.Method = "GET"

	// Create params without 'key' parameter
	params := map[string]interface{}{
		"in": "header",
	}

	p, err := GetPolicy(policy.PolicyMetadata{}, params)
	if err != nil {
		t.Fatalf("Failed to create policy: %v", err)
	}

	action := p.OnRequest(ctx, params)

	// Verify authentication failed
	if ctx.Metadata[MetadataKeyAuthSuccess] != false {
		t.Errorf("Expected auth.success to be false, got %v", ctx.Metadata[MetadataKeyAuthSuccess])
	}

	// Verify it's an ImmediateResponse
	response, ok := action.(policy.ImmediateResponse)
	if !ok {
		t.Fatalf("Expected ImmediateResponse, got %T", action)
	}

	if response.StatusCode != 401 {
		t.Errorf("Expected status code 401, got %d", response.StatusCode)
	}
}

// TestAPIKeyPolicy_MissingInParameter tests failure when 'in' parameter is missing
func TestAPIKeyPolicy_MissingInParameter(t *testing.T) {
	ctx := createMockRequestContext("/api/test", map[string][]string{})
	ctx.APIId = "test-api-id"
	ctx.APIName = "test-api"
	ctx.APIVersion = "v1.0.0"
	ctx.OperationPath = "/test"
	ctx.Method = "GET"

	// Create params without 'in' parameter
	params := map[string]interface{}{
		"key": "X-API-Key",
	}

	p, err := GetPolicy(policy.PolicyMetadata{}, params)
	if err != nil {
		t.Fatalf("Failed to create policy: %v", err)
	}

	action := p.OnRequest(ctx, params)

	// Verify authentication failed
	if ctx.Metadata[MetadataKeyAuthSuccess] != false {
		t.Errorf("Expected auth.success to be false, got %v", ctx.Metadata[MetadataKeyAuthSuccess])
	}

	// Verify it's an ImmediateResponse
	response, ok := action.(policy.ImmediateResponse)
	if !ok {
		t.Fatalf("Expected ImmediateResponse, got %T", action)
	}

	if response.StatusCode != 401 {
		t.Errorf("Expected status code 401, got %d", response.StatusCode)
	}
}

// TestAPIKeyPolicy_MissingKeyInHeader tests failure when API key header is missing
func TestAPIKeyPolicy_MissingKeyInHeader(t *testing.T) {
	ctx := createMockRequestContext("/api/test", map[string][]string{})
	ctx.APIId = "test-api-id"
	ctx.APIName = "test-api"
	ctx.APIVersion = "v1.0.0"
	ctx.OperationPath = "/test"
	ctx.Method = "GET"

	// Create params for header-based API key authentication
	params := map[string]interface{}{
		"key": "X-API-Key",
		"in":  "header",
	}

	p, err := GetPolicy(policy.PolicyMetadata{}, params)
	if err != nil {
		t.Fatalf("Failed to create policy: %v", err)
	}

	action := p.OnRequest(ctx, params)

	// Verify authentication failed
	if ctx.Metadata[MetadataKeyAuthSuccess] != false {
		t.Errorf("Expected auth.success to be false, got %v", ctx.Metadata[MetadataKeyAuthSuccess])
	}

	// Verify it's an ImmediateResponse
	response, ok := action.(policy.ImmediateResponse)
	if !ok {
		t.Fatalf("Expected ImmediateResponse, got %T", action)
	}

	if response.StatusCode != 401 {
		t.Errorf("Expected status code 401, got %d", response.StatusCode)
	}
}

// TestAPIKeyPolicy_MissingKeyInQuery tests failure when API key query parameter is missing
func TestAPIKeyPolicy_MissingKeyInQuery(t *testing.T) {
	ctx := createMockRequestContext("/api/test?other=value", map[string][]string{})
	ctx.APIId = "test-api-id"
	ctx.APIName = "test-api"
	ctx.APIVersion = "v1.0.0"
	ctx.OperationPath = "/test"
	ctx.Method = "GET"

	// Create params for query-based API key authentication
	params := map[string]interface{}{
		"key": "api_key",
		"in":  "query",
	}

	p, err := GetPolicy(policy.PolicyMetadata{}, params)
	if err != nil {
		t.Fatalf("Failed to create policy: %v", err)
	}

	action := p.OnRequest(ctx, params)

	// Verify authentication failed
	if ctx.Metadata[MetadataKeyAuthSuccess] != false {
		t.Errorf("Expected auth.success to be false, got %v", ctx.Metadata[MetadataKeyAuthSuccess])
	}

	// Verify it's an ImmediateResponse
	response, ok := action.(policy.ImmediateResponse)
	if !ok {
		t.Fatalf("Expected ImmediateResponse, got %T", action)
	}

	if response.StatusCode != 401 {
		t.Errorf("Expected status code 401, got %d", response.StatusCode)
	}
}

// TestAPIKeyPolicy_EmptyKeyAfterPrefixRemoval tests failure when key becomes empty after prefix removal
func TestAPIKeyPolicy_EmptyKeyAfterPrefixRemoval(t *testing.T) {
	headers := map[string][]string{
		"Authorization": {"Bearer "},
	}
	ctx := createMockRequestContext("/api/test", headers)
	ctx.APIId = "test-api-id"
	ctx.APIName = "test-api"
	ctx.APIVersion = "v1.0.0"
	ctx.OperationPath = "/test"
	ctx.Method = "GET"

	// Create params with prefix stripping
	params := map[string]interface{}{
		"key":          "Authorization",
		"in":           "header",
		"value-prefix": "Bearer ",
	}

	p, err := GetPolicy(policy.PolicyMetadata{}, params)
	if err != nil {
		t.Fatalf("Failed to create policy: %v", err)
	}

	action := p.OnRequest(ctx, params)

	// Verify authentication failed
	if ctx.Metadata[MetadataKeyAuthSuccess] != false {
		t.Errorf("Expected auth.success to be false, got %v", ctx.Metadata[MetadataKeyAuthSuccess])
	}

	// Verify it's an ImmediateResponse
	response, ok := action.(policy.ImmediateResponse)
	if !ok {
		t.Fatalf("Expected ImmediateResponse, got %T", action)
	}

	if response.StatusCode != 401 {
		t.Errorf("Expected status code 401, got %d", response.StatusCode)
	}
}

// TestAPIKeyPolicy_WrongPrefix tests failure when prefix doesn't match
func TestAPIKeyPolicy_WrongPrefix(t *testing.T) {
	// Generate realistic API key (apip_ + 64 hex chars)
	plainAPIKey := generateRealistic32ByteAPIKey()
	shortID := generateShortUniqueID()
	// Create request context with valid API key in header using consistent helper
	apiKeyValue := createTestAPIKeyValue(plainAPIKey, shortID)
	headers := map[string][]string{
		"Authorization": {"Basic " + apiKeyValue},
	}
	ctx := createMockRequestContext("/api/test", headers)
	ctx.APIId = "test-api-id"
	ctx.APIName = "test-api"
	ctx.APIVersion = "v1.0.0"
	ctx.OperationPath = "/test"
	ctx.Method = "GET"

	// Create params expecting "Bearer " prefix
	params := map[string]interface{}{
		"key":          "Authorization",
		"in":           "header",
		"value-prefix": "Bearer ",
	}

	p, err := GetPolicy(policy.PolicyMetadata{}, params)
	if err != nil {
		t.Fatalf("Failed to create policy: %v", err)
	}

	action := p.OnRequest(ctx, params)

	// Verify authentication failed
	if ctx.Metadata[MetadataKeyAuthSuccess] != false {
		t.Errorf("Expected auth.success to be false, got %v", ctx.Metadata[MetadataKeyAuthSuccess])
	}

	// Verify it's an ImmediateResponse
	response, ok := action.(policy.ImmediateResponse)
	if !ok {
		t.Fatalf("Expected ImmediateResponse, got %T", action)
	}

	if response.StatusCode != 401 {
		t.Errorf("Expected status code 401, got %d", response.StatusCode)
	}
}

// TestAPIKeyPolicy_InvalidAPIKey tests failure with invalid API key
func TestAPIKeyPolicy_InvalidAPIKey(t *testing.T) {
	// Setup mock API key store with invalid key (validation will fail)
	setupInvalidAPIKeyStore()

	headers := map[string][]string{
		"X-API-Key": {"invalid-key-123.test-id"},
	}
	ctx := createMockRequestContext("/api/test", headers)
	ctx.APIId = "test-api-id"
	ctx.APIName = "test-api"
	ctx.APIVersion = "v1.0.0"
	ctx.OperationPath = "/test"
	ctx.Method = "GET"

	// Create params for header-based API key authentication
	params := map[string]interface{}{
		"key": "X-API-Key",
		"in":  "header",
	}

	p, err := GetPolicy(policy.PolicyMetadata{}, params)
	if err != nil {
		t.Fatalf("Failed to create policy: %v", err)
	}

	action := p.OnRequest(ctx, params)

	// Verify authentication failed
	if ctx.Metadata[MetadataKeyAuthSuccess] != false {
		t.Errorf("Expected auth.success to be false, got %v", ctx.Metadata[MetadataKeyAuthSuccess])
	}

	// Verify it's an ImmediateResponse
	response, ok := action.(policy.ImmediateResponse)
	if !ok {
		t.Fatalf("Expected ImmediateResponse, got %T", action)
	}

	if response.StatusCode != 401 {
		t.Errorf("Expected status code 401, got %d", response.StatusCode)
	}
}

// TestAPIKeyPolicy_MissingAPIDetails tests failure when API details are missing
func TestAPIKeyPolicy_MissingAPIDetails(t *testing.T) {
	headers := map[string][]string{
		"X-API-Key": {"test-key-123.test-id"},
	}
	ctx := createMockRequestContext("/api/test", headers)
	// Missing API details

	params := map[string]interface{}{
		"key": "X-API-Key",
		"in":  "header",
	}

	p, err := GetPolicy(policy.PolicyMetadata{}, params)
	if err != nil {
		t.Fatalf("Failed to create policy: %v", err)
	}

	action := p.OnRequest(ctx, params)

	// Verify authentication failed
	if ctx.Metadata[MetadataKeyAuthSuccess] != false {
		t.Errorf("Expected auth.success to be false, got %v", ctx.Metadata[MetadataKeyAuthSuccess])
	}

	// Verify it's an ImmediateResponse
	response, ok := action.(policy.ImmediateResponse)
	if !ok {
		t.Fatalf("Expected ImmediateResponse, got %T", action)
	}

	if response.StatusCode != 401 {
		t.Errorf("Expected status code 401, got %d", response.StatusCode)
	}
}

// TestAPIKeyPolicy_CaseInsensitiveHeader tests case-insensitive header matching
func TestAPIKeyPolicy_CaseInsensitiveHeader(t *testing.T) {
	// Generate realistic API key (apip_ + 64 hex chars)
	plainAPIKey := generateRealistic32ByteAPIKey()

	// Create request context with valid API key in query parameter
	id := generateShortUniqueID()
	// Setup mock API key store with valid key
	setupValidAPIKeyStore(t, "test-api-id", plainAPIKey, id)

	// Create request context with valid API key in header using consistent helper
	apiKeyValue := createTestAPIKeyValue(plainAPIKey, id)
	headers := map[string][]string{
		"x-api-key": {apiKeyValue},
	}
	ctx := createMockRequestContext("/api/test", headers)
	ctx.APIId = "test-api-id"
	ctx.APIName = "test-api"
	ctx.APIVersion = "v1.0.0"
	ctx.OperationPath = "/test"
	ctx.Method = "GET"

	// Create params with different casing
	params := map[string]interface{}{
		"key": "X-API-Key", // Different case than header
		"in":  "header",
	}

	p, err := GetPolicy(policy.PolicyMetadata{}, params)
	if err != nil {
		t.Fatalf("Failed to create policy: %v", err)
	}

	action := p.OnRequest(ctx, params)

	// Verify successful authentication (case-insensitive matching)
	if ctx.Metadata[MetadataKeyAuthSuccess] != true {
		t.Errorf("Expected auth.success to be true for case-insensitive header, got %v", ctx.Metadata[MetadataKeyAuthSuccess])
	}

	_, ok := action.(policy.UpstreamRequestModifications)
	if !ok {
		t.Fatalf("Expected UpstreamRequestModifications, got %T", action)
	}
}

// TestAPIKeyPolicy_CaseInsensitivePrefix tests case-insensitive prefix matching
func TestAPIKeyPolicy_CaseInsensitivePrefix(t *testing.T) {
	// Generate realistic API key (apip_ + 64 hex chars)
	plainAPIKey := generateRealistic32ByteAPIKey()

	// Create request context with valid API key in query parameter
	id := generateShortUniqueID()
	// Setup mock API key store with valid key
	setupValidAPIKeyStore(t, "test-api-id", plainAPIKey, id)

	// Create request context with valid API key in header using consistent helper
	apiKeyValue := createTestAPIKeyValue(plainAPIKey, id)
	headers := map[string][]string{
		"Authorization": {"bearer " + apiKeyValue}, // lowercase "bearer"
	}
	ctx := createMockRequestContext("/api/test", headers)
	ctx.APIId = "test-api-id"
	ctx.APIName = "test-api"
	ctx.APIVersion = "v1.0.0"
	ctx.OperationPath = "/test"
	ctx.Method = "GET"

	// Create params with uppercase prefix
	params := map[string]interface{}{
		"key":          "Authorization",
		"in":           "header",
		"value-prefix": "Bearer ", // uppercase "Bearer"
	}

	p, err := GetPolicy(policy.PolicyMetadata{}, params)
	if err != nil {
		t.Fatalf("Failed to create policy: %v", err)
	}

	action := p.OnRequest(ctx, params)

	// Verify successful authentication (case-insensitive prefix matching)
	if ctx.Metadata[MetadataKeyAuthSuccess] != true {
		t.Errorf("Expected auth.success to be true for case-insensitive prefix, got %v", ctx.Metadata[MetadataKeyAuthSuccess])
	}

	_, ok := action.(policy.UpstreamRequestModifications)
	if !ok {
		t.Fatalf("Expected UpstreamRequestModifications, got %T", action)
	}
}

// TestAPIKeyPolicy_QueryParamWithSpecialCharacters tests query parameter parsing with URL encoding
func TestAPIKeyPolicy_QueryParamWithSpecialCharacters(t *testing.T) {
	// Generate realistic API key (apip_ + 64 hex chars)
	plainAPIKey := generateRealistic32ByteAPIKey()

	// Create request context with valid API key in query parameter
	id := generateShortUniqueID()
	// Setup mock API key store with valid key
	setupValidAPIKeyStore(t, "test-api-id", plainAPIKey, id)

	// Create request context with valid API key in header using consistent helper
	apiKeyValue := createTestAPIKeyValue(plainAPIKey, id)
	encodedAPIKey := apiKeyValue // No need for complex encoding, just test the mechanism
	ctx := createMockRequestContext("/api/test?api_key="+encodedAPIKey+"&other=value%20with%20spaces", map[string][]string{})
	ctx.APIId = "test-api-id"
	ctx.APIName = "test-api"
	ctx.APIVersion = "v1.0.0"
	ctx.OperationPath = "/test"
	ctx.Method = "GET"

	params := map[string]interface{}{
		"key": "api_key",
		"in":  "query",
	}

	p, err := GetPolicy(policy.PolicyMetadata{}, params)
	if err != nil {
		t.Fatalf("Failed to create policy: %v", err)
	}

	action := p.OnRequest(ctx, params)

	// Verify successful authentication with URL-decoded key
	if ctx.Metadata[MetadataKeyAuthSuccess] != true {
		t.Errorf("Expected auth.success to be true for URL-encoded query, got %v", ctx.Metadata[MetadataKeyAuthSuccess])
	}

	_, ok := action.(policy.UpstreamRequestModifications)
	if !ok {
		t.Fatalf("Expected UpstreamRequestModifications, got %T", action)
	}
}

// TestAPIKeyPolicy_ErrorResponseFormatJSON tests JSON error response format (default)
func TestAPIKeyPolicy_ErrorResponseFormatJSON(t *testing.T) {
	ctx := createMockRequestContext("/api/test", map[string][]string{})
	ctx.APIId = "test-api-id"
	ctx.APIName = "test-api"
	ctx.APIVersion = "v1.0.0"
	ctx.OperationPath = "/test"
	ctx.Method = "GET"

	params := map[string]interface{}{
		"key": "X-API-Key",
		"in":  "header",
	}

	p, err := GetPolicy(policy.PolicyMetadata{}, params)
	if err != nil {
		t.Fatalf("Failed to create policy: %v", err)
	}

	action := p.OnRequest(ctx, params)

	response := action.(policy.ImmediateResponse)
	if response.Headers["content-type"] != "application/json" {
		t.Errorf("Expected content-type to be application/json, got %s", response.Headers["content-type"])
	}

	// Verify JSON structure
	var errBody map[string]interface{}
	if err := json.Unmarshal(response.Body, &errBody); err != nil {
		t.Errorf("Expected JSON error response, got unmarshal error: %v, body: %s", err, string(response.Body))
	}

	if errBody["error"] != "Unauthorized" {
		t.Errorf("Expected error field to be 'Unauthorized', got %v", errBody["error"])
	}

	if errBody["message"] != "Valid API key required" {
		t.Errorf("Expected message field to be 'Valid API key required', got %v", errBody["message"])
	}
}

// TestAPIKeyPolicy_Mode tests the policy's processing mode configuration
func TestAPIKeyPolicy_Mode(t *testing.T) {
	p := &APIKeyPolicy{}
	mode := p.Mode()

	// Verify processing mode configuration
	if mode.RequestHeaderMode != policy.HeaderModeProcess {
		t.Errorf("Expected RequestHeaderMode to be Process, got %v", mode.RequestHeaderMode)
	}

	if mode.RequestBodyMode != policy.BodyModeSkip {
		t.Errorf("Expected RequestBodyMode to be Skip, got %v", mode.RequestBodyMode)
	}

	if mode.ResponseHeaderMode != policy.HeaderModeSkip {
		t.Errorf("Expected ResponseHeaderMode to be Skip, got %v", mode.ResponseHeaderMode)
	}

	if mode.ResponseBodyMode != policy.BodyModeSkip {
		t.Errorf("Expected ResponseBodyMode to be Skip, got %v", mode.ResponseBodyMode)
	}
}

// TestAPIKeyPolicy_OnResponse tests that OnResponse returns nil (no response processing)
func TestAPIKeyPolicy_OnResponse(t *testing.T) {
	p := &APIKeyPolicy{}
	ctx := &policy.ResponseContext{}
	params := map[string]interface{}{}

	action := p.OnResponse(ctx, params)

	if action != nil {
		t.Errorf("Expected OnResponse to return nil, got %v", action)
	}
}

// TestAPIKeyPolicy_MultipleHeaderValues tests behavior with multiple values for the same header
func TestAPIKeyPolicy_MultipleHeaderValues(t *testing.T) {
	// Generate realistic API key (apip_ + 64 hex chars)
	plainAPIKey := generateRealistic32ByteAPIKey()

	// Create request context with valid API key in query parameter
	id := generateShortUniqueID()
	// Setup mock API key store with valid key
	setupValidAPIKeyStore(t, "test-api-id", plainAPIKey, id)

	// Create request context with valid API key in header using consistent helper
	apiKeyValue := createTestAPIKeyValue(plainAPIKey, id)
	headers := map[string][]string{
		"X-API-Key": {apiKeyValue, "other-key"},
	}
	ctx := createMockRequestContext("/api/test", headers)
	ctx.APIId = "test-api-id"
	ctx.APIName = "test-api"
	ctx.APIVersion = "v1.0.0"
	ctx.OperationPath = "/test"
	ctx.Method = "GET"

	params := map[string]interface{}{
		"key": "X-API-Key",
		"in":  "header",
	}

	p, err := GetPolicy(policy.PolicyMetadata{}, params)
	if err != nil {
		t.Fatalf("Failed to create policy: %v", err)
	}

	action := p.OnRequest(ctx, params)

	// Verify successful authentication (uses first header value)
	if ctx.Metadata[MetadataKeyAuthSuccess] != true {
		t.Errorf("Expected auth.success to be true with multiple header values, got %v", ctx.Metadata[MetadataKeyAuthSuccess])
	}

	_, ok := action.(policy.UpstreamRequestModifications)
	if !ok {
		t.Fatalf("Expected UpstreamRequestModifications, got %T", action)
	}
}

// TestAPIKeyPolicy_EmptyStringParameters tests behavior with empty string parameters
func TestAPIKeyPolicy_EmptyStringParameters(t *testing.T) {
	ctx := createMockRequestContext("/api/test", map[string][]string{})
	ctx.APIId = "test-api-id"
	ctx.APIName = "test-api"
	ctx.APIVersion = "v1.0.0"
	ctx.OperationPath = "/test"
	ctx.Method = "GET"

	// Test empty key parameter
	params := map[string]interface{}{
		"key": "",
		"in":  "header",
	}

	p, err := GetPolicy(policy.PolicyMetadata{}, params)
	if err != nil {
		t.Fatalf("Failed to create policy: %v", err)
	}

	action := p.OnRequest(ctx, params)

	// Verify authentication failed
	if ctx.Metadata[MetadataKeyAuthSuccess] != false {
		t.Errorf("Expected auth.success to be false for empty key parameter, got %v", ctx.Metadata[MetadataKeyAuthSuccess])
	}

	response, ok := action.(policy.ImmediateResponse)
	if !ok {
		t.Fatalf("Expected ImmediateResponse, got %T", action)
	}

	if response.StatusCode != 401 {
		t.Errorf("Expected status code 401, got %d", response.StatusCode)
	}
}

// TestAPIKeyPolicy_NonStringParameters tests behavior with non-string parameter types
func TestAPIKeyPolicy_NonStringParameters(t *testing.T) {
	ctx := createMockRequestContext("/api/test", map[string][]string{})
	ctx.APIId = "test-api-id"
	ctx.APIName = "test-api"
	ctx.APIVersion = "v1.0.0"
	ctx.OperationPath = "/test"
	ctx.Method = "GET"

	// Test with non-string key parameter
	params := map[string]interface{}{
		"key": 123, // integer instead of string
		"in":  "header",
	}

	p, err := GetPolicy(policy.PolicyMetadata{}, params)
	if err != nil {
		t.Fatalf("Failed to create policy: %v", err)
	}

	action := p.OnRequest(ctx, params)

	// Verify authentication failed
	if ctx.Metadata[MetadataKeyAuthSuccess] != false {
		t.Errorf("Expected auth.success to be false for non-string key parameter, got %v", ctx.Metadata[MetadataKeyAuthSuccess])
	}

	response, ok := action.(policy.ImmediateResponse)
	if !ok {
		t.Fatalf("Expected ImmediateResponse, got %T", action)
	}

	if response.StatusCode != 401 {
		t.Errorf("Expected status code 401, got %d", response.StatusCode)
	}
}

// TestAPIKeyPolicy_InvalidInParameter tests behavior with invalid 'in' parameter values
func TestAPIKeyPolicy_InvalidInParameter(t *testing.T) {
	ctx := createMockRequestContext("/api/test", map[string][]string{})
	ctx.APIId = "test-api-id"
	ctx.APIName = "test-api"
	ctx.APIVersion = "v1.0.0"
	ctx.OperationPath = "/test"
	ctx.Method = "GET"

	// Test with invalid 'in' parameter
	params := map[string]interface{}{
		"key": "X-API-Key",
		"in":  "body", // Invalid location
	}

	p, err := GetPolicy(policy.PolicyMetadata{}, params)
	if err != nil {
		t.Fatalf("Failed to create policy: %v", err)
	}

	action := p.OnRequest(ctx, params)

	// Should fail since 'body' is not a valid location
	if ctx.Metadata[MetadataKeyAuthSuccess] != false {
		t.Errorf("Expected auth.success to be false for invalid 'in' parameter, got %v", ctx.Metadata[MetadataKeyAuthSuccess])
	}

	response, ok := action.(policy.ImmediateResponse)
	if !ok {
		t.Fatalf("Expected ImmediateResponse, got %T", action)
	}

	if response.StatusCode != 401 {
		t.Errorf("Expected status code 401, got %d", response.StatusCode)
	}
}

// TestAPIKeyPolicy_QueryParamNotFound tests behavior when query parameter doesn't exist
func TestAPIKeyPolicy_QueryParamNotFound(t *testing.T) {
	ctx := createMockRequestContext("/api/test?different_param=value", map[string][]string{})
	ctx.APIId = "test-api-id"
	ctx.APIName = "test-api"
	ctx.APIVersion = "v1.0.0"
	ctx.OperationPath = "/test"
	ctx.Method = "GET"

	params := map[string]interface{}{
		"key": "api_key", // Looking for this param, but it doesn't exist
		"in":  "query",
	}

	p, err := GetPolicy(policy.PolicyMetadata{}, params)
	if err != nil {
		t.Fatalf("Failed to create policy: %v", err)
	}

	action := p.OnRequest(ctx, params)

	// Should fail since the expected query parameter is not found
	if ctx.Metadata[MetadataKeyAuthSuccess] != false {
		t.Errorf("Expected auth.success to be false when query param not found, got %v", ctx.Metadata[MetadataKeyAuthSuccess])
	}

	response, ok := action.(policy.ImmediateResponse)
	if !ok {
		t.Fatalf("Expected ImmediateResponse, got %T", action)
	}

	if response.StatusCode != 401 {
		t.Errorf("Expected status code 401, got %d", response.StatusCode)
	}
}

// TestAPIKeyPolicy_EmptyValuePrefix tests behavior with empty value-prefix parameter
func TestAPIKeyPolicy_EmptyValuePrefix(t *testing.T) {
	// Generate realistic API key (apip_ + 64 hex chars)
	plainAPIKey := generateRealistic32ByteAPIKey()

	// Create request context with valid API key in query parameter
	id := generateShortUniqueID()
	// Setup mock API key store with valid key
	setupValidAPIKeyStore(t, "test-api-id", plainAPIKey, id)

	// Create request context with valid API key in header using consistent helper
	apiKeyValue := createTestAPIKeyValue(plainAPIKey, id)
	headers := map[string][]string{
		"Authorization": {apiKeyValue}, // No prefix
	}
	ctx := createMockRequestContext("/api/test", headers)
	ctx.APIId = "test-api-id"
	ctx.APIName = "test-api"
	ctx.APIVersion = "v1.0.0"
	ctx.OperationPath = "/test"
	ctx.Method = "GET"

	params := map[string]interface{}{
		"key":          "Authorization",
		"in":           "header",
		"value-prefix": "", // Empty prefix
	}

	p, err := GetPolicy(policy.PolicyMetadata{}, params)
	if err != nil {
		t.Fatalf("Failed to create policy: %v", err)
	}

	action := p.OnRequest(ctx, params)

	// Should succeed since empty prefix means no prefix removal
	if ctx.Metadata[MetadataKeyAuthSuccess] != true {
		t.Errorf("Expected auth.success to be true with empty prefix, got %v", ctx.Metadata[MetadataKeyAuthSuccess])
	}

	_, ok := action.(policy.UpstreamRequestModifications)
	if !ok {
		t.Fatalf("Expected UpstreamRequestModifications, got %T", action)
	}
}

// TestAPIKeyPolicy_LongAPIKey tests behavior with very long API keys
func TestAPIKeyPolicy_LongAPIKey(t *testing.T) {
	// Create a realistic long API key by using a large number of bytes (but still hex encoded properly)
	// Generate 128 random bytes (instead of normal 32) to make it longer
	longRandomBytes := make([]byte, 128)
	_, err := rand.Read(longRandomBytes)
	if err != nil {
		t.Fatalf("Failed to generate long random bytes: %v", err)
	}
	longPlainAPIKey := "apip_" + hex.EncodeToString(longRandomBytes) // This creates a very long but realistic key

	// Create request context with valid API key in query parameter
	id := generateShortUniqueID()
	// Setup mock API key store with valid key
	setupValidAPIKeyStore(t, "test-api-id", longPlainAPIKey, id)

	// Create request context with valid API key in header using consistent helper
	apiKeyValue := createTestAPIKeyValue(longPlainAPIKey, id)
	headers := map[string][]string{
		"X-API-Key": {apiKeyValue},
	}
	ctx := createMockRequestContext("/api/test", headers)
	ctx.APIId = "test-api-id"
	ctx.APIName = "test-api"
	ctx.APIVersion = "v1.0.0"
	ctx.OperationPath = "/test"
	ctx.Method = "GET"

	params := map[string]interface{}{
		"key": "X-API-Key",
		"in":  "header",
	}

	p, err := GetPolicy(policy.PolicyMetadata{}, params)
	if err != nil {
		t.Fatalf("Failed to create policy: %v", err)
	}

	action := p.OnRequest(ctx, params)

	// Should succeed with long API key
	if ctx.Metadata[MetadataKeyAuthSuccess] != true {
		t.Errorf("Expected auth.success to be true with long API key, got %v", ctx.Metadata[MetadataKeyAuthSuccess])
	}

	_, ok := action.(policy.UpstreamRequestModifications)
	if !ok {
		t.Fatalf("Expected UpstreamRequestModifications, got %T", action)
	}
}

// TestAPIKeyPolicy_MultipleQueryParams tests handling of multiple query parameters with same name
func TestAPIKeyPolicy_MultipleQueryParams(t *testing.T) {
	// Generate realistic API key (apip_ + 64 hex chars)
	plainAPIKey := generateRealistic32ByteAPIKey()

	// Create request context with valid API key in query parameter
	id := generateShortUniqueID()
	// Setup mock API key store with valid key
	setupValidAPIKeyStore(t, "test-api-id", plainAPIKey, id)

	// Create request context with valid API key in header using consistent helper
	apiKeyValue := createTestAPIKeyValue(plainAPIKey, id)
	// Should use the first occurrence
	ctx := createMockRequestContext("/api/test?api_key="+apiKeyValue+"&api_key=invalid-key&other=value", map[string][]string{})
	ctx.APIId = "test-api-id"
	ctx.APIName = "test-api"
	ctx.APIVersion = "v1.0.0"
	ctx.OperationPath = "/test"
	ctx.Method = "GET"

	params := map[string]interface{}{
		"key": "api_key",
		"in":  "query",
	}

	p, err := GetPolicy(policy.PolicyMetadata{}, params)
	if err != nil {
		t.Fatalf("Failed to create policy: %v", err)
	}

	action := p.OnRequest(ctx, params)

	// Should succeed using the first query parameter value
	if ctx.Metadata[MetadataKeyAuthSuccess] != true {
		t.Errorf("Expected auth.success to be true with multiple query params, got %v", ctx.Metadata[MetadataKeyAuthSuccess])
	}

	_, ok := action.(policy.UpstreamRequestModifications)
	if !ok {
		t.Fatalf("Expected UpstreamRequestModifications, got %T", action)
	}
}

// TestAPIKeyPolicy_ShortUniqueIDFormat tests the new 22-character short ID format
func TestAPIKeyPolicy_ShortUniqueIDFormat(t *testing.T) {
	// Generate realistic API key (apip_ + 64 hex chars)
	plainAPIKey := generateRealistic32ByteAPIKey()

	// Create API key with new short ID format (22 characters, no underscores)
	id := generateShortUniqueID()
	// Setup mock API key store with valid key
	setupValidAPIKeyStore(t, "test-api-id", plainAPIKey, id)

	if len(id) != 22 {
		t.Fatalf("Expected short ID to be 22 characters, got %d", len(id))
	}

	// Verify no underscores in the short ID
	if strings.Contains(id, "_") {
		t.Errorf("Short ID should not contain underscores: %s", id)
	}

	// Create API key with proper format: apip_<key>_<short_id>
	// Create request context with valid API key in header using consistent helper
	apiKeyValue := createTestAPIKeyValue(plainAPIKey, id)

	t.Logf("Using API key value: %s", apiKeyValue)

	// Verify the format: should have exactly two "_" separators
	parts := strings.Split(apiKeyValue, "_")
	if len(parts) != 3 {
		t.Fatalf("Expected API key to have format 'key_id', got: %s", apiKeyValue)
	}

	if parts[2] != id {
		t.Errorf("Expected ID part to be %s, got %s", id, parts[2])
	}

	headers := map[string][]string{
		"X-API-Key": {apiKeyValue},
	}
	ctx := createMockRequestContext("/api/test", headers)
	ctx.APIId = "test-api-id"
	ctx.APIName = "test-api"
	ctx.APIVersion = "v1.0.0"
	ctx.OperationPath = "/test"
	ctx.Method = "GET"

	// Create params for header-based API key authentication
	params := map[string]interface{}{
		"key": "X-API-Key",
		"in":  "header",
	}

	p, err := GetPolicy(policy.PolicyMetadata{}, params)
	if err != nil {
		t.Fatalf("Failed to create policy: %v", err)
	}

	// Execute policy
	action := p.OnRequest(ctx, params)

	// Verify successful authentication with new format
	if ctx.Metadata[MetadataKeyAuthSuccess] != true {
		t.Errorf("Expected auth.success to be true with short ID format, got %v", ctx.Metadata[MetadataKeyAuthSuccess])
	}

	if ctx.Metadata[MetadataKeyAuthMethod] != "api-key" {
		t.Errorf("Expected auth.method to be 'api-key', got %v", ctx.Metadata[MetadataKeyAuthMethod])
	}

	// Verify it's an UpstreamRequestModifications action
	_, ok := action.(policy.UpstreamRequestModifications)
	if !ok {
		t.Fatalf("Expected UpstreamRequestModifications, got %T", action)
	}
}

// Helper functions

// createMockRequestContext creates a mock request context
func createMockRequestContext(path string, headers map[string][]string) *policy.RequestContext {
	return &policy.RequestContext{
		SharedContext: &policy.SharedContext{
			RequestID: "test-request-id",
			Metadata:  make(map[string]interface{}),
		},
		Headers: policy.NewHeaders(headers),
		Body:    nil,
		Path:    path,
		Method:  "GET",
	}
}

// setupValidAPIKeyStore mocks the API key store to return valid for a specific key
func setupValidAPIKeyStore(t *testing.T, apiId, plainAPIKey, id string) {
	// Get the singleton API key store
	store := policy.GetAPIkeyStoreInstance()

	// Clear any existing data for clean test
	err := store.RemoveAPIKeysByAPI(apiId)
	if err != nil {
		t.Fatalf("Failed to remove existing API keys: %v", err)
	}

	// Store a valid API key for testing
	// In reality, the plainAPIKey would be something like "apip_<64 hex chars>"
	// This gets hashed using SHA256 and stored in the policy engine
	hashedAPIKey := hashAPIKeyWithSHA256(plainAPIKey)

	testAPIKey := &policy.APIKey{
		ID:         id, // Use fixed test ID
		Name:       "Test API Key",
		APIKey:     hashedAPIKey, // Store hashed key (as it would be in reality)
		APIId:      apiId,
		Operations: `["*"]`, // Allow all operations for testing
		Status:     policy.Active,
	}

	err = store.StoreAPIKey(apiId, testAPIKey)
	if err != nil {
		t.Fatalf("Failed to setup valid API key store: %v", err)
	}

	// Log the stored key for debugging
	t.Logf("Stored API key with ID: %s, hashed key: %s", id, hashedAPIKey)
}

// Helper function to create test API key with the same fixed ID used in setupValidAPIKeyStore
func createTestAPIKeyValue(plainAPIKey, id string) string {
	return plainAPIKey + "_" + id
}

// setupInvalidAPIKeyStore mocks the API key store to return invalid for any key
func setupInvalidAPIKeyStore() {
	// No need to store anything - any key validation will fail
	// This simulates the scenario where the provided key is not found in the store
}
