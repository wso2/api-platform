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
import { Link as RouterLink, useNavigate } from 'react-router-dom';
import {
  Avatar,
  Box,
  Button,
  Card,
  Chip,
  Dialog,
  DialogActions,
  DialogContent,
  DialogContentText,
  DialogTitle,
  Grid,
  IconButton,
  InputAdornment,
  PageContent,
  PageTitle,
  Stack,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  TextField,
  Tooltip,
  Typography,
  Skeleton,
} from '@wso2/oxygen-ui';
import { Plus, Search, Trash2, Edit } from '@wso2/oxygen-ui-icons-react';
import ErrorAlert from '../../../../Components/common/ErrorAlert';
import { useGatewayList } from '../../../../hooks/useGateway';
import { useAppShell } from '../../../../contexts/AppShellContext';
import { buildOrgPath } from '../../../../utils/projectRouting';
import { formatRelativeTime } from '../../../../contexts/llmProvider';
import { FormattedMessage, useIntl } from 'react-intl';
import NoGW from '../../../../assets/images/NoGW.svg';
import { useEnvironments } from '../../../../hooks/useEnvironments';
import { useRole } from '../../../../contexts/RoleContext';

const MAX_GATEWAYS_PER_ORG = 3;

function truncateText(text: string, maxLength: number): string {
  if (text.length <= maxLength) return text;
  return `${text.slice(0, maxLength).trim()}…`;
}

export default function GatewaysList() {
  const navigate = useNavigate();
  const intl = useIntl();
  const { currentOrganization } = useAppShell();
  const { role } = useRole();
  const isAdmin = role === 'admin';
  const [searchQuery, setSearchQuery] = useState('');
  const [deleteTarget, setDeleteTarget] = useState<{
    id: string;
    name: string;
  } | null>(null);

  const { gateways, isLoading, error, refetch, deleteGatewayById } =
    useGatewayList();

  // Prefetch environments so they're available when navigating to Add/Edit views
  useEnvironments();

  const newGatewayPath = buildOrgPath(currentOrganization, '/gateways/new');
  const aiGatewayCount = gateways.filter(
    (gateway) => gateway.functionalityType === 'ai'
  ).length;
  const apiGatewayCount = gateways.filter(
    (gateway) => gateway.functionalityType === 'regular'
  ).length;
  const isGatewayQuotaReached =
    aiGatewayCount + apiGatewayCount >= MAX_GATEWAYS_PER_ORG;
  const gatewayQuotaTooltip = `You cannot continue because your organization already has ${aiGatewayCount} AI gateway${
    aiGatewayCount === 1 ? '' : 's'
  } and ${apiGatewayCount} API gateway${
    apiGatewayCount === 1 ? '' : 's'
  }. The maximum limit is 3 gateways in total.`;

  // Filter only AI gateways
  const aiGateways = useMemo(() => {
    return gateways.filter((gw) => gw.functionalityType === 'ai');
  }, [gateways]);

  const filteredGateways = useMemo(() => {
    const query = searchQuery.trim().toLowerCase();
    if (!query) return aiGateways;

    return aiGateways.filter((gateway) => {
      const haystack = [
        gateway.name,
        gateway.displayName,
        gateway.description,
        gateway.vhost,
      ]
        .filter(Boolean)
        .join(' ')
        .toLowerCase();
      return haystack.includes(query);
    });
  }, [aiGateways, searchQuery]);

  const handleDeleteConfirm = () => {
    if (!deleteTarget) return;
    deleteGatewayById(deleteTarget.id);
    setDeleteTarget(null);
  };

  const getStatusChip = (gateway: any) => {
    const isActive = gateway.isActive;
    return (
      <Chip
        size="small"
        variant="outlined"
        label={
          isActive ? (
            <FormattedMessage
              id="aiWorkspace.pages.appShell.appShellPages.gateways.GatewaysList.status.active"
              defaultMessage="Active"
            />
          ) : (
            <FormattedMessage
              id="aiWorkspace.pages.appShell.appShellPages.gateways.GatewaysList.status.inactive"
              defaultMessage="Inactive"
            />
          )
        }
        color={isActive ? 'success' : 'error'}
      />
    );
  };

  return (
    <PageContent fullWidth>
      <Grid container spacing={2} sx={{ width: '100%', m: 0 }}>
        <Grid size={{ xs: 12 }}>
          <Box
            sx={{
              display: 'flex',
              alignItems: 'flex-start',
              justifyContent: 'space-between',
              flexWrap: 'nowrap',
              gap: 2,
            }}
          >
            <PageTitle sx={{ minWidth: 0, flex: 1 }}>
              <PageTitle.Header>
                <FormattedMessage
                  id="aiWorkspace.pages.appShell.appShellPages.gateways.GatewaysList.ai.gateways"
                  defaultMessage="AI Gateways"
                />
              </PageTitle.Header>
              <PageTitle.SubHeader>
                <FormattedMessage
                  id="aiWorkspace.pages.appShell.appShellPages.gateways.GatewaysList.manage.and.monitor.your.ai.gateway.deployments"
                  defaultMessage="Manage and monitor your AI gateway deployments."
                />
              </PageTitle.SubHeader>
            </PageTitle>

            <Stack
              direction="row"
              spacing={1.5}
              sx={{ ml: 'auto', flexShrink: 0 }}
            >
              {isAdmin && filteredGateways.length > 0 ? (
                <Tooltip
                  title={isGatewayQuotaReached ? gatewayQuotaTooltip : ''}
                  disableHoverListener={!isGatewayQuotaReached}
                >
                  <Box component="span">
                    <Button
                      variant="contained"
                      component={RouterLink}
                      to={newGatewayPath}
                      startIcon={<Plus size={20} />}
                      disabled={isGatewayQuotaReached}
                      sx={{
                        opacity: isGatewayQuotaReached ? 0.55 : 1,
                        '&.Mui-disabled': {
                          opacity: isGatewayQuotaReached ? 0.55 : 1,
                        },
                      }}
                    >
                      <FormattedMessage
                        id="aiWorkspace.pages.appShell.appShellPages.gateways.GatewaysList.add.ai.gateway"
                        defaultMessage="Add AI Gateway"
                      />
                    </Button>
                  </Box>
                </Tooltip>
              ) : null}
            </Stack>
          </Box>
        </Grid>

        {isLoading ? (
          <>
            <Grid size={{ xs: 12 }}>
              <TextField
                fullWidth
                placeholder={intl.formatMessage({
                  id: 'aiWorkspace.pages.appShell.appShellPages.gateways.GatewaysList.search.ai.gateways',
                  defaultMessage: 'Search AI Gateways...',
                })}
                value={searchQuery}
                disabled
                onChange={(event) => setSearchQuery(event.target.value)}
                slotProps={{
                  input: {
                    startAdornment: (
                      <InputAdornment position="start">
                        <Search size={20} />
                      </InputAdornment>
                    ),
                  },
                }}
              />
            </Grid>

            <Grid size={{ xs: 12 }}>
              <Card>
                <TableContainer>
                  <Table size="small">
                    <TableHead>
                      <TableRow>
                        <TableCell>
                          <FormattedMessage
                            id="aiWorkspace.pages.appShell.appShellPages.gateways.GatewaysList.table.name"
                            defaultMessage="Name"
                          />
                        </TableCell>
                        <TableCell>
                          <FormattedMessage
                            id="aiWorkspace.pages.appShell.appShellPages.gateways.GatewaysList.table.description"
                            defaultMessage="Description"
                          />
                        </TableCell>
                        <TableCell>
                          <FormattedMessage
                            id="aiWorkspace.pages.appShell.appShellPages.gateways.GatewaysList.table.status"
                            defaultMessage="Status"
                          />
                        </TableCell>
                        <TableCell>
                          <FormattedMessage
                            id={
                              isAdmin
                                ? 'aiWorkspace.pages.appShell.appShellPages.gateways.GatewaysList.table.last.updated'
                                : 'aiWorkspace.pages.appShell.appShellPages.gateways.GatewaysList.table.vhost'
                            }
                            defaultMessage={isAdmin ? 'Last Updated' : 'VHost'}
                          />
                        </TableCell>
                        {isAdmin ? (
                          <TableCell align="right">
                            <FormattedMessage
                              id="aiWorkspace.pages.appShell.appShellPages.gateways.GatewaysList.table.actions"
                              defaultMessage="Actions"
                            />
                          </TableCell>
                        ) : null}
                      </TableRow>
                    </TableHead>
                    <TableBody>
                      {[0, 1, 2].map((key) => (
                        <TableRow key={key}>
                          <TableCell sx={{ minWidth: 220 }}>
                            <Stack
                              direction="row"
                              spacing={1}
                              alignItems="center"
                            >
                              <Skeleton
                                variant="circular"
                                width={28}
                                height={28}
                              />
                              <Skeleton
                                variant="text"
                                width={130}
                                height={20}
                              />
                            </Stack>
                          </TableCell>
                          <TableCell>
                            <Skeleton variant="text" width="80%" height={20} />
                          </TableCell>
                          <TableCell>
                            <Skeleton
                              variant="rounded"
                              width={70}
                              height={24}
                            />
                          </TableCell>
                          <TableCell>
                            <Skeleton variant="text" width={90} height={20} />
                          </TableCell>
                          {isAdmin ? (
                            <TableCell align="right">
                              <Skeleton
                                variant="rounded"
                                width={52}
                                height={24}
                                sx={{ ml: 'auto' }}
                              />
                            </TableCell>
                          ) : null}
                        </TableRow>
                      ))}
                    </TableBody>
                  </Table>
                </TableContainer>
              </Card>
            </Grid>
          </>
        ) : error ? (
          <Grid size={{ xs: 12 }}>
            <ErrorAlert error={error} onRetry={refetch} />
          </Grid>
        ) : filteredGateways.length === 0 ? (
          <Grid size={{ xs: 12 }}>
            <Box
              sx={{
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                py: 6,
              }}
            >
              <Stack
                spacing={1.5}
                alignItems="center"
                justifyContent="center"
                sx={{ textAlign: 'center' }}
              >
                <Box
                  component="img"
                  src={NoGW}
                  alt="No gateways"
                  sx={{ width: 200, maxWidth: '80%' }}
                />
                <Typography variant="body1" color="text.secondary">
                  No available AI gateway
                </Typography>
                {!isAdmin ? (
                  <Typography variant="body2" color="text.secondary">
                    <FormattedMessage
                      id="aiWorkspace.pages.appShell.appShellPages.gateways.GatewaysList.only.admin.can.add"
                      defaultMessage="Only organization admins can add AI Gateways. Please contact your administrator."
                    />
                  </Typography>
                ) : null}
                {isAdmin ? (
                  <Tooltip
                    title={isGatewayQuotaReached ? gatewayQuotaTooltip : ''}
                    disableHoverListener={!isGatewayQuotaReached}
                  >
                    <Box component="span">
                      <Button
                        variant="contained"
                        component={RouterLink}
                        to={newGatewayPath}
                        startIcon={<Plus size={20} />}
                        disabled={isGatewayQuotaReached}
                        sx={{
                          opacity: isGatewayQuotaReached ? 0.55 : 1,
                          '&.Mui-disabled': {
                            opacity: isGatewayQuotaReached ? 0.55 : 1,
                          },
                        }}
                      >
                        <FormattedMessage
                          id="aiWorkspace.pages.appShell.appShellPages.gateways.GatewaysList.add.ai.gateway"
                          defaultMessage="Add AI Gateway"
                        />
                      </Button>
                    </Box>
                  </Tooltip>
                ) : null}
              </Stack>
            </Box>
          </Grid>
        ) : (
          <>
            <Grid size={{ xs: 12 }}>
              <TextField
                fullWidth
                placeholder={intl.formatMessage({
                  id: 'aiWorkspace.pages.appShell.appShellPages.gateways.GatewaysList.search.ai.gateways',
                  defaultMessage: 'Search AI Gateways...',
                })}
                value={searchQuery}
                onChange={(event) => setSearchQuery(event.target.value)}
                slotProps={{
                  input: {
                    startAdornment: (
                      <InputAdornment position="start">
                        <Search size={20} />
                      </InputAdornment>
                    ),
                  },
                }}
              />
            </Grid>

            <Grid size={{ xs: 12 }}>
              <Card>
                <TableContainer>
                  <Table size="small">
                    <TableHead>
                      <TableRow>
                        <TableCell>
                          <FormattedMessage
                            id="aiWorkspace.pages.appShell.appShellPages.gateways.GatewaysList.table.name"
                            defaultMessage="Name"
                          />
                        </TableCell>
                        <TableCell>
                          <FormattedMessage
                            id="aiWorkspace.pages.appShell.appShellPages.gateways.GatewaysList.table.description"
                            defaultMessage="Description"
                          />
                        </TableCell>
                        <TableCell>
                          <FormattedMessage
                            id="aiWorkspace.pages.appShell.appShellPages.gateways.GatewaysList.table.status"
                            defaultMessage="Status"
                          />
                        </TableCell>
                        <TableCell>
                          <FormattedMessage
                            id={
                              isAdmin
                                ? 'aiWorkspace.pages.appShell.appShellPages.gateways.GatewaysList.table.last.updated'
                                : 'aiWorkspace.pages.appShell.appShellPages.gateways.GatewaysList.table.vhost'
                            }
                            defaultMessage={isAdmin ? 'Last Updated' : 'VHost'}
                          />
                        </TableCell>
                        {isAdmin ? (
                          <TableCell align="right">
                            <FormattedMessage
                              id="aiWorkspace.pages.appShell.appShellPages.gateways.GatewaysList.table.actions"
                              defaultMessage="Actions"
                            />
                          </TableCell>
                        ) : null}
                      </TableRow>
                    </TableHead>
                    <TableBody>
                      {filteredGateways.length === 0 ? (
                        <TableRow>
                          <TableCell colSpan={isAdmin ? 5 : 4}>
                            <Typography variant="body2" color="text.secondary">
                              <FormattedMessage
                                id="aiWorkspace.pages.appShell.appShellPages.gateways.GatewaysList.no.ai.gateways.found"
                                defaultMessage="No AI Gateways found."
                              />
                            </Typography>
                          </TableCell>
                        </TableRow>
                      ) : (
                        filteredGateways.map((gateway) => (
                          <TableRow
                            key={gateway.id}
                            hover={isAdmin}
                            sx={{ cursor: isAdmin ? 'pointer' : 'default' }}
                            onClick={
                              isAdmin
                                ? () =>
                                    navigate(
                                      buildOrgPath(
                                        currentOrganization,
                                        `/gateways/view/${gateway.name}`
                                      )
                                    )
                                : undefined
                            }
                          >
                            <TableCell sx={{ minWidth: 220 }}>
                              <Box
                                sx={{
                                  display: 'flex',
                                  alignItems: 'center',
                                  gap: 1,
                                }}
                              >
                                <Avatar
                                  color="secondary"
                                  sx={{
                                    width: 36,
                                    height: 36,
                                    backgroundColor: 'primary.light',
                                    color: 'primary.contrastText',
                                    fontSize: 16,
                                  }}
                                >
                                  {(gateway.displayName || gateway.name || '—')
                                    .trim()
                                    .slice(0, 2)
                                    .toUpperCase()}
                                </Avatar>
                                <Box>
                                  <Typography
                                    variant="h6"
                                    sx={{ fontWeight: 600 }}
                                  >
                                    {truncateText(
                                      gateway.displayName || gateway.name,
                                      25
                                    )}
                                  </Typography>
                                </Box>
                              </Box>
                            </TableCell>
                            <TableCell>
                              <Typography
                                variant="body2"
                                color="text.secondary"
                                sx={{
                                  overflow: 'hidden',
                                  textOverflow: 'ellipsis',
                                  whiteSpace: 'nowrap',
                                  maxWidth: 300,
                                }}
                              >
                                {gateway.description || '—'}
                              </Typography>
                            </TableCell>
                            <TableCell>{getStatusChip(gateway)}</TableCell>
                            <TableCell>
                              {isAdmin ? (
                                gateway.createdAt ? (
                                  formatRelativeTime(gateway.createdAt)
                                ) : (
                                  '—'
                                )
                              ) : (
                                <Typography
                                  variant="body2"
                                  color="text.secondary"
                                  sx={{
                                    overflow: 'hidden',
                                    textOverflow: 'ellipsis',
                                    whiteSpace: 'nowrap',
                                    maxWidth: 280,
                                  }}
                                >
                                  {gateway.vhost || '—'}
                                </Typography>
                              )}
                            </TableCell>
                            {isAdmin ? (
                              <TableCell align="right">
                                <IconButton
                                  size="small"
                                  onClick={(event) => {
                                    event.stopPropagation();
                                    navigate(
                                      buildOrgPath(
                                        currentOrganization,
                                        `/gateways/edit/${gateway.name}`
                                      )
                                    );
                                  }}
                                  aria-label={intl.formatMessage(
                                    {
                                      id: 'aiWorkspace.pages.appShell.appShellPages.gateways.GatewaysList.edit.gateway',
                                      defaultMessage: 'Edit {name}',
                                    },
                                    {
                                      name: gateway.displayName || gateway.name,
                                    }
                                  )}
                                >
                                  <Edit size={16} />
                                </IconButton>
                                <IconButton
                                  size="small"
                                  color="error"
                                  onClick={(event) => {
                                    event.stopPropagation();
                                    setDeleteTarget({
                                      id: gateway.id,
                                      name: gateway.displayName || gateway.name,
                                    });
                                  }}
                                  aria-label={intl.formatMessage(
                                    {
                                      id: 'aiWorkspace.pages.appShell.appShellPages.gateways.GatewaysList.delete.gateway',
                                      defaultMessage: 'Delete {name}',
                                    },
                                    {
                                      name: gateway.displayName || gateway.name,
                                    }
                                  )}
                                >
                                  <Trash2 size={16} />
                                </IconButton>
                              </TableCell>
                            ) : null}
                          </TableRow>
                        ))
                      )}
                    </TableBody>
                  </Table>
                </TableContainer>
              </Card>
            </Grid>
          </>
        )}
      </Grid>

      <Dialog
        open={Boolean(deleteTarget)}
        onClose={() => setDeleteTarget(null)}
      >
        <DialogTitle>
          <FormattedMessage
            id="aiWorkspace.pages.appShell.appShellPages.gateways.GatewaysList.delete.ai.gateway"
            defaultMessage="Delete AI Gateway"
          />
        </DialogTitle>
        <DialogContent>
          <DialogContentText>
            <FormattedMessage
              id="aiWorkspace.pages.appShell.appShellPages.gateways.GatewaysList.are.you.sure.you.want.to.delete"
              defaultMessage="Are you sure you want to delete {name}?"
              values={{ name: deleteTarget?.name }}
            />
          </DialogContentText>
        </DialogContent>
        <DialogActions>
          <Button
            onClick={() => setDeleteTarget(null)}
            variant="outlined"
            color="secondary"
          >
            <FormattedMessage
              id="aiWorkspace.pages.appShell.appShellPages.gateways.GatewaysList.cancel"
              defaultMessage="Cancel"
            />
          </Button>
          <Button color="error" onClick={handleDeleteConfirm}>
            <FormattedMessage
              id="aiWorkspace.pages.appShell.appShellPages.gateways.GatewaysList.delete"
              defaultMessage="Delete"
            />
          </Button>
        </DialogActions>
      </Dialog>
    </PageContent>
  );
}
