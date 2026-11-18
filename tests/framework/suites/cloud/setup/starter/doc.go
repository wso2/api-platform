/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except in compliance
 * with the License. You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

// Package starter makes sure the shared organisation holds what a cloud suite run needs.
//
// The finalizer package removes what a run left behind; a Starter is its counterpart at the
// other end of the run. It holds the prerequisites of an organisation, such as the gateway the
// scenarios deploy to, and prepares each of them once, however many scenarios and runners ask for
// it: the prerequisite is built when it is missing and is only checked when it cannot be built. A
// failed preparation is shared by every caller instead of being tried again, except one that failed
// because its own caller's context expired or was cancelled: that says nothing about the
// prerequisite, so the next caller prepares it again. A prerequisite outlives the run: it carries
// no runner suffix, so the finalizer never sweeps it. The cloud suite's step packages supply the
// Prerequisites of the platform they drive.
package starter
