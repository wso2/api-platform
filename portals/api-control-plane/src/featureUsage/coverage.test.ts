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

import { describe, expect, it } from 'vitest';

import { API_PLATFORM_FEATURES, policyFeatureOf, type ApiPlatformFeatureCode } from './taxonomy';
import taxonomyYaml from './api-platform-taxonomy.yaml?raw';

/*
 * A console feature in the taxonomy is only measured once some UI action emits
 * it. This fails when a feature is added to api-platform-taxonomy.yaml (with
 * `portal: api-control-plane`) but no console code reports it, so a taxonomy
 * entry never ships without its instrumentation.
 */

/** The console's own source, without tests, the src/test helpers or the taxonomy module. */
const SOURCES = Object.entries(
  import.meta.glob<string>('../**/*.{ts,tsx}', { eager: true, import: 'default', query: '?raw' }),
)
  // Files in this directory are keyed `./name.ts`, elsewhere `../dir/name.ts`.
  .filter(([file]) => !/\.test\.tsx?$|^\.\.\/test\/|(^|\/)taxonomy(\.generated)?\.ts$/.test(file))
  .map(([, text]) => text);

/** Every `apip.…` string literal in the console's source. */
const referencedCodes = new Set(
  SOURCES.flatMap((text) => [...text.matchAll(/['"`](apip\.[a-z0-9.-]+)['"`]/g)].map((m) => m[1])),
);

/** Policy names the taxonomy classifies, each reported through trackPolicyAttachment. */
const policyNames = [...taxonomyYaml.matchAll(/values: \[([^\]]*)\]/g)]
  .flatMap((m) => [...m[1].matchAll(/"([a-z0-9-]+)"/g)].map((v) => v[1]))
  .concat(['analytics-header-filter', 'cors', 'subscription-validation']);
const reportsPolicies = SOURCES.some((text) => text.includes('trackPolicyAttachment('));
const policyCodes = new Set<string>(
  reportsPolicies ? policyNames.map(policyFeatureOf).filter((code) => code !== undefined) : [],
);

describe('instrumentation coverage', () => {
  it.each(Object.keys(API_PLATFORM_FEATURES) as ApiPlatformFeatureCode[])(
    '%s is reported by some console code',
    (code) => {
      expect(referencedCodes.has(code) || policyCodes.has(code)).toBe(true);
    },
  );
});
