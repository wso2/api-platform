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

package hub

// V1 Policy Contract — supported and unsupported actions for the event gateway.
//
// Supported:
//   - Header mutations (set/remove)
//   - Body mutations (replace content)
//   - CEL execution conditions
//   - Immediate short-circuit (drop message / reject subscription)
//
// Unsupported (validation error unless connector explicitly maps):
//   - Path/host/query rewrites
//   - Upstream name changes
//   - Response status code replacement
//   - Dynamic metadata

// ActionSupported indicates whether the given action type is supported.
var ActionSupported = map[string]bool{
	"headers_to_set":     true,
	"headers_to_remove":  true,
	"body":               true,
	"immediate_response": true,
}

// ActionUnsupported lists HTTP-only action types that are validation errors
// in the event-gateway context.
var ActionUnsupported = map[string]string{
	"path":                       "path rewrite has no meaning in event context",
	"host":                       "host rewrite has no meaning in event context",
	"method":                     "method rewrite has no meaning in event context",
	"upstream_name":              "upstream name change has no meaning in event context",
	"query_parameters_to_add":    "query parameter mutation has no meaning in event context",
	"query_parameters_to_remove": "query parameter removal has no meaning in event context",
	"status_code":                "status code replacement has no meaning in event context",
	"dynamic_metadata":           "dynamic metadata is not supported in event context",
}
