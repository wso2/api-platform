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

import type { CreateEnvironmentInput, Environment } from '../types';

let environments: Environment[] = [
  { id: 'development', name: 'Development', critical: false, createdAt: '2023-09-01T00:00:00.000Z' },
  { id: 'production', name: 'Production', critical: true, createdAt: '2023-09-01T00:00:00.000Z' },
];

export function listEnvironments(): Environment[] {
  return environments;
}

export function createEnvironment(input: CreateEnvironmentInput): Environment {
  const environment: Environment = {
    id: `environment-${Date.now()}`,
    ...input,
    createdAt: new Date().toISOString(),
  };
  environments = [...environments, environment];
  return environment;
}

export function deleteEnvironment(id: string): void {
  environments = environments.filter((environment) => environment.id !== id);
}
