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
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
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

// finalizerCollections are the organisation-wide collections the finalizer reads, in deletion
// order: a gateway goes before the environment it runs in, and a project before the pipeline it
// is bound to. idFields name the list-item fields that identify a resource in its own route.
var finalizerCollections = []struct {
	collection string
	idFields   []string
	wait       bool
}{
	{collection: "/managed-gateways", idFields: []string{"id"}},
	{collection: "/projects", idFields: []string{"id"}},
	{collection: secretsCollection, idFields: []string{"handle", "id"}},
	{collection: "/pipelines", idFields: []string{"name"}, wait: true},
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
		got, err := f.finals.Sweep(ctx, cloudSweeps(f.client, base, token))
		removed = append(removed, got...)
		if err != nil {
			errs = append(errs, err)
		}
	}
	return removed, errors.Join(errs...)
}

// cloudSweeps describes the organisation's collections to the framework's finalizer.
func cloudSweeps(client *httpx.Client, base, token string) []finalizer.Collection {
	sweeps := make([]finalizer.Collection, 0, len(finalizerCollections))
	for _, entry := range finalizerCollections {
		sweeps = append(sweeps, finalizer.Collection{
			Name: entry.collection,
			List: func(ctx context.Context) ([]finalizer.Leftover, error) {
				items, err := listCollection(ctx, client, base, token, entry.collection)
				if errors.Is(err, errCollectionNotServed) {
					// A route the platform does not serve cannot hold a leftover.
					slog.Default().Warn("the platform does not serve this collection; skipping it", "collection", entry.collection)
					return nil, nil
				}
				if err != nil {
					return nil, err
				}
				leftovers := make([]finalizer.Leftover, 0, len(items))
				for _, item := range items {
					leftovers = append(leftovers, finalizer.Leftover{ID: firstString(item, entry.idFields), Names: namesOf(item)})
				}
				return leftovers, nil
			},
			Delete: func(ctx context.Context, id string) error {
				return deleteLeftover(ctx, client, base, token, entry.collection, id, entry.wait)
			},
		})
	}
	return sweeps
}

// errCollectionNotServed marks a collection whose list route the platform answers with 404.
var errCollectionNotServed = errors.New("the platform does not serve this collection")

func listCollection(ctx context.Context, client *httpx.Client, base, token, collection string) ([]map[string]any, error) {
	resp, err := client.Do(ctx, httpx.Request{
		Method:  http.MethodGet,
		URL:     cloudAPIURL(base, collection),
		Headers: map[string]string{"Authorization": "Bearer " + token},
	}, 0, 0)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil, errCollectionNotServed
	}
	if resp.StatusCode != http.StatusOK {
		return nil, responseError(resp)
	}
	return listItems(resp)
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
