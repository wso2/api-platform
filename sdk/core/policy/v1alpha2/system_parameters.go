/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.com) All Rights Reserved.
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

package policyv1alpha2

const (
	// SystemParamConfigRefKey stores the config expression extracted from
	// wso2/defaultValue in policy systemParameters schemas.
	SystemParamConfigRefKey = "__wso2_internal_ref"

	// SystemParamDefaultValueKey stores the schema default value paired with
	// SystemParamConfigRefKey for runtime fallback on missing config keys.
	SystemParamDefaultValueKey = "__wso2_internal_default"
)
