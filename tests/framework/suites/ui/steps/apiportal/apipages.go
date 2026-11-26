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
	"regexp"
	"strings"

	"github.com/cucumber/godog"
	playwright "github.com/mxschmitt/playwright-go"

	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
	"github.com/wso2/api-platform/tests/framework/core/util/unique"
)

const keyAPIPageAPIs = "uiAPIPortalAPIPageAPIs"

// apiPageRoles lists the APIs the API page fixtures create next to the REST details API.
var apiPageRoles = []string{"graphql", "websocket", "websub", "soap", "noplans"}

// asyncChannel is the channel the WebSocket and WebSub fixtures declare.
const asyncChannel = "/notifications"

// registerAPIPageSteps wires the steps of the API page, documentation and console journeys.
func (u *Steps) registerAPIPageSteps(sc *godog.ScenarioContext) {
	sc.Step(`^the API Portal has API page fixtures$`, u.seedAPIPageFixtures)
	sc.Step(`^the REST API page header offers Subscribe, Documentation, Try with AI and API Keys$`, u.restHeaderButtons)
	sc.Step(`^the (GraphQL|WebSocket|WebSub|SOAP) API page shows the sections of its type$`, u.typedAPIPageSections)
	sc.Step(`^the page of the API without plans has no plans panel and no Subscribe button, and offers API Keys$`, u.noPlansAPIPage)
	sc.Step(`^the GraphQL API Tryout offers the GraphiQL console with its production and sandbox endpoints$`, u.graphQLTryout)
	sc.Step(`^the (WebSocket|WebSub) API Tryout opens the AsyncAPI tryout$`, u.asyncTryout)
}

// seedAPIPageFixtures creates the REST details API and, next to it, a GraphQL API hidden from AI
// agents, WebSocket and WebSub APIs with one channel, a SOAP API, and a REST API with API key
// security and no plans.
func (u *Steps) seedAPIPageFixtures(ctx context.Context) error {
	if err := u.seedRESTAPIDetailsFixture(ctx); err != nil {
		return err
	}
	specs := map[string]catalogueAPI{
		"graphql":   {Type: "GRAPHQL", Badge: "GraphQL", Plans: []string{"Bronze"}, Hidden: true},
		"websocket": {Type: "WS", Badge: "WebSocket", Plans: []string{"AsyncUnlimited"}},
		"websub":    {Type: "WEBSUB", Badge: "WebSub", Plans: []string{"AsyncUnlimited"}},
		"soap":      {Type: "SOAP", Badge: "SOAP", Plans: []string{"Bronze"}},
		"noplans":   {Type: "REST", Badge: "REST", APIKey: true},
	}
	apis := map[string]catalogueAPI{}
	ids := make([]string, 0, len(apiPageRoles))
	for _, role := range apiPageRoles {
		api := specs[role]
		id, err := unique.Unique(ctx, "api-page-"+role)
		if err != nil {
			return err
		}
		api.ID = id
		api.Name = "IT API Page " + api.Badge + " " + id
		if err := u.createCatalogueAPI(ctx, api); err != nil {
			return err
		}
		apis[role] = api
		ids = append(ids, id)
	}
	raw, err := json.Marshal(apis)
	if err != nil {
		return err
	}
	if err := tcontext.Set(ctx, keyAPIPageAPIs, string(raw)); err != nil {
		return err
	}
	return u.waitForPortalCards(ctx, "/apis", ids...)
}

// apiPageAPIs returns the APIs the API page fixtures created.
func apiPageAPIs(ctx context.Context) (map[string]catalogueAPI, error) {
	raw, err := contextString(ctx, keyAPIPageAPIs)
	if err != nil {
		return nil, err
	}
	apis := map[string]catalogueAPI{}
	if err := json.Unmarshal([]byte(raw), &apis); err != nil {
		return nil, fmt.Errorf("decoding API page fixtures: %w", err)
	}
	return apis, nil
}

// asyncDefinition is an AsyncAPI document with one channel, for WebSocket and WebSub APIs.
func asyncDefinition(name string) map[string]any {
	return map[string]any{
		"asyncapi": "2.6.0", "info": map[string]string{"title": name, "version": "1.0.0"},
		"channels": map[string]any{asyncChannel: map[string]any{
			"subscribe": map[string]any{"message": map[string]any{"payload": map[string]string{"type": "string"}}},
			"publish":   map[string]any{"message": map[string]any{"payload": map[string]string{"type": "string"}}},
		}},
	}
}

// apiKeyRESTDefinition is a one-operation OpenAPI document secured by an API key header.
func apiKeyRESTDefinition(name string) map[string]any {
	definition := catalogueRESTDefinition(name)
	definition["components"] = map[string]any{"securitySchemes": map[string]any{
		"ApiKeyAuth": map[string]string{"type": "apiKey", "in": "header", "name": "apikey"},
	}}
	definition["security"] = []any{map[string]any{"ApiKeyAuth": []any{}}}
	return definition
}

// restHeaderButtons asserts the REST details API's header: Subscribe and Documentation, Try with
// AI because the API is visible to agents, API Keys because REST APIs offer them whatever their
// security schemes, and both of its plans in the plans panel.
func (u *Steps) restHeaderButtons(ctx context.Context) error {
	page, err := u.openRESTAPIDetailsPage(ctx, "")
	if err != nil {
		return err
	}
	actions := page.Locator(".aov-hero-actions")
	if err := u.expect.Locator(actions.Locator(`a[href="#subscriptionPlans"]`)).ToContainText("Subscribe"); err != nil {
		return err
	}
	if err := u.expect.Locator(actions.Locator(`a[href$="/docs/specification"]`)).ToContainText("Documentation"); err != nil {
		return err
	}
	if err := u.expect.Locator(actions.Locator(".dp-btn--ai")).ToContainText("Try with AI"); err != nil {
		return err
	}
	if err := u.expect.Locator(actions.Locator(`a[href$="/api-keys"]`)).ToContainText("API Keys"); err != nil {
		return fmt.Errorf("the REST API page did not offer API Keys: %w", err)
	}
	cards := page.Locator("#subscriptionPlans .aov-plan-card")
	if err := u.expect.Locator(cards).ToHaveCount(2); err != nil {
		return err
	}
	for _, plan := range []string{"Bronze", "Silver"} {
		if err := u.expect.Locator(cards.Filter(playwright.LocatorFilterOptions{HasText: plan})).ToHaveCount(1); err != nil {
			return fmt.Errorf("the plans panel has no %s card: %w", plan, err)
		}
	}
	return nil
}

// typedAPIPageSections opens an API of the given type and asserts the sections and header
// buttons that type gets. Only WebSocket APIs offer API Keys; GraphQL, WebSub and SOAP do not.
func (u *Steps) typedAPIPageSections(ctx context.Context, apiType string) error {
	apis, err := apiPageAPIs(ctx)
	if err != nil {
		return err
	}
	role := map[string]string{"GraphQL": "graphql", "WebSocket": "websocket", "WebSub": "websub", "SOAP": "soap"}[apiType]
	api := apis[role]
	page, err := u.openAPIPage(ctx, api)
	if err != nil {
		return err
	}
	actions := page.Locator(".aov-hero-actions")
	if err := u.expect.Locator(page.Locator(".aov-hero-badges .dp-badge").Filter(playwright.LocatorFilterOptions{HasText: api.Badge}).First()).ToBeVisible(); err != nil {
		return fmt.Errorf("the %s page has no %s badge: %w", apiType, api.Badge, err)
	}
	if err := u.expectCount(actions.Locator(`a[href$="/api-keys"]`), apiType == "WebSocket"); err != nil {
		return fmt.Errorf("the %s page's API Keys button: %w", apiType, err)
	}
	documentation := actions.Locator(`a[href$="/docs/specification"]`)
	switch apiType {
	case "GraphQL":
		if err := u.sectionTitles(page, "Endpoints"); err != nil {
			return err
		}
		if err := u.expect.Locator(actions.Locator(".dp-btn--ai")).ToHaveCount(0); err != nil {
			return fmt.Errorf("an API hidden from agents offered Try with AI: %w", err)
		}
		return u.expect.Locator(documentation).ToBeVisible()
	case "WebSocket", "WebSub":
		if err := u.sectionTitles(page, "Endpoints", "Channels"); err != nil {
			return err
		}
		row := page.Locator(".aov-channel-row").Filter(playwright.LocatorFilterOptions{HasText: asyncChannel})
		if err := u.expect.Locator(row.Locator(".aov-channel-badge--sub")).ToHaveCount(1); err != nil {
			return err
		}
		if err := u.expectCount(row.Locator(".aov-channel-badge--pub"), apiType == "WebSocket"); err != nil {
			return fmt.Errorf("the %s channel's PUB badge: %w", apiType, err)
		}
		return u.expect.Locator(documentation).ToBeVisible()
	default:
		if err := u.expect.Locator(documentation).ToHaveCount(0); err != nil {
			return fmt.Errorf("a SOAP API offered Documentation: %w", err)
		}
		download := actions.Locator("a[download]")
		if err := u.expect.Locator(download).ToContainText("Download"); err != nil {
			return err
		}
		return u.expect.Locator(download).ToHaveAttribute("href", regexp.MustCompile(`type=API_DEFINITION&fileName=definition\.wsdl`))
	}
}

// sectionTitles asserts that the API page shows a section with each title.
func (u *Steps) sectionTitles(page playwright.Page, titles ...string) error {
	for _, title := range titles {
		section := page.Locator(".aov-section-title").Filter(playwright.LocatorFilterOptions{HasText: regexp.MustCompile("^" + title + "$")})
		if err := u.expect.Locator(section).ToBeVisible(); err != nil {
			return fmt.Errorf("the API page has no %s section: %w", title, err)
		}
	}
	return nil
}

// openAPIPage opens an API's page and waits for its header.
func (u *Steps) openAPIPage(ctx context.Context, api catalogueAPI) (playwright.Page, error) {
	if err := u.openPortalPage(ctx, "/api/"+api.ID); err != nil {
		return nil, err
	}
	page, err := u.page(ctx)
	if err != nil {
		return nil, err
	}
	if err := u.expect.Locator(page.Locator(".aov-hero-name")).ToHaveText(api.Name); err != nil {
		return nil, err
	}
	return page, nil
}

// noPlansAPIPage asserts that an API without plans shows neither the plans panel nor Subscribe,
// while it still offers API Keys.
func (u *Steps) noPlansAPIPage(ctx context.Context) error {
	apis, err := apiPageAPIs(ctx)
	if err != nil {
		return err
	}
	page, err := u.openAPIPage(ctx, apis["noplans"])
	if err != nil {
		return err
	}
	if err := u.expect.Locator(page.Locator("#subscriptionPlans")).ToHaveCount(0); err != nil {
		return err
	}
	actions := page.Locator(".aov-hero-actions")
	if err := u.expect.Locator(actions.Locator(`a[href="#subscriptionPlans"]`)).ToHaveCount(0); err != nil {
		return err
	}
	return u.expect.Locator(actions.Locator(`a[href$="/api-keys"]`)).ToContainText("API Keys")
}

// graphQLTryout opens the GraphQL API's Tryout and asserts the console the portal renders: the
// endpoint selector with production and sandbox, and the token input.
func (u *Steps) graphQLTryout(ctx context.Context) error {
	page, err := u.openTryout(ctx, "graphql")
	if err != nil {
		return err
	}
	if err := u.expect.Locator(page.Locator(".graphql-tryout-container")).ToBeAttached(); err != nil {
		return err
	}
	options := page.Locator("#graphql-endpoint-type option")
	if err := u.expect.Locator(options).ToHaveText([]string{"Production", "Sandbox"}); err != nil {
		return err
	}
	if err := u.expect.Locator(page.Locator("#graphql-token-input")).ToBeAttached(); err != nil {
		return err
	}
	return u.expect.Locator(page.Locator("#graphql-schema-introspection-json")).ToBeAttached()
}

// asyncTryout opens a WebSocket or WebSub API's Tryout and asserts that the portal rendered the
// AsyncAPI viewer in tryout mode with the API's channel.
func (u *Steps) asyncTryout(ctx context.Context, apiType string) error {
	role := strings.ToLower(apiType)
	page, err := u.openTryout(ctx, role)
	if err != nil {
		return err
	}
	if err := u.expect.Locator(page.Locator("#root")).ToBeAttached(); err != nil {
		return err
	}
	content, err := page.Content()
	if err != nil {
		return fmt.Errorf("reading the %s Tryout: %w", apiType, err)
	}
	if !strings.Contains(content, "const isWebSocketTryout = true;") {
		return fmt.Errorf("the %s Tryout did not render the AsyncAPI viewer in tryout mode", apiType)
	}
	if !strings.Contains(content, asyncChannel) {
		return fmt.Errorf("the %s Tryout does not carry the %s channel", apiType, asyncChannel)
	}
	return nil
}

// openTryout opens an API's documentation, follows its Tryout link and returns the page.
func (u *Steps) openTryout(ctx context.Context, role string) (playwright.Page, error) {
	apis, err := apiPageAPIs(ctx)
	if err != nil {
		return nil, err
	}
	if err := u.openPortalPage(ctx, "/api/"+apis[role].ID+"/docs/specification"); err != nil {
		return nil, err
	}
	page, err := u.page(ctx)
	if err != nil {
		return nil, err
	}
	tryout := page.Locator(`.adoc-nav a[href$="/docs/specification?tryout=true"]`)
	if err := u.expect.Locator(tryout).ToContainText("Tryout"); err != nil {
		return nil, err
	}
	if err := tryout.Click(); err != nil {
		return nil, fmt.Errorf("opening the %s Tryout: %w", role, err)
	}
	if err := u.expect.Page(page).ToHaveURL(regexp.MustCompile(`tryout=true`)); err != nil {
		return nil, err
	}
	return page, nil
}
