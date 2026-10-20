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

import { act, waitFor } from '@testing-library/react';
import { http as mswHttp, HttpResponse } from 'msw';
import { beforeEach, describe, expect, it } from 'vitest';

import {
  accepts,
  apiUrl,
  aPublication,
  aPublicationDraftDetails,
  failure,
  noContent,
  recorder,
  resource,
  type Recorder,
} from '../../../test/msw';
import { renderApiHook, settle } from '../../../test/renderApiHook';
import { server } from '../../../test/server';
import { resetHttpClient } from '../../core/http';
import {
  useApiPublicationDraft,
  useApiPublicationDraftThumbnail,
  useApiPublicationThumbnail,
  useApiPublications,
  usePublishApiToApiPortal,
  useSaveApiPublicationDraft,
  useSaveApiPublicationDraftThumbnail,
  useUnpublishApiFromApiPortal,
} from './apiPublications.hooks';

/**
 * What the writes do to what a screen has already read. A save, publish or
 * unpublish changes several things at once, but a screen that is leaving, or
 * was just handed the answer, should not read them all again — and a failure
 * must still find out where things really stand.
 */

const API = 'loan-mgmt';
const PORTAL = 'acme-portal';
const TYPE = 'rest-api';
const DRAFT = `/api-portals/${PORTAL}/apis/${TYPE}/${API}/draft`;
const LISTING = '/api-publications';
const emptyRollup = { list: [], pagination: { total: 0, limit: 100, offset: 0 } };

let draftReads: Recorder;
let rollupReads: Recorder;

beforeEach(() => {
  draftReads = recorder();
  rollupReads = recorder();
  resetHttpClient();
  server.use(
    resource(DRAFT, aPublicationDraftDetails(), { record: draftReads }),
    resource(LISTING, emptyRollup, { record: rollupReads }),
  );
});

/** A screen that has read the rollup and the draft, then offers the writes. */
const useScreen = () => ({
  draft: useApiPublicationDraft(PORTAL, TYPE, API),
  publish: usePublishApiToApiPortal(),
  rollup: useApiPublications(TYPE, API, { limit: 100 }),
  save: useSaveApiPublicationDraft(),
  unpublish: useUnpublishApiFromApiPortal(),
});

async function renderLoadedScreen() {
  const view = renderApiHook(useScreen);
  await waitFor(() => {
    expect(view.result.current.draft.isSuccess).toBe(true);
    expect(view.result.current.rollup.isSuccess).toBe(true);
  });
  return view;
}

describe('publishing', () => {
  it('brings the rollup up to date before it settles, and does not read the draft it consumed', async () => {
    server.use(accepts('post', `/api-portals/${PORTAL}/apis/${TYPE}/${API}/publish`, aPublication()));
    const { result } = await renderLoadedScreen();

    await act(() => result.current.publish.mutateAsync({ apiPortalId: PORTAL, apiType: 'rest-api', apiId: API }));

    expect(rollupReads.count()).toBe(2);
    expect(draftReads.count()).toBe(1);
  });
});

describe('saving a draft', () => {
  it('keeps the draft the server returned, without reading it back', async () => {
    server.use(accepts('put', DRAFT, aPublicationDraftDetails({ displayName: 'Saved Name' })));
    const { result } = await renderLoadedScreen();

    await act(() =>
      result.current.save.mutateAsync({
        apiPortalId: PORTAL,
        apiType: TYPE,
        apiId: API,
        body: { displayName: 'Saved Name', version: '1.0.0' },
      }),
    );

    await waitFor(() => expect(result.current.draft.data?.displayName).toBe('Saved Name'));
    await settle();
    expect(draftReads.count()).toBe(1);
  });
});

describe('a status change that fails', () => {
  it('reads what is on screen again, to find out where things stand', async () => {
    server.use(failure('post', `/api-portals/${PORTAL}/apis/${TYPE}/${API}/unpublish`, 409, 'PUBLICATION_STATE_CONFLICT'));
    const { result } = await renderLoadedScreen();

    await act(() => result.current.unpublish.mutateAsync({ apiPortalId: PORTAL, apiType: 'rest-api', apiId: API }).catch(() => undefined));

    await waitFor(() => expect(draftReads.count()).toBe(2));
  });

  it('also reads again when a publish fails', async () => {
    server.use(failure('post', `/api-portals/${PORTAL}/apis/${TYPE}/${API}/publish`, 409, 'PUBLICATION_STATE_CONFLICT'));
    const { result } = await renderLoadedScreen();

    await act(() => result.current.publish.mutateAsync({ apiPortalId: PORTAL, apiType: 'rest-api', apiId: API }).catch(() => undefined));

    await waitFor(() => expect(draftReads.count()).toBe(2));
  });
});

describe('reading a thumbnail', () => {
  const serveThumbnails = () =>
    server.use(
      ...['draft', 'publication'].map((tier) =>
        mswHttp.get(apiUrl(`/api-portals/${PORTAL}/apis/${TYPE}/${API}/${tier}/thumbnail`), async ({ request }) => {
          await thumbnailReads.capture(request);
          return new HttpResponse(new Uint8Array([0x89, 0x50]), { headers: { 'Content-Type': 'image/png' } });
        }),
      ),
    );

  let thumbnailReads: Recorder;
  beforeEach(() => {
    thumbnailReads = recorder();
  });

  it('waits until the caller knows there is one to read', async () => {
    serveThumbnails();

    renderApiHook(() => ({
      draft: useApiPublicationDraftThumbnail(PORTAL, TYPE, undefined),
      publication: useApiPublicationThumbnail(PORTAL, TYPE, undefined),
    }));
    await settle();

    expect(thumbnailReads.count()).toBe(0);
  });

  it('reads the draft and the publication thumbnail as an image', async () => {
    serveThumbnails();

    const { result } = renderApiHook(() => ({
      draft: useApiPublicationDraftThumbnail(PORTAL, TYPE, API),
      publication: useApiPublicationThumbnail(PORTAL, TYPE, API),
    }));

    await waitFor(() => {
      expect(result.current.draft.data?.contentType).toBe('image/png');
      expect(result.current.publication.data?.contentType).toBe('image/png');
    });
    expect(thumbnailReads.count()).toBe(2);
  });
});

describe('saving a thumbnail', () => {
  it('marks what the screen has read stale once saved, without reading the draft again', async () => {
    server.use(noContent('put', `${DRAFT}/thumbnail`));
    const { result } = renderApiHook(() => ({
      draft: useApiPublicationDraft(PORTAL, TYPE, API),
      save: useSaveApiPublicationDraftThumbnail(),
    }));
    await waitFor(() => expect(result.current.draft.isSuccess).toBe(true));

    await act(() =>
      result.current.save.mutateAsync({
        apiPortalId: PORTAL,
        apiType: TYPE,
        apiId: API,
        file: new Blob(['img'], { type: 'image/png' }),
      }),
    );
    await settle();

    expect(draftReads.count()).toBe(1);
    expect(result.current.draft.isStale).toBe(true);
  });
});
