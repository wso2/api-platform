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

import { Skeleton, Stack } from '@wso2/oxygen-ui';

/**
 * One placeholder for the whole project overview while the project and its APIs load: a title, a
 * subtitle and a block for the content. The page is either the API list or an empty project's
 * welcome view, and only the loaded data says which, so the placeholder stays neutral.
 */
export function ProjectHomeSkeleton() {
  return (
    <Stack aria-busy="true" spacing={2} sx={{ pt: 1 }}>
      <Skeleton height={26} sx={{ borderRadius: 1.5 }} variant="rounded" width={250} />
      <Skeleton height={12} sx={{ borderRadius: 1 }} variant="rounded" width={375} />
      <Skeleton height={320} sx={{ borderRadius: 2, mt: 2 }} variant="rounded" width="100%" />
    </Stack>
  );
}
