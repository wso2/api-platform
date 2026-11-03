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

// Package a2aagent declares the A2A trip-planner component: a real A2A 1.0 agent built on the
// reference SDK, serving both HTTP bindings and a public Agent Card. It is the upstream behind
// every Agent proxy the integration suites deploy through the control plane.
//
// The agent keeps its tasks in memory, so it is a per-block component rather than a shared one:
// a task created by one block must never be observable from another.
package a2aagent
