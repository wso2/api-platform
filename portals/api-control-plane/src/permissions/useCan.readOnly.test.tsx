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

import { useEffect } from 'react';
import { describe, expect, it } from 'vitest';

import { ResourceLimitsProvider, useResourceLimits } from '../hooks/useResourceLimits';
import { renderWithProviders, screen } from '../test/utils';

import { Can } from './Can';
import { useActionPermission, useCan, useCanAny } from './useCan';

const READ_ONLY_REASON = 'No active plan.';

/** Reports read-only the way the cloud gate does. */
function Supplier({ children }: { children: React.ReactNode }) {
  const { setReadOnly } = useResourceLimits();
  useEffect(() => {
    setReadOnly(true, READ_ONLY_REASON);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);
  return <>{children}</>;
}

/**
 * The pages this guards use the hooks directly rather than `<Can>` — an edit
 * page holding its own `canEdit`, a card holding its own `canDelete`. Those are
 * exactly the controls that stayed live when only `<Can>` honoured read-only.
 */
function Subject() {
  const canEdit = useCan('UpdateRESTAPI');
  const canRead = useCan('GetRESTAPI');
  const menuReachable = useCanAny(['UpdateRESTAPI', 'GetRESTAPI']);
  const writesOnlyMenu = useCanAny(['UpdateRESTAPI', 'DeleteRESTAPI']);
  const action = useActionPermission('UpdateRESTAPI');

  return (
    <>
      <span data-testid="can-edit">{String(canEdit)}</span>
      <span data-testid="can-read">{String(canRead)}</span>
      <span data-testid="menu-reachable">{String(menuReachable)}</span>
      <span data-testid="writes-only-menu">{String(writesOnlyMenu)}</span>
      <span data-testid="action-allowed">{String(action.allowed)}</span>
      <span data-testid="action-reason">{action.reason}</span>
      <span data-testid="action-tooltip">{action.tooltip ?? ''}</span>
    </>
  );
}

const render = (readOnly: boolean) =>
  renderWithProviders(
    <ResourceLimitsProvider>
      {readOnly ? (
        <Supplier>
          <Subject />
        </Supplier>
      ) : (
        <Subject />
      )}
    </ResourceLimitsProvider>,
  );

const text = (id: string) => screen.getByTestId(id).textContent;

describe('permission hooks under read-only', () => {
  it('blocks a write and leaves a read alone', () => {
    render(true);
    expect(text('can-edit')).toBe('false');
    expect(text('can-read')).toBe('true');
  });

  it('keeps a mixed menu reachable but closes a writes-only one', () => {
    render(true);
    expect(text('menu-reachable')).toBe('true');
    expect(text('writes-only-menu')).toBe('false');
  });

  it("explains itself with the supplier's own reason", () => {
    render(true);
    expect(text('action-allowed')).toBe('false');
    expect(text('action-reason')).toBe('read-only');
    expect(text('action-tooltip')).toBe(READ_ONLY_REASON);
  });

  it('changes nothing with no supplier — the standalone path', () => {
    render(false);
    expect(text('can-edit')).toBe('true');
    expect(text('can-read')).toBe('true');
    expect(text('writes-only-menu')).toBe('true');
    expect(text('action-allowed')).toBe('true');
    expect(text('action-tooltip')).toBe('');
  });
});

describe('Can still disables rather than hides', () => {
  it('shows a denied="hide" write control, disabled, while read-only', () => {
    renderWithProviders(
      <ResourceLimitsProvider>
        <Supplier>
          <Can denied="hide" do="UpdateRESTAPI">
            <button type="button">Edit</button>
          </Can>
        </Supplier>
      </ResourceLimitsProvider>,
    );
    expect(screen.getByRole('button', { name: 'Edit' })).toBeDisabled();
  });
});
