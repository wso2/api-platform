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

package utils

import (
	"testing"

	commonconstants "github.com/wso2/api-platform/common/constants"
	"github.com/wso2/api-platform/platform-api/internal/dto"
)

func TestResolveImportProject(t *testing.T) {
	const (
		projectUUID = "019feb20-bd8f-74f1-9489-8814a129cd80"
		handle      = "new-project"
	)

	t.Run("prefers project-handle over project-id uuid", func(t *testing.T) {
		got := ResolveImportProject(dto.ArtifactImportMetadata{
			Annotations: map[string]string{
				commonconstants.AnnotationProjectHandle: handle,
				commonconstants.AnnotationProjectID:     projectUUID,
			},
		})
		if got != handle {
			t.Fatalf("ResolveImportProject() = %q, want handle %q", got, handle)
		}
	})

	t.Run("falls back to project-id for older artifacts", func(t *testing.T) {
		got := ResolveImportProject(dto.ArtifactImportMetadata{
			Annotations: map[string]string{
				commonconstants.AnnotationProjectID: handle,
			},
		})
		if got != handle {
			t.Fatalf("ResolveImportProject() = %q, want project-id fallback %q", got, handle)
		}
	})

	t.Run("falls back to deprecated label", func(t *testing.T) {
		got := ResolveImportProject(dto.ArtifactImportMetadata{
			Labels: map[string]string{
				commonconstants.DeprecatedLabelProjectID: handle,
			},
		})
		if got != handle {
			t.Fatalf("ResolveImportProject() = %q, want label fallback %q", got, handle)
		}
	})

	t.Run("trims padded handle", func(t *testing.T) {
		got := ResolveImportProject(dto.ArtifactImportMetadata{
			Annotations: map[string]string{
				commonconstants.AnnotationProjectHandle: "  " + handle + "  ",
				commonconstants.AnnotationProjectID:     projectUUID,
			},
		})
		if got != handle {
			t.Fatalf("ResolveImportProject() = %q, want trimmed handle", got)
		}
	})

	t.Run("whitespace-only handle falls back to project-id", func(t *testing.T) {
		got := ResolveImportProject(dto.ArtifactImportMetadata{
			Annotations: map[string]string{
				commonconstants.AnnotationProjectHandle: " \t ",
				commonconstants.AnnotationProjectID:     handle,
			},
		})
		if got != handle {
			t.Fatalf("ResolveImportProject() = %q, want project-id fallback", got)
		}
	})
}
