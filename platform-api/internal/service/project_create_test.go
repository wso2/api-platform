/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
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

package service

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/internal/apperror"
)

func TestCreateProject_RejectsAnInvalidSuppliedHandle(t *testing.T) {
	for name, handle := range map[string]string{
		"uppercase":           "My-Project",
		"underscore":          "my_project",
		"leading hyphen":      "-my-project",
		"trailing hyphen":     "my-project-",
		"consecutive hyphens": "my--project",
		"too long":            strings.Repeat("a", 41),
		"too short":           "ab",
	} {
		t.Run(name, func(t *testing.T) {
			// The nil project repository is never reached: the handle is rejected first.
			svc := NewProjectService(nil, listProjectsOrgRepo{}, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

			_, err := svc.CreateProject(&api.CreateProjectRequest{DisplayName: "Project", Id: &handle}, "org-1", "actor")

			require.Error(t, err)
			assert.True(t, apperror.ValidationFailed.Is(err), "got %v", err)
		})
	}
}
