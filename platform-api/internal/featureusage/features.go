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

package featureusage

// FeatureCreateRESTAPI is the taxonomy code for REST API creation ("REST API
// Creation"). It is reported once per call to a REST API route that creates
// an API — POST /rest-apis and POST /rest-apis/import-openapi — and once per
// REST API created with the MCP server's create_api tool, whether the
// creation succeeded or failed; the event's status and outcome tell which.
const FeatureCreateRESTAPI = "apip.manage-apis.rest-api-create"

// enabledFeatures is the explicit allowlist of taxonomy features this process
// reports. A code missing from it is refused even if the taxonomy has it, so
// instrumenting a new feature is a reviewed change here, not a side effect of
// a taxonomy update.
var enabledFeatures = map[string]bool{
	FeatureCreateRESTAPI: true,
}
