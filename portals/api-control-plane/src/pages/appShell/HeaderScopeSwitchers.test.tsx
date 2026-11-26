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

import { AppShell, Header, readHeaderBreadcrumbsState } from '@wso2/oxygen-ui';
import { useLocation } from 'react-router-dom';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import type { GraphQLApiListItem } from '@/api/resources/graphqlApis';
import type { RestApi } from '@/api/resources/restApis';
import { routes } from '@/routes/paths';
import { anOrganization, aProject } from '@/test/msw';
import type { AuthState } from '@/contexts/auth/authTypes';
import { makeAuthState } from '@/test/mockAuthState';
import { makeConsoleScope } from '@/test/mockScope';
import { renderWithProviders, screen, within } from '@/test/utils';

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
 * Oxygen's `Header.Switchers` is `display: none` below the `md` breakpoint and
 * flips to `flex` in a media query. jsdom never applies that override, so the
 * breadcrumb computes as hidden and the default role query skips it — hence
 * `hidden: true` on the trigger lookups. Each menu renders in a portal on
 * `body`, outside that box, so it is queried normally.
 */
const trigger = (name: string) => screen.getByRole('button', { name, hidden: true });

/** The › button a project page shows while no API is selected. */
const selectApiButton = () => trigger('Select API');

const scopeAtProject = () => makeConsoleScope({ project: PROJECT, projects: [PROJECT] });

describe('HeaderScopeSwitchers — the API switcher lists both resource types', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(useAllRestApis).mockReturnValue(restApisResult(REST_API_LIST));
    vi.mocked(useAllGraphQLApis).mockReturnValue(graphqlApisResult(GRAPHQL_API_LIST));
  });

  it('offers a GraphQL API in the open-API menu, not just REST ones', async () => {
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
 * "an API is selected" or it falls back to the › open button every time a
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

  it('shows the GraphQL API as the current segment, not the › open button', async () => {
    renderWithProviders(
      <Header>
        <HeaderScopeSwitchers />
      </Header>,
      { scope: scopeAtGraphqlApi() },
    );

    expect(await screen.findByText('Countries API')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Select API', hidden: true })).not.toBeInTheDocument();
  });
});

/**
 * Switching APIs keeps the current page, as the org and project switchers do
 * (`findTargetAtScope`) — but a GraphQL API only takes that page when it has
 * a GraphQL equivalent, and otherwise opens on its own Overview rather than a
 * REST path its handle would not resolve on.
 */
describe('HeaderScopeSwitchers — switching from a REST API to a GraphQL API', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(useAllRestApis).mockReturnValue(restApisResult(REST_API_LIST));
    vi.mocked(useAllGraphQLApis).mockReturnValue(graphqlApisResult(GRAPHQL_API_LIST));
  });

  const switchToGraphqlApiFrom = async (route: string) => {
    const { user } = renderWithProviders(
      <AppShell>
        <AppShell.Navbar>
          <Header>
            <HeaderScopeSwitchers />
          </Header>
        </AppShell.Navbar>
        <Located />
      </AppShell>,
      {
        route,
        scope: makeConsoleScope({
          isApiScope: true,
          params: { apiHandler: 'orders-api', orgHandle: ORG, projectHandler: PROJECT_ID },
          project: PROJECT,
          projects: [PROJECT],
        }),
      },
    );
    await user.click(trigger('Switch api'));
    await user.click(await screen.findByRole('menuitemradio', { name: /Countries API/ }));
  };

  it('stays on the page the GraphQL API also has', async () => {
    await switchToGraphqlApiFrom(routes.apiDevelopPolicies(ORG, PROJECT_ID, 'orders-api'));

    expect(
      screen.getByText(`at ${routes.graphqlApiDevelopPolicies(ORG, PROJECT_ID, 'countries-graphql-api')}`),
    ).toBeInTheDocument();
  });

  it('opens the GraphQL API’s Overview from a REST-only page', async () => {
    await switchToGraphqlApiFrom(routes.apiDevelopDefinition(ORG, PROJECT_ID, 'orders-api'));

    expect(screen.getByText(`at ${routes.graphqlApi(ORG, PROJECT_ID, 'countries-graphql-api')}`)).toBeInTheDocument();
  });
});

const organizationTrigger = () => trigger('Switch organization');

const ACME = anOrganization({ id: 'acme-org', displayName: 'Acme' });
const GLOBEX = anOrganization({ id: 'globex-org', displayName: 'Globex' });

const renderSwitchers = (
  organizations: ReturnType<typeof anOrganization>[],
  authState: AuthState = makeAuthState(),
) =>
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
      authState,
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

  it('names the current organization in the trail', () => {
    renderSwitchers([ACME]);

    expect(screen.getByRole('button', { name: 'Acme', hidden: true })).toBeInTheDocument();
  });

  it('lists the other organizations in the switch menu', async () => {
    const { user } = renderSwitchers([ACME, GLOBEX]);

    await user.click(organizationTrigger());

    expect(await screen.findByRole('menuitemradio', { name: /Globex/ })).toBeInTheDocument();
  });

  // The Platform API lists only the organizations the user has used; where the
  // session switches organizations, the identity provider lists them all.
  it("lists the identity provider's organizations where the session switches", async () => {
    const { user } = renderSwitchers(
      [ACME],
      makeAuthState({
        canSwitchOrganization: true,
        organizations: [
          { id: 'ou-acme', handle: ACME.id, name: 'Acme' },
          { id: 'ou-initech', handle: 'initech-org', name: 'Initech' },
        ],
      }),
    );

    await user.click(organizationTrigger());

    expect(await screen.findByRole('menuitemradio', { name: /Initech/ })).toBeInTheDocument();
  });
});

/** Renders the current location's pathname and router state, so a test sees where a click landed and what it carried. */
function LocatedWithState() {
  const location = useLocation();
  return (
    <>
      <span>{`at ${location.pathname}`}</span>
      <span data-testid="state">{JSON.stringify(location.state ?? null)}</span>
    </>
  );
}

const SCENARIO_PROJECTS = [
  aProject({ id: 'retail-apis', displayName: 'Retail APIs', description: 'Everything the shops call' }),
  aProject({ id: 'plain-project', displayName: 'Plain project', description: undefined }),
];

const DESCRIBED_REST_APIS = [
  { ...REST_API_LIST[0], description: 'Places and tracks orders' },
  { ...REST_API_LIST[0], id: 'bare-api', displayName: 'Bare API', description: undefined },
] as RestApi[];

const renderTrail = (
  scope: ReturnType<typeof makeConsoleScope>,
  options: { route?: string; permissionMode?: 'enforce' | 'permissive'; scopes?: string[] } = {},
) =>
  renderWithProviders(
    <AppShell>
      <AppShell.Navbar>
        <Header>
          <HeaderScopeSwitchers />
        </Header>
      </AppShell.Navbar>
      <LocatedWithState />
    </AppShell>,
    {
      route: options.route,
      permissionMode: options.permissionMode,
      authState: makeAuthState({
        user: { name: 'Test User', email: 'test.user@example.com', scopes: options.scopes },
      }),
      scope,
    },
  );

describe('HeaderScopeSwitchers — project menu', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(useAllRestApis).mockReturnValue(restApisResult([]));
    vi.mocked(useAllGraphQLApis).mockReturnValue(graphqlApisResult([]));
  });

  const scopeInProject = () =>
    makeConsoleScope({
      project: SCENARIO_PROJECTS[0],
      projects: SCENARIO_PROJECTS,
      params: { orgHandle: ORG, projectHandler: 'retail-apis' },
    });

  it('shows a search field and each project’s name only, with no description or id', async () => {
    const { user } = renderTrail(scopeInProject());

    await user.click(trigger('Switch project'));

    // Only two projects — below Oxygen's own search threshold.
    expect(await screen.findByPlaceholderText('Search projects...')).toBeInTheDocument();
    expect(screen.queryByText('Everything the shops call')).not.toBeInTheDocument();
    expect(screen.queryByText('plain-project')).not.toBeInTheDocument();
    expect(screen.queryByText('retail-apis')).not.toBeInTheDocument();
  });

  it('opens the chosen project from the organization’s Projects page', async () => {
    // The Projects page has no project-level counterpart, so the current page
    // resolves back to the same URL: picking a project must still leave it.
    const { user } = renderTrail(
      makeConsoleScope({ projects: SCENARIO_PROJECTS, params: { orgHandle: ORG } }),
      { route: routes.projects(ORG) },
    );

    await user.click(trigger('Select Project'));
    await user.click(await screen.findByRole('menuitemradio', { name: /Retail APIs/ }));

    expect(screen.getByText(`at ${routes.projectHome(ORG, 'retail-apis')}`)).toBeInTheDocument();
  });

  it('opens the new-project page from the New project action', async () => {
    const { user } = renderTrail(scopeInProject());

    await user.click(trigger('Switch project'));
    await user.click(await screen.findByRole('menuitem', { name: 'New project' }));

    expect(screen.getByText(`at ${routes.newProject(ORG)}`)).toBeInTheDocument();
  });

  it('leaves out New project for a user who cannot create projects', async () => {
    const { user } = renderTrail(scopeInProject(), { permissionMode: 'enforce', scopes: [] });

    await user.click(trigger('Switch project'));

    await screen.findByRole('menuitemradio', { name: /Retail APIs/ });
    expect(screen.queryByRole('menuitem', { name: 'New project' })).not.toBeInTheDocument();
  });
});

describe('HeaderScopeSwitchers — API menu', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(useAllRestApis).mockReturnValue(restApisResult(DESCRIBED_REST_APIS));
    vi.mocked(useAllGraphQLApis).mockReturnValue(graphqlApisResult(GRAPHQL_API_LIST));
  });

  const scopeInApi = () =>
    makeConsoleScope({
      isApiScope: true,
      params: { apiHandler: 'orders-api', orgHandle: ORG, projectHandler: PROJECT_ID },
      project: PROJECT,
      projects: [PROJECT],
    });

  it('badges each API with its type, and shows a description only where there is one', async () => {
    const { user } = renderTrail(scopeInApi());

    await user.click(trigger('Switch api'));

    const orders = await screen.findByRole('menuitemradio', { name: /Orders API/ });
    expect(within(orders).getByText('REST')).toBeInTheDocument();
    expect(within(orders).getByText('Places and tracks orders')).toBeInTheDocument();
    const graphql = screen.getByRole('menuitemradio', { name: /Countries API/ });
    expect(within(graphql).getByText('GraphQL')).toBeInTheDocument();
    // No description means the entry is the name alone: not the id in its place.
    const bare = screen.getByRole('menuitemradio', { name: /Bare API/ });
    expect(within(bare).queryByText('bare-api')).not.toBeInTheDocument();
    expect(screen.getByPlaceholderText('Search APIs...')).toBeInTheDocument();
  });

  it('opens the creation wizard from New API, reporting the breadcrumb as its entry point', async () => {
    const { user } = renderTrail(scopeInApi());

    await user.click(trigger('Switch api'));
    await user.click(await screen.findByRole('menuitem', { name: 'New API' }));

    expect(screen.getByText(`at ${routes.newApi(ORG, PROJECT_ID)}`)).toBeInTheDocument();
    expect(screen.getByTestId('state')).toHaveTextContent('header-breadcrumb');
  });

  it('keeps the APIs that loaded selectable when the other kind fails to load', async () => {
    vi.mocked(useAllGraphQLApis).mockReturnValue({
      ...graphqlApisResult([]),
      error: new Error('graphql down'),
    } as ReturnType<typeof useAllGraphQLApis>);
    const { user } = renderTrail(scopeInApi());

    await user.click(trigger('Switch api'));

    expect(await screen.findByRole('menuitemradio', { name: /Orders API/ })).toBeInTheDocument();
    expect(screen.queryByText('Failed to load APIs')).not.toBeInTheDocument();
    expect(screen.getByText('APIs (some failed to load)')).toBeInTheDocument();
  });

  it('shows the blocking error only when no API loaded at all', async () => {
    vi.mocked(useAllRestApis).mockReturnValue({
      ...restApisResult([]),
      error: new Error('rest down'),
    } as ReturnType<typeof useAllRestApis>);
    vi.mocked(useAllGraphQLApis).mockReturnValue(graphqlApisResult([]));
    const { user } = renderTrail(
      makeConsoleScope({ project: PROJECT, projects: [PROJECT], params: { orgHandle: ORG, projectHandler: PROJECT_ID } }),
    );

    await user.click(trigger('Select API'));

    expect(await screen.findByText('Failed to load APIs')).toBeInTheDocument();
  });

  it('leaves out New API for a user who can create neither kind', async () => {
    const { user } = renderTrail(scopeInApi(), { permissionMode: 'enforce', scopes: [] });

    await user.click(trigger('Switch api'));

    await screen.findByRole('menuitemradio', { name: /Orders API/ });
    expect(screen.queryByRole('menuitem', { name: 'New API' })).not.toBeInTheDocument();
  });
});

describe('HeaderScopeSwitchers — clicking a name in the trail', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(useAllRestApis).mockReturnValue(restApisResult(REST_API_LIST));
    vi.mocked(useAllGraphQLApis).mockReturnValue(graphqlApisResult([]));
  });

  const scopeInApi = () =>
    makeConsoleScope({
      isApiScope: true,
      params: { apiHandler: 'orders-api', orgHandle: ORG, projectHandler: PROJECT_ID },
      project: PROJECT,
      projects: [PROJECT],
    });

  it('goes up to the project from the API', async () => {
    const { user } = renderTrail(scopeInApi(), {
      route: routes.apiDevelopPolicies(ORG, PROJECT_ID, 'orders-api'),
    });

    await user.click(screen.getByRole('button', { name: PROJECT.displayName!, hidden: true }));

    expect(screen.getByText(new RegExp(`^at /organizations/${ORG}/projects/${PROJECT_ID}`))).toBeInTheDocument();
    expect(screen.queryByText(/orders-api/)).not.toBeInTheDocument();
  });

  it('goes up to the organization from the project', async () => {
    const { user } = renderTrail(makeConsoleScope({ project: PROJECT, projects: [PROJECT] }), {
      route: routes.projectHome(ORG, PROJECT_ID),
    });

    await user.click(screen.getByRole('button', { name: anOrganization().displayName!, hidden: true }));

    expect(screen.queryByText(new RegExp(`projects/${PROJECT_ID}`))).not.toBeInTheDocument();
  });
});

/*
 * The selection is kept where every app using Oxygen's header reads it, so the
 * next visit starts where the user left off.
 */
describe('HeaderScopeSwitchers — remembering where the user is', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    window.localStorage.clear();
    vi.mocked(useAllRestApis).mockReturnValue(restApisResult(REST_API_LIST));
    vi.mocked(useAllGraphQLApis).mockReturnValue(graphqlApisResult(GRAPHQL_API_LIST));
  });

  it('stores the organization and project in scope', () => {
    renderWithProviders(
      <Header>
        <HeaderScopeSwitchers />
      </Header>,
      { scope: scopeAtProject() },
    );

    expect(readHeaderBreadcrumbsState()?.value).toEqual({
      organization: ORG,
      project: PROJECT_ID,
    });
  });

  it('keeps the last project while the user is on an organization-level page', () => {
    window.localStorage.setItem(
      'oxygen-ui:header-breadcrumbs',
      JSON.stringify({ version: 1, value: { organization: ORG, project: 'earlier' }, updatedAt: 1 }),
    );

    renderWithProviders(
      <Header>
        <HeaderScopeSwitchers />
      </Header>,
      { scope: makeConsoleScope({ params: { orgHandle: ORG }, project: undefined, projects: [PROJECT] }) },
    );

    expect(readHeaderBreadcrumbsState()?.value).toEqual({
      organization: ORG,
      project: 'earlier',
    });
  });
});
