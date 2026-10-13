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

import React from 'react';
import { Box, Card, Skeleton, Stack, Typography } from '@wso2/oxygen-ui';

type KindSummaryCardProps = {
  label: string;
  icon: React.ReactNode;
  count: number;
  isLoading?: boolean;
  selected: boolean;
  onSelect: () => void;
};

export default function KindSummaryCard({
  label,
  icon,
  count,
  isLoading,
  selected,
  onSelect,
}: KindSummaryCardProps): React.JSX.Element {
  return (
    <Card
      onClick={onSelect}
      sx={{
        p: 2,
        height: '100%',
        cursor: 'pointer',
        borderColor: selected ? 'primary.main' : 'divider',
        background: (theme) =>
          selected
            ? `color-mix(in srgb, ${theme.palette.primary.main} 8%, transparent)`
            : undefined,
        '&:hover': {
          borderColor: 'primary.main',
        },
      }}
    >
      <Stack spacing={1.5}>
        <Stack direction="row" alignItems="center" spacing={1}>
          <Box
            sx={{
              display: 'flex',
              alignItems: 'center',
              flexShrink: 0,
              color: 'primary.main',
              lineHeight: 0,
            }}
          >
            {icon}
          </Box>
        </Stack>

        <Box sx={{ minWidth: 0 }}>
          <Typography variant="body2" color="text.secondary" noWrap>
            {label}
          </Typography>
          {isLoading ? (
            <Skeleton variant="text" width={64} height={52} />
          ) : (
            <Stack direction="row" spacing={1} alignItems="baseline">
              <Typography variant="h3" sx={{ fontWeight: 600 }}>
                {count}
              </Typography>
              <Typography variant="caption" color="text.secondary">
                {count === 0 ? 'Not set up' : 'total'}
              </Typography>
            </Stack>
          )}
        </Box>
      </Stack>
    </Card>
  );
}
