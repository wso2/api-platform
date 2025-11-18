package models

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/gin-gonic/gin"
	"github.com/oapi-codegen/runtime"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

// RestAPIConfiguration defines model for REST API Configuration
type RestAPIConfiguration struct {
	Data APIConfigData `json:"data"`
}

// type APIConfigData struct {
// 	Name    string `json:"name"`
// 	Version string `json:"version"`
// 	Context string `json:"context"`
// }

// WebSubAPIConfiguration defines model for WebSub/Async API Configuration
type WebSubAPIConfiguration struct {
	Data WebhookAPIData `json:"data"`
}

// Defines values for APIConfigDataApiType.
const (
	APIConfigDataApiTypeAsyncsse       APIConfigDataApiType = "async/sse"
	APIConfigDataApiTypeAsyncwebsocket APIConfigDataApiType = "async/websocket"
	APIConfigDataApiTypeAsyncwebsub    APIConfigDataApiType = "async/websub"
	APIConfigDataApiTypeHttprest       APIConfigDataApiType = "http/rest"
)

// Defines values for APIConfigurationKind.
const (
	APIConfigurationKindAsyncsse       APIConfigurationKind = "async/sse"
	APIConfigurationKindAsyncwebsocket APIConfigurationKind = "async/websocket"
	APIConfigurationKindAsyncwebsub    APIConfigurationKind = "async/websub"
	APIConfigurationKindHttprest       APIConfigurationKind = "http/rest"
)

// Defines values for APIConfigurationVersion.
const (
	ApiPlatformWso2Comv1 APIConfigurationVersion = "api-platform.wso2.com/v1"
)

// Defines values for APIDetailResponseApiMetadataStatus.
const (
	APIDetailResponseApiMetadataStatusDeployed APIDetailResponseApiMetadataStatus = "deployed"
	APIDetailResponseApiMetadataStatusFailed   APIDetailResponseApiMetadataStatus = "failed"
	APIDetailResponseApiMetadataStatusPending  APIDetailResponseApiMetadataStatus = "pending"
)

// Defines values for APIListItemStatus.
const (
	APIListItemStatusDeployed APIListItemStatus = "deployed"
	APIListItemStatusFailed   APIListItemStatus = "failed"
	APIListItemStatusPending  APIListItemStatus = "pending"
)

// Defines values for OperationMethod.
const (
	DELETE  OperationMethod = "DELETE"
	GET     OperationMethod = "GET"
	HEAD    OperationMethod = "HEAD"
	OPTIONS OperationMethod = "OPTIONS"
	PATCH   OperationMethod = "PATCH"
	POST    OperationMethod = "POST"
	PUT     OperationMethod = "PUT"
)

// Defines values for ServerProtocol.
const (
	Kafka     ServerProtocol = "kafka"
	Mqtt      ServerProtocol = "mqtt"
	Sse       ServerProtocol = "sse"
	Websocket ServerProtocol = "websocket"
	Websub    ServerProtocol = "websub"
)

// Defines values for WebhookAPIDataApiType.
const (
	Asyncsse       WebhookAPIDataApiType = "async/sse"
	Asyncwebsocket WebhookAPIDataApiType = "async/websocket"
	Asyncwebsub    WebhookAPIDataApiType = "async/websub"
	Httprest       WebhookAPIDataApiType = "http/rest"
)

// APIConfigData defines model for APIConfigData.
type APIConfigData struct {
	// ApiType API type
	ApiType APIConfigDataApiType `json:"apiType"`

	// Context Base path for all API routes (must start with /, no trailing slash)
	Context string `json:"context"`

	// Name Human-readable API name (must be URL-friendly - only letters, numbers, spaces, hyphens, underscores, and dots allowed)
	Name string `json:"name"`

	// Operations List of HTTP operations/routes
	Operations []Operation `json:"operations"`

	// Upstream List of backend service URLs
	Upstream []Upstream `json:"upstream"`

	// Version Semantic version of the API
	Version string `json:"version"`
}

// APIConfigDataApiType API type
type APIConfigDataApiType string

// APIConfiguration defines model for APIConfiguration.
type APIConfiguration struct {
	// Data API configuration payload (REST or Async API variants)
	Data APIConfiguration_Data `json:"data"`

	// Kind API type
	Kind APIConfigurationKind `json:"kind"`

	// Version API specification version
	Version APIConfigurationVersion `json:"version"`
}

// APIConfiguration_Data API configuration payload (REST or Async API variants)
type APIConfiguration_Data struct {
	union json.RawMessage
}

// APIConfigurationKind API type
type APIConfigurationKind string

// APIConfigurationVersion API specification version
type APIConfigurationVersion string

// APICreateResponse defines model for APICreateResponse.
type APICreateResponse struct {
	CreatedAt *time.Time `json:"created_at,omitempty"`

	// Id Unique identifier for the created API configuration
	Id      *openapi_types.UUID `json:"id,omitempty"`
	Message *string             `json:"message,omitempty"`
	Status  *string             `json:"status,omitempty"`
}

// APIDetailResponse defines model for APIDetailResponse.
type APIDetailResponse struct {
	Api *struct {
		Configuration *APIConfiguration   `json:"configuration,omitempty"`
		Id            *openapi_types.UUID `json:"id,omitempty"`
		Metadata      *struct {
			CreatedAt  *time.Time                          `json:"created_at,omitempty"`
			DeployedAt *time.Time                          `json:"deployed_at,omitempty"`
			Status     *APIDetailResponseApiMetadataStatus `json:"status,omitempty"`
			UpdatedAt  *time.Time                          `json:"updated_at,omitempty"`
		} `json:"metadata,omitempty"`
	} `json:"api,omitempty"`
	Status *string `json:"status,omitempty"`
}

// APIDetailResponseApiMetadataStatus defines model for APIDetailResponse.Api.Metadata.Status.
type APIDetailResponseApiMetadataStatus string

// APIListItem defines model for APIListItem.
type APIListItem struct {
	Context   *string             `json:"context,omitempty"`
	CreatedAt *time.Time          `json:"created_at,omitempty"`
	Id        *openapi_types.UUID `json:"id,omitempty"`
	Name      *string             `json:"name,omitempty"`
	Status    *APIListItemStatus  `json:"status,omitempty"`
	UpdatedAt *time.Time          `json:"updated_at,omitempty"`
	Version   *string             `json:"version,omitempty"`
}

// APIListItemStatus defines model for APIListItem.Status.
type APIListItemStatus string

// APIUpdateResponse defines model for APIUpdateResponse.
type APIUpdateResponse struct {
	Id        *openapi_types.UUID `json:"id,omitempty"`
	Message   *string             `json:"message,omitempty"`
	Status    *string             `json:"status,omitempty"`
	UpdatedAt *time.Time          `json:"updated_at,omitempty"`
}

// Channel Channel (topic/event stream) definition for async APIs.
type Channel struct {
	// Bindings Protocol-specific channel bindings (arbitrary key/value structure).
	Bindings *map[string]interface{} `json:"bindings,omitempty"`

	// Description Human-readable description of the channel.
	Description *string `json:"description,omitempty"`

	// Parameters Path/channel parameters (keyed by parameter name).
	Parameters *map[string]struct {
		Description *string `json:"description,omitempty"`

		// Schema JSON Schema fragment for the parameter value.
		Schema *map[string]interface{} `json:"schema,omitempty"`
	} `json:"parameters,omitempty"`

	// Path Channel path or topic identifier relative to API context.
	Path string `json:"path"`

	// Publish Producer (send) operation definition.
	Publish *struct {
		// Message Event/message definition transported over a channel.
		Message *ChannelMessage `json:"message,omitempty"`
		Summary *string         `json:"summary,omitempty"`
	} `json:"publish,omitempty"`

	// Subscribe Consumer (receive) operation definition.
	Subscribe *struct {
		// Message Event/message definition transported over a channel.
		Message *ChannelMessage `json:"message,omitempty"`
		Summary *string         `json:"summary,omitempty"`
	} `json:"subscribe,omitempty"`
}

// ChannelMessage Event/message definition transported over a channel.
type ChannelMessage struct {
	// ContentType Content type of the payload.
	ContentType *string `json:"content_type,omitempty"`

	// Name Logical message name.
	Name string `json:"name"`

	// Payload JSON Schema representation of the message body.
	Payload map[string]interface{} `json:"payload"`

	// Summary Short description of the message.
	Summary *string `json:"summary,omitempty"`
}

// ErrorResponse defines model for ErrorResponse.
type ErrorResponse struct {
	// Errors Detailed validation errors
	Errors *[]ValidationError `json:"errors,omitempty"`

	// Message High-level error description
	Message string `json:"message"`
	Status  string `json:"status"`
}

// Operation defines model for Operation.
type Operation struct {
	// Method HTTP method
	Method OperationMethod `json:"method"`

	// Path Route path with optional {param} placeholders
	Path string `json:"path"`
}

// OperationMethod HTTP method
type OperationMethod string

// Server Server definition for async or WebSub APIs.
type Server struct {
	// Bindings Protocol-specific server bindings (arbitrary key/value structure).
	Bindings *map[string]interface{} `json:"bindings,omitempty"`

	// Description Human-readable description of this server.
	Description *string `json:"description,omitempty"`

	// Protocol Transport protocol used by the server.
	Protocol ServerProtocol `json:"protocol"`

	// ProtocolVersion Version of the selected protocol (if applicable).
	ProtocolVersion *string `json:"protocolVersion,omitempty"`

	// Security Security requirements for this server (each item maps scheme name to optional scopes array).
	Security *[]map[string][]string `json:"security,omitempty"`

	// Url Base URL or connection string for the server (variables may be denoted by {name}).
	Url string `json:"url"`

	// Variables Templated variables contained in the server URL.
	Variables *map[string]struct {
		// Default Default value for the variable.
		Default string `json:"default"`

		// Description Description of the variable.
		Description *string `json:"description,omitempty"`

		// Enum Allowed values.
		Enum *[]string `json:"enum,omitempty"`
	} `json:"variables,omitempty"`
}

// ServerProtocol Transport protocol used by the server.
type ServerProtocol string

// Upstream defines model for Upstream.
type Upstream struct {
	// Url Backend service URL (may include path prefix like /api/v2)
	Url string `json:"url"`
}

// ValidationError defines model for ValidationError.
type ValidationError struct {
	// Field Field that failed validation
	Field *string `json:"field,omitempty"`

	// Message Human-readable error message
	Message *string `json:"message,omitempty"`
}

// WebhookAPIData defines model for WebhookAPIData.
type WebhookAPIData struct {
	// Channels List of operations - HTTP operations for REST APIs or event/topic operations for async APIs
	Channels []Channel `json:"channels"`

	// Context Base path for all API routes (must start with /, no trailing slash)
	Context string `json:"context"`

	// Name Human-readable API name (must be URL-friendly - only letters, numbers, spaces, hyphens, underscores, and dots allowed)
	Name string `json:"name"`

	// Servers List of backend service URLs (for REST APIs) or event hub URLs (for async APIs)
	Servers []Server `json:"servers"`

	// Version Semantic version of the API
	Version string `json:"version"`
}

// WebhookAPIDataApiType API type
type WebhookAPIDataApiType string

// CreateAPIJSONRequestBody defines body for CreateAPI for application/json ContentType.
type CreateAPIJSONRequestBody = APIConfiguration

// UpdateAPIJSONRequestBody defines body for UpdateAPI for application/json ContentType.
type UpdateAPIJSONRequestBody = APIConfiguration

// AsAPIConfigData returns the union data inside the APIConfiguration_Data as a APIConfigData
func (t APIConfiguration_Data) AsAPIConfigData() (APIConfigData, error) {
	var body APIConfigData
	err := json.Unmarshal(t.union, &body)
	return body, err
}

// FromAPIConfigData overwrites any union data inside the APIConfiguration_Data as the provided APIConfigData
func (t *APIConfiguration_Data) FromAPIConfigData(v APIConfigData) error {
	b, err := json.Marshal(v)
	t.union = b
	return err
}

// MergeAPIConfigData performs a merge with any union data inside the APIConfiguration_Data, using the provided APIConfigData
func (t *APIConfiguration_Data) MergeAPIConfigData(v APIConfigData) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}

	merged, err := runtime.JSONMerge(t.union, b)
	t.union = merged
	return err
}

// AsWebhookAPIData returns the union data inside the APIConfiguration_Data as a WebhookAPIData
func (t APIConfiguration_Data) AsWebhookAPIData() (WebhookAPIData, error) {
	var body WebhookAPIData
	err := json.Unmarshal(t.union, &body)
	return body, err
}

// FromWebhookAPIData overwrites any union data inside the APIConfiguration_Data as the provided WebhookAPIData
func (t *APIConfiguration_Data) FromWebhookAPIData(v WebhookAPIData) error {
	b, err := json.Marshal(v)
	t.union = b
	return err
}

// MergeWebhookAPIData performs a merge with any union data inside the APIConfiguration_Data, using the provided WebhookAPIData
func (t *APIConfiguration_Data) MergeWebhookAPIData(v WebhookAPIData) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}

	merged, err := runtime.JSONMerge(t.union, b)
	t.union = merged
	return err
}

func (t APIConfiguration_Data) MarshalJSON() ([]byte, error) {
	b, err := t.union.MarshalJSON()
	return b, err
}

func (t *APIConfiguration_Data) UnmarshalJSON(b []byte) error {
	err := t.union.UnmarshalJSON(b)
	return err
}

// ServerInterface represents all server handlers.
type ServerInterface interface {
	// List all API configurations
	// (GET /apis)
	ListAPIs(c *gin.Context)
	// Create a new API configuration
	// (POST /apis)
	CreateAPI(c *gin.Context)
	// Delete an API configuration
	// (DELETE /apis/{name}/{version})
	DeleteAPI(c *gin.Context, name string, version string)
	// Get API configuration by name and version
	// (GET /apis/{name}/{version})
	GetAPIByNameVersion(c *gin.Context, name string, version string)
	// Update an existing API configuration
	// (PUT /apis/{name}/{version})
	UpdateAPI(c *gin.Context, name string, version string)
	// Health check endpoint
	// (GET /health)
	HealthCheck(c *gin.Context)
}

// ServerInterfaceWrapper converts contexts to parameters.
type ServerInterfaceWrapper struct {
	Handler            ServerInterface
	HandlerMiddlewares []MiddlewareFunc
	ErrorHandler       func(*gin.Context, error, int)
}

type MiddlewareFunc func(c *gin.Context)

// ListAPIs operation middleware
func (siw *ServerInterfaceWrapper) ListAPIs(c *gin.Context) {

	for _, middleware := range siw.HandlerMiddlewares {
		middleware(c)
		if c.IsAborted() {
			return
		}
	}

	siw.Handler.ListAPIs(c)
}

// CreateAPI operation middleware
func (siw *ServerInterfaceWrapper) CreateAPI(c *gin.Context) {

	for _, middleware := range siw.HandlerMiddlewares {
		middleware(c)
		if c.IsAborted() {
			return
		}
	}

	siw.Handler.CreateAPI(c)
}

// DeleteAPI operation middleware
func (siw *ServerInterfaceWrapper) DeleteAPI(c *gin.Context) {

	var err error

	// ------------- Path parameter "name" -------------
	var name string

	err = runtime.BindStyledParameterWithOptions("simple", "name", c.Param("name"), &name, runtime.BindStyledParameterOptions{Explode: false, Required: true})
	if err != nil {
		siw.ErrorHandler(c, fmt.Errorf("Invalid format for parameter name: %w", err), http.StatusBadRequest)
		return
	}

	// ------------- Path parameter "version" -------------
	var version string

	err = runtime.BindStyledParameterWithOptions("simple", "version", c.Param("version"), &version, runtime.BindStyledParameterOptions{Explode: false, Required: true})
	if err != nil {
		siw.ErrorHandler(c, fmt.Errorf("Invalid format for parameter version: %w", err), http.StatusBadRequest)
		return
	}

	for _, middleware := range siw.HandlerMiddlewares {
		middleware(c)
		if c.IsAborted() {
			return
		}
	}

	siw.Handler.DeleteAPI(c, name, version)
}

// GetAPIByNameVersion operation middleware
func (siw *ServerInterfaceWrapper) GetAPIByNameVersion(c *gin.Context) {

	var err error

	// ------------- Path parameter "name" -------------
	var name string

	err = runtime.BindStyledParameterWithOptions("simple", "name", c.Param("name"), &name, runtime.BindStyledParameterOptions{Explode: false, Required: true})
	if err != nil {
		siw.ErrorHandler(c, fmt.Errorf("Invalid format for parameter name: %w", err), http.StatusBadRequest)
		return
	}

	// ------------- Path parameter "version" -------------
	var version string

	err = runtime.BindStyledParameterWithOptions("simple", "version", c.Param("version"), &version, runtime.BindStyledParameterOptions{Explode: false, Required: true})
	if err != nil {
		siw.ErrorHandler(c, fmt.Errorf("Invalid format for parameter version: %w", err), http.StatusBadRequest)
		return
	}

	for _, middleware := range siw.HandlerMiddlewares {
		middleware(c)
		if c.IsAborted() {
			return
		}
	}

	siw.Handler.GetAPIByNameVersion(c, name, version)
}

// UpdateAPI operation middleware
func (siw *ServerInterfaceWrapper) UpdateAPI(c *gin.Context) {

	var err error

	// ------------- Path parameter "name" -------------
	var name string

	err = runtime.BindStyledParameterWithOptions("simple", "name", c.Param("name"), &name, runtime.BindStyledParameterOptions{Explode: false, Required: true})
	if err != nil {
		siw.ErrorHandler(c, fmt.Errorf("Invalid format for parameter name: %w", err), http.StatusBadRequest)
		return
	}

	// ------------- Path parameter "version" -------------
	var version string

	err = runtime.BindStyledParameterWithOptions("simple", "version", c.Param("version"), &version, runtime.BindStyledParameterOptions{Explode: false, Required: true})
	if err != nil {
		siw.ErrorHandler(c, fmt.Errorf("Invalid format for parameter version: %w", err), http.StatusBadRequest)
		return
	}

	for _, middleware := range siw.HandlerMiddlewares {
		middleware(c)
		if c.IsAborted() {
			return
		}
	}

	siw.Handler.UpdateAPI(c, name, version)
}

// HealthCheck operation middleware
func (siw *ServerInterfaceWrapper) HealthCheck(c *gin.Context) {

	for _, middleware := range siw.HandlerMiddlewares {
		middleware(c)
		if c.IsAborted() {
			return
		}
	}

	siw.Handler.HealthCheck(c)
}

// GinServerOptions provides options for the Gin server.
type GinServerOptions struct {
	BaseURL      string
	Middlewares  []MiddlewareFunc
	ErrorHandler func(*gin.Context, error, int)
}

// RegisterHandlers creates http.Handler with routing matching OpenAPI spec.
func RegisterHandlers(router gin.IRouter, si ServerInterface) {
	RegisterHandlersWithOptions(router, si, GinServerOptions{})
}

// RegisterHandlersWithOptions creates http.Handler with additional options
func RegisterHandlersWithOptions(router gin.IRouter, si ServerInterface, options GinServerOptions) {
	errorHandler := options.ErrorHandler
	if errorHandler == nil {
		errorHandler = func(c *gin.Context, err error, statusCode int) {
			c.JSON(statusCode, gin.H{"msg": err.Error()})
		}
	}

	wrapper := ServerInterfaceWrapper{
		Handler:            si,
		HandlerMiddlewares: options.Middlewares,
		ErrorHandler:       errorHandler,
	}

	router.GET(options.BaseURL+"/apis", wrapper.ListAPIs)
	router.POST(options.BaseURL+"/apis", wrapper.CreateAPI)
	router.DELETE(options.BaseURL+"/apis/:name/:version", wrapper.DeleteAPI)
	router.GET(options.BaseURL+"/apis/:name/:version", wrapper.GetAPIByNameVersion)
	router.PUT(options.BaseURL+"/apis/:name/:version", wrapper.UpdateAPI)
	router.GET(options.BaseURL+"/health", wrapper.HealthCheck)
}

// Base64 encoded, gzipped, json marshaled Swagger object
var swaggerSpec = []string{

	"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",
}

// GetSwagger returns the content of the embedded swagger specification file
// or error if failed to decode
func decodeSpec() ([]byte, error) {
	zipped, err := base64.StdEncoding.DecodeString(strings.Join(swaggerSpec, ""))
	if err != nil {
		return nil, fmt.Errorf("error base64 decoding spec: %w", err)
	}
	zr, err := gzip.NewReader(bytes.NewReader(zipped))
	if err != nil {
		return nil, fmt.Errorf("error decompressing spec: %w", err)
	}
	var buf bytes.Buffer
	_, err = buf.ReadFrom(zr)
	if err != nil {
		return nil, fmt.Errorf("error decompressing spec: %w", err)
	}

	return buf.Bytes(), nil
}

var rawSpec = decodeSpecCached()

// a naive cached of a decoded swagger spec
func decodeSpecCached() func() ([]byte, error) {
	data, err := decodeSpec()
	return func() ([]byte, error) {
		return data, err
	}
}

// Constructs a synthetic filesystem for resolving external references when loading openapi specifications.
func PathToRawSpec(pathToFile string) map[string]func() ([]byte, error) {
	res := make(map[string]func() ([]byte, error))
	if len(pathToFile) > 0 {
		res[pathToFile] = rawSpec
	}

	return res
}

// GetSwagger returns the Swagger specification corresponding to the generated code
// in this file. The external references of Swagger specification are resolved.
// The logic of resolving external references is tightly connected to "import-mapping" feature.
// Externally referenced files must be embedded in the corresponding golang packages.
// Urls can be supported but this task was out of the scope.
func GetSwagger() (swagger *openapi3.T, err error) {
	resolvePath := PathToRawSpec("")

	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true
	loader.ReadFromURIFunc = func(loader *openapi3.Loader, url *url.URL) ([]byte, error) {
		pathToFile := url.String()
		pathToFile = path.Clean(pathToFile)
		getSpec, ok := resolvePath[pathToFile]
		if !ok {
			err1 := fmt.Errorf("path not found: %s", pathToFile)
			return nil, err1
		}
		return getSpec()
	}
	var specData []byte
	specData, err = rawSpec()
	if err != nil {
		return
	}
	swagger, err = loader.LoadFromData(specData)
	if err != nil {
		return
	}
	return
}
