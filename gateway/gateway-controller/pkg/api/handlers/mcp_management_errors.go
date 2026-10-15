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

package handlers

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/service/agent"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/service/restapi"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/templateengine"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/templateengine/funcs"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils"
)

// The functions in this file map a create/update failure from a kind's service
// to the message the equivalent management REST handler returns, so a model
// calling wso2_apip_gw_deploy_api or wso2_apip_gw_apply_config learns exactly
// what a REST client would. validation failures list every field, internal
// failures stay generic.
//
// Each per-kind function is a copy of its REST handler's error mapping, not a
// call into it.
// The comment above each one names the REST code it mirrors; when that REST
// code changes, change the copy here too.

// mcpWriteOp is the write a failure happened on.
type mcpWriteOp string

const (
	mcpWriteOpCreate mcpWriteOp = "create"
	mcpWriteOpUpdate mcpWriteOp = "update"
)

// mcpWriteErrorMapper holds one kind's copied REST error mapping, split by
// operation because several REST handlers map create and update differently.
type mcpWriteErrorMapper struct {
	Create func(err error) string
	Update func(handle string, err error) string
}

// mcpWriteErrorMappers has one entry for every kind in kindAliases
// (mcp_management_kinds.go). When adding a kind, copy its REST handler's
// create/update error mapping into an entry here;
// TestEveryKindHasAWriteErrorMapping fails until you do.
var mcpWriteErrorMappers = map[string]mcpWriteErrorMapper{
	models.KindRestApi: {
		Create: mcpRestAPICreateError,
		Update: mcpRestAPIUpdateError,
	},
	models.KindAgent: {
		// Create passes an empty handle, exactly as CreateAgent does.
		Create: func(err error) string { return mcpAgentWriteError("", err) },
		Update: mcpAgentWriteError,
	},
	models.KindLlmProxy: {
		Create: mcpLLMProxyCreateError,
		Update: mcpLLMProxyUpdateError,
	},
	models.KindLlmProviderTemplate: {
		Create: mcpLLMTemplateCreateError,
		Update: mcpLLMTemplateUpdateError,
	},
	models.KindLlmProvider: {
		Create: mcpLLMProviderWriteError,
		Update: func(_ string, err error) string { return mcpLLMProviderWriteError(err) },
	},
	models.KindMcp: {
		Create: mcpMCPProxyWriteError,
		Update: func(_ string, err error) string { return mcpMCPProxyWriteError(err) },
	},
	models.KindSecret: {
		// CreateSecret and UpdateSecret (secret_handler.go) return err.Error() on
		// every branch, so both slots do too. The entry must stay: without it, Secret
		// would fall into mcpWriteError's fail-closed default and every Secret error
		// would become a generic message.
		Create: func(err error) string { return err.Error() },
		Update: func(_ string, err error) string { return err.Error() },
	},
}

// mcpWriteError maps a failed create or update to the error the tool returns.
// handle is the id argument and is empty on create. The service error itself is
// never wrapped: only the mapped message reaches the model, and write() has
// already logged the original.
func mcpWriteError(kind string, op mcpWriteOp, handle string, err error, log *slog.Logger) error {
	// The write succeeded and only building the response failed. Running this
	// through the REST mapping would report the save itself as failed, and the
	// model would retry into a conflict.
	var readBack *mcpReadBackError
	if errors.As(err, &readBack) {
		return readBack
	}

	var msg string
	mapper, ok := mcpWriteErrorMappers[kind]
	switch {
	case !ok:
		// Fail closed: without a mapping there is no way to tell request detail
		// from internal detail, so neither is echoed.
		log.Warn("No MCP write-error mapping for kind; returning a generic error",
			slog.String("kind", kind))
		msg = "the Gateway could not complete the operation"
	case op == mcpWriteOpUpdate:
		msg = mapper.Update(handle, err)
	default:
		msg = mapper.Create(err)
	}

	if op == mcpWriteOpUpdate {
		return fmt.Errorf("failed to update %s %q: %s", kind, handle, msg)
	}
	return fmt.Errorf("failed to create %s: %s", kind, msg)
}

// mcpReadBackError is a failure building a tool's response after the service
// call succeeded. mcpWriteError returns it unchanged rather than mapping it.
type mcpReadBackError struct {
	msg string
}

func (e *mcpReadBackError) Error() string {
	return e.msg
}

// Shared helpers

// mcpFormatValidationErrors renders the validator's errors as the text form of
// the REST response's "errors" array: REST's message, then one numbered
// "field: message" line per entry.
func mcpFormatValidationErrors(validationErrors []config.ValidationError) string {
	noun := "errors"
	if len(validationErrors) == 1 {
		noun = "error"
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Configuration validation failed (%d %s):", len(validationErrors), noun)
	for i, e := range validationErrors {
		if e.Field == "" {
			fmt.Fprintf(&b, "\n  %d. %s", i+1, e.Message)
			continue
		}
		fmt.Fprintf(&b, "\n  %d. %s: %s", i+1, e.Field, e.Message)
	}
	return b.String()
}

// mcpRenderErrorMessage mirrors mapRenderError (helpers.go). It reports false
// when err is not a render error, so the caller moves on to its next check.
func mcpRenderErrorMessage(err error) (string, bool) {
	var renderErr *templateengine.RenderError
	if !errors.As(err, &renderErr) {
		return "", false
	}
	var secretErr *funcs.SecretError
	if errors.As(renderErr, &secretErr) {
		return secretErr.Error(), true
	}
	var tmplParseErr *templateengine.TemplateParseError
	if errors.As(renderErr, &tmplParseErr) {
		return tmplParseErr.Error(), true
	}
	return fmt.Sprintf("Failed to render configuration: %v", renderErr.Cause), true
}

// mcpIsRestAPICreateBadRequest mirrors isRestAPICreateBadRequest
// (rest_api_handler.go).
func mcpIsRestAPICreateBadRequest(err error) bool {
	if err == nil {
		return false
	}

	message := strings.ToLower(err.Error())
	return strings.Contains(message, "failed to parse configuration") ||
		strings.Contains(message, "resource kind is required") ||
		strings.Contains(message, "unsupported resource kind") ||
		strings.Contains(message, "invalid or missing origin")
}

// Per-kind mappings

// mcpRestAPICreateError mirrors RestAPIHandler.mapCreateError
// (rest_api_handler.go).
func mcpRestAPICreateError(err error) string {
	if msg, ok := mcpRenderErrorMessage(err); ok {
		return msg
	}

	if storage.IsConflictError(err) {
		return err.Error()
	}

	var validationErr *utils.ValidationErrorListError
	if errors.As(err, &validationErr) {
		return mcpFormatValidationErrors(validationErr.Errors)
	}

	if mcpIsRestAPICreateBadRequest(err) {
		return err.Error()
	}

	return "Failed to create configuration"
}

// mcpRestAPIUpdateError mirrors RestAPIHandler.mapUpdateError
// (rest_api_handler.go).
func mcpRestAPIUpdateError(handle string, err error) string {
	if msg, ok := mcpRenderErrorMessage(err); ok {
		return msg
	}

	var parseErr *restapi.ParseError
	if errors.As(err, &parseErr) {
		return fmt.Sprintf("Failed to parse configuration: %v", parseErr.Cause)
	}

	var handleErr *restapi.HandleMismatchError
	if errors.As(err, &handleErr) {
		return handleErr.Error()
	}

	var validationErr *restapi.ValidationError
	if errors.As(err, &validationErr) {
		return mcpFormatValidationErrors(validationErr.Errors)
	}

	if errors.Is(err, restapi.ErrNotFound) {
		return fmt.Sprintf("RestAPI with handle '%s' not found", handle)
	}

	if storage.IsConflictError(err) {
		return err.Error()
	}

	return "Failed to update configuration"
}

// mcpAgentWriteError mirrors AgentHandler.mapWriteError (agent_handler.go),
// which REST uses for both create and update.
func mcpAgentWriteError(handle string, err error) string {
	if msg, ok := mcpRenderErrorMessage(err); ok {
		return msg
	}

	var parseErr *agent.ParseError
	if errors.As(err, &parseErr) {
		return fmt.Sprintf("Failed to parse configuration: %v", parseErr.Cause)
	}

	var kindErr *agent.KindMismatchError
	if errors.As(err, &kindErr) {
		return kindErr.Error()
	}

	var handleErr *agent.HandleMismatchError
	if errors.As(err, &handleErr) {
		return handleErr.Error()
	}

	var validationErr *agent.ValidationError
	if errors.As(err, &validationErr) {
		return mcpFormatValidationErrors(validationErr.Errors)
	}

	if errors.Is(err, agent.ErrNotFound) {
		return fmt.Sprintf("Agent with handle '%s' not found", handle)
	}

	if storage.IsConflictError(err) {
		return err.Error()
	}

	if storage.IsDatabaseUnavailableError(err) {
		return "Database storage not available"
	}

	return "Failed to persist agent configuration"
}

// mcpLLMProxyCreateError mirrors the error handling in APIServer.CreateLLMProxy
// (llm_proxy_handler.go).
func mcpLLMProxyCreateError(err error) string {
	if msg, ok := mcpRenderErrorMessage(err); ok {
		return msg
	}
	if utils.IsPolicyDefinitionMissingError(err) {
		return utils.PolicyDefinitionMissingUserMessage
	}
	if errors.Is(err, utils.ErrLLMProxyValidation) {
		return err.Error()
	}
	if storage.IsNotFoundError(err) {
		return "LLM proxy configuration not found"
	}
	if storage.IsConflictError(err) {
		return err.Error()
	}
	return "Failed to create LLM proxy"
}

// mcpLLMProxyUpdateError mirrors the error handling in APIServer.UpdateLLMProxy
// (llm_proxy_handler.go). Like REST, it has no conflict branch.
func mcpLLMProxyUpdateError(handle string, err error) string {
	if msg, ok := mcpRenderErrorMessage(err); ok {
		return msg
	}
	if storage.IsNotFoundError(err) {
		return fmt.Sprintf("LLM proxy configuration with handle '%s' not found", handle)
	}
	if utils.IsPolicyDefinitionMissingError(err) {
		return utils.PolicyDefinitionMissingUserMessage
	}
	if errors.Is(err, utils.ErrLLMProxyValidation) {
		return err.Error()
	}
	return "Failed to update LLM proxy configuration"
}

// mcpLLMTemplateCreateError mirrors the error handling in
// APIServer.CreateLLMProviderTemplate (llm_provider_template_handler.go).
func mcpLLMTemplateCreateError(err error) string {
	if errors.Is(err, utils.ErrLLMTemplateValidation) {
		return err.Error()
	}
	if storage.IsConflictError(err) {
		return err.Error()
	}
	return "Failed to create LLM provider template"
}

// mcpLLMTemplateUpdateError mirrors the error handling in
// APIServer.UpdateLLMProviderTemplate (llm_provider_template_handler.go).
func mcpLLMTemplateUpdateError(handle string, err error) string {
	if errors.Is(err, utils.ErrLLMTemplateNotFound) {
		return fmt.Sprintf("Template with id '%s' not found", handle)
	}
	if errors.Is(err, utils.ErrLLMTemplateValidation) {
		return err.Error()
	}
	return "Failed to update LLM provider template"
}

// mcpLLMProviderWriteError mirrors the error handling in
// APIServer.CreateLLMProvider and APIServer.UpdateLLMProvider
// (llm_provider_handler.go), which is the same for both.
func mcpLLMProviderWriteError(err error) string {
	if msg, ok := mcpRenderErrorMessage(err); ok {
		return msg
	}
	if utils.IsPolicyDefinitionMissingError(err) {
		return utils.PolicyDefinitionMissingUserMessage
	}
	return err.Error()
}

// mcpMCPProxyWriteError mirrors the error handling in APIServer.CreateMCPProxy
// (mcp_proxy_handler.go), whose conflict and default branches both return
// err.Error().
//
// It is used for update too, deliberately NOT mirroring
// APIServer.UpdateMCPProxy: that handler reports every non-render failure,
// validation included, as "MCP configuration with handle '<id>' not found",
// which would hide exactly the detail this mapping exists to surface.
func mcpMCPProxyWriteError(err error) string {
	if msg, ok := mcpRenderErrorMessage(err); ok {
		return msg
	}
	return err.Error()
}
