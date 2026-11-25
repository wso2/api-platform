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
	"regexp"
	"strconv"
	"strings"
	"sync"

	"cel.dev/cel-go/cel"
	"cel.dev/cel-go/common/types"
	"github.com/getkin/kin-openapi/openapi3"

	commonconstants "github.com/wso2/api-platform/common/constants"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
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

// decode turns a JSON or YAML body into a BudgetRequest, read as its content type says
// (config.Parser.Parse's rule: JSON or YAML by type, either when the type is neither). The
// spec's BudgetRequest schema is the source of every field rule; defaults are filled in
// from it too. The one rule it cannot express is checked here.
func decode(body []byte, contentType string) (*api.BudgetRequest, error) {
	var doc any
	if err := new(config.Parser).Parse(body, contentType, &doc); err != nil {
		return nil, &ParseError{Cause: err}
	}
	// Round-trip through JSON so maps and numbers have the types the validator expects.
	raw, err := json.Marshal(doc)
	if err != nil {
		return nil, &ParseError{Cause: err}
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, fmt.Errorf("failed to decode budget: %w", err)
	}

	dropControlPlaneFields(value)

	// Read before the schema's defaults fill blockAt in: only a Block budget takes one.
	blockAtGiven := hasLimitField(value, "blockAt")
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

	if budget.Spec.Limit.Action == api.Allow {
		if blockAtGiven {
			return nil, &ValidationError{Errors: []config.ValidationError{{
				Field: "spec.limit.blockAt", Message: "applies only to the block action",
			}}}
		}
		budget.Spec.Limit.BlockAt = nil
	}
	if errs := labelKeyErrors(budget.Metadata.Labels); len(errs) > 0 {
		return nil, &ValidationError{Errors: errs}
	}
	if errs := selectionShapes(budget.Spec); len(errs) > 0 {
		return nil, &ValidationError{Errors: errs}
	}
	addProjectFilter(&budget)
	if errs := expressionErrors(budget.Spec); len(errs) > 0 {
		return nil, &ValidationError{Errors: errs}
	}
	if errs := projectOwner(budget); len(errs) > 0 {
		return nil, &ValidationError{Errors: errs}
	}
	return &budget, nil
}

// controlPlaneOnlyFields are the control plane's Budget fields the gateway has no use
// for: they are accepted, so the same Budget deploys on either side, and dropped before
// validation, so they are not checked, stored or returned.
var controlPlaneOnlyFields = []string{"displayName", "description", "warnings"}

// dropControlPlaneFields removes the control-plane-only fields from the body's spec.
func dropControlPlaneFields(value any) {
	doc, _ := value.(map[string]any)
	spec, _ := doc["spec"].(map[string]any)
	for _, field := range controlPlaneOnlyFields {
		delete(spec, field)
	}
}

// hasLimitField reports whether the body sets spec.limit.<name>.
func hasLimitField(value any, name string) bool {
	doc, _ := value.(map[string]any)
	spec, _ := doc["spec"].(map[string]any)
	limit, _ := spec["limit"].(map[string]any)
	_, ok := limit[name]
	return ok
}

// maxFilters is the schema's limit on spec.filters.
const maxFilters = 20

// addProjectFilter adds the owning project as the last filter when a budget with the
// project-id annotation has no project filter, as the control plane does: a project budget
// counts only its project's usage. The stored document, the response and the push to the
// control plane list it.
func addProjectFilter(budget *api.BudgetRequest) {
	if budget.Metadata.Annotations == nil {
		return
	}
	project := strings.TrimSpace((*budget.Metadata.Annotations)[commonconstants.AnnotationProjectID])
	if project == "" {
		return
	}
	var filters []api.BudgetFilter
	if budget.Spec.Filters != nil {
		filters = *budget.Spec.Filters
	}
	if len(filters) >= maxFilters {
		return // projectOwner says to list it
	}
	for _, f := range filters {
		if f.Type == api.BudgetFilterTypeProject {
			return
		}
	}
	filters = append(filters, api.BudgetFilter{Type: api.BudgetFilterTypeProject, Id: &project})
	budget.Spec.Filters = &filters
}

// projectOwner checks a budget owned by a project (the project-id annotation) is that
// project's own, as the control plane checks it: its project filter is that project, so
// the kind means the same wherever it is applied; it is not split by project; and it
// exempts only that project's agents. The annotation itself scopes nothing;
// addProjectFilter adds the filter when it is left out.
func projectOwner(budget api.BudgetRequest) []config.ValidationError {
	if budget.Metadata.Annotations == nil {
		return nil
	}
	project, ok := (*budget.Metadata.Annotations)[commonconstants.AnnotationProjectID]
	if !ok {
		return nil
	}
	project = strings.TrimSpace(project)
	if project == "" {
		return []config.ValidationError{{Field: "metadata.annotations", Message: commonconstants.AnnotationProjectID + " must name a project"}}
	}
	var errs []config.ValidationError
	listed := false
	if budget.Spec.Filters != nil {
		for i, f := range *budget.Spec.Filters {
			if f.Type != api.BudgetFilterTypeProject {
				continue
			}
			if f.Id == nil || *f.Id != project {
				errs = append(errs, config.ValidationError{
					Field:   fmt.Sprintf("spec.filters[%d].id", i),
					Message: fmt.Sprintf("a budget of project %s counts only that project: its project filter is that project", project),
				})
				continue
			}
			listed = true
		}
	}
	if !listed && len(errs) == 0 {
		errs = append(errs, config.ValidationError{
			Field:   "spec.filters",
			Message: fmt.Sprintf("a budget of project %s must filter on that project: {type: project, id: %s}", project, project),
		})
	}
	if split := budget.Spec.Limit.SplitBy; split != nil && split.Type == api.BudgetSplitTypeProject {
		errs = append(errs, config.ValidationError{Field: "spec.limit.splitBy.type", Message: fmt.Sprintf("a budget of project %s cannot be split by project", project)})
	}
	if budget.Spec.Exemptions != nil {
		for i, e := range *budget.Spec.Exemptions {
			field := fmt.Sprintf("spec.exemptions[%d]", i)
			switch {
			case e.Type == api.BudgetExemptionTypeProject:
				errs = append(errs, config.ValidationError{Field: field + ".type", Message: fmt.Sprintf("a budget of project %s cannot exempt a project", project)})
			case e.Type == api.BudgetExemptionTypeAgent && (e.Project == nil || *e.Project != project):
				errs = append(errs, config.ValidationError{Field: field + ".project", Message: fmt.Sprintf("a budget of project %s can exempt only its agents", project)})
			}
		}
	}
	return errs
}

// selectionShapes checks what the schema cannot say about each filter, the split and each
// exemption: an attribute takes only an expression; an llmProvider filter only its id; a
// project or API key filter only its id, or nothing for any one; an agent filter its id and
// project, or neither for any agent; each filter type but attribute is used once; a split takes an expression for an attribute and nothing else otherwise, and is not
// by a type that is also a filter, except attribute.
func selectionShapes(spec api.BudgetSpec) []config.ValidationError {
	var errs []config.ValidationError
	set := func(p *string) bool { return p != nil && *p != "" }
	add := func(field, message string) {
		errs = append(errs, config.ValidationError{Field: field, Message: message})
	}
	filtered := map[api.BudgetFilterType]bool{}
	// pinned marks the types a filter narrows to one value; such a type cannot be split.
	pinned := map[api.BudgetFilterType]bool{}
	if spec.Filters != nil {
		for i, f := range *spec.Filters {
			field := fmt.Sprintf("spec.filters[%d]", i)
			if f.Type != api.BudgetFilterTypeAttribute {
				if filtered[f.Type] {
					add(field+".type", "each filter type except attribute can be used once")
				}
				filtered[f.Type] = true
				if set(f.Id) {
					pinned[f.Type] = true
				}
			}
			switch f.Type {
			case api.BudgetFilterTypeAttribute:
				if !set(f.Expression) {
					add(field+".expression", "is required for an attribute")
				}
				if f.Id != nil || f.Project != nil {
					add(field, "an attribute takes an expression, not id or project")
				}
			default:
				if f.Expression != nil {
					add(field+".expression", "applies only to an attribute")
				}
				if f.Type == api.BudgetFilterTypeLlmProvider && !set(f.Id) {
					add(field+".id", "is required for an llmProvider")
				}
				if f.Type == api.BudgetFilterTypeAgent && set(f.Id) && !set(f.Project) {
					add(field+".project", "is required with an agent id: agent names are unique only per project")
				}
				if f.Type == api.BudgetFilterTypeAgent && !set(f.Id) && f.Project != nil {
					add(field+".id", "is required with project: give both for one agent, or neither for any agent")
				}
				if f.Type != api.BudgetFilterTypeAgent && f.Project != nil {
					add(field+".project", "applies only to an agent")
				}
			}
		}
	}
	if split := spec.Limit.SplitBy; split != nil {
		const field = "spec.limit.splitBy"
		if split.Type == api.BudgetSplitTypeAttribute {
			if !set(split.Expression) {
				add(field+".expression", "is required for an attribute")
			}
		} else {
			if split.Expression != nil {
				add(field+".expression", "applies only to an attribute")
			}
			if pinned[api.BudgetFilterType(split.Type)] {
				add(field+".type", "cannot be a type a filter narrows to one value: one value cannot be split")
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
// spec.filters[0].expression.
func schemaErrors(err error) []config.ValidationError {
	var out []config.ValidationError
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
			out = append(out, config.ValidationError{Field: fieldPath(se.JSONPointer()), Message: message})
			return
		}
		out = append(out, config.ValidationError{Message: e.Error()})
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

// celEnv is the environment the budget system policy compiles expressions in
// (system-policies/budget newCELEnv): the policy engine's request, with Auth.
var celEnv = sync.OnceValues(func() (*cel.Env, error) {
	return cel.NewEnv(cel.Variable("request", cel.MapType(cel.StringType, cel.DynType)))
})

// expressionErrors compiles every attribute expression as the budget system policy does,
// so a budget the policy would skip is refused instead: it must compile, and return bool
// as a condition (a filter or an exemption) or a string or list as the value a split
// divides the limit by. Claims are dynamic, so most types are known only at run.
func expressionErrors(spec api.BudgetSpec) []config.ValidationError {
	env, err := celEnv()
	if err != nil {
		return []config.ValidationError{{Field: "spec", Message: "expressions cannot be checked: " + err.Error()}}
	}
	var errs []config.ValidationError
	check := func(field, expression string, split bool) {
		ast, issues := env.Compile(expression)
		if issues != nil && issues.Err() != nil {
			errs = append(errs, config.ValidationError{Field: field, Message: "does not compile: " + issues.Err().Error()})
			return
		}
		out := ast.OutputType()
		if out == types.DynType {
			return
		}
		if split && out != types.StringType && out.Kind() != types.ListKind {
			errs = append(errs, config.ValidationError{Field: field, Message: fmt.Sprintf("must return a string or a list of strings for a split, not %s", out)})
		}
		if !split && out != types.BoolType {
			errs = append(errs, config.ValidationError{Field: field, Message: fmt.Sprintf("must return bool, not %s", out)})
			return
		}
		if tooCostly(env, ast) {
			errs = append(errs, config.ValidationError{Field: field, Message: "is too costly to run on every request"})
		}
	}
	if spec.Filters != nil {
		for i, f := range *spec.Filters {
			if f.Type == api.BudgetFilterTypeAttribute && f.Expression != nil {
				check(fmt.Sprintf("spec.filters[%d].expression", i), *f.Expression, false)
			}
		}
	}
	if split := spec.Limit.SplitBy; split != nil && split.Type == api.BudgetSplitTypeAttribute && split.Expression != nil {
		check("spec.limit.splitBy.expression", *split.Expression, true)
	}
	if spec.Exemptions != nil {
		for i, e := range *spec.Exemptions {
			if e.Type == api.BudgetExemptionTypeAttribute && e.Expression != nil {
				check(fmt.Sprintf("spec.exemptions[%d].expression", i), *e.Expression, false)
			}
		}
	}
	return errs
}

// maxExpressionCost is the budget system policy's cost limit for one expression per request
// (system-policies/budget maxExpressionCost): above it the policy stops the expression and
// the budget counts nothing.
const maxExpressionCost = 10000

// tooCostly runs the expression once on an empty request under the policy's cost limit.
// Claims are known only per request, so a static estimate cannot bound most expressions;
// one that exceeds the limit with no input (nested macros over big literal lists) always
// will. Other errors (a claim that is not there) are expected here and ignored.
func tooCostly(env *cel.Env, ast *cel.Ast) bool {
	prg, err := env.Program(ast, cel.CostLimit(maxExpressionCost))
	if err != nil {
		return false
	}
	_, _, err = prg.Eval(map[string]any{"request": map[string]any{}})
	return err != nil && strings.Contains(err.Error(), "cost limit exceeded")
}

// labelWord is one label's form, as the control plane's: 1-63 letters, digits, '.', '_'
// or '-', starting and ending with a letter or digit. The schema checks the count and the
// form; repeats are checked here.
var labelWord = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9._-]{0,61}[a-zA-Z0-9])?$`)

func labelKeyErrors(labels *[]string) []config.ValidationError {
	if labels == nil {
		return nil
	}
	var errs []config.ValidationError
	seen := map[string]bool{}
	for _, l := range *labels {
		switch {
		case !labelWord.MatchString(l):
			errs = append(errs, config.ValidationError{Field: "metadata.labels", Message: fmt.Sprintf(
				"label %q must be one word of 1-63 letters, digits, '.', '_' or '-', starting and ending with a letter or digit", l)})
		case seen[l]:
			errs = append(errs, config.ValidationError{Field: "metadata.labels", Message: fmt.Sprintf("label %q is listed twice", l)})
		}
		seen[l] = true
	}
	return errs
}
