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

package errors

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildError_Error_WithWrappedError(t *testing.T) {
	wrappedErr := errors.New("underlying error")
	buildErr := &BuildError{
		Phase:    "Test",
		Message:  "something failed",
		Err:      wrappedErr,
		ExitCode: ExitUnknownError,
	}

	result := buildErr.Error()
	assert.Equal(t, "[Test] something failed: underlying error", result)
}

func TestBuildError_Error_WithoutWrappedError(t *testing.T) {
	buildErr := &BuildError{
		Phase:    "Test",
		Message:  "something failed",
		Err:      nil,
		ExitCode: ExitUnknownError,
	}

	result := buildErr.Error()
	assert.Equal(t, "[Test] something failed", result)
}

func TestBuildError_Unwrap(t *testing.T) {
	wrappedErr := errors.New("underlying error")
	buildErr := &BuildError{
		Phase:    "Test",
		Message:  "something failed",
		Err:      wrappedErr,
		ExitCode: ExitUnknownError,
	}

	result := buildErr.Unwrap()
	assert.Equal(t, wrappedErr, result)
}

func TestBuildError_Unwrap_NilError(t *testing.T) {
	buildErr := &BuildError{
		Phase:    "Test",
		Message:  "something failed",
		Err:      nil,
		ExitCode: ExitUnknownError,
	}

	result := buildErr.Unwrap()
	assert.Nil(t, result)
}

func TestNewDiscoveryError(t *testing.T) {
	wrappedErr := errors.New("file not found")
	err := NewDiscoveryError("failed to discover policies", wrappedErr)

	assert.Equal(t, "Discovery", err.Phase)
	assert.Equal(t, "failed to discover policies", err.Message)
	assert.Equal(t, wrappedErr, err.Err)
	assert.Equal(t, ExitDiscoveryError, err.ExitCode)
}

func TestNewDiscoveryError_NilError(t *testing.T) {
	err := NewDiscoveryError("no policies found", nil)

	assert.Equal(t, "Discovery", err.Phase)
	assert.Equal(t, "no policies found", err.Message)
	assert.Nil(t, err.Err)
	assert.Equal(t, ExitDiscoveryError, err.ExitCode)
}

func TestNewValidationError(t *testing.T) {
	wrappedErr := errors.New("schema invalid")
	err := NewValidationError("policy validation failed", wrappedErr)

	assert.Equal(t, "Validation", err.Phase)
	assert.Equal(t, "policy validation failed", err.Message)
	assert.Equal(t, wrappedErr, err.Err)
	assert.Equal(t, ExitValidationError, err.ExitCode)
}

func TestNewGenerationError(t *testing.T) {
	wrappedErr := errors.New("template error")
	err := NewGenerationError("code generation failed", wrappedErr)

	assert.Equal(t, "Generation", err.Phase)
	assert.Equal(t, "code generation failed", err.Message)
	assert.Equal(t, wrappedErr, err.Err)
	assert.Equal(t, ExitGenerationError, err.ExitCode)
}

func TestNewCompilationError(t *testing.T) {
	wrappedErr := errors.New("go build failed")
	err := NewCompilationError("compilation failed", wrappedErr)

	assert.Equal(t, "Compilation", err.Phase)
	assert.Equal(t, "compilation failed", err.Message)
	assert.Equal(t, wrappedErr, err.Err)
	assert.Equal(t, ExitCompilationError, err.ExitCode)
}

func TestNewPackagingError(t *testing.T) {
	wrappedErr := errors.New("dockerfile error")
	err := NewPackagingError("packaging failed", wrappedErr)

	assert.Equal(t, "Packaging", err.Phase)
	assert.Equal(t, "packaging failed", err.Message)
	assert.Equal(t, wrappedErr, err.Err)
	assert.Equal(t, ExitPackagingError, err.ExitCode)
}

func TestNewDockerError(t *testing.T) {
	wrappedErr := errors.New("docker build failed")
	err := NewDockerError("docker operation failed", wrappedErr)

	assert.Equal(t, "Docker Build", err.Phase)
	assert.Equal(t, "docker operation failed", err.Message)
	assert.Equal(t, wrappedErr, err.Err)
	assert.Equal(t, ExitDockerError, err.ExitCode)
}

func TestExitCodes(t *testing.T) {
	// Verify exit code values
	assert.Equal(t, ExitCode(0), ExitSuccess)
	assert.Equal(t, ExitCode(10), ExitDiscoveryError)
	assert.Equal(t, ExitCode(20), ExitValidationError)
	assert.Equal(t, ExitCode(30), ExitGenerationError)
	assert.Equal(t, ExitCode(40), ExitCompilationError)
	assert.Equal(t, ExitCode(50), ExitPackagingError)
	assert.Equal(t, ExitCode(60), ExitDockerError)
	assert.Equal(t, ExitCode(99), ExitUnknownError)
}

func TestBuildError_ErrorsIs(t *testing.T) {
	originalErr := errors.New("original error")
	buildErr := NewDiscoveryError("wrapped", originalErr)

	// errors.Is should work with Unwrap
	assert.True(t, errors.Is(buildErr, originalErr))
}

func TestBuildError_ErrorsAs(t *testing.T) {
	originalErr := errors.New("original error")
	buildErr := NewValidationError("wrapped", originalErr)

	// errors.As should work
	var target *BuildError
	assert.True(t, errors.As(buildErr, &target))
	assert.Equal(t, "Validation", target.Phase)
}
