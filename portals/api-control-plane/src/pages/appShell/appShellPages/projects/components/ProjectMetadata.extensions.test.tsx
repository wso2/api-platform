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

import { describe, expect, it } from 'vitest';

import {
  ExtensionsProvider,
  PROJECT_HEADER_ACTIONS_SLOT,
  type ApiControlPlaneHeaderAction,
} from '@/extensions';
import { NO_RESOURCE_LIMITS, PortProvider, type CloudHostPort } from '@/hostPort';
import { aProject } from '@/test/msw';
import { renderWithProviders, screen } from '@/test/utils';

import { ProjectMetadata } from './ProjectMetadata';

const port: CloudHostPort = {
  orgHandle: 'acme',
  projectHandle: 'retail',
  navigate: () => {},
  notify: () => {},
  apiFetch: async () => undefined as never,
  billing: { organization: async () => null },
  resourceLimits: NO_RESOURCE_LIMITS,
  resources: { invalidate: () => {} },
  permissions: { canAny: () => true },
};

const headingEntry: ApiControlPlaneHeaderAction = {
  id: 'manage-project-button',
  slot: PROJECT_HEADER_ACTIONS_SLOT,
  order: 10,
  render: (hostPort) => <a href={`/manage/${hostPort.projectHandle}`}>Manage project</a>,
};

const renderMetadata = (extensions: ApiControlPlaneHeaderAction[]) =>
  renderWithProviders(
    <ExtensionsProvider extensions={extensions}>
      <PortProvider value={port}>
        <ProjectMetadata project={aProject()} />
      </PortProvider>
    </ExtensionsProvider>
  );

describe('ProjectMetadata cloud extensions', () => {
  it('renders an entry registered against the project heading slot, with the project in the Port', () => {
    renderMetadata([headingEntry]);

    expect(screen.getByText('Retail APIs')).toBeInTheDocument();
    expect(
      screen.getByRole('link', { name: 'Manage project' }).getAttribute('href')
    ).toBe('/manage/retail');
  });

  it('ignores an entry registered against another slot', () => {
    renderMetadata([{ ...headingEntry, slot: 'header.actions' }]);

    expect(screen.getByText('Retail APIs')).toBeInTheDocument();
    expect(screen.queryByRole('link', { name: 'Manage project' })).not.toBeInTheDocument();
  });

  it('renders without a Port when nothing is registered, as a standalone console does', () => {
    renderWithProviders(<ProjectMetadata project={aProject()} />);

    expect(screen.getByText('Retail APIs')).toBeInTheDocument();
    expect(screen.queryByRole('link', { name: 'Manage project' })).not.toBeInTheDocument();
  });
});
