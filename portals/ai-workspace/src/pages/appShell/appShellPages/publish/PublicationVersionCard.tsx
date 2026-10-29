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

import type { ReactNode } from 'react';
import { alpha, Box, Paper, Stack, Typography } from '@wso2/oxygen-ui';
import { Circle } from '@wso2/oxygen-ui-icons-react';

import { hairline } from './publishStyles';

export type PublicationVersionTone = 'draft' | 'published';

type PublicationVersionBanner = {
  /** When it last changed; the switch already names the version. */
  meta: ReactNode;
};

type PublicationVersionCardProps = {
  /** Left out when there is no saved version to describe, such as a draft that was never saved. */
  banner?: PublicationVersionBanner;
  children: ReactNode;
  tone: PublicationVersionTone;
};

/** One version of the listing, framed in its own colour so the draft and the live one are never confused. */
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
