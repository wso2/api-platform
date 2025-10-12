package constants

import "errors"

var (
	ErrHandleExists          = errors.New("handle already exists")
	ErrInvalidHandle         = errors.New("invalid handle format")
	ErrOrganizationNotFound  = errors.New("organization not found")
	ErrMultipleOrganizations = errors.New("multiple organizations found")
)

var (
	ErrProjectNameExists           = errors.New("project name already exists in organization")
	ErrProjectNotFound             = errors.New("project not found")
	ErrInvalidProjectName          = errors.New("invalid project name")
	ErrDefaultProjectAlreadyExists = errors.New("default project for organization already exists")
)

var (
	ErrAPINotFound           = errors.New("api not found")
	ErrAPIAlreadyExists      = errors.New("api already exists in project")
	ErrInvalidAPIContext     = errors.New("invalid api context format")
	ErrInvalidAPIVersion     = errors.New("invalid api version format")
	ErrInvalidAPIName        = errors.New("invalid api name format")
	ErrInvalidLifecycleState = errors.New("invalid lifecycle state")
	ErrInvalidAPIType        = errors.New("invalid api type")
	ErrInvalidTransport      = errors.New("invalid transport protocol")
)
