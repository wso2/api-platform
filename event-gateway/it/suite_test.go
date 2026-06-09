// --------------------------------------------------------------------
// Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.
// --------------------------------------------------------------------

package it

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cucumber/godog"
)

var (
	composeManager *ComposeManager
	testState      *TestState
)

// TestFeatures is the main entry point for BDD integration tests.
func TestFeatures(t *testing.T) {
	suite := godog.TestSuite{
		TestSuiteInitializer: InitializeTestSuite,
		ScenarioInitializer:  InitializeScenario,
		Options: &godog.Options{
			Strict:   true,
			Format:   "pretty",
			Paths:    getFeaturePaths(),
			TestingT: t,
		},
	}

	if exitCode := suite.Run(); exitCode != 0 {
		t.Fatal("non-zero status returned, failed to run feature tests")
	}
}

// getFeaturePaths returns the feature files to run (can be overridden via IT_FEATURE_PATHS env var).
func getFeaturePaths() []string {
	defaults := []string{
		"features/health.feature",
		"features/websub-api-management.feature",
		"features/websub-e2e.feature",
		"features/webbroker-api-management.feature",
		"features/webbroker-e2e.feature",
		"features/websub-webhook-secrets.feature",
	}

	raw := strings.TrimSpace(os.Getenv("IT_FEATURE_PATHS"))
	if raw == "" {
		return defaults
	}

	var paths []string
	for _, entry := range strings.Split(raw, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		if strings.HasPrefix(entry, "features/") || filepath.IsAbs(entry) {
			paths = append(paths, entry)
		} else {
			paths = append(paths, filepath.Join("features", entry))
		}
	}
	if len(paths) == 0 {
		return defaults
	}
	log.Printf("Using feature paths from IT_FEATURE_PATHS: %s", strings.Join(paths, ", "))
	return paths
}

// InitializeTestSuite sets up the stack once before all scenarios.
func InitializeTestSuite(ctx *godog.TestSuiteContext) {
	ctx.BeforeSuite(func() {
		log.Println("=== Event Gateway Integration Test Suite Starting ===")

		if err := CheckDockerAvailable(); err != nil {
			log.Fatalf("Pre-flight check failed: %v", err)
		}

		if err := CheckPortsAvailable(); err != nil {
			log.Fatalf("Pre-flight check failed: %v", err)
		}

		var err error
		composeManager, err = NewComposeManager("../docker-compose.dev.yaml")
		if err != nil {
			log.Fatalf("Failed to create compose manager: %v", err)
		}

		if err := composeManager.Start(); err != nil {
			log.Fatalf("Failed to start services: %v", err)
		}

		// Extra settle time for xDS push to complete
		log.Println("Giving services extra time to settle...")
		time.Sleep(5 * time.Second)

		testState = NewTestState(DefaultConfig())
		log.Println("=== Test suite ready ===")
	})

	ctx.AfterSuite(func() {
		log.Println("=== Event Gateway Integration Test Suite Tearing Down ===")
		if composeManager != nil {
			logsDir := filepath.Join("logs", "compose-logs.txt")
			if err := composeManager.DumpLogs(logsDir); err != nil {
				log.Printf("Warning: failed to dump compose logs: %v", err)
			}
			composeManager.Cleanup()
		}
	})
}

// InitializeScenario registers step definitions and resets state before each scenario.
func InitializeScenario(ctx *godog.ScenarioContext) {
	whSecretSteps := &WebhookSecretSteps{}

	ctx.Before(func(gctx context.Context, sc *godog.Scenario) (context.Context, error) {
		if testState != nil {
			testState.Reset()
		}
		whSecretSteps.Reset()
		return gctx, nil
	})

	RegisterHealthSteps(ctx, testState)
	RegisterWebSubSteps(ctx, testState)
	RegisterWebBrokerSteps(ctx, testState)
	RegisterCommonSteps(ctx, testState)
	RegisterWebhookSecretSteps(ctx, testState, whSecretSteps)
}
