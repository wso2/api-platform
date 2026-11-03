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

import { screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { routes } from '@/routes/paths';
import { renderWithProviders } from '@/test/utils';
import { ProjectQuickActions } from './ProjectQuickActions';

const navigate = vi.hoisted(() => vi.fn());
const preload = vi.hoisted(() => vi.fn());

vi.mock('react-router-dom', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react-router-dom')>()),
  useNavigate: () => navigate,
}));
vi.mock('@/routes/preloadApiCreate', () => ({ preloadApiCreationWizard: preload }));

describe('ProjectQuickActions', () => {
  beforeEach(() => {
    navigate.mockClear();
    preload.mockClear();
  });

  it('warms the API creation wizard chunk on mount', () => {
    renderWithProviders(<ProjectQuickActions orgHandle="acme" projectId="p1" />);

    expect(preload).toHaveBeenCalledTimes(1);
  });

  it('opens the new-API flow for the project', async () => {
    renderWithProviders(<ProjectQuickActions orgHandle="acme" projectId="p1" />);

    await userEvent.click(screen.getByRole('button', { name: 'Import an OpenAPI definition' }));

    expect(navigate).toHaveBeenCalledWith(routes.newApi('acme', 'p1'));
  });
});
