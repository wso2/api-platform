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

import { aPublication, aPublicationDraftDetails, aRestApi } from '@/test/msw';
import { draftFormValuesToInput, emptyDraftFormValues, resolveDraftFormValues } from './publicationForm';

const api = aRestApi({
  description: 'Manage loans.',
  displayName: 'Loans',
  upstream: { main: { url: 'https://backend.internal' }, sandbox: { url: 'https://sandbox.backend.internal' } },
  version: '2.0.0',
});

describe('resolveDraftFormValues', () => {
  it('prefers the draft, then the publication', () => {
    const draft = aPublicationDraftDetails({
      endpoints: { productionUrl: 'https://draft.example.com', sandboxUrl: 'https://draft-sandbox.example.com' },
    });
    const publication = aPublication({ endpoints: { productionUrl: 'https://live.example.com' } });

    expect(resolveDraftFormValues(draft, publication, api)).toMatchObject({
      productionUrl: 'https://draft.example.com',
      sandboxUrl: 'https://draft-sandbox.example.com',
    });
    expect(resolveDraftFormValues(undefined, publication, api)).toMatchObject({
      productionUrl: 'https://live.example.com',
      sandboxUrl: '',
    });
  });

  it("falls back to the API's own name, version, description and backend URLs", () => {
    expect(resolveDraftFormValues(undefined, undefined, api)).toEqual({
      description: 'Manage loans.',
      displayName: 'Loans',
      productionUrl: 'https://backend.internal',
      sandboxUrl: 'https://sandbox.backend.internal',
      version: '2.0.0',
      agentVisibility: 'VISIBLE',
    });
  });

  it('is empty when there is nothing at all', () => {
    expect(resolveDraftFormValues(undefined, undefined, undefined)).toBe(emptyDraftFormValues);
  });

  it('reads the agent visibility from the draft, then the publication, and defaults to VISIBLE', () => {
    const draft = aPublicationDraftDetails({ agentVisibility: 'HIDDEN' });
    const publication = aPublication({ agentVisibility: 'HIDDEN' });

    expect(resolveDraftFormValues(draft, undefined, api).agentVisibility).toBe('HIDDEN');
    expect(resolveDraftFormValues(undefined, publication, api).agentVisibility).toBe('HIDDEN');
    expect(resolveDraftFormValues(aPublicationDraftDetails(), undefined, api).agentVisibility).toBe('VISIBLE');
    expect(resolveDraftFormValues(undefined, undefined, api).agentVisibility).toBe('VISIBLE');
  });
});

describe('draftFormValuesToInput', () => {
  it('sends the agent visibility with the other details', () => {
    expect(draftFormValuesToInput({ ...emptyDraftFormValues, agentVisibility: 'HIDDEN' })).toMatchObject({
      agentVisibility: 'HIDDEN',
    });
  });
});
