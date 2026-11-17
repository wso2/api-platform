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

import { useCallback } from 'react';

import { useAuth } from '@/contexts/auth/AuthProvider';
import { trackFeatureUsage } from './collector';
import {
  isFeatureCode,
  type ApiPlatformFeatureCode,
  type FeatureDimensionArgs,
  type FeatureDimensions,
} from './taxonomy';

export type TrackFeature = <C extends ApiPlatformFeatureCode>(
  code: C,
  ...dimensions: FeatureDimensionArgs<C>
) => void;

/**
 * Returns `trackFeature(code, dimensions)` for reporting one step of a feature
 * from the taxonomy. Safe to call from any event handler: it never throws, never
 * awaits, and does nothing when tracking is off. The dimensions may be left out
 * for a feature that has no required one.
 *
 * The action is attributed to the signed-in user and organization, read from
 * the session here so no call site has to pass (or can misstate) them.
 */
export function useTrackFeature(): TrackFeature {
  const { user } = useAuth();
  const userId = user?.id;
  const companyId = user?.org?.id;

  return useCallback<TrackFeature>(
    (code, ...[dimensions]) => trackFeatureUsage(code, dimensions, { companyId, userId }),
    [companyId, userId],
  );
}

/** {@link TrackFeature} for code that cannot see the taxonomy's types. */
export type TrackFeatureByName = (
  code: string,
  dimensions?: Readonly<Record<string, string>>,
) => void;

/**
 * Returns `trackFeature(code, dimensions)` taking the code as a plain string,
 * for the extensions the host port is handed to (cloud pages that replace a
 * built-in one and report its features). A code the taxonomy doesn't list as a
 * console feature is dropped, and so is any dimension or value the feature
 * doesn't declare (`featureMetadata`), so an extension can report only what a
 * built-in page could.
 */
export function useTrackFeatureByName(): TrackFeatureByName {
  const { user } = useAuth();
  const userId = user?.id;
  const companyId = user?.org?.id;

  return useCallback<TrackFeatureByName>(
    (code, dimensions) => {
      if (!isFeatureCode(code)) return;
      trackFeatureUsage(code, dimensions as FeatureDimensions<typeof code>, { companyId, userId });
    },
    [companyId, userId],
  );
}
