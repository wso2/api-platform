package devportal

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/wso2/api-platform/cli/utils"
	"gopkg.in/yaml.v3"
)

const (
	BuildCmdLiteral = "build"
	BuildCmdExample = `# Build the api project for devportal
ap devportal build           #Build the project in current directory

# Build the project in a specified directory
ap devportal build -f /path/to/project`
)

var (
	buildProjectDir string
)

var buildCmd = &cobra.Command{
	Use:     BuildCmdLiteral,
	Short:   "Build the API project for devportal",
	Long:    "Build the API project located in the specified directory (or current directory if not specified) for deployment to the devportal.",
	Example: BuildCmdExample,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runBuildCommand(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	utils.AddStringFlag(buildCmd, utils.FlagFile, &buildProjectDir, "", "Path to the API project directory (defaults to current directory)")
}

func runBuildCommand() error {
	if buildProjectDir == "" {
		buildProjectDir = "."
	}

	projectRoot, err := filepath.Abs(buildProjectDir)
	if err != nil {
		return fmt.Errorf("failed to resolve project directory: %w", err)
	}

	projectConfigDir := filepath.Join(projectRoot, ".api-platform")
	if _, err := os.Stat(projectConfigDir); os.IsNotExist(err) {
		return fmt.Errorf("unable to find api project directory, please execute this command inside api project")
	} else if err != nil {
		return fmt.Errorf("failed to inspect api project directory: %w", err)
	}

	projectConfigPath := filepath.Join(projectConfigDir, "config.yaml")
	if _, err := os.Stat(projectConfigPath); os.IsNotExist(err) {
		return fmt.Errorf("unable to find api project directory, please execute this command inside api project")
	} else if err != nil {
		return fmt.Errorf("failed to inspect project config: %w", err)
	}

	projectConfig, err := loadProjectConfig(projectConfigPath)
	if err != nil {
		return err
	}

	isDevportalConfigExists := projectConfig.isDevportalConfigExists()
	// create default devportal config if not exists and add to project config
	if !isDevportalConfigExists {
		portalConfig, err := projectConfig.createDefaultDevPortalConfig(projectRoot)
		if err != nil {
			return err
		}
		projectConfig.DevPortals = append(projectConfig.DevPortals, *portalConfig)
		if err := saveProjectConfig(projectConfigPath, projectConfig); err != nil {
			return err
		}
	}

	buildDir := filepath.Join(projectRoot, "build")
	if err := os.RemoveAll(buildDir); err != nil {
		return fmt.Errorf("failed to clean build directory: %w", err)
	}
	if err := os.MkdirAll(buildDir, 0755); err != nil {
		return fmt.Errorf("failed to recreate build directory: %w", err)
	}

	zipPaths, err := buildDevPortalArchives(projectRoot, buildDir, projectConfig.DevPortals)
	if err != nil {
		return err
	}

	for _, zipPath := range zipPaths {
		fmt.Printf("DevPortal build created at %s\n", zipPath)
	}
	return nil
}

type apiProjectConfig struct {
	Version            string                   `yaml:"version,omitempty"`
	FilePaths          apiProjectFilePaths      `yaml:"filePaths,omitempty"`
	GovernanceRulesets []string                 `yaml:"governanceRulesets,omitempty"`
	AutoSync           map[string]interface{}   `yaml:"autoSync,omitempty"`
	DevPortals         []devPortalProjectConfig `yaml:"devportals,omitempty"`
}

type apiProjectFilePaths struct {
	DeploymentArtifact string `yaml:"deploymentArtifact,omitempty"`
	APIMetadata        string `yaml:"apiMetadata,omitempty"`
	APIDefinition      string `yaml:"apiDefinition,omitempty"`
	Docs               string `yaml:"docs,omitempty"`
	Tests              string `yaml:"tests,omitempty"`
}

type devPortalProjectConfig struct {
	Name       string                `yaml:"name,omitempty"`
	PortalRoot string                `yaml:"portalRoot,omitempty"`
	FilePaths  devPortalProjectPaths `yaml:"filePaths,omitempty"`
}

type devPortalProjectPaths struct {
	APIMetadata   string `yaml:"apiMetadata,omitempty"`
	APIDefinition string `yaml:"apiDefinition,omitempty"`
	Docs          string `yaml:"docs,omitempty"`
	Content       string `yaml:"content,omitempty"`
}

type projectAPIResource struct {
	Metadata struct {
		Name string `yaml:"name"`
	} `yaml:"metadata"`
	Spec struct {
		Description         string                       `yaml:"description"`
		ReferenceID         string                       `yaml:"referenceID"`
		Tags                []string                     `yaml:"tags"`
		Labels              []string                     `yaml:"labels"`
		BusinessInformation devPortalBusinessInformation `yaml:"businessInformation"`
		Endpoints           devPortalEndpoints           `yaml:"endpoints"`
	} `yaml:"spec"`
}

type gatewayResource struct {
	Spec struct {
		DisplayName       string   `yaml:"displayName"`
		Version           string   `yaml:"version"`
		SubscriptionPlans []string `yaml:"subscriptionPlans"`
	} `yaml:"spec"`
}

type devPortalManifest struct {
	APIVersion string                `yaml:"apiVersion"`
	Kind       string                `yaml:"kind"`
	Metadata   devPortalManifestMeta `yaml:"metadata"`
	Spec       devPortalManifestSpec `yaml:"spec"`
}

type devPortalManifestMeta struct {
	Name string `yaml:"name"`
}

type devPortalManifestSpec struct {
	DisplayName          string                       `yaml:"displayName"`
	Version              string                       `yaml:"version"`
	Description          string                       `yaml:"description"`
	Provider             string                       `yaml:"provider"`
	ReferenceID          string                       `yaml:"referenceID"`
	Tags                 []string                     `yaml:"tags"`
	Labels               []string                     `yaml:"labels"`
	SubscriptionPolicies []string                     `yaml:"subscriptionPolicies"`
	Visibility           string                       `yaml:"visibility"`
	VisibleGroups        []string                     `yaml:"visibleGroups"`
	BusinessInformation  devPortalBusinessInformation `yaml:"businessInformation"`
	Endpoints            devPortalEndpoints           `yaml:"endpoints"`
}

type devPortalBusinessInformation struct {
	BusinessOwner       string `yaml:"businessOwner"`
	BusinessOwnerEmail  string `yaml:"businessOwnerEmail"`
	TechnicalOwner      string `yaml:"technicalOwner"`
	TechnicalOwnerEmail string `yaml:"technicalOwnerEmail"`
}

type devPortalEndpoints struct {
	SandboxURL    string `yaml:"sandboxUrl"`
	ProductionURL string `yaml:"productionUrl"`
}

func loadProjectConfig(configPath string) (*apiProjectConfig, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read project config: %w", err)
	}

	var config apiProjectConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse project config: %w", err)
	}

	normalizeAPIProjectConfig(&config)
	return &config, nil
}

func saveProjectConfig(configPath string, config *apiProjectConfig) error {
	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal project config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to save project config: %w", err)
	}

	return nil
}

func (c *apiProjectConfig) isDevportalConfigExists() bool {
	if len(c.DevPortals) == 0 {
		return false
	}
	return true
}

func (c *apiProjectConfig) createDefaultDevPortalConfig(projectRoot string) (*devPortalProjectConfig, error) {
	portalConfig := &devPortalProjectConfig{
		Name:       "default",
		PortalRoot: "./devportal",
		FilePaths: devPortalProjectPaths{
			APIMetadata:   "./devportal.yaml",
			APIDefinition: "./definition.yaml",
			Docs:          "./docs",
			Content:       "./content",
		},
	}

	portalRoot := resolveProjectPath(projectRoot, portalConfig.PortalRoot)
	if err := os.MkdirAll(portalRoot, 0755); err != nil {
		return nil, fmt.Errorf("failed to create devportal directory: %w", err)
	}

	definitionSource := resolveProjectPath(projectRoot, c.FilePaths.APIDefinition)
	definitionTarget := filepath.Join(portalRoot, "definition.yaml")
	if err := copyFile(definitionSource, definitionTarget); err != nil {
		return nil, err
	}

	docsSource := resolveProjectPath(projectRoot, c.FilePaths.Docs)
	docsTarget := filepath.Join(portalRoot, "docs")
	if err := copyDirectory(docsSource, docsTarget); err != nil {
		return nil, err
	}

	contentDir := filepath.Join(portalRoot, "content")
	if err := os.MkdirAll(contentDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create devportal content directory: %w", err)
	}

	manifest, err := buildDefaultDevPortalManifest(projectRoot, c)
	if err != nil {
		return nil, err
	}

	manifestData, err := yaml.Marshal(manifest)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal devportal manifest: %w", err)
	}

	manifestPath := filepath.Join(portalRoot, "devportal.yaml")
	if err := os.WriteFile(manifestPath, manifestData, 0644); err != nil {
		return nil, fmt.Errorf("failed to write devportal manifest: %w", err)
	}

	return portalConfig, nil
}

func buildDefaultDevPortalManifest(projectRoot string, projectConfig *apiProjectConfig) (*devPortalManifest, error) {
	apiMetadataPath := resolveProjectPath(projectRoot, projectConfig.FilePaths.APIMetadata)
	gatewayPath := resolveProjectPath(projectRoot, projectConfig.FilePaths.DeploymentArtifact)

	apiMetadataData, err := os.ReadFile(apiMetadataPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read api metadata: %w", err)
	}

	var apiMetadata projectAPIResource
	if err := yaml.Unmarshal(apiMetadataData, &apiMetadata); err != nil {
		return nil, fmt.Errorf("failed to parse api metadata: %w", err)
	}

	gatewayData, err := os.ReadFile(gatewayPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read gateway artifact: %w", err)
	}

	var gateway gatewayResource
	if err := yaml.Unmarshal(gatewayData, &gateway); err != nil {
		return nil, fmt.Errorf("failed to parse gateway artifact: %w", err)
	}

	tags := apiMetadata.Spec.Tags
	if len(tags) == 0 {
		tags = []string{"default"}
	}

	labels := apiMetadata.Spec.Labels
	if len(labels) == 0 {
		labels = []string{"default"}
	}

	return &devPortalManifest{
		APIVersion: "devportal.api-platform.wso2.com/v1",
		Kind:       "RestApi",
		Metadata: devPortalManifestMeta{
			Name: strings.TrimSpace(apiMetadata.Metadata.Name),
		},
		Spec: devPortalManifestSpec{
			DisplayName:          strings.TrimSpace(gateway.Spec.DisplayName),
			Version:              strings.TrimSpace(gateway.Spec.Version),
			Description:          strings.TrimSpace(apiMetadata.Spec.Description),
			Provider:             "WSO2",
			ReferenceID:          strings.TrimSpace(apiMetadata.Spec.ReferenceID),
			Tags:                 tags,
			Labels:               labels,
			SubscriptionPolicies: gateway.Spec.SubscriptionPlans,
			Visibility:           "PUBLIC",
			VisibleGroups:        []string{},
			BusinessInformation:  apiMetadata.Spec.BusinessInformation,
			Endpoints:            apiMetadata.Spec.Endpoints,
		},
	}, nil
}

func resolveProjectPath(projectRoot, pathValue string) string {
	trimmed := strings.TrimSpace(pathValue)
	if trimmed == "" {
		return projectRoot
	}

	trimmed = strings.TrimPrefix(trimmed, "./")
	return filepath.Join(projectRoot, filepath.Clean(trimmed))
}

func normalizeAPIProjectConfig(config *apiProjectConfig) {
	if strings.TrimSpace(config.FilePaths.DeploymentArtifact) == "" {
		config.FilePaths.DeploymentArtifact = "./gateway.yaml"
	}
	if strings.TrimSpace(config.FilePaths.APIMetadata) == "" {
		config.FilePaths.APIMetadata = "./api.yaml"
	}
	if strings.TrimSpace(config.FilePaths.APIDefinition) == "" {
		config.FilePaths.APIDefinition = "./definition.yaml"
	}
	if strings.TrimSpace(config.FilePaths.Docs) == "" {
		config.FilePaths.Docs = "./docs"
	}
	if strings.TrimSpace(config.FilePaths.Tests) == "" {
		config.FilePaths.Tests = "./tests"
	}
}

func normalizeDevPortalProjectConfig(config *devPortalProjectConfig) {
	if strings.TrimSpace(config.Name) == "" {
		config.Name = "default"
	}
	if strings.TrimSpace(config.PortalRoot) == "" {
		config.PortalRoot = "./devportal"
	}
	if strings.TrimSpace(config.FilePaths.APIMetadata) == "" {
		config.FilePaths.APIMetadata = "./devportal.yaml"
	}
	if strings.TrimSpace(config.FilePaths.APIDefinition) == "" {
		config.FilePaths.APIDefinition = "./definition.yaml"
	}
	if strings.TrimSpace(config.FilePaths.Docs) == "" {
		config.FilePaths.Docs = "./docs"
	}
	if strings.TrimSpace(config.FilePaths.Content) == "" {
		config.FilePaths.Content = "./content"
	}
}

func buildDevPortalArchives(projectRoot, buildDir string, portalConfigs []devPortalProjectConfig) ([]string, error) {
	zipPaths := make([]string, 0, len(portalConfigs))

	for i := range portalConfigs {
		normalizeDevPortalProjectConfig(&portalConfigs[i])

		if err := validateDevPortalConfig(projectRoot, &portalConfigs[i]); err != nil {
			return nil, err
		}

		stagingDir, err := createDevPortalArchiveStagingDir(projectRoot, buildDir, &portalConfigs[i])
		if err != nil {
			return nil, err
		}

		zipPath := filepath.Join(buildDir, buildDevPortalZipFileName(portalConfigs[i].Name))
		if err := utils.ZipDirectory(stagingDir, zipPath); err != nil {
			_ = os.RemoveAll(stagingDir)
			return nil, fmt.Errorf("failed to build devportal archive for %s: %w", portalConfigs[i].Name, err)
		}
		if err := os.RemoveAll(stagingDir); err != nil {
			return nil, fmt.Errorf("failed to clean staging directory for devportal config %q: %w", portalConfigs[i].Name, err)
		}

		zipPaths = append(zipPaths, zipPath)
	}

	return zipPaths, nil
}

func validateDevPortalConfig(projectRoot string, portalConfig *devPortalProjectConfig) error {
	portalRoot := resolveProjectPath(projectRoot, portalConfig.PortalRoot)
	if err := ensurePathExists(portalRoot, true, portalConfig.Name, "portalRoot"); err != nil {
		return err
	}

	requiredPaths := []struct {
		label string
		path  string
		isDir bool
	}{
		{label: "apiMetadata", path: resolvePortalConfigPath(projectRoot, portalConfig, portalConfig.FilePaths.APIMetadata), isDir: false},
		{label: "apiDefinition", path: resolvePortalConfigPath(projectRoot, portalConfig, portalConfig.FilePaths.APIDefinition), isDir: false},
		{label: "docs", path: resolvePortalConfigPath(projectRoot, portalConfig, portalConfig.FilePaths.Docs), isDir: true},
		{label: "content", path: resolvePortalConfigPath(projectRoot, portalConfig, portalConfig.FilePaths.Content), isDir: true},
	}

	for _, requiredPath := range requiredPaths {
		if err := ensurePathExists(requiredPath.path, requiredPath.isDir, portalConfig.Name, requiredPath.label); err != nil {
			return err
		}
	}

	return nil
}

func ensurePathExists(path string, wantDir bool, portalName, fieldName string) error {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("devportal config %q is invalid: %s path does not exist: %s", portalName, fieldName, path)
		}
		return fmt.Errorf("failed to inspect %s for devportal config %q: %w", fieldName, portalName, err)
	}

	if wantDir && !info.IsDir() {
		return fmt.Errorf("devportal config %q is invalid: %s must be a directory: %s", portalName, fieldName, path)
	}
	if !wantDir && info.IsDir() {
		return fmt.Errorf("devportal config %q is invalid: %s must be a file: %s", portalName, fieldName, path)
	}

	return nil
}

func createDevPortalArchiveStagingDir(projectRoot, buildDir string, portalConfig *devPortalProjectConfig) (string, error) {
	stagingRoot, err := os.MkdirTemp(buildDir, "devportal-build-*")
	if err != nil {
		return "", fmt.Errorf("failed to create staging directory for devportal config %q: %w", portalConfig.Name, err)
	}
	cleanupOnError := true
	defer func() {
		if cleanupOnError {
			_ = os.RemoveAll(stagingRoot)
		}
	}()

	copyOperations := []struct {
		source string
		target string
		isDir  bool
	}{
		{
			source: resolvePortalConfigPath(projectRoot, portalConfig, portalConfig.FilePaths.APIMetadata),
			target: filepath.Join(stagingRoot, archiveRelativePath(portalConfig.FilePaths.APIMetadata)),
			isDir:  false,
		},
		{
			source: resolvePortalConfigPath(projectRoot, portalConfig, portalConfig.FilePaths.APIDefinition),
			target: filepath.Join(stagingRoot, archiveRelativePath(portalConfig.FilePaths.APIDefinition)),
			isDir:  false,
		},
		{
			source: resolvePortalConfigPath(projectRoot, portalConfig, portalConfig.FilePaths.Docs),
			target: filepath.Join(stagingRoot, archiveRelativePath(portalConfig.FilePaths.Docs)),
			isDir:  true,
		},
		{
			source: resolvePortalConfigPath(projectRoot, portalConfig, portalConfig.FilePaths.Content),
			target: filepath.Join(stagingRoot, archiveRelativePath(portalConfig.FilePaths.Content)),
			isDir:  true,
		},
	}

	for _, copyOperation := range copyOperations {
		if copyOperation.isDir {
			if err := copyDirectory(copyOperation.source, copyOperation.target); err != nil {
				return "", err
			}
			continue
		}

		if err := copyFile(copyOperation.source, copyOperation.target); err != nil {
			return "", err
		}
	}

	cleanupOnError = false
	return stagingRoot, nil
}

func resolvePortalConfigPath(projectRoot string, portalConfig *devPortalProjectConfig, pathValue string) string {
	portalRoot := resolveProjectPath(projectRoot, portalConfig.PortalRoot)
	trimmed := strings.TrimSpace(pathValue)
	if trimmed == "" {
		return portalRoot
	}

	return filepath.Clean(filepath.Join(portalRoot, trimmed))
}

func archiveRelativePath(pathValue string) string {
	cleanPath := filepath.Clean(strings.TrimSpace(pathValue))
	cleanPath = strings.TrimPrefix(cleanPath, "."+string(os.PathSeparator))
	cleanPath = strings.TrimPrefix(cleanPath, "."+"/")

	for strings.HasPrefix(cleanPath, ".."+string(os.PathSeparator)) || cleanPath == ".." {
		cleanPath = strings.TrimPrefix(cleanPath, ".."+string(os.PathSeparator))
		if cleanPath == ".." {
			cleanPath = ""
		}
	}

	cleanPath = strings.TrimPrefix(cleanPath, string(os.PathSeparator))
	cleanPath = strings.TrimPrefix(cleanPath, "/")
	if cleanPath == "" || cleanPath == "." {
		return "content"
	}

	return cleanPath
}

func buildDevPortalZipFileName(portalName string) string {
	trimmedName := strings.TrimSpace(portalName)
	if trimmedName == "" || trimmedName == "default" {
		return "devportal.zip"
	}

	return fmt.Sprintf("devportal_%s.zip", sanitizeArchiveName(trimmedName))
}

func sanitizeArchiveName(value string) string {
	sanitized := strings.ToLower(strings.TrimSpace(value))
	sanitized = strings.ReplaceAll(sanitized, " ", "-")
	sanitized = strings.ReplaceAll(sanitized, "/", "-")
	sanitized = strings.ReplaceAll(sanitized, "\\", "-")
	sanitized = strings.ReplaceAll(sanitized, "_", "-")
	sanitized = strings.Trim(sanitized, "-.")
	if sanitized == "" {
		return "default"
	}

	return sanitized
}

func copyFile(sourcePath, targetPath string) error {
	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("failed to open source file %s: %w", sourcePath, err)
	}
	defer sourceFile.Close()

	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		return fmt.Errorf("failed to create target directory for %s: %w", targetPath, err)
	}

	targetFile, err := os.Create(targetPath)
	if err != nil {
		return fmt.Errorf("failed to create target file %s: %w", targetPath, err)
	}
	defer targetFile.Close()

	if _, err := io.Copy(targetFile, sourceFile); err != nil {
		return fmt.Errorf("failed to copy %s to %s: %w", sourcePath, targetPath, err)
	}

	return nil
}

func copyDirectory(sourceDir, targetDir string) error {
	if err := os.RemoveAll(targetDir); err != nil {
		return fmt.Errorf("failed to reset target directory %s: %w", targetDir, err)
	}

	return filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return fmt.Errorf("failed to resolve relative path for %s: %w", path, err)
		}

		targetPath := filepath.Join(targetDir, relPath)
		if info.IsDir() {
			return os.MkdirAll(targetPath, 0755)
		}

		return copyFile(path, targetPath)
	})
}
