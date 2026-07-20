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

package utils

import (
	"strings"
	"testing"

	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/model"

	"gopkg.in/yaml.v3"
)

// TestBuildAPIDeploymentYAML_EmitsUpstreamDefinitions verifies the reusable pool is emitted
// into spec.upstreamDefinitions with name, basePath, timeout, and weighted backends.
func TestBuildAPIDeploymentYAML_EmitsUpstreamDefinitions(t *testing.T) {
	util := &APIUtil{}
	ctx := "/test"
	weight := 80
	apiModel := &model.API{
		Name:    "Pool API",
		Handle:  "pool-api",
		Version: "v1.0",
		Kind:    constants.RestApi,
		Configuration: model.RestAPIConfig{
			Context:  &ctx,
			Upstream: model.UpstreamConfig{Main: &model.UpstreamEndpoint{URL: "http://main:8080"}},
			UpstreamDefinitions: []model.ReusableUpstream{
				{
					Name:      "alt-backend",
					BasePath:  "/api/v2",
					Timeout:   &model.UpstreamTimeout{Connect: "5s"},
					Upstreams: []model.UpstreamTarget{{URL: "http://alt:9090", Weight: &weight}},
				},
			},
			Operations: []model.Operation{
				{Request: &model.OperationRequest{Method: "GET", Path: "/x"}},
			},
		},
	}

	d, err := util.BuildAPIDeploymentYAML(apiModel)
	if err != nil {
		t.Fatalf("BuildAPIDeploymentYAML() error = %v", err)
	}
	if len(d.Spec.UpstreamDefinitions) != 1 {
		t.Fatalf("want 1 upstreamDefinition, got %+v", d.Spec.UpstreamDefinitions)
	}
	got := d.Spec.UpstreamDefinitions[0]
	if got.Name != "alt-backend" || got.BasePath != "/api/v2" {
		t.Errorf("definition mismatch: want name=%q basePath=%q, got %+v", "alt-backend", "/api/v2", got)
	}
	if got.Timeout == nil || got.Timeout.Connect != "5s" {
		t.Errorf("Timeout = %+v, want connect 5s", got.Timeout)
	}
	if len(got.Upstreams) != 1 || got.Upstreams[0].URL != "http://alt:9090" ||
		got.Upstreams[0].Weight == nil || *got.Upstreams[0].Weight != 80 {
		t.Errorf("Upstreams = %+v, want url http://alt:9090 weight 80", got.Upstreams)
	}

	out, err := yaml.Marshal(d)
	if err != nil {
		t.Fatalf("yaml.Marshal error = %v", err)
	}
	if !strings.Contains(string(out), "upstreamDefinitions:") || !strings.Contains(string(out), "alt-backend") {
		t.Errorf("emitted YAML missing upstreamDefinitions/alt-backend:\n%s", out)
	}
}

// TestBuildAPIDeploymentYAML_EmitsPerOpUpstreamRef verifies a per-operation upstream ref is
// emitted into the operation in the deployment YAML, and only for the overridden operation.
func TestBuildAPIDeploymentYAML_EmitsPerOpUpstreamRef(t *testing.T) {
	util := &APIUtil{}
	ctx := "/test"
	apiModel := &model.API{
		Name:    "PerOp API",
		Handle:  "perop-api",
		Version: "v1.0",
		Kind:    constants.RestApi,
		Configuration: model.RestAPIConfig{
			Context:  &ctx,
			Upstream: model.UpstreamConfig{Main: &model.UpstreamEndpoint{URL: "http://main:8080"}},
			UpstreamDefinitions: []model.ReusableUpstream{
				{Name: "alt-backend", Upstreams: []model.UpstreamTarget{{URL: "http://alt:9090"}}},
			},
			Operations: []model.Operation{
				{Request: &model.OperationRequest{
					Method: "GET", Path: "/whoami",
					Upstream: &model.OperationUpstream{Main: &model.OperationUpstreamRef{Ref: "alt-backend"}},
				}},
				{Request: &model.OperationRequest{Method: "GET", Path: "/ping"}},
			},
		},
	}

	d, err := util.BuildAPIDeploymentYAML(apiModel)
	if err != nil {
		t.Fatalf("BuildAPIDeploymentYAML() error = %v", err)
	}
	if len(d.Spec.Operations) != 2 {
		t.Fatalf("want 2 operations, got %+v", d.Spec.Operations)
	}
	whoami := d.Spec.Operations[0]
	if whoami.Upstream == nil || whoami.Upstream.Main == nil || whoami.Upstream.Main.Ref != "alt-backend" {
		t.Errorf("/whoami per-op ref missing or wrong: %+v", whoami.Upstream)
	}
	ping := d.Spec.Operations[1]
	if ping.Upstream != nil {
		t.Errorf("/ping must not carry a per-op upstream: %+v", ping.Upstream)
	}

	out, err := yaml.Marshal(d)
	if err != nil {
		t.Fatalf("yaml.Marshal error = %v", err)
	}
	if !strings.Contains(string(out), "ref: alt-backend") {
		t.Errorf("emitted YAML missing per-op ref:\n%s", out)
	}
}

// TestBuildAPIDeploymentYAML_WebSubOmitsUpstreamDefinitions ensures the pool is emitted only
// for REST APIs: a WebSub deployment must not carry upstreamDefinitions even if the model has them.
func TestBuildAPIDeploymentYAML_WebSubOmitsUpstreamDefinitions(t *testing.T) {
	util := &APIUtil{}
	ctx := "/events"
	apiModel := &model.API{
		Name:    "WebSub API",
		Handle:  "websub-api",
		Version: "v1.0",
		Kind:    constants.WebSubApi,
		Configuration: model.RestAPIConfig{
			Context: &ctx,
			UpstreamDefinitions: []model.ReusableUpstream{
				{Name: "alt-backend", Upstreams: []model.UpstreamTarget{{URL: "http://alt:9090"}}},
			},
		},
	}

	d, err := util.BuildAPIDeploymentYAML(apiModel)
	if err != nil {
		t.Fatalf("BuildAPIDeploymentYAML() error = %v", err)
	}
	if len(d.Spec.UpstreamDefinitions) != 0 {
		t.Errorf("WebSub deployment must not emit upstreamDefinitions, got %+v", d.Spec.UpstreamDefinitions)
	}
	if d.Spec.Upstream != nil {
		t.Errorf("WebSub deployment must not emit upstream, got %+v", d.Spec.Upstream)
	}
}

// TestUpstreamDefinitions_RoundTripThroughModel verifies the pool and per-operation refs
// survive RESTAPIToModel and ModelToRESTAPI without loss.
func TestUpstreamDefinitions_RoundTripThroughModel(t *testing.T) {
	util := &APIUtil{}
	basePath := "/api/v2"
	connect := "5s"
	weight := 80
	pool := []api.ReusableUpstream{{
		Name:     "alt-backend",
		BasePath: &basePath,
		Timeout:  &api.UpstreamTimeout{Connect: &connect},
	}}
	pool[0].Upstreams = append(pool[0].Upstreams, struct {
		Url    string `json:"url" yaml:"url"`
		Weight *int   `json:"weight,omitempty" yaml:"weight,omitempty"`
	}{Url: "http://alt:9090", Weight: &weight})

	mainURL := "http://main:8080"
	ops := []api.Operation{{Request: api.OperationRequest{
		Method: api.OperationRequestMethodGET, Path: "/whoami",
		Upstream: &api.OperationUpstream{Main: &struct {
			Ref api.UpstreamReference `json:"ref" yaml:"ref"`
		}{Ref: "alt-backend"}},
	}}}

	rest := &api.RESTAPI{
		DisplayName:         "RoundTrip API",
		Context:             "/rt",
		Version:             "v1",
		ProjectId:           "proj-handle",
		Upstream:            api.Upstream{Main: api.UpstreamDefinition{Url: &mainURL}},
		UpstreamDefinitions: &pool,
		Operations:          &ops,
	}

	m := util.RESTAPIToModel(rest, "org-1")
	if len(m.Configuration.UpstreamDefinitions) != 1 {
		t.Fatalf("model pool missing: %+v", m.Configuration.UpstreamDefinitions)
	}
	back, err := util.ModelToRESTAPI(m, "proj-handle")
	if err != nil {
		t.Fatalf("ModelToRESTAPI: %v", err)
	}
	if back.UpstreamDefinitions == nil || len(*back.UpstreamDefinitions) != 1 {
		t.Fatalf("round-trip pool missing: %+v", back.UpstreamDefinitions)
	}
	def := (*back.UpstreamDefinitions)[0]
	if def.Name != "alt-backend" || def.BasePath == nil || *def.BasePath != "/api/v2" ||
		def.Timeout == nil || def.Timeout.Connect == nil || *def.Timeout.Connect != "5s" ||
		len(def.Upstreams) != 1 || def.Upstreams[0].Url != "http://alt:9090" ||
		def.Upstreams[0].Weight == nil || *def.Upstreams[0].Weight != 80 {
		t.Errorf("round-trip pool mismatch: %+v", def)
	}
	backOps := *back.Operations
	if len(backOps) != 1 || backOps[0].Request.Upstream == nil || backOps[0].Request.Upstream.Main == nil ||
		backOps[0].Request.Upstream.Main.Ref != "alt-backend" {
		t.Errorf("round-trip per-op ref mismatch: %+v", backOps)
	}
}

// TestBuildAPIDeploymentYAML_EmitsAPILevelUpstreamRefs verifies API-level main and sandbox
// refs are emitted as upstream.main.ref / upstream.sandbox.ref in the deployment YAML.
func TestBuildAPIDeploymentYAML_EmitsAPILevelUpstreamRefs(t *testing.T) {
	util := &APIUtil{}
	ctx := "/test"
	apiModel := &model.API{
		Name:    "Ref API",
		Handle:  "ref-api",
		Version: "v1.0",
		Kind:    constants.RestApi,
		Configuration: model.RestAPIConfig{
			Context: &ctx,
			Upstream: model.UpstreamConfig{
				Main:    &model.UpstreamEndpoint{Ref: "production-pool"},
				Sandbox: &model.UpstreamEndpoint{Ref: "sandbox-pool"},
			},
			UpstreamDefinitions: []model.ReusableUpstream{
				{Name: "production-pool", Upstreams: []model.UpstreamTarget{{URL: "http://prod:9090"}}},
				{Name: "sandbox-pool", Upstreams: []model.UpstreamTarget{{URL: "http://sb:9090"}}},
			},
			Operations: []model.Operation{
				{Request: &model.OperationRequest{Method: "GET", Path: "/x"}},
			},
		},
	}

	d, err := util.BuildAPIDeploymentYAML(apiModel)
	if err != nil {
		t.Fatalf("BuildAPIDeploymentYAML() error = %v", err)
	}
	if d.Spec.Upstream == nil || d.Spec.Upstream.Main == nil || d.Spec.Upstream.Main.Ref != "production-pool" {
		t.Errorf("main ref mismatch: %+v", d.Spec.Upstream)
	}
	if d.Spec.Upstream.Main.URL != "" {
		t.Errorf("main url must be empty when ref is set, got %q", d.Spec.Upstream.Main.URL)
	}
	if d.Spec.Upstream.Sandbox == nil || d.Spec.Upstream.Sandbox.Ref != "sandbox-pool" {
		t.Errorf("sandbox ref mismatch: %+v", d.Spec.Upstream.Sandbox)
	}

	out, err := yaml.Marshal(d)
	if err != nil {
		t.Fatalf("yaml.Marshal error = %v", err)
	}
	if !strings.Contains(string(out), "ref: production-pool") || !strings.Contains(string(out), "ref: sandbox-pool") {
		t.Errorf("emitted YAML missing API-level refs:\n%s", out)
	}
}

// TestBuildAPIDeploymentYAML_EmitsWeightedTargetsIncludingZero verifies a multi-target pool
// keeps every target with its weight, and that an explicit zero weight is preserved rather
// than dropped by omitempty.
func TestBuildAPIDeploymentYAML_EmitsWeightedTargetsIncludingZero(t *testing.T) {
	util := &APIUtil{}
	ctx := "/test"
	w80, w20, w0 := 80, 20, 0
	apiModel := &model.API{
		Name:    "Weighted API",
		Handle:  "weighted-api",
		Version: "v1.0",
		Kind:    constants.RestApi,
		Configuration: model.RestAPIConfig{
			Context:  &ctx,
			Upstream: model.UpstreamConfig{Main: &model.UpstreamEndpoint{URL: "http://main:8080"}},
			UpstreamDefinitions: []model.ReusableUpstream{
				{
					Name: "weighted-pool",
					Upstreams: []model.UpstreamTarget{
						{URL: "http://backend-a:8080", Weight: &w80},
						{URL: "http://backend-b:8080", Weight: &w20},
						{URL: "http://backend-c:8080", Weight: &w0},
					},
				},
			},
			Operations: []model.Operation{
				{Request: &model.OperationRequest{Method: "GET", Path: "/x"}},
			},
		},
	}

	d, err := util.BuildAPIDeploymentYAML(apiModel)
	if err != nil {
		t.Fatalf("BuildAPIDeploymentYAML() error = %v", err)
	}
	got := d.Spec.UpstreamDefinitions[0].Upstreams
	if len(got) != 3 {
		t.Fatalf("want 3 targets, got %+v", got)
	}
	for i, want := range []int{80, 20, 0} {
		if got[i].Weight == nil || *got[i].Weight != want {
			t.Errorf("target %d weight = %v, want %d", i, got[i].Weight, want)
		}
	}

	out, err := yaml.Marshal(d)
	if err != nil {
		t.Fatalf("yaml.Marshal error = %v", err)
	}
	if !strings.Contains(string(out), "weight: 0") {
		t.Errorf("explicit zero weight must survive into the YAML:\n%s", out)
	}
}
