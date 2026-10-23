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

// Package finalizer removes what a cloud suite run left behind in the shared organisation.
//
// The framework's cleanup registry deletes only what a scenario registered and gives up on a
// delete that failed, so a failed scenario, a refused delete or an interrupted step can leave
// resources in a long-lived organisation. A Tracker records the name suffix of every runner of
// a run and, after the run, deletes each entry of the organisation's collections that is named
// with one of those suffixes. It is shared by the cloud suite's step packages; each supplies
// the Collections of the platform it drives.
package finalizer

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
)

// Leftover is one entry a Sweep found in the system under test.
type Leftover struct {
	// ID is what Collection.Delete needs to delete the entry.
	ID string
	// Names are every value a generated name can appear in for this entry, such as its id,
	// handle and display name. A handle cut to fit a length limit loses the runner suffix, but
	// the display name it was cut from keeps it.
	Names []string
}

// Collection is one list of the system under test that a Tracker reads and deletes from.
type Collection struct {
	// Name identifies the collection in diagnostics and in the removed list.
	Name string
	// List returns the entries of the collection.
	List func(ctx context.Context) ([]Leftover, error)
	// Delete removes one entry by its ID. An entry that is already gone is not an error.
	Delete func(ctx context.Context, id string) error
}

// Tracker holds the runner suffixes of one run and sweeps the organisation for entries named
// with them. Every name from the framework's unique generator carries its runner's suffix.
type Tracker struct {
	mu       sync.Mutex
	suffixes map[string]struct{}
	log      *slog.Logger
}

// NewTracker returns a Tracker that has seen no runner yet. A nil logger selects the
// default logger.
func NewTracker(log *slog.Logger) *Tracker {
	if log == nil {
		log = slog.Default()
	}
	return &Tracker{suffixes: map[string]struct{}{}, log: log}
}

// Track records a runner's name suffix, as Generator.Suffix reports it. An empty suffix is
// ignored, because it would match every name.
func (t *Tracker) Track(suffix string) {
	suffix = normalizeName(suffix)
	if suffix == "" {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.suffixes[suffix] = struct{}{}
}

// Tracked reports whether any runner suffix has been recorded.
func (t *Tracker) Tracked() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.suffixes) > 0
}

// Sweep runs through the collections in order. For each it lists the collection, keeps the entries named
// with a tracked suffix and deletes them, carrying on past a failure. It returns what it
// removed as "<collection name>/<id>", each one a leak the registry cleanup missed, and the joined
// errors for what it could not list or delete.
func (t *Tracker) Sweep(ctx context.Context, collections []Collection) ([]string, error) {
	t.mu.Lock()
	suffixes := make([]string, 0, len(t.suffixes))
	for suffix := range t.suffixes {
		suffixes = append(suffixes, suffix)
	}
	t.mu.Unlock()
	if len(suffixes) == 0 {
		return nil, nil
	}

	var removed []string
	var errs []error
	for _, collection := range collections {
		if collection.List == nil || collection.Delete == nil {
			errs = append(errs, fmt.Errorf("cleanup: collection %q needs both a list and a delete function", collection.Name))
			continue
		}
		items, err := collection.List(ctx)
		if err != nil {
			errs = append(errs, fmt.Errorf("listing %s: %w", collection.Name, err))
			continue
		}
		for _, item := range items {
			if !OwnedBy(item.Names, suffixes) {
				continue
			}
			if item.ID == "" {
				errs = append(errs, fmt.Errorf("%s holds a leftover with no id to delete it by", collection.Name))
				continue
			}
			t.log.Warn("removing a resource the run left behind", "collection", collection.Name, "id", item.ID)
			if err := collection.Delete(ctx, item.ID); err != nil {
				errs = append(errs, err)
				continue
			}
			removed = append(removed, collection.Name+"/"+item.ID)
		}
	}
	return removed, errors.Join(errs...)
}

// OwnedBy reports whether any name carries one of the suffixes as a whole name segment, so an
// entry of another run, or one the suite never creates, is never matched. Names are compared
// lower-cased with underscores read as dashes, the form a generated handle takes.
func OwnedBy(names, suffixes []string) bool {
	for _, name := range names {
		name = normalizeName(name)
		if name == "" {
			continue
		}
		for _, suffix := range suffixes {
			suffix = normalizeName(suffix)
			if suffix == "" {
				continue
			}
			if strings.Contains(name, "-"+suffix+"-") || strings.HasSuffix(name, "-"+suffix) {
				return true
			}
		}
	}
	return false
}

func normalizeName(value string) string {
	return strings.ToLower(strings.ReplaceAll(strings.TrimSpace(value), "_", "-"))
}
