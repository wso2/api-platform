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

import type { UseQueryResult } from '@tanstack/react-query';
import { Route, Routes, useLocation } from 'react-router-dom';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import type { ApiError } from '../api/core/errors';
import { organizations } from '../api/mocks/data';
import type {
  RestApi,
  RestApiListResponse,
} from '../api/resources/restApis';
import { routes } from '../routes/paths';
import { makeConsoleScope } from '../test/mockScope';
import { renderWithProviders, screen } from '../test/utils';

vi.mock('../api/resources/restApis', async (importActual) => ({
  ...(await importActual<typeof import('../api/resources/restApis')>()),
  useRestApis: vi.fn(),
}));

import { useRestApis } from '../api/resources/restApis';
import { ScopeGate } from './ScopeGate';

const API_LIST = [
  {
    context: '/orders',
    displayName: 'Orders API',
    id: 'orders-api',
    projectId: 'retail-apis',
    upstream: { main: { url: 'https://backend.example.com' } },
    version: '1.0.0',
  },
] as RestApi[];

const listQuery = (list: RestApi[]) =>
  ({
    data: { count: list.length, list },
    isPending: false,
  }) as UseQueryResult<RestApiListResponse, ApiError>;

// `makeConsoleScope` seeds itself from these fixtures, and `ScopeGate` reads the
// org handle from scope rather than from the URL — so the routes under test have
// to be built from the same handles.
const ORG = organizations[0].id;

/*
 * The project list is supplied by the test rather than taken from
 * `makeConsoleScope`'s defaults, because the picker reads `displayName`/`id` (the
 * generated shape) while `api/mocks/data` still carries `name`/`handler`. Pinning
 * it here keeps these tests about the gate rather than about which fixture shape
 * happens to be current.
 */
const PROJECT_OPTION = { displayName: 'Retail APIs', id: 'retail-apis' };
const PROJECT = PROJECT_OPTION.id;
const projectScope = () =>
  makeConsoleScope({
    isProjectScope: false,
    project: undefined,
    projects: [PROJECT_OPTION] as ReturnType<
      typeof makeConsoleScope
    >['projects'],
  });

/** The select's own control — `getByLabelText` also matches the visible label. */
const selectFor = (name: string) => screen.getByRole('combobox', { name });

/**
 * An option inside an open select. Matched by role rather than text: once a
 * value is chosen the select renders it again in its own display, so plain text
 * matches two nodes.
 */
const optionFor = (name: RegExp) => screen.getByRole('option', { name });

/** Renders the current pathname so a test can assert where submitting landed. */
function Located() {
  return <span>{`at ${useLocation().pathname}`}</span>;
}

describe('ScopeGate', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(useRestApis).mockReturnValue(listQuery(API_LIST));
  });

  const renderGate = (
    route: string,
    gate: React.ReactNode,
    scope: ReturnType<typeof makeConsoleScope>
  ) =>
    renderWithProviders(
      <Routes>
        <Route path="*" element={gate} />
      </Routes>,
      { route, scope }
    );

  it('renders the page once the required scope is on the route', () => {
    renderGate(
      routes.apis(ORG, PROJECT),
      <ScopeGate requires="project" to={routes.apis}>
        <span>page body</span>
      </ScopeGate>,
      makeConsoleScope()
    );

    expect(screen.getByText('page body')).toBeInTheDocument();
  });

  it('prompts for a project instead of the page when none is selected', () => {
    renderGate(
      routes.apis(ORG, null),
      <ScopeGate
        prompt="APIs are created and managed at the project level."
        requires="project"
        to={routes.apis}
      >
        <span>page body</span>
      </ScopeGate>,
      projectScope()
    );

    expect(
      screen.getByText('APIs are created and managed at the project level.')
    ).toBeInTheDocument();
    expect(screen.queryByText('page body')).not.toBeInTheDocument();
  });

  it('navigates to the fully scoped page after picking a project', async () => {
    const { user } = renderGate(
      routes.apis(ORG, null),
      <>
        <ScopeGate requires="project" to={routes.apis}>
          <span>page body</span>
        </ScopeGate>
        <Located />
      </>,
      projectScope()
    );

    await user.click(selectFor('Project'));
    await user.click(optionFor(/Retail APIs/));
    await user.click(screen.getByRole('button', { name: 'Go to Project Level' }));

    expect(
      screen.getByText(`at ${routes.apis(ORG, PROJECT)}`)
    ).toBeInTheDocument();
  });

  it('asks for both handles on an api-level page outside any project', async () => {
    const { user } = renderGate(
      routes.apiDeploy(ORG, null, null),
      <>
        <ScopeGate requires="api" to={routes.apiDeploy}>
          <span>page body</span>
        </ScopeGate>
        <Located />
      </>,
      projectScope()
    );

    // Disabled until a project narrows the API list.
    const continueButton = screen.getByRole('button', {
      name: 'Go to API Level',
    });
    expect(continueButton).toBeDisabled();

    await user.click(selectFor('Project'));
    await user.click(optionFor(/Retail APIs/));
    await user.click(selectFor('API'));
    await user.click(optionFor(/Orders API/));
    await user.click(continueButton);

    expect(
      screen.getByText(
        `at ${routes.apiDeploy(ORG, PROJECT, 'orders-api')}`
      )
    ).toBeInTheDocument();
  });

  it('asks only for the API when the route already has a project', () => {
    renderGate(
      routes.apiDeploy(ORG, PROJECT, null),
      <ScopeGate requires="api" to={routes.apiDeploy}>
        <span>page body</span>
      </ScopeGate>,
      makeConsoleScope()
    );

    expect(selectFor('API')).toBeInTheDocument();
    expect(
      screen.queryByRole('combobox', { name: 'Project' })
    ).not.toBeInTheDocument();
  });
});
