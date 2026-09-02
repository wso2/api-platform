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

// An in-memory EnvironmentPort. This is the only EnvironmentPort that exists
// today — there is no real backend yet, so EnvironmentsFeature constructs one
// of these per mount. Swap in a real, BFF-backed implementation later without
// touching EnvironmentsList/EnvironmentForm, which only ever see the
// EnvironmentPort interface.

import { createEnvironment, deleteEnvironment, listEnvironments } from './mocks/environmentsStore';
import type { CreateEnvironmentInput, Environment, EnvironmentPort } from './types';

export function createMockEnvironmentPort(): EnvironmentPort {
  return {
    async list(): Promise<Environment[]> {
      return listEnvironments();
    },
    async create(input: CreateEnvironmentInput): Promise<Environment> {
      return createEnvironment(input);
    },
    async remove(id: string): Promise<void> {
      deleteEnvironment(id);
    },
  };
}
