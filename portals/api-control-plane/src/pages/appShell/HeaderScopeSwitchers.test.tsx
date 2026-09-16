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
import { useLocation } from 'react-router-dom';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import type { GraphQLApiListItem } from '@/api/resources/graphqlApis';
import type { RestApi } from '@/api/resources/restApis';
import { routes } from '@/routes/paths';
import { anOrganization, aProject } from '@/test/msw';
import { makeConsoleScope } from '@/test/mockScope';
import { renderWithProviders, screen } from '@/test/utils';

vi.mock('@/api/resources/restApis', async (importActual) => ({
  ...(await importActual<typeof import('@/api/resources/restApis')>()),
  useAllRestApis: vi.fn(),
}));
vi.mock('@/api/resources/graphqlApis', async (importActual) => ({
  ...(await importActual<typeof import('@/api/resources/graphqlApis')>()),
  useAllGraphQLApis: vi.fn(),
}));

import { useAllGraphQLApis } from '@/api/resources/graphqlApis';
import { useAllRestApis } from '@/api/resources/restApis';
import { HeaderScopeSwitchers } from './HeaderScopeSwitchers';

const ORG = anOrganization().id;
const PROJECT_ID = 'retail-apis';
const PROJECT = aProject({ id: PROJECT_ID });

const REST_API_LIST = [
  { context: '/orders', displayName: 'Orders API', id: 'orders-api', kind: 'RestApi', projectId: PROJECT_ID, upstream: { main: { url: 'https://backend.example.com' } }, version: '1.0.0' },
] as RestApi[];

const GRAPHQL_API_LIST = [
  { context: '/countries', displayName: 'Countries API', id: 'countries-graphql-api', kind: 'GraphQLApi', projectId: PROJECT_ID, upstream: { main: { url: 'https://countries.example.com/graphql' } }, version: '1.0.0' },
] as GraphQLApiListItem[];

const restApisResult = (list: RestApi[]) => ({
  data: list.length ? { list, pagination: { limit: 100, offset: 0, total: list.length } } : undefined,
  error: undefined,
  isPending: false,
  isPlaceholderData: false,
});

const graphqlApisResult = (list: GraphQLApiListItem[]) => ({
  data: list.length ? { list, pagination: { limit: 100, offset: 0, total: list.length } } : undefined,
  error: undefined,
  isPending: false,
  isPlaceholderData: false,
});

/** Renders the current pathname so a test can assert where a switch landed. */
function Located() {
  return <span>{`at ${useLocation().pathname}`}</span>;
}

/**
 * `APIQuickSelector`'s icon button sets its own `aria-label` while also
 * sitting inside a `Tooltip`'s wrapping `<span>` carrying the identical
 * label — two elements match `getByLabelText`/`getByRole` name computation
 * for the same string, so grab the actual `<button>` directly rather than
 * either query (which either finds zero or "multiple elements").
 */
const selectApiButton = () =>
  screen.getAllByLabelText('Select API').find((el): el is HTMLButtonElement => el instanceof HTMLButtonElement)!;

const scopeAtProject = () => makeConsoleScope({ project: PROJECT, projects: [PROJECT] });

describe('HeaderScopeSwitchers — the API switcher lists both resource types', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(useAllRestApis).mockReturnValue(restApisResult(REST_API_LIST));
    vi.mocked(useAllGraphQLApis).mockReturnValue(graphqlApisResult(GRAPHQL_API_LIST));
  });

  it('offers a GraphQL API through the quick selector, not just REST ones', async () => {
    const { user } = renderWithProviders(
      <Header>
        <HeaderScopeSwitchers />
      </Header>,
      { scope: scopeAtProject() },
    );

    await user.click(selectApiButton());

    expect(screen.getByText('Orders API')).toBeInTheDocument();
    expect(screen.getByText('Countries API')).toBeInTheDocument();
  });

  it('switches to a GraphQL API’s own Overview page, not the REST detail route', async () => {
    const { user } = renderWithProviders(
      <>
        <Header>
          <HeaderScopeSwitchers />
        </Header>
        <Located />
      </>,
      { scope: scopeAtProject() },
    );

    await user.click(selectApiButton());
    await user.click(screen.getByText('Countries API'));

    expect(
      screen.getByText(`at ${routes.graphqlApi(ORG, PROJECT_ID, 'countries-graphql-api')}`),
    ).toBeInTheDocument();
  });

  it('still switches to a REST API through the REST detail route', async () => {
    const { user } = renderWithProviders(
      <>
        <Header>
          <HeaderScopeSwitchers />
        </Header>
        <Located />
      </>,
      { scope: scopeAtProject() },
    );

    await user.click(selectApiButton());
    await user.click(screen.getByText('Orders API'));

    expect(
      screen.getByText(`at ${routes.api(ORG, PROJECT_ID, 'orders-api')}`),
    ).toBeInTheDocument();
  });
});

/*
 * A GraphQL API sets `params.graphqlApiHandler`, never `params.apiHandler`
 * (see `graphqlApiPath`'s doc comment) — the switcher has to treat either as
 * "an API is selected" or it falls back to the quick-selector every time a
 * GraphQL API is open, instead of showing it as the current selection the
 * way an open REST API already does.
 */
describe('HeaderScopeSwitchers — a GraphQL API in scope', () => {
  const GRAPHQL_API = 'countries-graphql-api';

  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(useAllRestApis).mockReturnValue(restApisResult(REST_API_LIST));
    vi.mocked(useAllGraphQLApis).mockReturnValue(graphqlApisResult(GRAPHQL_API_LIST));
  });

  const scopeAtGraphqlApi = () =>
    makeConsoleScope({
      isGraphQLApiScope: true,
      params: { graphqlApiHandler: GRAPHQL_API, orgHandle: ORG, projectHandler: PROJECT_ID },
      project: PROJECT,
      projects: [PROJECT],
    });

  it('shows the APIs switcher pill with the GraphQL API selected, not the quick-selector', async () => {
    renderWithProviders(
      <Header>
        <HeaderScopeSwitchers />
      </Header>,
      { scope: scopeAtGraphqlApi() },
    );

    expect(await screen.findByText('Countries API')).toBeInTheDocument();
    expect(screen.queryAllByLabelText('Select API')).toHaveLength(0);
  });
});

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
    // The API switcher fetches whenever a project is in scope — mocked here
    // (not via MSW) since `useAllRestApis`/`useAllGraphQLApis` are already
    // module-mocked above for the API-switcher suites in this same file.
    vi.clearAllMocks();
    vi.mocked(useAllRestApis).mockReturnValue(restApisResult([]));
    vi.mocked(useAllGraphQLApis).mockReturnValue(graphqlApisResult([]));
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
