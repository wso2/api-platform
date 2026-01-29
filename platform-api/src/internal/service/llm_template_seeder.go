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
	"fmt"

	"platform-api/src/internal/model"
	"platform-api/src/internal/repository"
)

// LLMTemplateSeeder seeds a set of default LLM provider templates into the DB
// for a given organization. This is used to avoid "template not found" when
// creating LLM providers.
//
// Seeding is idempotent: existing templates are not overwritten.
type LLMTemplateSeeder struct {
	repo      repository.LLMProviderTemplateRepository
	templates []*model.LLMProviderTemplate
}

func NewLLMTemplateSeeder(repo repository.LLMProviderTemplateRepository, templates []*model.LLMProviderTemplate) *LLMTemplateSeeder {
	return &LLMTemplateSeeder{repo: repo, templates: templates}
}

func (s *LLMTemplateSeeder) SeedForOrg(orgUUID string) error {
	if s == nil || s.repo == nil {
		return nil
	}
	if orgUUID == "" {
		return fmt.Errorf("orgUUID is empty")
	}
	if len(s.templates) == 0 {
		return nil
	}

	totalCount, err := s.repo.Count(orgUUID)
	if err != nil {
		return fmt.Errorf("failed to count existing templates: %w", err)
	}
	existing, err := s.repo.List(orgUUID, totalCount, 0)
	if err != nil {
		return fmt.Errorf("failed to list existing templates: %w", err)
	}
	existingByID := make(map[string]struct{}, len(existing))
	existingByHandle := make(map[string]*model.LLMProviderTemplate, len(existing))
	for _, t := range existing {
		if t == nil {
			continue
		}
		existingByID[t.ID] = struct{}{}
		existingByHandle[t.ID] = t
	}

	for _, tpl := range s.templates {
		if tpl == nil || tpl.ID == "" {
			continue
		}
		if _, ok := existingByID[tpl.ID]; ok {
			current := existingByHandle[tpl.ID]
			if current != nil && current.Metadata == nil && tpl.Metadata != nil {
				current.Metadata = tpl.Metadata
				if current.Name == "" {
					current.Name = tpl.Name
				}
				if err := s.repo.Update(current); err != nil {
					return fmt.Errorf("failed to update template metadata for %s: %w", tpl.ID, err)
				}
			}
			continue
		}

		toCreate := &model.LLMProviderTemplate{
			OrganizationUUID: orgUUID,
			ID:               tpl.ID,
			Name:             tpl.Name,
			Description:      tpl.Description,
			CreatedBy:        tpl.CreatedBy,
			Metadata:         tpl.Metadata,
			PromptTokens:     tpl.PromptTokens,
			CompletionTokens: tpl.CompletionTokens,
			TotalTokens:      tpl.TotalTokens,
			RemainingTokens:  tpl.RemainingTokens,
			RequestModel:     tpl.RequestModel,
			ResponseModel:    tpl.ResponseModel,
		}
		if err := s.repo.Create(toCreate); err != nil {
			// Be tolerant to concurrent startup / repeated seeding.
			exists, existsErr := s.repo.Exists(tpl.ID, orgUUID)
			if existsErr == nil && exists {
				continue
			}
			return fmt.Errorf("failed to create template %s: %w", tpl.ID, err)
		}
	}

	return nil
}
