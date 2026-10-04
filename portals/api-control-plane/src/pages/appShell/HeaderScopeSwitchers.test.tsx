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

import { AppShell, Header } from '@wso2/oxygen-ui';
import { beforeEach, describe, expect, it } from 'vitest';

import { makeConsoleScope } from '../../test/mockScope';
import { anOrganization, collection } from '../../test/msw';
import { server } from '../../test/server';
import { renderWithProviders, screen } from '../../test/utils';
import { HeaderScopeSwitchers } from './HeaderScopeSwitchers';

/**
 * Oxygen's `Header.Switchers` is `display: none` below the `md` breakpoint and
 * flips to `flex` in a media query. jsdom never applies that override, so the
 * switchers compute as hidden and the default role query skips them — hence
 * `hidden: true` on the trigger lookups. The menu itself renders in a portal on
 * `body`, outside that box, so it is queried normally.
 */
const organizationTrigger = () =>
  screen.getByRole('combobox', { name: 'Organizations', hidden: true });

const ACME = anOrganization({ id: 'acme-org', displayName: 'Acme' });
const GLOBEX = anOrganization({ id: 'globex-org', displayName: 'Globex' });

const renderSwitchers = (organizations: ReturnType<typeof anOrganization>[]) =>
  renderWithProviders(
    // `Header.Switchers` needs its compound parent, and `Header` needs the shell.
    <AppShell>
      <AppShell.Navbar>
        <Header>
          <HeaderScopeSwitchers />
        </Header>
      </AppShell.Navbar>
    </AppShell>,
    {
      route: `/organizations/${ACME.id}/projects/project-1/home`,
      scope: makeConsoleScope({
        organization: ACME,
        organizations,
        params: { orgHandle: ACME.id, projectHandler: 'project-1' },
      }),
    },
  );

describe('HeaderScopeSwitchers organization switcher', () => {
  beforeEach(() => {
    // The API switcher fetches whenever a project is in scope.
    server.use(collection('/rest-apis', []));
  });

  it('stays inert, at full contrast, when the user belongs to a single organization', async () => {
    const { user } = renderSwitchers([ACME]);

    const trigger = organizationTrigger();
    // Still names the org — going read-only must not blank the header.
    expect(trigger).toHaveTextContent('Acme');
    // Read-only, not disabled: `disabled` would grey the name out, and the org
    // you are in is information the header should keep showing plainly.
    expect(trigger).toHaveClass('Mui-readOnly');
    expect(trigger).not.toHaveClass('Mui-disabled');
    // No chevron, because there is nothing it could drop down. Scoped to this
    // field — the project switcher beside it keeps its own.
    expect(trigger.closest('.MuiFormControl-root')?.querySelector('.MuiSelect-icon')).toBeNull();

    await user.click(trigger);

    expect(screen.queryByRole('listbox')).not.toBeInTheDocument();
  });

  it('opens the picker when there is more than one organization', async () => {
    const { user } = renderSwitchers([ACME, GLOBEX]);

    await user.click(organizationTrigger());

    expect(await screen.findByRole('listbox')).toBeInTheDocument();
    expect(screen.getByRole('option', { name: /Globex/ })).toBeInTheDocument();
  });
});
