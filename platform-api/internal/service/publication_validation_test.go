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

	"github.com/wso2/api-platform/platform-api/internal/model"
)

func TestValidateDraftFieldLengths(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(p *model.Publication)
		wantErr string
	}{
		{"within limits", func(p *model.Publication) {}, ""},
		{"description at limit", func(p *model.Publication) { p.Description = strings.Repeat("a", 1023) }, ""},
		{"multibyte description at limit counts characters", func(p *model.Publication) { p.Description = strings.Repeat("é", 1023) }, ""},
		{"description over limit", func(p *model.Publication) { p.Description = strings.Repeat("a", 1024) }, "description must not exceed 1023 characters"},
		{"displayName over limit", func(p *model.Publication) { p.DisplayName = strings.Repeat("a", 256) }, "displayName must not exceed 255 characters"},
		{"version over limit", func(p *model.Publication) { p.Version = strings.Repeat("1", 31) }, "version must not exceed 30 characters"},
		{"productionUrl over limit", func(p *model.Publication) { p.ProductionURL = "https://" + strings.Repeat("a", 250) }, "productionUrl must not exceed 255 characters"},
		{"sandboxUrl over limit", func(p *model.Publication) { p.SandboxURL = "https://" + strings.Repeat("a", 250) }, "sandboxUrl must not exceed 255 characters"},
		{"businessOwner over limit", func(p *model.Publication) { p.BusinessOwner = strings.Repeat("a", 256) }, "businessOwner must not exceed 255 characters"},
		{"businessOwnerEmail over limit", func(p *model.Publication) { p.BusinessOwnerEmail = strings.Repeat("a", 250) + "@b.com" }, "businessOwnerEmail must not exceed 255 characters"},
		{"technicalOwner over limit", func(p *model.Publication) { p.TechnicalOwner = strings.Repeat("a", 256) }, "technicalOwner must not exceed 255 characters"},
		{"technicalOwnerEmail over limit", func(p *model.Publication) { p.TechnicalOwnerEmail = strings.Repeat("a", 250) + "@b.com" }, "technicalOwnerEmail must not exceed 255 characters"},
		{"owner fields at limit", func(p *model.Publication) {
			p.BusinessOwner = strings.Repeat("a", 255)
			p.TechnicalOwner = strings.Repeat("é", 255)
		}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &model.Publication{DisplayName: "Orders", Version: "1.0"}
			tt.mutate(p)
			err := validateDraftFieldLengths(p)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("got %v, want error containing %q", err, tt.wantErr)
			}
		})
	}
}
