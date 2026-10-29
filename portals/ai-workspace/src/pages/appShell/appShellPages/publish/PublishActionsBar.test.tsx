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
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import { describe, expect, it, vi } from 'vitest';

import { renderWithProviders, screen } from '../../../../test/utils';
import { PublishActionsBar, type PublishActionsBarProps } from './PublishActionsBar';

const everything = { publish: true, unpublish: true, deprecate: true };

const renderBar = (props: Partial<PublishActionsBarProps> = {}) => {
  const handlers = {
    onDeprecate: vi.fn(),
    onPublish: vi.fn(),
    onSaveDraft: vi.fn(),
    onUnpublish: vi.fn(),
  };
  const view = renderWithProviders(
    <PublishActionsBar
      canDeprecate={false}
      canSaveDraft
      deprecating={false}
      isPublished={false}
      permissions={everything}
      publishing={false}
      savingDraft={false}
      unpublishing={false}
      {...handlers}
      {...props}
    />
  );
  return { ...view, ...handlers };
};

/** The main side of the split button, whatever it currently says. */
const mainButton = () => screen.getByRole('button', { name: /^(Publish|Republish|Deprecate|Unpublish)$/ });

const openMenu = async (user: ReturnType<typeof renderBar>['user']) =>
  user.click(screen.getByRole('button', { name: 'More publish actions' }));

describe('PublishActionsBar', () => {
  describe('Save Draft', () => {
    it('saves when there are edits', async () => {
      const { onSaveDraft, user } = renderBar();

      await user.click(screen.getByRole('button', { name: 'Save Draft' }));

      expect(onSaveDraft).toHaveBeenCalledTimes(1);
    });

    it('is off while there is nothing to save', () => {
      renderBar({ canSaveDraft: false });

      expect(screen.getByRole('button', { name: 'Save Draft' })).toBeDisabled();
    });
  });

  describe('the main button', () => {
    it('publishes by default', async () => {
      const { onPublish, user } = renderBar();

      await user.click(mainButton());

      expect(onPublish).toHaveBeenCalledTimes(1);
    });

    it('reads Republish once the listing is already published', () => {
      renderBar({ canDeprecate: true, isPublished: true });

      expect(screen.getByRole('button', { name: 'Republish' })).toBeInTheDocument();
    });

    it('is off for a user who may not publish', () => {
      renderBar({ permissions: { ...everything, publish: false } });

      expect(mainButton()).toBeDisabled();
    });
  });

  describe('the other actions', () => {
    it('lists Deprecate and Unpublish, both off before there is a live listing', async () => {
      const { user } = renderBar();

      await openMenu(user);

      expect(screen.getByRole('menuitem', { name: 'Deprecate' })).toHaveAttribute('aria-disabled', 'true');
      expect(screen.getByRole('menuitem', { name: 'Unpublish' })).toHaveAttribute('aria-disabled', 'true');
    });

    it('arms Deprecate on a published listing, then fires it from the main button', async () => {
      const { onDeprecate, onPublish, user } = renderBar({ canDeprecate: true, isPublished: true });

      await openMenu(user);
      await user.click(screen.getByRole('menuitem', { name: 'Deprecate' }));
      expect(onDeprecate).not.toHaveBeenCalled();

      await user.click(mainButton());

      expect(onDeprecate).toHaveBeenCalledTimes(1);
      expect(onPublish).not.toHaveBeenCalled();
    });

    it('offers Unpublish, but not Deprecate, on a listing that is already deprecated', async () => {
      const { onUnpublish, user } = renderBar({ canDeprecate: false, isPublished: true });

      await openMenu(user);
      expect(screen.getByRole('menuitem', { name: 'Deprecate' })).toHaveAttribute('aria-disabled', 'true');

      await user.click(screen.getByRole('menuitem', { name: 'Unpublish' }));
      await user.click(mainButton());

      expect(onUnpublish).toHaveBeenCalledTimes(1);
    });

    it('turns off an action the user lacks the scope for, even when it applies', async () => {
      const { user } = renderBar({
        canDeprecate: true,
        isPublished: true,
        permissions: { publish: true, unpublish: true, deprecate: false },
      });

      await openMenu(user);

      expect(screen.getByRole('menuitem', { name: 'Deprecate' })).toHaveAttribute('aria-disabled', 'true');
      expect(screen.getByRole('menuitem', { name: 'Unpublish' })).not.toHaveAttribute('aria-disabled');
    });

    it('goes back to Publish when the live state changes under an armed action', async () => {
      const { rerender, user } = renderBar({ canDeprecate: true, isPublished: true });
      await openMenu(user);
      await user.click(screen.getByRole('menuitem', { name: 'Deprecate' }));
      expect(mainButton()).toHaveTextContent('Deprecate');

      rerender(
        <PublishActionsBar
          canDeprecate={false}
          canSaveDraft
          deprecating={false}
          isPublished
          onDeprecate={vi.fn()}
          onPublish={vi.fn()}
          onSaveDraft={vi.fn()}
          onUnpublish={vi.fn()}
          permissions={everything}
          publishing={false}
          savingDraft={false}
          unpublishing={false}
        />
      );

      expect(mainButton()).toHaveTextContent('Publish');
    });
  });

  it('locks every action while one is running', () => {
    renderBar({ canDeprecate: true, isPublished: true, publishing: true });

    expect(screen.getByRole('button', { name: 'Save Draft' })).toBeDisabled();
    expect(mainButton()).toBeDisabled();
    expect(screen.getByRole('button', { name: 'More publish actions' })).toBeDisabled();
  });
});
