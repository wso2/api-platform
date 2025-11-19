package setheader

import (
	"fmt"
	"strings"

	"github.com/yourorg/policy-engine/worker/policies"
)

// HeaderAction represents the action to perform on a header
type HeaderAction string

const (
	ActionSet    HeaderAction = "SET"
	ActionAppend HeaderAction = "APPEND"
	ActionDelete HeaderAction = "DELETE"
)

// SetHeaderPolicy implements header manipulation
// T081: SetHeaderPolicy struct implementation
type SetHeaderPolicy struct{}

// NewPolicy creates a new SetHeaderPolicy instance
// T088: NewPolicy factory function
func NewPolicy() policies.Policy {
	return &SetHeaderPolicy{}
}

// Name returns the policy name
// T082: SetHeaderPolicy.Name() implementation
func (p *SetHeaderPolicy) Name() string {
	return "setHeader"
}

// Validate validates the policy configuration
// T083: SetHeaderPolicy.Validate() implementation
func (p *SetHeaderPolicy) Validate(config map[string]interface{}) error {
	// Validate action parameter
	actionRaw, ok := config["action"]
	if !ok {
		return fmt.Errorf("action parameter is required")
	}

	action, ok := actionRaw.(string)
	if !ok {
		return fmt.Errorf("action must be a string")
	}

	action = strings.ToUpper(action)
	if action != string(ActionSet) && action != string(ActionAppend) && action != string(ActionDelete) {
		return fmt.Errorf("action must be SET, APPEND, or DELETE")
	}

	// Validate headerName parameter
	headerNameRaw, ok := config["headerName"]
	if !ok {
		return fmt.Errorf("headerName parameter is required")
	}

	headerName, ok := headerNameRaw.(string)
	if !ok {
		return fmt.Errorf("headerName must be a string")
	}

	if len(headerName) == 0 {
		return fmt.Errorf("headerName cannot be empty")
	}

	// Validate headerValue for SET and APPEND actions
	if action == string(ActionSet) || action == string(ActionAppend) {
		headerValueRaw, ok := config["headerValue"]
		if !ok {
			return fmt.Errorf("headerValue is required for %s action", action)
		}

		_, ok = headerValueRaw.(string)
		if !ok {
			return fmt.Errorf("headerValue must be a string")
		}
	}

	return nil
}

// ExecuteRequest processes request headers
// T084-T086: ExecuteRequest implementation for SET, DELETE, APPEND actions
func (p *SetHeaderPolicy) ExecuteRequest(ctx *policies.RequestContext, config map[string]interface{}) *policies.RequestPolicyAction {
	return p.executeHeaderModification(ctx.Headers, config)
}

// ExecuteResponse processes response headers
// T087: ExecuteResponse implementation
func (p *SetHeaderPolicy) ExecuteResponse(ctx *policies.ResponseContext, config map[string]interface{}) *policies.ResponsePolicyAction {
	headerMods := p.executeHeaderModification(ctx.ResponseHeaders, config)

	// Convert request action to response action
	if reqMods, ok := headerMods.Action.(policies.UpstreamRequestModifications); ok {
		return &policies.ResponsePolicyAction{
			Action: policies.UpstreamResponseModifications{
				SetHeaders:    reqMods.SetHeaders,
				RemoveHeaders: reqMods.RemoveHeaders,
				AppendHeaders: reqMods.AppendHeaders,
			},
		}
	}

	// Fallback: no modifications
	return &policies.ResponsePolicyAction{
		Action: policies.UpstreamResponseModifications{},
	}
}

// executeHeaderModification performs the header modification based on action
func (p *SetHeaderPolicy) executeHeaderModification(headers map[string][]string, config map[string]interface{}) *policies.RequestPolicyAction {
	action := strings.ToUpper(config["action"].(string))
	headerName := strings.ToLower(config["headerName"].(string)) // Normalize to lowercase

	mods := policies.UpstreamRequestModifications{
		SetHeaders:    make(map[string]string),
		RemoveHeaders: []string{},
		AppendHeaders: make(map[string][]string),
	}

	switch HeaderAction(action) {
	case ActionSet:
		// T084: SET action implementation
		headerValue := config["headerValue"].(string)
		mods.SetHeaders[headerName] = headerValue

	case ActionDelete:
		// T085: DELETE action implementation
		mods.RemoveHeaders = append(mods.RemoveHeaders, headerName)

	case ActionAppend:
		// T086: APPEND action implementation
		headerValue := config["headerValue"].(string)
		mods.AppendHeaders[headerName] = []string{headerValue}
	}

	return &policies.RequestPolicyAction{
		Action: mods,
	}
}
