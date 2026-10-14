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

import { PageContent, Stack } from '@wso2/oxygen-ui';
import type { PageContentProps } from '@wso2/oxygen-ui';

export type AppPageProps = PageContentProps;

/**
 * The outer container of every page in the app shell: `PageContent` around the
 * page's own content.
 *
 * Each page renders this itself; the shell adds no container of its own, so a
 * page decides its width and padding. Defaults match the console's standard
 * layout (full width, `py: 5`); any `PageContent` prop overrides them.
 */
export function AppPage({ children, fullWidth = true, sx, ...pageContentProps }: AppPageProps) {
  return (
    <PageContent
      fullWidth={fullWidth}
      sx={[{ py: 5 }, ...(sx === undefined ? [] : Array.isArray(sx) ? sx : [sx])]}
      {...pageContentProps}
    >
      <Stack spacing={1}>{children}</Stack>
    </PageContent>
  );
}
