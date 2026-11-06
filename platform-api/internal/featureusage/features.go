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
// an API — POST /rest-apis and POST /rest-apis/import-openapi — whether the
// call succeeded or failed; the event's response status tells which.
const FeatureCreateRESTAPI = "apip.manage-apis.create-rest-api"

// enabledFeatures is the explicit allowlist of taxonomy features this process
// reports. A code missing from it is refused even if the taxonomy has it, so
// instrumenting a new feature is a reviewed change here, not a side effect of
// a taxonomy update.
var enabledFeatures = map[string]bool{
	FeatureCreateRESTAPI: true,
}

// CreationMethod is the creation_method dimension of FeatureCreateRESTAPI.
type CreationMethod string

const (
	// CreationMethodScratch is an API the user started from a blank definition
	// in the API Control Plane console's "Start from scratch" approach; the
	// console declares it on the import-openapi call.
	CreationMethodScratch CreationMethod = "scratch"
	// CreationMethodContract is every other creation: the console's "From
	// contract" approach and any direct Platform API or CLI call, which always
	// supplies the definition itself.
	CreationMethodContract CreationMethod = "contract"
)

// The dimensions of FeatureCreateRESTAPI, as the taxonomy names them.
const (
	dimCreationMethod     = "creation_method"
	dimHasSandboxUpstream = "has_sandbox_upstream"
)

// RESTAPICreationDefaults are the dimensions every FeatureCreateRESTAPI call
// starts with: "contract", unless the console declares "scratch".
func RESTAPICreationDefaults() map[string]any {
	return map[string]any{dimCreationMethod: string(CreationMethodContract)}
}

// SetCreationMethod records the creation_method dimension.
func (c *Call) SetCreationMethod(method CreationMethod) {
	c.SetDimension(dimCreationMethod, string(method))
}

// SetHasSandboxUpstream records the has_sandbox_upstream dimension.
func (c *Call) SetHasSandboxUpstream(hasSandbox bool) {
	c.SetDimension(dimHasSandboxUpstream, hasSandbox)
}
