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
	"fmt"
	"log/slog"
	"net/url"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"

	"cel.dev/cel-go/cel"
	"cel.dev/cel-go/common/types"
	"github.com/google/uuid"
	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

// LazyResourceType is the lazy-resource type the gateway controller publishes budgets as.
const LazyResourceType = "Budget"

// Allocation and action values, as in the Budget kind.
const (
	allocationShared   = "shared"
	allocationSeparate = "separate"
	actionBlock        = "block"
)

// Allocated value types, used in allowance names.
const (
	valueTypeAttribute   = "attribute"
	valueTypeLLMProvider = "llmProvider"
)

// spendNamespace roots the IDs spend is counted under. Never change it: every gateway
// must derive the same IDs for the same budget.
var spendNamespace = uuid.MustParse("8d740d5b-3b73-4e13-84a6-5336674f813f")

// maxAllowancesPerBudget caps the allowances one request can charge for one budget. List
// claims allocated separately multiply; a request beyond the cap is not covered.
const maxAllowancesPerBudget = 64

// resourceSource is where budgets come from: the policy engine's lazy-resource store.
type resourceSource interface {
	Version() uint64
	GetResourcesByType(resourceType string) (map[string]*policy.LazyResource, error)
}

// budget is one active budget, ready to match requests.
type budget struct {
	uuid uuid.UUID
	name string
	// spendID roots the budget's allowance IDs: a UUIDv5 of its name, so the same budget on
	// every gateway counts against one total whatever UUID each gateway gave it, and the
	// shared store's keys hold no names.
	spendID  uuid.UUID
	amount   float64
	limit    string // the amount as written, for refusals
	currency string
	period   string
	action   string
	blockAt  int

	// providerID is the one LLM provider covered; empty for all of them.
	providerID string
	// providerSeparate gives each provider its own allowance.
	providerSeparate bool

	consumers  []consumer
	exemptions []cel.Program
}

// consumer is one consumer expression: a condition when shared, the value to allocate
// by when separate.
type consumer struct {
	separate bool
	program  cel.Program
}

// blockPoint is the spend at which a Block budget refuses: amount x blockAt %.
func (b *budget) blockPoint() float64 {
	return b.amount * float64(b.blockAt) / 100
}

// allocated is one value an allowance is separated by.
type allocated struct {
	valueType string
	value     string
}

// matches reports whether the budget covers the request, and the allowance names it
// charges: one when nothing is separate, otherwise one per combination of separate values.
func (b *budget) matches(provider string, activation map[string]any) ([]string, bool) {
	if b.providerID != "" && b.providerID != provider {
		return nil, false
	}
	for _, exemption := range b.exemptions {
		if ok, _ := evalBool(exemption, activation); ok {
			return nil, false
		}
	}

	combinations := [][]allocated{nil}
	for _, c := range b.consumers {
		if !c.separate {
			if ok, _ := evalBool(c.program, activation); !ok {
				return nil, false
			}
			continue
		}
		values, ok := evalValues(c.program, activation)
		if !ok {
			return nil, false
		}
		if len(combinations)*len(values) > maxAllowancesPerBudget {
			slog.Warn("Budget not applied: the request would charge too many allowances",
				slog.String("budget", b.name), slog.Int("limit", maxAllowancesPerBudget))
			return nil, false
		}
		next := make([][]allocated, 0, len(combinations)*len(values))
		for _, combination := range combinations {
			for _, value := range values {
				extended := append(append([]allocated{}, combination...), allocated{valueTypeAttribute, value})
				next = append(next, extended)
			}
		}
		combinations = next
	}
	if b.providerSeparate {
		for i := range combinations {
			combinations[i] = append(combinations[i], allocated{valueTypeLLMProvider, provider})
		}
	}

	names := make([]string, 0, len(combinations))
	for _, combination := range combinations {
		names = append(names, b.allowanceUUID(combination))
	}
	return names, true
}

// allowanceUUID is the UUIDv5, under the budget's spendID, of the allocated values sorted
// by type (stable, so consumers keep their order) and written type=value, percent-encoded,
// joined by "|". A shared budget has the UUID of the empty name.
func (b *budget) allowanceUUID(values []allocated) string {
	sorted := append([]allocated{}, values...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].valueType < sorted[j].valueType })
	parts := make([]string, len(sorted))
	for i, v := range sorted {
		parts[i] = v.valueType + "=" + url.QueryEscape(v.value)
	}
	return uuid.NewSHA1(b.spendID, []byte(strings.Join(parts, "|"))).String()
}

// evalBool runs a condition. An error, such as a missing claim, or a non-bool is false.
func evalBool(program cel.Program, activation map[string]any) (bool, error) {
	out, _, err := program.Eval(activation)
	if err != nil {
		return false, err
	}
	value, ok := out.Value().(bool)
	if !ok {
		return false, fmt.Errorf("expression returned %s, not bool", out.Type().TypeName())
	}
	return value, nil
}

// evalValues runs a separate consumer's expression: a string, or a list of strings. Empty
// strings are dropped; nothing left, an error or another type means no match.
func evalValues(program cel.Program, activation map[string]any) ([]string, bool) {
	out, _, err := program.Eval(activation)
	if err != nil {
		return nil, false
	}
	var values []string
	switch out.Type() {
	case types.StringType:
		values = []string{out.Value().(string)}
	case types.ListType:
		native, err := out.ConvertToNative(reflect.TypeOf([]string{}))
		if err != nil {
			return nil, false
		}
		values = native.([]string)
	default:
		return nil, false
	}
	seen := make(map[string]bool, len(values))
	kept := values[:0:0]
	for _, v := range values {
		if v != "" && !seen[v] {
			seen[v] = true
			kept = append(kept, v)
		}
	}
	return kept, len(kept) > 0
}

// catalog keeps the active budgets compiled, rebuilding them when the lazy-resource store
// changes.
type catalog struct {
	source resourceSource
	env    *cel.Env

	mu      sync.Mutex
	version uint64
	built   bool
	budgets []*budget
}

func newCatalog(source resourceSource) (*catalog, error) {
	env, err := newCELEnv()
	if err != nil {
		return nil, err
	}
	return &catalog{source: source, env: env}, nil
}

// newCELEnv declares the variables budget expressions use: the policy engine's request
// (Headers, Path, Method, Metadata) plus request.Auth (Claims, Subject, Type).
func newCELEnv() (*cel.Env, error) {
	return cel.NewEnv(cel.Variable("request", cel.MapType(cel.StringType, cel.DynType)))
}

// current returns the active budgets, sorted by name.
func (c *catalog) current() []*budget {
	c.mu.Lock()
	defer c.mu.Unlock()
	version := c.source.Version()
	if c.built && version == c.version {
		return c.budgets
	}
	resources, err := c.source.GetResourcesByType(LazyResourceType)
	if err != nil {
		slog.Error("Failed to read budgets; keeping the previous set", slog.Any("error", err))
		return c.budgets
	}
	budgets := make([]*budget, 0, len(resources))
	for id, resource := range resources {
		b, err := c.compile(resource.Resource)
		if err != nil {
			slog.Error("Budget skipped: it cannot be applied", slog.String("budget", id), slog.Any("error", err))
			continue
		}
		budgets = append(budgets, b)
	}
	sort.Slice(budgets, func(i, j int) bool { return budgets[i].name < budgets[j].name })
	c.budgets, c.version, c.built = budgets, version, true
	return budgets
}

// compile turns a published budget into a budget ready to match.
func (c *catalog) compile(body map[string]any) (*budget, error) {
	id, err := uuid.Parse(str(body["uuid"]))
	if err != nil {
		return nil, fmt.Errorf("uuid: %w", err)
	}
	limit, _ := body["limit"].(map[string]any)
	resource, _ := body["resource"].(map[string]any)
	if limit == nil || resource == nil {
		return nil, fmt.Errorf("limit and resource are required")
	}
	amount, err := strconv.ParseFloat(str(limit["amount"]), 64)
	if err != nil || amount <= 0 {
		return nil, fmt.Errorf("limit.amount %q is not a positive amount", str(limit["amount"]))
	}
	b := &budget{
		uuid:     id,
		name:     str(body["name"]),
		amount:   amount,
		limit:    str(limit["amount"]),
		currency: str(limit["currency"]),
		period:   str(limit["period"]),
		action:   str(limit["action"]),
		blockAt:  100,
	}
	if b.name == "" {
		return nil, fmt.Errorf("name is required")
	}
	b.spendID = uuid.NewSHA1(spendNamespace, []byte(b.name))
	switch b.period {
	case periodDay, periodWeek, periodMonth:
	default:
		return nil, fmt.Errorf("limit.period %q is not day, week or month", b.period)
	}
	if blockAt, ok := limit["blockAt"].(float64); ok {
		b.blockAt = int(blockAt)
	}
	switch str(resource["allocation"]) {
	case allocationShared:
		b.providerID = str(resource["id"])
	case allocationSeparate:
		b.providerSeparate = true
	default:
		return nil, fmt.Errorf("resource.allocation %q is not shared or separate", str(resource["allocation"]))
	}

	consumers, _ := body["consumers"].([]any)
	for i, raw := range consumers {
		entry, _ := raw.(map[string]any)
		separate := str(entry["allocation"]) == allocationSeparate
		program, err := c.program(str(entry["expression"]), separate)
		if err != nil {
			return nil, fmt.Errorf("consumers[%d]: %w", i, err)
		}
		b.consumers = append(b.consumers, consumer{separate: separate, program: program})
	}
	exemptions, _ := body["exemptions"].([]any)
	for i, raw := range exemptions {
		entry, _ := raw.(map[string]any)
		program, err := c.program(str(entry["expression"]), false)
		if err != nil {
			return nil, fmt.Errorf("exemptions[%d]: %w", i, err)
		}
		b.exemptions = append(b.exemptions, program)
	}
	return b, nil
}

// program compiles an expression and checks its declared result type where CEL knows it:
// bool for a condition; string or list for a separate value. Claims are dynamic, so most
// results are only checked when they run.
func (c *catalog) program(expression string, separate bool) (cel.Program, error) {
	ast, issues := c.env.Compile(expression)
	if issues != nil && issues.Err() != nil {
		return nil, fmt.Errorf("expression does not compile: %w", issues.Err())
	}
	out := ast.OutputType()
	if out != types.DynType {
		if separate && out != types.StringType && out.Kind() != types.ListKind {
			return nil, fmt.Errorf("a separate consumer's expression must return a string or a list of strings, not %s", out)
		}
		if !separate && out != types.BoolType {
			return nil, fmt.Errorf("expression must return bool, not %s", out)
		}
	}
	return c.env.Program(ast)
}

func str(v any) string {
	s, _ := v.(string)
	return s
}

// activation builds the CEL variables for a request.
func activation(headers map[string][]string, path, method string, metadata map[string]any, auth *policy.AuthContext) map[string]any {
	claims := map[string]any{}
	authVars := map[string]any{"Claims": claims, "Subject": "", "Type": ""}
	if auth != nil && auth.Authenticated {
		for k, v := range auth.TypedProperties {
			claims[k] = v
		}
		if auth.Subject != "" {
			claims["sub"] = auth.Subject
		}
		if auth.Issuer != "" {
			claims["iss"] = auth.Issuer
		}
		if len(auth.Audience) > 0 {
			claims["aud"] = auth.Audience
		}
		authVars["Subject"] = auth.Subject
		authVars["Type"] = auth.AuthType
	}
	if headers == nil {
		headers = map[string][]string{}
	}
	if metadata == nil {
		metadata = map[string]any{}
	}
	return map[string]any{"request": map[string]any{
		"Headers":  headers,
		"Path":     path,
		"Method":   method,
		"Metadata": metadata,
		"Auth":     authVars,
	}}
}
