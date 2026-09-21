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

package main

// The per-row transforms now live in the shared migrationcore package (the ONE
// implementation reused by the live dual-write path). These file-local aliases keep
// the batch call sites unchanged.
import "github.com/wso2/api-platform/platform-api/migrationcore"

var (
	reinterpretTZ         = migrationcore.ReinterpretTZ
	boolToSmallint        = migrationcore.BoolToSmallint
	truncateStr           = migrationcore.TruncateStr
	slug                  = migrationcore.Slug
	parseTransport        = migrationcore.ParseTransport
	topLevelUnknownFields = migrationcore.TopLevelUnknownFields
	reshapeRestAPIConfig  = migrationcore.ReshapeRestAPIConfig
	reshapeWebSubConfig   = migrationcore.ReshapeWebSubConfig
	preprocessWebSubRaw   = migrationcore.PreprocessWebSubRaw
	reshapeWebBrokerConfig = migrationcore.ReshapeWebBrokerConfig
	remarshalConfig       = migrationcore.RemarshalConfig
	caseConvertThrottleUnit = migrationcore.CaseConvertThrottleUnit
)
