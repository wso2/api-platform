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

package fsutil

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidatePathExists_FileExists(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.txt")
	err := os.WriteFile(filePath, []byte("content"), 0644)
	require.NoError(t, err)

	err = ValidatePathExists(filePath, "test file")
	assert.NoError(t, err)
}

func TestValidatePathExists_DirectoryExists(t *testing.T) {
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "subdir")
	err := os.Mkdir(subDir, 0755)
	require.NoError(t, err)

	err = ValidatePathExists(subDir, "test directory")
	assert.NoError(t, err)
}

func TestValidatePathExists_FileNotExists(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "nonexistent.txt")

	err := ValidatePathExists(filePath, "policy file")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "policy file does not exist")
	assert.Contains(t, err.Error(), filePath)
}

func TestValidatePathExists_DirectoryNotExists(t *testing.T) {
	tmpDir := t.TempDir()
	dirPath := filepath.Join(tmpDir, "nonexistent-dir")

	err := ValidatePathExists(dirPath, "policy directory")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "policy directory does not exist")
}

func TestCopyFile_Success(t *testing.T) {
	tmpDir := t.TempDir()

	// Create source file
	srcPath := filepath.Join(tmpDir, "source.txt")
	content := []byte("test content for copy")
	err := os.WriteFile(srcPath, content, 0644)
	require.NoError(t, err)

	// Copy file
	dstPath := filepath.Join(tmpDir, "destination.txt")
	err = CopyFile(srcPath, dstPath)
	assert.NoError(t, err)

	// Verify destination file exists and has correct content
	dstContent, err := os.ReadFile(dstPath)
	require.NoError(t, err)
	assert.Equal(t, content, dstContent)
}

func TestCopyFile_SourceNotFound(t *testing.T) {
	tmpDir := t.TempDir()

	srcPath := filepath.Join(tmpDir, "nonexistent.txt")
	dstPath := filepath.Join(tmpDir, "destination.txt")

	err := CopyFile(srcPath, dstPath)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to open source file")
}

func TestCopyFile_DestinationDirectoryNotExists(t *testing.T) {
	tmpDir := t.TempDir()

	// Create source file
	srcPath := filepath.Join(tmpDir, "source.txt")
	err := os.WriteFile(srcPath, []byte("content"), 0644)
	require.NoError(t, err)

	// Try to copy to nonexistent directory
	dstPath := filepath.Join(tmpDir, "nonexistent-dir", "destination.txt")
	err = CopyFile(srcPath, dstPath)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create destination file")
}

func TestCopyFile_LargeFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a larger source file (1MB)
	srcPath := filepath.Join(tmpDir, "large.bin")
	content := make([]byte, 1024*1024)
	for i := range content {
		content[i] = byte(i % 256)
	}
	err := os.WriteFile(srcPath, content, 0644)
	require.NoError(t, err)

	// Copy file
	dstPath := filepath.Join(tmpDir, "large_copy.bin")
	err = CopyFile(srcPath, dstPath)
	assert.NoError(t, err)

	// Verify destination file
	dstContent, err := os.ReadFile(dstPath)
	require.NoError(t, err)
	assert.Equal(t, len(content), len(dstContent))
	assert.Equal(t, content, dstContent)
}

func TestCopyFile_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create empty source file
	srcPath := filepath.Join(tmpDir, "empty.txt")
	err := os.WriteFile(srcPath, []byte{}, 0644)
	require.NoError(t, err)

	// Copy file
	dstPath := filepath.Join(tmpDir, "empty_copy.txt")
	err = CopyFile(srcPath, dstPath)
	assert.NoError(t, err)

	// Verify destination file is empty
	dstContent, err := os.ReadFile(dstPath)
	require.NoError(t, err)
	assert.Empty(t, dstContent)
}

func TestCopyFile_OverwriteExisting(t *testing.T) {
	tmpDir := t.TempDir()

	// Create source file
	srcPath := filepath.Join(tmpDir, "source.txt")
	srcContent := []byte("new content")
	err := os.WriteFile(srcPath, srcContent, 0644)
	require.NoError(t, err)

	// Create existing destination file
	dstPath := filepath.Join(tmpDir, "destination.txt")
	err = os.WriteFile(dstPath, []byte("old content"), 0644)
	require.NoError(t, err)

	// Copy should overwrite
	err = CopyFile(srcPath, dstPath)
	assert.NoError(t, err)

	// Verify destination has new content
	dstContent, err := os.ReadFile(dstPath)
	require.NoError(t, err)
	assert.Equal(t, srcContent, dstContent)
}
