/*
 *  Copyright (c) 2025, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 */

package constants

import "errors"

var (
	ErrHandleExists          = errors.New("handle already exists")
	ErrOrganizationExists    = errors.New("organization already exists with the given UUID")
	ErrInvalidHandle         = errors.New("invalid handle format")
	ErrOrganizationNotFound  = errors.New("organization not found")
	ErrMultipleOrganizations = errors.New("multiple organizations found")
)

var (
	ErrProjectExists                         = errors.New("project already exists in organization")
	ErrProjectNotFound                       = errors.New("project not found")
	ErrInvalidProjectName                    = errors.New("invalid project name")
	ErrOrganizationMustHAveAtLeastOneProject = errors.New("organization must have at least one project")
	ErrProjectHasAssociatedAPIs              = errors.New("project has associated APIs")
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
