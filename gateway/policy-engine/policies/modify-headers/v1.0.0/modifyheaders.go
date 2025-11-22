package modifyheaders

import (
	"fmt"
	"strings"

	"github.com/policy-engine/sdk/policies"
)

// HeaderAction represents the action to perform on a header
type HeaderAction string

const (
	ActionSet    HeaderAction = "SET"
	ActionAppend HeaderAction = "APPEND"
	ActionDelete HeaderAction = "DELETE"
)

// HeaderModification represents a single header modification operation
type HeaderModification struct {
	Action HeaderAction
	Name   string
	Value  string
}

// ModifyHeadersPolicy implements comprehensive header manipulation for both request and response
type ModifyHeadersPolicy struct{}

// NewPolicy creates a new ModifyHeadersPolicy instance
func NewPolicy() policies.Policy {
	return &ModifyHeadersPolicy{}
}

// Validate validates the policy configuration
func (p *ModifyHeadersPolicy) Validate(params map[string]interface{}) error {
	// At least one of requestHeaders or responseHeaders must be present
	requestHeadersRaw, hasRequestHeaders := params["requestHeaders"]
	responseHeadersRaw, hasResponseHeaders := params["responseHeaders"]

	if !hasRequestHeaders && !hasResponseHeaders {
		return fmt.Errorf("at least one of 'requestHeaders' or 'responseHeaders' must be specified")
	}

	// Validate requestHeaders if present
	if hasRequestHeaders {
		if err := p.validateHeaderModifications(requestHeadersRaw, "requestHeaders"); err != nil {
			return err
		}
	}

	// Validate responseHeaders if present
	if hasResponseHeaders {
		if err := p.validateHeaderModifications(responseHeadersRaw, "responseHeaders"); err != nil {
			return err
		}
	}

	return nil
}

// validateHeaderModifications validates a list of header modifications
func (p *ModifyHeadersPolicy) validateHeaderModifications(headersRaw interface{}, fieldName string) error {
	headers, ok := headersRaw.([]interface{})
	if !ok {
		return fmt.Errorf("%s must be an array", fieldName)
	}

	if len(headers) == 0 {
		return fmt.Errorf("%s cannot be empty", fieldName)
	}

	for i, headerRaw := range headers {
		headerMap, ok := headerRaw.(map[string]interface{})
		if !ok {
			return fmt.Errorf("%s[%d] must be an object with 'action', 'name', and optionally 'value' fields", fieldName, i)
		}

		// Validate action
		actionRaw, ok := headerMap["action"]
		if !ok {
			return fmt.Errorf("%s[%d] missing required 'action' field", fieldName, i)
		}
		action, ok := actionRaw.(string)
		if !ok {
			return fmt.Errorf("%s[%d].action must be a string", fieldName, i)
		}
		action = strings.ToUpper(action)
		if action != string(ActionSet) && action != string(ActionAppend) && action != string(ActionDelete) {
			return fmt.Errorf("%s[%d].action must be SET, APPEND, or DELETE", fieldName, i)
		}

		// Validate name
		nameRaw, ok := headerMap["name"]
		if !ok {
			return fmt.Errorf("%s[%d] missing required 'name' field", fieldName, i)
		}
		name, ok := nameRaw.(string)
		if !ok {
			return fmt.Errorf("%s[%d].name must be a string", fieldName, i)
		}
		if len(name) == 0 {
			return fmt.Errorf("%s[%d].name cannot be empty", fieldName, i)
		}

		// Validate value for SET and APPEND actions
		if action == string(ActionSet) || action == string(ActionAppend) {
			valueRaw, ok := headerMap["value"]
			if !ok {
				return fmt.Errorf("%s[%d].value is required for %s action", fieldName, i, action)
			}
			_, ok = valueRaw.(string)
			if !ok {
				return fmt.Errorf("%s[%d].value must be a string", fieldName, i)
			}
		}
	}

	return nil
}

// parseHeaderModifications parses header modifications from config
func (p *ModifyHeadersPolicy) parseHeaderModifications(headersRaw interface{}) []HeaderModification {
	headers, ok := headersRaw.([]interface{})
	if !ok {
		return nil
	}

	modifications := make([]HeaderModification, 0, len(headers))
	for _, headerRaw := range headers {
		headerMap, ok := headerRaw.(map[string]interface{})
		if !ok {
			continue
		}

		mod := HeaderModification{
			Action: HeaderAction(strings.ToUpper(headerMap["action"].(string))),
			Name:   strings.ToLower(headerMap["name"].(string)), // Normalize to lowercase
		}

		if valueRaw, ok := headerMap["value"]; ok {
			mod.Value = valueRaw.(string)
		}

		modifications = append(modifications, mod)
	}

	return modifications
}

// applyHeaderModifications applies header modifications and returns the result
func (p *ModifyHeadersPolicy) applyHeaderModifications(modifications []HeaderModification) (map[string]string, []string, map[string][]string) {
	setHeaders := make(map[string]string)
	removeHeaders := []string{}
	appendHeaders := make(map[string][]string)

	for _, mod := range modifications {
		switch mod.Action {
		case ActionSet:
			setHeaders[mod.Name] = mod.Value
		case ActionDelete:
			removeHeaders = append(removeHeaders, mod.Name)
		case ActionAppend:
			appendHeaders[mod.Name] = []string{mod.Value}
		}
	}

	return setHeaders, removeHeaders, appendHeaders
}

// OnRequest modifies request headers
func (p *ModifyHeadersPolicy) OnRequest(ctx *policies.RequestContext, params map[string]interface{}) policies.RequestAction {
	// Check if requestHeaders are configured
	requestHeadersRaw, ok := params["requestHeaders"]
	if !ok {
		// No request headers to modify, pass through
		return policies.UpstreamRequestModifications{}
	}

	// Parse modifications
	modifications := p.parseHeaderModifications(requestHeadersRaw)
	if len(modifications) == 0 {
		return policies.UpstreamRequestModifications{}
	}

	// Apply modifications
	setHeaders, removeHeaders, appendHeaders := p.applyHeaderModifications(modifications)

	return policies.UpstreamRequestModifications{
		SetHeaders:    setHeaders,
		RemoveHeaders: removeHeaders,
		AppendHeaders: appendHeaders,
	}
}

// OnResponse modifies response headers
func (p *ModifyHeadersPolicy) OnResponse(ctx *policies.ResponseContext, params map[string]interface{}) policies.ResponseAction {
	// Check if responseHeaders are configured
	responseHeadersRaw, ok := params["responseHeaders"]
	if !ok {
		// No response headers to modify, pass through
		return policies.UpstreamResponseModifications{}
	}

	// Parse modifications
	modifications := p.parseHeaderModifications(responseHeadersRaw)
	if len(modifications) == 0 {
		return policies.UpstreamResponseModifications{}
	}

	// Apply modifications
	setHeaders, removeHeaders, appendHeaders := p.applyHeaderModifications(modifications)

	return policies.UpstreamResponseModifications{
		SetHeaders:    setHeaders,
		RemoveHeaders: removeHeaders,
		AppendHeaders: appendHeaders,
	}
}
