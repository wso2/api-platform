import { Route, Routes } from 'react-router-dom';
import type { UseQueryResult } from '@tanstack/react-query';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { projects as mockProjects } from '../../api/mocks/data';
import type { ApiClient } from '../../api/ApiClientProvider';
import { renderWithProviders, screen, waitFor, within } from '../../test/utils';
import { makeConsoleScope } from '../../test/mockScope';
import type { Project } from '../../types/domain';

vi.mock('../../api/hooks/useMvpQueries', async (importActual) => ({
  ...(await importActual<typeof import('../../api/hooks/useMvpQueries')>()),
  useProjects: vi.fn(),
}));

import { useProjects } from '../../api/hooks/useMvpQueries';
import { ProjectListPage } from './ProjectListPage';

const ORG = 'api-platform-demo';

// Minimal cast helper for the bits of the query result the page reads.
const queryResult = (overrides: Partial<UseQueryResult<Project[], Error>>) =>
  overrides as UseQueryResult<Project[], Error>;

function renderPage(apiClient?: Partial<ApiClient>) {
  return renderWithProviders(
    <Routes>
      <Route
        path="/organizations/:orgHandle/projects"
        element={<ProjectListPage />}
      />
    </Routes>,
    {
      route: `/organizations/${ORG}/projects`,
      scope: makeConsoleScope(),
      apiClient,
    }
  );
}

describe('ProjectListPage', () => {
  beforeEach(() => vi.clearAllMocks());

  it('shows the loading state', () => {
    vi.mocked(useProjects).mockReturnValue(queryResult({ isLoading: true }));
    renderPage();
    expect(screen.getByText('Loading projects')).toBeInTheDocument();
  });

  it('shows an error state with the message', () => {
    vi.mocked(useProjects).mockReturnValue(
      queryResult({ isLoading: false, error: new Error('boom') })
    );
    renderPage();
    expect(screen.getByText(/Unable to load projects\. boom/)).toBeInTheDocument();
  });

  it('shows the empty state when there are no projects', () => {
    vi.mocked(useProjects).mockReturnValue(
      queryResult({ isLoading: false, data: [] })
    );
    renderPage();
    expect(screen.getByText('No projects found')).toBeInTheDocument();
  });

  it('renders the projects and filters by search', async () => {
    vi.mocked(useProjects).mockReturnValue(
      queryResult({ isLoading: false, data: mockProjects })
    );
    const { user } = renderPage();

    expect(screen.getByText('Retail APIs')).toBeInTheDocument();
    expect(screen.getByText('Internal Tools')).toBeInTheDocument();

    await user.type(screen.getByPlaceholderText('Search projects'), 'internal');

    expect(screen.queryByText('Retail APIs')).not.toBeInTheDocument();
    expect(screen.getByText('Internal Tools')).toBeInTheDocument();
  });

  it('deletes a project after type-to-confirm', async () => {
    vi.mocked(useProjects).mockReturnValue(
      queryResult({ isLoading: false, data: mockProjects })
    );
    const deleteProject = vi.fn().mockResolvedValue(undefined);
    const { user } = renderPage({ deleteProject });

    // Open the actions menu on the first card (Retail APIs) and choose Delete.
    await user.click(screen.getAllByLabelText('Project actions')[0]);
    await user.click(screen.getByRole('menuitem', { name: /Delete/ }));

    // Type-to-confirm guards the irreversible delete.
    const dialog = screen.getByRole('dialog');
    const confirmButton = within(dialog).getByRole('button', { name: 'Delete' });
    expect(confirmButton).toBeDisabled();

    await user.type(within(dialog).getByRole('textbox'), 'Retail APIs');
    await user.click(confirmButton);

    await waitFor(() => expect(deleteProject).toHaveBeenCalledTimes(1));
    expect(deleteProject.mock.calls[0][1]).toMatchObject({ name: 'Retail APIs' });
  });
});
