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

import {
  alpha,
  Box,
  Chip,
  IconButton,
  Stack,
  Tooltip,
  Typography,
  type Theme,
} from '@wso2/oxygen-ui';
import { Trash2 } from '@wso2/oxygen-ui-icons-react';
import { FormattedMessage } from 'react-intl';

import type { Operation } from '@/api/resources/restApis';

type ChipColor = 'default' | 'error' | 'info' | 'primary' | 'secondary' | 'success' | 'warning';

const methodColor = (method: string): ChipColor => {
  switch (method.toUpperCase()) {
    case 'GET':
      return 'info';
    case 'POST':
      return 'success';
    case 'PUT':
      return 'warning';
    case 'DELETE':
      return 'error';
    case 'PATCH':
      return 'secondary';
    default:
      return 'default';
  }
};

const methodTone = (theme: Theme, method: string) => {
  const color = methodColor(method);
  return color === 'default' ? theme.palette.text.primary : theme.palette[color].main;
};

export type SwaggerOperationsViewProps = {
  operations: Operation[];
  isOperationDisabled?: (operation: Operation, index: number) => boolean;
  onDelete?: (index: number) => void;
  showDelete?: boolean;
};

/** Compact Swagger-style operation summary shared by pages that do not need Swagger's detail UI. */
export function SwaggerOperationsView({
  operations,
  isOperationDisabled,
  onDelete,
  showDelete = false,
}: SwaggerOperationsViewProps) {
  if (operations.length === 0) {
    return (
      <Typography color="text.secondary" variant="body2">
        <FormattedMessage
          id="apiControlPlane.components.SwaggerOperationsView.empty"
          defaultMessage="No operations available."
        />
      </Typography>
    );
  }

  return (
    <Stack spacing={1.5}>
      {operations.map((operation, index) => {
        const { method, path } = operation.request;
        const disabled = isOperationDisabled?.(operation, index) ?? false;
        return (
          <Box
            key={`${method}-${path}-${index}`}
            sx={(theme) => {
              const tone = methodTone(theme, method);
              return {
                alignItems: 'center',
                bgcolor: alpha(tone, 0.08),
                border: '1px solid',
                borderColor: alpha(tone, 0.3),
                borderRadius: 0.5,
                display: 'flex',
                gap: 1.5,
                minHeight: 48,
                opacity: disabled ? 0.45 : 1,
                px: 1,
                py: 0.75,
              };
            }}
          >
            <Chip
              color={methodColor(method)}
              label={method}
              size="small"
              sx={{ borderRadius: 0.4, flexShrink: 0, fontWeight: 700, minWidth: 80 }}
            />
            <Typography sx={{ flexShrink: 0, fontSize: '0.95rem', fontWeight: 700 }}>
              {path}
            </Typography>
            {operation.description && (
              <Typography
                color="text.secondary"
                noWrap
                sx={{ flex: 1, minWidth: 0, opacity: 0.7 }}
                variant="body2"
              >
                {operation.description}
              </Typography>
            )}
            {showDelete && onDelete && (
              <Tooltip title="Delete resource">
                <Box component="span" sx={{ display: 'inline-flex', flexShrink: 0 }}>
                  <IconButton
                    aria-label={`Delete ${method} ${path}`}
                    color="error"
                    disabled={disabled}
                    onClick={() => onDelete(index)}
                    size="small"
                  >
                    <Trash2 size={17} />
                  </IconButton>
                </Box>
              </Tooltip>
            )}
          </Box>
        );
      })}
    </Stack>
  );
}
