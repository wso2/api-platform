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

import { aGateway, aPublication, aPublicationDraftDetails, aRestApi } from '@/test/msw';
import {
  buildProductionUrlOptions,
  deletedPlansOf,
  draftFormValuesToInput,
  editedPlanIdsOf,
  emptyDraftFormValues,
  resolveDraftFormValues,
  sameSelection,
} from './publicationForm';

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

  it("falls back to the API's own name, version, description, leaving both URLs empty", () => {
    expect(resolveDraftFormValues(undefined, undefined, api)).toEqual({
      description: 'Manage loans.',
      displayName: 'Loans',
      productionUrl: '',
      sandboxUrl: '',
      version: '2.0.0',
      agentVisibility: 'VISIBLE',
      subscriptionPlanIds: [],
      docIds: [],
    });
  });

  it('reads the selected documents from the draft, then the publication', () => {
    const draft = aPublicationDraftDetails({ docIds: ['guide'] });
    const publication = aPublication({ docIds: ['faq'] });

    expect(resolveDraftFormValues(draft, publication, api).docIds).toEqual(['guide']);
    expect(resolveDraftFormValues(undefined, publication, api).docIds).toEqual(['faq']);
    expect(resolveDraftFormValues(aPublicationDraftDetails({ docIds: undefined }), undefined, api).docIds).toEqual([]);
  });

  it('defaults the Production URL to the given gateway URL', () => {
    expect(resolveDraftFormValues(undefined, undefined, api, 'https://gw.test/loans').productionUrl).toBe(
      'https://gw.test/loans',
    );
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

describe('deleted plans', () => {
  const selections = [
    { deleted: false, displayName: 'Gold', edited: false, id: 'gold' },
    { deleted: true, displayName: 'Platinum', edited: false, id: 'platinum' },
  ];

  it('leaves a deleted plan out of the form, whichever tier it is read from', () => {
    const draft = aPublicationDraftDetails({ subscriptionPlanIds: ['gold', 'platinum'], subscriptionPlans: selections });
    const publication = aPublication({ subscriptionPlanIds: ['gold', 'platinum'], subscriptionPlans: selections });

    expect(resolveDraftFormValues(draft, undefined, undefined).subscriptionPlanIds).toEqual(['gold']);
    expect(resolveDraftFormValues(undefined, publication, undefined).subscriptionPlanIds).toEqual(['gold']);
  });

  it('reads the plan ids as they are when the server sends no plan details', () => {
    const draft = aPublicationDraftDetails({ subscriptionPlanIds: ['gold', 'platinum'], subscriptionPlans: undefined });

    expect(resolveDraftFormValues(draft, undefined, undefined).subscriptionPlanIds).toEqual(['gold', 'platinum']);
  });

  it('lists the deleted plans with their stored names', () => {
    const draft = aPublicationDraftDetails({ subscriptionPlanIds: ['gold', 'platinum'], subscriptionPlans: selections });

    expect(deletedPlansOf(draft)).toEqual([{ displayName: 'Platinum', id: 'platinum' }]);
    expect(deletedPlansOf(undefined)).toEqual([]);
  });
});

describe('edited plans', () => {
  it('lists the plans edited since they were saved, leaving out deleted ones', () => {
    const draft = aPublicationDraftDetails({
      subscriptionPlans: [
        { deleted: false, displayName: 'Gold', edited: true, id: 'gold' },
        { deleted: false, displayName: 'Silver', edited: false, id: 'silver' },
        { deleted: true, displayName: 'Platinum', edited: true, id: 'platinum' },
      ],
    });

    expect(editedPlanIdsOf(draft)).toEqual(['gold']);
    expect(editedPlanIdsOf(undefined)).toEqual([]);
  });
});

describe('draftFormValuesToInput', () => {
  it('sends the agent visibility with the other details', () => {
    expect(draftFormValuesToInput({ ...emptyDraftFormValues, agentVisibility: 'HIDDEN' })).toMatchObject({
      agentVisibility: 'HIDDEN',
    });
  });
});

describe('draftFormValuesToInput documents', () => {
  it('always sends the selected documents, an empty selection included', () => {
    expect(draftFormValuesToInput({ ...emptyDraftFormValues, docIds: ['guide'] }).docIds).toEqual(['guide']);
    expect(draftFormValuesToInput(emptyDraftFormValues).docIds).toEqual([]);
  });
});

describe('sameSelection', () => {
  it('ignores order and compares membership', () => {
    expect(sameSelection(['a', 'b'], ['b', 'a'])).toBe(true);
    expect(sameSelection(['a'], ['a', 'b'])).toBe(false);
    expect(sameSelection(['a', 'b'], ['a', 'c'])).toBe(false);
    expect(sameSelection([], [])).toBe(true);
  });
});

describe('buildProductionUrlOptions', () => {
  const gateway = (overrides: Parameters<typeof aGateway>[0], isDeployed = true) => ({
    ...aGateway(overrides),
    associatedAt: '2026-01-01T00:00:00Z',
    isDeployed,
  });

  it("joins each deployed gateway's address with the API context, in the order given", () => {
    const gateways = [
      gateway({ displayName: 'One', endpoints: ['https://one.test/'], id: 'one' }),
      gateway({ displayName: 'Two', endpoints: ['two.test'], id: 'two' }),
    ];

    expect(buildProductionUrlOptions(gateways, '/loans')).toEqual([
      { gatewayName: 'One', url: 'https://one.test/loans' },
      { gatewayName: 'Two', url: 'https://two.test/loans' },
    ]);
  });

  it('leaves out gateways the API is not deployed to and gateways without an address', () => {
    const gateways = [
      gateway({ displayName: 'Idle', endpoints: ['https://idle.test'], id: 'idle' }, false),
      gateway({ displayName: 'Blank', endpoints: [], id: 'blank' }),
    ];

    expect(buildProductionUrlOptions(gateways, '/loans')).toEqual([]);
  });
});
