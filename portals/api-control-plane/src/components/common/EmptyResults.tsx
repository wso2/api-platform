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
import { SecondaryText } from './SecondaryText';

export type EmptyResultsProps = {
  title: ReactNode;
  description?: ReactNode;
  illustration?: ReactNode;
  actionLabel?: ReactNode;
  onAction?: () => void;
  animate?: boolean;
};

export function EmptyResults({
  title,
  description,
  illustration,
  actionLabel,
  onAction,
  animate = true,
}: EmptyResultsProps) {
  return (
    <Stack
      alignItems="center"
      spacing={1.5}
      sx={{
        border: '1px dashed',
        borderColor: 'divider',
        borderRadius: '20px',
        px: 3,
        py: 6,
        textAlign: 'center',
      }}
    >
      {illustration && (
        <Box
          aria-hidden
          sx={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            width: 72,
            height: 72,
            mb: 2,
            border: 1,
            borderColor: 'divider',
            borderRadius: '20px',
            bgcolor: 'action.hover',
            color: 'primary.main',
            '@keyframes emptyResultsFloat': {
              '0%, 100%': { transform: 'translateY(0)' },
              '50%': { transform: 'translateY(-6px)' },
            },
            animation: animate ? 'emptyResultsFloat 3s ease-in-out infinite' : 'none',
            '@media (prefers-reduced-motion: reduce)': { animation: 'none' },
          }}
        >
          {illustration}
        </Box>
      )}
      <Typography variant="h6" sx={{ fontWeight: 600 }}>
        {title}
      </Typography>
      {description && <SecondaryText sx={{ maxWidth: 560 }}>{description}</SecondaryText>}
      {actionLabel && onAction && (
        <Box sx={{ pt: 1 }}>
          <Button variant="outlined" color="secondary" onClick={onAction}>
            {actionLabel}
          </Button>
        </Box>
      )}
    </Stack>
  );
}
