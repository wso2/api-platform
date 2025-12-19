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
	"strings"
)

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
	return fmt.Sprintf("%s-v%s.zip", ToKebabCase(policyName), version)
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
