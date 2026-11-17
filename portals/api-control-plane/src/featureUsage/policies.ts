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

import { policyFeatureOf, type FeatureDimensions, type PolicyFeatureCode } from './taxonomy';
import type { TrackFeature } from './useTrackFeature';

/** What is known about where a policy was attached; every policy feature declares both. */
export type PolicyAttachmentContext = Pick<
  FeatureDimensions<'apip.policies.cors-attach'>,
  'artifact_kind' | 'policy_scope'
>;

/**
 * Reports one policy newly attached to an API, under the capability feature its
 * name belongs to (`api-key-auth` → `apip.policies.client-authentication-attach`).
 * A policy the taxonomy doesn't classify, such as a custom one, is not
 * reported. `policy_name` is sent only by the features that declare it; for the
 * single-policy capabilities (CORS, access control, analytics filtering) the
 * metadata builder leaves it out.
 */
export function trackPolicyAttachment(
  trackFeature: TrackFeature,
  policyName: string,
  context: PolicyAttachmentContext,
): void {
  const code = policyFeatureOf(policyName);
  if (code === undefined) return;
  // `code` is a union, so it can't select one feature's dimension type; the
  // dimensions below are valid for every member (see policyDimensions in
  // taxonomy.ts), and featureMetadata drops any a feature doesn't declare.
  const track = trackFeature as (
    code: PolicyFeatureCode,
    dimensions: PolicyAttachmentContext & { policy_name: string },
  ) => void;
  track(code, { ...context, policy_name: policyName });
}
