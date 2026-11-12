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
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
	"github.com/wso2/api-platform/tests/framework/core/util/unique"
)

// KindAPIPortalAPIKey identifies an API key a developer generated, as "apiId/keyId". Keys are
// revoked before their API is deleted.
var KindAPIPortalAPIKey = cleanup.Kind{Name: "api-portal-api-key", Order: 12}

const (
	keyDeveloperAPI      = "uiAPIPortalDeveloperAPI"
	keySubscription      = "uiAPIPortalSubscription"
	keySubscriptionToken = "uiAPIPortalSubscriptionToken"
	keyAPIKeyName        = "uiAPIPortalAPIKeyName"
	keyAPIKeyID          = "uiAPIPortalAPIKeyID"
	keyAPIKeySecret      = "uiAPIPortalAPIKeySecret"
)

// registerDeveloperSteps wires the steps of the developer's applications, subscriptions and API
// keys journeys.
func (u *Steps) registerDeveloperSteps(sc *godog.ScenarioContext) {
	sc.Step(`^the API Portal has an API with two free plans and API key security$`, u.seedDeveloperAPI)
	sc.Step(`^the developer's Applications page invites them to create their first application$`, u.applicationsEmptyState)
	sc.Step(`^the application form refuses an empty name and stops a name at 100 characters$`, u.applicationFormValidation)
	sc.Step(`^the developer has an API key for that API$`, u.seedDeveloperAPIKey)
	sc.Step(`^the developer associates that API key with the application, then removes it$`, u.associateAndRemoveKey)
	sc.Step(`^the API key is still listed on the API's API Keys page$`, u.apiKeyStillListed)
	sc.Step(`^a signed-out visitor who clicks Subscribe on that API is sent to sign in$`, u.subscribeNeedsLogin)
	sc.Step(`^the developer subscribes to the Bronze plan on the API page and sees the token once$`, u.subscribeOnAPIPage)
	sc.Step(`^the API page offers to switch to the Silver plan$`, u.offersSwitchPlan)
	sc.Step(`^the developer reveals, regenerates, suspends and resumes the subscription in Subscriptions$`, u.manageSubscription)
	sc.Step(`^the developer switches the subscription to the Silver plan, which keeps its token$`, u.switchPlan)
	sc.Step(`^the developer unsubscribes in Subscriptions$`, u.unsubscribe)
	sc.Step(`^the developer's Subscriptions page has no subscriptions$`, u.subscriptionsEmpty)
	sc.Step(`^subscribing again gives the developer a new token$`, u.subscribeAgain)
	sc.Step(`^the developer generates an API key with an expiry on the API's API Keys page and sees it once$`, u.generateAPIKey)
	sc.Step(`^the API key is listed with its name, status, expiry and no application$`, u.apiKeyListed)
	sc.Step(`^the developer associates the API key with the application from the API Keys page$`, u.associateKeyFromKeysPage)
	sc.Step(`^filtering the API Keys page by that application lists the key$`, u.filterKeysByApplication)
	sc.Step(`^the developer regenerates the API key and sees a new key once$`, u.regenerateAPIKey)
	sc.Step(`^the Keys overview lists the API key with its API$`, u.keysOverviewListsKey)
	sc.Step(`^the developer revokes the API key$`, u.revokeAPIKey)
	sc.Step(`^the API key is listed as revoked, without regenerate or revoke actions$`, u.apiKeyRevoked)
}

// seedDeveloperAPI creates a REST API with the Bronze and Silver plans, secured by an API key.
func (u *Steps) seedDeveloperAPI(ctx context.Context) error {
	id, err := unique.Unique(ctx, "developer-api")
	if err != nil {
		return err
	}
	api := catalogueAPI{ID: id, Name: "IT Developer API " + id, Type: "REST", Badge: "REST", Plans: []string{"Bronze", "Silver"}, APIKey: true}
	if err := u.createCatalogueAPI(ctx, api); err != nil {
		return err
	}
	raw, err := json.Marshal(api)
	if err != nil {
		return err
	}
	if err := tcontext.Set(ctx, keyDeveloperAPI, string(raw)); err != nil {
		return err
	}
	return u.waitForPortalCards(ctx, "/apis", id)
}

// developerAPI returns the API seedDeveloperAPI created.
func developerAPI(ctx context.Context) (catalogueAPI, error) {
	var api catalogueAPI
	raw, err := contextString(ctx, keyDeveloperAPI)
	if err != nil {
		return api, err
	}
	return api, json.Unmarshal([]byte(raw), &api)
}

// applicationsEmptyState checks the Applications page of a developer with no applications.
func (u *Steps) applicationsEmptyState(ctx context.Context) error {
	if err := u.openPortalPage(ctx, "/applications"); err != nil {
		return err
	}
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	if err := u.expect.Locator(page.Locator(".dp-empty-title")).ToHaveText("No applications yet"); err != nil {
		return err
	}
	if err := u.expect.Locator(page.Locator(".app-card")).ToHaveCount(0); err != nil {
		return err
	}
	return u.expect.Locator(page.Locator("#apps-create-btn-empty")).ToBeVisible()
}

// applicationFormValidation opens the create form and checks that Create stays disabled with an
// empty or blank name and that the name field holds at most 100 characters. The form is closed
// without creating anything.
func (u *Steps) applicationFormValidation(ctx context.Context) error {
	if err := u.openPortalPage(ctx, "/applications"); err != nil {
		return err
	}
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	create := page.Locator("#app-create-confirm")
	return wizardAll(
		wizardClick(page, "#apps-create-btn, #apps-create-btn-empty"),
		u.wizardVisible(page, "#app-create-modal"),
		func() error { return u.expect.Locator(create).ToBeDisabled() },
		wizardFill(page, "#app-name-input", "   "),
		func() error { return u.expect.Locator(create).ToBeDisabled() },
		wizardFill(page, "#app-name-input", strings.Repeat("a", 101)),
		u.wizardHasValue(page, "#app-name-input", strings.Repeat("a", 100)),
		func() error { return u.expect.Locator(create).ToBeEnabled() },
		wizardClick(page, "#app-create-cancel"),
		u.wizardHidden(page, "#app-create-modal"),
		func() error { return u.expect.Locator(page.Locator(".app-card")).ToHaveCount(0) },
	)
}

// generateKeyByAPI generates an API key for the developer's API through the signed-in session
// and registers it for cleanup. It returns the key's id.
func (u *Steps) generateKeyByAPI(ctx context.Context, page playwright.Page, apiID, name string) (string, error) {
	status, body, err := u.portalCall(page, http.MethodPost, "/apis/"+apiID+"/api-keys/generate", map[string]string{"id": name})
	if err != nil {
		return "", err
	}
	if status != http.StatusCreated && status != http.StatusOK {
		return "", fmt.Errorf("generating API key %q: status %d", name, status)
	}
	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal([]byte(body), &created); err != nil || created.ID == "" {
		created.ID = name
	}
	if err := u.registerAPIKey(ctx, apiID, created.ID); err != nil {
		return "", err
	}
	return created.ID, nil
}

// registerAPIKey registers a developer's API key for revocation at cleanup.
func (u *Steps) registerAPIKey(ctx context.Context, apiID, keyID string) error {
	username, _, err := u.credentials(ctx, "developer")
	if err != nil {
		return err
	}
	return cleanup.Register(ctx, cleanup.Resource{Kind: KindAPIPortalAPIKey, ID: apiID + "/" + keyID, Actor: username})
}

// seedDeveloperAPIKey generates an API key for the developer's API.
func (u *Steps) seedDeveloperAPIKey(ctx context.Context) error {
	api, err := developerAPI(ctx)
	if err != nil {
		return err
	}
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	name, err := unique.Unique(ctx, "app-key")
	if err != nil {
		return err
	}
	name = strings.ReplaceAll(name, "_", "-")
	if _, err := u.generateKeyByAPI(ctx, page, api.ID, name); err != nil {
		return err
	}
	return tcontext.Set(ctx, keyAPIKeyName, name)
}

// associateAndRemoveKey associates the developer's API key with the application from the
// application page, checks that it is listed there, then removes the association.
func (u *Steps) associateAndRemoveKey(ctx context.Context) error {
	api, err := developerAPI(ctx)
	if err != nil {
		return err
	}
	name, err := contextString(ctx, keyAPIKeyName)
	if err != nil {
		return err
	}
	page, _, _, err := u.applicationDetails(ctx)
	if err != nil {
		return err
	}
	if err := u.expect.Locator(page.Locator("#no-associated-api-keys")).ToBeVisible(); err != nil {
		return err
	}
	if err := page.Locator("#btn-open-associate-key").Click(); err != nil {
		return fmt.Errorf("opening the associate dialog: %w", err)
	}
	if err := u.expect.Locator(page.Locator("#associate-key-modal")).ToBeVisible(); err != nil {
		return err
	}
	if _, err := page.Locator("#associate-key-api-select").SelectOption(playwright.SelectOptionValues{Values: &[]string{api.ID}}); err != nil {
		return fmt.Errorf("choosing the API: %w", err)
	}
	keyOption := page.Locator("#associate-key-select option").Filter(playwright.LocatorFilterOptions{HasText: name})
	if err := u.expect.Locator(keyOption).ToHaveCount(1); err != nil {
		return err
	}
	value, err := keyOption.GetAttribute("value")
	if err != nil {
		return err
	}
	if _, err := page.Locator("#associate-key-select").SelectOption(playwright.SelectOptionValues{Values: &[]string{value}}); err != nil {
		return fmt.Errorf("choosing the key: %w", err)
	}
	if _, err := page.ExpectNavigation(func() error { return page.Locator("#btn-submit-associate-key").Click() },
		playwright.PageExpectNavigationOptions{WaitUntil: playwright.WaitUntilStateDomcontentloaded}); err != nil {
		return fmt.Errorf("associating the key: %w", err)
	}
	table := page.Locator("#application-api-keys-table")
	if err := u.expect.Locator(table).ToContainText(name); err != nil {
		return err
	}
	remove := table.Locator(`.btn-remove-key-association[data-key-name="` + name + `"]`)
	if _, err := page.ExpectNavigation(func() error { return remove.Click() },
		playwright.PageExpectNavigationOptions{WaitUntil: playwright.WaitUntilStateDomcontentloaded}); err != nil {
		return fmt.Errorf("removing the association: %w", err)
	}
	return u.expect.Locator(page.Locator("#no-associated-api-keys")).ToBeVisible()
}

// openAPIKeysPage opens the API Keys page of the developer's API.
func (u *Steps) openAPIKeysPage(ctx context.Context) (playwright.Page, catalogueAPI, error) {
	api, err := developerAPI(ctx)
	if err != nil {
		return nil, api, err
	}
	if err := u.openPortalPage(ctx, "/api/"+api.ID+"/api-keys"); err != nil {
		return nil, api, err
	}
	page, err := u.page(ctx)
	if err != nil {
		return nil, api, err
	}
	return page, api, u.expect.Locator(page.Locator("#api-keys-config")).ToBeAttached()
}

// apiKeyRow finds an API key's row on an API Keys page by its name.
func apiKeyRow(page playwright.Page, table, name string) playwright.Locator {
	return page.Locator(table + " tbody tr").Filter(playwright.LocatorFilterOptions{
		Has: page.Locator(".ak-key-name", playwright.PageLocatorOptions{HasText: regexp.MustCompile("^" + regexp.QuoteMeta(name) + "$")}),
	})
}

// apiKeyStillListed checks that removing the association left the key on the API Keys page.
func (u *Steps) apiKeyStillListed(ctx context.Context) error {
	name, err := contextString(ctx, keyAPIKeyName)
	if err != nil {
		return err
	}
	page, _, err := u.openAPIKeysPage(ctx)
	if err != nil {
		return err
	}
	row := apiKeyRow(page, "#api-keys-table", name)
	if err := u.expect.Locator(row.Locator(".ak-status")).ToContainText("active"); err != nil {
		return err
	}
	return u.expect.Locator(row.Locator(".ak-app-name")).ToHaveCount(0)
}

// subscribeNeedsLogin signs out, clicks Subscribe on the API page and checks the login form, then
// signs the developer in.
func (u *Steps) subscribeNeedsLogin(ctx context.Context) error {
	api, err := developerAPI(ctx)
	if err != nil {
		return err
	}
	if err := u.logoutAPIPortal(ctx); err != nil {
		return err
	}
	page, err := u.openAPIPage(ctx, api)
	if err != nil {
		return err
	}
	subscribe := page.Locator(`#subscriptionPlans .aov-plan-card[data-policy-name="Bronze"] a.dp-btn`)
	if err := u.expect.Locator(subscribe).ToHaveText("Subscribe"); err != nil {
		return err
	}
	if err := subscribe.Click(); err != nil {
		return fmt.Errorf("clicking Subscribe signed out: %w", err)
	}
	if err := u.expect.Locator(page.Locator("#local-login-form")).ToBeVisible(); err != nil {
		return err
	}
	return u.signInThroughLocalFormAs(ctx, "developer")
}

// subscribeOnAPIPage subscribes to the Bronze plan from its card and checks the token dialog,
// then registers the subscription for cleanup.
func (u *Steps) subscribeOnAPIPage(ctx context.Context) error {
	api, err := developerAPI(ctx)
	if err != nil {
		return err
	}
	page, err := u.openAPIPage(ctx, api)
	if err != nil {
		return err
	}
	if err := page.Locator("#subscribe-btn-Bronze").Click(); err != nil {
		return fmt.Errorf("subscribing to Bronze: %w", err)
	}
	modal := page.Locator("#apiTokenModal")
	if err := u.expect.Locator(modal).ToBeVisible(); err != nil {
		return err
	}
	if err := u.expect.Locator(page.Locator("#apiTokenPlanName")).ToHaveText("Bronze"); err != nil {
		return err
	}
	token, err := page.Locator("#apiTokenValue").TextContent()
	if err != nil {
		return err
	}
	if strings.TrimSpace(token) == "" {
		return fmt.Errorf("the token dialog shows no token")
	}
	subscription, stored, err := u.currentSubscription(page, api.ID)
	if err != nil {
		return err
	}
	if stored != strings.TrimSpace(token) {
		return fmt.Errorf("the token dialog does not show the subscription's token")
	}
	username, _, err := u.credentials(ctx, "developer")
	if err != nil {
		return err
	}
	if err := cleanup.Register(ctx, cleanup.Resource{Kind: KindAPIPortalSubscription, ID: subscription, Actor: username}); err != nil {
		return err
	}
	if err := tcontext.Set(ctx, keySubscription, subscription); err != nil {
		return err
	}
	if err := tcontext.Set(ctx, keySubscriptionToken, stored); err != nil {
		return err
	}
	return page.Locator("#apiTokenModalDone").Click()
}

// currentSubscription reads the signed-in developer's subscription to an API and its token.
func (u *Steps) currentSubscription(page playwright.Page, apiID string) (string, string, error) {
	status, body, err := u.portalCall(page, http.MethodGet, "/subscriptions?artifactId="+apiID, nil)
	if err != nil {
		return "", "", err
	}
	if status != http.StatusOK {
		return "", "", fmt.Errorf("listing the subscriptions to %q: status %d", apiID, status)
	}
	var list struct {
		List []struct {
			SubscriptionID    string `json:"subscriptionId"`
			SubscriptionToken string `json:"subscriptionToken"`
			PlanName          string `json:"subscriptionPlanName"`
		} `json:"list"`
	}
	if err := json.Unmarshal([]byte(body), &list); err != nil {
		return "", "", fmt.Errorf("decoding the subscriptions to %q: %w", apiID, err)
	}
	if len(list.List) != 1 {
		return "", "", fmt.Errorf("the developer holds %d subscriptions to %q, want 1", len(list.List), apiID)
	}
	id := list.List[0].SubscriptionID
	status, body, err = u.portalCall(page, http.MethodGet, "/subscriptions/"+id, nil)
	if err != nil {
		return "", "", err
	}
	if status != http.StatusOK {
		return "", "", fmt.Errorf("reading subscription %q: status %d", id, status)
	}
	var one struct {
		SubscriptionToken string `json:"subscriptionToken"`
	}
	if err := json.Unmarshal([]byte(body), &one); err != nil {
		return "", "", err
	}
	return id, one.SubscriptionToken, nil
}

// offersSwitchPlan checks that, with a Bronze subscription, the Silver card offers Switch plan.
func (u *Steps) offersSwitchPlan(ctx context.Context) error {
	api, err := developerAPI(ctx)
	if err != nil {
		return err
	}
	page, err := u.openAPIPage(ctx, api)
	if err != nil {
		return err
	}
	if err := u.expect.Locator(page.Locator(`.aov-plan-card[data-policy-name="Bronze"] .api-view-sub-btn`)).ToContainText("View subscription"); err != nil {
		return err
	}
	return u.expect.Locator(page.Locator("#subscribe-btn-Silver")).ToContainText("Switch plan")
}

// openSubscriptionManage opens the Manage dialog of the subscription on the Subscriptions page.
func (u *Steps) openSubscriptionManage(ctx context.Context) (playwright.Page, string, error) {
	subscription, err := contextString(ctx, keySubscription)
	if err != nil {
		return nil, "", err
	}
	if err := u.openPortalPage(ctx, "/subscriptions"); err != nil {
		return nil, "", err
	}
	page, err := u.page(ctx)
	if err != nil {
		return nil, "", err
	}
	if err := page.Locator("#sub-row-" + subscription + " .sub-action-btn--manage").Click(); err != nil {
		return nil, "", fmt.Errorf("opening Manage subscription: %w", err)
	}
	return page, subscription, u.expect.Locator(page.Locator("#subManageModal")).ToBeVisible()
}

// manageSubscription reveals the token, regenerates it, and suspends and resumes the
// subscription, checking the dialog and the stored subscription after each action.
func (u *Steps) manageSubscription(ctx context.Context) error {
	api, err := developerAPI(ctx)
	if err != nil {
		return err
	}
	old, err := contextString(ctx, keySubscriptionToken)
	if err != nil {
		return err
	}
	page, _, err := u.openSubscriptionManage(ctx)
	if err != nil {
		return err
	}
	if err := wizardAll(
		u.wizardHasText(page, "#subManageApiName", api.Name),
		u.wizardHasText(page, "#subManagePlanName", "Bronze"),
		u.wizardHasText(page, "#subManageStatusBadge", "Active"),
		wizardClick(page, "#subManageRevealBtn"),
		u.wizardHasText(page, "#subManageTokenVal", old),
		wizardClick(page, "#subManageRegenerateBtn"),
		u.wizardVisible(page, "#subRegenerateDialog"),
		wizardClick(page, "#subRegenerateConfirmBtn"),
		func() error {
			return u.expectToast(page, "Token regenerated. Copy your new token before closing this window.")
		},
	); err != nil {
		return err
	}
	_, regenerated, err := u.currentSubscription(page, api.ID)
	if err != nil {
		return err
	}
	if regenerated == old || regenerated == "" {
		return fmt.Errorf("regenerating did not replace the token")
	}
	if err := u.expect.Locator(page.Locator("#subManageTokenVal")).ToHaveText(regenerated); err != nil {
		return err
	}
	if err := tcontext.Set(ctx, keySubscriptionToken, regenerated); err != nil {
		return err
	}
	for _, want := range []string{"Suspended", "Active"} {
		if err := page.Locator("#subManageSuspendBtn").Click(); err != nil {
			return fmt.Errorf("changing the subscription to %s: %w", want, err)
		}
		if err := u.expect.Locator(page.Locator("#subManageStatusBadge")).ToHaveText(want); err != nil {
			return err
		}
		if err := u.subscriptionStatus(ctx, page, want); err != nil {
			return err
		}
	}
	return nil
}

// subscriptionStatus checks the stored status of the scenario's subscription.
func (u *Steps) subscriptionStatus(ctx context.Context, page playwright.Page, want string) error {
	subscription, err := contextString(ctx, keySubscription)
	if err != nil {
		return err
	}
	stored := map[string]string{"Suspended": "INACTIVE", "Active": "ACTIVE"}[want]
	status, body, err := u.portalCall(page, http.MethodGet, "/subscriptions/"+subscription, nil)
	if err != nil {
		return err
	}
	if status != http.StatusOK {
		return fmt.Errorf("reading subscription %q: status %d", subscription, status)
	}
	var one struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal([]byte(body), &one); err != nil {
		return err
	}
	if one.Status != stored {
		return fmt.Errorf("subscription %q is stored as %s, want %s", subscription, one.Status, stored)
	}
	return nil
}

// switchPlan switches the subscription to Silver from the API page and checks the plan and that
// the token is kept.
func (u *Steps) switchPlan(ctx context.Context) error {
	api, err := developerAPI(ctx)
	if err != nil {
		return err
	}
	token, err := contextString(ctx, keySubscriptionToken)
	if err != nil {
		return err
	}
	page, err := u.openAPIPage(ctx, api)
	if err != nil {
		return err
	}
	if err := wizardAll(
		wizardClick(page, "#subscribe-btn-Silver"),
		u.wizardVisible(page, "#apiUnsubDialog"),
		u.wizardContains(page, "#apiUnsubTitle", "Switch to Silver plan?"),
		u.wizardContains(page, "#apiUnsubDesc", "Your subscription token will remain the same."),
		wizardClick(page, "#apiUnsubConfirmBtn"),
		u.wizardHidden(page, "#apiUnsubDialog"),
		// The page refreshes the plans after the switch; wait for it to settle on Silver.
		u.wizardContains(page, `.aov-plan-card[data-policy-name="Silver"] .api-view-sub-btn`, "View subscription"),
		func() error {
			return page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{State: playwright.LoadStateLoad})
		},
	); err != nil {
		return err
	}
	if err := u.openPortalPage(ctx, "/subscriptions"); err != nil {
		return err
	}
	subscription, err := contextString(ctx, keySubscription)
	if err != nil {
		return err
	}
	if err := u.expect.Locator(page.Locator("#sub-row-"+subscription)).ToHaveAttribute("data-plan-name", "Silver"); err != nil {
		return err
	}
	_, kept, err := u.currentSubscription(page, api.ID)
	if err != nil {
		return err
	}
	if kept != token {
		return fmt.Errorf("switching plans changed the subscription token")
	}
	return nil
}

// unsubscribe ends the subscription from its Manage dialog and checks that its row is gone.
func (u *Steps) unsubscribe(ctx context.Context) error {
	page, subscription, err := u.openSubscriptionManage(ctx)
	if err != nil {
		return err
	}
	if err := wizardAll(
		wizardClick(page, "#subManageUnsubBtn"),
		u.wizardVisible(page, "#subUnsubDialog"),
		wizardClick(page, "#subUnsubConfirmBtn"),
		func() error { return u.expectToast(page, "Subscription removed successfully!") },
	); err != nil {
		return err
	}
	if err := u.openPortalPage(ctx, "/subscriptions"); err != nil {
		return err
	}
	if err := u.expect.Locator(page.Locator("#sub-row-" + subscription)).ToHaveCount(0); err != nil {
		return err
	}
	registry, err := cleanup.Of(ctx)
	if err != nil {
		return err
	}
	registry.Deregister(KindAPIPortalSubscription, subscription)
	return nil
}

// subscriptionsEmpty checks the Subscriptions page of a developer without subscriptions.
func (u *Steps) subscriptionsEmpty(ctx context.Context) error {
	if err := u.openPortalPage(ctx, "/subscriptions"); err != nil {
		return err
	}
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	if err := u.expect.Locator(page.Locator("#no-subscriptions")).ToBeVisible(); err != nil {
		return err
	}
	return u.expect.Locator(page.Locator("[id^=sub-row-]")).ToHaveCount(0)
}

// subscribeAgain subscribes to Bronze again and checks that the new token differs from the last
// one the developer held.
func (u *Steps) subscribeAgain(ctx context.Context) error {
	previous, err := contextString(ctx, keySubscriptionToken)
	if err != nil {
		return err
	}
	if err := u.subscribeOnAPIPage(ctx); err != nil {
		return err
	}
	current, err := contextString(ctx, keySubscriptionToken)
	if err != nil {
		return err
	}
	if current == previous {
		return fmt.Errorf("subscribing again reused the previous token")
	}
	return nil
}

// generateAPIKey generates an API key with an expiry from the API Keys page, checks the one-time
// secret dialog, and registers the key for cleanup.
func (u *Steps) generateAPIKey(ctx context.Context) error {
	page, api, err := u.openAPIKeysPage(ctx)
	if err != nil {
		return err
	}
	name, err := unique.Unique(ctx, "journey-key")
	if err != nil {
		return err
	}
	name = strings.ReplaceAll(name, "_", "-")
	if err := wizardAll(
		wizardClick(page, "#btn-open-generate-api-key, #btn-open-generate-api-key-empty"),
		u.wizardVisible(page, "#generateApiKeyModal"),
		wizardFill(page, "#api-key-name", "-bad name"),
		wizardClick(page, "#btn-submit-generate-api-key"),
		func() error {
			return u.expectToast(page, "Enter a valid name: start with a letter or number, then up to 128 URL-safe characters.")
		},
		wizardFill(page, "#api-key-name", name),
		wizardFill(page, "#api-key-expires", "2099-12-31T23:59"),
		wizardClick(page, "#btn-submit-generate-api-key"),
		u.wizardVisible(page, "#showApiKeySecretModal"),
		u.wizardHasText(page, "#ak-secret-key-name", name),
	); err != nil {
		return err
	}
	secret, err := page.Locator("#api-key-secret-value").TextContent()
	if err != nil {
		return err
	}
	if strings.TrimSpace(secret) == "" {
		return fmt.Errorf("the secret dialog shows no key")
	}
	keyID, err := u.apiKeyID(page, api.ID, name)
	if err != nil {
		return err
	}
	if err := u.registerAPIKey(ctx, api.ID, keyID); err != nil {
		return err
	}
	for key, value := range map[string]string{keyAPIKeyName: name, keyAPIKeyID: keyID, keyAPIKeySecret: strings.TrimSpace(secret)} {
		if err := tcontext.Set(ctx, key, value); err != nil {
			return err
		}
	}
	if _, err := page.ExpectNavigation(func() error { return page.Locator("#ak-secret-done").Click() },
		playwright.PageExpectNavigationOptions{WaitUntil: playwright.WaitUntilStateDomcontentloaded}); err != nil {
		return fmt.Errorf("closing the secret dialog: %w", err)
	}
	return nil
}

// apiKeyID finds the id of the developer's API key with a name.
func (u *Steps) apiKeyID(page playwright.Page, apiID, name string) (string, error) {
	status, body, err := u.portalCall(page, http.MethodGet, "/apis/"+apiID+"/api-keys", nil)
	if err != nil {
		return "", err
	}
	if status != http.StatusOK {
		return "", fmt.Errorf("listing the API keys of %q: status %d", apiID, status)
	}
	var list struct {
		List []struct {
			ID          string `json:"id"`
			DisplayName string `json:"displayName"`
		} `json:"list"`
	}
	if err := json.Unmarshal([]byte(body), &list); err != nil {
		return "", fmt.Errorf("decoding the API keys of %q: %w", apiID, err)
	}
	for _, key := range list.List {
		if key.DisplayName == name || key.ID == name {
			return key.ID, nil
		}
	}
	return "", fmt.Errorf("API key %q is not listed for %q", name, apiID)
}

// apiKeyListed checks the generated key's row: its name, active status, an expiry date rather
// than Never, and no application.
func (u *Steps) apiKeyListed(ctx context.Context) error {
	name, err := contextString(ctx, keyAPIKeyName)
	if err != nil {
		return err
	}
	page, _, err := u.openAPIKeysPage(ctx)
	if err != nil {
		return err
	}
	row := apiKeyRow(page, "#api-keys-table", name)
	if err := u.expect.Locator(row.Locator(".ak-status")).ToContainText("active"); err != nil {
		return err
	}
	if err := u.expect.Locator(row.Locator(".ak-expires")).ToContainText("2099"); err != nil {
		return err
	}
	if err := u.expect.Locator(row.Locator(".ak-key-masked")).ToHaveText("••••••••"); err != nil {
		return err
	}
	return u.expect.Locator(row.Locator(".ak-app-btn--empty")).ToHaveCount(1)
}

// associateKeyFromKeysPage associates the generated key with the scenario's application from the
// API Keys page.
func (u *Steps) associateKeyFromKeysPage(ctx context.Context) error {
	name, err := contextString(ctx, keyAPIKeyName)
	if err != nil {
		return err
	}
	appID, err := contextString(ctx, keyAPIPortalApplication)
	if err != nil {
		return err
	}
	appName, err := contextString(ctx, keyAPIPortalApplicationName)
	if err != nil {
		return err
	}
	page, _, err := u.openAPIKeysPage(ctx)
	if err != nil {
		return err
	}
	if err := apiKeyRow(page, "#api-keys-table", name).Locator(".btn-app-key").Click(); err != nil {
		return fmt.Errorf("opening the application dialog: %w", err)
	}
	if err := u.expect.Locator(page.Locator("#ak-app-modal")).ToBeVisible(); err != nil {
		return err
	}
	if _, err := page.Locator("#ak-app-select").SelectOption(playwright.SelectOptionValues{Values: &[]string{appID}}); err != nil {
		return fmt.Errorf("choosing the application: %w", err)
	}
	if _, err := page.ExpectNavigation(func() error { return page.Locator("#btn-submit-app-association").Click() },
		playwright.PageExpectNavigationOptions{WaitUntil: playwright.WaitUntilStateDomcontentloaded}); err != nil {
		return fmt.Errorf("associating the application: %w", err)
	}
	return u.expect.Locator(apiKeyRow(page, "#api-keys-table", name).Locator(".ak-app-name")).ToHaveText(appName)
}

// filterKeysByApplication filters the API Keys page by the application and checks the key's row.
func (u *Steps) filterKeysByApplication(ctx context.Context) error {
	name, err := contextString(ctx, keyAPIKeyName)
	if err != nil {
		return err
	}
	appID, err := contextString(ctx, keyAPIPortalApplication)
	if err != nil {
		return err
	}
	page, _, err := u.openAPIKeysPage(ctx)
	if err != nil {
		return err
	}
	if _, err := page.Locator("#ak-app-filter").SelectOption(playwright.SelectOptionValues{Values: &[]string{appID}}); err != nil {
		return fmt.Errorf("filtering by application: %w", err)
	}
	return u.expect.Locator(apiKeyRow(page, "#api-keys-table", name)).ToBeVisible()
}

// regenerateAPIKey regenerates the key and checks that the one-time dialog shows a new key.
func (u *Steps) regenerateAPIKey(ctx context.Context) error {
	name, err := contextString(ctx, keyAPIKeyName)
	if err != nil {
		return err
	}
	old, err := contextString(ctx, keyAPIKeySecret)
	if err != nil {
		return err
	}
	page, _, err := u.openAPIKeysPage(ctx)
	if err != nil {
		return err
	}
	if err := wizardAll(
		func() error { return apiKeyRow(page, "#api-keys-table", name).Locator(".btn-regenerate-key").Click() },
		u.wizardVisible(page, "#regenerateApiKeyModal"),
		u.wizardHasValue(page, "#regenerate-api-key-name", name),
		wizardClick(page, "#btn-submit-regenerate-api-key"),
		u.wizardVisible(page, "#showApiKeySecretModal"),
	); err != nil {
		return err
	}
	secret, err := page.Locator("#api-key-secret-value").TextContent()
	if err != nil {
		return err
	}
	if secret = strings.TrimSpace(secret); secret == "" || secret == old {
		return fmt.Errorf("regenerating did not show a new key")
	}
	if _, err := page.ExpectNavigation(func() error { return page.Locator("#ak-secret-done").Click() },
		playwright.PageExpectNavigationOptions{WaitUntil: playwright.WaitUntilStateDomcontentloaded}); err != nil {
		return fmt.Errorf("closing the secret dialog: %w", err)
	}
	return tcontext.Set(ctx, keyAPIKeySecret, secret)
}

// keysOverviewListsKey checks that Keys > API Keys lists the key together with its API.
func (u *Steps) keysOverviewListsKey(ctx context.Context) error {
	name, err := contextString(ctx, keyAPIKeyName)
	if err != nil {
		return err
	}
	api, err := developerAPI(ctx)
	if err != nil {
		return err
	}
	if err := u.openPortalPage(ctx, "/api-keys"); err != nil {
		return err
	}
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	row := apiKeyRow(page, "#api-keys-overview-table", name)
	if err := u.expect.Locator(row).ToContainText(api.Name); err != nil {
		return err
	}
	return u.expect.Locator(row.Locator(".ak-row-revoke")).ToBeEnabled()
}

// revokeAPIKey revokes the key from the API Keys page through its confirmation dialog.
func (u *Steps) revokeAPIKey(ctx context.Context) error {
	name, err := contextString(ctx, keyAPIKeyName)
	if err != nil {
		return err
	}
	page, _, err := u.openAPIKeysPage(ctx)
	if err != nil {
		return err
	}
	if err := apiKeyRow(page, "#api-keys-table", name).Locator(".btn-revoke-key").Click(); err != nil {
		return fmt.Errorf("revoking the key: %w", err)
	}
	if err := u.expect.Locator(page.Locator("#ak-revoke-name")).ToHaveText(name); err != nil {
		return err
	}
	return page.Locator("#ak-revoke-confirm").Click()
}

// apiKeyRevoked checks the revoked key's row and deregisters it from cleanup.
func (u *Steps) apiKeyRevoked(ctx context.Context) error {
	name, err := contextString(ctx, keyAPIKeyName)
	if err != nil {
		return err
	}
	keyID, err := contextString(ctx, keyAPIKeyID)
	if err != nil {
		return err
	}
	page, api, err := u.openAPIKeysPage(ctx)
	if err != nil {
		return err
	}
	row := apiKeyRow(page, "#api-keys-table", name)
	if err := u.expect.Locator(row.Locator(".ak-status")).Not().ToContainText("active"); err != nil {
		return err
	}
	if err := u.expect.Locator(row.Locator(".btn-regenerate-key")).ToBeDisabled(); err != nil {
		return err
	}
	if err := u.expect.Locator(row.Locator(".btn-revoke-key")).ToBeDisabled(); err != nil {
		return err
	}
	registry, err := cleanup.Of(ctx)
	if err != nil {
		return err
	}
	registry.Deregister(KindAPIPortalAPIKey, api.ID+"/"+keyID)
	return nil
}
