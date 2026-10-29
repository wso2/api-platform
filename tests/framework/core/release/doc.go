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

// Package release computes the test framework's own release versioning: the next patch to cut
// when none is given explicitly, which floating tags (latest, the bare major, major.minor) a
// release should also receive, and which of a minor line's existing patches fall outside the
// retention limit and should be retired.
//
// It is pure — no git, network, or registry calls — so it is fully covered by ordinary unit
// tests that run in any pull request's CI, with no registry credentials required. cmd/release
// wraps it with the git and GHCR I/O a real release run needs.
package release
