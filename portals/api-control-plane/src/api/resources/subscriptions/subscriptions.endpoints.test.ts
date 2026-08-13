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

import { beforeEach, describe, expect, it } from 'vitest';

import {
  aSubscription,
  accepts,
  collection,
  failure,
  noContent,
  recorder,
  resource,
  type Recorder,
} from '../../../test/msw';
import { server } from '../../../test/server';
import { ApiError } from '../../core/errors';
import { resetHttpClient } from '../../core/http';
import {
  createSubscription,
  deleteSubscription,
  getSubscription,
  listSubscriptions,
  updateSubscription,
} from './subscriptions.endpoints';

/**
 * Contract tests for `/subscriptions`.
 *
 * The asymmetry worth pinning: `GetSubscription` needs only the path id, but
 * update and delete additionally require `subscriberId` as a **query**
 * parameter. Nothing in the URL hints at that, so a caller that omits it gets a
 * 400 rather than an obvious mistake.
 */

const SUBSCRIPTION_ID = 'subscription-1';
const RESOURCE = `/subscriptions/${SUBSCRIPTION_ID}`;

let requests: Recorder;

beforeEach(() => {
  requests = recorder();
  resetHttpClient();
});

describe('listSubscriptions', () => {
  it('GETs the collection', async () => {
    server.use(collection('/subscriptions', [aSubscription()], { record: requests }));

    await listSubscriptions();

    expect(requests.last()?.method).toBe('GET');
    expect(requests.last()?.url.pathname).toBe('/api/v0.9/subscriptions');
  });

  it('passes the artifact, application and subscriber filters through', async () => {
    // These are how a subscription list is scoped to one API or one
    // application; dropping one silently widens the list.
    server.use(collection('/subscriptions', [], { record: requests }));

    await listSubscriptions({
      query: {
        artifactId: 'pizza-shack',
        applicationId: 'checkout-app',
        subscriberId: 'user-1',
        limit: 10,
      },
    });

    expect(Object.fromEntries(requests.last()!.params)).toEqual({
      applicationId: 'checkout-app',
      artifactId: 'pizza-shack',
      limit: '10',
      subscriberId: 'user-1',
    });
  });
});

describe('getSubscription', () => {
  it('GETs one subscription with only the path id', async () => {
    server.use(resource(RESOURCE, aSubscription(), { record: requests }));

    await getSubscription(SUBSCRIPTION_ID);

    expect(requests.last()?.url.pathname).toBe('/api/v0.9/subscriptions/subscription-1');
    expect(requests.last()?.params.get('subscriberId')).toBeNull();
  });

  it('percent-encodes the id', async () => {
    server.use(resource('/subscriptions/:subscriptionId', aSubscription(), { record: requests }));

    await getSubscription('weird/id');

    expect(requests.last()?.url.pathname).toBe('/api/v0.9/subscriptions/weird%2Fid');
  });
});

describe('createSubscription', () => {
  it('POSTs to the collection with the request body', async () => {
    server.use(accepts('post', '/subscriptions', aSubscription(), { record: requests }));

    await createSubscription({
      artifactId: 'pizza-shack',
      applicationId: 'checkout-app',
    } as never);

    expect(requests.last()?.method).toBe('POST');
    expect(JSON.parse(requests.last()!.body)).toMatchObject({
      artifactId: 'pizza-shack',
    });
  });
});

describe('updateSubscription', () => {
  it('PUTs to the resource path and carries subscriberId as a query parameter', async () => {
    server.use(accepts('put', RESOURCE, aSubscription(), { record: requests }));

    await updateSubscription(SUBSCRIPTION_ID, aSubscription(), {
      query: { subscriberId: 'user-1' },
    });

    expect(requests.last()?.method).toBe('PUT');
    expect(requests.last()?.url.pathname).toBe('/api/v0.9/subscriptions/subscription-1');
    expect(requests.last()?.params.get('subscriberId')).toBe('user-1');
  });
});

describe('deleteSubscription', () => {
  it('DELETEs the resource path and carries subscriberId as a query parameter', async () => {
    server.use(noContent('delete', RESOURCE, { record: requests }));

    await deleteSubscription(SUBSCRIPTION_ID, { query: { subscriberId: 'user-1' } });

    expect(requests.last()?.method).toBe('DELETE');
    expect(requests.last()?.params.get('subscriberId')).toBe('user-1');
  });

  it('resolves to nothing on 204', async () => {
    server.use(noContent('delete', RESOURCE));

    await expect(
      deleteSubscription(SUBSCRIPTION_ID, { query: { subscriberId: 'user-1' } })
    ).resolves.toBeUndefined();
  });
});

describe('failures', () => {
  it('labels the failing operation', async () => {
    server.use(failure('get', RESOURCE, 404, 'NOT_FOUND'));

    const error = (await getSubscription(SUBSCRIPTION_ID).catch(
      (e: unknown) => e
    )) as ApiError;

    expect(error.operation).toBe('GetSubscription');
  });

  it('rejects when the server refuses the update', async () => {
    server.use(failure('put', RESOURCE, 403, 'FORBIDDEN'));

    await expect(
      updateSubscription(SUBSCRIPTION_ID, aSubscription(), {
        query: { subscriberId: 'user-1' },
      })
    ).rejects.toBeInstanceOf(ApiError);
  });
});
