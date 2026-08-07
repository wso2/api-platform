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

import React, { useEffect, useMemo, useRef, useState } from 'react';
import { Link as RouterLink, useNavigate } from 'react-router-dom';
import {
  Box,
  Button,
  Card,
  Chip,
  IconButton,
  InputAdornment,
  PageContent,
  PageTitle,
  Skeleton,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TablePagination,
  TableRow,
  TextField,
  Typography,
} from '@wso2/oxygen-ui';
import { KeyRound, Plus, Search, Trash2 } from '@wso2/oxygen-ui-icons-react';
import { FormattedMessage } from 'react-intl';
import { listSecrets, type SecretMetadata } from '../../../../apis/secretApis';
import { useAppShell } from '../../../../contexts/AppShellContext';
import { buildOrgPath } from '../../../../utils/projectRouting';
import { getErrorMessage } from '../../../../utils/apiError';
import ErrorAlert from '../../../../Components/common/ErrorAlert';
import DeleteSecretDialog from './DeleteSecretDialog';

const ROWS_PER_PAGE_OPTIONS = [10, 25, 50];

function formatDate(value?: string): string {
  if (!value) return '—';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return '—';
  return date.toLocaleDateString(undefined, { year: 'numeric', month: 'short', day: 'numeric' });
}

export default function SecretsList(): React.JSX.Element {
  const navigate = useNavigate();
  const { currentOrganization } = useAppShell();

  const [secrets, setSecrets] = useState<SecretMetadata[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<Error | null>(null);
  const [searchQuery, setSearchQuery] = useState('');
  const [deleteTarget, setDeleteTarget] = useState<SecretMetadata | null>(null);
  const [page, setPage] = useState(0);
  const [rowsPerPage, setRowsPerPage] = useState(ROWS_PER_PAGE_OPTIONS[0]);

  const organizationId = currentOrganization?.uuid ?? '';
  const basePath = buildOrgPath(currentOrganization, '/settings/secrets');

  // Guards against out-of-order responses (e.g. two concurrent fetches from a
  // double-click on Retry, or — defensively — an organization change): a response
  // for a superseded request must not replace the list a more recent request
  // already populated. Each mount gets its own counter, since OrgShell remounts
  // this whole page (via a key on the organization id) on every organization switch.
  const requestIdRef = useRef(0);

  const fetchSecrets = async () => {
    const requestId = ++requestIdRef.current;
    try {
      setIsLoading(true);
      setError(null);
      const response = await listSecrets({ limit: 100 });
      if (requestIdRef.current !== requestId) return; // superseded by a newer request
      setSecrets(response.list ?? []);
    } catch (err) {
      if (requestIdRef.current !== requestId) return;
      setError(err instanceof Error ? err : new Error(getErrorMessage(err, 'Failed to load secrets.')));
    } finally {
      if (requestIdRef.current === requestId) setIsLoading(false);
    }
  };

  useEffect(() => {
    if (!organizationId) return;
    void fetchSecrets();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [organizationId]);

  const filteredSecrets = useMemo(() => {
    const query = searchQuery.trim().toLowerCase();
    if (!query) return secrets;
    return secrets.filter((secret) =>
      [secret.displayName, secret.id, secret.description].filter(Boolean).join(' ').toLowerCase().includes(query)
    );
  }, [searchQuery, secrets]);

  useEffect(() => {
    setPage(0);
  }, [rowsPerPage, searchQuery]);

  // Clamps to the last valid page when the list shrinks out from under the current
  // page (e.g. deleting the only secret on the last page), instead of rendering blank.
  const pageCount = Math.max(1, Math.ceil(filteredSecrets.length / rowsPerPage));
  const safePage = Math.min(page, pageCount - 1);

  const pageSecrets = useMemo(
    () => filteredSecrets.slice(safePage * rowsPerPage, safePage * rowsPerPage + rowsPerPage),
    [filteredSecrets, safePage, rowsPerPage]
  );

  return (
    <PageContent fullWidth>
      <Box sx={{ display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between', gap: 2, flexWrap: 'wrap' }}>
        <PageTitle>
          <PageTitle.Header>
            <FormattedMessage
              id="aiWorkspace.pages.appShell.appShellPages.secret.SecretsList.title"
              defaultMessage="Secrets"
            />
          </PageTitle.Header>
          <PageTitle.SubHeader>
            <FormattedMessage
              id="aiWorkspace.pages.appShell.appShellPages.secret.SecretsList.subtitle"
              defaultMessage="Encrypted credentials referenced by LLM providers, proxies, and API configurations by handle."
            />
          </PageTitle.SubHeader>
        </PageTitle>

        {secrets.length > 0 && (
          <Button
            variant="contained"
            component={RouterLink}
            to={`${basePath}/new`}
            startIcon={<Plus size={20} />}
            data-cyid="create-secret-button"
          >
            <FormattedMessage
              id="aiWorkspace.pages.appShell.appShellPages.secret.SecretsList.create"
              defaultMessage="Create"
            />
          </Button>
        )}
      </Box>

      {error && !isLoading && (
        <Box sx={{ py: 2 }}>
          <ErrorAlert error={error} onRetry={fetchSecrets} />
        </Box>
      )}

      {isLoading ? (
        <Card sx={{ mt: 3 }}>
          <TableContainer>
            <Table size="small">
              <TableHead>
                <TableRow>
                  <TableCell>Name</TableCell>
                  <TableCell>Handle</TableCell>
                  <TableCell>Type</TableCell>
                  <TableCell>Last updated</TableCell>
                  <TableCell align="right">Actions</TableCell>
                </TableRow>
              </TableHead>
              <TableBody>
                {[...Array(3)].map((_, index) => (
                  <TableRow key={index}>
                    <TableCell><Skeleton variant="text" width="70%" /></TableCell>
                    <TableCell><Skeleton variant="text" width="60%" /></TableCell>
                    <TableCell><Skeleton variant="text" width="50%" /></TableCell>
                    <TableCell><Skeleton variant="text" width="60%" /></TableCell>
                    <TableCell align="right"><Skeleton variant="circular" width={24} height={24} /></TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </TableContainer>
        </Card>
      ) : !error && secrets.length === 0 ? (
        <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'center', py: 6 }}>
          <Box sx={{ textAlign: 'center', maxWidth: 420 }}>
            <KeyRound size={32} style={{ marginBottom: 12, opacity: 0.6 }} />
            <Typography variant="h6" sx={{ fontWeight: 700 }}>
              <FormattedMessage
                id="aiWorkspace.pages.appShell.appShellPages.secret.SecretsList.empty.title"
                defaultMessage="Create your first secret"
              />
            </Typography>
            <Typography variant="body2" color="text.secondary" sx={{ mt: 0.5, mb: 2 }}>
              <FormattedMessage
                id="aiWorkspace.pages.appShell.appShellPages.secret.SecretsList.empty.subtitle"
                defaultMessage="Store an API key or credential once, then reference its handle from any LLM provider, proxy, or MCP proxy."
              />
            </Typography>
            <Button
              variant="contained"
              component={RouterLink}
              to={`${basePath}/new`}
              startIcon={<Plus size={18} />}
              data-cyid="create-secret-button"
            >
              <FormattedMessage
                id="aiWorkspace.pages.appShell.appShellPages.secret.SecretsList.create"
                defaultMessage="Create"
              />
            </Button>
          </Box>
        </Box>
      ) : !error ? (
        <>
          <Box sx={{ my: 2 }}>
            <TextField
              fullWidth
              placeholder="Search secrets..."
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
          </Box>
          <Card>
            <TableContainer>
              <Table size="small">
                <TableHead>
                  <TableRow>
                    <TableCell>Name</TableCell>
                    <TableCell>Handle</TableCell>
                    <TableCell>Type</TableCell>
                    <TableCell>Last updated</TableCell>
                    <TableCell align="right">Actions</TableCell>
                  </TableRow>
                </TableHead>
                <TableBody>
                  {filteredSecrets.length === 0 ? (
                    <TableRow>
                      <TableCell colSpan={5}>
                        <Typography variant="body2" color="text.secondary">
                          No secrets match your search.
                        </Typography>
                      </TableCell>
                    </TableRow>
                  ) : (
                    pageSecrets.map((secret) => (
                      <TableRow
                        key={secret.uuid}
                        hover
                        role="link"
                        tabIndex={0}
                        onClick={() => navigate(`${basePath}/${secret.id}`)}
                        onKeyDown={(event) => {
                          if (event.key === 'Enter' || event.key === ' ') {
                            event.preventDefault();
                            navigate(`${basePath}/${secret.id}`);
                          }
                        }}
                        aria-label={`View ${secret.displayName}`}
                        sx={{ cursor: 'pointer' }}
                        data-cyid={`secret-row-${secret.id}`}
                      >
                        <TableCell sx={{ fontWeight: 500 }}>{secret.displayName}</TableCell>
                        <TableCell>
                          <Typography variant="body2" sx={{ fontFamily: 'monospace' }}>
                            {secret.id}
                          </Typography>
                        </TableCell>
                        <TableCell>
                          <Chip label={secret.type} size="small" variant="outlined" />
                        </TableCell>
                        <TableCell>{formatDate(secret.updatedAt)}</TableCell>
                        <TableCell align="right">
                          <IconButton
                            size="small"
                            color="error"
                            onClick={(event) => {
                              event.stopPropagation();
                              setDeleteTarget(secret);
                            }}
                            aria-label={`Delete ${secret.displayName}`}
                            data-cyid={`delete-secret-${secret.id}`}
                          >
                            <Trash2 size={16} />
                          </IconButton>
                        </TableCell>
                      </TableRow>
                    ))
                  )}
                </TableBody>
              </Table>
            </TableContainer>
            {filteredSecrets.length > 0 && (
              <TablePagination
                component="div"
                count={filteredSecrets.length}
                page={safePage}
                onPageChange={(_event, nextPage) => setPage(nextPage)}
                rowsPerPage={rowsPerPage}
                onRowsPerPageChange={(event) => setRowsPerPage(parseInt(event.target.value, 10))}
                rowsPerPageOptions={ROWS_PER_PAGE_OPTIONS}
              />
            )}
          </Card>
        </>
      ) : null}

      <DeleteSecretDialog
        secret={deleteTarget}
        onClose={() => setDeleteTarget(null)}
        onDeleted={(handle) => setSecrets((prev) => prev.filter((s) => s.id !== handle))}
      />
    </PageContent>
  );
}
