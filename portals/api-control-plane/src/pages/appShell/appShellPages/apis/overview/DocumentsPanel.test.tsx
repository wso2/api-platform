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

import { describe, expect, it, beforeEach } from 'vitest';

import { ApiScopeProvider } from '@/api/core/ApiScopeProvider';
import { resetHttpClient } from '@/api/core/http';
import type { ApiDocumentMetadata } from '@/api/resources/apiDocuments';
import { routes } from '@/routes/paths';
import { makeConsoleScope } from '@/test/mockScope';
import { collection } from '@/test/msw';
import { server } from '@/test/server';
import { renderWithProviders, screen } from '@/test/utils';
import { DocumentsPanel } from './DocumentsPanel';

const ORG = 'api-platform-demo';
const PROJECT = 'retail-apis';
const API = 'orders-api';
const COLLECTION = `/apis/rest-api/${API}/docs`;

const aDocument = (id: string): ApiDocumentMetadata => ({
  displayName: `Doc ${id}`,
  id,
  type: 'HowTo',
  updatedAt: '2026-09-28T10:00:00Z',
  updatedBy: 'admin',
});

function renderPanel() {
  return renderWithProviders(
    <ApiScopeProvider orgId={ORG} projectId={PROJECT}>
      <DocumentsPanel />
    </ApiScopeProvider>,
    {
      scope: makeConsoleScope({
        params: { apiHandler: API, orgHandle: ORG, projectHandler: PROJECT },
      }),
    },
  );
}

beforeEach(() => resetHttpClient());

describe('overview DocumentsPanel', () => {
  it('offers to create the first document when there are none', async () => {
    server.use(collection(COLLECTION, []));
    renderPanel();

    expect(await screen.findByText('No Documents available for this API')).toBeInTheDocument();
    expect(screen.queryByRole('button')).not.toBeInTheDocument();
    expect(screen.queryByRole('link', { name: /View More/ })).not.toBeInTheDocument();
    expect(screen.getByRole('link', { name: /Create Document/ })).toHaveAttribute(
      'href',
      `${routes.apiDevelopDocuments(ORG, PROJECT, API)}?mode=create`,
    );
  });

  it('links each row to that document on the Documents page', async () => {
    server.use(collection(COLLECTION, [aDocument('one')]));
    renderPanel();

    const row = await screen.findByRole('link', { name: /Doc one/ });
    expect(row).toHaveAttribute('href', `${routes.apiDevelopDocuments(ORG, PROJECT, API)}?doc=one`);
    // Nothing more to see than what is listed, so no "View More" — and documents exist, so no create link.
    expect(screen.queryByRole('link', { name: /View More/ })).not.toBeInTheDocument();
    expect(screen.queryByRole('link', { name: /Create Document/ })).not.toBeInTheDocument();
  });

  it('shows five documents and sends the rest to the Documents page', async () => {
    server.use(
      collection(
        COLLECTION,
        Array.from({ length: 7 }, (_, index) => aDocument(`${index + 1}`)),
      ),
    );
    renderPanel();

    expect(await screen.findByRole('link', { name: /Doc 5/ })).toBeInTheDocument();
    expect(screen.queryByRole('link', { name: /Doc 6/ })).not.toBeInTheDocument();
    expect(screen.queryByRole('navigation')).not.toBeInTheDocument();
    expect(screen.getByRole('link', { name: /View More/ })).toHaveAttribute(
      'href',
      routes.apiDevelopDocuments(ORG, PROJECT, API),
    );
  });
});
