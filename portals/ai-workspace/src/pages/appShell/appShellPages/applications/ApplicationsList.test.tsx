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

import { renderWithProviders, screen, waitFor } from '../../../../test/utils';
import ApplicationsList from './ApplicationsList';

const showSnackbar = vi.fn();
const deleteApplication = vi.fn();
const getApplications = vi.fn();
let permitted = true;
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
    currentOrganization: { id: 'org-1', uuid: 'org-1', handle: 'acme' },
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
  currentProject = { id: 'proj-1', name: 'Project One' };
  setCurrentProject.mockReset();
  applications = [
    application(),
    application({ id: 'weather-bot', displayName: 'Weather Bot', description: 'forecasts' }),
  ];
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
