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

import { Route, Routes } from 'react-router-dom';
import { beforeEach, describe, expect, it } from 'vitest';

import { ApiScopeProvider } from '@/api/core/ApiScopeProvider';
import { resetHttpClient } from '@/api/core/http';
import {
  aProject,
  collection,
  failure,
  noContent,
  recorder,
  type ProjectFixture,
  type Recorder,
} from '@/test/msw';
import { server } from '@/test/server';
import { renderWithProviders, screen, waitFor, within } from '@/test/utils';
import { makeConsoleScope } from '@/test/mockScope';
import { routes } from '@/routes/paths';
import { ProjectListPage } from './ProjectListPage';

const ORG = 'api-platform-demo';

const projectFixtures: ProjectFixture[] = [
  aProject({ id: 'retail', displayName: 'Retail APIs' }),
  aProject({ id: 'internal-tools', displayName: 'Internal Tools' }),
];

/** Enough projects to force a second page at the default size of 12. */
const manyProjects = Array.from({ length: 14 }, (_, index) =>
  aProject({
    id: `project-${index + 1}`,
    displayName: `Project ${index + 1}`,
  }),
);

let requests: Recorder;

/**
 * The page's hooks read `ApiScopeContext`, not the console scope, so the
 * provider has to be mounted here — without it every query stays `enabled:
 * false` and the page renders its loading state forever.
 */
function renderPage() {
  return renderWithProviders(
    <ApiScopeProvider orgId={ORG}>
      <Routes>
        <Route path="/organizations/:orgHandle/projects" element={<ProjectListPage />} />
        {/* Stands in for the project home, so opening a project is observable. */}
        <Route path={routes.projectHome()} element={<div>project home</div>} />
      </Routes>
    </ApiScopeProvider>,
    {
      route: `/organizations/${ORG}/projects`,
      scope: makeConsoleScope(),
    },
  );
}

beforeEach(() => {
  requests = recorder();
  resetHttpClient();
  // Cards count the APIs in their project; every card issues one of each.
  server.use(collection('/rest-apis', []));
  server.use(collection('/graphql-apis', []));
});

describe('ProjectListPage', () => {
  it('shows the loading state before the first response lands', () => {
    server.use(collection('/projects', projectFixtures));
    renderPage();
    expect(screen.getByText('Loading projects')).toBeInTheDocument();
  });

  it('shows an error state with the reason', async () => {
    server.use(
      failure('get', '/projects', 500, 'INTERNAL_SERVER_ERROR', {
        message: 'boom',
      }),
    );
    renderPage();
    expect(await screen.findByText(/Unable to load projects\./)).toBeInTheDocument();
  });

  it('shows the empty state when the organization has no projects', async () => {
    server.use(collection('/projects', []));
    renderPage();
    expect(await screen.findByText('Create your first Project')).toBeInTheDocument();
  });

  it('renders the first page and asks the server for the paging window', async () => {
    server.use(collection('/projects', projectFixtures, { record: requests }));
    renderPage();

    expect(await screen.findByText('Retail APIs')).toBeInTheDocument();
    expect(screen.getByText('Internal Tools')).toBeInTheDocument();
    expect(requests.last()?.params.get('limit')).toBe('12');
    expect(requests.last()?.params.get('offset')).toBe('0');
  });

  it('searches server-side rather than filtering the current page', async () => {
    server.use(collection('/projects', projectFixtures, { record: requests }));
    const { user } = renderPage();

    await screen.findByText('Retail APIs');
    await user.type(screen.getByPlaceholderText('Search projects'), 'internal');

    await waitFor(() => expect(requests.last()?.params.get('query')).toBe('internal'));
    await waitFor(() => expect(screen.queryByText('Retail APIs')).not.toBeInTheDocument());
    expect(screen.getByText('Internal Tools')).toBeInTheDocument();
  });

  it('requests projects newest-first', async () => {
    server.use(collection('/projects', projectFixtures, { record: requests }));
    renderPage();

    await screen.findByText('Retail APIs');
    expect(requests.last()?.params.get('sortBy')).toBe('createdAt');
    expect(requests.last()?.params.get('sortOrder')).toBe('desc');
  });

  it('returns to the first page when the search changes', async () => {
    server.use(collection('/projects', manyProjects, { record: requests }));
    const { user } = renderPage();

    await screen.findByText('Project 1');
    await user.click(screen.getByRole('button', { name: /next page/i }));
    await waitFor(() => expect(requests.last()?.params.get('offset')).toBe('12'));

    await user.type(screen.getByPlaceholderText('Search projects'), 'Project 1');

    await waitFor(() => expect(requests.last()?.params.get('offset')).toBe('0'));
  });

  it('requests the next page when the pagination control advances', async () => {
    server.use(collection('/projects', manyProjects, { record: requests }));
    const { user } = renderPage();

    await screen.findByText('Project 1');
    expect(screen.queryByText('Project 13')).not.toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: /next page/i }));

    await waitFor(() => expect(requests.last()?.params.get('offset')).toBe('12'));
    expect(await screen.findByText('Project 13')).toBeInTheDocument();
  });

  it('switches to the table view and deletes from a row', async () => {
    server.use(
      collection('/projects', projectFixtures),
      noContent('delete', '/projects/:projectId', { record: requests }),
    );
    const { user } = renderPage();

    await screen.findByText('Retail APIs');
    await user.click(screen.getByRole('button', { name: 'List view' }));

    const rows = await screen.findByTestId('project-list-view');
    expect(within(rows).getByText('Retail APIs')).toBeInTheDocument();

    // The row's own delete button, in place of the settings action the card
    // used to carry.
    await user.click(within(rows).getByRole('button', { name: 'Delete Retail APIs' }));

    const dialog = screen.getByRole('dialog');
    await user.type(within(dialog).getByRole('textbox'), 'Retail APIs');
    await user.click(within(dialog).getByRole('button', { name: 'Delete' }));

    await waitFor(() => expect(requests.count()).toBe(1));
    expect(requests.last()?.url.pathname).toMatch(/\/projects\/retail$/);
  });

  it('opens a project from the keyboard in both views', async () => {
    // Pointer users get the whole card/row as the target; without an explicit
    // focusable role, keyboard users had no way in at all — the delete button
    // was the only thing in a row they could reach.
    server.use(collection('/projects', projectFixtures));
    const { user } = renderPage();

    await screen.findByText('Retail APIs');
    const card = screen.getByRole('button', { name: 'Open Retail APIs' });
    card.focus();
    await user.keyboard('{Enter}');

    expect(await screen.findByText('project home')).toBeInTheDocument();
  });

  it('opens a project from a table row with Space', async () => {
    server.use(collection('/projects', projectFixtures));
    const { user } = renderPage();

    await screen.findByText('Retail APIs');
    await user.click(screen.getByRole('button', { name: 'List view' }));

    const rows = await screen.findByTestId('project-list-view');
    const row = within(rows).getByRole('button', { name: 'Open Retail APIs' });
    row.focus();
    await user.keyboard(' ');

    expect(await screen.findByText('project home')).toBeInTheDocument();
  });

  it('leaves the row alone when the delete button takes the keypress', async () => {
    // The delete button's key events bubble through the row, so Enter on it
    // must open the confirm dialog and not also navigate into the project.
    server.use(collection('/projects', projectFixtures));
    const { user } = renderPage();

    await screen.findByText('Retail APIs');
    await user.click(screen.getByRole('button', { name: 'List view' }));

    const rows = await screen.findByTestId('project-list-view');
    within(rows).getByRole('button', { name: 'Delete Retail APIs' }).focus();
    await user.keyboard('{Enter}');

    expect(await screen.findByRole('dialog')).toBeInTheDocument();
    expect(screen.queryByText('project home')).not.toBeInTheDocument();
  });

  it('deletes a project after type-to-confirm', async () => {
    server.use(
      collection('/projects', projectFixtures),
      noContent('delete', '/projects/:projectId', { record: requests }),
    );
    const { user } = renderPage();

    await screen.findByText('Retail APIs');
    // The card keeps its overflow menu; the row view exposes delete directly.
    const cards = screen.getAllByRole('button', { name: 'Project actions' });
    await user.click(cards[0]);
    await user.click(await screen.findByRole('menuitem', { name: 'Delete' }));

    // Type-to-confirm guards the irreversible delete.
    const dialog = screen.getByRole('dialog');
    const confirmButton = within(dialog).getByRole('button', { name: 'Delete' });
    expect(confirmButton).toBeDisabled();

    await user.type(within(dialog).getByRole('textbox'), 'Retail APIs');
    await user.click(confirmButton);

    await waitFor(() => expect(requests.count()).toBe(1));
    expect(requests.last()?.url.pathname).toMatch(/\/projects\/retail$/);
  });
});
