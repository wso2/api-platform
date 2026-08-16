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

package apiportal

import (
	"path/filepath"

	"github.com/wso2/api-platform/tests/framework/core/builder"
)

// BuildSpec returns the source-build metadata for API Portal.
func BuildSpec(version string) (builder.Spec, error) {
	return builder.Spec{
		Component:        "api-portal",
		SourceDir:        "portals/api-portal",
		SupportsCoverage: false,
		Images: []builder.Image{{
			Name:       "ghcr.io/wso2/api-platform/api-portal:" + version,
			Dockerfile: "portals/api-portal/Dockerfile",
			Context:    "portals/api-portal",
		}},
		Plan: func(repoRoot, v string, _ bool) ([]builder.Command, error) {
			return []builder.Command{{
				Directory: filepath.Join(repoRoot, "portals/api-portal"),
				Args: []string{"docker", "buildx", "build", "--tag",
					"ghcr.io/wso2/api-platform/api-portal:" + v, "--load", "."},
			}}, nil
		},
	}, nil
}
