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

import type { Environment } from '../types';

/**
 * A build named for display. Build ids are already readable — the date the build
 * was prepared and that day's index — so this only labels one.
 */
export function buildLabel(buildId?: string): string {
  return buildId ? `Build ${buildId}` : '—';
}

/**
 * The distinct builds an environment's gateways are currently serving, newest id
 * last. Usually one; more than one means the environment's gateways were deployed
 * to separately, and a promotion out of it has to say which build it carries —
 * exactly the choice the API requires.
 */
export function buildsRunningIn(environment?: Environment): string[] {
  const seen = new Set<string>();
  for (const gateway of environment?.gateways ?? []) {
    if (gateway.status === 'DEPLOYED' && gateway.buildId) {
      seen.add(gateway.buildId);
    }
  }
  return [...seen].sort();
}
