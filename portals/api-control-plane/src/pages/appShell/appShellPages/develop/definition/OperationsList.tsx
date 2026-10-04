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

import { useMemo, useState } from 'react';
import {
  Box,
  InputAdornment,
  MenuItem,
  Select,
  Stack,
  TextField,
  Typography,
} from '@wso2/oxygen-ui';
import { Search } from '@wso2/oxygen-ui-icons-react';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';

import { type Operation } from '@/api/resources/restApis';
import { SwaggerOperationsView } from '@/components/SwaggerOperationsView';

const HTTP_METHODS = ['GET', 'POST', 'PUT', 'DELETE', 'PATCH', 'HEAD', 'OPTIONS'] as const;
type HttpMethod = (typeof HTTP_METHODS)[number];

const messages = defineMessages({
  searchResources: {
    id: 'develop.definition.OperationsList.searchResources',
    defaultMessage: 'Search resources',
  },
  allMethods: {
    id: 'develop.definition.OperationsList.allMethods',
    defaultMessage: 'All methods',
  },
  operationsParseError: {
    id: 'develop.definition.OperationsList.operationsParseError',
    defaultMessage: 'The current spec cannot be parsed. Fix any syntax errors to preview operations.',
  },
});

interface OperationsListProps {
  /** All operations extracted from the spec */
  operations: Operation[];
  /** Callback when an operation is deleted (receives index in filtered operations) */
  onDelete: (index: number) => void;
  /** Whether to show the delete button on each operation */
  showDelete?: boolean;
  /** Whether the spec can be parsed (shows error state if false) */
  canParse: boolean;
}

export function OperationsList({
  operations,
  onDelete,
  showDelete = true,
  canParse,
}: OperationsListProps) {
  const intl = useIntl();

  const [searchTerm, setSearchTerm] = useState('');
  const [methodFilter, setMethodFilter] = useState<HttpMethod | 'all'>('all');

  const filteredOperations = useMemo<Operation[]>(() => {
    const searchLower = searchTerm.trim().toLowerCase();
    return operations.filter((operation) => {
      const matchesMethod = methodFilter === 'all' || operation.request.method === methodFilter;
      const matchesSearch =
        searchLower === '' ||
        operation.request.path.toLowerCase().includes(searchLower) ||
        operation.name?.toLowerCase().includes(searchLower) ||
        operation.description?.toLowerCase().includes(searchLower);
      return matchesMethod && matchesSearch;
    });
  }, [operations, searchTerm, methodFilter]);

  return (
    <Box sx={{ display: 'flex', flex: 1, flexDirection: 'column', minHeight: 0 }}>
      {/* Search and filter controls */}
      <Box sx={{ p: 2, pb: 1.5, flexShrink: 0 }}>
        <Stack alignItems="center" direction="row" spacing={1.5}>
          <TextField
            fullWidth
            onChange={(event) => setSearchTerm(event.target.value)}
            placeholder={intl.formatMessage(messages.searchResources)}
            size="small"
            slotProps={{
              input: {
                startAdornment: (
                  <InputAdornment position="start">
                    <Search size={18} />
                  </InputAdornment>
                ),
              },
            }}
            value={searchTerm}
          />
          <Select
            onChange={(event) => setMethodFilter(event.target.value as HttpMethod | 'all')}
            size="small"
            sx={{ flexShrink: 0, minWidth: 140 }}
            value={methodFilter}
          >
            <MenuItem value="all">
              <FormattedMessage {...messages.allMethods} />
            </MenuItem>
            {HTTP_METHODS.map((value) => (
              <MenuItem key={value} value={value}>
                {value}
              </MenuItem>
            ))}
          </Select>
        </Stack>
      </Box>

      {/* Operations list */}
      <Box sx={{ flex: 1, minHeight: 0, overflowY: 'auto', px: 2, pb: 2 }}>
        {canParse ? (
          <SwaggerOperationsView
            onDelete={(index) => {
              const operation = filteredOperations[index];
              if (operation) {
                const fullIndex = operations.findIndex(
                  (op) =>
                    op.request.method === operation.request.method &&
                    op.request.path === operation.request.path,
                );
                if (fullIndex !== -1) {
                  onDelete(fullIndex);
                }
              }
            }}
            operations={filteredOperations}
            showDelete={showDelete}
          />
        ) : (
          <Typography color="text.secondary" variant="body2">
            {intl.formatMessage(messages.operationsParseError)}
          </Typography>
        )}
      </Box>
    </Box>
  );
}