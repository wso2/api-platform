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

import type { ReactNode } from 'react';
import { Box, Button, Stack, Typography } from '@wso2/oxygen-ui';

type QuickStartBannerProps = {
  title: string;
  description: string;
  actionLabel?: string;
  onAction?: () => void;
  icon?: ReactNode;
};

/**
 * Slim hero banner shown at the top of the home/overview dashboards
 */
export function QuickStartBanner({
  title,
  description,
  actionLabel,
  onAction,
  icon,
}: QuickStartBannerProps) {
  return (
    <Box>
      <Stack
        alignItems={{ sm: 'center' }}
        direction={{ xs: 'column', sm: 'row' }}
        justifyContent="space-between"
        spacing={2}
      >
        <Stack alignItems="center" direction="row" spacing={2}>
          {icon && (
            <Box
              sx={{
                alignItems: 'center',
                bgcolor: 'primary.main',
                borderRadius: 2,
                color: 'primary.contrastText',
                display: 'flex',
                height: 72,
                justifyContent: 'center',
                width: 72,
              }}
            >
              {icon}
            </Box>
          )}
          <Box>
            <Typography sx={{ fontWeight: 700 }} variant="h3">
              {title}
            </Typography>
            <Typography color="text.secondary" variant="body2">
              {description}
            </Typography>
          </Box>
        </Stack>
        {actionLabel && onAction && (
          <Button onClick={onAction} sx={{ flexShrink: 0 }} variant="contained">
            {actionLabel}
          </Button>
        )}
      </Stack>
    </Box>
  );
}
