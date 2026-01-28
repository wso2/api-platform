/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

package it

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"testing"

	"github.com/cucumber/godog"
	"github.com/wso2/api-platform/cli/it/steps"
)

var (
	// Global infrastructure manager
	infraManager *InfrastructureManager

	// Global test reporter
	testReporter *TestReporter

	// Global test state
	testState *TestState

	// Global test configuration
	testConfig *TestConfig

	// Path to the loaded test config file
	testConfigPath string

	// Step handlers
	cliSteps    *steps.CLISteps
	assertSteps *steps.AssertSteps

	// Coverage collector
	coverageCollector *CoverageCollector

	// CLI coverage directory path
	cliCoverDir string
)

// TestFeatures is the main entry point for BDD tests
func TestFeatures(t *testing.T) {
	// Load configuration
	configPath := "test-config.yaml"
	var err error
	testConfig, err = LoadTestConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to load test config: %v", err)
	}
	testConfigPath = configPath

	// Initialize reporter
	logsDir, err := filepath.Abs("logs")
	if err != nil {
		t.Fatalf("Failed to resolve logs directory: %v", err)
	}
	testReporter = NewTestReporter(logsDir)
	if err := testReporter.Setup(); err != nil {
		t.Fatalf("Failed to setup test reporter: %v", err)
	}

	// Print header
	fmt.Printf("\n%s╔══════════════════════════════════════════════════════════════════════════════════╗%s\n", ColorBold, ColorReset)
	fmt.Printf("%s║                        CLI INTEGRATION TESTS                                     ║%s\n", ColorBold, ColorReset)
	fmt.Printf("%s╚══════════════════════════════════════════════════════════════════════════════════╝%s\n", ColorBold, ColorReset)
	fmt.Printf("\n%s→ Config:%s %s\n", ColorCyan, ColorReset, configPath)

	enabledTests := testConfig.GetEnabledTests()
	allTests := testConfig.GetAllTests()
	fmt.Printf("%s→ Tests:%s  %d/%d enabled\n\n", ColorCyan, ColorReset, len(enabledTests), len(allTests))

	suite := godog.TestSuite{
		TestSuiteInitializer: InitializeTestSuite,
		ScenarioInitializer:  InitializeScenario,
		Options: &godog.Options{
			Format:   "progress",
			Paths:    []string{"features"},
			TestingT: t,
			NoColors: false,
			Output:   io.Discard,
		},
	}

	exitCode := suite.Run()

	// Print summary
	testReporter.PrintSummary()

	if exitCode != 0 {
		t.Fatal("Integration tests failed")
	}
}

// InitializeTestSuite sets up the test suite (runs once before all scenarios)
func InitializeTestSuite(ctx *godog.TestSuiteContext) {
	ctx.BeforeSuite(func() {
		fmt.Printf("\n%s┌──────────────────────────────────────────────────────────────────────────────────┐%s\n", ColorBlue, ColorReset)
		fmt.Printf("%s│  PHASE 1: Infrastructure Setup                                                   │%s\n", ColorBlue, ColorReset)
		fmt.Printf("%s└──────────────────────────────────────────────────────────────────────────────────┘%s\n\n", ColorBlue, ColorReset)

		// Backup and clear user's real CLI config to run tests against a clean config.
		// If a backup exists it will be restored in AfterSuite.
		if err := backupAndClearUserConfig(); err != nil {
			log.Fatalf("Failed to backup/clear user config: %v", err)
		}

		// Pre-flight checks
		if err := CheckDockerAvailable(); err != nil {
			log.Fatalf("Pre-flight check failed: Docker is not available. %v", err)
		}
		fmt.Printf("  %s[DOCKER]%s  Docker available %s✓%s\n", ColorBlue, ColorReset, ColorGreen, ColorReset)

		// Export docker registry + image tag from test config so steps can use them
		if testConfig != nil {
			if testConfig.Infrastructure.DockerRegistry != "" {
				os.Setenv("TEST_DOCKER_REGISTRY", testConfig.Infrastructure.DockerRegistry)
			}
			if testConfig.Infrastructure.ImageTag != "" {
				os.Setenv("TEST_IMAGE_TAG", testConfig.Infrastructure.ImageTag)
			}
		}

		// Verify required ports are free before starting infrastructure
		if err := CheckPortsAvailable(); err != nil {
			log.Fatalf("Pre-flight check failed: Required ports are not available. %v", err)
		}
		fmt.Printf("  %s[PORTS]%s  Required ports free %s✓%s\n", ColorBlue, ColorReset, ColorGreen, ColorReset)

		// Setup coverage collection (before infrastructure so directory is ready)
		coverageCollector = NewCoverageCollector(DefaultCoverageConfig())
		if err := coverageCollector.Setup(); err != nil {
			log.Printf("Warning: Failed to setup coverage: %v", err)
		}

		// Create and store CLI coverage directory path
		cliCoverDir, _ = filepath.Abs("coverage/cli")
		if err := os.MkdirAll(cliCoverDir, 0755); err != nil {
			log.Printf("Warning: Failed to create CLI coverage directory: %v", err)
		}

		// Initialize infrastructure manager
		infraManager = NewInfrastructureManager(testReporter, testConfig, testConfigPath)

		// Get required infrastructure based on enabled tests
		required := testConfig.GetRequiredInfrastructure()

		// Setup infrastructure
		if err := infraManager.SetupInfrastructure(required); err != nil {
			log.Fatalf("Phase 1 failed: %v", err)
		}

		fmt.Printf("\n%s✓ Phase 1 Complete: Infrastructure ready%s\n", ColorGreen, ColorReset)

		fmt.Printf("\n%s┌──────────────────────────────────────────────────────────────────────────────────┐%s\n", ColorPurple, ColorReset)
		fmt.Printf("%s│  PHASE 2: Test Execution                                                         │%s\n", ColorPurple, ColorReset)
		fmt.Printf("%s└──────────────────────────────────────────────────────────────────────────────────┘%s\n\n", ColorPurple, ColorReset)
	})

	ctx.AfterSuite(func() {
		fmt.Printf("\n%s┌──────────────────────────────────────────────────────────────────────────────────┐%s\n", ColorGray, ColorReset)
		fmt.Printf("%s│  Cleaning up infrastructure...                                                   │%s\n", ColorGray, ColorReset)
		fmt.Printf("%s└──────────────────────────────────────────────────────────────────────────────────┘%s\n", ColorGray, ColorReset)

		if infraManager != nil {
			if err := infraManager.Teardown(); err != nil {
				fmt.Printf("%sWarning: Teardown error: %v%s\n", ColorYellow, err, ColorReset)
			}
		}

		// Generate coverage reports
		if coverageCollector != nil {
			log.Println("Generating coverage reports...")
			if err := coverageCollector.MergeAndGenerateReport(); err != nil {
				log.Printf("Warning: Failed to generate coverage report: %v", err)
			}
		}

		// Restore user's CLI config that was backed up at test startup.
		if err := restoreUserConfig(); err != nil {
			fmt.Printf("%sWarning: Failed to restore user config: %v%s\n", ColorYellow, err, ColorReset)
		}
	})
}

// backupAndClearUserConfig backs up the user's config file (if present)
// to config.yaml.backup and writes an empty config to the original path.
func backupAndClearUserConfig() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	configPath := filepath.Join(home, ".wso2ap", "config.yaml")
	backupPath := configPath + ".backup"

	// If config exists, copy to backup
	if _, err := os.Stat(configPath); err == nil {
		data, err := os.ReadFile(configPath)
		if err != nil {
			return err
		}
		if err := os.WriteFile(backupPath, data, 0600); err != nil {
			return err
		}
	}

	// Ensure config dir exists
	if err := os.MkdirAll(filepath.Dir(configPath), 0700); err != nil {
		return err
	}

	// Write empty config
	emptyConfig := "# WSO2 API Platform CLI Configuration\ngateways: []\n"
	if err := os.WriteFile(configPath, []byte(emptyConfig), 0600); err != nil {
		return err
	}

	return nil
}

// restoreUserConfig restores the backed up config if present and removes the backup.
func restoreUserConfig() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	configPath := filepath.Join(home, ".wso2ap", "config.yaml")
	backupPath := configPath + ".backup"

	if _, err := os.Stat(backupPath); err == nil {
		// Restore from backup
		data, err := os.ReadFile(backupPath)
		if err != nil {
			return err
		}
		if err := os.WriteFile(configPath, data, 0600); err != nil {
			return err
		}
		// Remove backup
		if err := os.Remove(backupPath); err != nil {
			return err
		}
	} else {
		// No backup - remove the test config file we wrote
		_ = os.Remove(configPath)
	}
	return nil
}

// InitializeScenario sets up each test scenario
func InitializeScenario(ctx *godog.ScenarioContext) {
	// Initialize test state for each scenario
	ctx.Before(func(c context.Context, sc *godog.Scenario) (context.Context, error) {
		testState = NewTestState()
		if err := testState.Reset(); err != nil {
			return c, err
		}

		// Set CLI binary path
		if infraManager != nil {
			testState.SetCLIBinaryPath(infraManager.GetCLIBinaryPath())
		}

		// Set CLI coverage directory for coverage collection
		if cliCoverDir != "" {
			testState.SetCLICoverDir(cliCoverDir)
		}

		// Initialize step handlers
		cliSteps = steps.NewCLISteps(testState)
		assertSteps = steps.NewAssertSteps(testState)

		// Extract test ID from tags if available
		for _, tag := range sc.Tags {
			if len(tag.Name) > 1 && tag.Name[0] == '@' {
				testState.SetTestInfo(tag.Name[1:], sc.Name)
				testReporter.StartTest(tag.Name[1:], sc.Name)
				break
			}
		}

		return c, nil
	})

	ctx.After(func(c context.Context, sc *godog.Scenario, err error) (context.Context, error) {
		passed := err == nil
		errorMsg := ""
		if err != nil {
			errorMsg = err.Error()
		}

		testReporter.EndTest(testState, passed, errorMsg)

		// Log the result
		if testState.TestID != "" {
			testReporter.LogTest(testState.TestID, testState.TestName, passed,
				testReporter.generateLogFileName(testState.TestID, testState.TestName))
		}

		testState.Cleanup()
		return c, nil
	})

	// Register step definitions
	registerInfrastructureSteps(ctx)
	registerCLISteps(ctx)
	registerAssertSteps(ctx)
	registerGatewaySteps(ctx)
	registerAPISteps(ctx)
	registerMCPSteps(ctx)
	registerBuildSteps(ctx)
}

// registerInfrastructureSteps registers infrastructure-related step definitions
func registerInfrastructureSteps(ctx *godog.ScenarioContext) {
	ctx.Step(`^the CLI is available$`, theCliIsAvailable)
	ctx.Step(`^the gateway is running$`, theGatewayIsRunning)
	ctx.Step(`^the MCP server is running$`, theMCPServerIsRunning)
	ctx.Step(`^Docker is available$`, dockerIsAvailable)
}

// registerCLISteps registers CLI execution step definitions
func registerCLISteps(ctx *godog.ScenarioContext) {
	ctx.Step(`^I run "([^"]*)"$`, iRunCommand)
	ctx.Step(`^I run ap with arguments "([^"]*)"$`, iRunApWithArguments)
	ctx.Step(`^I run ap gateway add with name "([^"]*)" and server "([^"]*)"$`, iRunGatewayAdd)
	ctx.Step(`^I run ap gateway add with name "([^"]*)" and server "([^"]*)" and auth "([^"]*)"$`, iRunGatewayAddWithAuth)
}

// registerAssertSteps registers assertion step definitions
func registerAssertSteps(ctx *godog.ScenarioContext) {
	ctx.Step(`^the exit code should be (\d+)$`, theExitCodeShouldBe)
	ctx.Step(`^the output should contain "([^"]*)"$`, theOutputShouldContain)
	ctx.Step(`^the output should not contain "([^"]*)"$`, theOutputShouldNotContain)
	ctx.Step(`^the stderr should contain "([^"]*)"$`, theStderrShouldContain)
	ctx.Step(`^the stdout should contain "([^"]*)"$`, theStdoutShouldContain)
	ctx.Step(`^the command should succeed$`, theCommandShouldSucceed)
	ctx.Step(`^the command should fail$`, theCommandShouldFail)
}

// registerGatewaySteps registers gateway management step definitions
func registerGatewaySteps(ctx *godog.ScenarioContext) {
	ctx.Step(`^I have a gateway named "([^"]*)" configured$`, iHaveGatewayConfigured)
	ctx.Step(`^I have a gateway named "([^"]*)" with server "([^"]*)"$`, iHaveGatewayWithServer)
	ctx.Step(`^the gateway "([^"]*)" should exist$`, theGatewayShouldExist)
	ctx.Step(`^the gateway "([^"]*)" should not exist$`, theGatewayShouldNotExist)
	ctx.Step(`^I set the current gateway to "([^"]*)"$`, iSetCurrentGateway)
	ctx.Step(`^the current gateway should be "([^"]*)"$`, theCurrentGatewayShouldBe)
	ctx.Step(`^no gateway is configured$`, noGatewayIsConfigured)
	ctx.Step(`^I reset the CLI configuration$`, iResetCLIConfiguration)
}

// registerAPISteps registers API management step definitions
func registerAPISteps(ctx *godog.ScenarioContext) {
	ctx.Step(`^I apply the sample API$`, iApplyTheSampleAPI)
	ctx.Step(`^I apply the resource file "([^"]*)"$`, iApplyResourceFile)
	ctx.Step(`^the API "([^"]*)" should be deployed$`, theAPIShouldBeDeployed)
	ctx.Step(`^the API "([^"]*)" should not exist$`, theAPIShouldNotExist)
	ctx.Step(`^I delete the API "([^"]*)"$`, iDeleteTheAPI)
}

// registerMCPSteps registers MCP management step definitions
func registerMCPSteps(ctx *godog.ScenarioContext) {
	ctx.Step(`^I generate MCP config from server "([^"]*)"$`, iGenerateMCPConfig)
	ctx.Step(`^I generate MCP config to output "([^"]*)"$`, iGenerateMCPConfigToOutput)
	ctx.Step(`^the MCP config should be generated$`, theMCPConfigShouldBeGenerated)
	ctx.Step(`^the MCP "([^"]*)" should be deployed$`, theMCPShouldBeDeployed)
	ctx.Step(`^the MCP "([^"]*)" should not exist$`, theMCPShouldNotExist)
}

// registerBuildSteps registers build step definitions
func registerBuildSteps(ctx *godog.ScenarioContext) {
	ctx.Step(`^I build gateway with manifest "([^"]*)"$`, iBuildGatewayWithManifest)
	ctx.Step(`^the build should complete successfully$`, theBuildShouldComplete)
}

// Step implementations

func theCliIsAvailable() error {
	if testState.CLIBinaryPath == "" {
		return fmt.Errorf("CLI binary path not set")
	}
	if _, err := os.Stat(testState.CLIBinaryPath); os.IsNotExist(err) {
		return fmt.Errorf("CLI binary not found at %s", testState.CLIBinaryPath)
	}
	return nil
}

func theGatewayIsRunning() error {
	return infraManager.waitForGatewayHealth()
}

func theMCPServerIsRunning() error {
	return infraManager.waitForMCPServer()
}

func dockerIsAvailable() error {
	return CheckDockerAvailable()
}

func iRunCommand(command string) error {
	return cliSteps.RunCommand(command)
}

func iRunApWithArguments(args string) error {
	return cliSteps.RunWithArgs(args)
}

func iRunGatewayAdd(name, server string) error {
	return cliSteps.RunGatewayAdd(name, server, "none")
}

func iRunGatewayAddWithAuth(name, server, auth string) error {
	return cliSteps.RunGatewayAdd(name, server, auth)
}

func theExitCodeShouldBe(expected int) error {
	return assertSteps.ExitCodeShouldBe(expected)
}

func theOutputShouldContain(text string) error {
	return assertSteps.OutputShouldContain(text)
}

func theOutputShouldNotContain(text string) error {
	return assertSteps.OutputShouldNotContain(text)
}

func theStderrShouldContain(text string) error {
	return assertSteps.StderrShouldContain(text)
}

func theStdoutShouldContain(text string) error {
	return assertSteps.StdoutShouldContain(text)
}

func theCommandShouldSucceed() error {
	return assertSteps.CommandShouldSucceed()
}

func theCommandShouldFail() error {
	return assertSteps.CommandShouldFail()
}

func iHaveGatewayConfigured(name string) error {
	return cliSteps.EnsureGatewayExists(name)
}

func theGatewayShouldExist(name string) error {
	return assertSteps.GatewayShouldExist(name)
}

func theGatewayShouldNotExist(name string) error {
	return assertSteps.GatewayShouldNotExist(name)
}

func iSetCurrentGateway(name string) error {
	return cliSteps.SetCurrentGateway(name)
}

func theCurrentGatewayShouldBe(name string) error {
	return assertSteps.CurrentGatewayShouldBe(name)
}

func noGatewayIsConfigured() error {
	// This is handled by the isolated config directory
	return nil
}

func iApplyTheSampleAPI() error {
	return cliSteps.ApplySampleAPI()
}

func theAPIShouldBeDeployed(name string) error {
	return assertSteps.APIShouldBeDeployed(name)
}

func theAPIShouldNotExist(name string) error {
	return assertSteps.APIShouldNotExist(name)
}

func iDeleteTheAPI(name string) error {
	return cliSteps.DeleteAPI(name)
}

func iGenerateMCPConfig(server string) error {
	return cliSteps.GenerateMCPConfig(server, "")
}

func iGenerateMCPConfigToOutput(output string) error {
	return cliSteps.GenerateMCPConfig("http://localhost:3001/mcp", output)
}

func theMCPConfigShouldBeGenerated() error {
	return assertSteps.MCPConfigShouldBeGenerated()
}

func theMCPShouldBeDeployed(name string) error {
	return assertSteps.MCPShouldBeDeployed(name)
}

func theMCPShouldNotExist(name string) error {
	return assertSteps.MCPShouldNotExist(name)
}

func iBuildGatewayWithManifest(manifest string) error {
	return cliSteps.BuildGatewayWithManifest(manifest)
}

func theBuildShouldComplete() error {
	return assertSteps.BuildShouldComplete()
}

func iHaveGatewayWithServer(name, server string) error {
	return cliSteps.EnsureGatewayExistsWithServer(name, server)
}

func iResetCLIConfiguration() error {
	return cliSteps.ResetConfiguration()
}

func iApplyResourceFile(filePath string) error {
	return cliSteps.ApplyResourceFile(filePath)
}
