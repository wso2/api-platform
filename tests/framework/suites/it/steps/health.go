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

package steps

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/cucumber/godog"

	"github.com/wso2/api-platform/tests/framework/core/cleanup"
	"github.com/wso2/api-platform/tests/framework/core/components"
	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	"github.com/wso2/api-platform/tests/framework/core/util/retry"
	stepscommon "github.com/wso2/api-platform/tests/framework/suites/it/steps/common"
)

// healthComponents are the component names a health step accepts. Each health endpoint is
// served on a different listener of its component.
const healthComponents = `(platform-api|API Portal|gateway controller|gateway runtime)`

// healthTransitionTimeout bounds a wait for a health endpoint to change state. A database
// check reports a failed server within its idle period plus the cache lifetime, but reports a
// recovered one only after the server has restarted and the failures have left the check's
// failure window.
const healthTransitionTimeout = 3 * time.Minute

// databaseOutageKind restores a database server a scenario took down. It is cleaned before
// every other kind, so the components are reachable again when their own cleanup runs.
var databaseOutageKind = cleanup.Kind{Name: "database-server-outage", Order: 0}

// Database server outages a scenario can apply, named by the step verb that applies them.
const (
	outageStopped = "stop"
	outagePaused  = "pause"
)

func (b *Base) registerHealthSteps(sc *godog.ScenarioContext) {
	sc.Step(`^I send a "([^"]*)" request to the `+healthComponents+
		` health path "([^"]*)"(?: with header "([^"]*)" set to "([^"]*)")?$`, b.sendHealth)
	sc.Step(`^I send a "([^"]*)" request to the `+healthComponents+
		` health path "([^"]*)" until status (\d+)$`, b.sendHealthUntilStatus)
	sc.Step(`^I (stop|start|pause|unpause) the "([^"]*)" database server$`, b.controlDatabaseServer)
}

// healthRequest builds a request for a health path of a component. The scenario's headers
// apply to every component; the scenario's host override applies to the gateway runtime only,
// whose health routes answer on every virtual host.
func (b *Base) healthRequest(ctx context.Context, method, component, path, header, value string) (httpx.Request, error) {
	method = strings.ToUpper(strings.TrimSpace(method))
	if method == "" {
		return httpx.Request{}, fmt.Errorf("a health request needs a method")
	}
	path, err := stepscommon.Expand(ctx, path)
	if err != nil {
		return httpx.Request{}, err
	}
	if !strings.HasPrefix(path, "/") {
		return httpx.Request{}, fmt.Errorf("health path %q must start with /", path)
	}
	url, err := b.healthURL(component, path)
	if err != nil {
		return httpx.Request{}, err
	}
	headers := b.scenarioHeaders(ctx)
	if name := strings.TrimSpace(header); name != "" {
		expanded, expandErr := stepscommon.Expand(ctx, value)
		if expandErr != nil {
			return httpx.Request{}, expandErr
		}
		headers[name] = expanded
	}
	req := httpx.Request{Method: method, URL: url, Headers: headers}
	if component == "gateway runtime" {
		req.Host = b.requestHost(ctx)
	}
	return req, nil
}

// healthURL resolves a health path against the listener the component serves it on.
func (b *Base) healthURL(component, path string) (string, error) {
	var (
		base string
		err  error
	)
	switch component {
	case "platform-api":
		base, err = b.topo.URL("platform-api", "https")
	case "API Portal":
		base, err = b.topo.URL("api-portal", "https")
	case "gateway controller":
		base, err = b.topo.URL("platform-gateway", "admin")
	case "gateway runtime":
		return b.gatewayURL(path)
	default:
		return "", fmt.Errorf("unknown health component %q", component)
	}
	if err != nil {
		return "", err
	}
	return base + path, nil
}

// healthFunnel returns the funnel whose client trusts the component's certificate.
func (b *Base) healthFunnel(component string) *httpx.Funnel {
	if component == "API Portal" {
		return b.portalFunnel
	}
	return b.funnel
}

func (b *Base) sendHealth(ctx context.Context, method, component, path, header, value string) error {
	req, err := b.healthRequest(ctx, method, component, path, header, value)
	if err != nil {
		return err
	}
	if _, err := b.healthFunnel(component).Send(ctx, req); err != nil {
		return fmt.Errorf("%s %s health path %s: %w", req.Method, component, path, err)
	}
	return nil
}

// sendHealthUntilStatus polls a health path until it answers with the wanted status, and
// publishes that answer.
func (b *Base) sendHealthUntilStatus(ctx context.Context, method, component, path string, want int) error {
	req, err := b.healthRequest(ctx, method, component, path, "", "")
	if err != nil {
		return err
	}
	funnel := b.healthFunnel(component)
	return retry.Await[*httpx.Response](ctx, retry.Options{Timeout: healthTransitionTimeout},
		func(ctx context.Context) (*httpx.Response, error) {
			resp, sendErr := funnel.Send(ctx, req)
			if sendErr != nil {
				return nil, retry.Transient(sendErr)
			}
			return resp, nil
		},
		func(r *httpx.Response) bool { return r != nil && r.StatusCode == want },
		fmt.Sprintf("waiting for %s %s health path %s to return %d", req.Method, component, path, want))
}

// controlDatabaseServer takes the block's database server down or brings it back. Stopping it
// makes every connection fail; pausing it leaves connections open but unanswered. Taking it
// down registers its restoration for cleanup, so a failing scenario cannot leave the block
// without a database.
func (b *Base) controlDatabaseServer(ctx context.Context, action, engine string) error {
	server, err := b.topo.DatabaseServer(components.DBType(strings.TrimSpace(engine)))
	if err != nil {
		return err
	}
	switch action {
	case "stop":
		if err := registerDatabaseOutage(ctx, b.databaseServer, engine, outageStopped); err != nil {
			return err
		}
		return server.Halt(ctx)
	case "pause":
		if err := registerDatabaseOutage(ctx, b.databaseServer, engine, outagePaused); err != nil {
			return err
		}
		return server.Pause(ctx)
	case "start":
		if err := server.Resume(ctx); err != nil {
			return err
		}
		return deregisterDatabaseOutage(ctx, engine, outageStopped)
	case "unpause":
		if err := server.Unpause(ctx); err != nil {
			return err
		}
		return deregisterDatabaseOutage(ctx, engine, outagePaused)
	default:
		return fmt.Errorf("unknown database server action %q", action)
	}
}

// databaseServer resolves the block's database server for an engine.
func (b *Base) databaseServer(engine components.DBType) (databaseServer, error) {
	return b.topo.DatabaseServer(engine)
}

// databaseServerLookup resolves a block's database server by engine.
type databaseServerLookup func(components.DBType) (databaseServer, error)

// databaseServer is the part of a database server container an outage restoration needs.
type databaseServer interface {
	Resume(context.Context) error
	Unpause(context.Context) error
}

// registerDatabaseOutage records an outage so cleanup undoes it. The resource ID names the
// engine and the outage, which tells the deleter how to restore the server.
func registerDatabaseOutage(ctx context.Context, lookup databaseServerLookup, engine, outage string) error {
	reg, err := cleanup.Of(ctx)
	if err != nil {
		return err
	}
	if err := reg.RegisterDeleter(databaseOutageKind, func(ctx context.Context, res cleanup.Resource) error {
		return restoreDatabaseServer(ctx, lookup, res.ID)
	}); err != nil {
		return err
	}
	return reg.Register(cleanup.Resource{
		Kind: databaseOutageKind, ID: databaseOutageID(engine, outage), Actor: "admin",
		Description: fmt.Sprintf("%s database server taken down by the scenario", engine),
	})
}

func deregisterDatabaseOutage(ctx context.Context, engine, outage string) error {
	reg, err := cleanup.Of(ctx)
	if err != nil {
		return err
	}
	reg.Deregister(databaseOutageKind, databaseOutageID(engine, outage))
	return nil
}

func databaseOutageID(engine, outage string) string {
	return strings.TrimSpace(engine) + "/" + outage
}

// restoreDatabaseServer undoes the outage a cleanup resource ID records.
func restoreDatabaseServer(ctx context.Context, lookup databaseServerLookup, id string) error {
	engine, outage, ok := strings.Cut(id, "/")
	if !ok || engine == "" {
		return fmt.Errorf("malformed database outage id %q", id)
	}
	server, err := lookup(components.DBType(engine))
	if err != nil {
		return err
	}
	switch outage {
	case outageStopped:
		return server.Resume(ctx)
	case outagePaused:
		return server.Unpause(ctx)
	default:
		return fmt.Errorf("database outage id %q names no known outage", id)
	}
}
