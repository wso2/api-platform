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
  draftFormValuesToInput,
  emptyDraftFormValues,
  resolveDraftFormValues,
  sameDraftFormValues,
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
    });
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

  it('reads the subscription plans from the draft, then the publication, and starts with none', () => {
    const draft = aPublicationDraftDetails({ subscriptionPlanIds: ['gold', 'silver'] });
    const publication = aPublication({ subscriptionPlanIds: ['bronze'] });

    expect(resolveDraftFormValues(draft, publication, api).subscriptionPlanIds).toEqual(['gold', 'silver']);
    expect(resolveDraftFormValues(undefined, publication, api).subscriptionPlanIds).toEqual(['bronze']);
    // A draft that offers no plans stays at none, rather than taking the publication's.
    expect(
      resolveDraftFormValues(aPublicationDraftDetails({ subscriptionPlanIds: [] }), publication, api)
        .subscriptionPlanIds,
    ).toEqual([]);
    expect(resolveDraftFormValues(aPublicationDraftDetails(), undefined, api).subscriptionPlanIds).toEqual([]);
    expect(resolveDraftFormValues(undefined, undefined, api).subscriptionPlanIds).toEqual([]);
  });
});

describe('draftFormValuesToInput', () => {
  it('sends the agent visibility with the other details', () => {
    expect(draftFormValuesToInput({ ...emptyDraftFormValues, agentVisibility: 'HIDDEN' })).toMatchObject({
      agentVisibility: 'HIDDEN',
    });
  });

  it('sends the selected subscription plans as their handles', () => {
    expect(
      draftFormValuesToInput({ ...emptyDraftFormValues, subscriptionPlanIds: ['gold', 'silver'] }).subscriptionPlanIds,
    ).toEqual(['gold', 'silver']);
  });

  it('sends an empty plan list rather than leaving it out, since an omitted list clears the plans', () => {
    const input = draftFormValuesToInput(emptyDraftFormValues);

    expect(input).toHaveProperty('subscriptionPlanIds');
    expect(input.subscriptionPlanIds).toEqual([]);
  });

  it('sends back the plans loaded from the draft when a save changes something else', () => {
    const draft = aPublicationDraftDetails({ subscriptionPlanIds: ['gold'] });
    const values = { ...resolveDraftFormValues(draft, undefined, api), description: 'Edited.' };

    expect(draftFormValuesToInput(values)).toMatchObject({ description: 'Edited.', subscriptionPlanIds: ['gold'] });
  });
});

describe('sameDraftFormValues', () => {
  const values = { ...emptyDraftFormValues, displayName: 'Loans', subscriptionPlanIds: ['gold', 'silver'] };

  it('ignores the order plans were picked in', () => {
    expect(sameDraftFormValues(values, { ...values, subscriptionPlanIds: ['silver', 'gold'] })).toBe(true);
  });

  it('tells a different plan selection apart', () => {
    expect(sameDraftFormValues(values, { ...values, subscriptionPlanIds: ['gold'] })).toBe(false);
    expect(sameDraftFormValues(values, { ...values, subscriptionPlanIds: [] })).toBe(false);
  });

  it('tells any other edited field apart', () => {
    expect(sameDraftFormValues(values, { ...values, displayName: 'Loans API' })).toBe(false);
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
