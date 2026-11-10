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

import { fireEvent } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';

// Monaco is heavy and jsdom-hostile — swap it for a plain <textarea> that
// mirrors the editor's two inputs (value, onChange). Everything the component
// reads from the editor goes through the `value` prop, and every edit the user
// makes flows out through `onChange`, so this stub is behaviourally equivalent
// for the purposes of a component test.
vi.mock('@monaco-editor/react', () => ({
  default: (props: { value?: string; onChange?: (next: string | undefined) => void }) => (
    <textarea
      aria-label="Definition editor"
      onChange={(e) => props.onChange?.(e.target.value)}
      value={props.value ?? ''}
    />
  ),
}));

import { ApiScopeProvider } from '@/api/core/ApiScopeProvider';
import { resetHttpClient } from '@/api/core/http';
import { accepts, failure, recorder, resource, type Recorder } from '@/test/msw';
import { server } from '@/test/server';
import { makeConsoleScope } from '@/test/mockScope';
import { renderWithProviders, screen, waitFor, within } from '@/test/utils';
import { DefinitionPanel } from './DefinitionPanel';

/**
 * Component tests for DefinitionPanel — the API Definition tab on an existing
 * REST API, where a user views the stored OpenAPI spec, edits it, and saves
 * the result through `GET /rest-apis/{id}/openapi`, `POST /rest-apis/validate-openapi`,
 * and `PUT /rest-apis/{id}/openapi`.
 */

const ORG = 'api-platform-demo';
const PROJECT = 'retail-apis';
const API = 'orders-api';
const OPENAPI_PATH = `/rest-apis/${API}/openapi`;
const DETAIL_PATH = `/rest-apis/${API}`;

const SAMPLE_SPEC = `openapi: 3.0.0
info:
  title: Orders API
  version: 1.0.0
paths:
  /orders:
    get:
      summary: List orders
      responses:
        '200':
          description: OK
`;

let requests: Recorder;

beforeEach(() => {
  requests = recorder();
  resetHttpClient();
  // Default: the detail call succeeds with a minimal object. Individual tests
  // override the openapi path to shape the state they want.
  server.use(resource(DETAIL_PATH, { id: API, displayName: 'Orders API' }));
});

function renderPanel() {
  return renderWithProviders(
    <ApiScopeProvider orgId={ORG} projectId={PROJECT}>
      <DefinitionPanel />
    </ApiScopeProvider>,
    { scope: makeConsoleScope({ params: { apiHandler: API, orgHandle: ORG, projectHandler: PROJECT } }) },
  );
}

describe('DefinitionPanel — read states', () => {
  it('shows a loading placeholder while the spec request is in flight', () => {
    // No handler registered yet for the openapi path — the query stays pending.
    // We render + synchronously assert; the loading label is up on first paint.
    server.use(resource(OPENAPI_PATH, { content: SAMPLE_SPEC }));
    renderPanel();
    expect(screen.getByText(/Loading API definition/i)).toBeInTheDocument();
  });

  it('renders the empty state with "Import definition" when the spec is absent (404)', async () => {
    server.use(failure('get', OPENAPI_PATH, 404, 'NOT_FOUND'));
    renderPanel();

    expect(await screen.findByText(/No API definition/i)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /Import definition/i })).toBeInTheDocument();
  });

  it('renders the spec header (title, version chip, resource count) when a spec exists', async () => {
    server.use(resource(OPENAPI_PATH, { content: SAMPLE_SPEC }));
    renderPanel();

    expect(await screen.findByRole('heading', { name: /Definition/i })).toBeInTheDocument();
    // Version chip is derived from the YAML — "OpenAPI 3.0.0".
    expect(screen.getByText(/OpenAPI 3\.0\.0/)).toBeInTheDocument();
    // Resource count is derived from spec paths — this spec has exactly one op.
    expect(screen.getByText(/1 resource\b/)).toBeInTheDocument();
    // Edit, Download, Import, YAML/JSON toggle are all up on the loaded view.
    expect(screen.getByRole('button', { name: /Edit/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /Download Definition/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /^YAML$/ })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /^JSON$/ })).toBeInTheDocument();
  });
});

describe('DefinitionPanel — edit + save', () => {
  it('reveals the Save/Reset bar after clicking Edit', async () => {
    server.use(resource(OPENAPI_PATH, { content: SAMPLE_SPEC }));
    renderPanel();

    // Before: Edit is shown, Save/Reset are not.
    const editBtn = await screen.findByRole('button', { name: /Edit/i });
    expect(screen.queryByRole('button', { name: /^Save$/i })).not.toBeInTheDocument();

    fireEvent.click(editBtn);

    // After: Save + Reset appear.
    expect(await screen.findByRole('button', { name: /^Save$/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /^Reset$/i })).toBeInTheDocument();
  });

  it('clicking Reset discards unsaved edits and leaves edit mode', async () => {
    server.use(resource(OPENAPI_PATH, { content: SAMPLE_SPEC }));
    renderPanel();

    fireEvent.click(await screen.findByRole('button', { name: /Edit/i }));
    const editor = await screen.findByRole('textbox', { name: /Definition editor/i });
    fireEvent.change(editor, { target: { value: 'junk edit that will be discarded' } });

    fireEvent.click(screen.getByRole('button', { name: /^Reset$/i }));

    // Reset leaves edit mode → Save/Reset disappear and Edit comes back.
    await waitFor(() =>
      expect(screen.queryByRole('button', { name: /^Save$/i })).not.toBeInTheDocument(),
    );
    expect(screen.getByRole('button', { name: /Edit/i })).toBeInTheDocument();
  });

  it('surfaces backend validation errors in the Save bar and does not PUT', async () => {
    server.use(
      resource(OPENAPI_PATH, { content: SAMPLE_SPEC }),
      accepts('post', '/rest-apis/validate-openapi', {
        isValid: false,
        errors: [{ message: 'missing responses on GET /orders' }],
      }),
      // Register the PUT with the recorder so an unwanted call is observable.
      accepts('put', OPENAPI_PATH, { content: SAMPLE_SPEC }, { record: requests }),
    );
    renderPanel();

    fireEvent.click(await screen.findByRole('button', { name: /Edit/i }));
    const editor = await screen.findByRole('textbox', { name: /Definition editor/i });
    fireEvent.change(editor, { target: { value: 'openapi: 3.0.0\nmissing: stuff' } });

    fireEvent.click(screen.getByRole('button', { name: /^Save$/i }));

    // Alert surfaces the backend's error line.
    expect(await screen.findByText(/missing responses on GET \/orders/i)).toBeInTheDocument();
    // PUT must not have fired.
    expect(requests.count()).toBe(0);
  });

  it('fires PUT and exits edit mode when the user confirms a valid-spec save', async () => {
    // The Save button stays disabled until the parsed spec is structurally
    // different from the saved one — a YAML comment or whitespace change
    // parses back to the same object and leaves isDirty=false. Edit a field
    // the parser actually sees (version).
    const EDITED_SPEC = SAMPLE_SPEC.replace('version: 1.0.0', 'version: 2.0.0');
    server.use(
      resource(OPENAPI_PATH, { content: SAMPLE_SPEC }),
      accepts('post', '/rest-apis/validate-openapi', {
        isValid: true,
        errors: [],
        content: EDITED_SPEC,
      }),
      accepts('put', OPENAPI_PATH, { content: EDITED_SPEC }, { record: requests }),
    );
    renderPanel();

    fireEvent.click(await screen.findByRole('button', { name: /Edit/i }));
    const editor = await screen.findByRole('textbox', { name: /Definition editor/i });
    fireEvent.change(editor, { target: { value: EDITED_SPEC } });

    // Save becomes enabled once the parsed-structure diff is non-empty.
    const saveBtn = await screen.findByRole('button', { name: /^Save$/i });
    await waitFor(() => expect(saveBtn).not.toBeDisabled());
    fireEvent.click(saveBtn);

    // Save flow pops a confirmation dialog ("Update API") first. Click through
    // on the Save button inside it to trigger the real PUT.
    const confirmDialog = await screen.findByRole('dialog');
    fireEvent.click(within(confirmDialog).getByRole('button', { name: /^Save$/i }));

    await waitFor(() => expect(requests.count()).toBe(1));
    expect(requests.last()?.method).toBe('PUT');
    // The successful save returns the panel to its read mode (no Save bar).
    await waitFor(() =>
      expect(screen.queryByRole('button', { name: /^Save$/i })).not.toBeInTheDocument(),
    );
  });
});

describe('DefinitionPanel — view toggles and download', () => {
  it('switches between the source editor and the resources list', async () => {
    server.use(resource(OPENAPI_PATH, { content: SAMPLE_SPEC }));
    renderPanel();

    // Default view is source — the mocked editor textarea is on-screen.
    expect(await screen.findByRole('textbox', { name: /Definition editor/i })).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: /View Resources/i }));

    // Editor disappears when the user flips to the operations view.
    await waitFor(() =>
      expect(screen.queryByRole('textbox', { name: /Definition editor/i })).not.toBeInTheDocument(),
    );
    // "View Definition" is the inverse label that only appears in resources view.
    expect(screen.getByRole('button', { name: /View Definition/i })).toBeInTheDocument();
  });

  it('invokes a Blob download when the user clicks Download Definition', async () => {
    server.use(resource(OPENAPI_PATH, { content: SAMPLE_SPEC }));

    const createObjectURL = vi.fn(() => 'blob:test/mock');
    const revokeObjectURL = vi.fn();
    URL.createObjectURL = createObjectURL;
    URL.revokeObjectURL = revokeObjectURL;

    renderPanel();

    fireEvent.click(await screen.findByRole('button', { name: /Download Definition/i }));

    // The handler composes a Blob and anchors it with URL.createObjectURL, then
    // immediately revokes it. Both calls are observable; the actual file-save
    // dialog is a browser-only side effect we cannot assert on in jsdom.
    expect(createObjectURL).toHaveBeenCalledTimes(1);
    expect(revokeObjectURL).toHaveBeenCalledWith('blob:test/mock');
  });
});
