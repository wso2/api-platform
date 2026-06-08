package apiproject

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
	"github.com/wso2/api-platform/cli/utils"
)

const (
	InitCmdLiteral = "init"
	InitCmdExample = `# Initialize a new API project
ap apiproject init --display-name foo-api --type rest --version 1.0 --context /foo

# Add a API project fully interactively cobra
ap apiproject init`
)

var displayName string
var apiType string
var apiVersion string
var apiContext string
var addNoInteractive bool

var initCmd = &cobra.Command{
	Use:     InitCmdLiteral,
	Short:   "Initialize a new API project",
	Long:    "Initialize a new API project with the specified parameters.",
	Example: InitCmdExample,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runInitCommand(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	utils.AddStringFlag(initCmd, utils.FlagName, &displayName, "", "Display name of the API")
	utils.AddStringFlag(initCmd, utils.FlagType, &apiType, "", "Type of the API")
	utils.AddStringFlag(initCmd, utils.FlagVersion, &apiVersion, "", "Version of the API")
	utils.AddStringFlag(initCmd, utils.FlagContext, &apiContext, "", "Context of the API")
	utils.AddBoolFlag(initCmd, utils.FlagNoInteractive, &addNoInteractive, false, "Skip interactive prompts")
}

func runInitCommand() error {
	var err error
	if !addNoInteractive {
		if strings.TrimSpace(displayName) == "" {
			displayName, err = utils.PromptInput("Enter API Project name: ")
			if err != nil {
				return fmt.Errorf("Failed to read display name: %w", err)
			}
		}
		if strings.TrimSpace(apiType) == "" {
			apiType, err = utils.PromptInput("Enter API type (e.g., rest): ")
			if err != nil {
				return fmt.Errorf("Failed to read API type: %w", err)
			}
		}
		if strings.TrimSpace(apiVersion) == "" {
			apiVersion, err = utils.PromptInput("Enter API version: ")
			if err != nil {
				return fmt.Errorf("Failed to read API version: %w", err)
			}
		}
		if strings.TrimSpace(apiContext) == "" {
			apiContext, err = utils.PromptInput("Enter API context (e.g., /foo): ")
			if err != nil {
				return fmt.Errorf("Failed to read API context: %w", err)
			}
		}
	}

	displayName = strings.TrimSpace(displayName)
	apiType = strings.ToLower(strings.TrimSpace(apiType))
	apiVersion = strings.TrimSpace(apiVersion)
	apiContext = strings.TrimSpace(apiContext)

	if displayName == "" {
		return fmt.Errorf("display name is required")
	}
	if apiType == "" {
		return fmt.Errorf("API type is required")
	}
	if apiVersion == "" {
		return fmt.Errorf("API version is required")
	}
	if apiContext == "" {
		return fmt.Errorf("API context is required")
	}

	if apiType != utils.APITypeREST {
		return fmt.Errorf("unsupported API type: %s", apiType)
	}

	if err := buildDirectoryStructure(displayName, apiType, apiVersion, apiContext); err != nil {
		return err
	}

	fmt.Printf("API project created at .%c%s\n", os.PathSeparator, displayName)
	return nil
}

func buildDirectoryStructure(name, apiType, version, context string) error {
	if apiType != utils.APITypeREST {
		return fmt.Errorf("API project scaffolding currently supports only %s APIs", utils.APITypeREST)
	}

	projectDirName, err := validateProjectDirectoryName(name)
	if err != nil {
		return err
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to determine current working directory: %w", err)
	}

	projectRoot := filepath.Join(cwd, projectDirName)
	if _, err := os.Stat(projectRoot); err == nil {
		return fmt.Errorf("project directory already exists: %s", projectRoot)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("failed to inspect project directory: %w", err)
	}

	directories := []string{
		filepath.Join(projectRoot, ".api-platform"),
		filepath.Join(projectRoot, "docs"),
		filepath.Join(projectRoot, "tests"),
	}
	for _, dir := range directories {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	resourceName := buildResourceName(name, version)
	files := map[string]string{
		filepath.Join(projectRoot, ".api-platform", "config.yaml"): buildConfigYAML(),
		filepath.Join(projectRoot, "api.yaml"):                     buildAPIYAML(resourceName),
		filepath.Join(projectRoot, "gateway.yaml"):                 buildGatewayYAML(resourceName, name, version, context),
		filepath.Join(projectRoot, "definition.yaml"):              buildDefinitionYAML(name, version, context),
	}
	for path, content := range files {
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to write %s: %w", path, err)
		}
	}

	return nil
}

func validateProjectDirectoryName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("display name is required")
	}

	if name == "." || name == ".." {
		return "", fmt.Errorf("display name cannot be %q", name)
	}

	if strings.ContainsRune(name, os.PathSeparator) {
		return "", fmt.Errorf("display name cannot contain path separators")
	}

	if os.PathSeparator != '/' && strings.ContainsRune(name, '/') {
		return "", fmt.Errorf("display name cannot contain path separators")
	}

	return name, nil
}

func buildResourceName(name, version string) string {
	normalized := strings.ToLower(strings.TrimSpace(name))
	normalized = strings.ReplaceAll(normalized, "_", "-")
	normalized = strings.ReplaceAll(normalized, " ", "-")

	invalidChars := regexp.MustCompile(`[^a-z0-9.-]+`)
	repeatedHyphens := regexp.MustCompile(`-+`)

	normalized = invalidChars.ReplaceAllString(normalized, "-")
	normalized = repeatedHyphens.ReplaceAllString(normalized, "-")
	normalized = strings.Trim(normalized, "-.")

	if normalized == "" {
		normalized = "api"
	}

	return fmt.Sprintf("%s-%s", normalized, version)
}

func buildConfigYAML() string {
	return `version: 1.0.0

# Default file paths (can be customized)
filePaths:
  deploymentArtifact: ./gateway.yaml
  apiMetadata: ./api.yaml
  apiDefinition: ./definition.yaml
  docs: ./docs
  tests: ./tests

# Governance rulesets for design-time validation
governanceRulesets: []

# Auto-sync configuration for vscode plugin
autoSync:
  gatewayArtifactFromDefinition: true  # Auto-generate gateway.yaml when definition.yaml changes
`
}

func buildAPIYAML(resourceName string) string {
	return fmt.Sprintf(`apiVersion: management.api-platform.wso2.com/v1
kind: Api
metadata:
  name: %q
spec:
  description: ""
  gatewayType: wso2/api-platform
  status: PUBLISHED
  referenceID: ""
  tags: []
  labels: []
  businessInformation:
    businessOwner: ""
    businessOwnerEmail: ""
    technicalOwner: ""
    technicalOwnerEmail: ""
  endpoints:
    sandboxUrl: ""
    productionUrl: ""
`, resourceName)
}

func buildGatewayYAML(resourceName, displayName, version, context string) string {
	return fmt.Sprintf(`apiVersion: gateway.api-platform.wso2.com/v1
kind: RestApi
metadata:
  name: %q
spec:
  displayName: %q
  version: %q
  context: %q
  upstream:
    main:
      url: "http://sample-backend.org:9080"           # Change this to your backend URL
  operations:
    - path: /*
      method: GET
    - path: /*
      method: POST
    - path: /*
      method: PUT
    - path: /*
      method: DELETE
	- path: /*
	  method: OPTIONS
`, resourceName, displayName, version, context)
}

func buildDefinitionYAML(displayName, version, context string) string {
	return fmt.Sprintf(`openapi: 3.0.3
info:
  title: %q
  version: %q
servers:
  - url: %q
paths:
  "/*":
    get:
      responses:
        "200":
          description: OK
    post:
      responses:
        "200":
          description: OK
    put:
      responses:
        "200":
          description: OK
    delete:
      responses:
        "200":
          description: OK
    options:
      responses:
        "200":
          description: OK
`, displayName, version, context)
}
