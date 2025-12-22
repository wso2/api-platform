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

package types

// DockerBuildOptions contains options for building Docker images
type DockerBuildOptions struct {
	TempDir                string
	PolicyEngineBin        string
	Policies               []*DiscoveredPolicy
	PolicyEngineImage      string
	GatewayControllerImage string
	RouterImage            string
	ImageTag               string
	BuilderVersion         string
}

// DockerBuildResult contains the results of building Docker images
type DockerBuildResult struct {
	PolicyEngineImage      string
	GatewayControllerImage string
	RouterImage            string
	ManifestPath           string
	Success                bool
	Errors                 []error
}
