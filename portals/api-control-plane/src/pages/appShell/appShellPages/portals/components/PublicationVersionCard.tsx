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

import { alpha, Box, Paper, Stack, Typography } from '@wso2/oxygen-ui';
import { Circle } from '@wso2/oxygen-ui-icons-react';
import type { ReactNode } from 'react';

import { hairline } from '@/theme/receipes';

export type PublicationVersionTone = 'draft' | 'published';

export type PublicationVersionBanner = {
  /** When the version last changed. The switch already says which version it is. */
  meta: ReactNode;
};

type PublicationVersionCardProps = {
  /** Left out when there is no saved version to describe, such as a draft that was never saved. */
  banner?: PublicationVersionBanner;
  children: ReactNode;
  /** Draft is the working copy; published is the live, read-only one and is set apart in the success colour. */
  tone: PublicationVersionTone;
};

/**
 * The frame around one version of the listing, draft or published: a banner
 * describing the one on screen, over the tab's content. The banner and border
 * change colour with the version, so it is never unclear whether the fields are
 * the working copy or the live one.
 */
export function PublicationVersionCard({ banner, children, tone }: PublicationVersionCardProps) {
  const published = tone === 'published';

  return (
    <Paper
      sx={{
        borderColor: published ? 'success.main' : 'divider',
        display: 'flex',
        flex: 1,
        flexDirection: 'column',
        minHeight: 0,
        overflow: 'hidden',
      }}
      variant="outlined"
    >
      {banner && (
        <Stack
          alignItems="center"
          direction="row"
          spacing={1.5}
          sx={(theme) => ({
            bgcolor: published ? alpha(theme.palette.success.main, 0.08) : 'action.hover',
            borderBottom: hairline(theme),
            borderColor: 'divider',
            flexShrink: 0,
            flexWrap: 'wrap',
            px: 3,
            py: 1.5,
          })}
        >
          <Box sx={{ color: published ? 'success.main' : 'primary.main', display: 'flex' }}>
            <Circle fill="currentColor" size={8} />
          </Box>
          <Typography color="text.secondary" variant="body2">
            {banner.meta}
          </Typography>
        </Stack>
      )}
      <Box sx={{ flex: 1, minHeight: 0, overflowY: 'auto' }}>{children}</Box>
    </Paper>
  );
}
