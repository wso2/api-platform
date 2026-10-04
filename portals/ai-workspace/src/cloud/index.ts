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
 *
 * Injection seam for cloud-only extensions and branding. `main.tsx` imports
 * `cloudExtensions` and `cloudBrandLogo` from here unconditionally, so this
 * file must always exist and export both — this is what lets a downstream
 * build overlay just this one file/directory with the real cloud host module
 * (`portals/cloud-plugins/apip-cloud-ui/src/hosts/ai-workspace.tsx`), without
 * ever touching App.tsx/main.tsx/extensions.tsx. Mirrors
 * `portals/api-control-plane/src/cloud/index.ts`.
 */

import type { BrandLogo } from "../branding/BrandLogoProvider";
import type { AIWorkspaceCloudEntry } from "../extensions";

export const cloudExtensions: AIWorkspaceCloudEntry[] = [];

/** Cloud logo placeholder; undefined falls back to the on-prem logo. */
export const cloudBrandLogo: BrandLogo | undefined = undefined;
