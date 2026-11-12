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

package apiportal

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/cucumber/godog"
	playwright "github.com/mxschmitt/playwright-go"

	"github.com/wso2/api-platform/tests/framework/core/cleanup"
	"github.com/wso2/api-platform/tests/framework/core/util/retry"
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
	"github.com/wso2/api-platform/tests/framework/core/util/unique"
)

// KindAPIPortalSubscription identifies a free subscription a developer holds in the API Portal.
var KindAPIPortalSubscription = cleanup.Kind{Name: "api-portal-subscription", Order: 14}

const (
	keyCatalogueAPIs      = "uiAPIPortalCatalogueAPIs"
	keyCatalogueEmptyView = "uiAPIPortalCatalogueEmptyView"

	// catalogueSOAPDefinition is the smallest WSDL the portal stores for a SOAP API.
	catalogueSOAPDefinition = `<?xml version="1.0"?><definitions xmlns="http://schemas.xmlsoap.org/wsdl/"></definitions>`
)

// catalogueAPI is one API the catalogue fixtures create, keyed by its role in the journey.
type catalogueAPI struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Type       string   `json:"type"`
	Badge      string   `json:"badge"`
	Tags       []string `json:"tags"`
	Plans      []string `json:"plans"`
	Hidden     bool     `json:"hidden"`
	Deprecated bool     `json:"deprecated"`
	APIKey     bool     `json:"apiKey"`
}

// catalogueRoles lists the APIs of the catalogue fixtures in a fixed order.
var catalogueRoles = []string{"rest", "graphql", "websocket", "websub", "mcp"}

// registerCatalogueSteps wires the catalogue journey steps.
func (u *Steps) registerCatalogueSteps(sc *godog.ScenarioContext) {
	sc.Step(`^the API Portal has catalogue fixtures$`, u.seedCatalogueFixtures)
	sc.Step(`^the developer subscribes to the catalogue REST API$`, u.subscribeToCatalogueREST)
	sc.Step(`^the user opens the API listing$`, u.openAPIListing)
	sc.Step(`^each catalogue API card shows its type, tags, plan count and badges, and no MCP server is listed$`, u.catalogueCardsShowDetails)
	sc.Step(`^the user searches the API listing for the catalogue (REST|GraphQL|MCP) (?:API|server)$`, u.searchCatalogue)
	sc.Step(`^the API listing shows only the catalogue (REST|GraphQL) API, counted once$`, u.catalogueShowsOnly)
	sc.Step(`^the API listing shows no catalogue API$`, u.catalogueShowsNone)
	sc.Step(`^the user clears the API listing search$`, u.clearCatalogueSearch)
	sc.Step(`^the API listing shows every catalogue API again$`, u.catalogueShowsAll)
	sc.Step(`^the user opens the API listing of a view with no APIs$`, u.openEmptyViewListing)
	sc.Step(`^the API listing shows that no APIs are available yet$`, u.listingIsEmpty)
}

// seedCatalogueFixtures creates a REST API with two plans and tags, a GraphQL API hidden from AI
// agents, a WebSocket API, a deprecated WebSub API, an MCP server and a view without labels.
func (u *Steps) seedCatalogueFixtures(ctx context.Context) error {
	apis := map[string]catalogueAPI{}
	specs := map[string]catalogueAPI{
		"rest":      {Type: "REST", Badge: "REST", Tags: []string{"payments", "beta"}, Plans: []string{"Bronze", "Silver"}},
		"graphql":   {Type: "GRAPHQL", Badge: "GraphQL", Plans: []string{"Bronze"}, Hidden: true},
		"websocket": {Type: "WS", Badge: "WebSocket", Plans: []string{"AsyncUnlimited"}},
		"websub":    {Type: "WEBSUB", Badge: "WebSub", Plans: []string{"AsyncUnlimited"}, Deprecated: true},
		"mcp":       {Type: "MCP", Badge: "MCP"},
	}
	for _, role := range catalogueRoles {
		api := specs[role]
		id, err := unique.Unique(ctx, "catalogue-"+role)
		if err != nil {
			return err
		}
		api.ID = id
		api.Name = "IT Catalogue " + api.Badge + " " + id
		if err := u.createCatalogueAPI(ctx, api); err != nil {
			return err
		}
		apis[role] = api
	}
	raw, err := json.Marshal(apis)
	if err != nil {
		return err
	}
	if err := tcontext.Set(ctx, keyCatalogueAPIs, string(raw)); err != nil {
		return err
	}
	view, err := u.createEmptyView(ctx)
	if err != nil {
		return err
	}
	if err := tcontext.Set(ctx, keyCatalogueEmptyView, view); err != nil {
		return err
	}
	ids := make([]string, 0, len(catalogueRoles)-1)
	for _, role := range catalogueRoles[:len(catalogueRoles)-1] {
		ids = append(ids, apis[role].ID)
	}
	if err := u.waitForPortalCards(ctx, "/apis", ids...); err != nil {
		return err
	}
	return u.waitForPortalCards(ctx, "/mcps", apis["mcp"].ID)
}

// createCatalogueAPI creates one catalogue API or MCP server and registers it for cleanup.
func (u *Steps) createCatalogueAPI(ctx context.Context, api catalogueAPI) error {
	status := "PUBLISHED"
	if api.Deprecated {
		status = "DEPRECATED"
	}
	visibility := "VISIBLE"
	if api.Hidden {
		visibility = "HIDDEN"
	}
	plans := make([]map[string]string, 0, len(api.Plans))
	for _, plan := range api.Plans {
		plans = append(plans, map[string]string{"id": plan})
	}
	metadata := map[string]any{
		"id": api.ID, "name": api.Name, "version": "v1.0", "type": api.Type, "status": status,
		"agentVisibility": visibility, "labels": []string{"default"}, "tags": api.Tags,
		"description": "Catalogue journey " + api.Badge + " API.",
		"endPoints": map[string]string{
			"productionURL": "https://backend.example.invalid/" + api.ID,
			"sandboxURL":    "https://sandbox.example.invalid/" + api.ID,
		},
	}
	if len(plans) > 0 {
		metadata["subscriptionPlans"] = plans
	}
	collection, kind := "apis", KindAPIPortalAPI
	var definition any
	fileName := "definition.json"
	switch api.Type {
	case "GRAPHQL":
		definition = "type Query { hello: String }\n"
	case "SOAP":
		definition, fileName = catalogueSOAPDefinition, "definition.wsdl"
	case "MCP":
		collection, kind = "mcp-servers", KindAPIPortalMCP
		definition = []map[string]any{{
			"type": "TOOL", "name": "hello", "description": "A greeting tool",
			"inputSchema": map[string]any{"type": "object"},
		}}
	case "WS", "WEBSUB":
		definition = asyncDefinition(api.Name)
	default:
		definition = catalogueRESTDefinition(api.Name)
		if api.APIKey {
			definition = apiKeyRESTDefinition(api.Name)
		}
	}
	if err := u.createPortalArtifactFile(ctx, "/api-portal/api/v0.9/"+collection, metadata, definition, fileName); err != nil {
		return fmt.Errorf("creating catalogue %s %q: %w", api.Type, api.ID, err)
	}
	if err := cleanup.Register(ctx, cleanup.Resource{Kind: kind, ID: api.ID, Actor: u.topo.Admin.Username}); err != nil {
		_ = u.deletePortalArtifact(ctx, collection, api.ID)
		return fmt.Errorf("registering catalogue %s %q for cleanup: %w", api.Type, api.ID, err)
	}
	return nil
}

// catalogueRESTDefinition is a one-operation OpenAPI document titled name.
func catalogueRESTDefinition(name string) map[string]any {
	return map[string]any{
		"openapi": "3.0.3", "info": map[string]string{"title": name, "version": "1.0.0"},
		"paths": map[string]any{"/ping": map[string]any{
			"get": map[string]any{"responses": map[string]any{"200": map[string]string{"description": "ok"}}},
		}},
	}
}

// createEmptyView creates a view with no labels, which surfaces no APIs, and registers it.
func (u *Steps) createEmptyView(ctx context.Context) (string, error) {
	page, err := u.page(ctx)
	if err != nil {
		return "", err
	}
	view, err := unique.Unique(ctx, "empty-view")
	if err != nil {
		return "", err
	}
	if err := cleanup.Register(ctx, cleanup.Resource{Kind: KindAPIPortalView, ID: view, Actor: u.topo.Admin.Username}); err != nil {
		return "", fmt.Errorf("registering view %q for cleanup: %w", view, err)
	}
	status, body, err := u.portalCall(page, http.MethodPost, "/views", map[string]any{
		"id": view, "displayName": "IT Empty View " + view, "labels": []string{},
	})
	if err != nil {
		return "", err
	}
	if status != http.StatusCreated {
		return "", fmt.Errorf("creating view %q: status %d %s", view, status, body)
	}
	return view, nil
}

// catalogueAPIs returns the APIs the catalogue fixtures created.
func catalogueAPIs(ctx context.Context) (map[string]catalogueAPI, error) {
	raw, err := contextString(ctx, keyCatalogueAPIs)
	if err != nil {
		return nil, err
	}
	apis := map[string]catalogueAPI{}
	if err := json.Unmarshal([]byte(raw), &apis); err != nil {
		return nil, fmt.Errorf("decoding catalogue fixtures: %w", err)
	}
	return apis, nil
}

// subscribeToCatalogueREST subscribes the signed-in developer to the catalogue REST API's Bronze
// plan and registers the subscription for cleanup.
func (u *Steps) subscribeToCatalogueREST(ctx context.Context) error {
	apis, err := catalogueAPIs(ctx)
	if err != nil {
		return err
	}
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	_, err = u.createSubscription(ctx, page, apis["rest"].ID, "Bronze")
	return err
}

// createSubscription subscribes the signed-in user to an API's free plan, registers the
// subscription for cleanup, and returns its id.
func (u *Steps) createSubscription(ctx context.Context, page playwright.Page, apiID, plan string) (string, error) {
	status, body, err := u.portalCall(page, http.MethodPost, "/subscriptions", map[string]string{
		"artifactId": apiID, "subscriptionPlanId": plan,
	})
	if err != nil {
		return "", err
	}
	if status != http.StatusCreated {
		return "", fmt.Errorf("subscribing to %q: status %d %s", apiID, status, body)
	}
	var created struct {
		SubscriptionID string `json:"subscriptionId"`
	}
	if err := json.Unmarshal([]byte(body), &created); err != nil || created.SubscriptionID == "" {
		return "", fmt.Errorf("reading the subscription to %q: %s", apiID, body)
	}
	username, _, err := u.credentials(ctx, "developer")
	if err != nil {
		return "", err
	}
	if err := cleanup.Register(ctx, cleanup.Resource{Kind: KindAPIPortalSubscription, ID: created.SubscriptionID, Actor: username}); err != nil {
		_, _, _ = u.portalCall(page, http.MethodDelete, "/subscriptions/"+created.SubscriptionID+"?immediate=true", nil)
		return "", fmt.Errorf("registering subscription %q for cleanup: %w", created.SubscriptionID, err)
	}
	return created.SubscriptionID, nil
}

// openAPIListing opens the APIs page of the default view.
func (u *Steps) openAPIListing(ctx context.Context) error {
	if err := u.openPortalPage(ctx, "/apis"); err != nil {
		return err
	}
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	return u.expect.Locator(page.Locator(".apilist-results-heading")).ToContainText("APIs")
}

// catalogueCardsShowDetails asserts each catalogue API's card: name, version, type badge, tags,
// plan count, AI Ready unless hidden from agents, Deprecated only on the deprecated API; and that
// the MCP server has no card.
func (u *Steps) catalogueCardsShowDetails(ctx context.Context) error {
	apis, err := catalogueAPIs(ctx)
	if err != nil {
		return err
	}
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	for _, role := range catalogueRoles[:len(catalogueRoles)-1] {
		api := apis[role]
		card := page.Locator("#apiCard-" + api.ID)
		if err := u.expect.Locator(card.Locator(".api-card-title")).ToHaveText(api.Name); err != nil {
			return fmt.Errorf("%s card: %w", role, err)
		}
		if err := u.expect.Locator(card.Locator(".api-version")).ToHaveText("v1.0"); err != nil {
			return fmt.Errorf("%s card: %w", role, err)
		}
		if err := u.expect.Locator(card.Locator(".api-card-meta-row > .dp-badge").First()).ToContainText(api.Badge); err != nil {
			return fmt.Errorf("%s card type badge: %w", role, err)
		}
		// The portal does not keep the order tags were given in.
		if err := u.expect.Locator(card.Locator(".dp-tag")).ToHaveCount(len(api.Tags)); err != nil {
			return fmt.Errorf("%s card tags: %w", role, err)
		}
		for _, tag := range api.Tags {
			if err := u.expect.Locator(card.Locator(".dp-tag").Filter(playwright.LocatorFilterOptions{HasText: regexpExact(tag)})).ToHaveCount(1); err != nil {
				return fmt.Errorf("%s card tag %q: %w", role, tag, err)
			}
		}
		planText := fmt.Sprintf("%d plan", len(api.Plans))
		if len(api.Plans) != 1 {
			planText += "s"
		}
		if err := u.expect.Locator(card.Locator(".api-plans-link")).ToHaveText(planText); err != nil {
			return fmt.Errorf("%s card plan count: %w", role, err)
		}
		if err := u.expectCount(card.Locator(".dp-badge--ai"), !api.Hidden); err != nil {
			return fmt.Errorf("%s card AI Ready badge: %w", role, err)
		}
		if err := u.expectCount(card.Locator(".dp-badge--deprecated"), api.Deprecated); err != nil {
			return fmt.Errorf("%s card Deprecated badge: %w", role, err)
		}
	}
	return u.expect.Locator(page.Locator("#apiCard-" + apis["mcp"].ID)).ToHaveCount(0)
}

// expectCount asserts that locator matches exactly one element when present, and none otherwise.
func (u *Steps) expectCount(locator playwright.Locator, present bool) error {
	if present {
		return u.expect.Locator(locator).ToHaveCount(1)
	}
	return u.expect.Locator(locator).ToHaveCount(0)
}

// searchCatalogue searches the API listing for a catalogue API's name with Enter.
func (u *Steps) searchCatalogue(ctx context.Context, which string) error {
	apis, err := catalogueAPIs(ctx)
	if err != nil {
		return err
	}
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	name := apis[strings.ToLower(which)].Name
	if err := page.Locator("#query").Fill(name); err != nil {
		return fmt.Errorf("entering the API listing search: %w", err)
	}
	if err := page.Locator("#query").Press("Enter"); err != nil {
		return fmt.Errorf("submitting the API listing search: %w", err)
	}
	return u.expect.Page(page).ToHaveURL(regexp.MustCompile(`[?&]query=`))
}

// catalogueShowsOnly asserts that the search lists only one catalogue API and counts one result.
func (u *Steps) catalogueShowsOnly(ctx context.Context, which string) error {
	apis, err := catalogueAPIs(ctx)
	if err != nil {
		return err
	}
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	want := apis[strings.ToLower(which)]
	if err := u.expect.Locator(page.Locator(".api-card")).ToHaveCount(1); err != nil {
		return fmt.Errorf("searching %q: %w", want.Name, err)
	}
	if err := u.expect.Locator(page.Locator("#apiCard-" + want.ID)).ToBeVisible(); err != nil {
		return err
	}
	return u.expect.Locator(page.Locator(".apilist-results-count")).ToHaveText("1")
}

// catalogueShowsNone asserts that the search lists no catalogue API.
func (u *Steps) catalogueShowsNone(ctx context.Context) error {
	apis, err := catalogueAPIs(ctx)
	if err != nil {
		return err
	}
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	for _, role := range catalogueRoles {
		if err := u.expect.Locator(page.Locator("#apiCard-" + apis[role].ID)).ToHaveCount(0); err != nil {
			return fmt.Errorf("the search listed the catalogue %s: %w", role, err)
		}
	}
	return nil
}

// clearCatalogueSearch empties the search box and submits it.
func (u *Steps) clearCatalogueSearch(ctx context.Context) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	if err := page.Locator("#query").Fill(""); err != nil {
		return fmt.Errorf("clearing the API listing search: %w", err)
	}
	return page.Locator("#query").Press("Enter")
}

// catalogueShowsAll asserts that every catalogue API is listed again after the search is cleared.
func (u *Steps) catalogueShowsAll(ctx context.Context) error {
	apis, err := catalogueAPIs(ctx)
	if err != nil {
		return err
	}
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	if err := u.expect.Page(page).Not().ToHaveURL(regexp.MustCompile(`[?&]query=[^&]`)); err != nil {
		return err
	}
	return retry.Await(ctx, retry.Options{}, func(context.Context) (bool, error) {
		for _, role := range catalogueRoles[:len(catalogueRoles)-1] {
			count, err := page.Locator("#apiCard-" + apis[role].ID).Count()
			if err != nil {
				return false, retry.Transient(err)
			}
			if count != 1 {
				return false, nil
			}
		}
		return true, nil
	}, func(ready bool) bool { return ready }, "waiting for the cleared API listing")
}

// openEmptyViewListing opens the APIs page of the view without labels.
func (u *Steps) openEmptyViewListing(ctx context.Context) error {
	view, err := contextString(ctx, keyCatalogueEmptyView)
	if err != nil {
		return err
	}
	base, err := u.apiPortalURL()
	if err != nil {
		return err
	}
	return u.navigate(ctx, base+"/api-portal/default/views/"+view+"/apis")
}

// listingIsEmpty asserts the empty state of an APIs page with nothing to list.
func (u *Steps) listingIsEmpty(ctx context.Context) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	status, err := navigationStatus(ctx)
	if err != nil {
		return err
	}
	if status != http.StatusOK {
		return fmt.Errorf("the empty view's APIs page status = %d, want 200", status)
	}
	if err := u.expect.Locator(page.Locator(".apilist-empty-title")).ToHaveText("No APIs available yet"); err != nil {
		return err
	}
	return u.expect.Locator(page.Locator(".api-card")).ToHaveCount(0)
}
