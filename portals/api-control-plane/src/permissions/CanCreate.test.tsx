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

import { Button } from '@wso2/oxygen-ui';
import { useEffect } from 'react';
import { describe, expect, it } from 'vitest';

import {
  ResourceLimitsProvider,
  useResourceLimits,
  type ResourceLimitSet,
} from '../hooks/useResourceLimits';
import { renderWithProviders, screen } from '../test/utils';

import { Can } from './Can';
import { CanCreate } from './CanCreate';
import { isWriteOperation } from './evaluate';

/** Supplies limits the way a cloud plugin would, then renders the subject. */
function Supplier({
  limits,
  readOnly,
  children,
}: {
  limits?: ResourceLimitSet | null;
  readOnly?: { reason: string };
  children: React.ReactNode;
}) {
  const { setResourceLimits, setReadOnly } = useResourceLimits();
  // In an effect, like the real gate. RTL's `render` already wraps this in
  // `act`, so wrapping again here would nest and defer the update.
  useEffect(() => {
    if (limits !== undefined) setResourceLimits(limits);
    if (readOnly) setReadOnly(true, readOnly.reason);
    // One push is enough; the real gate re-supplies on its own signals.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);
  return <>{children}</>;
}

const renderCreateButton = (opts: {
  limits?: ResourceLimitSet | null;
  readOnly?: { reason: string };
}) =>
  renderWithProviders(
    <ResourceLimitsProvider>
      <Supplier limits={opts.limits} readOnly={opts.readOnly}>
        <CanCreate component="restApis" do="CreateRESTAPI">
          <Button variant="contained">Create API</Button>
        </CanCreate>
      </Supplier>
    </ResourceLimitsProvider>,
  );

const button = () => screen.getByRole('button', { name: 'Create API' });

describe('CanCreate', () => {
  // The standalone, on-prem and OSS path: nothing supplies a limit, so nothing
  // changes from how the console behaved before limits existed.
  it('leaves the control enabled when nothing has supplied limits', () => {
    renderCreateButton({});
    expect(button()).toBeEnabled();
  });

  it('leaves the control enabled below the limit', () => {
    renderCreateButton({ limits: { restApis: { max: 5, used: 4 } } });
    expect(button()).toBeEnabled();
  });

  it('disables the control at the limit', () => {
    renderCreateButton({ limits: { restApis: { max: 5, used: 5 } } });
    expect(button()).toBeDisabled();
  });

  it('ignores a limit for a component it does not create', () => {
    renderCreateButton({ limits: { gateways: { max: 1, used: 1 } } });
    expect(button()).toBeEnabled();
  });

  // Portals are capped on their own meter, exactly as APIs and gateways are,
  // so the three create surfaces behave identically at a full quota.
  it.each(['restApis', 'gateways', 'portals'] as const)(
    'disables a %s create action at its own limit',
    (component) => {
      renderWithProviders(
        <ResourceLimitsProvider>
          <Supplier limits={{ [component]: { max: 2, used: 2 } }}>
            <CanCreate component={component} do="CreateRESTAPI">
              <Button variant="contained">Create API</Button>
            </CanCreate>
          </Supplier>
        </ResourceLimitsProvider>,
      );
      expect(button()).toBeDisabled();
    },
  );

  it('disables for a read-only organization even with quota to spare', () => {
    renderCreateButton({
      limits: { restApis: { max: 5, used: 0 } },
      readOnly: { reason: 'Your trial has ended.' },
    });
    expect(button()).toBeDisabled();
  });

  it('treats an unlimited ceiling as no limit', () => {
    renderCreateButton({ limits: { restApis: { max: -1, used: 9000 } } });
    expect(button()).toBeEnabled();
  });

  // Fails open: a count billing could not read must not disable the control.
  it('treats an unreadable count as not at the limit', () => {
    renderCreateButton({ limits: { restApis: { max: 5, used: -1 } } });
    expect(button()).toBeEnabled();
  });

  it('renders a plain control when no component is given', () => {
    renderWithProviders(
      <ResourceLimitsProvider>
        <Supplier limits={{ restApis: { max: 1, used: 1 } }}>
          <CanCreate do="CreateRESTAPI">
            <Button variant="contained">Create API</Button>
          </CanCreate>
        </Supplier>
      </ResourceLimitsProvider>,
    );
    expect(button()).toBeEnabled();
  });
});

describe('isWriteOperation', () => {
  it('classifies creates, updates and deletes as writes', () => {
    expect(isWriteOperation('CreateRESTAPI')).toBe(true);
    expect(isWriteOperation('DeleteRESTAPI')).toBe(true);
  });

  // 85 read operations also accept a `:manage` scope, which is why the rule is
  // "accepts any :read scope" rather than "accepts a manage/create scope".
  it('classifies reads as reads even when they also accept :manage', () => {
    expect(isWriteOperation('getAgentProxy')).toBe(false);
    expect(isWriteOperation('ListRESTAPIs')).toBe(false);
  });

  // Fail open: an out-of-date generated map must leave controls enabled.
  it('treats an unknown operation as a read', () => {
    expect(isWriteOperation('SomeOperationThatDoesNotExist')).toBe(false);
  });
});

describe('Can, read-only mode', () => {
  const renderCan = (operation: string, readOnly: boolean) =>
    renderWithProviders(
      <ResourceLimitsProvider>
        <Supplier readOnly={readOnly ? { reason: 'Your trial has ended.' } : undefined}>
          <Can denied="disable" do={operation}>
            <Button>Act</Button>
          </Can>
        </Supplier>
      </ResourceLimitsProvider>,
    );

  it('disables a write operation while the organization is read-only', () => {
    renderCan('DeleteRESTAPI', true);
    expect(screen.getByRole('button', { name: 'Act' })).toBeDisabled();
  });

  // The point of deriving write-ness from the spec: a read-only organization
  // must still be able to browse.
  it('leaves a read operation enabled while read-only', () => {
    renderCan('ListRESTAPIs', true);
    expect(screen.getByRole('button', { name: 'Act' })).toBeEnabled();
  });

  it('leaves a write operation enabled when not read-only', () => {
    renderCan('DeleteRESTAPI', false);
    expect(screen.getByRole('button', { name: 'Act' })).toBeEnabled();
  });
});
