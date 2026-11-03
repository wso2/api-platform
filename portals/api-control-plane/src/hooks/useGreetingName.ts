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

import { HOME_GREETING_SLOT, type ApiControlPlaneGreetingSource } from '@/extensions';
import { usePort } from '@/hostPort';
import { useSlot } from '@/slots';

/**
 * The name to greet the signed-in user by: the first non-empty one a
 * `home.greeting` entry supplies, else `fallback`. Never throws and never
 * blanks the greeting — a source that fails or has no name leaves `fallback` in
 * place, so a deployment with no source, or a failing one, greets as before.
 */
export function useGreetingName(fallback: string): string {
  const port = usePort();
  const sources = useSlot<ApiControlPlaneGreetingSource>(HOME_GREETING_SLOT);
  for (const source of sources) {
    try {
      const name = source.resolveName(port)?.trim();
      if (name) return name;
    } catch {
      /* this source has no name; try the next */
    }
  }
  return fallback;
}
