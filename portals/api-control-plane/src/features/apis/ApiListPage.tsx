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
  Button,
  Chip,
  InputAdornment,
  PageContent,
  PageTitle,
  Stack,
  TextField,
  ToggleButton,
  ToggleButtonGroup,
  Typography,
} from '@wso2/oxygen-ui';
import {
  Boxes,
  LayoutGrid,
  List,
  Plus,
  Search,
} from '@wso2/oxygen-ui-icons-react';
import { useNavigate, useParams } from 'react-router-dom';

import { useApis, useDeleteApi } from '../../api/hooks/useMvpQueries';
import { ApiCardGrid } from '../../components/cards/ApiCardGrid';
import { filterApis, groupApisByKind } from '../../components/cards/apiDisplay';
import { ApiListView } from '../../components/cards/ApiListView';
import { ConfirmDialog } from '../../components/ConfirmDialog';
import { useNotifications } from '../../components/Notifications';
import {
  EmptyState,
  ErrorState,
  LoadingState,
} from '../../components/StateViews';
import { routes } from '../../routes/paths';
import type { Api } from '../../types/domain';

type ViewMode = 'grid' | 'list';

function ApiSection({
  title,
  icon,
  components,
  view,
  onOpen,
  onDelete,
}: {
  title: string;
  icon: React.ReactNode;
  components: Api[];
  view: ViewMode;
  onOpen: (component: Api) => void;
  onDelete: (component: Api) => void;
}) {
  if (components.length === 0) return null;
  return (
    <Stack spacing={1.5}>
      <Box sx={{ alignItems: 'center', display: 'flex', gap: 1 }}>
        <Box sx={{ color: 'text.secondary', display: 'inline-flex' }}>
          {icon}
        </Box>
        <Typography variant="h6">{title}</Typography>
        <Chip label={components.length} size="small" />
        <Box sx={{ bgcolor: 'divider', flex: 1, height: '1px', ml: 1 }} />
      </Box>
      {view === 'grid' ? (
        <ApiCardGrid
          components={components}
          onDelete={onDelete}
          onOpen={onOpen}
        />
      ) : (
        <ApiListView
          components={components}
          onDelete={onDelete}
          onOpen={onOpen}
        />
      )}
    </Stack>
  );
}

export function ApiListPage() {
  const { orgHandle = '', projectHandler = '' } = useParams();
  const navigate = useNavigate();
  const apisQuery = useApis();
  const deleteApiMutation = useDeleteApi();
  const { notify } = useNotifications();
  const [search, setSearch] = useState('');
  const [view, setView] = useState<ViewMode>('grid');
  const [toDelete, setToDelete] = useState<Api | null>(null);

  const confirmDelete = () => {
    if (!toDelete) return;
    deleteApiMutation.mutate(toDelete, {
      onSuccess: () => {
        notify(`Deleted "${toDelete.displayName}".`, 'success');
        setToDelete(null);
      },
      onError: (error) =>
        notify(
          error instanceof Error ? error.message : 'Delete failed',
          'error'
        ),
    });
  };

  const components = useMemo(() => apisQuery.data || [], [apisQuery.data]);
  const searched = useMemo(
    () => filterApis(components, search),
    [components, search]
  );
  const groups = useMemo(() => groupApisByKind(searched), [searched]);
  const matchCount = groups.apiProxies.length + groups.others.length;

  const openApi = (component: Api) =>
    navigate(routes.api(orgHandle, projectHandler, component.handler));
  const createApi = () => navigate(routes.newApi(orgHandle, projectHandler));

  if (apisQuery.isLoading) return <LoadingState label="Loading APIs" />;
  if (apisQuery.error) {
    return <ErrorState message="Unable to load APIs" />;
  }

  return (
    <PageContent fullWidth>
      <PageTitle>
        <PageTitle.Header>APIs</PageTitle.Header>
        <PageTitle.SubHeader>API proxies in this project.</PageTitle.SubHeader>
        <PageTitle.Actions>
          <Button onClick={createApi} startIcon={<Plus />} variant="contained">
            Create API
          </Button>
        </PageTitle.Actions>
      </PageTitle>

      {components.length === 0 ? (
        <EmptyState
          actionLabel="Create API"
          description="Create an API Proxy to get started."
          onAction={createApi}
          title="No APIs yet"
        />
      ) : (
        <Stack spacing={2}>
          <Box
            sx={{
              alignItems: 'center',
              display: 'flex',
              flexWrap: 'wrap',
              gap: 1.5,
            }}
          >
            <TextField
              onChange={(event) => setSearch(event.target.value)}
              placeholder="Search APIs"
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
              sx={{ flex: 1, maxWidth: 360, minWidth: 200 }}
              value={search}
            />
            <ToggleButtonGroup
              exclusive
              onChange={(_event, value: ViewMode | null) => {
                if (value) setView(value);
              }}
              size="small"
              sx={{ ml: 'auto' }}
              value={view}
            >
              <ToggleButton aria-label="Grid view" value="grid">
                <LayoutGrid size={16} />
              </ToggleButton>
              <ToggleButton aria-label="List view" value="list">
                <List size={16} />
              </ToggleButton>
            </ToggleButtonGroup>
          </Box>

          {matchCount === 0 ? (
            <EmptyState
              title="No matching APIs"
              description="Try a different API name or clear the filters."
            />
          ) : (
            <Stack spacing={4}>
              <ApiSection
                components={groups.apiProxies}
                icon={<Boxes size={16} />}
                onDelete={setToDelete}
                onOpen={openApi}
                title="API Proxies"
                view={view}
              />
              <ApiSection
                components={groups.others}
                icon={<Boxes size={16} />}
                onDelete={setToDelete}
                onOpen={openApi}
                title="Other APIs"
                view={view}
              />
            </Stack>
          )}
        </Stack>
      )}

      <ConfirmDialog
        confirmInputLabel={`Type "${toDelete?.displayName ?? ''}" to confirm`}
        confirmLabel="Delete"
        confirmPhrase={toDelete?.displayName ?? ''}
        destructive
        loading={deleteApiMutation.isPending}
        message={
          toDelete
            ? `This permanently deletes the API proxy "${toDelete.displayName}" ` +
              'and all related details. This action is irreversible.'
            : ''
        }
        onCancel={() => setToDelete(null)}
        onConfirm={confirmDelete}
        open={toDelete !== null}
        title="Delete API"
      />
    </PageContent>
  );
}
