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

import { beforeEach, describe, expect, it, vi } from 'vitest';

import type { PublishableApiType } from '@/api/resources/apiPublications';
import { resetTrackedFeatures, trackedFeatures } from '@/test/featureUsage';
import { aPublicationSummary, type PublicationSummaryFixture } from '@/test/msw';
import { renderWithProviders, screen } from '@/test/utils';
import { graphqlPublishTarget, restPublishTarget } from '../publishTarget';
import { PortalPublicationCard } from './PortalPublicationCard';

vi.mock('@/featureUsage/moesif');

beforeEach(() => resetTrackedFeatures());

const renderCard = (
  publishDeniedTooltip?: string,
  publication: PublicationSummaryFixture = aPublicationSummary(),
  apiType: PublishableApiType = 'rest-api',
) =>
  renderWithProviders(
    <PortalPublicationCard
      oneClickPublishAllowed
      onPublish={() => {}}
      orgHandle="acme"
      publication={publication}
      publishDeniedTooltip={publishDeniedTooltip}
      publishing={false}
      target={(apiType === 'rest-api' ? restPublishTarget : graphqlPublishTarget)('acme', 'shop', 'pets')}
    />,
  );

describe('PortalPublicationCard', () => {
  it('offers Publish when the caller may publish', () => {
    renderCard();

    expect(screen.getByRole('button', { name: 'Publish' })).toBeEnabled();
  });

  it('disables Publish, and its menu, for a caller who may not publish', () => {
    renderCard("You don't have permission to perform this action.");

    expect(screen.getByRole('button', { name: 'Publish' })).toBeDisabled();
    expect(screen.getByRole('button', { name: /more/i })).toBeDisabled();
  });

  it.each(['rest-api', 'graphql-api'] as const)(
    'reports opening the %s listing on the portal from View in Portal',
    async (apiType) => {
      const { user } = renderCard(undefined, aPublicationSummary({ status: 'PUBLISHED' }), apiType);

      const link = screen.getByRole('link', { name: 'View in Portal' });
      // Stops jsdom from attempting the new-tab navigation; the click itself is what is counted.
      link.addEventListener('click', (event) => event.preventDefault());
      await user.click(link);

      expect(trackedFeatures()).toEqual([
        ['apip.lifecycle.api-portal-open', { api_type: apiType }],
      ]);
    },
  );
});
