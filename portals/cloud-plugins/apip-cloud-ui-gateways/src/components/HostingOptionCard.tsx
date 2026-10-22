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

import type { FC, ReactNode } from 'react';
import { Box, Card, Radio, Typography } from '@wso2/oxygen-ui';

export type HostingOptionCardProps = {
  icon: ReactNode;
  title: string;
  description: string;
  selected: boolean;
  onClick: () => void;
};

/**
 * One hosting choice on the first step of Add Gateway — bigger and more
 * explanatory than `GatewayTypeCard`'s row (icon, title, two-line description,
 * a radio rather than a plain selected state), since a hosting choice is a
 * one-time, harder-to-reverse decision the admin needs more to go on than a
 * type pick does.
 */
const HostingOptionCard: FC<HostingOptionCardProps> = ({ icon, title, description, selected, onClick }) => (
  <Card
    role="radio"
    aria-checked={selected}
    tabIndex={0}
    onClick={onClick}
    onKeyDown={(event) => {
      if (event.key === 'Enter' || event.key === ' ') {
        event.preventDefault();
        onClick();
      }
    }}
    sx={{
      flex: 1,
      display: 'flex',
      alignItems: 'flex-start',
      gap: 1.5,
      p: 2,
      border: '1px solid',
      borderColor: selected ? 'primary.main' : 'divider',
      cursor: 'pointer',
      transition: 'border-color 0.15s ease, background-color 0.15s ease',
      '&:hover': selected ? undefined : { borderColor: 'text.disabled' },
    }}
  >
    <Box
      sx={{
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        width: 36,
        height: 36,
        borderRadius: '8px',
        border: '1px solid',
        borderColor: 'divider',
        bgcolor: 'action.hover',
        color: 'primary.main',
        flexShrink: 0,
      }}
    >
      {icon}
    </Box>
    <Box sx={{ flexGrow: 1, minWidth: 0 }}>
      <Typography variant="body2" sx={{ fontWeight: 600 }}>
        {title}
      </Typography>
      <Typography variant="caption" color="text.secondary" sx={{ display: 'block', opacity: 0.7 }}>
        {description}
      </Typography>
    </Box>
    <Radio checked={selected} tabIndex={-1} sx={{ p: 0, mt: 0.25 }} />
  </Card>
);

export default HostingOptionCard;
