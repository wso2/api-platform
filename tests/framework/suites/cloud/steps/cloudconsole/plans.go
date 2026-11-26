/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except in compliance
 * with the License. You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package cloudconsole

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/cucumber/godog"

	"github.com/wso2/api-platform/tests/framework/core/cleanup"
	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
)

const (
	subscriptionPlansCollection = "/subscription-plans"
	// keyPlanWord is the word every synthetic plan of a scenario carries in its display name, so a
	// search for it finds the scenario's plans and nothing else in the shared organisation.
	keyPlanWord = "cloudSubscriptionPlanWord"
	// keyPlanPrefix starts the context key holding the id of the plan a scenario names by a label.
	keyPlanPrefix = "cloudSubscriptionPlan:"
)

// planValueKeys are the plan fields a values table may set: status is a string, limits and
// pricing carry JSON.
var planValueKeys = map[string]bool{"status": true, "limits": true, "pricing": true}

// registerPlanSteps binds the steps that create, change, read and list subscription plans a
// scenario names by a label.
func (s *Steps) registerPlanSteps(sc stepRegistrar) {
	sc.Step(`^I submit the synthetic subscription plan "([^"]*)" with:$`, s.submitPlan)
	sc.Step(`^I submit another subscription plan with the id of synthetic subscription plan "([^"]*)" with:$`, s.submitDuplicatePlan)
	sc.Step(`^I update the synthetic subscription plan "([^"]*)" with:$`, s.updatePlan)
	sc.Step(`^I read the (synthetic subscription plan|pricing of the synthetic subscription plan) "([^"]*)"$`, s.readPlan)
	sc.Step(`^I list the scenario's synthetic subscription plans with a limit of (\d+)$`, s.listPlans)
	sc.Step(`^the cloud API response should list (\d+) of (\d+) subscription plans$`, s.assertPlanPage)
	sc.Step(`^the listed synthetic subscription plan "([^"]*)" should allow (\d+) requests per (MINUTE|HOUR|DAY)$`, s.assertListedPlanLimit)
	sc.Step(`^the subscription plan answer should have (\d+) limits?$`, s.assertPlanLimitCount)
	sc.Step(`^the subscription plan answer should carry no pricing$`, s.assertPlanHasNoPricing)
	sc.Step(`^the subscription plan answer should name the console user as its creator$`, s.assertPlanCreator)
	sc.Step(`^the subscription plan answer should be located at synthetic subscription plan "([^"]*)"$`, s.assertPlanLocation)
}

// submitPlan creates a plan named by label and publishes the answer. The plan is registered for
// cleanup before the call and dropped again when the platform refuses it.
func (s *Steps) submitPlan(ctx context.Context, label string, table *godog.Table) error {
	httpx.ClearPublished(ctx)
	key, err := planKey(label)
	if err != nil {
		return err
	}
	if optionalValue(ctx, key) != "" {
		return fmt.Errorf("the scenario already has a synthetic subscription plan %q", label)
	}
	if err := generateResourceAndStore(ctx, strings.TrimSuffix(syntheticPlanPrefix, "-"), key); err != nil {
		return err
	}
	id, err := requiredValue(ctx, key)
	if err != nil {
		return err
	}
	body, err := s.planBodyFrom(ctx, id, "Plan "+label, table)
	if err != nil {
		return err
	}
	if err := cleanup.Register(ctx, cleanup.Resource{
		Kind: cleanup.KindSubscriptionPlan, ID: id, Actor: resourceActor, Description: "subscription plan " + id,
	}); err != nil {
		return fmt.Errorf("registering subscription plan %q for cleanup: %w", id, err)
	}
	resp, err := s.doAuthenticated(ctx, http.MethodPost, subscriptionPlansCollection, body)
	if err != nil {
		return err
	}
	if refusedCreate(resp) {
		deregister(ctx, cleanup.KindSubscriptionPlan, id)
	}
	return nil
}

// submitDuplicatePlan sends a create that reuses the id of the plan named by label. The id is
// already registered for cleanup, so nothing new is.
func (s *Steps) submitDuplicatePlan(ctx context.Context, label string, table *godog.Table) error {
	httpx.ClearPublished(ctx)
	id, err := planID(ctx, label)
	if err != nil {
		return err
	}
	body, err := s.planBodyFrom(ctx, id, "Duplicate", table)
	if err != nil {
		return err
	}
	_, err = s.doAuthenticated(ctx, http.MethodPost, subscriptionPlansCollection, body)
	return err
}

// updatePlan replaces the plan named by label with the table's fields and publishes the answer,
// which may be a refusal.
func (s *Steps) updatePlan(ctx context.Context, label string, table *godog.Table) error {
	httpx.ClearPublished(ctx)
	id, err := planID(ctx, label)
	if err != nil {
		return err
	}
	body, err := s.planBodyFrom(ctx, id, "Plan "+label, table)
	if err != nil {
		return err
	}
	_, err = s.doAuthenticated(ctx, http.MethodPut, subscriptionPlansCollection+"/"+url.PathEscape(id), body)
	return err
}

// readPlan reads the plan named by label, or its pricing, and publishes the answer.
func (s *Steps) readPlan(ctx context.Context, what, label string) error {
	httpx.ClearPublished(ctx)
	id, err := planID(ctx, label)
	if err != nil {
		return err
	}
	path := subscriptionPlansCollection + "/" + url.PathEscape(id)
	if strings.HasPrefix(what, "pricing") {
		path += "/pricing"
	}
	_, err = s.doAuthenticated(ctx, http.MethodGet, path, nil)
	return err
}

// listPlans searches the plans for the scenario's word and publishes the page.
func (s *Steps) listPlans(ctx context.Context, limit int) error {
	httpx.ClearPublished(ctx)
	word, err := requiredValue(ctx, keyPlanWord)
	if err != nil {
		return err
	}
	query := url.Values{"query": {word}, "limit": {strconv.Itoa(limit)}}
	_, err = s.doAuthenticated(ctx, http.MethodGet, subscriptionPlansCollection+"?"+query.Encode(), nil)
	return err
}

// assertPlanPage checks the size of the published page and the total the platform reports.
func (s *Steps) assertPlanPage(ctx context.Context, listed, total int) error {
	page, response, err := publishedPlanPage(ctx)
	if err != nil {
		return err
	}
	if len(page.List) != listed || page.Pagination.Total == nil || *page.Pagination.Total != total {
		return fmt.Errorf("expected %d of %d subscription plans: %s", listed, total, response.Describe())
	}
	return nil
}

// assertListedPlanLimit checks the first limit of the listed plan named by label.
func (s *Steps) assertListedPlanLimit(ctx context.Context, label string, count int, unit string) error {
	id, err := planID(ctx, label)
	if err != nil {
		return err
	}
	page, response, err := publishedPlanPage(ctx)
	if err != nil {
		return err
	}
	for _, plan := range page.List {
		if plan.ID != id {
			continue
		}
		if len(plan.Limits) == 0 || plan.Limits[0].LimitCount != count || plan.Limits[0].TimeUnit != unit {
			return fmt.Errorf("expected plan %s to allow %d requests per %s: %s", id, count, unit, response.Describe())
		}
		return nil
	}
	return fmt.Errorf("the page does not list plan %s: %s", id, response.Describe())
}

// assertPlanLimitCount checks how many limits the published plan has.
func (s *Steps) assertPlanLimitCount(ctx context.Context, count int) error {
	plan, response, err := publishedPlan(ctx)
	if err != nil {
		return err
	}
	if len(plan.Limits) != count {
		return fmt.Errorf("expected %d limits: %s", count, response.Describe())
	}
	return nil
}

// assertPlanHasNoPricing checks that the published plan carries no pricing field.
func (s *Steps) assertPlanHasNoPricing(ctx context.Context) error {
	plan, response, err := publishedPlan(ctx)
	if err != nil {
		return err
	}
	if plan.Pricing != nil {
		return fmt.Errorf("expected no pricing: %s", response.Describe())
	}
	return nil
}

// consoleUserDisplayName is how the platform shows the console user as a resource's creator: the
// display name it takes from the token's name claim, or the user's id (sub) when the token has no
// name.
func consoleUserDisplayName(ctx context.Context) (string, error) {
	token, err := bearerToken(ctx)
	if err != nil {
		return "", err
	}
	_, claims, err := decodeJWT(token)
	if err != nil {
		return "", err
	}
	name, _ := claims["name"].(string)
	if strings.TrimSpace(name) == "" {
		name, _ = claims["sub"].(string)
	}
	if strings.TrimSpace(name) == "" {
		return "", fmt.Errorf("the console token carries neither a name nor a sub claim")
	}
	return name, nil
}

// assertPlanCreator checks that the published plan names the console user as its creator.
func (s *Steps) assertPlanCreator(ctx context.Context) error {
	plan, response, err := publishedPlan(ctx)
	if err != nil {
		return err
	}
	want, err := consoleUserDisplayName(ctx)
	if err != nil {
		return err
	}
	if plan.CreatedBy != want {
		return fmt.Errorf("expected the plan's creator to be the console user %q: %s", want, response.Describe())
	}
	return nil
}

// assertPlanLocation checks the Location header of the published create answer.
func (s *Steps) assertPlanLocation(ctx context.Context, label string) error {
	id, err := planID(ctx, label)
	if err != nil {
		return err
	}
	response, err := httpx.Published(ctx)
	if err != nil {
		return err
	}
	want := "/api/v0.9" + subscriptionPlansCollection + "/" + id
	if got := response.Headers.Get("Location"); got != want {
		return fmt.Errorf("expected the Location %q, got %q (%s)", want, got, response.Describe())
	}
	return nil
}

// planBodyFrom builds a plan request: the id, a display name that carries the scenario's word,
// and the table's fields.
func (s *Steps) planBodyFrom(ctx context.Context, id, name string, table *godog.Table) (map[string]any, error) {
	values, err := tableValues(table)
	if err != nil {
		return nil, err
	}
	word, err := scenarioPlanWord(ctx)
	if err != nil {
		return nil, err
	}
	body := map[string]any{"id": id, "displayName": word + " " + name}
	for key, raw := range values {
		if !planValueKeys[key] {
			return nil, fmt.Errorf("unsupported subscription plan value %q", key)
		}
		value, err := expandCloudValue(ctx, raw)
		if err != nil {
			return nil, err
		}
		if key == "status" {
			body[key] = value
			continue
		}
		var parsed any
		if err := json.Unmarshal([]byte(value), &parsed); err != nil {
			return nil, fmt.Errorf("subscription plan value %q is not valid JSON: %w", key, err)
		}
		body[key] = parsed
	}
	return body, nil
}

// scenarioPlanWord returns the scenario's plan word, generating it on first use.
func scenarioPlanWord(ctx context.Context) (string, error) {
	if word := optionalValue(ctx, keyPlanWord); word != "" {
		return word, nil
	}
	if err := generateResourceAndStore(ctx, "plans", keyPlanWord); err != nil {
		return "", err
	}
	return requiredValue(ctx, keyPlanWord)
}

func planKey(label string) (string, error) {
	label = strings.TrimSpace(label)
	if label == "" {
		return "", fmt.Errorf("a synthetic subscription plan needs a label")
	}
	return keyPlanPrefix + label, nil
}

func planID(ctx context.Context, label string) (string, error) {
	key, err := planKey(label)
	if err != nil {
		return "", err
	}
	id := optionalValue(ctx, key)
	if id == "" {
		return "", fmt.Errorf("the scenario has no synthetic subscription plan %q", label)
	}
	return id, nil
}

// subscriptionPlan is the part of a plan answer the assertions read.
type subscriptionPlan struct {
	ID        string `json:"id"`
	CreatedBy string `json:"createdBy"`
	Limits    []struct {
		TimeUnit   string `json:"timeUnit"`
		LimitCount int    `json:"limitCount"`
	} `json:"limits"`
	Pricing json.RawMessage `json:"pricing"`
}

func publishedPlan(ctx context.Context) (subscriptionPlan, *httpx.Response, error) {
	var plan subscriptionPlan
	response, err := httpx.Published(ctx)
	if err != nil {
		return plan, nil, err
	}
	if err := json.Unmarshal(response.Body, &plan); err != nil || plan.ID == "" {
		return plan, response, fmt.Errorf("the answer is not a subscription plan: %s", response.Describe())
	}
	return plan, response, nil
}

type planPage struct {
	List       []subscriptionPlan `json:"list"`
	Pagination struct {
		Total *int `json:"total"`
	} `json:"pagination"`
}

func publishedPlanPage(ctx context.Context) (planPage, *httpx.Response, error) {
	var page planPage
	response, err := httpx.Published(ctx)
	if err != nil {
		return page, nil, err
	}
	if err := json.Unmarshal(response.Body, &page); err != nil || page.List == nil {
		return page, response, fmt.Errorf("the answer is not a page of subscription plans: %s", response.Describe())
	}
	return page, response, nil
}
