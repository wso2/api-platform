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
  aDeployRequest,
  aDeployment,
  accepts,
  collection,
  failure,
  noContent,
  recorder,
  resource,
  type Recorder,
} from '../../../../test/msw';
import { server } from '../../../../test/server';
import { ApiError } from '../../../core/errors';
import { resetHttpClient } from '../../../core/http';
import {
  deleteDeployment,
  deployApi,
  getDeployment,
  listDeployments,
  restoreDeployment,
  undeployDeployment,
} from './deployments.endpoints';

/**
 * Contract tests for `/rest-apis/{restApiId}/deployments`.
 *
 * What distinguishes this resource from a top-level one is that **every path is
 * rooted at a parent API**. Most of these tests are therefore about the URL: a
 * missing or mis-encoded parent segment addresses another API's deployments,
 * which is a data-integrity problem rather than a 404.
 */

const API_ID = 'pizza-shack';
const DEPLOYMENT_ID = 'deployment-1';
const COLLECTION = `/rest-apis/${API_ID}/deployments`;
const RESOURCE = `${COLLECTION}/${DEPLOYMENT_ID}`;

let requests: Recorder;

beforeEach(() => {
  requests = recorder();
  resetHttpClient();
});

describe('listDeployments', () => {
  it('GETs the deployments of one API, not a global collection', async () => {
    server.use(collection(COLLECTION, [aDeployment()], { record: requests }));

    await listDeployments(API_ID);

    expect(requests.last()?.method).toBe('GET');
    expect(requests.last()?.url.pathname).toBe(
      '/api/v0.9/rest-apis/pizza-shack/deployments'
    );
  });

  it('passes the gateway and status filters through', async () => {
    // The deploy screen lists per gateway; dropping these would show every
    // deployment under whichever gateway happened to be selected.
    server.use(collection(COLLECTION, [], { record: requests }));

    await listDeployments(API_ID, {
      query: { gatewayId: 'shared-gateway', status: 'DEPLOYED', limit: 5 },
    });

    expect(Object.fromEntries(requests.last()!.params)).toEqual({
      gatewayId: 'shared-gateway',
      limit: '5',
      status: 'DEPLOYED',
    });
  });

  it('percent-encodes the parent API handle', async () => {
    server.use(
      collection('/rest-apis/:restApiId/deployments', [], { record: requests })
    );

    await listDeployments('weird/handle');

    expect(requests.last()?.url.pathname).toBe(
      '/api/v0.9/rest-apis/weird%2Fhandle/deployments'
    );
  });

  it('returns the collection envelope, pagination included', async () => {
    server.use(collection(COLLECTION, [aDeployment(), aDeployment()]));

    const response = await listDeployments(API_ID);

    expect(response.list).toHaveLength(2);
    expect(response.pagination).toMatchObject({ total: 2 });
  });
});

describe('getDeployment', () => {
  it('GETs one deployment beneath its API', async () => {
    server.use(resource(RESOURCE, aDeployment(), { record: requests }));

    await getDeployment(API_ID, DEPLOYMENT_ID);

    expect(requests.last()?.url.pathname).toBe(
      '/api/v0.9/rest-apis/pizza-shack/deployments/deployment-1'
    );
  });

  it('encodes both path segments', async () => {
    server.use(
      resource('/rest-apis/:restApiId/deployments/:deploymentId', aDeployment(), {
        record: requests,
      })
    );

    await getDeployment('a/b', 'c d');

    expect(requests.last()?.url.pathname).toBe(
      '/api/v0.9/rest-apis/a%2Fb/deployments/c%20d'
    );
  });
});

describe('deployApi', () => {
  it('POSTs to the API’s deployment collection with the request body', async () => {
    server.use(
      accepts('post', COLLECTION, aDeployment({ status: 'DEPLOYING' }), {
        record: requests,
      })
    );

    await deployApi(API_ID, aDeployRequest({ gatewayId: 'shared-gateway' }));

    expect(requests.last()?.method).toBe('POST');
    expect(requests.last()?.url.pathname).toBe(
      '/api/v0.9/rest-apis/pizza-shack/deployments'
    );
    expect(JSON.parse(requests.last()!.body)).toMatchObject({
      gatewayId: 'shared-gateway',
    });
  });

  it('returns the deployment in its transitional state', async () => {
    // The gateway acknowledges asynchronously, so the immediate response is
    // DEPLOYING rather than DEPLOYED — the polling predicate depends on that.
    server.use(accepts('post', COLLECTION, aDeployment({ status: 'DEPLOYING' })));

    await expect(deployApi(API_ID, aDeployRequest())).resolves.toMatchObject({
      status: 'DEPLOYING',
    });
  });
});

describe('undeployDeployment', () => {
  it('POSTs to the undeploy sub-path', async () => {
    server.use(
      accepts('post', `${RESOURCE}/undeploy`, aDeployment(), {
        record: requests,
        status: 200,
      })
    );

    await undeployDeployment(API_ID, DEPLOYMENT_ID);

    expect(requests.last()?.url.pathname).toBe(
      '/api/v0.9/rest-apis/pizza-shack/deployments/deployment-1/undeploy'
    );
  });

  it('sends no request body', async () => {
    // The action is fully identified by the path; a body would be ignored and
    // would only confuse the CSRF/content-type handling.
    server.use(
      accepts('post', `${RESOURCE}/undeploy`, aDeployment(), {
        record: requests,
        status: 200,
      })
    );

    await undeployDeployment(API_ID, DEPLOYMENT_ID);

    expect(requests.last()?.body).toBe('');
  });
});

describe('restoreDeployment', () => {
  it('POSTs to the restore sub-path', async () => {
    server.use(
      accepts('post', `${RESOURCE}/restore`, aDeployment(), {
        record: requests,
        status: 200,
      })
    );

    await restoreDeployment(API_ID, DEPLOYMENT_ID);

    expect(requests.last()?.url.pathname).toBe(
      '/api/v0.9/rest-apis/pizza-shack/deployments/deployment-1/restore'
    );
  });
});

describe('deleteDeployment', () => {
  it('DELETEs the deployment', async () => {
    server.use(noContent('delete', RESOURCE, { record: requests }));

    await deleteDeployment(API_ID, DEPLOYMENT_ID);

    expect(requests.last()?.method).toBe('DELETE');
    expect(requests.last()?.url.pathname).toBe(
      '/api/v0.9/rest-apis/pizza-shack/deployments/deployment-1'
    );
  });

  it('resolves to nothing on 204', async () => {
    server.use(noContent('delete', RESOURCE));

    await expect(deleteDeployment(API_ID, DEPLOYMENT_ID)).resolves.toBeUndefined();
  });
});

describe('failures', () => {
  it('surfaces the gateway-unavailable code so the UI can explain the cause', async () => {
    // A deploy blocked because the gateway is not connected is a different
    // story from a validation error, and only `code` distinguishes them.
    server.use(
      failure('post', COLLECTION, 503, 'GATEWAY_CONNECTION_UNAVAILABLE', {
        message: 'The gateway is not currently connected.',
      })
    );

    const error = (await deployApi(API_ID, aDeployRequest()).catch(
      (e: unknown) => e
    )) as ApiError;

    expect(error.code).toBe('GATEWAY_CONNECTION_UNAVAILABLE');
    expect(error.status).toBe(503);
  });

  it('labels each action distinctly for logs', async () => {
    server.use(
      failure('post', `${RESOURCE}/undeploy`, 409, 'DEPLOYMENT_ACTIVE')
    );

    const error = (await undeployDeployment(API_ID, DEPLOYMENT_ID).catch(
      (e: unknown) => e
    )) as ApiError;

    expect(error.operation).toBe('UndeployDeployment');
  });

  it('rejects rather than resolving when a delete fails', async () => {
    server.use(failure('delete', RESOURCE, 409, 'DEPLOYMENT_ACTIVE'));

    await expect(
      deleteDeployment(API_ID, DEPLOYMENT_ID)
    ).rejects.toBeInstanceOf(ApiError);
  });
});
