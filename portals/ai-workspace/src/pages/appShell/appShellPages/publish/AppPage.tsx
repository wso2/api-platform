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

import { PageContent, Stack, ThemeProvider } from '@wso2/oxygen-ui';
import type { PageContentProps } from '@wso2/oxygen-ui';

import { withControlPlaneTypography } from './publishTypography';

/**
 * `PageContent` for the Publish pages, under the control plane's type scale (dialogs and menus
 * included, through the theme) so they read at the same sizes.
 */
export function AppPage({ children, fullWidth = true, sx, ...pageContentProps }: PageContentProps) {
  return (
    <ThemeProvider theme={withControlPlaneTypography}>
      <PageContent fullWidth={fullWidth} sx={sx} {...pageContentProps}>
        <Stack spacing={1}>{children}</Stack>
      </PageContent>
    </ThemeProvider>
  );
}
