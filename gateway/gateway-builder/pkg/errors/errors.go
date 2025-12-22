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

package errors

import (
	"fmt"
	"os"
)

// ExitCode represents different error categories
type ExitCode int

const (
	ExitSuccess ExitCode = 0
	ExitDiscoveryError ExitCode = 10
	ExitValidationError ExitCode = 20
	ExitGenerationError ExitCode = 30
	ExitCompilationError ExitCode = 40
	ExitPackagingError ExitCode = 50
	ExitDockerError ExitCode = 60
	ExitUnknownError ExitCode = 99
)

// BuildError represents a structured error during the build process
type BuildError struct {
	Phase   string
	Message string
	Err     error
	ExitCode ExitCode
}

func (e *BuildError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Phase, e.Message, e.Err)
	}
	return fmt.Sprintf("[%s] %s", e.Phase, e.Message)
}

func (e *BuildError) Unwrap() error {
	return e.Err
}

// NewDiscoveryError creates a discovery phase error
func NewDiscoveryError(message string, err error) *BuildError {
	return &BuildError{
		Phase:    "Discovery",
		Message:  message,
		Err:      err,
		ExitCode: ExitDiscoveryError,
	}
}

// NewValidationError creates a validation phase error
func NewValidationError(message string, err error) *BuildError {
	return &BuildError{
		Phase:    "Validation",
		Message:  message,
		Err:      err,
		ExitCode: ExitValidationError,
	}
}

// NewGenerationError creates a code generation phase error
func NewGenerationError(message string, err error) *BuildError {
	return &BuildError{
		Phase:    "Generation",
		Message:  message,
		Err:      err,
		ExitCode: ExitGenerationError,
	}
}

// NewCompilationError creates a compilation phase error
func NewCompilationError(message string, err error) *BuildError {
	return &BuildError{
		Phase:    "Compilation",
		Message:  message,
		Err:      err,
		ExitCode: ExitCompilationError,
	}
}

// NewPackagingError creates a packaging phase error
func NewPackagingError(message string, err error) *BuildError {
	return &BuildError{
		Phase:    "Packaging",
		Message:  message,
		Err:      err,
		ExitCode: ExitPackagingError,
	}
}

// NewDockerError creates a Docker build phase error
func NewDockerError(message string, err error) *BuildError {
	return &BuildError{
		Phase:    "Docker Build",
		Message:  message,
		Err:      err,
		ExitCode: ExitDockerError,
	}
}

// FatalError prints an error and exits with the appropriate code
func FatalError(err error) {
	if buildErr, ok := err.(*BuildError); ok {
		fmt.Fprintf(os.Stderr, "ERROR: %s\n", buildErr.Error())
		os.Exit(int(buildErr.ExitCode))
	}
	fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
	os.Exit(int(ExitUnknownError))
}
