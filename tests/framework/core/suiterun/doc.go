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

// Package suiterun runs a declarative suite from a Go test binary. A project supplies its
// step definitions and cleanup handlers through Options and calls Main from TestMain and Run
// from a test; flag parsing, topology loading, image builds, selection, coverage and log
// capture are handled here. The framework's own suite and a suite in another repository
// that imports the framework as a module use the same entry points.
package suiterun
