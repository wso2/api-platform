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
	"context"
	"os"
	"testing"

	"github.com/cucumber/godog"
)

func TestMain(m *testing.M) {
	// Allow GetConfig() to generate an ephemeral secret_encryption_key so tests
	// that exercise subscription_repository.go don't panic at startup.
	os.Setenv("APIP_DEMO_MODE", "true")
	os.Exit(m.Run())
}

// TestFeatures is the go test entry point that runs the godog (Cucumber) suite.
// It is selected by `go test -tags integration ./internal/integration/...` and
// runs every scenario in features/ against the database engine chosen by IT_DB
// (sqlite | postgres | sqlserver), exactly as the Makefile it-* targets do.
func TestFeatures(t *testing.T) {
	status := godog.TestSuite{
		Name:                "platform-api-cross-db",
		ScenarioInitializer: initializeScenario,
		Options: &godog.Options{
			Format:   "pretty",
			Paths:    []string{"features"},
			Strict:   true,
			TestingT: t,
		},
	}.Run()
	if status != 0 {
		t.Fatalf("godog suite failed with status %d", status)
	}
}

// initializeScenario wires a fresh world (and a fresh database) per scenario and
// registers every step group, so scenarios stay independent.
func initializeScenario(ctx *godog.ScenarioContext) {
	w := &world{}

	ctx.Before(func(ctx context.Context, sc *godog.Scenario) (context.Context, error) {
		*w = world{}
		return ctx, nil
	})
	ctx.After(func(ctx context.Context, sc *godog.Scenario, err error) (context.Context, error) {
		w.close()
		return ctx, nil
	})

	// Shared background steps.
	ctx.Step(`^a clean platform-api database$`, w.aCleanDatabase)

	registerCRUDSteps(ctx, w)
	registerCascadeSteps(ctx, w)
	registerPaginationSteps(ctx, w)
	registerLLMSteps(ctx, w)
	registerMCPSteps(ctx, w)
	registerWebBrokerSteps(ctx, w)
	registerSecretSteps(ctx, w)
	registerDeploymentSteps(ctx, w)
}
