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

package testutils

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"
)

// invalidIdentifierChars matches any character that is not valid in a Go identifier.
var invalidIdentifierChars = regexp.MustCompile(`[^a-zA-Z0-9_]`)

// SanitizePackageName converts a policy name to a valid Go package name.
// Go identifiers must match [a-zA-Z_][a-zA-Z0-9_]*, so this function:
// - Replaces all non-alphanumeric/underscore characters with underscores
// - Prefixes with underscore if the name starts with a digit
// - Returns "_" if the result would be empty
func SanitizePackageName(name string) string {
	result := invalidIdentifierChars.ReplaceAllString(name, "_")
	// If starts with a digit, prefix with underscore
	if len(result) > 0 && result[0] >= '0' && result[0] <= '9' {
		result = "_" + result
	}
	if result == "" {
		result = "_"
	}
	return result
}

// WriteFile creates a file at the specified path with the given content.
// Parent directories are created if they don't exist.
func WriteFile(t *testing.T, path, content string) {
	t.Helper()
	dir := filepath.Dir(path)
	err := os.MkdirAll(dir, 0755)
	require.NoError(t, err, "failed to create directory %s", dir)
	err = os.WriteFile(path, []byte(content), 0644)
	require.NoError(t, err, "failed to write file %s", path)
}

// CreateDir creates a directory at the specified path.
// Parent directories are created if they don't exist.
func CreateDir(t *testing.T, path string) {
	t.Helper()
	err := os.MkdirAll(path, 0755)
	require.NoError(t, err, "failed to create directory %s", path)
}

// CreateSourceFile creates a Go source file in the specified directory.
func CreateSourceFile(t *testing.T, dir, filename, content string) {
	t.Helper()
	path := filepath.Join(dir, filename)
	WriteFile(t, path, content)
}

// CreateMinimalGoSource creates a minimal valid Go source file in the directory.
// Useful when you just need a compilable Go package.
// The packageName is sanitized to be a valid Go identifier.
func CreateMinimalGoSource(t *testing.T, dir, packageName string) {
	t.Helper()
	safeName := SanitizePackageName(packageName)
	content := "package " + safeName + "\n"
	CreateSourceFile(t, dir, safeName+".go", content)
}

// CreateMainGoSource creates a main.go file with a minimal main function.
func CreateMainGoSource(t *testing.T, dir string) {
	t.Helper()
	content := `package main

func main() {}
`
	CreateSourceFile(t, dir, "main.go", content)
}

// CreateGoSourceWithImport creates a Go source file that imports a specific package.
func CreateGoSourceWithImport(t *testing.T, dir, packageName, importPath string) {
	t.Helper()
	content := `package ` + packageName + `

import _ "` + importPath + `"
`
	CreateSourceFile(t, dir, packageName+".go", content)
}
