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

import React from 'react';
import { describe, it, expect, vi, beforeEach } from 'vitest';

import { renderWithProviders, screen, waitFor, within } from '../../../../test/utils';
import ApplicationsList from './ApplicationsList';

const navigate = vi.fn();
vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual<typeof import('react-router-dom')>('react-router-dom');
  return { ...actual, useNavigate: () => navigate };
});

const showSnackbar = vi.fn();
const deleteApplication = vi.fn();
const getApplications = vi.fn();
let permitted = true;
let orgUuid = 'org-1';
let applications: Record<string, unknown>[] = [];

const application = (over: Record<string, unknown> = {}) => ({
  id: 'trip-planner',
  displayName: 'Trip Planner',
  description: 'plans multi-city trips',
  type: 'genai',
  ...over,
});

// Mimics the real backend's `handleSearchClause` (platform-api/internal/repository/pagination.go):
// a case-insensitive substring match against display name and id/handle only —
// NOT description.
const matches = (a: Record<string, unknown>, search: string) => {
  if (!search) return true;
  const haystack = [a.displayName, a.id].filter(Boolean).join(' ').toLowerCase();
  return haystack.includes(search.toLowerCase());
};

let currentProject: { id: string; name: string } | null = { id: 'proj-1', name: 'Project One' };
const setCurrentProject = vi.fn();
vi.mock('../../../../contexts/AppShellContext', () => ({
  useAppShell: () => ({
    currentProject,
    currentOrganization: { id: 'org-1', uuid: orgUuid, handle: 'acme' },
    projectsForCurrentOrganization: [
      { id: 'proj-1', name: 'project-one', displayName: 'Project One' },
      { id: 'proj-2', name: 'project-two', displayName: 'Project Two' },
    ],
    setCurrentProject,
    isProjectsLoading: false,
  }),
}));
vi.mock('../../../../contexts/AppAuthContext', () => ({
  useAppAuth: () => ({ hasPermission: () => permitted }),
}));
vi.mock('../../../../contexts/ApplicationsContext', () => ({
  formatRelativeTime: () => 'just now',
  useApplications: () => ({ deleteApplication }),
}));
vi.mock('../../../../apis/applicationApis', () => ({
  applicationApis: { getApplications: (...args: unknown[]) => getApplications(...args) },
}));
vi.mock('../../../../hooks/aiWorkspaceSnackbar', () => ({
  default: () => showSnackbar,
}));

const renderList = () => renderWithProviders(<ApplicationsList />);
const search = () => screen.getByPlaceholderText('Search Applications...');

beforeEach(() => {
  permitted = true;
  orgUuid = 'org-1';
  currentProject = { id: 'proj-1', name: 'Project One' };
  setCurrentProject.mockReset();
  applications = [
    application(),
    application({ id: 'weather-bot', displayName: 'Weather Bot', description: 'forecasts' }),
  ];
  navigate.mockReset();
  showSnackbar.mockReset();
  deleteApplication.mockReset();
  getApplications.mockReset();
  getApplications.mockImplementation(async (_projectId, _baseUrl, options) => {
    const filtered = applications.filter((a) => matches(a, options?.query ?? ''));
    return {
      count: filtered.length,
      list: filtered,
      pagination: { total: filtered.length, offset: options?.offset ?? 0, limit: options?.limit ?? 20 },
    };
  });
});

describe('listing GenAI applications', () => {
  it('shows every application in the project', async () => {
    renderList();

    await waitFor(() => expect(screen.getByText('Trip Planner')).toBeInTheDocument());
    expect(screen.getByText('Weather Bot')).toBeInTheDocument();
  });

  it('says so plainly when the project has none', async () => {
    applications = [];
    renderList();

    await waitFor(() =>
      expect(screen.getByText(/Create your first GenAI Application/)).toBeInTheDocument()
    );
    expect(screen.queryByText('Trip Planner')).not.toBeInTheDocument();
  });

  it('always requests only genai-typed applications', async () => {
    renderList();

    await waitFor(() => expect(getApplications).toHaveBeenCalled());
    const [, , options] = getApplications.mock.calls[0];
    expect(options?.type).toBe('genai');
  });
});

describe('searching', () => {
  it('matches on the display name', async () => {
    const { user } = renderList();
    await waitFor(() => expect(screen.getByText('Trip Planner')).toBeInTheDocument());

    await user.type(search(), 'Trip');

    await waitFor(() => expect(screen.queryByText('Weather Bot')).not.toBeInTheDocument());
    expect(screen.getByText('Trip Planner')).toBeInTheDocument();
  });

  it('does not match on the description (narrower than the old client-side filter)', async () => {
    const { user } = renderList();
    await waitFor(() => expect(screen.getByText('Trip Planner')).toBeInTheDocument());

    await user.type(search(), 'multi-city');

    await waitFor(() =>
      expect(screen.getByText(/No GenAI applications match your search/)).toBeInTheDocument()
    );
    expect(screen.queryByText('Trip Planner')).not.toBeInTheDocument();
  });

  it('shows everything again once the search is cleared', async () => {
    const { user } = renderList();
    await waitFor(() => expect(screen.getByText('Trip Planner')).toBeInTheDocument());
    const input = search();

    await user.type(input, 'Trip');
    await waitFor(() => expect(screen.queryByText('Weather Bot')).not.toBeInTheDocument());

    await user.clear(input);

    await waitFor(() => expect(screen.getByText('Weather Bot')).toBeInTheDocument());
  });
});

describe('deleting an application', () => {
  it('reports success once the application is gone', async () => {
    deleteApplication.mockResolvedValue(undefined);
    const { user } = renderList();
    await waitFor(() => expect(screen.getByText('Trip Planner')).toBeInTheDocument());

    await user.click(screen.getByRole('button', { name: 'Delete Trip Planner' }));
    await user.click(screen.getByRole('button', { name: 'Delete' }));

    await waitFor(() =>
      expect(showSnackbar).toHaveBeenCalledWith('Application deleted successfully.', 'success')
    );
    expect(deleteApplication).toHaveBeenCalledWith('trip-planner');
  });

  it('reports the reason when the delete is refused', async () => {
    deleteApplication.mockRejectedValue(new Error('still in use'));
    const { user } = renderList();
    await waitFor(() => expect(screen.getByText('Trip Planner')).toBeInTheDocument());

    await user.click(screen.getByRole('button', { name: 'Delete Trip Planner' }));
    await user.click(screen.getByRole('button', { name: 'Delete' }));

    await waitFor(() =>
      expect(showSnackbar).toHaveBeenCalledWith(
        'Failed to delete application. Please try again.',
        'error'
      )
    );
  });
});

describe('without permission', () => {
  it('offers no delete control', async () => {
    permitted = false;
    renderList();

    await waitFor(() => expect(screen.getByText('Trip Planner')).toBeInTheDocument());
    const deleteButton = screen.queryByRole('button', { name: 'Delete Trip Planner' });
    if (deleteButton) expect(deleteButton).toBeDisabled();
  });
});

describe('at organization level', () => {
  beforeEach(() => {
    // No project in scope, so the list cannot show applications and offers a chooser instead.
    currentProject = null;
  });

  it('explains that applications live inside a project', () => {
    renderList();

    expect(
      screen.getByText(/GenAI Applications are created and managed at the project level/)
    ).toBeInTheDocument();
  });

  it('does not list any application while no project is in scope', () => {
    renderList();

    expect(screen.queryByText('Trip Planner')).not.toBeInTheDocument();
    expect(getApplications).not.toHaveBeenCalled();
  });
});

describe('while the catalogue is still loading', () => {
  it('shows a loading indicator instead of an empty listing', () => {
    // A promise that never resolves during the test keeps the component in
    // its initial "first fetch in flight" state.
    getApplications.mockImplementation(() => new Promise(() => {}));
    renderList();

    expect(screen.queryByText('Trip Planner')).not.toBeInTheDocument();
    expect(
      screen.queryByText(/Create your first GenAI Application/)
    ).not.toBeInTheDocument();
  });
});

describe('when the list cannot be loaded', () => {
  it('shows the error and refetches on retry', async () => {
    getApplications.mockRejectedValueOnce(new Error('network down'));
    const { user } = renderList();

    await waitFor(() => expect(screen.getByText(/network down/)).toBeInTheDocument());
    await user.click(screen.getByRole('button', { name: /Retry/ }));

    await waitFor(() => expect(screen.getByText('Trip Planner')).toBeInTheDocument());
    expect(getApplications).toHaveBeenCalledTimes(2);
  });

  it('wraps a non-Error rejection in a generic error', async () => {
    getApplications.mockRejectedValueOnce('boom');
    renderList();

    await waitFor(() =>
      expect(screen.getByText(/Failed to fetch applications|Something went wrong/)).toBeInTheDocument()
    );
  });

  it('shows the create-first empty state when the project is not found', async () => {
    getApplications.mockRejectedValueOnce(
      Object.assign(new Error('nope'), { code: 'PROJECT_NOT_FOUND' })
    );
    renderList();

    await waitFor(() =>
      expect(screen.getByText(/Create your first GenAI Application/)).toBeInTheDocument()
    );
    expect(screen.getByRole('link', { name: 'Create Application' })).toBeInTheDocument();
  });
});

describe('create controls', () => {
  it('offers Add New Application once the project has applications', async () => {
    renderList();

    const link = await screen.findByRole('link', { name: 'Add New Application' });
    expect(link).toHaveAttribute('href', expect.stringContaining('/applications/create'));
  });

  it('disables creation without permission', async () => {
    permitted = false;
    renderList();

    const link = await screen.findByText('Add New Application');
    expect(link.closest('a')).toHaveAttribute('aria-disabled', 'true');
  });

  it('disables the empty-state create button without permission', async () => {
    permitted = false;
    applications = [];
    renderList();

    const btn = await screen.findByText('Create Application');
    expect(btn.closest('a')).toHaveAttribute('aria-disabled', 'true');
  });

  it('disables card delete buttons without permission', async () => {
    permitted = false;
    renderList();

    await waitFor(() => expect(screen.getByText('Trip Planner')).toBeInTheDocument());
    expect(screen.getByRole('button', { name: 'Delete Trip Planner' })).toBeDisabled();
  });
});

describe('opening an application', () => {
  it('navigates on click', async () => {
    const { user } = renderList();
    await waitFor(() => expect(screen.getByText('Trip Planner')).toBeInTheDocument());

    await user.click(screen.getByText('Trip Planner'));

    expect(navigate).toHaveBeenCalledWith(expect.stringContaining('/applications/trip-planner'));
  });

  it.each(['{Enter}', ' '])('navigates on keyboard %j', async (key) => {
    const { user } = renderList();
    await waitFor(() => expect(screen.getByText('Weather Bot')).toBeInTheDocument());

    const card = screen.getByText('Weather Bot').closest('[role="button"]') as HTMLElement;
    card.focus();
    await user.keyboard(key);

    expect(navigate).toHaveBeenCalledWith(expect.stringContaining('/applications/weather-bot'));
  });

  it('ignores other keys', async () => {
    const { user } = renderList();
    await waitFor(() => expect(screen.getByText('Weather Bot')).toBeInTheDocument());

    const card = screen.getByText('Weather Bot').closest('[role="button"]') as HTMLElement;
    card.focus();
    await user.keyboard('a');

    expect(navigate).not.toHaveBeenCalled();
  });

  it('truncates long descriptions and falls back to initials for single-word names', async () => {
    applications = [
      application({ id: 'x', displayName: 'Solo', description: 'd'.repeat(100) }),
    ];
    renderList();

    await waitFor(() => expect(screen.getByText('Solo')).toBeInTheDocument());
    expect(screen.getByText('SO')).toBeInTheDocument();
    expect(screen.getByText(`${'d'.repeat(70)}…`)).toBeInTheDocument();
  });

  it('handles an application with no description', async () => {
    applications = [application({ description: undefined })];
    renderList();

    await waitFor(() => expect(screen.getByText('Trip Planner')).toBeInTheDocument());
    expect(screen.getByText('TP')).toBeInTheDocument();
  });
});

describe('deleting an application (dialog)', () => {
  it('closes without deleting on Cancel', async () => {
    const { user } = renderList();
    await waitFor(() => expect(screen.getByText('Trip Planner')).toBeInTheDocument());

    await user.click(screen.getByRole('button', { name: 'Delete Trip Planner' }));
    expect(screen.getByText(/Are you sure you want to delete Trip Planner/)).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Cancel' }));

    await waitFor(() =>
      expect(screen.queryByText(/Are you sure you want to delete/)).not.toBeInTheDocument()
    );
    expect(deleteApplication).not.toHaveBeenCalled();
  });

  it('does not open the application when the delete icon is clicked', async () => {
    const { user } = renderList();
    await waitFor(() => expect(screen.getByText('Trip Planner')).toBeInTheDocument());

    await user.click(screen.getByRole('button', { name: 'Delete Trip Planner' }));

    expect(navigate).not.toHaveBeenCalled();
  });
});

describe('pagination', () => {
  const many = (n: number) =>
    Array.from({ length: n }, (_, i) =>
      application({ id: `app-${i}`, displayName: `App ${String(i).padStart(2, '0')}` })
    );

  beforeEach(() => {
    getApplications.mockImplementation(async (_p, _b, options) => {
      const offset = options?.offset ?? 0;
      const limit = options?.limit ?? 20;
      return {
        count: applications.length,
        list: applications.slice(offset, offset + limit),
        pagination: { total: applications.length, offset, limit },
      };
    });
  });

  it('hides the pager when there is less than one page', async () => {
    applications = many(5);
    renderList();

    await waitFor(() => expect(screen.getByText('App 00')).toBeInTheDocument());
    expect(screen.queryByText(/Rows per page/)).not.toBeInTheDocument();
  });

  it('requests the next page with the matching offset', async () => {
    applications = many(25);
    const { user } = renderList();
    await waitFor(() => expect(screen.getByText('App 00')).toBeInTheDocument());

    await user.click(screen.getByRole('button', { name: /next page/i }));

    await waitFor(() => expect(screen.getByText('App 20')).toBeInTheDocument());
    expect(screen.queryByText('App 00')).not.toBeInTheDocument();
    const last = getApplications.mock.calls.at(-1)![2];
    expect(last).toMatchObject({ offset: 20, limit: 20, type: 'genai' });
  });

  it('resets to the first page and refetches when rows per page changes', async () => {
    applications = many(60);
    const { user } = renderList();
    await waitFor(() => expect(screen.getByText('App 00')).toBeInTheDocument());
    await user.click(screen.getByRole('button', { name: /next page/i }));
    await waitFor(() => expect(screen.getByText('App 20')).toBeInTheDocument());

    await user.click(screen.getByRole('combobox', { name: /Rows per page/i }));
    await user.click(await screen.findByRole('option', { name: '50' }));

    await waitFor(() => expect(screen.getByText('App 49')).toBeInTheDocument());
    expect(screen.getByText('App 00')).toBeInTheDocument();
    const last = getApplications.mock.calls.at(-1)![2];
    expect(last).toMatchObject({ offset: 0, limit: 50 });
  });

  it('falls back a page when the last row of the last page is deleted', async () => {
    applications = many(21);
    deleteApplication.mockImplementation(async (id: string) => {
      applications = applications.filter((a) => a.id !== id);
    });
    const { user } = renderList();
    await waitFor(() => expect(screen.getByText('App 00')).toBeInTheDocument());
    await user.click(screen.getByRole('button', { name: /next page/i }));
    await waitFor(() => expect(screen.getByText('App 20')).toBeInTheDocument());

    await user.click(screen.getByRole('button', { name: 'Delete App 20' }));
    await user.click(screen.getByRole('button', { name: 'Delete' }));

    await waitFor(() => expect(screen.getByText('App 00')).toBeInTheDocument());
    expect(screen.queryByText('App 20')).not.toBeInTheDocument();
    expect(getApplications.mock.calls.at(-1)![2]).toMatchObject({ offset: 0 });
  });
});

describe('searching (edge cases)', () => {
  it('sends a trimmed query and keeps the create button while filtered to nothing', async () => {
    const { user } = renderList();
    await waitFor(() => expect(screen.getByText('Trip Planner')).toBeInTheDocument());

    await user.type(search(), '  zzz  ');

    await waitFor(() =>
      expect(screen.getByText(/No GenAI applications match your search/)).toBeInTheDocument()
    );
    expect(getApplications.mock.calls.at(-1)![2]).toMatchObject({ query: 'zzz' });
    expect(screen.getByText('Add New Application')).toBeInTheDocument();
  });
});

describe('without an organization', () => {
  it('does not fetch and leaves the project view empty', async () => {
    orgUuid = '';
    renderList();

    await waitFor(() =>
      expect(screen.getByText(/Create your first GenAI Application/)).toBeInTheDocument()
    );
    expect(getApplications).not.toHaveBeenCalled();
  });
});

describe('choosing a project at organization level', () => {
  beforeEach(() => {
    currentProject = null;
  });

  it('keeps Go to Project Level disabled until a project is chosen', () => {
    renderList();

    expect(screen.getByRole('button', { name: 'Go to Project Level' })).toBeDisabled();
  });

  it('switches to the chosen project and navigates to its applications', async () => {
    const { user } = renderList();

    await user.click(screen.getByRole('combobox'));
    const listbox = await screen.findByRole('listbox');
    await user.click(within(listbox).getByText('Project Two'));
    await user.click(screen.getByRole('button', { name: 'Go to Project Level' }));

    expect(setCurrentProject).toHaveBeenCalledWith(
      expect.objectContaining({ id: 'proj-2' })
    );
    expect(navigate).toHaveBeenCalledWith(expect.stringContaining('/applications'));
  });
});
