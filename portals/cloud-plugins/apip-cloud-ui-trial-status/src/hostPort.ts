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
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import type { BillingOrganization } from './types';

/**
 * The slice of the host's Port this feature needs, hand-mirrored rather than
 * imported so the same component works in either portal (see the cloud
 * extensions convention: a feature receives a value of this shape as a prop,
 * never a host's own context).
 *
 * Billing is read through the host because that read is not a plain GET: it
 * performs first-login subscription activation as a side effect, so the host
 * owns it and every caller shares one request. `organization` resolves null
 * when the deployment has no billing upstream.
 */
export type TrialStatusHostPort = {
  billing: {
    organization: () => Promise<BillingOrganization | null>;
  };
};
