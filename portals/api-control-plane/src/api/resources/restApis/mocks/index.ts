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

/**
 * Bundled, verbatim OpenAPI samples used until platform-api serves definitions
 * (see `../definition.endpoints.ts`). They are loaded separately via dynamic
 * imports to preserve code-splitting. Reading List and Petstore support live
 * execution; the larger TrackSynq document is intended for rendering tests.
 */

import type { OpenApiDocument } from '../restApis.utils';

export type { OpenApiDocument };

/**
 * Identifies one bundled sample. A union rather than `string` so a typo in a
 * forced selection is a compile error, not a silent fallback at runtime.
 */
export type SampleDefinitionId = 'readingList' | 'petstore' | 'trackSynq';

/** Catalog of lazy loaders, preserving code-splitting for bundled JSON. */
const SAMPLE_LOADERS: Record<SampleDefinitionId, () => Promise<OpenApiDocument>> = {
  readingList: () =>
    import('./readingListApi.openapi.json').then((module) => module.default as OpenApiDocument),
  petstore: () =>
    import('./petstoreApi.openapi.json').then((module) => module.default as OpenApiDocument),
  trackSynq: () =>
    import('./quantumInventionsTrackSynqApi.openapi.json').then(
      (module) => module.default as OpenApiDocument,
    ),
};

/** The documents in the catalog. Declaration order is the selection order. */
export const SAMPLE_DEFINITION_IDS = Object.keys(SAMPLE_LOADERS) as SampleDefinitionId[];

/** The outcome for an API without a stored definition. */
export const NO_SAMPLE_DEFINITION = 'none' as const;

/** A document in the catalog, or the absence of one. */
export type SampleDefinitionChoice = SampleDefinitionId | typeof NO_SAMPLE_DEFINITION;

/** All possible per-API selections, including no definition. */
export const SAMPLE_DEFINITION_CHOICES: readonly SampleDefinitionChoice[] = [
  ...SAMPLE_DEFINITION_IDS,
  NO_SAMPLE_DEFINITION,
];

/** Picks a stable, varied outcome for each API using an FNV-1a hash. */
export const sampleDefinitionIdFor = (restApiId: string): SampleDefinitionChoice => {
  let hash = 0x51879dc2;
  for (let index = 0; index < restApiId.length; index += 1) {
    hash ^= restApiId.charCodeAt(index);
    hash = Math.imul(hash, 0x01000193) >>> 0;
  }
  return SAMPLE_DEFINITION_CHOICES[hash % SAMPLE_DEFINITION_CHOICES.length];
};

/** Loads one sample document by id. */
export const loadSampleDefinition = (id: SampleDefinitionId): Promise<OpenApiDocument> =>
  SAMPLE_LOADERS[id]();
