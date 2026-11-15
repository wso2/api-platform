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

package budget

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/getkin/kin-openapi/openapi3"
	"gopkg.in/yaml.v3"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
)

var (
	schemaOnce sync.Once
	schema     *openapi3.Schema
	schemaErr  error
)

// requestSchema is the BudgetRequest schema from the embedded management API spec.
func requestSchema() (*openapi3.Schema, error) {
	schemaOnce.Do(func() {
		swagger, err := api.GetSwagger()
		if err != nil {
			schemaErr = fmt.Errorf("failed to load the management API spec: %w", err)
			return
		}
		ref, ok := swagger.Components.Schemas["BudgetRequest"]
		if !ok || ref.Value == nil {
			schemaErr = errors.New("the management API spec has no BudgetRequest schema")
			return
		}
		schema = ref.Value
	})
	return schema, schemaErr
}

// decode turns a JSON or YAML body into a BudgetRequest. The spec's
// BudgetRequest schema is the source of every field rule; defaults are filled
// in from it too. The one rule it cannot express is checked here.
func decode(body []byte) (*api.BudgetRequest, error) {
	var doc any
	if err := yaml.Unmarshal(body, &doc); err != nil {
		return nil, &ValidationError{Errors: []FieldError{{Message: "the body is not valid JSON or YAML"}}}
	}
	// Round-trip through JSON so maps and numbers have the types the validator expects.
	raw, err := json.Marshal(doc)
	if err != nil {
		return nil, &ValidationError{Errors: []FieldError{{Message: "the body is not valid JSON or YAML"}}}
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, fmt.Errorf("failed to decode budget: %w", err)
	}

	s, err := requestSchema()
	if err != nil {
		return nil, err
	}
	if err := s.VisitJSON(value, openapi3.MultiErrors(), openapi3.VisitAsRequest(), openapi3.DefaultsSet(func() {})); err != nil {
		return nil, &ValidationError{Errors: schemaErrors(err)}
	}

	if raw, err = json.Marshal(value); err != nil {
		return nil, fmt.Errorf("failed to encode budget: %w", err)
	}
	var budget api.BudgetRequest
	if err := json.Unmarshal(raw, &budget); err != nil {
		return nil, fmt.Errorf("failed to decode budget: %w", err)
	}

	if r := budget.Spec.Resource; r.Allocation == api.BudgetResourceAllocationSeparate && r.Id != nil {
		return nil, &ValidationError{Errors: []FieldError{{
			Field:   "spec.resource.id",
			Message: "must be omitted for separate allocation, which gives each provider its own allowance",
		}}}
	}
	if errs := consumerShapes(budget.Spec); len(errs) > 0 {
		return nil, &ValidationError{Errors: errs}
	}
	return &budget, nil
}

// consumerShapes checks what the schema cannot say about each consumer and exemption type:
// an attribute takes an expression; a project or API key its name (id) when shared; an
// agent its name and project when shared; each type but attribute is used once.
func consumerShapes(spec api.BudgetSpec) []FieldError {
	var errs []FieldError
	set := func(p *string) bool { return p != nil && *p != "" }
	add := func(field, message string) { errs = append(errs, FieldError{Field: field, Message: message}) }
	if spec.Consumers != nil {
		seen := map[api.BudgetConsumerType]bool{}
		for i, c := range *spec.Consumers {
			field := fmt.Sprintf("spec.consumers[%d]", i)
			shared := c.Allocation == api.BudgetConsumerAllocationShared
			if c.Type != api.BudgetConsumerTypeAttribute {
				if seen[c.Type] {
					add(field+".type", "each consumer type except attribute can be used once")
				}
				seen[c.Type] = true
			}
			switch c.Type {
			case api.BudgetConsumerTypeAttribute:
				if !set(c.Expression) {
					add(field+".expression", "is required for an attribute")
				}
				if c.Id != nil || c.Project != nil {
					add(field, "an attribute takes an expression, not id or project")
				}
			default:
				if c.Expression != nil {
					add(field+".expression", "applies only to an attribute")
				}
				if shared && !set(c.Id) {
					add(field+".id", "is required when shared")
				}
				if !shared && (c.Id != nil || c.Project != nil) {
					add(field, "a separate consumer names no id or project: each value gets its own allowance")
				}
				if c.Type == api.BudgetConsumerTypeAgent && shared && !set(c.Project) {
					add(field+".project", "is required for a shared agent: agent names are unique only per project")
				}
				if c.Type != api.BudgetConsumerTypeAgent && c.Project != nil {
					add(field+".project", "applies only to an agent")
				}
			}
		}
	}
	if spec.Exemptions != nil {
		for i, e := range *spec.Exemptions {
			field := fmt.Sprintf("spec.exemptions[%d]", i)
			switch e.Type {
			case api.BudgetExemptionTypeAttribute:
				if !set(e.Expression) || e.Id != nil || e.Project != nil {
					add(field, "an attribute exemption takes only an expression")
				}
			case api.BudgetExemptionTypeProject:
				if !set(e.Id) || e.Expression != nil || e.Project != nil {
					add(field, "a project exemption takes only its id")
				}
			case api.BudgetExemptionTypeAgent:
				if !set(e.Id) || !set(e.Project) || e.Expression != nil {
					add(field, "an agent exemption takes its id and project")
				}
			}
		}
	}
	return errs
}

// schemaErrors flattens the validator's errors into field paths such as
// spec.consumers[0].expression.
func schemaErrors(err error) []FieldError {
	var out []FieldError
	var walk func(error)
	walk = func(e error) {
		var multi openapi3.MultiError
		if errors.As(e, &multi) {
			for _, inner := range multi {
				walk(inner)
			}
			return
		}
		var se *openapi3.SchemaError
		if errors.As(e, &se) {
			message := se.Reason
			if se.SchemaField == "pattern" && se.Schema != nil && se.Schema.Description != "" {
				message = "has an invalid format: " + se.Schema.Description
			}
			out = append(out, FieldError{Field: fieldPath(se.JSONPointer()), Message: message})
			return
		}
		out = append(out, FieldError{Message: e.Error()})
	}
	walk(err)
	return out
}

func fieldPath(pointer []string) string {
	var b strings.Builder
	for _, part := range pointer {
		if _, err := strconv.Atoi(part); err == nil {
			b.WriteString("[" + part + "]")
			continue
		}
		if b.Len() > 0 {
			b.WriteByte('.')
		}
		b.WriteString(part)
	}
	return b.String()
}
