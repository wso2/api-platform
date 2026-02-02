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

var versionDirRegex = regexp.MustCompile(`^v?\d+\.\d+\.\d+$`)

// ValidateLocalPolicyZip validates a local policy zip file structure and content.
// It ensures that the provided zip contains a policy-definition.yaml at the root
// of the archive (no nested single top-level folder is allowed). Name and version
// are not returned from the zip; they are expected to come from the manifest.
func ValidateLocalPolicyZip(zipPath, expectedName, expectedVersion string) error {
	// Ensure file is a zip
	zipFileName := filepath.Base(zipPath)
	if !strings.HasSuffix(zipFileName, ".zip") {
		return fmt.Errorf("policy file must be a .zip file, got: %s", zipFileName)
	}

	// Extract to a temp dir and validate presence of policy-definition.yaml at root
	tempExtractDir, err := os.MkdirTemp("", "policy-validate-*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory for validation: %w", err)
	}
	defer os.RemoveAll(tempExtractDir)

	if err := Unzip(zipPath, tempExtractDir); err != nil {
		return fmt.Errorf("failed to extract policy zip: %w", err)
	}

	policyDefPath := filepath.Join(tempExtractDir, "policy-definition.yaml")
	if _, err := os.Stat(policyDefPath); os.IsNotExist(err) {
		return fmt.Errorf("policy-definition.yaml not found at root of zip archive")
	} else if err != nil {
		return fmt.Errorf("failed to stat policy-definition.yaml: %w", err)
	}

	// Quick parse to ensure YAML is valid and contains name/version
	data, err := os.ReadFile(policyDefPath)
	if err != nil {
		return fmt.Errorf("failed to read policy-definition.yaml: %w", err)
	}

	var pd struct {
		Name    string `yaml:"name"`
		Version string `yaml:"version"`
	}
	if err := yaml.Unmarshal(data, &pd); err != nil {
		return fmt.Errorf("failed to parse policy-definition.yaml: %w", err)
	}

	if pd.Name == "" {
		return fmt.Errorf("'name' field is required in policy-definition.yaml")
	}

	if expectedName != "" && pd.Name != expectedName {
		return fmt.Errorf("name mismatch: manifest specifies '%s' but policy-definition.yaml contains '%s'", expectedName, pd.Name)
	}

	return nil
}

func ValidateLocalPolicyDir(dirPath string, expectedName string) error {
	info, err := os.Stat(dirPath)
	if err != nil {
		return fmt.Errorf("failed to stat policy directory: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("policy path is not a directory: %s", dirPath)
	}

	policyDefPath := filepath.Join(dirPath, "policy-definition.yaml")
	if _, err := os.Stat(policyDefPath); os.IsNotExist(err) {
		return fmt.Errorf("policy-definition.yaml not found at root of directory")
	} else if err != nil {
		return fmt.Errorf("failed to stat policy-definition.yaml: %w", err)
	}

	data, err := os.ReadFile(policyDefPath)
	if err != nil {
		return fmt.Errorf("failed to read policy-definition.yaml: %w", err)
	}

	var pd struct {
		Name    string `yaml:"name"`
		Version string `yaml:"version"`
	}
	if err := yaml.Unmarshal(data, &pd); err != nil {
		return fmt.Errorf("failed to parse policy-definition.yaml: %w", err)
	}

	if pd.Name == "" {
		return fmt.Errorf("'name' field is required in policy-definition.yaml")
	}

	if expectedName != "" && pd.Name != expectedName {
		return fmt.Errorf("name mismatch: manifest specifies '%s' but policy-definition.yaml contains '%s'", expectedName, pd.Name)
	}

	return nil
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
// This includes: output/, policies/<name>/<version>/ and a copy of the lock file
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

	// Copy lock file to temp gateway image build directory if provided
	if lockFilePath != "" {
		lockFileContent, err := os.ReadFile(lockFilePath)
		if err != nil {
			return fmt.Errorf("failed to read lock file: %w", err)
		}

		lockPath := filepath.Join(tempGatewayImageBuildDir, "policy-manifest-lock.yaml")
		if err := os.WriteFile(lockPath, lockFileContent, 0644); err != nil {
			return fmt.Errorf("failed to write lock file to temp gateway image build directory: %w", err)
		}
	}

	return nil
}

// SetupTempGatewayWorkspace prepares the workspace by creating required folders, copying
// local policies into the workspace, updating the manifest's filePath entries to point
// to the workspace paths, and writing the modified manifest as policy-manifest.yaml
// (renamed from the original build.yaml file for Docker build compatibility).
func SetupTempGatewayWorkspace(manifestFilePath string) (string, error) {
	tempGatewayImageBuildDir, err := GetTempGatewayImageBuildDir()
	if err != nil {
		return "", fmt.Errorf("failed to get temp gateway image build directory path: %w", err)
	}

	// Remove existing "gateway-image-build" directory if it exists
	if _, err := os.Stat(tempGatewayImageBuildDir); err == nil {
		if err := os.RemoveAll(tempGatewayImageBuildDir); err != nil {
			return "", fmt.Errorf("failed to remove existing temp gateway image build directory: %w", err)
		}
	}

	// Create the temp gateway image build directory structure
	if err := EnsureDir(tempGatewayImageBuildDir); err != nil {
		return "", fmt.Errorf("failed to create temp gateway image build directory: %w", err)
	}

	// Create output directory
	outputDir := filepath.Join(tempGatewayImageBuildDir, "output")
	if err := EnsureDir(outputDir); err != nil {
		return "", fmt.Errorf("failed to create output directory: %w", err)
	}

	// Create policies directory
	policiesDir := filepath.Join(tempGatewayImageBuildDir, "policies")
	if err := EnsureDir(policiesDir); err != nil {
		return "", fmt.Errorf("failed to create policies directory: %w", err)
	}

	// Read and parse manifest YAML (using a lightweight local struct to avoid import cycles)
	manifestData, err := os.ReadFile(manifestFilePath)
	if err != nil {
		return "", fmt.Errorf("failed to read manifest file: %w", err)
	}

	var manifest struct {
		Version  string `yaml:"version"`
		Policies []struct {
			Name     string `yaml:"name"`
			Version  string `yaml:"version,omitempty"`
			FilePath string `yaml:"filePath,omitempty"`
			Gomodule string `yaml:"gomodule,omitempty"`
		} `yaml:"policies"`
	}
	if err := yaml.Unmarshal(manifestData, &manifest); err != nil {
		return "", fmt.Errorf("failed to parse manifest YAML: %w", err)
	}

	// For each local policy with a filePath, copy it into the workspace and update the filePath
	manifestDir := filepath.Dir(manifestFilePath)
	for i := range manifest.Policies {
		p := &manifest.Policies[i]
		if p.FilePath == "" {
			continue // Skip Gomodule policies
		}

		// Resolve source path relative to manifest
		srcPath := p.FilePath
		if !filepath.IsAbs(srcPath) {
			srcPath = filepath.Join(manifestDir, srcPath)
		}

		// Copy into workspace (requires directory)
		workspaceRel, err := CopyPolicyToWorkspace(p.Name, p.Version, srcPath, true)
		if err != nil {
			return "", fmt.Errorf("failed to copy local policy %s v%s into workspace: %w", p.Name, p.Version, err)
		}

		// Update manifest entry to point to workspace-relative path
		p.FilePath = workspaceRel
	}

	// Marshal updated manifest and write into workspace as policy-manifest.yaml
	newManifestData, err := yaml.Marshal(&manifest)
	if err != nil {
		return "", fmt.Errorf("failed to marshal updated manifest: %w", err)
	}

	manifestDst := filepath.Join(tempGatewayImageBuildDir, "policy-manifest.yaml")
	if err := os.WriteFile(manifestDst, newManifestData, 0644); err != nil {
		return "", fmt.Errorf("failed to write updated manifest to workspace: %w", err)
	}

	return tempGatewayImageBuildDir, nil
}

// CopyPolicyToWorkspace copies a policy to the workspace
// For local policies: copies the source directory into workspace/policies/<original-dir-name>/
// Final structure in workspace: .wso2ap/.tmp/gateway-image-build/policies/<original-dir-name>/
func CopyPolicyToWorkspace(policyName, policyVersion, sourcePath string, isLocal bool) (string, error) {
	tempGatewayImageBuildDir, err := GetTempGatewayImageBuildDir()
	if err != nil {
		return "", fmt.Errorf("failed to get temp gateway image build directory path: %w", err)
	}

	if isLocal {
		// Use the original directory name from the source path (e.g., 'my-policy')
		dirName := filepath.Base(filepath.Clean(sourcePath))
		workspacePolicyDir := filepath.Join(tempGatewayImageBuildDir, "policies", dirName)

		// Ensure destination directory exists
		if err := EnsureDir(workspacePolicyDir); err != nil {
			return "", fmt.Errorf("failed to create workspace policy directory: %w", err)
		}

		info, err := os.Stat(sourcePath)
		if err != nil {
			return "", fmt.Errorf("failed to stat local policy path '%s': %w", sourcePath, err)
		}
		if !info.IsDir() {
			return "", fmt.Errorf("local policy '%s' must be a directory", sourcePath)
		}

		// Copy the entire local policy directory into workspace/policies/<original-dir-name>/
		if err := CopyDir(sourcePath, workspacePolicyDir); err != nil {
			return "", fmt.Errorf("failed to copy local policy directory: %w", err)
		}

		// Return relative path for manifest: policies/<original-dir-name>
		return filepath.Join("policies", dirName), nil
	}

	return "", fmt.Errorf("non-local policies are not supported by CLI workspace copy: %s", sourcePath)
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
