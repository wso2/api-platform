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
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/cucumber/godog"
	playwright "github.com/mxschmitt/playwright-go"

	"github.com/wso2/api-platform/tests/framework/core/cleanup"
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
	"github.com/wso2/api-platform/tests/framework/core/util/unique"
)

const (
	keyGalleryWorkflows = "uiAPIPortalGalleryWorkflows"
	keyPromptAPI        = "uiAPIPortalPromptAPI"

	// galleryMarkdown is the body of the hidden Markdown workflow.
	galleryMarkdown = "# Gallery Markdown workflow\n\nSteps written in Markdown.\n"
)

// galleryWorkflow is one workflow the gallery fixtures create.
type galleryWorkflow struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
	Hidden bool   `json:"hidden"`
	Type   string `json:"type"`
}

// galleryRoles lists the gallery fixtures in a fixed order.
var galleryRoles = []string{"arazzo", "markdown", "draft"}

// registerWorkflowSteps wires the API workflow and Try with AI journey steps.
func (u *Steps) registerWorkflowSteps(sc *godog.ScenarioContext) {
	sc.Step(`^the API Portal has a published Arazzo workflow, a published workflow hidden from agents and a draft workflow$`, u.seedGalleryWorkflows)
	sc.Step(`^the API Workflows gallery lists only the published workflows, with AI Ready only on the visible one$`, u.galleryListsPublished)
	sc.Step(`^the Arazzo workflow opens its flow and the Markdown workflow its page$`, u.galleryWorkflowsOpen)
	sc.Step(`^the workflow form refuses to continue without a name or without content$`, u.workflowFormValidation)
	sc.Step(`^the user saves an API workflow as a draft, and the gallery does not list it until it is published$`, u.draftThenPublishWorkflow)
	sc.Step(`^the user changes the description of that workflow and hides it from agents$`, u.editWorkflow)
	sc.Step(`^the user deletes that workflow$`, u.deleteWorkflowFromSettings)
	sc.Step(`^the API Portal has an API visible to AI agents$`, u.seedPromptAPI)
	sc.Step(`^the API's Try with AI prompt points agents at the API and downloads as a text file$`, u.tryWithAIPrompt)
	sc.Step(`^the home page's Discover with AI prompt downloads as a text file$`, u.discoverWithAIPrompt)
}

// seedGalleryWorkflows creates, in the default view, a published Arazzo workflow visible to
// agents, a published Markdown workflow hidden from them, and a draft.
func (u *Steps) seedGalleryWorkflows(ctx context.Context) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	specs := map[string]galleryWorkflow{
		"arazzo":   {Status: "PUBLISHED", Type: "ARAZZO"},
		"markdown": {Status: "PUBLISHED", Type: "MD", Hidden: true},
		"draft":    {Status: "DRAFT", Type: "MD"},
	}
	workflows := map[string]galleryWorkflow{}
	for _, role := range galleryRoles {
		workflow := specs[role]
		id, err := unique.Unique(ctx, "gallery-"+role)
		if err != nil {
			return err
		}
		workflow.ID, workflow.Name = id, "IT Gallery "+role+" "+id
		if err := cleanup.Register(ctx, cleanup.Resource{Kind: KindAPIPortalWorkflow, ID: "default/" + id, Actor: u.topo.Admin.Username}); err != nil {
			return fmt.Errorf("registering workflow %q for cleanup: %w", id, err)
		}
		visibility := "VISIBLE"
		if workflow.Hidden {
			visibility = "HIDDEN"
		}
		body := map[string]any{
			"id": id, "displayName": workflow.Name, "description": "Gallery journey " + role + " workflow.",
			"agentPrompt": "Follow the " + role + " workflow.", "status": workflow.Status,
			"agentVisibility": visibility, "contentType": workflow.Type,
		}
		if workflow.Type == "ARAZZO" {
			body["apiWorkflowDefinition"] = galleryArazzo(workflow.Name)
		} else {
			body["markdownContent"] = galleryMarkdown
		}
		status, response, err := u.portalCall(page, http.MethodPost, "/views/default/api-workflows", body)
		if err != nil {
			return err
		}
		if status != http.StatusCreated {
			return fmt.Errorf("creating workflow %q: status %d %s", id, status, response)
		}
		workflows[role] = workflow
	}
	raw, err := json.Marshal(workflows)
	if err != nil {
		return err
	}
	return tcontext.Set(ctx, keyGalleryWorkflows, string(raw))
}

// galleryArazzo is a minimal Arazzo document with one workflow of one step.
func galleryArazzo(title string) map[string]any {
	return map[string]any{
		"arazzo": "1.0.0", "info": map[string]string{"title": title, "version": "1.0.0"},
		"sourceDescriptions": []map[string]string{{"name": "petstore", "url": "https://petstore.example.invalid/openapi.json", "type": "openapi"}},
		"workflows": []map[string]any{{
			"workflowId": "listPets", "summary": "List the pets.",
			"steps": []map[string]any{{"stepId": "list", "operationId": "listPets", "successCriteria": []map[string]string{{"condition": "$statusCode == 200"}}}},
		}},
	}
}

// galleryWorkflows returns the gallery fixtures.
func galleryWorkflows(ctx context.Context) (map[string]galleryWorkflow, error) {
	raw, err := contextString(ctx, keyGalleryWorkflows)
	if err != nil {
		return nil, err
	}
	workflows := map[string]galleryWorkflow{}
	if err := json.Unmarshal([]byte(raw), &workflows); err != nil {
		return nil, fmt.Errorf("decoding the gallery fixtures: %w", err)
	}
	return workflows, nil
}

// galleryCard finds a workflow's card in the gallery by its name.
func galleryCard(page playwright.Page, name string) playwright.Locator {
	return page.Locator(".wf-card").Filter(playwright.LocatorFilterOptions{Has: page.Locator(".wf-card-name", playwright.PageLocatorOptions{HasText: name})})
}

// galleryListsPublished opens the gallery and checks that both published workflows have a card,
// the draft has none, and only the agent-visible one is AI Ready.
func (u *Steps) galleryListsPublished(ctx context.Context) error {
	workflows, err := galleryWorkflows(ctx)
	if err != nil {
		return err
	}
	if err := u.openPortalPage(ctx, "/api-workflows"); err != nil {
		return err
	}
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	for _, role := range galleryRoles {
		workflow := workflows[role]
		card := galleryCard(page, workflow.Name)
		if role == "draft" {
			if err := u.expect.Locator(card).ToHaveCount(0); err != nil {
				return fmt.Errorf("the gallery lists a draft workflow: %w", err)
			}
			continue
		}
		if err := u.expect.Locator(card).ToHaveCount(1); err != nil {
			return fmt.Errorf("the gallery does not list the %s workflow: %w", role, err)
		}
		if err := u.expectCount(card.Locator(".dp-badge--ai"), !workflow.Hidden); err != nil {
			return fmt.Errorf("the %s workflow's AI Ready badge: %w", role, err)
		}
	}
	return nil
}

// galleryWorkflowsOpen opens both published workflows from the gallery: the Arazzo one renders
// its flow viewer from the stored specification, the Markdown one its Markdown viewer.
func (u *Steps) galleryWorkflowsOpen(ctx context.Context) error {
	workflows, err := galleryWorkflows(ctx)
	if err != nil {
		return err
	}
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	for _, check := range []struct {
		role, viewer, content string
	}{{"arazzo", "#arazzoViewer", `"workflowId":"listPets"`}, {"markdown", "#markdownViewer", "Gallery Markdown workflow"}} {
		if err := u.openPortalPage(ctx, "/api-workflows"); err != nil {
			return err
		}
		if err := galleryCard(page, workflows[check.role].Name).Click(); err != nil {
			return fmt.Errorf("opening the %s workflow: %w", check.role, err)
		}
		if err := u.expect.Page(page).ToHaveURL(regexpEscapedSuffix("/api-workflows/" + workflows[check.role].ID)); err != nil {
			return err
		}
		if err := u.expect.Locator(page.Locator(check.viewer)).ToBeAttached(); err != nil {
			return fmt.Errorf("the %s workflow has no %s: %w", check.role, check.viewer, err)
		}
		content, err := page.Locator("#specContent").InputValue()
		if err != nil {
			return fmt.Errorf("reading the %s workflow's content: %w", check.role, err)
		}
		if !strings.Contains(strings.ReplaceAll(content, " ", ""), strings.ReplaceAll(check.content, " ", "")) {
			return fmt.Errorf("the %s workflow page does not carry its content %q", check.role, check.content)
		}
	}
	return nil
}

// openWorkflowForm opens the create-workflow form of the default view.
func (u *Steps) openWorkflowForm(ctx context.Context) (playwright.Page, error) {
	page, err := u.openSettingsPanel(ctx, "cfg-workflows")
	if err != nil {
		return nil, err
	}
	if err := page.Locator("#createApiWorkflowBtn").Click(); err != nil {
		return nil, fmt.Errorf("opening the workflow form: %w", err)
	}
	if err := u.expect.Locator(page.Locator("#afStep1")).ToBeVisible(); err != nil {
		return nil, err
	}
	return page, nil
}

// workflowFormValidation checks that the form stays on its first step without a name, and on its
// second step without content. The form is left without saving.
func (u *Steps) workflowFormValidation(ctx context.Context) error {
	page, err := u.openWorkflowForm(ctx)
	if err != nil {
		return err
	}
	return wizardAll(
		wizardClick(page, "#afContinueBtn"),
		func() error {
			return u.expect.Locator(page.Locator("#apiWorkflowName")).ToHaveClass(regexpWord("is-invalid"))
		},
		u.wizardVisible(page, "#afStep1"),
		wizardFill(page, "#apiWorkflowName", "IT Workflow without content"),
		func() error {
			return u.expect.Locator(page.Locator("#apiWorkflowName")).Not().ToHaveClass(regexpWord("is-invalid"))
		},
		wizardClick(page, "#afContinueBtn"),
		u.wizardVisible(page, "#afStep2"),
		wizardClick(page, "#afContinueBtn"),
		func() error { return u.expectToast(page, "Add an API workflow spec before continuing") },
		u.wizardVisible(page, "#afStep2"),
		u.wizardHidden(page, "#afStep3"),
	)
}

// draftThenPublishWorkflow creates a Markdown workflow saved as a draft, checks that the gallery
// does not list it, publishes it from the editor and checks that the gallery lists it.
func (u *Steps) draftThenPublishWorkflow(ctx context.Context) error {
	page, err := u.openWorkflowForm(ctx)
	if err != nil {
		return err
	}
	value, err := unique.Unique(ctx, "draft-workflow")
	if err != nil {
		return err
	}
	name := "IT Draft Workflow " + value
	if err := wizardAll(
		wizardFill(page, "#apiWorkflowName", name),
		wizardFill(page, "#apiWorkflowDescription", "Saved as a draft first."),
		wizardClick(page, "#afContinueBtn"),
		u.wizardVisible(page, "#afStep2"),
		wizardChoose(page, "#arazoFileInput", wizardFile("workflow.md", "text/markdown", []byte("# Draft workflow\n"))),
		u.wizardContains(page, "#uploadFileName", "workflow.md"),
		wizardClick(page, "#afContinueBtn"),
		u.wizardVisible(page, "#afStep3"),
		wizardFill(page, "#agentPromptField", "Follow the draft workflow."),
	); err != nil {
		return err
	}
	handle, err := page.Locator("#apiWorkflowHandle").InputValue()
	if err != nil {
		return fmt.Errorf("reading the workflow handle: %w", err)
	}
	if err := cleanup.Register(ctx, cleanup.Resource{Kind: KindAPIPortalWorkflow, ID: "default/" + handle, Actor: u.topo.Admin.Username}); err != nil {
		return fmt.Errorf("registering workflow %q for cleanup: %w", handle, err)
	}
	if err := tcontext.Set(ctx, keyAPIPortalWorkflow, handle); err != nil {
		return err
	}
	if err := wizardAll(
		wizardClick(page, "#saveApiWorkflowGroup .af-save-toggle"),
		wizardClick(page, "#saveDraftBtn"),
	); err != nil {
		return err
	}
	if err := u.workflowRowStatus(page, name, "Draft"); err != nil {
		return err
	}
	if err := u.galleryCount(ctx, name, 0); err != nil {
		return err
	}
	page, err = u.openWorkflowEditor(ctx, handle)
	if err != nil {
		return err
	}
	if err := wizardAll(
		wizardClick(page, "#afContinueBtn"),
		wizardClick(page, "#afContinueBtn"),
		u.wizardVisible(page, "#afStep3"),
		wizardClick(page, "#saveApiWorkflowBtn"),
	); err != nil {
		return err
	}
	if err := u.workflowRowStatus(page, name, "Published"); err != nil {
		return err
	}
	return u.galleryCount(ctx, name, 1)
}

// workflowRow finds a workflow's row in the settings list by its name.
func workflowRow(page playwright.Page, name string) playwright.Locator {
	return page.Locator("#apiWorkflowList tr").Filter(playwright.LocatorFilterOptions{Has: page.Locator(".cfg-cell-name", playwright.PageLocatorOptions{HasText: name})})
}

// workflowRowStatus waits for the settings list to show the workflow with a status.
func (u *Steps) workflowRowStatus(page playwright.Page, name, status string) error {
	if err := u.expect.Locator(page.Locator("#apiWorkflowList")).ToBeVisible(); err != nil {
		return err
	}
	return u.expect.Locator(workflowRow(page, name).Locator(".cfg-status-badge")).ToHaveText(status)
}

// galleryCount checks how many gallery cards carry a workflow's name.
func (u *Steps) galleryCount(ctx context.Context, name string, want int) error {
	if err := u.openPortalPage(ctx, "/api-workflows"); err != nil {
		return err
	}
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	return u.expect.Locator(galleryCard(page, name)).ToHaveCount(want)
}

// openWorkflowEditor opens a workflow's editor from its row menu in the settings.
func (u *Steps) openWorkflowEditor(ctx context.Context, handle string) (playwright.Page, error) {
	page, err := u.openSettingsPanel(ctx, "cfg-workflows")
	if err != nil {
		return nil, err
	}
	edit := page.Locator(`.api-workflow-edit-btn[data-api-workflow-id="` + handle + `"]`)
	if err := edit.Locator("xpath=ancestor::td").Locator(".cfg-menu-trigger").Click(); err != nil {
		return nil, fmt.Errorf("opening the workflow actions: %w", err)
	}
	if err := edit.Click(); err != nil {
		return nil, fmt.Errorf("opening the workflow editor: %w", err)
	}
	if err := u.expect.Locator(page.Locator("#apiWorkflowForm")).ToBeVisible(); err != nil {
		return nil, err
	}
	return page, nil
}

// editWorkflow changes the workflow's description and hides it from agents, then checks the
// settings row and the gallery card.
func (u *Steps) editWorkflow(ctx context.Context) error {
	handle, err := contextString(ctx, keyAPIPortalWorkflow)
	if err != nil {
		return err
	}
	page, err := u.openWorkflowEditor(ctx, handle)
	if err != nil {
		return err
	}
	name, err := page.Locator("#apiWorkflowName").InputValue()
	if err != nil {
		return err
	}
	description := "Edited description of " + handle + "."
	if err := wizardAll(
		wizardFill(page, "#apiWorkflowDescription", description),
		wizardClick(page, "#agentHiddenCard"),
		wizardClick(page, "#afContinueBtn"),
		wizardClick(page, "#afContinueBtn"),
		u.wizardVisible(page, "#afStep3"),
		u.wizardVisible(page, "#agentHiddenBanner"),
		wizardClick(page, "#saveApiWorkflowBtn"),
	); err != nil {
		return err
	}
	if err := u.workflowRowStatus(page, name, "Published"); err != nil {
		return err
	}
	if err := u.expect.Locator(workflowRow(page, name).Locator(".cfg-cell-context")).ToHaveText(description); err != nil {
		return err
	}
	if err := u.openPortalPage(ctx, "/api-workflows"); err != nil {
		return err
	}
	card := galleryCard(page, name)
	if err := u.expect.Locator(card.Locator(".wf-card-desc")).ToHaveText(description); err != nil {
		return err
	}
	return u.expect.Locator(card.Locator(".dp-badge--ai")).ToHaveCount(0)
}

// deleteWorkflowFromSettings deletes the workflow from its row menu and checks that the settings
// list and the gallery no longer show it.
func (u *Steps) deleteWorkflowFromSettings(ctx context.Context) error {
	handle, err := contextString(ctx, keyAPIPortalWorkflow)
	if err != nil {
		return err
	}
	page, err := u.openSettingsPanel(ctx, "cfg-workflows")
	if err != nil {
		return err
	}
	remove := page.Locator(`.api-workflow-delete-btn[data-api-workflow-id="` + handle + `"]`)
	row := remove.Locator("xpath=ancestor::tr")
	name, err := row.Locator(".cfg-cell-name").TextContent()
	if err != nil {
		return fmt.Errorf("reading the workflow name: %w", err)
	}
	if err := remove.Locator("xpath=ancestor::td").Locator(".cfg-menu-trigger").Click(); err != nil {
		return fmt.Errorf("opening the workflow actions: %w", err)
	}
	if err := remove.Click(); err != nil {
		return fmt.Errorf("choosing Delete: %w", err)
	}
	if err := u.expect.Locator(page.Locator("#deleteApiWorkflowNameTxt")).ToHaveText(name); err != nil {
		return err
	}
	if err := page.Locator("#deleteApiWorkflowConfirmBtn").Click(); err != nil {
		return fmt.Errorf("confirming Delete: %w", err)
	}
	if err := u.expect.Locator(page.Locator(`.api-workflow-delete-btn[data-api-workflow-id="` + handle + `"]`)).ToHaveCount(0); err != nil {
		return err
	}
	registry, err := cleanup.Of(ctx)
	if err != nil {
		return err
	}
	registry.Deregister(KindAPIPortalWorkflow, "default/"+handle)
	return u.galleryCount(ctx, name, 0)
}

// seedPromptAPI creates a REST API visible to agents for the Try with AI prompt.
func (u *Steps) seedPromptAPI(ctx context.Context) error {
	id, err := unique.Unique(ctx, "prompt-api")
	if err != nil {
		return err
	}
	api := catalogueAPI{ID: id, Name: "IT Prompt API " + id, Type: "REST", Badge: "REST", Plans: []string{"Bronze"}}
	if err := u.createCatalogueAPI(ctx, api); err != nil {
		return err
	}
	raw, err := json.Marshal(api)
	if err != nil {
		return err
	}
	if err := tcontext.Set(ctx, keyPromptAPI, string(raw)); err != nil {
		return err
	}
	return u.waitForPortalCards(ctx, "/apis", id)
}

// tryWithAIPrompt opens the API's Try with AI dialog, checks that its prompt points agents at the
// API's Markdown page, and downloads it as a text file with the same content.
func (u *Steps) tryWithAIPrompt(ctx context.Context) error {
	raw, err := contextString(ctx, keyPromptAPI)
	if err != nil {
		return err
	}
	var api catalogueAPI
	if err := json.Unmarshal([]byte(raw), &api); err != nil {
		return err
	}
	page, err := u.openAPIPage(ctx, api)
	if err != nil {
		return err
	}
	if err := page.Locator(".aov-hero-actions .dp-btn--ai").Click(); err != nil {
		return fmt.Errorf("opening Try with AI: %w", err)
	}
	prompt := page.Locator("#apiAgentPromptText")
	if err := u.expect.Locator(prompt).ToContainText("/api/" + api.ID + ".md"); err != nil {
		return err
	}
	return u.downloadMatches(page, "#apiBtnDownloadPrompt", "#apiAgentPromptText", api.ID+".txt")
}

// discoverWithAIPrompt opens the home page's Discover with AI dialog and downloads its prompt.
func (u *Steps) discoverWithAIPrompt(ctx context.Context) error {
	if err := u.openPortalPage(ctx, "/"); err != nil {
		return err
	}
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	if err := page.Locator(".hero-btn--ai").Click(); err != nil {
		return fmt.Errorf("opening Discover with AI: %w", err)
	}
	if err := u.expect.Locator(page.Locator("#discoverAiModal")).ToBeVisible(); err != nil {
		return err
	}
	if err := u.expect.Locator(page.Locator("#discoverPromptText")).ToContainText("llms.txt"); err != nil {
		return err
	}
	return u.downloadMatches(page, "#discoverBtnDownloadPrompt", "#discoverPromptText", "")
}

// downloadMatches clicks a download control and checks that the downloaded file is a .txt file,
// named want when want is set, holding the text of the prompt element.
func (u *Steps) downloadMatches(page playwright.Page, button, promptSelector, want string) error {
	prompt, err := page.Locator(promptSelector).TextContent()
	if err != nil {
		return fmt.Errorf("reading the prompt: %w", err)
	}
	download, err := page.ExpectDownload(func() error { return page.Locator(button).Click() })
	if err != nil {
		return fmt.Errorf("downloading the prompt: %w", err)
	}
	name := download.SuggestedFilename()
	if !strings.HasSuffix(name, ".txt") || (want != "" && name != want) {
		return fmt.Errorf("the prompt downloaded as %q, want a .txt file named %q", name, want)
	}
	dir, err := os.MkdirTemp("", "apiportal-prompt-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(dir) }()
	path := filepath.Join(dir, "prompt.txt")
	// SaveAs streams the file from the remote browser; Path is only available to a local one.
	if err := download.SaveAs(path); err != nil {
		return fmt.Errorf("saving the downloaded prompt: %w", err)
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()
	body, err := io.ReadAll(io.LimitReader(file, 1<<20))
	if err != nil {
		return err
	}
	if strings.TrimSpace(string(body)) != strings.TrimSpace(prompt) {
		return fmt.Errorf("the downloaded prompt differs from the prompt shown")
	}
	return nil
}

// regexpEscapedSuffix matches a URL that ends with suffix.
func regexpEscapedSuffix(suffix string) *regexp.Regexp {
	return regexp.MustCompile(regexp.QuoteMeta(suffix) + `$`)
}

// regexpWord matches a class list that contains class.
func regexpWord(class string) *regexp.Regexp {
	return regexp.MustCompile(`(^|\s)` + regexp.QuoteMeta(class) + `(\s|$)`)
}
