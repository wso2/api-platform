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

import { useState } from 'react';
import {
  Box,
  Button,
  Card,
  CardContent,
  Chip,
  Grid,
  IconButton,
  ListItemIcon,
  ListItemText,
  Menu,
  MenuItem,
  PageContent,
  PageTitle,
  Stack,
  Tooltip,
  Typography,
} from '@wso2/oxygen-ui';
import {
  Check,
  Copy,
  MoreVertical,
  Pencil,
  Trash2,
} from '@wso2/oxygen-ui-icons-react';
import { Link, useNavigate, useParams } from 'react-router-dom';

import { useApiPortal, useDeleteApiPortal } from '../../api/hooks/useMvpQueries';
import { ConfirmDialog } from '../../components/ConfirmDialog';
import { useNotifications } from '../../components/Notifications';
import { ErrorState, LoadingState } from '../../components/StateViews';
import { routes } from '../../routes/paths';
import type { ApiPortal } from '../../types/domain';
import { relativeTime } from '../../utils/relativeTime';
import {
  AUTH_LABEL,
  STATUS_CHIP_COLOR,
  STATUS_LABEL,
} from './apiPortalDisplay';

function CopyableInline({ value }: { value: string }) {
  const [copied, setCopied] = useState(false);
  const copy = async () => {
    try {
      await navigator.clipboard.writeText(value);
      setCopied(true);
      window.setTimeout(() => setCopied(false), 1400);
    } catch {
      /* clipboard blocked — swallow rather than surface an error toast */
    }
  };
  return (
    <Stack alignItems="center" direction="row" spacing={0.5}>
      <Typography sx={{ fontFamily: 'monospace', fontSize: 13.5 }}>
        {value}
      </Typography>
      <Tooltip title={copied ? 'Copied' : 'Copy'}>
        <IconButton aria-label="Copy value" onClick={copy} size="small">
          {copied ? <Check size={14} /> : <Copy size={14} />}
        </IconButton>
      </Tooltip>
    </Stack>
  );
}

function ReadOnlyRow({
  label,
  value,
  copyable,
  monospace,
}: {
  label: string;
  value: React.ReactNode;
  copyable?: string;
  monospace?: boolean;
}) {
  return (
    <Box>
      <Typography color="text.secondary" sx={{ fontSize: 12, mb: 0.5 }}>
        {label}
      </Typography>
      {copyable !== undefined ? (
        <CopyableInline value={copyable} />
      ) : monospace ? (
        <Typography sx={{ fontFamily: 'monospace', fontSize: 13.5 }}>
          {value}
        </Typography>
      ) : (
        <Typography sx={{ fontSize: 14 }}>{value}</Typography>
      )}
    </Box>
  );
}

export function ApiPortalDetailPage() {
  const { orgHandle = '', apiPortalId = '' } = useParams();
  const navigate = useNavigate();
  const { notify } = useNotifications();
  const apiPortalQuery = useApiPortal(orgHandle, apiPortalId);
  const deleteApiPortal = useDeleteApiPortal(orgHandle);

  const [menuAnchor, setMenuAnchor] = useState<HTMLElement | null>(null);
  const [confirmDelete, setConfirmDelete] = useState(false);

  if (apiPortalQuery.isLoading) {
    return <LoadingState label="Loading API Portal" />;
  }
  if (apiPortalQuery.error) {
    return <ErrorState message="Unable to load API Portal" />;
  }
  if (!apiPortalQuery.data) {
    return <ErrorState title="API Portal not found" />;
  }

  const apiPortal: ApiPortal = apiPortalQuery.data;
  const isOAuth2 = apiPortal.authType === 'oauth2';

  const goEdit = () =>
    navigate(routes.apiPortalEdit(orgHandle, apiPortal.id));

  const doDelete = () => {
    deleteApiPortal.mutate(apiPortal, {
      onSuccess: () => {
        notify(`Deleted "${apiPortal.name}".`, 'success');
        navigate(routes.apiPortal(orgHandle));
      },
      onError: (error) =>
        notify(
          error instanceof Error ? error.message : 'Delete failed',
          'error'
        ),
    });
  };

  return (
    <PageContent fullWidth>
      <PageTitle>
        <Link to={routes.apiPortal(orgHandle)}>
          <PageTitle.BackButton>Back to API Portal</PageTitle.BackButton>
        </Link>
        <PageTitle.Header>{apiPortal.name}</PageTitle.Header>
        <PageTitle.SubHeader>
          {apiPortal.url || apiPortal.handle}
        </PageTitle.SubHeader>
        <PageTitle.Actions>
          <Stack direction="row" spacing={1}>
            <Button
              onClick={goEdit}
              startIcon={<Pencil size={16} />}
              variant="contained"
            >
              Edit
            </Button>
            <IconButton
              aria-label="API Portal actions"
              onClick={(event) => setMenuAnchor(event.currentTarget)}
              size="small"
            >
              <MoreVertical size={18} />
            </IconButton>
            <Menu
              anchorEl={menuAnchor}
              onClose={() => setMenuAnchor(null)}
              open={Boolean(menuAnchor)}
            >
              <MenuItem
                onClick={() => {
                  setMenuAnchor(null);
                  setConfirmDelete(true);
                }}
                sx={{ color: 'error.main' }}
              >
                <ListItemIcon sx={{ color: 'inherit' }}>
                  <Trash2 size={16} />
                </ListItemIcon>
                <ListItemText>Delete</ListItemText>
              </MenuItem>
            </Menu>
          </Stack>
        </PageTitle.Actions>
      </PageTitle>

      <Grid container spacing={3}>
        {/* Overview */}
        <Grid size={{ xs: 12, md: 8 }}>
          <Card>
            <CardContent>
              <Typography sx={{ fontWeight: 700 }} variant="h6">
                Overview
              </Typography>
              <Typography color="text.secondary" sx={{ mt: 0.5 }} variant="body2">
                Read-only view of the registered API Portal. Use Edit to change
                any field.
              </Typography>

              <Stack spacing={2.5} sx={{ mt: 3 }}>
                <ReadOnlyRow
                  label="URL"
                  value={apiPortal.url || '—'}
                  copyable={apiPortal.url}
                />

                {apiPortal.description && (
                  <ReadOnlyRow
                    label="Description"
                    value={apiPortal.description}
                  />
                )}

                <ReadOnlyRow
                  label="Authentication"
                  value={
                    <Chip
                      color="primary"
                      label={AUTH_LABEL[apiPortal.authType]}
                      size="small"
                      variant="outlined"
                    />
                  }
                />

                {isOAuth2 ? (
                  <Box
                    sx={{
                      border: '1px solid',
                      borderColor: 'divider',
                      borderRadius: 2,
                      p: 2.25,
                    }}
                  >
                    <Typography
                      color="text.secondary"
                      sx={{
                        display: 'block',
                        fontWeight: 600,
                        letterSpacing: '.12em',
                        mb: 1.5,
                      }}
                      variant="caption"
                    >
                      IDP CLIENT CREDENTIALS
                    </Typography>
                    <Stack spacing={2}>
                      <ReadOnlyRow
                        label="STS token URL"
                        value={apiPortal.authConfig?.stsTokenUrl || '—'}
                        copyable={apiPortal.authConfig?.stsTokenUrl}
                      />
                      <ReadOnlyRow
                        label="Client ID"
                        value={apiPortal.authConfig?.clientId || '—'}
                        monospace
                      />
                      <ReadOnlyRow
                        label="Client secret"
                        value="•••••••••••• (never displayed after save)"
                      />
                    </Stack>
                  </Box>
                ) : (
                  <Typography color="text.secondary" variant="body2">
                    Platform API mints its own signed JWT with its configured
                    key to authenticate to this portal.
                  </Typography>
                )}
              </Stack>
            </CardContent>
          </Card>
        </Grid>

        {/* Details rail */}
        <Grid size={{ xs: 12, md: 4 }}>
          <Card sx={{ height: '100%' }}>
            <CardContent>
              <Typography sx={{ fontWeight: 700, mb: 1.5 }} variant="h6">
                Details
              </Typography>
              <Stack direction="row" sx={{ flexWrap: 'wrap', gap: 1, mb: 2.25 }}>
                <Chip
                  color={STATUS_CHIP_COLOR[apiPortal.workflowStatus]}
                  label={STATUS_LABEL[apiPortal.workflowStatus]}
                  size="small"
                />
              </Stack>
              <Stack spacing={1.75}>
                <ReadOnlyRow label="Identifier" value={apiPortal.handle} monospace />
                {apiPortal.createdAt && (
                  <ReadOnlyRow
                    label="Created"
                    value={relativeTime(apiPortal.createdAt)}
                  />
                )}
                {apiPortal.updatedAt && (
                  <ReadOnlyRow
                    label="Last updated"
                    value={relativeTime(apiPortal.updatedAt)}
                  />
                )}
              </Stack>
            </CardContent>
          </Card>
        </Grid>
      </Grid>

      <ConfirmDialog
        confirmInputLabel={`Type "${apiPortal.name}" to confirm`}
        confirmLabel="Delete"
        confirmPhrase={apiPortal.name}
        destructive
        loading={deleteApiPortal.isPending}
        message={`This permanently deletes the API Portal "${apiPortal.name}". This action is irreversible.`}
        onCancel={() => setConfirmDelete(false)}
        onConfirm={doDelete}
        open={confirmDelete}
        title="Delete API Portal"
      />
    </PageContent>
  );
}
