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

import { aPublicationSummary } from '@/test/msw';
import { renderWithProviders, screen } from '@/test/utils';
import { PortalPublicationCard } from './PortalPublicationCard';

const renderCard = (publishDeniedTooltip?: string) =>
  renderWithProviders(
    <PortalPublicationCard
      apiHandle="pets"
      orgHandle="acme"
      publication={aPublicationSummary()}
      publishDeniedTooltip={publishDeniedTooltip}
      publishPath={(id) => `/acme/apis/pets/portals/${id}/publish`}
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
});
