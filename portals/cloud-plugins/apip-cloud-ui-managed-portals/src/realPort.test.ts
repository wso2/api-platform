/*
 * Copyright (c) 2026, WSO2 LLC (http://www.wso2.com). All Rights Reserved.
 *
 * This software is the property of WSO2 LLC and its suppliers, if any.
 * Dissemination of any information or reproduction of any material contained
 * herein in any form is strictly forbidden, unless permitted by WSO2 expressly.
 * You may not alter or remove any copyright or other notice from copies of this content.
 */

import { afterEach, describe, expect, it, vi } from 'vitest';

import { CHANGE_FAILED_FALLBACK_MESSAGE, createRealPortalPort } from './realPort';

function serve(body: unknown) {
  vi.stubGlobal(
    'fetch',
    vi.fn(async () => new Response(JSON.stringify(body), { status: 200 }))
  );
}

const base = { id: 'devs', handle: 'devs', name: 'Developers', status: 'pending' };

describe('realPort background change fields', () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('maps a known change and change error', async () => {
    serve({
      ...base,
      change: { op: 'selfSignup', selfSignup: false },
      lastChangeError: { op: 'loginEnvironment', loginEnvironment: 'staging', message: 'Stopped.', at: '2026-11-09T00:00:00Z' },
    });

    const portal = await createRealPortalPort('/api', 'acme').get('devs');

    expect(portal.change).toEqual({ op: 'selfSignup', loginEnvironment: undefined, selfSignup: false });
    expect(portal.lastChangeError).toEqual({
      op: 'loginEnvironment',
      loginEnvironment: 'staging',
      selfSignup: undefined,
      message: 'Stopped.',
      at: '2026-11-09T00:00:00Z',
    });
  });

  it('reads an unknown change kind as no named change', async () => {
    serve({ ...base, change: { op: 'renameHost' } });

    const portal = await createRealPortalPort('/api', 'acme').get('devs');

    expect(portal.change).toBeUndefined();
  });

  it('keeps a change error with an unknown kind or an empty message', async () => {
    serve({ ...base, status: 'failed', lastChangeError: { op: 'renameHost', message: '  ' } });

    const portal = await createRealPortalPort('/api', 'acme').get('devs');

    expect(portal.lastChangeError).toEqual({
      op: undefined,
      loginEnvironment: undefined,
      selfSignup: undefined,
      message: CHANGE_FAILED_FALLBACK_MESSAGE,
      at: undefined,
    });
  });

  it('has no change error when the backend sends none', async () => {
    serve({ ...base, status: 'active' });

    const portal = await createRealPortalPort('/api', 'acme').get('devs');

    expect(portal.lastChangeError).toBeUndefined();
  });
});
