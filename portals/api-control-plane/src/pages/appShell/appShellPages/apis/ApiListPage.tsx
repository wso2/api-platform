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
  InputAdornment,
  PageTitle,
  Stack,
  TextField,
  ToggleButton,
  ToggleButtonGroup,
} from '@wso2/oxygen-ui';
import { LayoutGrid, List, Plus, Search } from '@wso2/oxygen-ui-icons-react';
import { useNavigate, useParams } from 'react-router-dom';

import {
  useDeleteRestApi,
  useRestApis,
  type RestApi,
} from '../../../../api/resources/restApis';
import { ApiCardGrid } from './ApiCardGrid';
import { ApiListView } from './ApiListView';
import { filterRestApis } from './restApiDisplay';
import { ConfirmDialog } from '../../../../components/ConfirmDialog';
import { useNotifications } from '../../../../components/Notifications';
import {
  EmptyState,
  ErrorState,
  LoadingState,
} from '../../../../components/StateViews';
import { routes } from '../../../../routes/paths';
import { ScopeGate } from '../../../../scope/ScopeGate';
import { FormattedMessage } from 'react-intl';

type ViewMode = 'grid' | 'list';

export function ApiListPage() {
  // Gating the whole body, not just the JSX: out of project scope `useRestApis`
  // stays disabled and `isPending` never clears, so the loading branch below
  // would sit there forever instead of the scope prompt showing.
  return (
    <ScopeGate
      prompt="APIs are created and managed at the project level."
      requires="project"
      to={routes.apis}
    >
      <ApiList />
    </ScopeGate>
  );
}

function ApiList() {
  const { orgHandle = '', projectHandler = '' } = useParams();
  const navigate = useNavigate();
  const apisQuery = useRestApis();
  const deleteApiMutation = useDeleteRestApi();
  const { notify } = useNotifications();
  const [search, setSearch] = useState('');
  const [view, setView] = useState<ViewMode>('grid');
  const [toDelete, setToDelete] = useState<RestApi | null>(null);

  const apis = useMemo(() => apisQuery.data?.list ?? [], [apisQuery.data]);
  const searched = useMemo(() => filterRestApis(apis, search), [apis, search]);

  const confirmDelete = () => {
    if (!toDelete?.id) return;
    deleteApiMutation.mutate(
      { restApiId: toDelete.id },
      {
        onSuccess: () => {
          notify(`Deleted "${toDelete.displayName}".`, 'success');
          setToDelete(null);
        },
        onError: (error) => notify(error.message || 'Delete failed', 'error'),
      }
    );
  };

  const openApi = (api: RestApi) =>
    navigate(routes.api(orgHandle, projectHandler, api.id ?? ''));
  const createApi = () => navigate(routes.newApi(orgHandle, projectHandler));

  // `isPending` rather than `isLoading`: the query stays disabled until the
  // route's org/project resolve, and in that window `isLoading` is already
  // false with no data — which would flash the "No APIs yet" empty state.
  if (apisQuery.isPending) return <LoadingState label="Loading APIs" />;
  if (apisQuery.error) {
    return <ErrorState message="Unable to load APIs" />;
  }

  return (
    <>
      <PageTitle>
        <PageTitle.Header>
          <FormattedMessage
            id="apiListPage.title"
            defaultMessage="APIs"
            description="Page title for the API list page"
          />
        </PageTitle.Header>
        <PageTitle.SubHeader>
          <FormattedMessage
            id="apiListPage.subHeader"
            defaultMessage="REST APIs in this project."
            description="Sub header for the API list page"
          />
        </PageTitle.SubHeader>
        <PageTitle.Actions>
          <Button onClick={createApi} startIcon={<Plus />} variant="contained">
            <FormattedMessage
              id="apiListPage.createApiButton"
              defaultMessage="Create API"
              description="Button label for creating a new API"
            />
          </Button>
        </PageTitle.Actions>
      </PageTitle>

      {apis.length === 0 ? (
        <EmptyState
          actionLabel="Create API"
          description="Create a REST API to get started."
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

          {searched.length === 0 ? (
            <EmptyState
              title="No matching APIs"
              description="Try a different API name or clear the filters."
            />
          ) : view === 'grid' ? (
            <ApiCardGrid
              apis={searched}
              onDelete={setToDelete}
              onOpen={openApi}
            />
          ) : (
            <ApiListView
              apis={searched}
              onDelete={setToDelete}
              onOpen={openApi}
            />
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
            ? `This permanently deletes the API "${toDelete.displayName}" ` +
              'and all related details. This action is irreversible.'
            : ''
        }
        onCancel={() => setToDelete(null)}
        onConfirm={confirmDelete}
        open={toDelete !== null}
        title="Delete API"
      />
    </>
  );
}
