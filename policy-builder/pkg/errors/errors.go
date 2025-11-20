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

// FatalError prints an error and exits with the appropriate code
func FatalError(err error) {
	if buildErr, ok := err.(*BuildError); ok {
		fmt.Fprintf(os.Stderr, "ERROR: %s\n", buildErr.Error())
		os.Exit(int(buildErr.ExitCode))
	}
	fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
	os.Exit(int(ExitUnknownError))
}
