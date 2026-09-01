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

// Package cleanup tracks resources created by a test runner and removes them during
// scenario and runner teardown.
//
// A Resource records its Kind, ID, and creating Actor. Deleters receive that actor
// information and are responsible for using the correct principal when removing the
// resource. A missing actor or resource ID is rejected at registration time.
//
// Resources are deleted in ascending Kind.Order. Resources of the same kind are deleted
// in reverse registration order; different kinds with the same order are deterministic by
// kind name. Registering the same kind and ID more than once is idempotent and emits a
// warning. A resource deleted explicitly by a test should be removed with Deregister so
// teardown does not attempt it again.
//
// Sweep is best effort: it attempts every pending resource, joins deletion errors, and
// logs failures with structured resource and attempt information. Retries are disabled by
// default. A suite can opt in through its YAML configuration:
//
// defaults:
//
//	cleanup:
//	  max-attempts: 3
//
// The value is the total number of attempts for one resource. Failed resources are requeued
// until that limit is reached; after the final failure they are removed from the active
// queue and the failure is returned. This prevents an endlessly failing deleter from
// creating an infinite teardown loop. Timestamps and retry state are included by slog's
// structured log output, including duplicate-registration and requeue-collision warnings.
//
// A Registry is installed in the runner context with Install. Register and Sweep use that
// context integration. The engine sweeps after every scenario and once more after the
// runner finishes, so setup fixtures can survive across scenarios while ordinary resources
// are cleaned up promptly. A missing registry is a wiring error; an invalid or nil registry
// cannot be installed.
//
// To add a resource type, define a Kind with an appropriate cleanup order, register each
// successfully created resource, and register a non-nil Deleter for the kind. The deleter
// may use any protocol required by that resource; cleanup does not assume every resource
// has a bearer-token REST delete endpoint.
package cleanup
