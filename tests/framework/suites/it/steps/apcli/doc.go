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

// Package apcli drives the ap CLI binary. It owns the session every CLI step package runs the
// binary through, and the steps for the CLI's gateway commands, which run against the
// topology's platform gateway.
//
// Each scenario owns an isolated CLI HOME and scratch workspace, created on first use and
// removed by the cleanup registry, so the gateways a scenario adds to the CLI configuration are
// private to it. Resources the CLI deploys to the platform gateway are registered for cleanup
// as soon as the CLI reports them applied.
package apcli
