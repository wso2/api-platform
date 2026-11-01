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
import { act } from 'react';

import { renderWithProviders, screen } from '../test/utils';
import {
  ResourceLimitsProvider,
  useResourceLimits,
  type ResourceLimitsContextType,
  type ResourceLimitSet,
} from './useResourceLimits';

/**
 * Renders the hook and hands the live context back, so a test can drive the
 * setters a supplier would call and read the verdicts a page would read.
 */
function renderLimits(options?: { withProvider?: boolean }) {
  const seen: { current: ResourceLimitsContextType | null } = { current: null };

  function Probe() {
    const value = useResourceLimits();
    seen.current = value;
    return (
      <>
        <span data-testid="can-create-apis">{String(value.canCreate('restApis'))}</span>
        <span data-testid="limit-message-apis">{value.limitMessage('restApis')}</span>
        <span data-testid="read-only">{String(value.readOnly)}</span>
      </>
    );
  }

  renderWithProviders(
    options?.withProvider === false ? (
      <Probe />
    ) : (
      <ResourceLimitsProvider>
        <Probe />
      </ResourceLimitsProvider>
    )
  );

  const supply = (limits: ResourceLimitSet | null) =>
    act(() => seen.current?.setResourceLimits(limits));

  return { seen, supply };
}

const canCreateApis = () => screen.getByTestId('can-create-apis').textContent;
const apiMessage = () => screen.getByTestId('limit-message-apis').textContent;

describe('useResourceLimits', () => {
  describe('with no supplier — the standalone, on-prem and OSS path', () => {
    it('allows creation outside the provider entirely', () => {
      renderLimits({ withProvider: false });
      expect(canCreateApis()).toBe('true');
      expect(apiMessage()).toBe('');
      expect(screen.getByTestId('read-only').textContent).toBe('false');
    });

    it('allows creation when a provider is mounted but nothing has supplied limits', () => {
      renderLimits();
      expect(canCreateApis()).toBe('true');
      expect(apiMessage()).toBe('');
    });
  });

  describe('isAtLimit', () => {
    it('blocks creation once used reaches max', () => {
      const { supply } = renderLimits();
      supply({ restApis: { max: 5, used: 5 } });
      expect(canCreateApis()).toBe('false');
    });

    it('blocks creation when used has somehow passed max', () => {
      const { supply } = renderLimits();
      supply({ restApis: { max: 5, used: 7 } });
      expect(canCreateApis()).toBe('false');
    });

    it('allows creation below max', () => {
      const { supply } = renderLimits();
      supply({ restApis: { max: 5, used: 4 } });
      expect(canCreateApis()).toBe('true');
    });

    // -1 is the agreed "no ceiling" marker; a literal comparison against used
    // would read it as a limit of minus one and block everything.
    it('treats max of -1 as unlimited', () => {
      const { supply } = renderLimits();
      supply({ restApis: { max: -1, used: 9000 } });
      expect(canCreateApis()).toBe('true');
    });

    // Fails open deliberately: a wrong "allowed" costs a failed submit, a wrong
    // "denied" locks a user out of their own product.
    it('treats used of -1 as an unreadable count, not as at-limit', () => {
      const { supply } = renderLimits();
      supply({ restApis: { max: 5, used: -1 } });
      expect(canCreateApis()).toBe('true');
    });

    it('leaves a component nobody capped enabled', () => {
      const { supply } = renderLimits();
      supply({ gateways: { max: 1, used: 1 } });
      expect(canCreateApis()).toBe('true');
    });

    it('restores everything when the limits are cleared', () => {
      const { supply } = renderLimits();
      supply({ restApis: { max: 5, used: 5 } });
      expect(canCreateApis()).toBe('false');
      supply(null);
      expect(canCreateApis()).toBe('true');
    });
  });

  describe('limitMessage', () => {
    it('falls back to a generic sentence naming the ceiling', () => {
      const { supply } = renderLimits();
      supply({ restApis: { max: 5, used: 5 } });
      expect(apiMessage()).toBe(
        'You cannot create more APIs because your organization has reached the maximum of 5.'
      );
    });

    // Only the supplier can name the cap that actually binds, which may be a
    // shared budget rather than this component's own ceiling.
    it("prefers the supplier's own wording when it gave one", () => {
      const { supply } = renderLimits();
      supply({
        restApis: {
          max: 5,
          used: 5,
          message: 'You have reached the maximum of 10 Managed Interfaces.',
        },
      });
      expect(apiMessage()).toBe('You have reached the maximum of 10 Managed Interfaces.');
    });

    it('ignores a blank supplier message rather than showing an empty tooltip', () => {
      const { supply } = renderLimits();
      supply({ restApis: { max: 5, used: 5, message: '   ' } });
      expect(apiMessage()).toContain('reached the maximum of 5');
    });

    it('says nothing while creation is still allowed', () => {
      const { supply } = renderLimits();
      supply({ restApis: { max: 5, used: 1 } });
      expect(apiMessage()).toBe('');
    });
  });

  describe('read-only', () => {
    it('blocks creation of everything, whatever the counts say', () => {
      const { seen, supply } = renderLimits();
      supply({ restApis: { max: 5, used: 0 } });
      act(() => seen.current?.setReadOnly(true));
      expect(canCreateApis()).toBe('false');
      expect(screen.getByTestId('read-only').textContent).toBe('true');
    });

    // Read-only is the broader fact, so explaining a per-component ceiling to a
    // user who cannot write at all would be a misleading answer.
    it('outranks a quota message', () => {
      const { seen, supply } = renderLimits();
      supply({ restApis: { max: 5, used: 5 } });
      act(() => seen.current?.setReadOnly(true, 'Your trial has ended.'));
      expect(apiMessage()).toBe('Your trial has ended.');
    });

    it('falls back to a default explanation when the reason is blank', () => {
      const { seen } = renderLimits();
      act(() => seen.current?.setReadOnly(true, '   '));
      expect(seen.current?.readOnlyMessage).toBe(
        'Your organization is read-only, so nothing can be created or changed.'
      );
    });

    it('clears the message and banner when read-only is lifted', () => {
      const { seen } = renderLimits();
      act(() => seen.current?.setReadOnly(true, 'Your trial has ended.'));
      act(() => seen.current?.setReadOnly(false));
      expect(seen.current?.readOnlyMessage).toBe('');
      expect(seen.current?.readOnlyBanner).toBeNull();
    });
  });

  describe('refresh', () => {
    it('bumps the signal the supplier watches', () => {
      const { seen } = renderLimits();
      const before = seen.current?.refreshSignal ?? -1;
      act(() => seen.current?.refresh());
      expect(seen.current?.refreshSignal).toBe(before + 1);
    });

    it('is a harmless no-op with no provider mounted', () => {
      const { seen } = renderLimits({ withProvider: false });
      expect(() => act(() => seen.current?.refresh())).not.toThrow();
      expect(seen.current?.refreshSignal).toBe(0);
    });
  });
});
