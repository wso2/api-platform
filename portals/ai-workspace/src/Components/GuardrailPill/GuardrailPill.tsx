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
import { Box, IconButton, Typography } from '@wso2/oxygen-ui';
import { X } from '@wso2/oxygen-ui-icons-react';

export type GuardrailPillProps = {
  label: string;
  onClick?: () => void;
  onRemove?: () => void;
  removeAriaLabel?: string;
};

export default function GuardrailPill({
  label,
  onClick,
  onRemove,
  removeAriaLabel = 'Remove guardrail',
}: GuardrailPillProps) {
  return (
    <Box
      onClick={onClick}
      onKeyDown={
        onClick
          ? (event) => {
              if (event.key === 'Enter' || event.key === ' ') {
                event.preventDefault();
                onClick();
              }
            }
          : undefined
      }
      role={onClick ? 'button' : undefined}
      tabIndex={onClick ? 0 : undefined}
      sx={{
        maxWidth: '100%',
        minHeight: 34,
        border: '1px solid',
        borderColor: 'rgba(237, 108, 2, 0.35)',
        borderRadius: 0.5,
        pl: 1.25,
        pr: 0.5,
        py: 0.25,
        display: 'inline-flex',
        alignItems: 'center',
        gap: 0.25,
        backgroundColor: 'rgba(237, 108, 2, 0.08)',
        transition:
          'background-color 160ms ease, border-color 160ms ease, box-shadow 160ms ease',
        cursor: onClick ? 'pointer' : 'default',
        '&:hover': onClick
          ? {
              borderColor: 'primary.main',
              backgroundColor: 'rgba(237, 108, 2, 0.14)',
              boxShadow: '0 1px 4px rgba(0, 0, 0, 0.08)',
            }
          : undefined,
        '&:focus-visible': onClick
          ? {
              outline: '2px solid',
              outlineColor: 'primary.main',
              outlineOffset: 2,
            }
          : undefined,
      }}
    >
      <Typography
        variant="body2"
        sx={{
          // color: '#AD4A08',
          fontWeight: 600,
          lineHeight: 1.2,
          maxWidth: '100%',
          whiteSpace: 'nowrap',
          overflow: 'hidden',
          textOverflow: 'ellipsis',
        }}
      >
        {label}
      </Typography>
      {onRemove ? (
        <IconButton
          size="small"
          aria-label={removeAriaLabel}
          onClick={(event) => {
            event.stopPropagation();
            onRemove();
          }}
          sx={{
            ml: 0.25,
            p: 0.4,
            color: '#9A5D39',
            '&:hover': {
              backgroundColor: 'rgba(211, 47, 47, 0.12)',
              color: 'error.main',
            },
          }}
        >
          <X size={13} />
        </IconButton>
      ) : null}
    </Box>
  );
}
