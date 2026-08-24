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

import type { ElementType } from 'react';
import { useCallback, useEffect, useMemo, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import {
  Avatar,
  Box,
  Card,
  CircularProgress,
  Divider,
  IconButton,
  InputAdornment,
  List,
  ListItemButton,
  PageContent,
  Stack,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TablePagination,
  TableRow,
  TableSortLabel,
  TextField,
  Tooltip,
  Typography,
  Dialog,
  DialogActions,
  DialogContent,
  DialogContentText,
  DialogTitle,
  Button,
} from '@wso2/oxygen-ui';
import {
  ChevronRight,
  Search,
  ShieldCheck,
  Trash2,
} from '@wso2/oxygen-ui-icons-react';
import { FormattedMessage, useIntl } from 'react-intl';
import ErrorAlert from '../../../../Components/common/ErrorAlert';
import {
  deleteGatewayCustomPolicy,
  getGatewayCustomPolicies,
  getGatewayCustomPolicy,
} from '../../../../apis/gatewayPolicyApis';
import type { GatewayCustomPolicy } from '../../../../apis/gatewayPolicyApis';
import { getLLMProviders } from '../../../../apis/llmProviderApis';
import type { LLMProvider } from '../../../../utils/types';
import { useAIWorkspaceSnackbar } from '../../../../hooks/aiWorkspaceSnackbar';
import { getErrorCode, getErrorMessage } from '../../../../utils/apiError';
import { useAppAuth } from '../../../../contexts/AppAuthContext';
import { useAppShell } from '../../../../contexts/AppShellContext';
import { buildOrgPath } from '../../../../utils/projectRouting';
import { PLATFORM_API_BASE_URL } from '../../../../paths';
import { SCOPES } from '../../../../auth/permissions';

const ROWS_PER_PAGE_OPTIONS = [10, 25, 50];

type SortableField = 'name' | 'version' | 'createdAt' | 'updatedAt';

const tableHeaders: {
  key: string;
  label: React.ReactNode;
  sortable?: boolean;
}[] = [
  {
    key: 'name',
    sortable: true,
    label: (
      <FormattedMessage
        id="aiWorkspace.pages.appShell.appShellPages.gateways.CustomPoliciesList.header.name"
        defaultMessage="Name"
      />
    ),
  },
  {
    key: 'version',
    sortable: true,
    label: (
      <FormattedMessage
        id="aiWorkspace.pages.appShell.appShellPages.gateways.CustomPoliciesList.header.version"
        defaultMessage="Version"
      />
    ),
  },
  {
    key: 'description',
    label: (
      <FormattedMessage
        id="aiWorkspace.pages.appShell.appShellPages.gateways.CustomPoliciesList.header.description"
        defaultMessage="Description"
      />
    ),
  },
  {
    key: 'createdAt',
    sortable: true,
    label: (
      <FormattedMessage
        id="aiWorkspace.pages.appShell.appShellPages.gateways.CustomPoliciesList.header.createdAt"
        defaultMessage="Created At"
      />
    ),
  },
  {
    key: 'updatedAt',
    sortable: true,
    label: (
      <FormattedMessage
        id="aiWorkspace.pages.appShell.appShellPages.gateways.CustomPoliciesList.header.updatedAt"
        defaultMessage="Updated At"
      />
    ),
  },
];

function compareValues(a: string, b: string, order: 'asc' | 'desc'): number {
  const result = a.localeCompare(b);
  return order === 'asc' ? result : -result;
}

function getSortValue(
  policy: GatewayCustomPolicy,
  field: SortableField
): string {
  return (policy[field] ?? '').toString();
}

function formatVersion(version: string): string {
  return `v${version.replace(/^v/i, '')}`;
}

function getInitials(name: string): string {
  const words = name.trim().split(/\s+/);
  if (words.length === 0) return '';
  if (words.length === 1) return words[0].slice(0, 2).toUpperCase();
  return `${words[0][0]}${words[1][0]}`.toUpperCase();
}

interface PolicyInUseDialogProps {
  open: boolean;
  gatewayCustomPolicyId: string;
  version: string;
  fallbackName?: string;
  onClose: () => void;
}

function PolicyInUseDialog({
  open,
  gatewayCustomPolicyId,
  version,
  fallbackName,
  onClose,
}: PolicyInUseDialogProps) {
  const navigate = useNavigate();
  const { currentOrganization } = useAppShell();
  const organizationId = currentOrganization?.uuid;

  const [policy, setPolicy] = useState<GatewayCustomPolicy | null>(null);
  const [usedByProviders, setUsedByProviders] = useState<LLMProvider[]>([]);
  const [totalProviderCount, setTotalProviderCount] = useState(0);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!open || !gatewayCustomPolicyId || !version || !organizationId) return;

    let isMounted = true;
    setIsLoading(true);
    setError(null);
    setPolicy(null);
    setUsedByProviders([]);
    setTotalProviderCount(0);

    Promise.allSettled([
      getGatewayCustomPolicy(gatewayCustomPolicyId, version),
      getLLMProviders(organizationId, PLATFORM_API_BASE_URL, gatewayCustomPolicyId),
    ])
      .then(([policyResult, providersResult]) => {
        if (!isMounted) return;
        if (policyResult.status === 'fulfilled') {
          setPolicy(policyResult.value);
        }
        if (providersResult.status === 'fulfilled') {
          const list = providersResult.value.list ?? [];
          setUsedByProviders(list);
          setTotalProviderCount(providersResult.value.pagination?.total ?? list.length);
        } else {
          setError(getErrorMessage(providersResult.reason, 'Failed to load policy usage.'));
        }
      })
      .finally(() => {
        if (isMounted) setIsLoading(false);
      });

    return () => {
      isMounted = false;
    };
  }, [open, gatewayCustomPolicyId, version, organizationId]);

  const handleProviderClick = (providerId: string) => {
    navigate(buildOrgPath(currentOrganization, `/service-provider/${providerId}`));
    onClose();
  };

  const displayName = policy?.displayName || policy?.name || fallbackName;

  return (
    <Dialog open={open} onClose={onClose} maxWidth="xs" fullWidth>
      <DialogTitle>
        <FormattedMessage
          id="aiWorkspace.pages.appShell.appShellPages.gateways.CustomPoliciesList.usageDialog.title"
          defaultMessage="Custom Policy In Use"
        />
      </DialogTitle>
      <DialogContent dividers>
        <Stack spacing={0.5} sx={{ mb: 2 }}>
          <Typography variant="body1" sx={{ fontWeight: 600 }} title={displayName}>
            {displayName}
          </Typography>
          {policy?.version ? (
            <Typography variant="body2" color="text.secondary">
              {formatVersion(policy.version)}
            </Typography>
          ) : null}
          {policy?.description ? (
            <Typography variant="body2" color="text.secondary">
              {policy.description}
            </Typography>
          ) : null}
        </Stack>

        <Divider sx={{ mb: 2 }} />

        {isLoading ? (
          <Box sx={{ display: 'flex', justifyContent: 'center', py: 3 }}>
            <CircularProgress size={24} />
          </Box>
        ) : error ? (
          <Typography variant="body2" color="error" sx={{ py: 1 }}>
            {error}
          </Typography>
        ) : usedByProviders.length === 0 ? (
          <Typography variant="body2" color="text.secondary" sx={{ py: 1 }}>
            <FormattedMessage
              id="aiWorkspace.pages.appShell.appShellPages.gateways.CustomPoliciesList.usageDialog.usage.unavailable"
              defaultMessage="This policy is currently in use, but its usage details could not be determined."
            />
          </Typography>
        ) : (
          <>
            <Typography variant="body2" color="text.secondary" sx={{ mb: 1.5 }}>
              <FormattedMessage
                id="aiWorkspace.pages.appShell.appShellPages.gateways.CustomPoliciesList.usageDialog.usage.description"
                defaultMessage="{count, plural, one {# LLM Provider is} other {# LLM Providers are}} using this policy. Remove it from the provider(s) below before deleting it."
                values={{ count: totalProviderCount }}
              />
            </Typography>
            {totalProviderCount > usedByProviders.length ? (
              <Typography variant="body2" color="text.secondary" sx={{ mb: 1.5 }}>
                <FormattedMessage
                  id="aiWorkspace.pages.appShell.appShellPages.gateways.CustomPoliciesList.usageDialog.usage.partial"
                  defaultMessage="Showing {shown} of {total}. There may be more providers blocking this deletion than listed here."
                  values={{ shown: usedByProviders.length, total: totalProviderCount }}
                />
              </Typography>
            ) : null}
            <List disablePadding>
              {usedByProviders.map((provider) => {
                const providerId = provider.id ?? provider.displayName;
                return (
                  <ListItemButton
                    key={providerId}
                    onClick={() => handleProviderClick(providerId)}
                    sx={{
                      borderRadius: 1,
                      mb: 0.5,
                      border: 1,
                      borderColor: 'divider',
                    }}
                  >
                    <Stack
                      direction="row"
                      spacing={1.5}
                      alignItems="center"
                      sx={{ flexGrow: 1, minWidth: 0 }}
                    >
                      <Avatar
                        sx={{
                          width: 32,
                          height: 32,
                          fontSize: '0.75rem',
                          fontWeight: 600,
                          bgcolor: 'primary.light',
                          color: 'primary.contrastText',
                        }}
                      >
                        {getInitials(provider.displayName)}
                      </Avatar>
                      <Typography
                        variant="body2"
                        sx={{ fontWeight: 600 }}
                        noWrap
                        title={provider.displayName}
                      >
                        {provider.displayName}
                      </Typography>
                    </Stack>
                    <ChevronRight size={16} />
                  </ListItemButton>
                );
              })}
            </List>
          </>
        )}
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose} variant="outlined" color="secondary">
          <FormattedMessage
            id="aiWorkspace.pages.appShell.appShellPages.gateways.CustomPoliciesList.usageDialog.close"
            defaultMessage="Close"
          />
        </Button>
      </DialogActions>
    </Dialog>
  );
}

interface CustomPoliciesListProps {
  // When true, renders bare (no PageContent padding) for embedding inside a
  // page that already provides its own PageContent — e.g. GatewaysList.
  embedded?: boolean;
}

export default function CustomPoliciesList({
  embedded = false,
}: CustomPoliciesListProps) {
  const intl = useIntl();
  const showSnackbar = useAIWorkspaceSnackbar();
  const { hasPermission } = useAppAuth();
  const canDelete = hasPermission(SCOPES.GATEWAY_CUSTOM_POLICY_DELETE);

  const [policies, setPolicies] = useState<GatewayCustomPolicy[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<Error | null>(null);

  const [searchQuery, setSearchQuery] = useState('');
  const [sortedColumn, setSortedColumn] = useState<SortableField>('name');
  const [sortOrder, setSortOrder] = useState<'asc' | 'desc'>('asc');
  const [page, setPage] = useState(0);
  const [rowsPerPage, setRowsPerPage] = useState(ROWS_PER_PAGE_OPTIONS[0]);

  const [deleteTarget, setDeleteTarget] = useState<{
    uuid: string;
    version: string;
    name: string;
  } | null>(null);
  const [isDeleting, setIsDeleting] = useState(false);
  const [usageDialogTarget, setUsageDialogTarget] = useState<{
    uuid: string;
    version: string;
    name: string;
  } | null>(null);

  const fetchPolicies = useCallback(async () => {
    setIsLoading(true);
    setError(null);
    try {
      const response = await getGatewayCustomPolicies();
      setPolicies(response.list || []);
    } catch (cause) {
      setError(
        cause instanceof Error
          ? cause
          : new Error('Failed to load custom policies')
      );
    } finally {
      setIsLoading(false);
    }
  }, []);

  useEffect(() => {
    void fetchPolicies();
  }, [fetchPolicies]);

  useEffect(() => {
    setPage(0);
  }, [rowsPerPage, searchQuery, sortedColumn, sortOrder]);

  const handleSortBy = (field: SortableField) => {
    if (sortedColumn === field) {
      setSortOrder((prev) => (prev === 'asc' ? 'desc' : 'asc'));
    } else {
      setSortedColumn(field);
      setSortOrder('asc');
    }
  };

  const filteredAndSorted = useMemo(() => {
    const query = searchQuery.trim().toLowerCase();
    const filtered = query
      ? policies.filter((policy) =>
          [policy.name, policy.version, policy.description]
            .filter(Boolean)
            .join(' ')
            .toLowerCase()
            .includes(query)
        )
      : policies;

    return [...filtered].sort((a, b) =>
      compareValues(
        getSortValue(a, sortedColumn),
        getSortValue(b, sortedColumn),
        sortOrder
      )
    );
  }, [policies, searchQuery, sortedColumn, sortOrder]);

  const pageItems = useMemo(
    () =>
      filteredAndSorted.slice(
        page * rowsPerPage,
        page * rowsPerPage + rowsPerPage
      ),
    [filteredAndSorted, page, rowsPerPage]
  );

  const handleDeleteConfirm = async () => {
    if (!deleteTarget) return;
    setIsDeleting(true);
    try {
      await deleteGatewayCustomPolicy(deleteTarget.uuid, deleteTarget.version);
      setDeleteTarget(null);
      showSnackbar('Custom policy deleted successfully', 'success');
      await fetchPolicies();
    } catch (cause) {
      showSnackbar(
        getErrorMessage(cause, 'Failed to delete the custom policy.'),
        'error'
      );
      if (getErrorCode(cause) === 'POLICY_IN_USE') {
        setUsageDialogTarget(deleteTarget);
        setDeleteTarget(null);
      }
    } finally {
      setIsDeleting(false);
    }
  };

  const Wrapper: ElementType = embedded ? Box : PageContent;

  return (
    <Wrapper {...(embedded ? {} : { fullWidth: true })}>
      <Box sx={{ mb: 2 }}>
        <Typography variant="h4">
          <FormattedMessage
            id="aiWorkspace.pages.appShell.appShellPages.gateways.CustomPoliciesList.title"
            defaultMessage="Custom Policies"
          />
        </Typography>
        <Typography variant="body2" color="text.secondary">
          <FormattedMessage
            id="aiWorkspace.pages.appShell.appShellPages.gateways.CustomPoliciesList.subtitle"
            defaultMessage="Policies synced from your gateways into this organization."
          />
        </Typography>
      </Box>

      {policies.length !== 0 && (
        <Box sx={{ mb: 2 }}>
          <TextField
            fullWidth
            placeholder={intl.formatMessage({
              id: 'aiWorkspace.pages.appShell.appShellPages.gateways.CustomPoliciesList.search.placeholder',
              defaultMessage: 'Search policies...',
            })}
            value={searchQuery}
            disabled={isLoading}
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
        </Box>
      )}

      {error ? (
        <ErrorAlert error={error} onRetry={fetchPolicies} />
      ) : !isLoading && pageItems.length === 0 ? (
        <Box
          sx={{
            border: '1px dashed',
            borderColor: 'divider',
            borderRadius: 2,
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            px: 3,
            py: 8,
          }}
        >
          <Stack spacing={1.5} alignItems="center" sx={{ textAlign: 'center' }}>
            <Box
              sx={{
                width: 64,
                height: 64,
                borderRadius: 2,
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                color: 'primary.main',
                bgcolor: (theme) =>
                  `color-mix(in srgb, ${theme.palette.primary.main} 12%, transparent)`,
                border: '1px solid',
                borderColor: (theme) =>
                  `color-mix(in srgb, ${theme.palette.primary.main} 30%, transparent)`,
              }}
            >
              <ShieldCheck size={28} />
            </Box>
            <Typography variant="body1" sx={{ fontWeight: 700 }}>
              <FormattedMessage
                id="aiWorkspace.pages.appShell.appShellPages.gateways.CustomPoliciesList.noData.title"
                defaultMessage="No custom policies found"
              />
            </Typography>
            <Typography
              variant="body2"
              color="text.secondary"
              sx={{ maxWidth: 380 }}
            >
              {searchQuery ? (
                <FormattedMessage
                  id="aiWorkspace.pages.appShell.appShellPages.gateways.CustomPoliciesList.noData.noResults"
                  defaultMessage="No custom policies match your search."
                />
              ) : (
                <FormattedMessage
                  id="aiWorkspace.pages.appShell.appShellPages.gateways.CustomPoliciesList.noData.description"
                  defaultMessage="Custom policies synced from your gateways will appear here."
                />
              )}
            </Typography>
          </Stack>
        </Box>
      ) : (
        <Card>
          <TableContainer>
            <Table size="small">
              <TableHead>
                <TableRow>
                  {tableHeaders.map((header) => (
                    <TableCell key={header.key}>
                      {header.sortable ? (
                        <TableSortLabel
                          active={sortedColumn === header.key}
                          direction={sortOrder}
                          onClick={() =>
                            handleSortBy(header.key as SortableField)
                          }
                        >
                          {header.label}
                        </TableSortLabel>
                      ) : (
                        header.label
                      )}
                    </TableCell>
                  ))}
                  {canDelete ? (
                    <TableCell align="right">
                      <FormattedMessage
                        id="aiWorkspace.pages.appShell.appShellPages.gateways.CustomPoliciesList.header.actions"
                        defaultMessage="Actions"
                      />
                    </TableCell>
                  ) : null}
                </TableRow>
              </TableHead>
              <TableBody>
                {isLoading ? (
                  <TableRow>
                    <TableCell colSpan={canDelete ? 6 : 5}>
                      <Typography variant="body2" color="text.secondary">
                        <FormattedMessage
                          id="aiWorkspace.pages.appShell.appShellPages.gateways.CustomPoliciesList.loading"
                          defaultMessage="Loading custom policies..."
                        />
                      </Typography>
                    </TableCell>
                  </TableRow>
                ) : (
                  pageItems.map((policy) => (
                    <TableRow key={`${policy.uuid}-${policy.version}`} hover>
                      <TableCell>
                        <Typography
                          variant="body2"
                          sx={{ fontWeight: 600 }}
                          title={policy.name}
                        >
                          {policy.name}
                        </Typography>
                      </TableCell>
                      <TableCell>
                        <Typography variant="body2">
                          {formatVersion(policy.version)}
                        </Typography>
                      </TableCell>
                      <TableCell>
                        <Typography
                          variant="body2"
                          color="text.secondary"
                          sx={{
                            overflow: 'hidden',
                            textOverflow: 'ellipsis',
                            whiteSpace: 'nowrap',
                            maxWidth: 320,
                          }}
                          title={policy.description || '-'}
                        >
                          {policy.description || '-'}
                        </Typography>
                      </TableCell>
                      <TableCell>
                        <Tooltip
                          title={
                            policy.createdAt
                              ? new Date(policy.createdAt).toLocaleTimeString()
                              : ''
                          }
                          placement="top"
                        >
                          <Typography variant="body2" color="text.secondary">
                            {policy.createdAt
                              ? new Date(policy.createdAt).toLocaleDateString()
                              : '-'}
                          </Typography>
                        </Tooltip>
                      </TableCell>
                      <TableCell>
                        <Tooltip
                          title={
                            policy.updatedAt
                              ? new Date(policy.updatedAt).toLocaleTimeString()
                              : ''
                          }
                          placement="top"
                        >
                          <Typography variant="body2" color="text.secondary">
                            {policy.updatedAt
                              ? new Date(policy.updatedAt).toLocaleDateString()
                              : '-'}
                          </Typography>
                        </Tooltip>
                      </TableCell>
                      {canDelete ? (
                        <TableCell align="right">
                          <IconButton
                            size="small"
                            color="error"
                            onClick={() =>
                              setDeleteTarget({
                                uuid: policy.uuid,
                                version: policy.version,
                                name: policy.name,
                              })
                            }
                            aria-label={intl.formatMessage(
                              {
                                id: 'aiWorkspace.pages.appShell.appShellPages.gateways.CustomPoliciesList.delete.policy',
                                defaultMessage: 'Delete {name}',
                              },
                              { name: policy.name }
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

          {filteredAndSorted.length > 0 && (
            <TablePagination
              component="div"
              count={filteredAndSorted.length}
              page={page}
              onPageChange={(_event, nextPage) => setPage(nextPage)}
              rowsPerPage={rowsPerPage}
              onRowsPerPageChange={(event) =>
                setRowsPerPage(parseInt(event.target.value, 10))
              }
              rowsPerPageOptions={ROWS_PER_PAGE_OPTIONS}
            />
          )}
        </Card>
      )}

      <Dialog open={Boolean(deleteTarget)} onClose={() => setDeleteTarget(null)}>
        <DialogTitle>
          <FormattedMessage
            id="aiWorkspace.pages.appShell.appShellPages.gateways.CustomPoliciesList.deleteDialog.title"
            defaultMessage="Delete Custom Policy"
          />
        </DialogTitle>
        <DialogContent>
          <DialogContentText>
            <FormattedMessage
              id="aiWorkspace.pages.appShell.appShellPages.gateways.CustomPoliciesList.deleteDialog.message"
              defaultMessage="Are you sure you want to delete {name}? This action cannot be undone."
              values={{ name: <strong>{deleteTarget?.name}</strong> }}
            />
          </DialogContentText>
        </DialogContent>
        <DialogActions>
          <Button
            onClick={() => setDeleteTarget(null)}
            variant="outlined"
            color="secondary"
            disabled={isDeleting}
          >
            <FormattedMessage
              id="aiWorkspace.pages.appShell.appShellPages.gateways.CustomPoliciesList.deleteDialog.cancel"
              defaultMessage="Cancel"
            />
          </Button>
          <Button
            color="error"
            variant="contained"
            onClick={() => void handleDeleteConfirm()}
            disabled={isDeleting}
          >
            <FormattedMessage
              id="aiWorkspace.pages.appShell.appShellPages.gateways.CustomPoliciesList.deleteDialog.confirm"
              defaultMessage="Delete"
            />
          </Button>
        </DialogActions>
      </Dialog>

      {usageDialogTarget && (
        <PolicyInUseDialog
          open
          gatewayCustomPolicyId={usageDialogTarget.uuid}
          version={usageDialogTarget.version}
          fallbackName={usageDialogTarget.name}
          onClose={() => setUsageDialogTarget(null)}
        />
      )}
    </Wrapper>
  );
}
