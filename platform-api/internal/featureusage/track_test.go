/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 */

package featureusage

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// capture is a Reporter that keeps what it is given.
type capture struct {
	mu  sync.Mutex
	got []Occurrence
}

func (c *capture) Record(o Occurrence) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.got = append(c.got, o)
}

func (c *capture) Close(context.Context) error { return nil }

func (c *capture) all() []Occurrence {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]Occurrence(nil), c.got...)
}

// serveTracked registers handler under a tracked route and sends one call.
func serveTracked(t *testing.T, rep Reporter, handler http.HandlerFunc) *httptest.ResponseRecorder {
	t.Helper()
	mux := http.NewServeMux()
	mux.Handle("POST /api/v0.9/rest-apis", Track(func() Reporter { return rep },
		FeatureCreateRESTAPI, RESTAPICreationDefaults(), handler))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v0.9/rest-apis?owner=alice", nil))
	return rec
}

func TestTrackReportsASuccessfulCall(t *testing.T) {
	rep := &capture{}
	before := time.Now()
	serveTracked(t, rep, func(w http.ResponseWriter, r *http.Request) {
		call := CallFrom(r.Context())
		call.SetOrganization(testOrgUUID)
		call.SetActor(testActorUUID)
		call.SetHasSandboxUpstream(true)
		call.SetResource(testAPIUUID)
		w.WriteHeader(http.StatusCreated)
	})

	got := rep.all()
	if len(got) != 1 {
		t.Fatalf("recorded %d occurrences, want 1", len(got))
	}
	o := got[0]
	if o.Feature != FeatureCreateRESTAPI || o.OrgUUID != testOrgUUID || o.ActorUUID != testActorUUID ||
		o.OccurrenceID != testAPIUUID {
		t.Errorf("unexpected occurrence: %+v", o)
	}
	if o.Dimensions[dimCreationMethod] != "contract" || o.Dimensions[dimHasSandboxUpstream] != true {
		t.Errorf("dimensions = %v, want the contract default plus the handler's sandbox fact", o.Dimensions)
	}
	inv := o.Invocation
	if inv.Method != http.MethodPost || inv.Status != http.StatusCreated ||
		inv.URI != "http://example.com/api/v0.9/rest-apis" || inv.RequestTime.Before(before) {
		t.Errorf("invocation = %+v", inv)
	}
	if _, err := validate(mustTaxonomy(t), enabledFeatures, o, time.Now()); err != nil {
		t.Errorf("a tracked call failed validation: %v", err)
	}
}

func TestTrackReportsAFailedCallWithItsStatus(t *testing.T) {
	rep := &capture{}
	serveTracked(t, rep, func(w http.ResponseWriter, r *http.Request) {
		call := CallFrom(r.Context())
		call.SetOrganization(testOrgUUID)
		call.SetCreationMethod(CreationMethodScratch)
		call.SetResource(testAPIUUID) // created, then rolled back
		http.Error(w, "boom", http.StatusInternalServerError)
	})
	got := rep.all()
	if len(got) != 1 {
		t.Fatalf("recorded %d occurrences, want 1", len(got))
	}
	o := got[0]
	if o.Invocation.Status != http.StatusInternalServerError || o.Dimensions[dimCreationMethod] != "scratch" {
		t.Errorf("occurrence = %+v, want status 500 with the declared method", o)
	}
	// A failed call is not identified by what it created and rolled back.
	if o.OccurrenceID == testAPIUUID || !canonicalUUID(o.OccurrenceID) {
		t.Errorf("occurrence ID = %q, want a fresh random UUID", o.OccurrenceID)
	}
}

func TestTrackGivesEachFailureItsOwnID(t *testing.T) {
	rep := &capture{}
	fail := func(w http.ResponseWriter, r *http.Request) {
		CallFrom(r.Context()).SetOrganization(testOrgUUID)
		w.WriteHeader(http.StatusConflict)
	}
	serveTracked(t, rep, fail)
	serveTracked(t, rep, fail)
	got := rep.all()
	if len(got) != 2 || got[0].OccurrenceID == got[1].OccurrenceID {
		t.Fatalf("two failures must be two events with distinct IDs, got %+v", got)
	}
}

// Nothing written means net/http answers 200; a body without WriteHeader too.
func TestTrackSeesTheImplicitStatus(t *testing.T) {
	for name, handler := range map[string]http.HandlerFunc{
		"no write": func(w http.ResponseWriter, r *http.Request) {
			CallFrom(r.Context()).SetOrganization(testOrgUUID)
		},
		"body only": func(w http.ResponseWriter, r *http.Request) {
			CallFrom(r.Context()).SetOrganization(testOrgUUID)
			_, _ = w.Write([]byte("{}"))
		},
	} {
		t.Run(name, func(t *testing.T) {
			rep := &capture{}
			serveTracked(t, rep, handler)
			if got := rep.all(); len(got) != 1 || got[0].Invocation.Status != http.StatusOK {
				t.Fatalf("got %+v, want one event with status 200", got)
			}
		})
	}
}

// A call that never reached an organization cannot be attributed.
func TestTrackSkipsACallWithoutAnOrganization(t *testing.T) {
	rep := &capture{}
	serveTracked(t, rep, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})
	if n := len(rep.all()); n != 0 {
		t.Fatalf("recorded %d occurrences for an unattributable call", n)
	}
}

func TestTrackPassesThroughWhenReportingIsOff(t *testing.T) {
	for name, rep := range map[string]Reporter{"no reporter": nil, "reporting off": Noop()} {
		t.Run(name, func(t *testing.T) {
			var sawCall bool
			rec := serveTracked(t, rep, func(w http.ResponseWriter, r *http.Request) {
				call := CallFrom(r.Context())
				sawCall = call != nil
				call.SetOrganization(testOrgUUID) // nil-safe
				w.WriteHeader(http.StatusCreated)
			})
			if sawCall {
				t.Error("an untracked call must not carry a Call")
			}
			if rec.Code != http.StatusCreated {
				t.Errorf("status = %d, want the handler's 201", rec.Code)
			}
		})
	}
}

// The defaults belong to the route, not to one call: a handler changing its
// call's dimensions must not leak into the next call.
func TestTrackDoesNotShareDimensionsBetweenCalls(t *testing.T) {
	rep := &capture{}
	mux := http.NewServeMux()
	mux.Handle("POST /api/v0.9/rest-apis", Track(func() Reporter { return rep }, FeatureCreateRESTAPI,
		RESTAPICreationDefaults(), http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			call := CallFrom(r.Context())
			call.SetOrganization(testOrgUUID)
			if r.URL.Query().Get("scratch") == "1" {
				call.SetCreationMethod(CreationMethodScratch)
			}
		})))
	for _, target := range []string{"/api/v0.9/rest-apis?scratch=1", "/api/v0.9/rest-apis"} {
		mux.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, target, nil))
	}
	got := rep.all()
	if len(got) != 2 || got[0].Dimensions[dimCreationMethod] != "scratch" || got[1].Dimensions[dimCreationMethod] != "contract" {
		t.Fatalf("got %+v; the second call must start from the contract default again", got)
	}
}
