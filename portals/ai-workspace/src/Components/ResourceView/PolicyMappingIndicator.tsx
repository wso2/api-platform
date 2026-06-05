/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import React from 'react';
import { Box, Tooltip, Typography } from '@wso2/oxygen-ui';

export type PolicyMappingItem = {
  id: string;
  displayName: string;
  version: string;
};

type PolicyMappingIndicatorProps = {
  policies: PolicyMappingItem[];
};

const COLORS = [
  '#FF6B35',
  '#1976d2',
  '#9C27B0',
  '#2E7D32',
  '#C62828',
  '#00838F',
  '#EF6C00',
  '#4527A0',
];

function getInitials(name: string): string {
  const words = name.trim().split(/\s+/);
  if (words.length >= 2) {
    return (words[0][0] + words[1][0]).toUpperCase();
  }
  return name.slice(0, 2).toUpperCase();
}

function getColor(index: number): string {
  return COLORS[index % COLORS.length];
}

export default function PolicyMappingIndicator({
  policies,
}: PolicyMappingIndicatorProps) {
  if (!policies || policies.length === 0) return null;

  return (
    <Box
      sx={{
        display: 'flex',
        alignItems: 'center',
        flexShrink: 0,
      }}
    >
      {policies.map((policy, index) => (
        <Tooltip
          key={policy.id}
          title={`${policy.displayName} (${policy.version.replace(/^v/, '')})`}
          arrow
        >
          <Box
            sx={{
              width: 28,
              height: 28,
              borderRadius: '50%',
              border: '2px solid',
              borderColor: 'background.paper',
              backgroundColor: getColor(index),
              display: 'inline-flex',
              alignItems: 'center',
              justifyContent: 'center',
              marginLeft: index === 0 ? 0 : '-6px',
              zIndex: policies.length - index,
              cursor: 'default',
            }}
          >
            <Typography
              sx={{
                color: '#fff',
                fontSize: 10,
                fontWeight: 700,
                lineHeight: 1,
                userSelect: 'none',
              }}
            >
              {getInitials(policy.displayName)}
            </Typography>
          </Box>
        </Tooltip>
      ))}
    </Box>
  );
}
