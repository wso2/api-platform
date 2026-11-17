/*
 * Copyright (c) 2026, WSO2 LLC (http://www.wso2.com). All Rights Reserved.
 *
 * This software is the property of WSO2 LLC and its suppliers, if any.
 * Dissemination of any information or reproduction of any material contained
 * herein in any form is strictly forbidden, unless permitted by WSO2 expressly.
 * You may not alter or remove any copyright or other notice from copies of this content.
 */

import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';

import PipelinesListPage from './PipelinesListPage';
import ProjectPipelinesPage from './ProjectPipelinesPage';
import type { Pipeline } from './types';

const pipeline: Pipeline = {
  id: 'release',
  name: 'release',
  promotionPaths: [{ sourceEnvironment: 'dev', targetEnvironments: ['prod'] }],
  isDefault: false,
  stages: [
    { id: 'dev', environmentId: 'dev' },
    { id: 'prod', environmentId: 'prod' },
  ],
};

const environments = [
  { id: 'dev', name: 'dev' },
  { id: 'prod', name: 'prod', critical: true },
];

const isDisabled = (element: HTMLElement) => (element as HTMLButtonElement).disabled;

describe('PipelinesListPage permissions', () => {
  const renderWith = (can: boolean) =>
    render(
      <PipelinesListPage
        pipelines={[pipeline]}
        environments={environments}
        canCreate={can}
        canUpdate={can}
        canDelete={can}
        onCreateClick={() => {}}
        onEditClick={() => {}}
        onDelete={() => {}}
      />
    );

  it('disables Create, Edit and Delete for a user holding none of the write scopes', () => {
    renderWith(false);

    expect(isDisabled(screen.getByRole('button', { name: 'Create Pipeline' }))).toBe(true);
    expect(isDisabled(screen.getByRole('button', { name: 'Edit release' }))).toBe(true);
    expect(isDisabled(screen.getByRole('button', { name: 'Delete release' }))).toBe(true);
  });

  it('offers Create, Edit and Delete to a user holding them', () => {
    renderWith(true);

    expect(isDisabled(screen.getByRole('button', { name: 'Create Pipeline' }))).toBe(false);
    expect(isDisabled(screen.getByRole('button', { name: 'Edit release' }))).toBe(false);
    expect(isDisabled(screen.getByRole('button', { name: 'Delete release' }))).toBe(false);
  });
});

describe('ProjectPipelinesPage permissions', () => {
  const renderWith = (canSave: boolean) =>
    render(
      <ProjectPipelinesPage
        pipelines={[]}
        organizationPipelines={[pipeline]}
        environments={environments}
        onAssociate={() => {}}
        canSave={canSave}
      />
    );

  it('says so and keeps Save disabled for a user who may not change the binding', () => {
    renderWith(false);

    expect(screen.getByText(/don't have permission to change this project's pipeline/i)).toBeTruthy();
    expect(isDisabled(screen.getByRole('button', { name: 'Save' }))).toBe(true);
  });

  it('shows no such notice to a user who may', () => {
    renderWith(true);

    expect(screen.queryByText(/don't have permission/i)).toBeNull();
  });
});
