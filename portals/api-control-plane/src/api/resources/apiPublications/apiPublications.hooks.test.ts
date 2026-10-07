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
import { beforeEach, describe, expect, it } from 'vitest';

import {
  accepts,
  aPublication,
  aPublicationDraftDetails,
  failure,
  recorder,
  resource,
  type Recorder,
} from '../../../test/msw';
import { renderApiHook } from '../../../test/renderApiHook';
import { server } from '../../../test/server';
import { resetHttpClient } from '../../core/http';
import {
  useApiPublicationDraft,
  useApiPublications,
  usePublishRestApiToApiPortal,
  useSaveApiPublicationDraft,
  useUnpublishRestApiFromApiPortal,
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
  publish: usePublishRestApiToApiPortal(),
  rollup: useApiPublications(TYPE, API, { limit: 100 }),
  save: useSaveApiPublicationDraft(),
  unpublish: useUnpublishRestApiFromApiPortal(),
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

    await act(() => result.current.publish.mutateAsync({ apiPortalId: PORTAL, apiId: API }));

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

    expect(result.current.draft.data?.displayName).toBe('Saved Name');
    expect(draftReads.count()).toBe(1);
  });
});

describe('a status change that fails', () => {
  it('reads what is on screen again, to find out where things stand', async () => {
    server.use(failure('post', `/api-portals/${PORTAL}/apis/${TYPE}/${API}/unpublish`, 409, 'PUBLICATION_STATE_CONFLICT'));
    const { result } = await renderLoadedScreen();

    await act(() => result.current.unpublish.mutateAsync({ apiPortalId: PORTAL, apiId: API }).catch(() => undefined));

    await waitFor(() => expect(draftReads.count()).toBe(2));
  });

  it('also reads again when a publish fails', async () => {
    server.use(failure('post', `/api-portals/${PORTAL}/apis/${TYPE}/${API}/publish`, 409, 'PUBLICATION_STATE_CONFLICT'));
    const { result } = await renderLoadedScreen();

    await act(() => result.current.publish.mutateAsync({ apiPortalId: PORTAL, apiId: API }).catch(() => undefined));

    await waitFor(() => expect(draftReads.count()).toBe(2));
  });
});
