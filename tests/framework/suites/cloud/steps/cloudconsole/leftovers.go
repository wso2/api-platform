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
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cucumber/godog"

	corecatalog "github.com/wso2/api-platform/tests/framework/core/catalog/cloudconsole"
	frameworkruntime "github.com/wso2/api-platform/tests/framework/core/runtime"
	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	"github.com/wso2/api-platform/tests/framework/core/util/retry"
	"github.com/wso2/api-platform/tests/framework/core/util/unique"
	"github.com/wso2/api-platform/tests/framework/suites/cloud/finalizer"
)

// finalizerDeleteTimeout bounds how long the finalizer keeps trying to delete one resource.
const finalizerDeleteTimeout = 90 * time.Second

// finalizerCollectionBudget bounds the time one collection may take to list and delete, so refused
// deletions in one collection cannot leave the later ones with no time. The suite's overall
// cleanup deadline caps the sum.
const finalizerCollectionBudget = 2 * time.Minute

// finalizerCollections are the organisation-wide collections the finalizer reads, in deletion
// order: a gateway goes before the environment it runs in, a project before the pipeline it
// is bound to, and an LLM provider before the secret it uses.
// idFields name the list-item fields that identify a resource in its own route. REST APIs and
// LLM proxies are listed per project, so projectScopedSweep reads them and runs ahead of these:
// each goes before the gateway it is deployed to and the project it belongs to.
var finalizerCollections = []struct {
	collection string
	idFields   []string
	wait       bool
}{
	{collection: "/managed-gateways", idFields: []string{"id"}},
	{collection: "/projects", idFields: []string{"id"}},
	{collection: llmProvidersCollection, idFields: []string{"id"}},
	{collection: secretsCollection, idFields: []string{"handle", "id"}},
	{collection: "/pipelines", idFields: []string{"id", "name"}, wait: true},
	{collection: environmentsCollection, idFields: []string{"name"}, wait: true},
}

// nameFields are the list-item fields a generated name can appear in.
var nameFields = []string{"id", "handle", "name", "displayName"}

// Finalizer removes whatever a run left behind in the cloud organisation through the cloud
// console API. It records each
// runner's name suffix and the platform it talks to, and after the run hands the platform's
// collections to the suite's finalizer package.
type Finalizer struct {
	mu     sync.Mutex
	bases  map[string]struct{}
	finals *finalizer.Tracker
	client *httpx.Client
	log    *slog.Logger
}

// NewFinalizer returns a finalizer that has seen no run yet.
func NewFinalizer() *Finalizer {
	log := slog.Default()
	return &Finalizer{
		bases:  map[string]struct{}{},
		finals: finalizer.NewTracker(log),
		client: httpx.NewClient(httpx.Options{MaxRetries: 1}),
		log:    log,
	}
}

// Register binds the cloud steps like Register and records, before each scenario, the runner's
// name suffix and the platform it talks to, so Finalize knows what to look for and where.
func (f *Finalizer) Register(sc *godog.ScenarioContext, topo *frameworkruntime.Topology, funnel *httpx.Funnel) {
	Register(sc, topo, funnel)
	sc.Before(func(ctx context.Context, _ *godog.Scenario) (context.Context, error) {
		if err := f.observe(ctx, topo); err != nil {
			f.log.Warn("cloud run finalizer cannot track this runner", "error", err)
		}
		return ctx, nil
	})
}

func (f *Finalizer) observe(ctx context.Context, topo *frameworkruntime.Topology) error {
	generator, err := unique.Of(ctx)
	if err != nil {
		return err
	}
	base, err := topo.URL(corecatalog.Name, corecatalog.EndpointAPIPPlatformAPI)
	if err != nil {
		return err
	}
	f.finals.Track(generator.Suffix())
	f.mu.Lock()
	defer f.mu.Unlock()
	f.bases[base] = struct{}{}
	return nil
}

// Finalize deletes the run's leftovers from every platform the run talked to. It returns the
// resources it removed, each one a leak the per-scenario cleanup missed, and an error for the
// ones it could not remove.
func (f *Finalizer) Finalize(ctx context.Context) ([]string, error) {
	f.mu.Lock()
	bases := make([]string, 0, len(f.bases))
	for base := range f.bases {
		bases = append(bases, base)
	}
	f.mu.Unlock()
	if !f.finals.Tracked() || len(bases) == 0 {
		return nil, nil
	}
	token := strings.TrimSpace(os.Getenv(cloudConsoleAccessTokenEnv))
	if token == "" {
		return nil, fmt.Errorf("cloud run finalizer: environment variable %s is required to look for leftovers", cloudConsoleAccessTokenEnv)
	}

	var removed []string
	var errs []error
	for _, base := range bases {
		got, err := f.finals.Sweep(ctx, cloudSweeps(f.client, base, token, f.finals.Owns))
		removed = append(removed, got...)
		if err != nil {
			errs = append(errs, err)
		}
	}
	return removed, errors.Join(errs...)
}

// cloudSweeps describes the organisation's collections to the framework's finalizer. owns reports
// whether a resource's names belong to a tracked run.
func cloudSweeps(client *httpx.Client, base, token string, owns func(names []string) bool) []finalizer.Collection {
	sweeps := make([]finalizer.Collection, 0, len(finalizerCollections)+1)
	for _, collection := range projectScopedCollections {
		sweeps = append(sweeps, projectScopedSweep(client, base, token, owns, collection))
	}
	for _, entry := range finalizerCollections {
		sweeps = append(sweeps, finalizer.Collection{
			Name:    entry.collection,
			Timeout: finalizerCollectionBudget,
			List: func(ctx context.Context) ([]finalizer.Leftover, error) {
				items, err := listCollection(ctx, client, base, token, entry.collection)
				if errors.Is(err, errCollectionNotServed) {
					// A route the platform does not serve cannot hold a leftover.
					slog.Default().Warn("the platform does not serve this collection; skipping it", "collection", entry.collection)
					return nil, nil
				}
				return leftoversOf(items, entry.idFields), err
			},
			Delete: func(ctx context.Context, id string) error {
				return deleteLeftover(ctx, client, base, token, entry.collection, id, entry.wait)
			},
		})
	}
	return sweeps
}

// projectScopedCollections are the collections the platform lists only for one project: a list
// request without projectId is refused with 400.
var projectScopedCollections = []string{"/rest-apis", llmProxiesCollection}

// projectScopedSweep describes a collection the platform lists per project, so it reads the
// default project and every project a tracked run created.
func projectScopedSweep(client *httpx.Client, base, token string, owns func(names []string) bool, collection string) finalizer.Collection {
	return finalizer.Collection{
		Name:    collection,
		Timeout: finalizerCollectionBudget,
		List: func(ctx context.Context) ([]finalizer.Leftover, error) {
			projects, err := listCollection(ctx, client, base, token, "/projects")
			if errors.Is(err, errCollectionNotServed) {
				return nil, nil
			}
			var errs []error
			if err != nil {
				errs = append(errs, fmt.Errorf("listing the projects that hold %s: %w", collection, err))
			}
			scopes := []string{defaultProjectID}
			for _, project := range projects {
				if id := firstString(project, []string{"id"}); id != "" && id != defaultProjectID && owns(namesOf(project)) {
					scopes = append(scopes, id)
				}
			}
			var leftovers []finalizer.Leftover
			for _, project := range scopes {
				items, err := listCollectionQuery(ctx, client, base, token, collection, url.Values{"projectId": {project}})
				if errors.Is(err, errCollectionNotServed) {
					continue
				}
				leftovers = append(leftovers, leftoversOf(items, []string{"id"})...)
				if err != nil {
					errs = append(errs, fmt.Errorf("project %s: %w", project, err))
				}
			}
			return leftovers, errors.Join(errs...)
		},
		Delete: func(ctx context.Context, id string) error {
			return deleteLeftover(ctx, client, base, token, collection, id, false)
		},
	}
}

// leftoversOf turns list items into the finalizer's entries.
func leftoversOf(items []map[string]any, idFields []string) []finalizer.Leftover {
	leftovers := make([]finalizer.Leftover, 0, len(items))
	for _, item := range items {
		leftovers = append(leftovers, finalizer.Leftover{ID: firstString(item, idFields), Names: namesOf(item)})
	}
	return leftovers
}

// errCollectionNotServed marks a collection whose list route the platform answers with 404.
var errCollectionNotServed = errors.New("the platform does not serve this collection")

// maxListPages bounds how many pages one collection is read through.
const maxListPages = 100

// listCollection reads every page of a collection.
func listCollection(ctx context.Context, client *httpx.Client, base, token, collection string) ([]map[string]any, error) {
	return listCollectionQuery(ctx, client, base, token, collection, nil)
}

// listCollectionQuery reads every page of a collection with extra query parameters. The first
// request carries no paging parameters; when the platform reports that more items exist, the next
// pages are read with the limit and offset it advertises. A failure part way returns the items of
// the pages already read together with the error.
func listCollectionQuery(ctx context.Context, client *httpx.Client, base, token, collection string, query url.Values) ([]map[string]any, error) {
	var all []map[string]any
	offset, limit := 0, 0
	for page := 0; page < maxListPages; page++ {
		params := url.Values{}
		for key, values := range query {
			params[key] = values
		}
		if page > 0 {
			params.Set("limit", strconv.Itoa(limit))
			params.Set("offset", strconv.Itoa(offset))
		}
		target := cloudAPIURL(base, collection)
		if len(params) > 0 {
			target += "?" + params.Encode()
		}
		resp, err := getListPage(ctx, client, target, token)
		if err != nil {
			return all, err
		}
		if resp.StatusCode == http.StatusNotFound && page == 0 {
			return nil, errCollectionNotServed
		}
		if resp.StatusCode != http.StatusOK {
			return all, responseError(resp)
		}
		items, err := listItems(resp)
		if err != nil {
			return all, err
		}
		paging := listPaging(resp)
		if page > 0 && paging != nil && paging.Offset != nil && *paging.Offset != offset {
			// A page from the wrong offset can repeat items already read, so it is not kept.
			return all, fmt.Errorf("%s did not honour offset %d, it answered from offset %d", collection, offset, *paging.Offset)
		}
		all = append(all, items...)
		if paging == nil || len(items) == 0 {
			return all, nil
		}
		offset += len(items)
		if paging.Total == nil || offset >= *paging.Total {
			return all, nil
		}
		limit = max(len(items), 1)
		if paging.Limit != nil && *paging.Limit > 0 {
			limit = *paging.Limit
		}
	}
	return all, fmt.Errorf("%s holds more than %d pages; the rest were not read", collection, maxListPages)
}

// getListPage reads one page of a list. A transport error, a 5xx or a 429 is retried, since a
// listing that is missed leaves the run's resources in the organisation for good. Any other answer
// is returned for the caller to judge.
func getListPage(ctx context.Context, client *httpx.Client, target, token string) (*httpx.Response, error) {
	return retry.Until(ctx, retry.Options{Fast: true},
		func(ctx context.Context) (*httpx.Response, error) {
			resp, err := client.Do(ctx, httpx.Request{
				Method:  http.MethodGet,
				URL:     target,
				Headers: map[string]string{"Authorization": "Bearer " + token},
			}, 0, 0)
			if err != nil {
				return nil, retry.Transient(err)
			}
			if resp.StatusCode >= http.StatusInternalServerError || resp.StatusCode == http.StatusTooManyRequests {
				return nil, retry.Transient(responseError(resp))
			}
			return resp, nil
		},
		func(resp *httpx.Response) bool { return resp != nil })
}

// pageInfo is the paging block of a list response. A field the platform leaves out is nil.
type pageInfo struct {
	Total  *int `json:"total"`
	Offset *int `json:"offset"`
	Limit  *int `json:"limit"`
}

// listPaging returns the paging block of a list response, or nil when the response has none.
func listPaging(resp *httpx.Response) *pageInfo {
	var document struct {
		Pagination *pageInfo `json:"pagination"`
	}
	if err := json.Unmarshal(resp.Body, &document); err != nil {
		return nil
	}
	return document.Pagination
}

// deleteLeftover deletes one resource, retrying a refusal for a while: a gateway that is still
// being built or an environment that still holds one is refused until it settles.
func deleteLeftover(ctx context.Context, client *httpx.Client, base, token, collection, id string, wait bool) error {
	return retry.Await(ctx, retry.Options{Timeout: finalizerDeleteTimeout, Interval: 5 * time.Second},
		func(ctx context.Context) (bool, error) {
			if err := deleteCloudResource(ctx, client, base, token, collection, id, wait); err != nil {
				return false, retry.Transient(err)
			}
			return true, nil
		},
		func(deleted bool) bool { return deleted },
		"deleting leftover "+collection+"/"+id)
}

// namesOf collects the values of an item a generated name can appear in.
func namesOf(item map[string]any) []string {
	names := make([]string, 0, len(nameFields))
	for _, field := range nameFields {
		if value, _ := item[field].(string); value != "" {
			names = append(names, value)
		}
	}
	return names
}

func firstString(item map[string]any, fields []string) string {
	for _, field := range fields {
		if value, _ := item[field].(string); value != "" {
			return value
		}
	}
	return ""
}
