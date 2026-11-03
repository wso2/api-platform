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
import { http as mswHttp, HttpResponse } from 'msw';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { ApiScopeProvider } from '@/api/core/ApiScopeProvider';
import { resetHttpClient } from '@/api/core/http';
import { apiUrl, failure, noContent, recorder, type Recorder } from '@/test/msw';
import { server } from '@/test/server';
import { renderWithProviders, screen, waitFor, within } from '@/test/utils';
import { makeAuthState } from '@/test/mockAuthState';
import { ApiThumbnailManager } from './ApiThumbnailManager';

// jsdom has no URL.createObjectURL / URL.revokeObjectURL — install stubs at
// module scope so React's passive cleanup effects can still revoke a URL
// scheduled by the component while the test harness unmounts it.
URL.createObjectURL = vi.fn(() => 'blob:test/mock');
URL.revokeObjectURL = vi.fn();

/**
 * Component tests for ApiThumbnailManager — the hover-overlay widget on the
 * API cards / detail page that uploads, replaces, or removes an API's thumbnail.
 *
 * What these tests exercise (vs. the hook/endpoint tests already covering the
 * underlying transport):
 *   - Camera / Trash controls render only when the user has the right permissions
 *     and the thumbnail exists for the Trash variant.
 *   - A non-JPEG/PNG upload is rejected client-side with a toast and never fires
 *     the PUT. Likewise a file over the 1 MiB cap.
 *   - A valid upload fires the PUT and surfaces a success toast.
 *   - Delete is a two-step flow (confirm dialog, then DELETE), not a single
 *     click, so an accidental Trash click doesn't blow away the thumbnail.
 *   - `disabled={true}` (gateway-managed APIs) hides the overlay entirely.
 *
 * Hover reveal itself is a CSS :hover pseudo-class — jsdom cannot resolve
 * computed styles from pseudo-classes, so we assert the overlay buttons' DOM
 * presence/absence rather than their rendered opacity. The buttons remain
 * reachable to assistive tech regardless of :hover, so clicking them without
 * hover first is representative, not a shortcut.
 */

const ORG = 'api-platform-demo';
const PROJECT = 'retail-apis';
const API_TYPE = 'rest-api';
const API_ID = 'orders-api';
const PATH = `/apis/${API_TYPE}/${API_ID}/thumbnail`;
const PNG_BYTES = new Uint8Array([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a]);

let requests: Recorder;

beforeEach(() => {
  requests = recorder();
  resetHttpClient();
  // Default: the API has no thumbnail yet. Individual tests override.
  server.use(failure('get', PATH, 404, 'NOT_FOUND'));
});

function renderManager(
  props: Partial<React.ComponentProps<typeof ApiThumbnailManager>> = {},
  options: { permissionMode?: 'permissive' | 'enforce'; scopes?: string[] } = {},
) {
  const authState = options.scopes
    ? makeAuthState({
        user: { name: 'Test User', email: 'test.user@example.com', scopes: options.scopes },
      })
    : undefined;
  return renderWithProviders(
    <ApiScopeProvider orgId={ORG} projectId={PROJECT}>
      <ApiThumbnailManager
        apiType={API_TYPE}
        apiId={API_ID}
        displayName="Orders API"
        size={64}
        {...props}
      />
    </ApiScopeProvider>,
    { permissionMode: options.permissionMode ?? 'permissive', authState },
  );
}

describe('ApiThumbnailManager — overlay visibility', () => {
  it('exposes the Camera button when the caller may upsert the thumbnail', async () => {
    renderManager();
    // Default permissive scope lets useCan('UpsertAPIThumbnail') return true.
    expect(await screen.findByRole('button', { name: /change thumbnail/i })).toBeInTheDocument();
  });

  it('hides the Trash button while no thumbnail exists (nothing to remove)', async () => {
    renderManager();
    await screen.findByRole('button', { name: /change thumbnail/i });
    expect(screen.queryByRole('button', { name: /remove thumbnail/i })).not.toBeInTheDocument();
  });

  it('exposes the Trash button once a thumbnail is cached', async () => {
    server.resetHandlers();
    server.use(
      mswHttp.get(apiUrl(PATH), () =>
        new HttpResponse(PNG_BYTES, { headers: { 'Content-Type': 'image/png' } }),
      ),
    );
    renderManager();
    expect(await screen.findByRole('button', { name: /remove thumbnail/i })).toBeInTheDocument();
  });

  it('hides every overlay control when disabled=true (gateway-managed APIs)', async () => {
    renderManager({ disabled: true });
    // Give any async hook work a tick before asserting absence.
    await Promise.resolve();
    expect(screen.queryByRole('button', { name: /change thumbnail/i })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /remove thumbnail/i })).not.toBeInTheDocument();
  });

  it('hides the overlay when the caller lacks the UpsertAPIThumbnail scope', async () => {
    // Enforce mode + a caller with no granted scopes → every useCan returns false.
    renderManager({}, { permissionMode: 'enforce', scopes: [] });
    await Promise.resolve();
    expect(screen.queryByRole('button', { name: /change thumbnail/i })).not.toBeInTheDocument();
  });
});

// Directly dispatches a `change` event on the hidden file input. user-event's
// `upload()` goes through a click + pointer-events check that fails on the
// `display: none` input the component uses for its OS file picker.
async function selectFile(file: File) {
  const fileInput = document.querySelector('input[type="file"]');
  if (!(fileInput instanceof HTMLInputElement)) {
    throw new Error('hidden file input not found');
  }
  fireEvent.change(fileInput, { target: { files: [file] } });
}

describe('ApiThumbnailManager — upload validation', () => {
  it('rejects a non-JPEG/PNG file client-side without firing the PUT', async () => {
    // Register a put handler that would record if it were ever hit — the client-
    // side mime check must stop the mutation before that happens.
    server.use(noContent('put', PATH, { record: requests }));
    renderManager();

    await screen.findByRole('button', { name: /change thumbnail/i });
    await selectFile(new File(['not an image'], 'notes.txt', { type: 'text/plain' }));

    expect(
      await screen.findByText(/Only JPEG and PNG images are supported/i),
    ).toBeInTheDocument();
    expect(requests.count()).toBe(0);
  });

  it('rejects a file over the 1 MiB cap client-side without firing the PUT', async () => {
    server.use(noContent('put', PATH, { record: requests }));
    renderManager();

    await screen.findByRole('button', { name: /change thumbnail/i });
    // File.size is computed from the Blob parts — a Uint8Array of the right
    // length is all the pre-flight size check inspects; the bytes themselves
    // don't need to be a real image.
    await selectFile(new File([new Uint8Array(2 * 1024 * 1024)], 'huge.png', { type: 'image/png' }));

    expect(await screen.findByText(/image must be 1 MB or smaller/i)).toBeInTheDocument();
    expect(requests.count()).toBe(0);
  });

  it('fires PUT and surfaces a success toast on a valid upload', async () => {
    server.use(noContent('put', PATH, { record: requests }));
    renderManager();

    await screen.findByRole('button', { name: /change thumbnail/i });
    await selectFile(new File([PNG_BYTES], 'icon.png', { type: 'image/png' }));

    await waitFor(() => expect(requests.count()).toBe(1));
    expect(requests.last()?.method).toBe('PUT');
    expect(await screen.findByText(/Thumbnail updated/i)).toBeInTheDocument();
  });
});

describe('ApiThumbnailManager — delete flow', () => {
  function serveExistingThumbnail() {
    server.resetHandlers();
    server.use(
      mswHttp.get(apiUrl(PATH), () =>
        new HttpResponse(PNG_BYTES, { headers: { 'Content-Type': 'image/png' } }),
      ),
    );
  }

  // The overlay has `pointer-events: none` until the parent is hovered. jsdom
  // cannot resolve :hover, so user-event refuses to click. Fire the event
  // directly — React's onClick still runs, which is what the test is about.
  it('opens the confirm dialog rather than deleting on a single Trash click', async () => {
    serveExistingThumbnail();
    const spy = vi.fn();
    server.use(
      mswHttp.delete(apiUrl(PATH), () => {
        spy();
        return new HttpResponse(null, { status: 204 });
      }),
    );
    renderManager();

    fireEvent.click(await screen.findByRole('button', { name: /remove thumbnail/i }));

    expect(await screen.findByRole('dialog')).toBeInTheDocument();
    expect(spy).not.toHaveBeenCalled();
  });

  it('fires DELETE and shows a success toast after dialog confirmation', async () => {
    serveExistingThumbnail();
    server.use(noContent('delete', PATH, { record: requests }));
    renderManager();

    fireEvent.click(await screen.findByRole('button', { name: /remove thumbnail/i }));
    const dialog = await screen.findByRole('dialog');
    // Scope to the dialog: the overlay's Trash button is also labelled
    // "Remove thumbnail", so an unscoped match would be ambiguous. The
    // dialog's own confirm button is labelled just "Remove".
    fireEvent.click(within(dialog).getByRole('button', { name: /^remove$/i }));

    await waitFor(() => expect(requests.count()).toBe(1));
    expect(requests.last()?.method).toBe('DELETE');
    expect(await screen.findByText(/Thumbnail removed/i)).toBeInTheDocument();
  });
});
