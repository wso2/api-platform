/*
 *  Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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
 */

// Package conformance hosts the Gateway API conformance runner for the WSO2 API
// Platform gateway. The actual entrypoint lives in conformance_test.go, which is
// guarded by the `conformance` build tag and invoked via run-conformance.sh. This
// file exists so the package is non-empty even when that tag is absent.
package conformance
