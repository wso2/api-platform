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

import { renderWithProviders, screen } from '@/test/utils';
import { PlanCard } from './PlanCard';

const description = 'For trying the API out.';

const chipsOf = (card: HTMLElement) => [...card.querySelectorAll('.MuiChip-root')];

describe('PlanCard', () => {
  it('draft: shows only the name and the description, whatever the plan has become', () => {
    renderWithProviders(
      <PlanCard deleted displayName="Plan" edited inactive description={description} selected />,
    );

    const card = screen.getByRole('checkbox', { name: /Plan/ });
    expect(chipsOf(card)).toHaveLength(0);
    expect(card.querySelector('del')).toBeNull();
    expect(card).toHaveTextContent(description);
  });

  it('published: a plain plan has no chip and keeps its description', () => {
    renderWithProviders(<PlanCard displayName="Plan" description={description} readOnly selected />);

    const card = screen.getByRole('checkbox', { name: /Plan/ });
    expect(chipsOf(card)).toHaveLength(0);
    expect(card).toHaveTextContent(description);
  });

  it('published: an edited plan gets an outlined chip with no icon, right after the name', () => {
    renderWithProviders(<PlanCard displayName="Plan" edited description={description} readOnly selected />);

    const card = screen.getByRole('checkbox', { name: /Plan/ });
    const [chip] = chipsOf(card);
    expect(chip).toHaveTextContent('Edited');
    expect(chip).toHaveClass('MuiChip-outlined', 'MuiChip-colorWarning');
    expect(chip.querySelector('svg')).toBeNull();
    expect(screen.getByText('Plan').nextElementSibling).toBe(chip);
    // The card could only show the plan's current description, so it shows none.
    expect(card).not.toHaveTextContent(description);
    expect(card).toHaveTextContent('Edited since last publish');
  });

  it('published: an inactive plan gets an outlined chip and keeps its description', () => {
    renderWithProviders(<PlanCard displayName="Plan" inactive description={description} readOnly selected />);

    const card = screen.getByRole('checkbox', { name: /Plan/ });
    const chips = chipsOf(card);
    expect(card).toHaveTextContent(description);
    expect(chips).toHaveLength(1);
    expect(chips[0]).toHaveTextContent('Inactive');
    expect(chips[0]).toHaveClass('MuiChip-outlined', 'MuiChip-colorDefault');
    expect(chips[0].querySelector('svg')).toBeNull();
    expect(card).not.toHaveTextContent('since last publish');
  });

  it('published: an inactive plan that was also edited wears Inactive but hides its description', () => {
    renderWithProviders(<PlanCard displayName="Plan" edited inactive description={description} readOnly selected />);

    const card = screen.getByRole('checkbox', { name: /Plan/ });
    const chips = chipsOf(card);
    expect(chips).toHaveLength(1);
    expect(chips[0]).toHaveTextContent('Inactive');
    expect(card).not.toHaveTextContent(description);
    expect(card).toHaveTextContent('Edited since last publish');
  });

  it('published: a deleted plan has its name struck through and no chip', () => {
    renderWithProviders(<PlanCard deleted displayName="Plan" edited inactive readOnly selected />);

    const card = screen.getByRole('checkbox', { name: /Plan/ });
    expect(chipsOf(card)).toHaveLength(0);
    expect(card.querySelector('del')).toHaveTextContent('Plan');
    // The listing still holds it, so it stays ticked even though it is greyed.
    expect(card).toHaveAttribute('aria-checked', 'true');
    expect(card).toHaveTextContent('Deleted since last publish');
  });

  it('draft: shows no since-last-publish line', () => {
    renderWithProviders(<PlanCard deleted displayName="Plan" edited description={description} selected />);

    expect(screen.getByRole('checkbox', { name: /Plan/ })).not.toHaveTextContent('since last publish');
  });

  it('shows no limit or price, only the name and the description', () => {
    renderWithProviders(<PlanCard description={description} displayName="Plan" selected={false} />);

    const card = screen.getByRole('checkbox', { name: /Plan/ });
    expect(card).toHaveTextContent(`Plan${description}`);
    expect(card).not.toHaveTextContent(/requests|Unlimited|\$/);
  });

  it('shows when a plan expires in a chip after its name', () => {
    renderWithProviders(
      <PlanCard displayName="Plan" expiryTime="2027-03-31T00:00:00Z" selected={false} />,
    );

    const [chip] = chipsOf(screen.getByRole('checkbox', { name: /Plan/ }));
    expect(chip).toHaveTextContent(/^Expires .*2027/);
    expect(screen.getByText('Plan').nextElementSibling).toBe(chip);
  });

  it('shows no expiry chip on a plan that never expires', () => {
    renderWithProviders(<PlanCard displayName="Plan" expiryTime={null} selected={false} />);

    expect(chipsOf(screen.getByRole('checkbox', { name: /Plan/ }))).toHaveLength(0);
  });
});
