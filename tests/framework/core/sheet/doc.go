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
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

// Package sheet ties product scenarios to the IDs of an external test-scenario sheet
// (for example AIW-PRV-001) and records a result for every ID a run touches.
//
// A scenario names the sheet IDs it covers as tags (@AIW-PRV-001). A scenario that
// covers several IDs — a UI journey — marks where each ID's checks begin with the
// step `sheet check "AIW-PRV-001"`, so a failure is attributed to the ID whose
// checks were running and every later ID is reported as not reached. The Recorder
// binds that step and the per-step bookkeeping to each runner, and a Sink writes one
// JSON report per runner for cmd/sheetreport to merge with the sheet manifests.
package sheet
