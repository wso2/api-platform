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

package utils

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// PolicyDefinition represents the structure of policy-definition.yaml
type PolicyDefinition struct {
	Name    string `yaml:"name"`
	Version string `yaml:"version"`
}

// normalizeNameForComparison converts a name to kebab-case for comparison
func normalizeNameForComparison(name string) string {
	// Convert to kebab-case (lowercase with hyphens)
	var result strings.Builder

	for i, r := range name {
		// Add hyphen before uppercase letters (except the first character)
		if i > 0 && r >= 'A' && r <= 'Z' {
			result.WriteRune('-')
		}
		result.WriteRune(r)
	}

	return strings.ToLower(result.String())
}

// ValidateLocalPolicyZip validates a local policy zip file structure and content
// Returns the extracted policy name and version if valid, error otherwise
func ValidateLocalPolicyZip(zipPath string) (policyName string, policyVersion string, err error) {
	// Ensure file is a zip
	zipFileName := filepath.Base(zipPath)
	if !strings.HasSuffix(zipFileName, ".zip") {
		return "", "", fmt.Errorf("policy file must be a .zip file, got: %s", zipFileName)
	}

	// 2. Extract and validate zip structure
	tempExtractDir, err := os.MkdirTemp("", "policy-validate-*")
	if err != nil {
		return "", "", fmt.Errorf("failed to create temp directory for validation: %w", err)
	}
	defer os.RemoveAll(tempExtractDir)

	if err := Unzip(zipPath, tempExtractDir); err != nil {
		return "", "", fmt.Errorf("failed to extract policy zip: %w", err)
	}

	// 3. Check for policy-definition.yaml at root
	policyDefPath := filepath.Join(tempExtractDir, "policy-definition.yaml")
	if _, err := os.Stat(policyDefPath); os.IsNotExist(err) {
		return "", "", fmt.Errorf("policy-definition.yaml not found at root of zip\n" +
			"Expected structure:\n" +
			"  <zip-file>.zip\n" +
			"  ├── policy-definition.yaml\n" +
			"  ├── <policy-code>.go\n" +
			"  └── go.mod")
	}

	// 4. Parse and validate policy-definition.yaml
	data, err := os.ReadFile(policyDefPath)
	if err != nil {
		return "", "", fmt.Errorf("failed to read policy-definition.yaml: %w", err)
	}

	var policyDef PolicyDefinition
	if err := yaml.Unmarshal(data, &policyDef); err != nil {
		return "", "", fmt.Errorf("failed to parse policy-definition.yaml: %w", err)
	}

	// 5. Ensure required fields exist in policy-definition.yaml
	if policyDef.Name == "" {
		return "", "", fmt.Errorf("'name' field is required in policy-definition.yaml")
	}
	if policyDef.Version == "" {
		return "", "", fmt.Errorf("'version' field is required in policy-definition.yaml")
	}

	// Normalize version to include 'v' prefix for returned value
	yamlVersion := policyDef.Version
	if !strings.HasPrefix(yamlVersion, "v") {
		yamlVersion = "v" + yamlVersion
	}

	return policyDef.Name, yamlVersion, nil
}

// CalculateSHA256 calculates SHA-256 checksum of a file
func CalculateSHA256(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("failed to calculate checksum: %w", err)
	}

	checksum := hex.EncodeToString(hash.Sum(nil))
	return fmt.Sprintf("sha256:%s", checksum), nil
}

// VerifyChecksum verifies if a file's checksum matches the expected value
func VerifyChecksum(filePath, expectedChecksum string) (bool, error) {
	actualChecksum, err := CalculateSHA256(filePath)
	if err != nil {
		return false, err
	}
	return actualChecksum == expectedChecksum, nil
}

// ZipDirectory creates a zip archive of a directory
func ZipDirectory(sourceDir, destZip string) error {
	// Create the destination zip file
	zipFile, err := os.Create(destZip)
	if err != nil {
		return fmt.Errorf("failed to create zip file: %w", err)
	}
	defer zipFile.Close()

	// Create a new zip writer
	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()

	// Walk through the source directory
	return filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Get the relative path
		relPath, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return err
		}

		// Skip the root directory itself
		if relPath == "." {
			return nil
		}

		// Create a zip header
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}

		// Set the header name to the relative path
		header.Name = relPath

		// Handle directories
		if info.IsDir() {
			header.Name += "/"
		} else {
			header.Method = zip.Deflate
		}

		// Create the zip entry
		writer, err := zipWriter.CreateHeader(header)
		if err != nil {
			return err
		}

		// If it's a file, write its contents
		if !info.IsDir() {
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			defer file.Close()

			_, err = io.Copy(writer, file)
			if err != nil {
				return err
			}
		}

		return nil
	})
}

// FindPoliciesFolders recursively finds all directories named "policies"
func FindPoliciesFolders(rootDir string) ([]string, error) {
	var policiesDirs []string

	err := filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() && info.Name() == "policies" {
			policiesDirs = append(policiesDirs, path)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to search for policies folders: %w", err)
	}

	return policiesDirs, nil
}

// PolicyExists searches for a policy in given search paths
// Returns the full path if found, error if not found
func PolicyExists(policyName, version string, searchPaths []string) (string, error) {
	// Format: <policy-name>-v<version>.zip or <policy-name>/v<version>/
	zipFileName := fmt.Sprintf("%s-v%s.zip", ToKebabCase(policyName), version)
	dirName := fmt.Sprintf("%s/v%s", ToKebabCase(policyName), version)

	for _, searchPath := range searchPaths {
		// Check for zip file
		zipPath := filepath.Join(searchPath, zipFileName)
		if _, err := os.Stat(zipPath); err == nil {
			return zipPath, nil
		}

		// Check for directory
		dirPath := filepath.Join(searchPath, dirName)
		if info, err := os.Stat(dirPath); err == nil && info.IsDir() {
			return dirPath, nil
		}
	}

	return "", fmt.Errorf("policy %s v%s not found in any search path", policyName, version)
}

// ToKebabCase converts a string to kebab-case
func ToKebabCase(s string) string {
	var result strings.Builder

	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			result.WriteRune('-')
		}
		result.WriteRune(r)
	}

	return strings.ToLower(result.String())
}

// FormatPolicyFileName creates a standardized policy file name
func FormatPolicyFileName(policyName, version string) string {
	return fmt.Sprintf("%s-%s.zip", ToKebabCase(policyName), version)
}

// EnsureDir creates a directory if it doesn't exist
func EnsureDir(dirPath string) error {
	if _, err := os.Stat(dirPath); os.IsNotExist(err) {
		if err := os.MkdirAll(dirPath, 0755); err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}
	}
	return nil
}

// CleanTempDir removes all contents of the temp directory
func CleanTempDir() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get user home directory: %w", err)
	}

	tempDir := filepath.Join(homeDir, TempPath)

	// Check if temp directory exists
	if _, err := os.Stat(tempDir); os.IsNotExist(err) {
		return nil // Nothing to clean
	}

	// Remove all contents
	if err := os.RemoveAll(tempDir); err != nil {
		return fmt.Errorf("failed to clean temp directory: %w", err)
	}

	return nil
}

// GetTempDir returns the path to the temp directory, creating it if necessary
func GetTempDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}

	tempDir := filepath.Join(homeDir, TempPath)
	if err := EnsureDir(tempDir); err != nil {
		return "", err
	}

	return tempDir, nil
}

// GetCacheDir returns the path to the policies cache directory
func GetCacheDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}

	cacheDir := filepath.Join(homeDir, PoliciesCachePath)
	if err := EnsureDir(cacheDir); err != nil {
		return "", err
	}

	return cacheDir, nil
}

// Unzip extracts a zip file to a destination directory
func Unzip(src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return fmt.Errorf("failed to open zip file: %w", err)
	}
	defer r.Close()

	if err := EnsureDir(dest); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	for _, f := range r.File {
		// Construct the full path
		fpath := filepath.Join(dest, f.Name)

		// Check for ZipSlip vulnerability
		if !strings.HasPrefix(fpath, filepath.Clean(dest)+string(os.PathSeparator)) {
			return fmt.Errorf("illegal file path: %s", fpath)
		}

		if f.FileInfo().IsDir() {
			// Create directory
			if err := EnsureDir(fpath); err != nil {
				return err
			}
			continue
		}

		// Create directory for file
		if err := EnsureDir(filepath.Dir(fpath)); err != nil {
			return err
		}

		// Extract file
		outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return fmt.Errorf("failed to create file: %w", err)
		}

		rc, err := f.Open()
		if err != nil {
			outFile.Close()
			return fmt.Errorf("failed to open zip entry: %w", err)
		}

		_, err = io.Copy(outFile, rc)
		outFile.Close()
		rc.Close()

		if err != nil {
			return fmt.Errorf("failed to extract file: %w", err)
		}
	}

	return nil
}

// GetTempGatewayImageBuildDir returns the path to the temp gateway image build output directory (.wso2ap/.tmp/gateway-image-build)
func GetTempGatewayImageBuildDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}

	tempGatewayImageBuildDir := filepath.Join(homeDir, TempPath, "gateway-image-build")
	return tempGatewayImageBuildDir, nil
}

// SetupTempGatewayImageBuildDir creates the temp "gateway-image-build" directory structure for the build
// This includes: output/, policies/<name>/<version>/, and policy-manifest-lock.yaml
// Location: .wso2ap/.tmp/gateway-image-build
// If a "gateway-image-build" directory already exists, it will be removed first
func SetupTempGatewayImageBuildDir(lockFilePath string) error {
	tempGatewayImageBuildDir, err := GetTempGatewayImageBuildDir()
	if err != nil {
		return fmt.Errorf("failed to get temp gateway image build directory path: %w", err)
	}

	// Remove existing "gateway-image-build" directory if it exists
	if _, err := os.Stat(tempGatewayImageBuildDir); err == nil {
		if err := os.RemoveAll(tempGatewayImageBuildDir); err != nil {
			return fmt.Errorf("failed to remove existing temp gateway image build directory: %w", err)
		}
	}

	// Create the temp gateway image build directory structure
	if err := EnsureDir(tempGatewayImageBuildDir); err != nil {
		return fmt.Errorf("failed to create temp gateway image build directory: %w", err)
	}

	// Create output directory
	outputDir := filepath.Join(tempGatewayImageBuildDir, "output")
	if err := EnsureDir(outputDir); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Create policies directory
	policiesDir := filepath.Join(tempGatewayImageBuildDir, "policies")
	if err := EnsureDir(policiesDir); err != nil {
		return fmt.Errorf("failed to create policies directory: %w", err)
	}

	// Copy lock file to temp gateway image build directory
	lockFileContent, err := os.ReadFile(lockFilePath)
	if err != nil {
		return fmt.Errorf("failed to read lock file: %w", err)
	}

	lockPath := filepath.Join(tempGatewayImageBuildDir, "policy-manifest-lock.yaml")
	if err := os.WriteFile(lockPath, lockFileContent, 0644); err != nil {
		return fmt.Errorf("failed to write lock file to temp gateway image build directory: %w", err)
	}

	return nil
}

// CopyPolicyToWorkspace copies a policy to the workspace using the new cache structure
// For hub policies: reads from cache using kebab-case path
// For local policies: reads from manifest filePath
// Final structure: .wso2ap/.tmp/gateway-image-build/policies/<kebab-case-name>/<version>/
func CopyPolicyToWorkspace(policyName, policyVersion, sourcePath string, isLocal bool, index *PolicyIndex) (string, error) {
	tempGatewayImageBuildDir, err := GetTempGatewayImageBuildDir()
	if err != nil {
		return "", fmt.Errorf("failed to get temp gateway image build directory path: %w", err)
	}

	// Generate kebab-case path for workspace
	kebabPath := GenerateUniqueCachePath(policyName, policyVersion, index)

	// Workspace destination: policies/<kebab-case-name>/<version>/
	workspacePolicyDir := filepath.Join(tempGatewayImageBuildDir, "policies", kebabPath)

	// Ensure destination directory exists
	if err := EnsureDir(workspacePolicyDir); err != nil {
		return "", fmt.Errorf("failed to create workspace policy directory: %w", err)
	}

	// Extract zip to temporary location
	tempExtractDir, err := os.MkdirTemp("", "policy-extract-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp extract directory: %w", err)
	}
	defer os.RemoveAll(tempExtractDir)

	if err := Unzip(sourcePath, tempExtractDir); err != nil {
		return "", fmt.Errorf("failed to extract policy: %w", err)
	}

	// Both hub and local policies have files at root, copy directly
	if err := CopyDir(tempExtractDir, workspacePolicyDir); err != nil {
		return "", fmt.Errorf("failed to copy policy contents: %w", err)
	}

	// Return the relative path for lock file: policies/<kebab-case-name>/<version>
	return filepath.Join("policies", kebabPath), nil
}

// CopyPolicyToTempGatewayImageBuild copies a validated policy zip to the temp gateway image build directory structure
// DEPRECATED: Use CopyPolicyToWorkspace instead for new cache structure
// The policy will be organized as: .wso2ap/.tmp/gateway-image-build/policies/<name>/<version>/
// Note: This function expects a validated policy zip file (via ValidateLocalPolicyZip)
func CopyPolicyToTempGatewayImageBuild(policyName, policyVersion, sourcePath string) error {
	tempGatewayImageBuildDir, err := GetTempGatewayImageBuildDir()
	if err != nil {
		return fmt.Errorf("failed to get temp gateway image build directory path: %w", err)
	}

	// Only zip files are supported (directories are no longer accepted)
	fileInfo, err := os.Stat(sourcePath)
	if err != nil {
		return fmt.Errorf("failed to stat source path: %w", err)
	}

	if fileInfo.IsDir() {
		return fmt.Errorf("policy source must be a zip file, not a directory: %s", sourcePath)
	}

	// Extract zip to temporary location
	tempExtractDir, err := os.MkdirTemp("", "policy-extract-*")
	if err != nil {
		return fmt.Errorf("failed to create temp extract directory: %w", err)
	}
	defer os.RemoveAll(tempExtractDir)

	if err := Unzip(sourcePath, tempExtractDir); err != nil {
		return fmt.Errorf("failed to extract policy: %w", err)
	}

	// Read the extracted content
	entries, err := os.ReadDir(tempExtractDir)
	if err != nil {
		return fmt.Errorf("failed to read extracted directory: %w", err)
	}

	// Extract policy name from zip filename
	zipFileName := filepath.Base(sourcePath)
	extractedPolicyName := ExtractPolicyNameFromZipFilename(zipFileName)

	// Find the version folder (ignore metadata folders)
	var versionFolderName string
	var versionFolderCount int
	for _, entry := range entries {
		// Skip macOS metadata folders and hidden folders
		if entry.IsDir() && !strings.HasPrefix(entry.Name(), "__") && !strings.HasPrefix(entry.Name(), ".") {
			versionFolderName = entry.Name()
			versionFolderCount++
		}
	}

	// Ensure we found exactly one version folder
	if versionFolderCount != 1 {
		return fmt.Errorf("invalid policy structure: expected single version folder, found %d (excluding metadata folders)", versionFolderCount)
	}

	// Create destination: .tmp/gateway-image-build/policies/<name>/<version>/
	policiesBaseDir := filepath.Join(tempGatewayImageBuildDir, "policies", extractedPolicyName)
	if err := EnsureDir(policiesBaseDir); err != nil {
		return fmt.Errorf("failed to create policy base directory: %w", err)
	}

	// Copy version folder to destination
	srcVersionDir := filepath.Join(tempExtractDir, versionFolderName)
	dstVersionDir := filepath.Join(policiesBaseDir, versionFolderName)

	if err := CopyDir(srcVersionDir, dstVersionDir); err != nil {
		return fmt.Errorf("failed to copy version directory: %w", err)
	}

	return nil
}

// ExtractPolicyNameFromZipFilename extracts the policy name from a zip filename
// Examples: basic-auth-v1.0.0.zip -> basic-auth, jwt-auth-v2.1.0.zip -> jwt-auth
func ExtractPolicyNameFromZipFilename(filename string) string {
	// Remove .zip extension
	name := strings.TrimSuffix(filename, ".zip")

	// Find the last occurrence of version pattern (e.g., -v1.0.0)
	// Match pattern like: -v<digit>
	re := regexp.MustCompile(`-v\d+`)
	loc := re.FindStringIndex(name)

	if loc != nil {
		// Return everything before the version part
		return name[:loc[0]]
	}

	// If no version pattern found, return the whole name
	return name
}

// CopyDir recursively copies a directory
func CopyDir(src, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return fmt.Errorf("failed to read directory: %w", err)
	}

	if err := EnsureDir(dst); err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := CopyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}

	return nil
}

// copyFile copies a single file
func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer destFile.Close()

	if _, err := io.Copy(destFile, sourceFile); err != nil {
		return fmt.Errorf("failed to copy file: %w", err)
	}

	// Copy file permissions
	sourceInfo, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("failed to stat source file: %w", err)
	}

	if err := os.Chmod(dst, sourceInfo.Mode()); err != nil {
		return fmt.Errorf("failed to set file permissions: %w", err)
	}

	return nil
}
