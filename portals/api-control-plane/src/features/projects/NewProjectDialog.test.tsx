import { describe, expect, it, vi } from 'vitest';

import { renderWithProviders, screen, waitFor } from '../../test/utils';
import type { Project } from '../../types/domain';
import { NewProjectDialog } from './NewProjectDialog';

const ORG = 'api-platform-demo';

const created: Project = {
  id: 'proj-new',
  orgId: 'org-1',
  name: 'Billing',
  handler: 'proj-new',
};

function setup(
  createProject = vi.fn().mockResolvedValue(created)
) {
  const onClose = vi.fn();
  const utils = renderWithProviders(
    <NewProjectDialog onClose={onClose} open orgHandle={ORG} />,
    { apiClient: { createProject } }
  );
  return { ...utils, createProject, onClose };
}

describe('NewProjectDialog', () => {
  it('disables Create until a name is entered', () => {
    setup();
    expect(screen.getByRole('button', { name: 'Create' })).toBeDisabled();
  });

  it('creates a project with name + description, then closes', async () => {
    const { user, createProject, onClose } = setup();

    await user.type(screen.getByLabelText(/Name/), 'Billing');
    await user.type(screen.getByLabelText(/Description/), 'Invoices');
    await user.click(screen.getByRole('button', { name: 'Create' }));

    await waitFor(() =>
      expect(createProject).toHaveBeenCalledWith(ORG, {
        name: 'Billing',
        description: 'Invoices',
      })
    );
    await waitFor(() => expect(onClose).toHaveBeenCalled());
  });

  it('keeps the dialog open and surfaces the error message on failure', async () => {
    const failing = vi
      .fn()
      .mockRejectedValue(new Error('Project already exists in organization'));
    const { user, onClose } = setup(failing);

    await user.type(screen.getByLabelText(/Name/), 'Retail APIs');
    await user.click(screen.getByRole('button', { name: 'Create' }));

    await screen.findByText('Project already exists in organization');
    expect(onClose).not.toHaveBeenCalled();
  });
});
