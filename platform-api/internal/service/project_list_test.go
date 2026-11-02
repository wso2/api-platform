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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/repository"
)

type listProjectsRepo struct {
	repository.ProjectRepository
	got repository.ListOptions
}

func (r *listProjectsRepo) ListProjects(_ string, opts repository.ListOptions) ([]*model.Project, error) {
	r.got = opts
	return []*model.Project{{Handle: "default", Name: "Default"}}, nil
}

func (r *listProjectsRepo) CountProjects(string, string) (int, error) { return 7, nil }

type listProjectsOrgRepo struct {
	repository.OrganizationRepository
}

func (listProjectsOrgRepo) GetOrganizationByUUID(string) (*model.Organization, error) {
	return &model.Organization{Handle: "acme"}, nil
}

type noIdentityRepo struct {
	repository.UserIdentityMappingRepository
}

func (noIdentityRepo) GetSubsByUUIDs([]string) (map[string]string, error) { return nil, nil }

func TestListProjects_PassesThePageAndReturnsTheTotal(t *testing.T) {
	projects := &listProjectsRepo{}
	svc := NewProjectService(projects, listProjectsOrgRepo{}, nil, nil, nil, nil, nil,
		NewIdentityService(noIdentityRepo{}), nil)

	items, total, err := svc.ListProjects("org-1", 5, 10)
	require.NoError(t, err)

	assert.Equal(t, repository.ListOptions{Limit: 5, Offset: 10, SortOrder: "desc"}, projects.got)
	assert.Equal(t, 7, total)
	require.Len(t, items, 1)
	require.NotNil(t, items[0].Id)
	assert.Equal(t, "default", *items[0].Id)
}
