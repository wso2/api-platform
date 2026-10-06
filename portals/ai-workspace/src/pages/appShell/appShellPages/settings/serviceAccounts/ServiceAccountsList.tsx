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

import { useCallback, useEffect, useState } from 'react';
import {
  Alert,
  Box,
  Button,
  Card,
  Chip,
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogContentText,
  DialogTitle,
  IconButton,
  InputAdornment,
  Link,
  ListItemIcon,
  ListItemText,
  Menu,
  MenuItem,
  PageContent,
  PageTitle,
  Stack,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TablePagination,
  TableRow,
  TextField,
  Tooltip,
  Typography,
} from '@wso2/oxygen-ui';
import {
  Ban,
  Check,
  CirclePlay,
  Copy,
  EllipsisVertical,
  Laptop,
  Pencil,
  Plus,
  RefreshCw,
  Search,
  Ticket,
  Trash2,
} from '@wso2/oxygen-ui-icons-react';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';
import {
  deleteServiceAccount,
  listServiceAccountRoles,
  listServiceAccounts,
  regenerateServiceAccountSecret,
  scopesOfRoles,
  updateServiceAccount,
  type ListServiceAccountsResponse,
  type ServiceAccount,
  type ServiceAccountCredentials,
  type ServiceAccountRole,
} from '../../../../../apis/serviceAccountApis';
import { SCOPES } from '../../../../../auth/permissions';
import ErrorAlert from '../../../../../Components/common/ErrorAlert';
import { useAppAuth } from '../../../../../contexts/AppAuthContext';
import { formatRelativeTime } from '../../../../../contexts/ApplicationsContext';
import { useAIWorkspaceSnackbar } from '../../../../../hooks/aiWorkspaceSnackbar';
import { getErrorMessage, getHttpStatus } from '../../../../../utils/apiError';
import CredentialsDialog from './CredentialsDialog';
import { useCopy } from './SecretField';
import ServiceAccountFormDialog from './ServiceAccountFormDialog';
import TestTokenDialog from './TestTokenDialog';

const P = 'aiWorkspace.pages.appShell.appShellPages.settings.serviceAccounts.ServiceAccountsList';
const messages = defineMessages({
  title: { id: `${P}.title`, defaultMessage: 'Service accounts' },
  subtitle: {
    id: `${P}.subtitle`,
    defaultMessage: 'Identities for scripts, pipelines and other tools that call the API Platform.',
  },
  create: { id: `${P}.create`, defaultMessage: 'New service account' },
  forbiddenTitle: { id: `${P}.forbiddenTitle`, defaultMessage: 'Service accounts are managed by administrators' },
  forbiddenBody: {
    id: `${P}.forbiddenBody`,
    defaultMessage: 'Ask an organization administrator if you need a service account.',
  },
  disabledTitle: { id: `${P}.disabledTitle`, defaultMessage: 'Service accounts are turned off' },
  disabledBody: {
    id: `${P}.disabledBody`,
    defaultMessage: 'An operator can turn them on in the Platform API configuration.',
  },
  emptyTitle: { id: `${P}.emptyTitle`, defaultMessage: 'Create your first service account' },
  emptyBody: {
    id: `${P}.emptyBody`,
    defaultMessage:
      'A service account belongs to no person. A pipeline or tool uses its client ID and secret to get a short-lived token.',
  },
  search: { id: `${P}.search`, defaultMessage: 'Search by name or ID' },
  noMatches: { id: `${P}.noMatches`, defaultMessage: 'No matching service accounts. Try a different search term.' },
  columnName: { id: `${P}.columnName`, defaultMessage: 'Name' },
  columnRoles: { id: `${P}.columnRoles`, defaultMessage: 'Roles' },
  columnStatus: { id: `${P}.columnStatus`, defaultMessage: 'Status' },
  // The server records when a token was last issued, not when one was last used.
  columnLastUsed: { id: `${P}.columnLastUsed`, defaultMessage: 'Last token issued' },
  columnActions: { id: `${P}.columnActions`, defaultMessage: 'Actions' },
  statusActive: { id: `${P}.statusActive`, defaultMessage: 'Active' },
  statusDisabled: { id: `${P}.statusDisabled`, defaultMessage: 'Disabled' },
  never: { id: `${P}.never`, defaultMessage: 'Never' },
  lastUsedDetail: { id: `${P}.lastUsedDetail`, defaultMessage: '{time} from {ip}' },
  copyClientId: { id: `${P}.copyClientId`, defaultMessage: 'Copy client ID of {name}' },
  copyClientIdTooltip: { id: `${P}.copyClientIdTooltip`, defaultMessage: 'Copy client ID: {clientId}' },
  clientIdCopied: { id: `${P}.clientIdCopied`, defaultMessage: 'Copied: {clientId}' },
  moreRoles: { id: `${P}.moreRoles`, defaultMessage: '+{count}' },
  actionsFor: { id: `${P}.actionsFor`, defaultMessage: 'Actions for {name}' },
  edit: { id: `${P}.edit`, defaultMessage: 'Edit' },
  testToken: { id: `${P}.testToken`, defaultMessage: 'Get token' },
  regenerate: { id: `${P}.regenerate`, defaultMessage: 'Regenerate secret' },
  disable: { id: `${P}.disable`, defaultMessage: 'Disable' },
  enable: { id: `${P}.enable`, defaultMessage: 'Enable' },
  delete: { id: `${P}.delete`, defaultMessage: 'Delete' },
  cancel: { id: `${P}.cancel`, defaultMessage: 'Cancel' },
  disableTitle: { id: `${P}.disableTitle`, defaultMessage: 'Disable "{name}"?' },
  disableBody: {
    id: `${P}.disableBody`,
    defaultMessage:
      'It will not be able to get new tokens, and tokens it already holds stop working within a few seconds. You can enable it again later.',
  },
  disabled: { id: `${P}.disabled`, defaultMessage: 'Disabled "{name}".' },
  enabled: {
    id: `${P}.enabled`,
    defaultMessage:
      'Enabled "{name}". Tokens issued before it was disabled stay invalid — the workload must request a new one.',
  },
  regenerateTitle: { id: `${P}.regenerateTitle`, defaultMessage: 'Regenerate the secret for "{name}"?' },
  regenerateBody: {
    id: `${P}.regenerateBody`,
    defaultMessage:
      'The current secret stops working immediately, with no overlap period, and tokens already issued stop working too. Anything still using the old secret fails until you give it the new one.',
  },
  deleteTitle: { id: `${P}.deleteTitle`, defaultMessage: 'Delete "{name}"?' },
  deleteBody: {
    id: `${P}.deleteBody`,
    defaultMessage:
      'This cannot be undone. The account is removed, its secret stops working, and every token it holds is rejected. If you might need it again, disable it instead.',
  },
  deleteInput: { id: `${P}.deleteInput`, defaultMessage: 'Type the ID ({id}) to confirm' },
  deleted: { id: `${P}.deleted`, defaultMessage: 'Deleted "{name}".' },
  actionFailed: { id: `${P}.actionFailed`, defaultMessage: 'The action could not be completed.' },
});

const ROWS_PER_PAGE_OPTIONS = [10, 25, 50];
const SEARCH_DEBOUNCE_MS = 300;
const VISIBLE_ROLES = 2;

type ConfirmKind = 'disable' | 'regenerate' | 'delete';
type RowAction = 'edit' | 'token' | 'enable' | ConfirmKind;

const CONFIRM_MESSAGES = {
  delete: { body: messages.deleteBody, label: messages.delete, title: messages.deleteTitle },
  disable: { body: messages.disableBody, label: messages.disable, title: messages.disableTitle },
  regenerate: { body: messages.regenerateBody, label: messages.regenerate, title: messages.regenerateTitle },
};

export default function ServiceAccountsList() {
  const intl = useIntl();
  const { hasPermission } = useAppAuth();

  if (!hasPermission(SCOPES.SERVICE_ACCOUNT_MANAGE)) {
    return (
      <PageContent>
        <Alert severity="info">
          <Typography sx={{ fontWeight: 600 }} variant="body2">
            {intl.formatMessage(messages.forbiddenTitle)}
          </Typography>
          {intl.formatMessage(messages.forbiddenBody)}
        </Alert>
      </PageContent>
    );
  }
  return <ServiceAccountsPage />;
}

function ServiceAccountsPage() {
  const intl = useIntl();
  const showSnackbar = useAIWorkspaceSnackbar();

  const [page, setPage] = useState(0);
  const [rowsPerPage, setRowsPerPage] = useState(ROWS_PER_PAGE_OPTIONS[0]);
  const [search, setSearch] = useState('');
  const [term, setTerm] = useState('');
  const [data, setData] = useState<ListServiceAccountsResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<Error | null>(null);

  const [roles, setRoles] = useState<ServiceAccountRole[]>([]);
  const [rolesLoading, setRolesLoading] = useState(true);
  const [rolesFailed, setRolesFailed] = useState(false);

  const [formTarget, setFormTarget] = useState<'create' | ServiceAccount | null>(null);
  const [credentials, setCredentials] = useState<ServiceAccountCredentials | null>(null);
  const [tokenTarget, setTokenTarget] = useState<{ account: ServiceAccount; secret?: string } | null>(null);
  const [confirm, setConfirm] = useState<{ kind: ConfirmKind; account: ServiceAccount } | null>(null);
  const [confirmText, setConfirmText] = useState('');
  const [acting, setActing] = useState(false);

  useEffect(() => {
    const timer = window.setTimeout(() => setTerm(search.trim()), SEARCH_DEBOUNCE_MS);
    return () => window.clearTimeout(timer);
  }, [search]);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const result = await listServiceAccounts({
        limit: rowsPerPage,
        offset: page * rowsPerPage,
        ...(term ? { query: term } : {}),
      });
      // A delete can empty the page being shown; step back to the last one with rows.
      if (result.list.length === 0 && page > 0 && result.pagination.total > 0) {
        setPage(Math.floor((result.pagination.total - 1) / rowsPerPage));
        return;
      }
      setData(result);
    } catch (err) {
      setError(err as Error);
    } finally {
      setLoading(false);
    }
  }, [page, rowsPerPage, term]);

  useEffect(() => {
    void load();
  }, [load]);

  useEffect(() => {
    listServiceAccountRoles()
      .then((result) => setRoles(result.list))
      .catch(() => setRolesFailed(true))
      .finally(() => setRolesLoading(false));
  }, []);

  const scopesFor = (account: ServiceAccount) => scopesOfRoles(account.roles, roles);

  const fail = (err: unknown) => showSnackbar(getErrorMessage(err, intl.formatMessage(messages.actionFailed)), 'error');

  const setStatus = async (account: ServiceAccount, status: 'active' | 'disabled') => {
    setActing(true);
    try {
      await updateServiceAccount(account.id, { status });
      showSnackbar(
        intl.formatMessage(status === 'active' ? messages.enabled : messages.disabled, { name: account.displayName }),
        'success',
      );
      setConfirm(null);
      void load();
    } catch (err) {
      fail(err);
    } finally {
      setActing(false);
    }
  };

  const runConfirm = async () => {
    if (!confirm) return;
    const { account, kind } = confirm;
    if (kind === 'disable') {
      await setStatus(account, 'disabled');
      return;
    }
    setActing(true);
    try {
      if (kind === 'regenerate') {
        const result = await regenerateServiceAccountSecret(account.id);
        setConfirm(null);
        setCredentials(result);
        // An edit dialog still open on this account follows the returned copy.
        setFormTarget((current) =>
          current !== null && current !== 'create' && current.id === account.id ? result.serviceAccount : current,
        );
      } else {
        await deleteServiceAccount(account.id);
        showSnackbar(intl.formatMessage(messages.deleted, { name: account.displayName }), 'success');
        setConfirm(null);
      }
      void load();
    } catch (err) {
      fail(err);
    } finally {
      setActing(false);
    }
  };

  const onRowAction = (account: ServiceAccount, action: RowAction) => {
    if (action === 'edit') setFormTarget(account);
    else if (action === 'token') setTokenTarget({ account });
    else if (action === 'enable') void setStatus(account, 'active');
    else {
      setConfirmText('');
      setConfirm({ account, kind: action });
    }
  };

  // A 404 on the list means the server has service accounts switched off.
  if (error && getHttpStatus(error) === 404) {
    return (
      <PageContent>
        <Header />
        <Alert severity="info">
          <Typography sx={{ fontWeight: 600 }} variant="body2">
            {intl.formatMessage(messages.disabledTitle)}
          </Typography>
          {intl.formatMessage(messages.disabledBody)}
        </Alert>
      </PageContent>
    );
  }

  const accounts = data?.list ?? [];
  const total = data?.pagination.total ?? 0;
  const isFirstRun = data !== null && total === 0 && !term && !search;
  const confirmMessages = confirm ? CONFIRM_MESSAGES[confirm.kind] : null;
  const confirmBlocked = confirm?.kind === 'delete' && confirmText !== confirm.account.id;

  return (
    <PageContent>
      <Header onCreate={isFirstRun || !data ? undefined : () => setFormTarget('create')} />

      {error ? (
        <ErrorAlert error={error} onRetry={() => void load()} />
      ) : !data ? (
        <Box sx={{ display: 'flex', justifyContent: 'center', py: 6 }}>
          <CircularProgress size={32} />
        </Box>
      ) : isFirstRun ? (
        <Box
          sx={{
            alignItems: 'center',
            border: '1px dashed',
            borderColor: 'divider',
            borderRadius: 2,
            display: 'flex',
            flexDirection: 'column',
            gap: 1.5,
            py: 6,
            px: 3,
            textAlign: 'center',
          }}
        >
          <Laptop size={40} />
          <Typography variant="h6">{intl.formatMessage(messages.emptyTitle)}</Typography>
          <Typography color="text.secondary" sx={{ maxWidth: 480 }} variant="body2">
            {intl.formatMessage(messages.emptyBody)}
          </Typography>
          <Button onClick={() => setFormTarget('create')} startIcon={<Plus size={18} />} variant="contained">
            <FormattedMessage {...messages.create} />
          </Button>
        </Box>
      ) : (
        <Stack spacing={2}>
          <TextField
            fullWidth
            onChange={(event) => {
              setSearch(event.target.value);
              setPage(0);
            }}
            placeholder={intl.formatMessage(messages.search)}
            size="small"
            slotProps={{
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
          <Card variant="outlined">
            <TableContainer sx={{ opacity: loading ? 0.6 : 1, overflowX: 'auto' }}>
              <Table size="small">
                <TableHead>
                  <TableRow>
                    <TableCell><FormattedMessage {...messages.columnName} /></TableCell>
                    <TableCell><FormattedMessage {...messages.columnRoles} /></TableCell>
                    <TableCell><FormattedMessage {...messages.columnStatus} /></TableCell>
                    <TableCell><FormattedMessage {...messages.columnLastUsed} /></TableCell>
                    <TableCell align="center" sx={{ width: 72 }}>
                      <FormattedMessage {...messages.columnActions} />
                    </TableCell>
                  </TableRow>
                </TableHead>
                <TableBody>
                  {accounts.length === 0 ? (
                    <TableRow>
                      <TableCell colSpan={5}>
                        <Typography color="text.secondary" sx={{ py: 3, textAlign: 'center' }} variant="body2">
                          {intl.formatMessage(messages.noMatches)}
                        </Typography>
                      </TableCell>
                    </TableRow>
                  ) : (
                    accounts.map((account) => (
                      <ServiceAccountRow
                        account={account}
                        key={account.id}
                        onAction={(action) => onRowAction(account, action)}
                      />
                    ))
                  )}
                </TableBody>
              </Table>
            </TableContainer>
            {total > ROWS_PER_PAGE_OPTIONS[0] && (
              <TablePagination
                component="div"
                count={total}
                onPageChange={(_event, next) => setPage(next)}
                onRowsPerPageChange={(event) => {
                  setRowsPerPage(parseInt(event.target.value, 10));
                  setPage(0);
                }}
                page={page}
                rowsPerPage={rowsPerPage}
                rowsPerPageOptions={ROWS_PER_PAGE_OPTIONS}
              />
            )}
          </Card>
        </Stack>
      )}

      {formTarget !== null && (
        <ServiceAccountFormDialog
          account={formTarget === 'create' ? null : formTarget}
          key={formTarget === 'create' ? 'create' : formTarget.id}
          onClose={() => setFormTarget(null)}
          onCreated={(result) => {
            setFormTarget(null);
            setCredentials(result);
            void load();
          }}
          onRegenerate={(account) => {
            setConfirmText('');
            setConfirm({ account, kind: 'regenerate' });
          }}
          onSaved={() => void load()}
          roles={{ failed: rolesFailed, list: roles, loading: rolesLoading }}
        />
      )}

      <CredentialsDialog
        credentials={credentials}
        onDone={() => setCredentials(null)}
        onTestToken={(result) => setTokenTarget({ account: result.serviceAccount, secret: result.clientSecret })}
        scopes={credentials ? scopesFor(credentials.serviceAccount) : []}
      />

      {tokenTarget && (
        <TestTokenDialog
          account={tokenTarget.account}
          initialSecret={tokenTarget.secret}
          key={tokenTarget.account.id}
          onClose={() => setTokenTarget(null)}
          onIssued={() => void load()}
          scopes={scopesFor(tokenTarget.account)}
        />
      )}

      {confirm && confirmMessages && (
        <Dialog fullWidth maxWidth="xs" onClose={acting ? undefined : () => setConfirm(null)} open>
          <DialogTitle>{intl.formatMessage(confirmMessages.title, { name: confirm.account.displayName })}</DialogTitle>
          <DialogContent>
            <DialogContentText>{intl.formatMessage(confirmMessages.body)}</DialogContentText>
            {confirm.kind === 'delete' && (
              <TextField
                autoFocus
                fullWidth
                label={intl.formatMessage(messages.deleteInput, { id: confirm.account.id })}
                onChange={(event) => setConfirmText(event.target.value)}
                size="small"
                sx={{ mt: 2 }}
                value={confirmText}
              />
            )}
          </DialogContent>
          <DialogActions>
            <Button disabled={acting} onClick={() => setConfirm(null)} variant="outlined">
              <FormattedMessage {...messages.cancel} />
            </Button>
            <Button
              color="error"
              disabled={acting || confirmBlocked}
              onClick={() => void runConfirm()}
              variant="contained"
            >
              {intl.formatMessage(confirmMessages.label)}
            </Button>
          </DialogActions>
        </Dialog>
      )}
    </PageContent>
  );
}

function Header({ onCreate }: { onCreate?: () => void }) {
  return (
    <PageTitle>
      <PageTitle.Header>
        <FormattedMessage {...messages.title} />
      </PageTitle.Header>
      <PageTitle.SubHeader>
        <FormattedMessage {...messages.subtitle} />
      </PageTitle.SubHeader>
      {onCreate && (
        <PageTitle.Actions>
          <Button onClick={onCreate} startIcon={<Plus size={18} />} variant="contained">
            <FormattedMessage {...messages.create} />
          </Button>
        </PageTitle.Actions>
      )}
    </PageTitle>
  );
}

function ServiceAccountRow({ account, onAction }: { account: ServiceAccount; onAction: (action: RowAction) => void }) {
  const intl = useIntl();
  const { copied, copy } = useCopy();
  const [menuAnchor, setMenuAnchor] = useState<HTMLElement | null>(null);
  const active = account.status === 'active';
  const shownRoles = account.roles.slice(0, VISIBLE_ROLES);
  const hiddenRoles = account.roles.slice(VISIBLE_ROLES);

  const choose = (action: RowAction) => {
    setMenuAnchor(null);
    onAction(action);
  };

  return (
    <TableRow hover>
      <TableCell sx={{ minWidth: 160 }}>
        <Stack alignItems="center" direction="row" spacing={0.5}>
          <Link
            color="inherit"
            component="button"
            onClick={() => onAction('edit')}
            sx={{ fontWeight: 600, textAlign: 'left' }}
            type="button"
            underline="hover"
            variant="body2"
          >
            {account.displayName}
          </Link>
          <Tooltip
            title={intl.formatMessage(copied ? messages.clientIdCopied : messages.copyClientIdTooltip, {
              clientId: account.clientId,
            })}
          >
            <IconButton
              aria-label={intl.formatMessage(messages.copyClientId, { name: account.displayName })}
              onClick={() => copy(account.clientId)}
              size="small"
            >
              {copied ? <Check size={14} /> : <Copy size={14} />}
            </IconButton>
          </Tooltip>
        </Stack>
      </TableCell>
      <TableCell>
        <Stack alignItems="center" direction="row" gap={0.5}>
          {shownRoles.map((role) => (
            <Chip key={role} label={role} size="small" />
          ))}
          {hiddenRoles.length > 0 && (
            <Tooltip title={hiddenRoles.join(', ')}>
              <Chip
                label={intl.formatMessage(messages.moreRoles, { count: hiddenRoles.length })}
                size="small"
                tabIndex={0}
                variant="outlined"
              />
            </Tooltip>
          )}
        </Stack>
      </TableCell>
      <TableCell>
        <Chip
          color={active ? 'success' : 'default'}
          label={intl.formatMessage(active ? messages.statusActive : messages.statusDisabled)}
          size="small"
          variant="outlined"
        />
      </TableCell>
      <TableCell>
        {account.lastUsedAt ? (
          <Tooltip
            title={
              account.lastUsedIp
                ? intl.formatMessage(messages.lastUsedDetail, {
                    ip: account.lastUsedIp,
                    time: intl.formatDate(account.lastUsedAt, { dateStyle: 'medium', timeStyle: 'short' }),
                  })
                : intl.formatDate(account.lastUsedAt, { dateStyle: 'medium', timeStyle: 'short' })
            }
          >
            <Typography variant="body2">{formatRelativeTime(account.lastUsedAt)}</Typography>
          </Tooltip>
        ) : (
          <Typography color="text.secondary" variant="body2">
            <FormattedMessage {...messages.never} />
          </Typography>
        )}
      </TableCell>
      <TableCell align="center">
        <IconButton
          aria-label={intl.formatMessage(messages.actionsFor, { name: account.displayName })}
          onClick={(event) => setMenuAnchor(event.currentTarget)}
          size="small"
        >
          <EllipsisVertical size={16} />
        </IconButton>
        <Menu anchorEl={menuAnchor} onClose={() => setMenuAnchor(null)} open={menuAnchor !== null}>
          <MenuItem onClick={() => choose('edit')}>
            <ListItemIcon><Pencil size={16} /></ListItemIcon>
            <ListItemText><FormattedMessage {...messages.edit} /></ListItemText>
          </MenuItem>
          <MenuItem disabled={!active} onClick={() => choose('token')}>
            <ListItemIcon><Ticket size={16} /></ListItemIcon>
            <ListItemText><FormattedMessage {...messages.testToken} /></ListItemText>
          </MenuItem>
          <MenuItem onClick={() => choose('regenerate')}>
            <ListItemIcon><RefreshCw size={16} /></ListItemIcon>
            <ListItemText><FormattedMessage {...messages.regenerate} /></ListItemText>
          </MenuItem>
          {active ? (
            <MenuItem onClick={() => choose('disable')}>
              <ListItemIcon><Ban size={16} /></ListItemIcon>
              <ListItemText><FormattedMessage {...messages.disable} /></ListItemText>
            </MenuItem>
          ) : (
            <MenuItem onClick={() => choose('enable')}>
              <ListItemIcon><CirclePlay size={16} /></ListItemIcon>
              <ListItemText><FormattedMessage {...messages.enable} /></ListItemText>
            </MenuItem>
          )}
          <MenuItem onClick={() => choose('delete')} sx={{ color: 'error.main' }}>
            <ListItemIcon sx={{ color: 'inherit' }}><Trash2 size={16} /></ListItemIcon>
            <ListItemText><FormattedMessage {...messages.delete} /></ListItemText>
          </MenuItem>
        </Menu>
      </TableCell>
    </TableRow>
  );
}
