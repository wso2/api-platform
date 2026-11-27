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

package image

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/wso2/api-platform/cli/internal/gateway"
	"github.com/wso2/api-platform/cli/internal/policy"
	"github.com/wso2/api-platform/cli/utils"
)

const (
	BuildCmdLiteral = "build"
	BuildCmdExample = `# Build gateway image with policies (uses current directory)
ap gateway image build

# Build with custom name
ap gateway image build --name my-gateway

# Build with custom path containing build files
ap gateway image build --name my-gateway --path ./my-policies --repository myregistry

# Build with platform specification
ap gateway image build --name my-gateway --platform linux/amd64

# Build with policies from a private GitHub repository, reading the token from POLICY_TOKEN
export GOPRIVATE='github.com/acme/*'
ap gateway image build --git-credential github.com=<github-username>:POLICY_TOKEN

# Build with a netrc file and a private module proxy (GONOPROXY=none sends GOPRIVATE modules through the proxy too)
ap gateway image build --netrc ~/.netrc-gateway-policies --go-env GOPROXY=https://goproxy.acme.com,direct --go-env GONOPROXY=none`
)

var (
	// Optional flags
	gatewayName              string
	gatewayVersion           string
	buildFilePath            string
	imageRepository          string
	gatewayBuilder           string
	gatewayControllerBaseImg string
	gatewayRuntimeBaseImg    string
	push                     bool
	noCache                  bool
	platform                 string
	outputDir                string
	fetchTimeout             time.Duration
	netrcFile                string
	gitCredentials           []string
	goEnvFlags               []string

	// Computed values
	imageTag      string
	builderAccess gateway.BuilderAccess
)

var buildCmd = &cobra.Command{
	Use:     BuildCmdLiteral,
	Short:   "Build gateway Docker image with policies",
	Long:    "Build a WSO2 API Platform Gateway Docker image with specified policies from build file.",
	Example: BuildCmdExample,
	Run: func(cmd *cobra.Command, args []string) {
		err := checkFetchTimeout(cmd.Flags().Changed("fetch-timeout"), fetchTimeout)
		if err == nil {
			err = runBuildCommand()
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	// Optional flags with defaults
	buildCmd.Flags().StringVar(&gatewayName, "name", "", "Gateway name (defaults to directory name)")
	buildCmd.Flags().StringVarP(&buildFilePath, "path", "p", ".", "Path to directory containing build files (default: current directory)")
	buildCmd.Flags().StringVar(&imageRepository, "repository", utils.DefaultImageRepository, "Docker image repository")
	buildCmd.Flags().BoolVar(&push, "push", false, "Push image to registry after build")
	buildCmd.Flags().BoolVar(&noCache, "no-cache", false, "Build without using cache")
	buildCmd.Flags().StringVar(&platform, "platform", "", "Target platform (e.g., linux/amd64)")
	buildCmd.Flags().StringVar(&outputDir, "output-dir", "", "Output directory for build artifacts")

	// Access to Go module policies in private repositories. StringArray, not
	// StringSlice: GOPRIVATE-style values contain commas.
	buildCmd.Flags().StringVar(&netrcFile, "netrc", "",
		"netrc file with credentials for private Go module hosts and proxies, mounted read-only into the gateway-builder container")
	buildCmd.Flags().StringArrayVar(&gitCredentials, "git-credential", nil,
		"Credentials for a private Go module host as <host>=<username>:<ENV_VAR>, where ENV_VAR names an environment variable holding the token (repeatable)")
	buildCmd.Flags().StringArrayVar(&goEnvFlags, "go-env", nil,
		"Go module setting for the gateway-builder container as KEY=VALUE, or KEY to copy it from your environment (repeatable). Supported: "+
			strings.Join(gateway.SupportedGoEnvKeys(), ", ")+". GOPRIVATE, GONOPROXY and GONOSUMDB are copied from your environment automatically")
	buildCmd.MarkFlagsMutuallyExclusive("netrc", "git-credential")

	// Zero means "not set": the builder then uses its own default (5m).
	buildCmd.Flags().DurationVar(&fetchTimeout, "fetch-timeout", 0,
		"How long to wait for each gomodule policy to download, for example 10m (default 5m)")
}

// resolveBuilderAccess validates the private-module flags before any build work
// starts. Tokens are read from the environment variables the flags name.
func resolveBuilderAccess() (gateway.BuilderAccess, error) {
	var access gateway.BuilderAccess
	var err error

	if access.GoEnv, err = gateway.ResolveGoEnv(goEnvFlags, os.LookupEnv); err != nil {
		return access, err
	}
	if netrcFile != "" {
		if access.NetrcFile, err = gateway.ValidateNetrcFile(netrcFile); err != nil {
			return access, err
		}
	}
	if access.GitCredentials, err = gateway.ParseGitCredentials(gitCredentials, os.LookupEnv); err != nil {
		return access, err
	}
	return access, nil
}

// checkFetchTimeout rejects an explicit --fetch-timeout that is not positive.
func checkFetchTimeout(set bool, d time.Duration) error {
	if set && d <= 0 {
		return fmt.Errorf("--fetch-timeout must be greater than zero, for example 10m")
	}
	return nil
}

// printBuilderAccess shows which Go settings and credentials reach the builder,
// followed by warnings for setups that are easy to get wrong.
func printBuilderAccess(access gateway.BuilderAccess) {
	if len(access.GoEnv) > 0 {
		fmt.Println("  Go module settings for the builder:")
		for _, s := range access.GoEnv {
			fmt.Printf("    • %-10s %s (%s)\n", s.Key, s.DisplayValue(), s.Source)
		}
	}
	if access.HasCredentials() {
		fmt.Println("  Builder credentials:")
		if access.NetrcFile != "" {
			fmt.Printf("    • netrc file %s (mounted read-only)\n", access.NetrcFile)
		}
		for _, c := range access.GitCredentials {
			fmt.Printf("    • %s as %s (from --git-credential)\n", c.Host, c.Username)
		}
	}
	defaultBuilder := fmt.Sprintf(utils.DefaultGatewayBuilder, gatewayVersion)
	for _, line := range builderAccessWarnings(access, gatewayBuilder, defaultBuilder, gatewayVersion) {
		fmt.Println(line)
	}
	if len(access.GoEnv) > 0 || access.HasCredentials() {
		fmt.Println()
	}
}

// builderAccessWarnings returns the warning lines printed under the builder settings.
func builderAccessWarnings(access gateway.BuilderAccess, builder, defaultBuilder, version string) []string {
	var lines []string
	if access.GoEnvValue("GOSUMDB") == "off" {
		lines = append(lines,
			"  ! GOSUMDB=off turns off checksum verification for every module, including the public policies.",
			"    To skip it only for private modules, set GOPRIVATE instead.",
			"  ! GOSUMDB=off also makes Go toolchain downloads fail (Go never runs an unverified toolchain), so a",
			"    policy that needs a newer Go version than the builder's cannot be built.")
	}
	if !access.HasCredentials() {
		return lines
	}
	if builder != defaultBuilder {
		lines = append(lines, fmt.Sprintf("  ! Credentials will be passed to builder image %s, which is not the default for gateway %s (set in %s)",
			builder, version, utils.DefaultBuildFile))
	}
	// The go command sends credentials only over HTTPS; git sends them wherever it is asked.
	if access.HasGoEnv("GOINSECURE") {
		lines = append(lines,
			"  ! GOINSECURE allows plain-HTTP downloads for matching modules, and git can then send the credentials",
			"    for those hosts unencrypted. Use HTTPS for every host you pass credentials for.")
	}
	if !access.HasGoEnv("GOPRIVATE") && !access.HasGoEnv("GONOSUMDB") && access.GoEnvValue("GOSUMDB") != "off" {
		lines = append(lines,
			"  ! GOPRIVATE is not set: private modules will fail verification against the public checksum database.",
			"    Set GOPRIVATE to a pattern matching your module paths, for example: export GOPRIVATE='github.com/acme/*'")
	}
	return lines
}

// initializeDefaults sets smart defaults for gateway name and constructs the image tag
func initializeDefaults(buildFile *policy.BuildFile) error {
	// Default gateway name from directory name if not provided
	if gatewayName == "" {
		absPath, err := filepath.Abs(buildFilePath)
		if err != nil {
			return fmt.Errorf("failed to get absolute path: %w", err)
		}
		gatewayName = filepath.Base(absPath)
	}

	// Default gateway version from build file if not provided via flag
	if buildFile.Gateway.Version == "" {
		return fmt.Errorf("gateway version is required: set gateway.version in build.yaml")
	}
	gatewayVersion = buildFile.Gateway.Version

	// Use custom images from build file if provided, otherwise construct from defaults
	if buildFile.Gateway.Images.Builder != "" {
		gatewayBuilder = buildFile.Gateway.Images.Builder
	} else {
		gatewayBuilder = fmt.Sprintf(utils.DefaultGatewayBuilder, gatewayVersion)
	}

	if buildFile.Gateway.Images.Controller != "" {
		gatewayControllerBaseImg = buildFile.Gateway.Images.Controller
	} else {
		gatewayControllerBaseImg = fmt.Sprintf(utils.DefaultGatewayController, gatewayVersion)
	}

	if buildFile.Gateway.Images.Runtime != "" {
		gatewayRuntimeBaseImg = buildFile.Gateway.Images.Runtime
	} else {
		gatewayRuntimeBaseImg = fmt.Sprintf(utils.DefaultGatewayRuntime, gatewayVersion)
	}

	// Construct the full image tag: repository/name:version
	imageTag = fmt.Sprintf("%s/%s:%s", imageRepository, gatewayName, gatewayVersion)

	return nil
}

func runBuildCommand() error {
	fmt.Println("=== Gateway Image Build ===")
	fmt.Println()

	var err error
	if builderAccess, err = resolveBuilderAccess(); err != nil {
		return err
	}

	// Step 1: Check Docker availability
	fmt.Println("[1/8] Checking Docker Availability")
	if err := utils.IsDockerAvailable(); err != nil {
		return fmt.Errorf("Docker is not available: %w", err)
	}
	fmt.Println("  ✓ Docker is available")

	// Check docker buildx if platform is specified
	if platform != "" {
		if err := utils.IsDockerBuildxAvailable(); err != nil {
			return fmt.Errorf("docker buildx is not available but required for --platform flag: %w\n\nPlease install docker buildx or remove the --platform flag", err)
		}
		fmt.Println("  ✓ Docker buildx is available")
	}

	fmt.Println()
	return runUnifiedBuild()
}

// getBuildFilePath returns the full path to the build file
func getBuildFilePath(basePath string) (string, error) {
	// Check if path exists
	info, err := os.Stat(basePath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("path does not exist: %s\n\nPlease provide a valid directory path using --path flag", basePath)
		}
		return "", fmt.Errorf("failed to access path: %w", err)
	}

	// If it's a directory, look for the build file
	if info.IsDir() {
		buildFile := filepath.Join(basePath, utils.DefaultBuildFile)
		if _, err := os.Stat(buildFile); err != nil {
			if os.IsNotExist(err) {
				return "", fmt.Errorf("build file '%s' not found in directory: %s\n\nExpected file: %s\n\nPlease create a %s file or specify a different path", utils.DefaultBuildFile, basePath, buildFile, utils.DefaultBuildFile)
			}
			return "", fmt.Errorf("failed to access build file: %w", err)
		}
		return buildFile, nil
	}

	// If it's a file, that's an error - we expect a directory
	return "", fmt.Errorf("--path must be a directory, not a file: %s\n\nPlease provide the directory path containing %s", basePath, utils.DefaultBuildFile)
}

func runUnifiedBuild() error {
	// Step 2: Read Build File
	fmt.Println("[2/6] Reading Build File")

	// Get build file path
	resolvedBuildFilePath, err := getBuildFilePath(buildFilePath)
	if err != nil {
		return err
	}

	buildFile, err := policy.ParseBuildFile(resolvedBuildFilePath)
	if err != nil {
		return fmt.Errorf("failed to parse build file at '%s': %w", resolvedBuildFilePath, err)
	}
	fmt.Printf("  ✓ Loaded build file with %s\n\n", policy.CountLabel(len(buildFile.Policies), ""))

	// Initialize computed values
	if err := initializeDefaults(buildFile); err != nil {
		return err
	}

	// Display resolved images used for the build
	fmt.Println("  Resolved images:")
	fmt.Printf("    • Builder:            %s\n", gatewayBuilder)
	fmt.Printf("    • Gateway Controller: %s\n", gatewayControllerBaseImg)
	fmt.Printf("    • Gateway Runtime:    %s\n\n", gatewayRuntimeBaseImg)
	if fetchTimeout > 0 {
		fmt.Printf("  Fetch timeout per gomodule policy: %s (--fetch-timeout)\n\n", fetchTimeout)
	}
	printBuilderAccess(builderAccess)

	// Step 3: Validate Build File and Separate Policies
	fmt.Println("[3/6] Validating build file")
	localPolicies, hubPolicies := policy.SeparatePolicies(buildFile)
	fmt.Printf("  → Hub policies: %d\n", len(hubPolicies))
	fmt.Printf("  → Local policies: %d\n\n", len(localPolicies))

	// Step 4: Process Local Policies
	fmt.Println("[4/6] Processing Local Policies")
	var processed []policy.ProcessedPolicy
	if len(localPolicies) > 0 {
		processed, err = policy.ProcessLocalPolicies(localPolicies)
		if err != nil {
			return fmt.Errorf("failed to process local policies: %w", err)
		}
		fmt.Printf("  ✓ Processed %s\n\n", policy.CountLabel(len(processed), "local"))
	}

	// Step 5: Prepare workspace and copy build file + policies
	fmt.Println()
	fmt.Println("[5/6] Preparing workspace and copying policies")
	tempDir, err := utils.SetupTempGatewayWorkspace(resolvedBuildFilePath)
	if err != nil {
		return fmt.Errorf("failed to setup workspace: %w", err)
	}
	defer os.RemoveAll(tempDir)

	fmt.Printf("  ✓ Workspace ready: %s\n\n", tempDir)

	// Step 6: Run Docker build (gateway-builder + image build)
	fmt.Println("[6/6] Building Gateway Images")
	if err := runDockerBuild(tempDir); err != nil {
		return fmt.Errorf("failed to build gateway images: %w", err)
	}
	fmt.Println("  ✓ All images built successfully")

	// Copy build-manifest.yaml back to user's directory
	lockSrc := filepath.Join(tempDir, "build-manifest.yaml")
	lockDst := filepath.Join(filepath.Dir(resolvedBuildFilePath), "build-manifest.yaml")
	if lockData, err := os.ReadFile(lockSrc); err != nil {
		fmt.Fprintf(os.Stderr, "  Warning: could not read build manifest file: %v\n", err)
	} else if err := os.WriteFile(lockDst, lockData, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "  Warning: could not write build manifest file: %v\n", err)
	} else {
		fmt.Printf("  ✓ Build manifest file written: build-manifest.yaml\n")
	}

	// Display Summary. Count every build file policy, local and gomodule alike: the
	// build fails if any of them cannot be resolved, so all of them are in the images.
	displayBuildSummary(len(buildFile.Policies))

	return nil
}

func displayBuildSummary(policyCount int) {
	fmt.Println("=== Build Summary ===")
	fmt.Println()

	// Images built
	fmt.Printf("✓ Built gateway images with %s:\n", policy.CountLabel(policyCount, ""))
	fmt.Printf("  • %s/%s-gateway-runtime:%s\n", imageRepository, gatewayName, gatewayVersion)
	fmt.Printf("  • %s/%s-gateway-controller:%s\n", imageRepository, gatewayName, gatewayVersion)
	fmt.Println()

	// Where images are
	if push {
		fmt.Printf("✓ Images pushed to registry: %s\n", imageRepository)
		if platform != "" {
			fmt.Printf("  Platform: %s\n", platform)
		}
	} else {
		fmt.Println("✓ Images available in local Docker")
		if platform != "" {
			fmt.Printf("  Platform: %s\n", platform)
		}
	}
	fmt.Println()

	if outputDir != "" {
		fmt.Printf("✓ Output artifacts copied to: %s\n", outputDir)
		fmt.Println()
	}
}

// runDockerBuild executes the docker build process for gateway images
func runDockerBuild(tempDir string) error {
	logFilePath, err := getDockerBuildLogPath(tempDir)
	if err != nil {
		return err
	}

	// Prepare build configuration
	config := gateway.DockerBuildConfig{
		TempDir:                    tempDir,
		GatewayBuilder:             gatewayBuilder,
		GatewayControllerBaseImage: gatewayControllerBaseImg,
		GatewayRuntimeBaseImage:    gatewayRuntimeBaseImg,
		ImageRepository:            imageRepository,
		GatewayName:                gatewayName,
		GatewayVersion:             gatewayVersion,
		Platform:                   platform,
		NoCache:                    noCache,
		Push:                       push,
		LogFilePath:                logFilePath,
		OutputCopyDir:              outputDir,
		BuilderAccess:              builderAccess,
		FetchTimeout:               fetchTimeout,
	}

	// Run the build
	if err := gateway.BuildGatewayImages(config); err != nil {
		return err
	}

	return nil
}

func getDockerBuildLogPath(tempDir string) (string, error) {
	logsDir := filepath.Join(filepath.Dir(tempDir), "logs")
	if err := utils.EnsureDir(logsDir); err != nil {
		return "", fmt.Errorf("failed to create logs directory: %w", err)
	}

	return filepath.Join(logsDir, filepath.Base(tempDir)+"-docker.log"), nil
}
