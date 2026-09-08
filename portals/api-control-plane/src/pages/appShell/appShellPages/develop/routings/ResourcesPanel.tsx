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
  Box,
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  FormControl,
  FormLabel,
  InputAdornment,
  MenuItem,
  PageTitle,
  Select,
  Stack,
  TextField,
} from '@wso2/oxygen-ui';
import { Plus, Search } from '@wso2/oxygen-ui-icons-react';
import { useState } from 'react';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';

import { useUpdateRestApi, type Operation, type RestApi } from '@/api/resources/restApis';
import { useNotifications } from '@/components/Notifications';
import { SwaggerOperationsView } from '@/components/SwaggerOperationsView';
import { SaveBar } from '../SaveBar';
import { useDirtyTracking } from '../useDirtyTracking';

const METHODS: Operation['request']['method'][] = [
  'GET',
  'POST',
  'PUT',
  'DELETE',
  'PATCH',
  'HEAD',
  'OPTIONS',
];

const messages = defineMessages({
  title: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.routings.ResourcesPanel.title',
    defaultMessage: 'Resources',
  },
  description: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.routings.ResourcesPanel.description',
    defaultMessage: 'View and manage the methods, paths, and descriptions available in {apiName}.',
  },
  addResource: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.routings.ResourcesPanel.addResource',
    defaultMessage: 'Add resource',
  },
  dialogTitle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.routings.ResourcesPanel.dialogTitle',
    defaultMessage: 'Add new resource',
  },
  method: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.routings.ResourcesPanel.method',
    defaultMessage: 'Method',
  },
  path: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.routings.ResourcesPanel.path',
    defaultMessage: 'Resource path',
  },
  pathHelp: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.routings.ResourcesPanel.pathHelp',
    defaultMessage: 'The path must start with / and must be unique for the selected method.',
  },
  descriptionLabel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.routings.ResourcesPanel.descriptionLabel',
    defaultMessage: 'Description',
  },
  cancel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.routings.ResourcesPanel.cancel',
    defaultMessage: 'Cancel',
  },
  add: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.routings.ResourcesPanel.add',
    defaultMessage: 'Add',
  },
  saved: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.routings.ResourcesPanel.saved',
    defaultMessage: 'Resources saved.',
  },
  searchResources: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.routings.ResourcesPanel.searchResources',
    defaultMessage: 'Search resources',
  },
  allMethods: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.routings.ResourcesPanel.allMethods',
    defaultMessage: 'All methods',
  },
});

const operationName = (method: string, path: string) => {
  const suffix = path
    .split(/[^A-Za-z0-9]+/)
    .filter(Boolean)
    .map((part) => `${part.charAt(0).toUpperCase()}${part.slice(1)}`)
    .join('');
  return `${method.toLowerCase()}${suffix || 'Root'}`;
};

export function ResourcesPanel({ api }: { api: RestApi }) {
  const intl = useIntl();
  const { notify } = useNotifications();
  const update = useUpdateRestApi();
  const [operations, setOperations] = useState<Operation[]>(api.operations ?? []);
  const [deletedOperations, setDeletedOperations] = useState<Set<string>>(new Set());
  const [dialogOpen, setDialogOpen] = useState(false);
  const [method, setMethod] = useState<Operation['request']['method']>('GET');
  const [path, setPath] = useState('/');
  const [description, setDescription] = useState('');
  const [search, setSearch] = useState('');
  const [methodFilter, setMethodFilter] = useState('all');
  const deletedOperationKeys = [...deletedOperations].sort();
  const { dirty, markSaved } = useDirtyTracking({ deletedOperationKeys, operations });

  const operationKey = (operation: Operation) =>
    `${operation.request.method}:${operation.request.path}`;

  const trimmedPath = path.trim();
  const duplicate = operations.some(
    (operation) => operation.request.method === method && operation.request.path === trimmedPath,
  );
  const pathValid = trimmedPath.startsWith('/') && !duplicate;
  const searchTerm = search.trim().toLowerCase();
  const visibleOperations = operations.filter((operation) => {
    const matchesMethod = methodFilter === 'all' || operation.request.method === methodFilter;
    const matchesSearch =
      searchTerm === '' ||
      operation.request.path.toLowerCase().includes(searchTerm) ||
      operation.name?.toLowerCase().includes(searchTerm) ||
      operation.description?.toLowerCase().includes(searchTerm);
    return matchesMethod && matchesSearch;
  });

  const closeDialog = () => {
    setDialogOpen(false);
    setMethod('GET');
    setPath('/');
    setDescription('');
  };

  const addResource = () => {
    if (!pathValid) return;
    setOperations((current) => [
      {
        name: operationName(method, trimmedPath),
        ...(description.trim() && { description: description.trim() }),
        request: { method, path: trimmedPath },
      },
      ...current,
    ]);
    closeDialog();
  };

  const save = () => {
    if (!api.id) return;
    const savedOperations = operations.filter(
      (operation) => !deletedOperations.has(operationKey(operation)),
    );
    update.mutate(
      { restApiId: api.id, body: { ...api, operations: savedOperations } },
      {
        onSuccess: () => {
          setOperations(savedOperations);
          setDeletedOperations(new Set());
          markSaved({ deletedOperationKeys: [], operations: savedOperations });
          notify(intl.formatMessage(messages.saved), 'success');
        },
      },
    );
  };

  return (
    <>
      <Stack alignItems="flex-start" direction="row" justifyContent="space-between" spacing={2}>
        <PageTitle>
          <PageTitle.Header>
            <FormattedMessage {...messages.title} />
          </PageTitle.Header>
          <PageTitle.SubHeader>
            <FormattedMessage {...messages.description} values={{ apiName: api.displayName }} />
          </PageTitle.SubHeader>
        </PageTitle>
        <Button
          onClick={() => setDialogOpen(true)}
          startIcon={<Plus size={18} />}
          sx={{ flexShrink: 0, whiteSpace: 'nowrap' }}
          variant="outlined"
        >
          <FormattedMessage {...messages.addResource} />
        </Button>
      </Stack>

      <Stack alignItems="center" direction="row" spacing={1.5} sx={{ mt: 2 }}>
        <TextField
          fullWidth
          onChange={(event) => setSearch(event.target.value)}
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
          value={search}
        />
        <TextField
          onChange={(event) => setMethodFilter(event.target.value)}
          select
          size="small"
          sx={{ flexShrink: 0, minWidth: 180 }}
          value={methodFilter}
        >
          <MenuItem value="all">
            <FormattedMessage {...messages.allMethods} />
          </MenuItem>
          {METHODS.map((value) => (
            <MenuItem key={value} value={value}>
              {value}
            </MenuItem>
          ))}
        </TextField>
      </Stack>

      <Box
        sx={{
          maxHeight: { md: 'calc(100vh - 360px)', xs: '55vh' },
          mt: 1.5,
          overflowY: 'auto',
          pr: 0.5,
        }}
      >
        <SwaggerOperationsView
          isOperationDisabled={(operation) => deletedOperations.has(operationKey(operation))}
          onDelete={(index) =>
            setDeletedOperations((current) =>
              new Set(current).add(operationKey(visibleOperations[index])),
            )
          }
          operations={visibleOperations}
          showDelete
        />
      </Box>

      <SaveBar
        dirty={dirty}
        disabled={!api.id}
        onCancel={() => {
          setOperations(api.operations ?? []);
          setDeletedOperations(new Set());
        }}
        onSave={save}
        saving={update.isPending}
      />

      <Dialog fullWidth maxWidth="xs" onClose={closeDialog} open={dialogOpen}>
        <DialogTitle>
          <FormattedMessage {...messages.dialogTitle} />
        </DialogTitle>
        <DialogContent>
          <Stack spacing={2} sx={{ pt: 0.5 }}>
            <FormControl fullWidth>
              <FormLabel id="resource-method-label">
                <FormattedMessage {...messages.method} />
              </FormLabel>
              <Select
                labelId="resource-method-label"
                onChange={(event) =>
                  setMethod(event.target.value as Operation['request']['method'])
                }
                size="small"
                value={method}
              >
                {METHODS.map((value) => (
                  <MenuItem key={value} value={value}>
                    {value}
                  </MenuItem>
                ))}
              </Select>
            </FormControl>
            <FormControl fullWidth>
              <FormLabel htmlFor="resource-path">
                <FormattedMessage {...messages.path} />
              </FormLabel>
              <TextField
                error={!pathValid}
                fullWidth
                helperText={!pathValid ? intl.formatMessage(messages.pathHelp) : undefined}
                id="resource-path"
                onChange={(event) => setPath(event.target.value)}
                size="small"
                value={path}
              />
            </FormControl>
            <FormControl fullWidth>
              <FormLabel htmlFor="resource-description">
                <FormattedMessage {...messages.descriptionLabel} />
              </FormLabel>
              <TextField
                fullWidth
                id="resource-description"
                multiline
                onChange={(event) => setDescription(event.target.value)}
                rows={3}
                size="small"
                value={description}
              />
            </FormControl>
          </Stack>
        </DialogContent>
        <DialogActions>
          <Button color="secondary" onClick={closeDialog} variant="outlined">
            <FormattedMessage {...messages.cancel} />
          </Button>
          <Button disabled={!pathValid} onClick={addResource} variant="contained">
            <FormattedMessage {...messages.add} />
          </Button>
        </DialogActions>
      </Dialog>
    </>
  );
}
