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
	"slices"
	"strings"
	"sync"
	"time"

	corecatalog "github.com/wso2/api-platform/tests/framework/core/catalog/cloudconsole"
	frameworkruntime "github.com/wso2/api-platform/tests/framework/core/runtime"
	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	"github.com/wso2/api-platform/tests/framework/core/util/retry"
	"github.com/wso2/api-platform/tests/framework/core/util/unique"
	"github.com/wso2/api-platform/tests/framework/suites/cloud/setup/starter"
)

// defaultGatewayDisplayName is the base of the display name of the gateway the starter creates
// when the organisation has none the scenarios can use. The starter appends its own name suffix,
// which the finalizer is told to look for, so the finalizer removes the gateway at the end of the
// run. The name is not that of a synthetic gateway, so the scenarios choose it. It is short on
// purpose: the platform limits a gateway's handle to 37 minus twice the environment name's length
// (15 characters for "development"), and the handle is the display name folded to lower case.
const defaultGatewayDisplayName = "dgw"

// isStarterGateway reports whether any of the names is that of a gateway a starter created.
func isStarterGateway(names []string) bool {
	for _, name := range names {
		if strings.HasPrefix(strings.ToLower(name), defaultGatewayDisplayName+"-") ||
			strings.Contains(strings.ToLower(name), "-"+defaultGatewayDisplayName+"-") {
			return true
		}
	}
	return false
}

// preferredGatewayEnvironment is the environment the starter creates the default gateway in when
// the default project's deployment pipeline reaches it.
const preferredGatewayEnvironment = "development"

// gatewayReadyTimeout bounds how long the starter waits for a gateway it created, or one that is
// still being built, to become active. The platform builds a gateway through its own control plane,
// which takes minutes; the wait never gets less than five.
const gatewayReadyTimeout = 15 * time.Minute

// gatewayPollInterval is how often the starter checks a gateway that is being built.
const gatewayPollInterval = 10 * time.Second

// gatewayStatusActive is the status of a gateway that is connected and serves traffic.
const gatewayStatusActive = "ACTIVE"

// gatewayOutcome says which gateway the starter settled on and whether it created it.
type gatewayOutcome struct {
	// ID is the gateway the scenarios will find.
	ID string
	// Created is true when the starter built the gateway, so the finalizer removes it at the end
	// of the run. A gateway the organisation already had is false here and is never removed.
	Created bool
}

// Starter makes sure the organisation a cloud run talks to holds the prerequisites the scenarios
// rely on. Today that is one regular gateway to deploy to; the scenarios find it with "I find an
// active APIP gateway" and never create it themselves, because a gateway takes minutes to build.
//
// What the starter creates is named with a suffix of its own, which it hands to the Finalizer
// before it creates anything, so the run-end sweep removes it even when the run fails early. A
// gateway the organisation already had is never created by the starter and never removed.
type Starter struct {
	starter   *starter.Starter
	client    *httpx.Client
	finalizer *Finalizer
	suffix    string
	log       *slog.Logger

	mu      sync.Mutex
	outcome gatewayOutcome
}

// NewStarter returns a starter that has prepared no organisation yet. The finalizer, when given,
// removes at the end of the run what the starter created.
func NewStarter(finalizer *Finalizer) (*Starter, error) {
	generator, err := unique.NewGenerator()
	if err != nil {
		return nil, fmt.Errorf("cloud run starter: %w", err)
	}
	log := slog.Default()
	return &Starter{
		starter:   starter.New(log),
		client:    httpx.NewClient(httpx.Options{MaxRetries: 1}),
		finalizer: finalizer,
		suffix:    strings.ToLower(generator.Suffix()),
		log:       log,
	}, nil
}

// gatewayDisplayName is the display name of the default gateway this starter creates.
func (s *Starter) gatewayDisplayName() string {
	return defaultGatewayDisplayName + "-" + s.suffix
}

// Suffix is the name suffix of the gateway this starter creates, which a finalizer needs to find it.
func (s *Starter) Suffix() string { return s.suffix }

// Gateway reports the gateway the last successful Start settled on and whether the starter created
// it.
func (s *Starter) Gateway() (id string, created bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.outcome.ID, s.outcome.Created
}

// Start makes sure the organisation behind the block's platform endpoint holds its prerequisites.
// It runs once per platform however many runners ask; the others wait for that run and share its
// result.
func (s *Starter) Start(ctx context.Context, topo *frameworkruntime.Topology) error {
	base, err := topo.URL(corecatalog.Name, corecatalog.EndpointAPIPPlatformAPI)
	if err != nil {
		return err
	}
	return s.StartAt(ctx, base)
}

// StartAt is Start for the platform API endpoint given as a URL, for a caller that has no topology.
func (s *Starter) StartAt(ctx context.Context, base string) error {
	token := strings.TrimSpace(os.Getenv(cloudConsoleAccessTokenEnv))
	if token == "" {
		return fmt.Errorf("cloud run starter: environment variable %s is required to prepare the organisation", cloudConsoleAccessTokenEnv)
	}
	if s.finalizer != nil {
		s.finalizer.Watch(base, s.suffix)
	}
	platform := organisationAPI{client: s.client, base: base, token: token}
	return s.starter.Ensure(ctx, base, []starter.Prerequisite{
		{Name: "default gateway", Ensure: func(ctx context.Context) error {
			outcome, err := ensureDefaultGateway(ctx, platform, s.gatewayDisplayName(), s.log)
			if err == nil {
				s.mu.Lock()
				s.outcome = outcome
				s.mu.Unlock()
			}
			return err
		}},
	})
}

// organisationAPI sends authenticated requests to the platform's API for one organisation.
type organisationAPI struct {
	client *httpx.Client
	base   string
	token  string
	// pollInterval is how often a gateway that is being built is checked; zero means
	// gatewayPollInterval.
	pollInterval time.Duration
}

func (p organisationAPI) do(ctx context.Context, method, path string, body []byte) (*httpx.Response, error) {
	return p.client.Do(ctx, httpx.Request{
		Method: method,
		URL:    cloudAPIURL(p.base, path),
		Headers: map[string]string{
			"Authorization": "Bearer " + p.token,
			"Content-Type":  "application/json",
		},
		Body: body,
	}, 0, 0)
}

func (p organisationAPI) get(ctx context.Context, path string) (*httpx.Response, error) {
	return p.do(ctx, http.MethodGet, path, nil)
}

// ensureDefaultGateway leaves the organisation with a regular gateway that is active and that the
// default project's deployment pipeline accepts a deployment to. A gateway that is still being
// built is waited for rather than duplicated; only when none exists is one created.
func ensureDefaultGateway(ctx context.Context, platform organisationAPI, displayName string, log *slog.Logger) (gatewayOutcome, error) {
	paths, err := pipelinePathsOf(ctx, defaultProjectID, platform.get)
	if err != nil {
		return gatewayOutcome{}, fmt.Errorf("reading the default project's deployment pipeline: %w", err)
	}
	environments := environmentsOf(paths)

	pin := strings.TrimSpace(os.Getenv(cloudGatewayPinEnv))
	filters := []gatewayFilter{regularGatewaysOnly, notSyntheticGateway, inPipelineEnvironments(environments)}
	if pin != "" {
		filters = append(filters, pinnedGateway(pin))
	}
	items, err := listCollectionQuery(ctx, platform.client, platform.base, platform.token, "/managed-gateways",
		url.Values{"functionalityType": {regularGatewayKind}})
	if err != nil {
		return gatewayOutcome{}, fmt.Errorf("listing the managed gateways: %w", err)
	}

	var building []string
	for _, item := range items {
		id, _ := item["id"].(string)
		if id == "" || rejectedByAny(item, filters) != "" {
			continue
		}
		switch status, _ := item["status"].(string); status {
		case gatewayStatusActive:
			log.Info("the organisation already has a gateway to deploy to; leaving it as it is", "gateway", id)
			return gatewayOutcome{ID: id}, nil
		case "PROVISIONING":
			building = append(building, id)
		}
	}
	if len(building) > 0 {
		log.Info("waiting for a gateway that is still being built", "gateway", building[0])
		return gatewayOutcome{ID: building[0]}, awaitGatewayActive(ctx, platform, building[0])
	}
	if pin != "" {
		return gatewayOutcome{}, fmt.Errorf("%s names gateway %q, which is not an active regular gateway in the default project's pipeline", cloudGatewayPinEnv, pin)
	}

	environment, err := defaultGatewayEnvironment(paths)
	if err != nil {
		return gatewayOutcome{}, err
	}
	id, err := createDefaultGateway(ctx, platform, environment, displayName)
	if err != nil {
		return gatewayOutcome{}, err
	}
	log.Info("created the default gateway; waiting for it to become active", "gateway", id, "environment", environment)
	return gatewayOutcome{ID: id, Created: true}, awaitGatewayActive(ctx, platform, id)
}

// rejectedByAny returns why the first filter that rejects a gateway does, or "" when none does.
func rejectedByAny(item map[string]any, filters []gatewayFilter) string {
	for _, filter := range filters {
		if rejected := filter(item); rejected != "" {
			return rejected
		}
	}
	return ""
}

// defaultGatewayEnvironment picks the environment the default gateway is created in: the
// preferred one when the pipeline reaches it, otherwise the first stage of the pipeline.
func defaultGatewayEnvironment(paths []promotionPath) (string, error) {
	if environmentsOf(paths)[preferredGatewayEnvironment] {
		return preferredGatewayEnvironment, nil
	}
	targets := map[string]bool{}
	for _, path := range paths {
		for _, target := range path.TargetEnvironments {
			targets[target] = true
		}
	}
	var stages []string
	for _, path := range paths {
		if path.SourceEnvironment != "" && !targets[path.SourceEnvironment] && !slices.Contains(stages, path.SourceEnvironment) {
			stages = append(stages, path.SourceEnvironment)
		}
	}
	if len(stages) == 0 {
		return "", errors.New("the default project's deployment pipeline has no environment to create a gateway in")
	}
	slices.Sort(stages)
	return stages[0], nil
}

// createDefaultGateway asks the platform for the default gateway and returns the id it answered
// with. A conflict that leaves a gateway of that name in the organisation means another run has
// just asked for the same gateway, so that gateway is the one to wait for.
func createDefaultGateway(ctx context.Context, platform organisationAPI, environment, displayName string) (string, error) {
	body, err := json.Marshal(map[string]any{
		"displayName":       displayName,
		"environment":       environment,
		"functionalityType": regularGatewayKind,
		"description":       "Gateway the APIP cloud integration tests deploy to. Created by the suite's starter and removed by its finalizer.",
	})
	if err != nil {
		return "", err
	}
	resp, err := platform.do(ctx, http.MethodPost, "/managed-gateways", body)
	if err != nil {
		return "", err
	}
	switch resp.StatusCode {
	case http.StatusCreated, http.StatusAccepted:
		id, err := stringField(resp, "id")
		if err != nil {
			return "", fmt.Errorf("creating the default gateway: %w", err)
		}
		return id, nil
	case http.StatusConflict:
		// A conflict can also mean the platform refused the request for a reason of its own,
		// such as a recipe it cannot run, so the gateway must be there before it is waited for.
		id := expectedGatewayID(environment, displayName, "")
		found, err := platform.get(ctx, "/managed-gateways/"+url.PathEscape(id))
		if err != nil {
			return "", err
		}
		if found.StatusCode != http.StatusOK {
			return "", fmt.Errorf("creating the default gateway: %w", responseError(resp))
		}
		return id, nil
	default:
		return "", fmt.Errorf("creating the default gateway: %w", responseError(resp))
	}
}

// awaitGatewayActive waits until the managed gateway reports ACTIVE. A gateway that reports FAILED
// stops the wait at once with the platform's reason, since it will not recover.
func awaitGatewayActive(ctx context.Context, platform organisationAPI, id string) error {
	interval := platform.pollInterval
	if interval <= 0 {
		interval = gatewayPollInterval
	}
	return retry.Await(ctx, retry.Options{Timeout: gatewayReadyTimeout, Interval: interval},
		func(ctx context.Context) (string, error) {
			resp, err := platform.get(ctx, "/managed-gateways/"+url.PathEscape(id))
			if err != nil {
				return "", retry.Transient(err)
			}
			if resp.StatusCode >= http.StatusInternalServerError || resp.StatusCode == http.StatusNotFound {
				return "", retry.Transient(responseError(resp))
			}
			if resp.StatusCode != http.StatusOK {
				return "", responseError(resp)
			}
			status, err := stringField(resp, "status")
			if err != nil {
				return "", err
			}
			if status == "FAILED" {
				reason, _ := stringField(resp, "statusReason")
				return "", fmt.Errorf("gateway %q failed to provision: %s; delete it and run again", id, reason)
			}
			return status, nil
		},
		func(status string) bool { return status == gatewayStatusActive },
		"waiting for gateway "+id+" to become active")
}
