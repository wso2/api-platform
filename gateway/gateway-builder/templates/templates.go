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

package templates

import (
	_ "embed"
)

// Embedded template files
// These files are embedded at compile time from the same directory
// The actual .tmpl files remain in the directory for easy editing

//go:embed plugin_registry.go.tmpl
var PluginRegistryTemplate string

//go:embed build_info.go.tmpl
var BuildInfoTemplate string

//go:embed Dockerfile.policy-engine.tmpl
var DockerfilePolicyEngineTmpl string

//go:embed Dockerfile.gateway-controller.tmpl
var DockerfileGatewayControllerTemplate string
