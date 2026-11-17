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

import { renderHook } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { useTrackFeatureByName } from './useTrackFeature';

const track = vi.hoisted(() => vi.fn());
vi.mock('./collector', () => ({ trackFeatureUsage: track }));

const USER = '11111111-1111-4111-8111-111111111111';
const ORG = '22222222-2222-4222-8222-222222222222';
vi.mock('@/contexts/auth/AuthProvider', () => ({
  useAuth: () => ({ user: { id: USER, org: { id: ORG } } }),
}));

beforeEach(() => track.mockReset());

/*
 * The host port's `trackFeature`, handed to cloud pages that replace a built-in
 * one: they name features by plain string, so the taxonomy has to be checked here.
 */
describe('useTrackFeatureByName', () => {
  it('reports a console feature by its code, attributed to the session', () => {
    const { result } = renderHook(() => useTrackFeatureByName());

    result.current('apip.lifecycle.api-deploy-click', {
      artifact_kind: 'rest-api',
      gateway_type: 'regular',
    });

    expect(track.mock.calls).toEqual([
      [
        'apip.lifecycle.api-deploy-click',
        { artifact_kind: 'rest-api', gateway_type: 'regular' },
        { companyId: ORG, userId: USER },
      ],
    ]);
  });

  it('drops a code that is not a console feature', () => {
    const { result } = renderHook(() => useTrackFeatureByName());

    result.current('apip.lifecycle.api-deploy');
    result.current('not-a-feature');

    expect(track).not.toHaveBeenCalled();
  });
});
