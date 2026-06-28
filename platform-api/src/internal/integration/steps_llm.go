//go:build integration

/*
 *  Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 *  WSO2 LLC. licenses this file to you under the Apache License,
 *  Version 2.0 (the "License"); you may not use this file except
 *  in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing,
 *  software distributed under the License is distributed on an
 *  "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 *  KIND, either express or implied. See the License for the
 *  specific language governing permissions and limitations
 *  under the License.
 */

package integration

import (
	"fmt"
	"strings"

	"github.com/cucumber/godog"

	"platform-api/src/internal/model"
	"platform-api/src/internal/repository"
)

// registerLLMSteps wires the LLM control-plane (template / provider / proxy)
// repository scenarios. These exercise the AI control plane purely at the store
// layer — no real LLM is contacted and the upstream API key is a dummy string,
// so the suite needs no provider credentials on any engine.
func registerLLMSteps(ctx *godog.ScenarioContext, w *world) {
	// Provider template. The "Given" and "When" phrasings share one handler.
	ctx.Step(`^(?:I create )?an LLM provider template "([^"]*)" version "([^"]*)"$`, w.createLLMTemplate)
	ctx.Step(`^reading the template by its handle returns version "([^"]*)"$`, w.readTemplateVersion)
	ctx.Step(`^I create a new version "([^"]*)" of the template$`, w.createTemplateVersion)
	ctx.Step(`^reading the original template handle still returns version "([^"]*)"$`, w.readTemplateVersion)
	ctx.Step(`^the latest version of the template family is "([^"]*)"$`, w.latestTemplateVersionIs)
	ctx.Step(`^listing template versions returns (\d+)$`, w.listTemplateVersions)

	// Provider.
	ctx.Step(`^(?:I create )?an LLM provider "([^"]*)" with upstream key "([^"]*)"$`, w.createLLMProvider)
	ctx.Step(`^reading the provider back returns upstream key "([^"]*)"$`, w.providerUpstreamKeyIs)
	ctx.Step(`^listing LLM providers by organization returns (\d+)$`, w.listLLMProviders)
	ctx.Step(`^I update the provider description to "([^"]*)"$`, w.updateProviderDescription)
	ctx.Step(`^reading the provider back shows description "([^"]*)"$`, w.providerDescriptionIs)
	ctx.Step(`^I delete the provider$`, w.deleteProvider)

	// Proxy.
	ctx.Step(`^I create an LLM proxy "([^"]*)" for that provider$`, w.createLLMProxy)
	ctx.Step(`^reading the proxy back references the provider$`, w.proxyReferencesProvider)
	ctx.Step(`^listing LLM proxies by organization returns (\d+)$`, w.listLLMProxiesByOrg)
	ctx.Step(`^listing LLM proxies by project returns (\d+)$`, w.listLLMProxiesByProject)
	ctx.Step(`^listing LLM proxies by provider returns (\d+)$`, w.listLLMProxiesByProvider)
	ctx.Step(`^I delete the proxy$`, w.deleteProxy)
}

// --- Provider template ------------------------------------------------------

func (w *world) createLLMTemplate(handle, version string) error {
	repo := repository.NewLLMProviderTemplateRepo(w.it.db)
	tpl := &model.LLMProviderTemplate{
		OrganizationUUID: w.orgID,
		ID:               handle,
		GroupID:          handle,
		Name:             "Template " + handle,
		ManagedBy:        "customer",
		Version:          version,
	}
	if err := repo.Create(tpl); err != nil {
		return fmt.Errorf("[%s] create LLM template failed: %w", w.it.driver, err)
	}
	if tpl.UUID == "" {
		return fmt.Errorf("[%s] create LLM template did not populate the generated UUID", w.it.driver)
	}
	w.templateUUID = tpl.UUID
	w.templateHandle = handle
	return nil
}

func (w *world) readTemplateVersion(version string) error {
	repo := repository.NewLLMProviderTemplateRepo(w.it.db)
	got, err := repo.GetByID(w.templateHandle, w.orgID)
	if err != nil {
		return fmt.Errorf("[%s] GetByID(%s) failed: %w", w.it.driver, w.templateHandle, err)
	}
	if got == nil || got.Version != version {
		return fmt.Errorf("[%s] GetByID(%s): want version %q, got %+v", w.it.driver, w.templateHandle, version, got)
	}
	return nil
}

func (w *world) createTemplateVersion(version string) error {
	repo := repository.NewLLMProviderTemplateRepo(w.it.db)
	// A new family version uses a version-suffixed handle but the same group_id,
	// which demotes the previous is_latest row (mirrors the template service).
	// The suffix is derived from the requested version (e.g. "v2.0" -> "v2-0").
	newHandle := fmt.Sprintf("%s-%s", w.templateHandle, strings.ReplaceAll(version, ".", "-"))
	tpl := &model.LLMProviderTemplate{
		OrganizationUUID: w.orgID,
		ID:               newHandle,
		GroupID:          w.templateHandle,
		Name:             "Template " + w.templateHandle,
		ManagedBy:        "customer",
		Version:          version,
	}
	if err := repo.CreateNewVersion(tpl); err != nil {
		return fmt.Errorf("[%s] CreateNewVersion(%s) failed: %w", w.it.driver, version, err)
	}
	return nil
}

func (w *world) latestTemplateVersionIs(version string) error {
	repo := repository.NewLLMProviderTemplateRepo(w.it.db)
	versions, err := repo.ListVersions(w.templateHandle, w.orgID, 100, 0)
	if err != nil {
		return fmt.Errorf("[%s] ListVersions failed: %w", w.it.driver, err)
	}
	for _, t := range versions {
		if t.IsLatest {
			if t.Version != version {
				return fmt.Errorf("[%s] latest version: want %q, got %q", w.it.driver, version, t.Version)
			}
			return nil
		}
	}
	return fmt.Errorf("[%s] no template version is marked latest", w.it.driver)
}

func (w *world) listTemplateVersions(want int) error {
	repo := repository.NewLLMProviderTemplateRepo(w.it.db)
	versions, err := repo.ListVersions(w.templateHandle, w.orgID, 100, 0)
	if err != nil {
		return fmt.Errorf("[%s] ListVersions failed: %w", w.it.driver, err)
	}
	if len(versions) != want {
		return fmt.Errorf("[%s] ListVersions: want %d, got %d", w.it.driver, want, len(versions))
	}
	count, err := repo.CountVersions(w.templateHandle, w.orgID)
	if err != nil {
		return fmt.Errorf("[%s] CountVersions failed: %w", w.it.driver, err)
	}
	if count != want {
		return fmt.Errorf("[%s] CountVersions: want %d, got %d", w.it.driver, want, count)
	}
	return nil
}

// --- Provider ---------------------------------------------------------------

func (w *world) createLLMProvider(handle, upstreamKey string) error {
	repo := repository.NewLLMProviderRepo(w.it.db)
	prov := &model.LLMProvider{
		OrganizationUUID: w.orgID,
		ID:               handle,
		Name:             "Provider " + handle,
		Version:          "v1.0",
		TemplateUUID:     w.templateUUID,
		CreatedBy:        "it-user",
		Configuration: model.LLMProviderConfig{
			Template: w.templateHandle,
			// The upstream API key is a dummy string — the store never calls the
			// provider, so no real credential is required.
			Upstream: &model.UpstreamConfig{
				Main: &model.UpstreamEndpoint{
					URL: "http://mock-openapi:4010/openai/v1",
					Auth: &model.UpstreamAuth{
						Type:   "api-key",
						Header: "Authorization",
						Value:  upstreamKey,
					},
				},
			},
		},
	}
	if err := repo.Create(prov); err != nil {
		return fmt.Errorf("[%s] create LLM provider failed: %w", w.it.driver, err)
	}
	if prov.UUID == "" {
		return fmt.Errorf("[%s] create LLM provider did not populate the generated UUID", w.it.driver)
	}
	w.providerUUID = prov.UUID
	w.providerHandle = handle
	return nil
}

func (w *world) providerUpstreamKeyIs(want string) error {
	prov, err := w.getProvider()
	if err != nil {
		return err
	}
	if prov.Configuration.Upstream == nil || prov.Configuration.Upstream.Main == nil || prov.Configuration.Upstream.Main.Auth == nil {
		return fmt.Errorf("[%s] provider upstream auth did not round-trip: %+v", w.it.driver, prov.Configuration.Upstream)
	}
	if got := prov.Configuration.Upstream.Main.Auth.Value; got != want {
		return fmt.Errorf("[%s] provider upstream key: want %q, got %q", w.it.driver, want, got)
	}
	return nil
}

func (w *world) listLLMProviders(want int) error {
	repo := repository.NewLLMProviderRepo(w.it.db)
	list, err := repo.List(w.orgID, 100, 0)
	if err != nil {
		return fmt.Errorf("[%s] List LLM providers failed: %w", w.it.driver, err)
	}
	if len(list) != want {
		return fmt.Errorf("[%s] List LLM providers: want %d, got %d", w.it.driver, want, len(list))
	}
	count, err := repo.Count(w.orgID)
	if err != nil {
		return fmt.Errorf("[%s] Count LLM providers failed: %w", w.it.driver, err)
	}
	if count != want {
		return fmt.Errorf("[%s] Count LLM providers: want %d, got %d", w.it.driver, want, count)
	}
	return nil
}

func (w *world) updateProviderDescription(desc string) error {
	repo := repository.NewLLMProviderRepo(w.it.db)
	prov, err := w.getProvider()
	if err != nil {
		return err
	}
	prov.Description = desc
	prov.UpdatedBy = "it-user"
	if err := repo.Update(prov); err != nil {
		return fmt.Errorf("[%s] Update LLM provider failed: %w", w.it.driver, err)
	}
	return nil
}

func (w *world) providerDescriptionIs(want string) error {
	prov, err := w.getProvider()
	if err != nil {
		return err
	}
	if prov.Description != want {
		return fmt.Errorf("[%s] provider description: want %q, got %q", w.it.driver, want, prov.Description)
	}
	return nil
}

func (w *world) deleteProvider() error {
	repo := repository.NewLLMProviderRepo(w.it.db)
	if err := repo.Delete(w.providerHandle, w.orgID); err != nil {
		return fmt.Errorf("[%s] Delete LLM provider failed: %w", w.it.driver, err)
	}
	return nil
}

func (w *world) getProvider() (*model.LLMProvider, error) {
	repo := repository.NewLLMProviderRepo(w.it.db)
	prov, err := repo.GetByID(w.providerHandle, w.orgID)
	if err != nil {
		return nil, fmt.Errorf("[%s] GetByID(%s) failed: %w", w.it.driver, w.providerHandle, err)
	}
	if prov == nil {
		return nil, fmt.Errorf("[%s] GetByID(%s): want the provider, got nil", w.it.driver, w.providerHandle)
	}
	return prov, nil
}

// --- Proxy ------------------------------------------------------------------

func (w *world) createLLMProxy(handle string) error {
	repo := repository.NewLLMProxyRepo(w.it.db)
	proxy := &model.LLMProxy{
		OrganizationUUID: w.orgID,
		ID:               handle,
		Name:             "Proxy " + handle,
		ProjectUUID:      w.projID,
		Version:          "v1.0",
		ProviderUUID:     w.providerUUID,
		CreatedBy:        "it-user",
		Configuration:    model.LLMProxyConfig{Provider: w.providerHandle},
	}
	if err := repo.Create(proxy); err != nil {
		return fmt.Errorf("[%s] create LLM proxy failed: %w", w.it.driver, err)
	}
	w.proxyHandle = handle
	return nil
}

func (w *world) proxyReferencesProvider() error {
	repo := repository.NewLLMProxyRepo(w.it.db)
	proxy, err := repo.GetByID(w.proxyHandle, w.orgID)
	if err != nil {
		return fmt.Errorf("[%s] GetByID(%s) failed: %w", w.it.driver, w.proxyHandle, err)
	}
	if proxy == nil || proxy.ProviderUUID != w.providerUUID {
		return fmt.Errorf("[%s] proxy provider reference mismatch: %+v", w.it.driver, proxy)
	}
	return nil
}

func (w *world) listLLMProxiesByOrg(want int) error {
	repo := repository.NewLLMProxyRepo(w.it.db)
	list, err := repo.List(w.orgID, 100, 0)
	if err != nil {
		return fmt.Errorf("[%s] List LLM proxies failed: %w", w.it.driver, err)
	}
	if len(list) != want {
		return fmt.Errorf("[%s] List LLM proxies: want %d, got %d", w.it.driver, want, len(list))
	}
	count, err := repo.Count(w.orgID)
	if err != nil {
		return fmt.Errorf("[%s] Count LLM proxies failed: %w", w.it.driver, err)
	}
	if count != want {
		return fmt.Errorf("[%s] Count LLM proxies: want %d, got %d", w.it.driver, want, count)
	}
	return nil
}

func (w *world) listLLMProxiesByProject(want int) error {
	repo := repository.NewLLMProxyRepo(w.it.db)
	list, err := repo.ListByProject(w.orgID, w.projID, 100, 0)
	if err != nil {
		return fmt.Errorf("[%s] ListByProject LLM proxies failed: %w", w.it.driver, err)
	}
	if len(list) != want {
		return fmt.Errorf("[%s] ListByProject LLM proxies: want %d, got %d", w.it.driver, want, len(list))
	}
	count, err := repo.CountByProject(w.orgID, w.projID)
	if err != nil {
		return fmt.Errorf("[%s] CountByProject LLM proxies failed: %w", w.it.driver, err)
	}
	if count != want {
		return fmt.Errorf("[%s] CountByProject LLM proxies: want %d, got %d", w.it.driver, want, count)
	}
	return nil
}

func (w *world) listLLMProxiesByProvider(want int) error {
	repo := repository.NewLLMProxyRepo(w.it.db)
	list, err := repo.ListByProvider(w.orgID, w.providerUUID, 100, 0)
	if err != nil {
		return fmt.Errorf("[%s] ListByProvider LLM proxies failed: %w", w.it.driver, err)
	}
	if len(list) != want {
		return fmt.Errorf("[%s] ListByProvider LLM proxies: want %d, got %d", w.it.driver, want, len(list))
	}
	count, err := repo.CountByProvider(w.orgID, w.providerUUID)
	if err != nil {
		return fmt.Errorf("[%s] CountByProvider LLM proxies failed: %w", w.it.driver, err)
	}
	if count != want {
		return fmt.Errorf("[%s] CountByProvider LLM proxies: want %d, got %d", w.it.driver, want, count)
	}
	return nil
}

func (w *world) deleteProxy() error {
	repo := repository.NewLLMProxyRepo(w.it.db)
	if err := repo.Delete(w.proxyHandle, w.orgID); err != nil {
		return fmt.Errorf("[%s] Delete LLM proxy failed: %w", w.it.driver, err)
	}
	return nil
}
