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

import { AppBreadcrumbs, Box, PageContent, Stack } from '@wso2/oxygen-ui';
import type { PageContentProps, SxProps, Theme } from '@wso2/oxygen-ui';

import { useScopeBreadcrumbs } from '../navigation/useScopeBreadcrumbs';

export type AppPageProps = PageContentProps & {
  /**
   * Leaves the breadcrumb trail out. For full-page flows (creation wizards, the
   * portal publication page): a wizard is building the very scope a trail would
   * describe, and a page with its own back button would only have the crumbs
   * repeat where the user came from.
   */
  hideBreadcrumbs?: boolean;
};

const breadcrumbSx: SxProps<Theme> = (theme) => ({
  '& .MuiBreadcrumbs-li .MuiTypography-root': {
    fontSize: theme.typography.body2.fontSize,
    opacity: 0.55,
  },
  '& .MuiBreadcrumbs-li:last-of-type .MuiTypography-root': {
    color: 'text.primary',
    fontWeight: theme.typography.fontWeightMedium,
    opacity: 1,
  },
  '& .MuiBreadcrumbs-separator': {
    opacity: 0.45,
  },
});

/**
 * The outer container of every page in the app shell: `PageContent` with the
 * scope breadcrumb trail above the page's own content.
 *
 * Each page renders this itself; the shell adds no container of its own, so a
 * page decides its width and padding. Defaults match the console's standard
 * layout (full width, `py: 5`); any `PageContent` prop overrides them. The
 * trail renders inside the page's `PageContent`, so it always lines up with
 * the page body whatever width the page picks.
 */
export function AppPage({
  children,
  fullWidth = true,
  hideBreadcrumbs = false,
  sx,
  ...pageContentProps
}: AppPageProps) {
  const breadcrumbs = useScopeBreadcrumbs();
  const showBreadcrumbs = !hideBreadcrumbs && breadcrumbs.length > 1;

  return (
    <PageContent
      fullWidth={fullWidth}
      sx={[{ py: 5 }, ...(sx === undefined ? [] : Array.isArray(sx) ? sx : [sx])]}
      {...pageContentProps}
    >
      <Stack spacing={1}>
        {showBreadcrumbs && (
          <Box sx={{ pb: 1 }}>
            <AppBreadcrumbs items={breadcrumbs} sx={breadcrumbSx} />
          </Box>
        )}
        {children}
      </Stack>
    </PageContent>
  );
}
