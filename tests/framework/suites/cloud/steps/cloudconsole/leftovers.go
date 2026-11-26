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
	"slices"
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
	"github.com/wso2/api-platform/tests/framework/suites/cloud/setup/finalizer"
)

// finalizerDeleteTimeout bounds how long the finalizer keeps trying to delete one resource.
const finalizerDeleteTimeout = 90 * time.Second

// finalizerCollectionBudget bounds the time one collection may take to list and delete, so refused
// deletions in one collection cannot leave the later ones with no time. The suite's overall
// cleanup deadline caps the sum.
const finalizerCollectionBudget = 2 * time.Minute

// finalizerCollections are the organisation-wide collections the finalizer reads, in deletion
// order: a secret goes after the REST APIs that reference it. Managed API Portals are not here: the
// finalizer empties the portals the run published to and never deletes one, see portalSweeps.
// idFields name the list-item fields that identify a resource in its own route. REST APIs are
// listed per project, so projectScopedSweep reads them and runs ahead of these: each goes before
// the gateway it is deployed to and the project it belongs to.
var finalizerCollections = []struct {
	collection string
	idFields   []string
	// budget bounds the collection's time; zero selects finalizerCollectionBudget.
	budget time.Duration
}{
	{collection: secretsCollection, idFields: []string{"id", "handle"}},
	{collection: selfHostedGatewaysPath, idFields: []string{"id"}},
	{collection: managedGatewaysCollection, idFields: []string{"id"}},
	// Each project is retried for finalizerDeleteTimeout, so the budget holds several of them.
	{collection: "/projects", idFields: []string{"id"}, budget: projectDeleteTimeout},
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
	f.Watch(base, generator.Suffix())
	return nil
}

// Watch records a platform and a name suffix whose resources Finalize must remove from it. It
// lets a caller that runs no scenarios, such as the manual setup command, say what to look for.
func (f *Finalizer) Watch(base, suffix string) {
	f.finals.Track(suffix)
	f.mu.Lock()
	defer f.mu.Unlock()
	f.bases[base] = struct{}{}
}

// Owns reports whether a name carries the suffix of one of this run's runners.
func (f *Finalizer) Owns(name string) bool {
	return f.finals.Owns([]string{name})
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
	token, err := consoleToken()
	if err != nil {
		return nil, fmt.Errorf("cloud run finalizer: looking for leftovers: %w", err)
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
	sweeps := make([]finalizer.Collection, 0, len(finalizerCollections)+4)
	// The listings go first: the platform removes a listing only through its API, which the sweeps
	// below delete.
	sweeps = append(sweeps, portalSweeps(client, base, token, owns)...)
	for _, collection := range projectScopedCollections {
		sweeps = append(sweeps, projectScopedSweep(client, base, token, owns, collection))
	}
	// A plan is deleted after the APIs that offer it.
	sweeps = append(sweeps, subscriptionPlanSweep(client, base, token))
	for _, entry := range finalizerCollections {
		budget := entry.budget
		if budget == 0 {
			budget = finalizerCollectionBudget
		}
		sweeps = append(sweeps, finalizer.Collection{
			Name:    entry.collection,
			Timeout: budget,
			List: func(ctx context.Context) ([]finalizer.Leftover, error) {
				items, err := listCollection(ctx, client, base, token, entry.collection)
				if errors.Is(err, errCollectionNotServed) {
					// A route the platform does not serve cannot hold a leftover.
					slog.Default().Warn("the platform does not serve this collection; skipping it", "collection", entry.collection)
					return nil, nil
				}
				leftovers := leftoversOf(items, entry.idFields)
				if entry.collection == managedGatewaysCollection {
					defaultGatewayLast(leftovers)
				}
				return leftovers, err
			},
			Delete: func(ctx context.Context, id string) error {
				if entry.collection == "/projects" {
					return deleteProjectThroughCloudAPI(ctx, client, base, token, id, finalizerDeleteTimeout)
				}
				return deleteLeftover(ctx, client, base, token, entry.collection, id)
			},
		})
	}
	return sweeps
}

// projectScopedCollections are the collections the platform lists only for one project: a list
// request without projectId is refused with 400.
var projectScopedCollections = []string{"/rest-apis", "/graphql-apis"}

// projectScopedSweep describes a collection the platform lists per project, so it reads the
// projects the suite works in and every project a tracked run created. Only the entries named with
// a tracked run's suffix are deleted, so an API the organisation owns stays.
func projectScopedSweep(client *httpx.Client, base, token string, owns func(names []string) bool, collection string) finalizer.Collection {
	return finalizer.Collection{
		Name:    collection,
		Timeout: finalizerCollectionBudget,
		List: func(ctx context.Context) ([]finalizer.Leftover, error) {
			items, err := projectScopedItems(ctx, client, base, token, owns, collection)
			return leftoversByID(items), err
		},
		Delete: func(ctx context.Context, id string) error {
			return deleteLeftover(ctx, client, base, token, collection, id)
		},
	}
}

// projectScopedItems reads a collection the platform lists per project, for the projects the suite
// works in and every project a tracked run created.
// A failure part way returns the items already read with the error.
func projectScopedItems(ctx context.Context, client *httpx.Client, base, token string, owns func(names []string) bool, collection string) ([]map[string]any, error) {
	projects, err := listCollection(ctx, client, base, token, "/projects")
	if errors.Is(err, errCollectionNotServed) {
		return nil, nil
	}
	var errs []error
	if err != nil {
		errs = append(errs, fmt.Errorf("listing the projects that hold %s: %w", collection, err))
	}
	// The suite's projects are read only when the organisation lists them: a project it lacks
	// is one the runners skipped, which the platform refuses to list. When the projects cannot be
	// listed both are read.
	var scopes []string
	if err != nil {
		// The list is not known, so both are read; the one the organisation lacks is refused.
		scopes = slices.Clone(suiteProjects)
	}
	for _, project := range projects {
		if id := firstString(project, []string{"id"}); slices.Contains(suiteProjects, id) {
			scopes = append(scopes, id)
		}
	}
	for _, project := range projects {
		if id := firstString(project, []string{"id"}); id != "" && owns(namesOf(project)) && !slices.Contains(scopes, id) {
			scopes = append(scopes, id)
		}
	}
	var items []map[string]any
	for _, project := range scopes {
		listed, err := listCollectionQuery(ctx, client, base, token, collection, url.Values{"projectId": {project}})
		if errors.Is(err, errCollectionNotServed) {
			continue
		}
		items = append(items, listed...)
		if err != nil {
			errs = append(errs, fmt.Errorf("project %s: %w", project, err))
		}
	}
	return items, errors.Join(errs...)
}

// syntheticPlanPrefix starts the id of every subscription plan a scenario creates.
const syntheticPlanPrefix = "syn-plan-"

// subscriptionPlanSweep deletes the subscription plans scenarios created. A plan the organisation
// owns does not carry the prefix, and a plan of another run does not carry this run's suffix, so
// both stay.
func subscriptionPlanSweep(client *httpx.Client, base, token string) finalizer.Collection {
	return finalizer.Collection{
		Name:    "/subscription-plans",
		Timeout: finalizerCollectionBudget,
		List: func(ctx context.Context) ([]finalizer.Leftover, error) {
			items, err := listCollection(ctx, client, base, token, "/subscription-plans")
			if errors.Is(err, errCollectionNotServed) {
				return nil, nil
			}
			var created []map[string]any
			for _, item := range items {
				if strings.HasPrefix(firstString(item, []string{"id"}), syntheticPlanPrefix) {
					created = append(created, item)
				}
			}
			return leftoversOf(created, []string{"id"}), err
		},
		Delete: func(ctx context.Context, id string) error {
			return deleteLeftover(ctx, client, base, token, "/subscription-plans", id)
		},
	}
}

// publishedAPICollections are the API collections whose APIs a portal can list, by the API type
// the publication routes use.
var publishedAPICollections = []struct{ apiType, collection string }{
	{"rest-api", "/rest-apis"},
	{"graphql-api", "/graphql-apis"},
}

// portalSweeps empties the managed API Portals of what the run put in them, and never deletes a
// portal: the starter's portal and every portal the organisation already had stay. It unpublishes
// each API of the run from every active portal that lists it. The only portals it deletes are
// those a scenario built for itself, which carry syntheticPortalPrefix.
func portalSweeps(client *httpx.Client, base, token string, owns func(names []string) bool) []finalizer.Collection {
	return []finalizer.Collection{
		{
			Name:    "portal-publications",
			Timeout: finalizerCollectionBudget,
			List: func(ctx context.Context) ([]finalizer.Leftover, error) {
				return portalPublications(ctx, client, base, token, owns)
			},
			Delete: func(ctx context.Context, id string) error {
				method, path, settled, err := unpublishRequest(id)
				if err != nil {
					return err
				}
				return retry.Await(ctx, retry.Options{Timeout: finalizerDeleteTimeout, Interval: 5 * time.Second},
					func(ctx context.Context) (bool, error) {
						resp, err := client.Do(ctx, httpx.Request{
							Method:  method,
							URL:     cloudAPIURL(base, path),
							Headers: map[string]string{"Authorization": "Bearer " + liveConsoleToken(token)},
						}, 0, 0)
						if err != nil {
							return false, retry.Transient(err)
						}
						if !settled(resp) {
							return false, retry.Transient(responseError(resp))
						}
						return true, nil
					},
					func(done bool) bool { return done },
					"unpublishing "+id)
			},
		},
		{
			Name:    managedPortalsCollection,
			Timeout: finalizerCollectionBudget,
			List: func(ctx context.Context) ([]finalizer.Leftover, error) {
				items, err := listCollection(ctx, client, base, token, managedPortalsCollection)
				if errors.Is(err, errCollectionNotServed) {
					return nil, nil
				}
				var built []map[string]any
				for _, item := range items {
					if id, _ := item["id"].(string); strings.HasPrefix(id, syntheticPortalPrefix) {
						built = append(built, item)
					}
				}
				return leftoversOf(built, []string{"id"}), err
			},
			Delete: func(ctx context.Context, id string) error {
				return deleteLeftover(ctx, client, base, token, managedPortalsCollection, id)
			},
		},
	}
}

// portalPublications lists, as publication ids, the listings the run's APIs hold in the active
// portals. An API that no portal lists has no publication route to read, so it is not a leftover.
func portalPublications(ctx context.Context, client *httpx.Client, base, token string, owns func(names []string) bool) ([]finalizer.Leftover, error) {
	portals, err := listCollection(ctx, client, base, token, managedPortalsCollection)
	if errors.Is(err, errCollectionNotServed) {
		return nil, nil
	}
	var errs []error
	if err != nil {
		errs = append(errs, fmt.Errorf("listing the managed API Portals: %w", err))
	}
	var active []string
	for _, portal := range portals {
		if status, _ := portal["status"].(string); status == portalStatusActive {
			if id := firstString(portal, []string{"id"}); id != "" {
				active = append(active, id)
			}
		}
	}
	if len(active) == 0 {
		return nil, errors.Join(errs...)
	}
	var leftovers []finalizer.Leftover
	for _, kind := range publishedAPICollections {
		items, err := projectScopedItems(ctx, client, base, token, owns, kind.collection)
		if err != nil {
			errs = append(errs, fmt.Errorf("listing %s: %w", kind.collection, err))
		}
		for _, item := range items {
			api := firstString(item, []string{"id"})
			if api == "" || !owns(namesOf(item)) {
				continue
			}
			for _, portal := range active {
				target := publicationTarget{portal: portal, apiType: kind.apiType, api: api}
				resp, err := client.Do(ctx, httpx.Request{
					Method:  http.MethodGet,
					URL:     cloudAPIURL(base, target.path("/publication")),
					Headers: map[string]string{"Authorization": "Bearer " + liveConsoleToken(token)},
				}, 0, 0)
				switch {
				case err != nil:
					errs = append(errs, fmt.Errorf("reading the publication %s: %w", target.id(), err))
				case resp.StatusCode == http.StatusNotFound:
				case resp.StatusCode == http.StatusOK:
					leftovers = append(leftovers, finalizer.Leftover{ID: target.id(), Names: namesOf(item)})
				default:
					errs = append(errs, fmt.Errorf("reading the publication %s: %w", target.id(), responseError(resp)))
				}
			}
		}
	}
	return leftovers, errors.Join(errs...)
}

// defaultGatewayLast moves a gateway the starter created behind every other gateway. The finalizer
// of a run never deletes it, since a run tracks only the suffixes of its runners; it is deleted when
// a person names the starter's suffix to the setup command. The platform refuses to delete an
// environment's default gateway while other gateways of its kind remain, and allows the last one to
// go.
func defaultGatewayLast(leftovers []finalizer.Leftover) {
	slices.SortStableFunc(leftovers, func(a, b finalizer.Leftover) int {
		return boolRank(isStarterGateway(a.Names)) - boolRank(isStarterGateway(b.Names))
	})
}

func boolRank(value bool) int {
	if value {
		return 1
	}
	return 0
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
				Headers: map[string]string{"Authorization": "Bearer " + liveConsoleToken(token)},
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
// being built is refused until it settles.
func deleteLeftover(ctx context.Context, client *httpx.Client, base, token, collection, id string) error {
	return retry.Await(ctx, retry.Options{Timeout: finalizerDeleteTimeout, Interval: 5 * time.Second},
		func(ctx context.Context) (bool, error) {
			if err := deleteCloudResource(ctx, client, base, token, collection, id); err != nil {
				return false, retry.Transient(err)
			}
			return true, nil
		},
		func(deleted bool) bool { return deleted },
		"deleting leftover "+collection+"/"+id)
}

// namesOf collects the values of an item a generated name can appear in.
// leftoversByID describes items that are deleted by their id, judged by that id alone: another
// name of an item, such as a display name, can carry a run's suffix without the run owning it.
func leftoversByID(items []map[string]any) []finalizer.Leftover {
	leftovers := make([]finalizer.Leftover, 0, len(items))
	for _, item := range items {
		id := firstString(item, []string{"id"})
		leftovers = append(leftovers, finalizer.Leftover{ID: id, Names: []string{id}})
	}
	return leftovers
}

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
