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

import { useMemo } from 'react';
import {
  Accordion,
  AccordionDetails,
  AccordionSummary,
  Box,
  Card,
  CardContent,
  Chip,
  Divider,
  InputAdornment,
  List,
  ListItemButton,
  ListItemText,
  ListSubheader,
  Stack,
  TextField,
  Typography,
} from '@wso2/oxygen-ui';
import { ChevronDown, Search } from '@wso2/oxygen-ui-icons-react';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';

import { SegmentedControl } from '@/components/common/SegmentedControl';
import { SecondaryText } from '@/components/common/SecondaryText';
import { MethodBadge } from '@/components/SwaggerOperationsView/MethodBadge';
import type { ConsoleOperation, ConsoleSchemaDoc } from '../types';

const messages = defineMessages({
  all: {
    id: 'apiControlPlane.components.ApiTestConsole.OperationSidebar.all',
    defaultMessage: 'All',
    description: 'Method filter option that shows every operation.',
  },
  endpointCount: {
    id: 'apiControlPlane.components.ApiTestConsole.OperationSidebar.endpointCount',
    defaultMessage: '{count, plural, one {# endpoint} other {# endpoints}}',
  },
  fieldCount: {
    id: 'apiControlPlane.components.ApiTestConsole.OperationSidebar.fieldCount',
    defaultMessage: '{count, plural, one {# field} other {# fields}}',
  },
  lastResponse: {
    id: 'apiControlPlane.components.ApiTestConsole.OperationSidebar.lastResponse',
    defaultMessage: 'Last response: {status}',
    description: 'Accessible description of the chip showing the most recent response status.',
  },
  methodFilter: {
    id: 'apiControlPlane.components.ApiTestConsole.OperationSidebar.methodFilter',
    defaultMessage: 'Filter by method',
    description: 'Accessible label for the HTTP method filter above the operation list.',
  },
  noMatches: {
    id: 'apiControlPlane.components.ApiTestConsole.OperationSidebar.noMatches',
    defaultMessage: 'No endpoints match.',
  },
  schemas: {
    id: 'apiControlPlane.components.ApiTestConsole.OperationSidebar.schemas',
    defaultMessage: 'Schemas',
    description: 'Heading of the list of data models the API defines.',
  },
  searchLabel: {
    id: 'apiControlPlane.components.ApiTestConsole.OperationSidebar.searchLabel',
    defaultMessage: 'Search endpoints',
    description: 'Accessible label and placeholder of the endpoint search box.',
  },
});

export type MethodFilter = 'all' | string;

const statusColor = (status: number): 'success' | 'warning' | 'error' =>
  status < 300 ? 'success' : status < 500 ? 'warning' : 'error';

/** Whether an operation matches the typed words. Every word must match. */
export const operationMatches = (operation: ConsoleOperation, search: string): boolean => {
  const terms = search.trim().toLowerCase().split(/\s+/).filter(Boolean);
  if (terms.length === 0) return true;
  const haystack = `${operation.path} ${operation.summary}`.toLowerCase();
  return terms.every((term) => haystack.includes(term));
};

export type OperationSidebarProps = {
  operations: ConsoleOperation[];
  schemas: ConsoleSchemaDoc[];
  selectedKey: string | undefined;
  selectedSchema: string | undefined;
  search: string;
  method: MethodFilter;
  /** Last response status per operation key. */
  lastStatus: Record<string, number | undefined>;
  onSearch: (value: string) => void;
  onMethod: (method: MethodFilter) => void;
  onSelectOperation: (key: string) => void;
  onSelectSchema: (name: string) => void;
};

export function OperationSidebar({
  operations,
  schemas,
  selectedKey,
  selectedSchema,
  search,
  method,
  lastStatus,
  onSearch,
  onMethod,
  onSelectOperation,
  onSelectSchema,
}: OperationSidebarProps) {
  const intl = useIntl();

  const methods = useMemo(() => Array.from(new Set(operations.map((o) => o.method))), [operations]);
  const visible = useMemo(
    () =>
      operations.filter(
        (o) => (method === 'all' || o.method === method) && operationMatches(o, search),
      ),
    [method, operations, search],
  );
  const groups = useMemo(() => {
    const byGroup = new Map<string, ConsoleOperation[]>();
    visible.forEach((operation) =>
      byGroup.set(operation.group, [...(byGroup.get(operation.group) ?? []), operation]),
    );
    return Array.from(byGroup.entries());
  }, [visible]);

  return (
    <Card component="aside" variant="outlined">
      <CardContent>
        <Stack spacing={1.5}>
          <TextField
            fullWidth
            onChange={(event) => onSearch(event.target.value)}
            placeholder={intl.formatMessage(messages.searchLabel)}
            size="small"
            slotProps={{
              htmlInput: { 'aria-label': intl.formatMessage(messages.searchLabel) },
              input: {
                startAdornment: (
                  <InputAdornment position="start">
                    <Search size={16} />
                  </InputAdornment>
                ),
              },
            }}
            value={search}
          />
          <SegmentedControl
            aria-label={intl.formatMessage(messages.methodFilter)}
            onChange={onMethod}
            options={[
              { label: <FormattedMessage {...messages.all} />, value: 'all' },
              ...methods.map((value) => ({ label: value, value })),
            ]}
            sx={{ '& .MuiToggleButton-root': { flex: 1, px: 1 } }}
            value={method}
          />
        </Stack>
      </CardContent>

      {groups.length === 0 && (
        <CardContent>
          <SecondaryText variant="body2">
            <FormattedMessage {...messages.noMatches} />
          </SecondaryText>
        </CardContent>
      )}

      {groups.map(([group, items]) => (
        <Box key={group}>
          <Divider />
          <List
            dense
            sx={{ px: 1, pb: 1 }}
            subheader={
              <ListSubheader disableSticky sx={{ px: 1 }}>
                <Stack alignItems="center" direction="row" justifyContent="space-between">
                  <span>{group}</span>
                  <SecondaryText variant="caption">
                    <FormattedMessage
                      {...messages.endpointCount}
                      values={{ count: items.length }}
                    />
                  </SecondaryText>
                </Stack>
              </ListSubheader>
            }
          >
            {items.map((operation) => {
              const status = lastStatus[operation.key];
              return (
                <ListItemButton
                  key={operation.key}
                  onClick={() => onSelectOperation(operation.key)}
                  selected={operation.key === selectedKey && !selectedSchema}
                  sx={{ borderRadius: 1, gap: 1.5, px: 1.5 }}
                >
                  <MethodBadge method={operation.method} />
                  <ListItemText
                    primary={operation.summary}
                    secondary={operation.path}
                    slotProps={{ primary: { noWrap: true }, secondary: { noWrap: true } }}
                  />
                  {status !== undefined && (
                    <Chip
                      aria-label={intl.formatMessage(messages.lastResponse, { status })}
                      color={statusColor(status)}
                      label={status}
                      size="small"
                      sx={{ height: 20, '& .MuiChip-label': { fontSize: 11, px: 1 } }}
                      variant="outlined"
                    />
                  )}
                </ListItemButton>
              );
            })}
          </List>
        </Box>
      ))}

      {schemas.length > 0 && (
        <Box>
          <Divider />
          <Accordion disableGutters elevation={0} sx={{ '&&': { border: 0, borderRadius: 0 } }}>
            <AccordionSummary expandIcon={<ChevronDown size={16} />}>
              <Stack
                alignItems="center"
                direction="row"
                justifyContent="space-between"
                sx={{ pr: 1, width: '100%' }}
              >
                <Typography variant="subtitle2">
                  <FormattedMessage {...messages.schemas} />
                </Typography>
                <SecondaryText variant="caption">{schemas.length}</SecondaryText>
              </Stack>
            </AccordionSummary>
            <AccordionDetails sx={{ p: 0 }}>
              <List dense sx={{ px: 1, pb: 1 }}>
                {schemas.map((schema) => (
                  <ListItemButton
                    key={schema.name}
                    onClick={() => onSelectSchema(schema.name)}
                    selected={selectedSchema === schema.name}
                    sx={{ borderRadius: 1, px: 1.5 }}
                  >
                    <ListItemText primary={schema.name} slotProps={{ primary: { noWrap: true } }} />
                    <SecondaryText variant="caption">
                      <FormattedMessage
                        {...messages.fieldCount}
                        values={{ count: schema.fields.length }}
                      />
                    </SecondaryText>
                  </ListItemButton>
                ))}
              </List>
            </AccordionDetails>
          </Accordion>
        </Box>
      )}
    </Card>
  );
}
