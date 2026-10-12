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

package framework

import "embed"

// Assets holds the repository files the engine reads when it runs without an api-platform
// checkout. Files under the module keep their module-relative path; snapshots of files that
// live elsewhere in the repository sit under snapshots/ at their repository-relative path. It
// carries no key material, which the framework generates per run. The core/assets package
// resolves repository paths against it.
//
//go:embed core/catalog/overlays core/catalog/platformgateway/docker-compose.yaml
//go:embed core/catalog/platformgateway/resources all:snapshots
var Assets embed.FS
