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
import PolicyMappingIndicator, {
  PolicyMappingItem,
} from './PolicyMappingIndicator';

export type ResourceViewItem = {
  method: string;
  path: string;
  summary?: string;
};

type ResourceRowProps = {
  resource: ResourceViewItem;
  faded?: boolean;
  leading?: React.ReactNode;
  trailing?: React.ReactNode;
  selected?: boolean;
  onClick?: () => void;
  showSummary?: boolean;
  enablePolicyMapping?: boolean;
  policies?: PolicyMappingItem[];
};

const DARK_MODE_SELECTOR = [
  'html[data-mui-color-scheme="dark"] &',
  'body[data-mui-color-scheme="dark"] &',
  'html[data-color-scheme="dark"] &',
  'body[data-color-scheme="dark"] &',
  'html[data-theme="dark"] &',
  'body[data-theme="dark"] &',
  'html.dark &',
  'body.dark &',
].join(', ');

function truncateText(value: string, maxLength = 90) {
  if (value.length <= maxLength) return value;
  return `${value.slice(0, maxLength - 3).trimEnd()}...`;
}

function getMethodTheme(methodRaw: string) {
  const method = methodRaw.toUpperCase();
  switch (method) {
    case 'GET':
      return {
        border: '#61affe',
        bg: 'rgba(97, 175, 254, 0.14)',
        badge: '#61affe',
      };
    case 'POST':
      return {
        border: '#49cc90',
        bg: 'rgba(73, 204, 144, 0.14)',
        badge: '#49cc90',
      };
    case 'PUT':
      return {
        border: '#fca130',
        bg: 'rgba(252, 161, 48, 0.16)',
        badge: '#fca130',
      };
    case 'DELETE':
      return {
        border: '#f93e3e',
        bg: 'rgba(249, 62, 62, 0.14)',
        badge: '#f93e3e',
      };
    case 'PATCH':
      return {
        border: '#50e3c2',
        bg: 'rgba(80, 227, 194, 0.14)',
        badge: '#50e3c2',
      };
    case 'HEAD':
      return {
        border: '#9012fe',
        bg: 'rgba(144, 18, 254, 0.14)',
        badge: '#9012fe',
      };
    case 'OPTIONS':
      return {
        border: '#0d5aa7',
        bg: 'rgba(13, 90, 167, 0.14)',
        badge: '#0d5aa7',
      };
    default:
      return {
        border: '#a0a0a0',
        bg: 'rgba(160, 160, 160, 0.12)',
        badge: '#a0a0a0',
      };
  }
}

function MethodBadge({ method }: { method: string }) {
  const t = getMethodTheme(method);

  return (
    <Box
      sx={{
        minWidth: 64,
        height: 30,
        px: 1.25,
        borderRadius: 0.5,
        display: 'inline-flex',
        alignItems: 'center',
        justifyContent: 'center',
        color: '#fff',
        fontWeight: 700,
        fontSize: 12,
        letterSpacing: 0.35,
        textTransform: 'uppercase',
        backgroundColor: t.badge,
      }}
    >
      {method}
    </Box>
  );
}

export default function ResourceRow({
  resource,
  faded,
  leading,
  trailing,
  selected,
  onClick,
  showSummary = true,
  enablePolicyMapping = false,
  policies = [],
}: ResourceRowProps) {
  const t = getMethodTheme(resource.method);
  const summaryMaxLength = 100;
  const summaryText = resource.summary
    ? truncateText(resource.summary, summaryMaxLength)
    : '';
  const isSummaryTruncated = Boolean(
    resource.summary && resource.summary.length > summaryMaxLength
  );

  return (
    <Box
      sx={{
        width: '100%',
        minWidth: 0,
        maxWidth: '100%',
        display: 'flex',
        alignItems: 'center',
        minHeight: 44,
        gap: 1.25,
        px: 1.1,
        py: 0.6,
        borderRadius: 0.75,
        border: `${selected ? 2 : 1}px solid ${t.border}`,
        backgroundColor: faded ? 'action.hover' : t.bg,
        boxShadow: selected
          ? `inset 0 0 0 2px ${t.border}, 0 0 0 1px #000`
          : 'none',
        opacity: faded ? 0.6 : 1,
        cursor: onClick ? 'pointer' : 'default',
        transition:
          'box-shadow 160ms ease, border-color 160ms ease, filter 160ms ease',
        '&:hover': onClick
          ? {
              filter: 'brightness(0.99)',
            }
          : undefined,
      }}
      onClick={onClick}
    >
      {leading}

      <Box
        sx={{
          display: 'flex',
          alignItems: 'center',
          gap: 1,
          minWidth: 0,
          maxWidth: '100%',
          flex: 1,
        }}
      >
        <MethodBadge method={resource.method} />

        <Box
          sx={{
            minWidth: 0,
            maxWidth: '100%',
            flex: 1,
            display: 'flex',
            flexDirection: 'column',
            alignItems: 'flex-start',
            gap: 0.25,
          }}
        >
          <Typography
            variant="body2"
            sx={{
              fontFamily:
                'ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace',
              fontWeight: 700,
              color: '#3b4151',
              [DARK_MODE_SELECTOR]: {
                color: '#fff !important',
              },
              minWidth: 0,
              maxWidth: '100%',
              width: '100%',
              whiteSpace: 'nowrap',
              overflow: 'hidden',
              textOverflow: 'ellipsis',
            }}
          >
            {resource.path}
          </Typography>
          {showSummary && resource.summary && (
            <Tooltip title={isSummaryTruncated ? resource.summary : ''} arrow>
              <Typography
                variant="body2"
                sx={{
                  color: '#3b4151',
                  [DARK_MODE_SELECTOR]: {
                    color: '#fff !important',
                  },
                  opacity: 0.9,
                  minWidth: 0,
                  width: '100%',
                  maxWidth: '100%',
                  whiteSpace: 'nowrap',
                  overflow: 'hidden',
                  textOverflow: 'ellipsis',
                  display: 'block',
                }}
              >
                {summaryText}
              </Typography>
            </Tooltip>
          )}
        </Box>
      </Box>

      {enablePolicyMapping && policies.length > 0 && (
        <PolicyMappingIndicator policies={policies} />
      )}

      <Box
        sx={{
          display: 'inline-flex',
          alignItems: 'center',
          color: '#3b4151',
          '& .MuiIconButton-root': {
            borderRadius: 0.5,
            p: 0.35,
            color: 'inherit',
          },
          '& .MuiIconButton-root:hover': {
            backgroundColor: 'transparent',
            opacity: 0.85,
          },
        }}
      >
        {trailing}
      </Box>
    </Box>
  );
}
