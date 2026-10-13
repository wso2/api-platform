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

import { useEffect, useState } from 'react';
import {
  Button,
  Chip,
  IconButton,
  Link,
  ListingTable,
  ListItemIcon,
  ListItemText,
  Menu,
  MenuItem,
  PageTitle,
  SearchBar,
  Stack,
  Tooltip,
  Typography,
} from '@wso2/oxygen-ui';
import {
  Ban,
  Check,
  ChevronLeft,
  ChevronRight,
  CirclePlay,
  Copy,
  EllipsisVertical,
  Pencil,
  Plus,
  RefreshCw,
  Ticket,
  Trash2,
} from '@wso2/oxygen-ui-icons-react';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';

import {
  isServiceAccountsDisabled,
  rolesOf,
  scopesOfRoles,
  useDeleteServiceAccount,
  useRegenerateServiceAccountSecret,
  useServiceAccountRoles,
  useServiceAccounts,
  useUpdateServiceAccount,
  type ServiceAccount,
  type ServiceAccountCredentials,
} from '@/api/resources/serviceAccounts';
import { ConfirmDialog } from '@/components/ConfirmDialog';
import { useNotifications } from '@/components/Notifications';
import { EmptyState, ErrorState, ForbiddenState, LoadingState } from '@/components/StateViews';
import { useDebouncedValue } from '@/hooks/useDebouncedValue';
import { useFormatters } from '@/i18n/useFormatters';
import { Can } from '@/permissions';
import { CredentialsDialog } from './components/CredentialsDialog';
import { ServiceAccountFormDialog } from './components/ServiceAccountFormDialog';
import { TestTokenDialog } from './components/TestTokenDialog';
import { useCopy } from './components/useCopy';

const messages = defineMessages({
  title: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.ServiceAccountsSettingsPage.title',
    defaultMessage: 'Service accounts',
  },
  subtitle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.ServiceAccountsSettingsPage.subtitle',
    defaultMessage: 'Identities for scripts, pipelines and other tools that call the API Platform.',
  },
  create: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.ServiceAccountsSettingsPage.create',
    defaultMessage: 'New service account',
  },
  loading: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.ServiceAccountsSettingsPage.loading',
    defaultMessage: 'Loading service accounts',
  },
  loadFailed: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.ServiceAccountsSettingsPage.loadFailed',
    defaultMessage: 'Unable to load service accounts',
  },
  forbiddenTitle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.ServiceAccountsSettingsPage.forbiddenTitle',
    defaultMessage: 'Service accounts are managed by administrators',
  },
  forbiddenBody: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.ServiceAccountsSettingsPage.forbiddenBody',
    defaultMessage: 'Ask an organization administrator if you need a service account.',
  },
  disabledTitle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.ServiceAccountsSettingsPage.disabledTitle',
    defaultMessage: 'Service accounts are turned off',
  },
  disabledBody: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.ServiceAccountsSettingsPage.disabledBody',
    defaultMessage: 'An operator can turn them on in the Platform API configuration.',
  },
  emptyTitle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.ServiceAccountsSettingsPage.emptyTitle',
    defaultMessage: 'Create your first service account',
  },
  emptyBody: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.ServiceAccountsSettingsPage.emptyBody',
    defaultMessage:
      'A service account belongs to no person. A pipeline or tool uses its client ID and secret to get a short-lived token.',
  },
  search: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.ServiceAccountsSettingsPage.search',
    defaultMessage: 'Search by name or ID',
  },
  noMatchesTitle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.ServiceAccountsSettingsPage.noMatchesTitle',
    defaultMessage: 'No matching service accounts',
  },
  noMatchesBody: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.ServiceAccountsSettingsPage.noMatchesBody',
    defaultMessage: 'Try a different search term.',
  },
  columnName: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.ServiceAccountsSettingsPage.columnName',
    defaultMessage: 'Name',
  },
  columnRoles: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.ServiceAccountsSettingsPage.columnRoles',
    defaultMessage: 'Roles',
  },
  columnStatus: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.ServiceAccountsSettingsPage.columnStatus',
    defaultMessage: 'Status',
  },
  columnLastUsed: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.ServiceAccountsSettingsPage.columnLastUsed',
    defaultMessage: 'Last token issued',
    description: 'Column header. The server records when a token was last issued, not when one was last used.',
  },
  columnActions: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.ServiceAccountsSettingsPage.columnActions',
    defaultMessage: 'Actions',
  },
  statusActive: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.ServiceAccountsSettingsPage.statusActive',
    defaultMessage: 'Active',
  },
  statusDisabled: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.ServiceAccountsSettingsPage.statusDisabled',
    defaultMessage: 'Disabled',
  },
  never: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.ServiceAccountsSettingsPage.never',
    defaultMessage: 'Never',
  },
  lastUsedDetail: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.ServiceAccountsSettingsPage.lastUsedDetail',
    defaultMessage: '{time} from {ip}',
    description: 'Tooltip: when and from which IP address the last token was issued.',
  },
  copyClientId: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.ServiceAccountsSettingsPage.copyClientId',
    defaultMessage: 'Copy client ID of {name}',
  },
  moreRoles: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.ServiceAccountsSettingsPage.moreRoles',
    defaultMessage: '+{count}',
    description: 'Chip standing for roles not shown in the list row; the tooltip names them.',
  },
  moreRolesLabel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.ServiceAccountsSettingsPage.moreRolesLabel',
    defaultMessage: '{count, plural, one {# more role} other {# more roles}}: {roles}',
  },
  copyClientIdTooltip: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.ServiceAccountsSettingsPage.copyClientIdTooltip',
    defaultMessage: 'Copy client ID: {clientId}',
    description: '{clientId} is a literal value, e.g. sa_acme_ci-bot_3f9a1c.',
  },
  clientIdCopied: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.ServiceAccountsSettingsPage.clientIdCopied',
    defaultMessage: 'Copied: {clientId}',
  },
  actionsFor: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.ServiceAccountsSettingsPage.actionsFor',
    defaultMessage: 'Actions for {name}',
  },
  edit: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.ServiceAccountsSettingsPage.edit',
    defaultMessage: 'Edit',
  },
  testToken: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.ServiceAccountsSettingsPage.testToken',
    defaultMessage: 'Get token',
  },
  regenerate: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.ServiceAccountsSettingsPage.regenerate',
    defaultMessage: 'Regenerate secret',
  },
  disable: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.ServiceAccountsSettingsPage.disable',
    defaultMessage: 'Disable',
  },
  enable: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.ServiceAccountsSettingsPage.enable',
    defaultMessage: 'Enable',
  },
  delete: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.ServiceAccountsSettingsPage.delete',
    defaultMessage: 'Delete',
  },
  disableTitle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.ServiceAccountsSettingsPage.disableTitle',
    defaultMessage: 'Disable "{name}"?',
  },
  disableBody: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.ServiceAccountsSettingsPage.disableBody',
    defaultMessage:
      'It will not be able to get new tokens, and tokens it already holds stop working within a few seconds. You can enable it again later.',
  },
  disabled: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.ServiceAccountsSettingsPage.disabled',
    defaultMessage: 'Disabled "{name}".',
  },
  enabled: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.ServiceAccountsSettingsPage.enabled',
    defaultMessage:
      'Enabled "{name}". Tokens issued before it was disabled stay invalid — the workload must request a new one.',
  },
  regenerateTitle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.ServiceAccountsSettingsPage.regenerateTitle',
    defaultMessage: 'Regenerate the secret for "{name}"?',
  },
  regenerateBody: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.ServiceAccountsSettingsPage.regenerateBody',
    defaultMessage:
      'The current secret stops working immediately, with no overlap period, and tokens already issued stop working too. Anything still using the old secret fails until you give it the new one.',
  },
  deleteTitle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.ServiceAccountsSettingsPage.deleteTitle',
    defaultMessage: 'Delete "{name}"?',
  },
  deleteBody: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.ServiceAccountsSettingsPage.deleteBody',
    defaultMessage:
      'This cannot be undone. The account is removed, its secret stops working, and every token it holds is rejected. If you might need it again, disable it instead.',
  },
  deleteInput: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.ServiceAccountsSettingsPage.deleteInput',
    defaultMessage: 'Type the ID to confirm',
  },
  deleted: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.ServiceAccountsSettingsPage.deleted',
    defaultMessage: 'Deleted "{name}".',
  },
  actionFailed: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.ServiceAccountsSettingsPage.actionFailed',
    defaultMessage: 'The action could not be completed.',
  },
  pageRange: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.ServiceAccountsSettingsPage.pageRange',
    defaultMessage: '{from}–{to} of {total}',
  },
  previousPage: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.ServiceAccountsSettingsPage.previousPage',
    defaultMessage: 'Previous page',
  },
  nextPage: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.ServiceAccountsSettingsPage.nextPage',
    defaultMessage: 'Next page',
  },
});

const PAGE_SIZE = 20;
const SEARCH_DEBOUNCE_MS = 300;

/** The offset of the last page that still has rows. */
const lastPageOffset = (total: number) =>
  total === 0 ? 0 : Math.floor((total - 1) / PAGE_SIZE) * PAGE_SIZE;

type Confirm = { kind: 'disable' | 'regenerate' | 'delete'; account: ServiceAccount };

/** The org Settings tab for service accounts. Administrators only. */
export function ServiceAccountsSettingsPage() {
  const intl = useIntl();
  return (
    <Can
      do="createServiceAccount"
      fallback={
        <ForbiddenState
          description={intl.formatMessage(messages.forbiddenBody)}
          title={intl.formatMessage(messages.forbiddenTitle)}
        />
      }
    >
      <ServiceAccountList />
    </Can>
  );
}

function ServiceAccountList() {
  const intl = useIntl();
  const { notify } = useNotifications();
  const [offset, setOffset] = useState(0);
  const [search, setSearch] = useState('');
  const term = useDebouncedValue(search.trim(), SEARCH_DEBOUNCE_MS);
  const accountsQuery = useServiceAccounts({ limit: PAGE_SIZE, offset, ...(term ? { query: term } : {}) });
  const rolesQuery = useServiceAccountRoles();
  const updateMutation = useUpdateServiceAccount();
  const regenerateMutation = useRegenerateServiceAccountSecret();
  const deleteMutation = useDeleteServiceAccount();
  const [formTarget, setFormTarget] = useState<'create' | ServiceAccount | null>(null);
  const [credentials, setCredentials] = useState<ServiceAccountCredentials | null>(null);
  const [tokenTarget, setTokenTarget] = useState<{ account: ServiceAccount; secret?: string } | null>(null);
  const [confirm, setConfirm] = useState<Confirm | null>(null);

  // A delete (here or by another admin) can empty the page being shown; step
  // back to the last page that still has rows.
  const total = accountsQuery.data?.pagination.total;
  useEffect(() => {
    if (total !== undefined && offset > 0 && offset >= total) setOffset(lastPageOffset(total));
  }, [offset, total]);

  const roles = rolesQuery.data?.list ?? [];
  const scopesFor = (account: ServiceAccount) => scopesOfRoles(rolesOf(account), roles);

  const fail = (error: { message?: string }) =>
    notify(error.message || intl.formatMessage(messages.actionFailed), 'error');

  const setStatus = (account: ServiceAccount, status: 'active' | 'disabled', onDone?: () => void) =>
    updateMutation.mutate(
      { body: { status }, id: account.id },
      {
        onError: fail,
        onSuccess: () => {
          notify(
            intl.formatMessage(status === 'active' ? messages.enabled : messages.disabled, {
              name: account.displayName,
            }),
            'success',
          );
          onDone?.();
        },
      },
    );

  const runConfirm = () => {
    if (!confirm) return;
    const { account, kind } = confirm;
    if (kind === 'disable') {
      setStatus(account, 'disabled', () => setConfirm(null));
    } else if (kind === 'regenerate') {
      regenerateMutation.mutate(
        { id: account.id },
        {
          onError: fail,
          onSuccess: (result) => {
            setConfirm(null);
            setCredentials(result);
            // The page outlives the dialog, so drop the mutation's copy of the
            // secret now rather than when the page unmounts.
            regenerateMutation.reset();
          },
        },
      );
    } else {
      deleteMutation.mutate(
        { id: account.id },
        {
          onError: fail,
          onSuccess: () => {
            notify(intl.formatMessage(messages.deleted, { name: account.displayName }), 'success');
            setConfirm(null);
          },
        },
      );
    }
  };

  if (accountsQuery.isPending) return <LoadingState label={intl.formatMessage(messages.loading)} />;
  if (accountsQuery.error) {
    return isServiceAccountsDisabled(accountsQuery.error) ? (
      <EmptyState
        description={intl.formatMessage(messages.disabledBody)}
        title={intl.formatMessage(messages.disabledTitle)}
      />
    ) : (
      <ErrorState message={intl.formatMessage(messages.loadFailed)} />
    );
  }

  const accounts = accountsQuery.data.list ?? [];
  const isFirstRun = accountsQuery.data.pagination.total === 0 && !term && !search;
  // Rows are empty while the clamp above moves back a page.
  const steppingBack = accounts.length === 0 && offset > 0;

  const confirmMessages = confirm
    ? {
        delete: { body: messages.deleteBody, label: messages.delete, title: messages.deleteTitle },
        disable: { body: messages.disableBody, label: messages.disable, title: messages.disableTitle },
        regenerate: { body: messages.regenerateBody, label: messages.regenerate, title: messages.regenerateTitle },
      }[confirm.kind]
    : null;

  return (
    <>
      <PageTitle>
        <PageTitle.Header variant="h5">
          <FormattedMessage {...messages.title} />
        </PageTitle.Header>
        <PageTitle.SubHeader>
          <FormattedMessage {...messages.subtitle} />
        </PageTitle.SubHeader>
        {!isFirstRun && (
          <PageTitle.Actions>
            <Button onClick={() => setFormTarget('create')} startIcon={<Plus size={18} />} variant="contained">
              <FormattedMessage {...messages.create} />
            </Button>
          </PageTitle.Actions>
        )}
      </PageTitle>

      {isFirstRun ? (
        <EmptyState
          actionIcon={<Plus size={18} />}
          actionLabel={intl.formatMessage(messages.create)}
          description={intl.formatMessage(messages.emptyBody)}
          onAction={() => setFormTarget('create')}
          title={intl.formatMessage(messages.emptyTitle)}
        />
      ) : (
        <Stack spacing={2.5}>
          <SearchBar
            fullWidth
            onChange={(event) => {
              setSearch(event.target.value);
              setOffset(0);
            }}
            placeholder={intl.formatMessage(messages.search)}
            value={search}
          />
          {steppingBack ? (
            <LoadingState label={intl.formatMessage(messages.loading)} />
          ) : accounts.length === 0 ? (
            <EmptyState
              description={intl.formatMessage(messages.noMatchesBody)}
              title={intl.formatMessage(messages.noMatchesTitle)}
            />
          ) : (
            <ListingTable.Provider>
              <ListingTable.Container
                sx={{ opacity: accountsQuery.isPlaceholderData ? 0.6 : 1, overflowX: 'auto' }}
              >
                <ListingTable>
                  <ListingTable.Head>
                    <ListingTable.Row>
                      <ListingTable.Cell>
                        <FormattedMessage {...messages.columnName} />
                      </ListingTable.Cell>
                      <ListingTable.Cell>
                        <FormattedMessage {...messages.columnRoles} />
                      </ListingTable.Cell>
                      <ListingTable.Cell>
                        <FormattedMessage {...messages.columnStatus} />
                      </ListingTable.Cell>
                      <ListingTable.Cell>
                        <FormattedMessage {...messages.columnLastUsed} />
                      </ListingTable.Cell>
                      <ListingTable.Cell align="center" sx={{ width: 72 }}>
                        <FormattedMessage {...messages.columnActions} />
                      </ListingTable.Cell>
                    </ListingTable.Row>
                  </ListingTable.Head>
                  <ListingTable.Body>
                    {accounts.map((account) => (
                      <ServiceAccountRow
                        account={account}
                        key={account.id}
                        onAction={(action) => {
                          if (action === 'edit') setFormTarget(account);
                          else if (action === 'token') setTokenTarget({ account });
                          else if (action === 'enable') setStatus(account, 'active');
                          else setConfirm({ account, kind: action });
                        }}
                      />
                    ))}
                  </ListingTable.Body>
                </ListingTable>
              </ListingTable.Container>
            </ListingTable.Provider>
          )}
          {total !== undefined && total > PAGE_SIZE && (
            <Stack alignItems="center" direction="row" justifyContent="flex-end" spacing={1}>
              <Typography color="text.secondary" variant="body2">
                <FormattedMessage
                  {...messages.pageRange}
                  values={{ from: offset + 1, to: Math.min(offset + PAGE_SIZE, total), total }}
                />
              </Typography>
              <IconButton
                aria-label={intl.formatMessage(messages.previousPage)}
                disabled={offset === 0}
                onClick={() => setOffset(Math.max(0, offset - PAGE_SIZE))}
                size="small"
              >
                <ChevronLeft size={18} />
              </IconButton>
              <IconButton
                aria-label={intl.formatMessage(messages.nextPage)}
                disabled={offset + PAGE_SIZE >= total}
                onClick={() => setOffset(offset + PAGE_SIZE)}
                size="small"
              >
                <ChevronRight size={18} />
              </IconButton>
            </Stack>
          )}
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
          }}
          onRegenerate={(account) => setConfirm({ account, kind: 'regenerate' })}
        />
      )}

      <CredentialsDialog
        credentials={credentials}
        onDone={() => setCredentials(null)}
        onTestToken={(result) =>
          setTokenTarget({ account: result.serviceAccount, secret: result.clientSecret })
        }
        scopes={credentials ? scopesFor(credentials.serviceAccount) : []}
      />

      {tokenTarget && (
        <TestTokenDialog
          account={tokenTarget.account}
          initialSecret={tokenTarget.secret}
          key={tokenTarget.account.id}
          onClose={() => setTokenTarget(null)}
          scopes={scopesFor(tokenTarget.account)}
        />
      )}

      {confirm && confirmMessages && (
        <ConfirmDialog
          confirmInputLabel={confirm.kind === 'delete' ? intl.formatMessage(messages.deleteInput) : undefined}
          confirmLabel={intl.formatMessage(confirmMessages.label)}
          confirmPhrase={confirm.kind === 'delete' ? confirm.account.id : undefined}
          destructive
          loading={updateMutation.isPending || regenerateMutation.isPending || deleteMutation.isPending}
          message={intl.formatMessage(confirmMessages.body)}
          onCancel={() => setConfirm(null)}
          onConfirm={runConfirm}
          open
          title={intl.formatMessage(confirmMessages.title, { name: confirm.account.displayName })}
        />
      )}
    </>
  );
}

type RowAction = 'edit' | 'token' | 'enable' | Confirm['kind'];

function ServiceAccountRow({
  account,
  onAction,
}: {
  account: ServiceAccount;
  onAction: (action: RowAction) => void;
}) {
  const intl = useIntl();
  const { dateTime, relativeTime } = useFormatters();
  const { copied, copy } = useCopy();
  const [menuAnchor, setMenuAnchor] = useState<HTMLElement | null>(null);
  const active = account.status === 'active';

  const choose = (action: RowAction) => {
    setMenuAnchor(null);
    onAction(action);
  };

  return (
    <ListingTable.Row>
      <ListingTable.Cell sx={{ minWidth: 160 }}>
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
      </ListingTable.Cell>
      <ListingTable.Cell>
        <RoleChips roles={rolesOf(account)} />
      </ListingTable.Cell>
      <ListingTable.Cell>
        <Chip
          color={active ? 'success' : 'default'}
          label={intl.formatMessage(active ? messages.statusActive : messages.statusDisabled)}
          size="small"
          variant="outlined"
        />
      </ListingTable.Cell>
      <ListingTable.Cell>
        {account.lastUsedAt ? (
          <Tooltip
            title={
              account.lastUsedIp
                ? intl.formatMessage(messages.lastUsedDetail, {
                    ip: account.lastUsedIp,
                    time: dateTime(account.lastUsedAt),
                  })
                : dateTime(account.lastUsedAt)
            }
          >
            <Typography variant="body2">{relativeTime(account.lastUsedAt)}</Typography>
          </Tooltip>
        ) : (
          <Typography color="text.secondary" variant="body2">
            <FormattedMessage {...messages.never} />
          </Typography>
        )}
      </ListingTable.Cell>
      <ListingTable.Cell align="center">
        <IconButton
          aria-label={intl.formatMessage(messages.actionsFor, { name: account.displayName })}
          onClick={(event) => setMenuAnchor(event.currentTarget)}
          size="small"
        >
          <EllipsisVertical size={16} />
        </IconButton>
        <Menu anchorEl={menuAnchor} onClose={() => setMenuAnchor(null)} open={menuAnchor !== null}>
          <MenuItem onClick={() => choose('edit')}>
            <ListItemIcon>
              <Pencil size={16} />
            </ListItemIcon>
            <ListItemText>
              <FormattedMessage {...messages.edit} />
            </ListItemText>
          </MenuItem>
          <MenuItem disabled={!active} onClick={() => choose('token')}>
            <ListItemIcon>
              <Ticket size={16} />
            </ListItemIcon>
            <ListItemText>
              <FormattedMessage {...messages.testToken} />
            </ListItemText>
          </MenuItem>
          <MenuItem onClick={() => choose('regenerate')}>
            <ListItemIcon>
              <RefreshCw size={16} />
            </ListItemIcon>
            <ListItemText>
              <FormattedMessage {...messages.regenerate} />
            </ListItemText>
          </MenuItem>
          {active ? (
            <MenuItem onClick={() => choose('disable')}>
              <ListItemIcon>
                <Ban size={16} />
              </ListItemIcon>
              <ListItemText>
                <FormattedMessage {...messages.disable} />
              </ListItemText>
            </MenuItem>
          ) : (
            <MenuItem onClick={() => choose('enable')}>
              <ListItemIcon>
                <CirclePlay size={16} />
              </ListItemIcon>
              <ListItemText>
                <FormattedMessage {...messages.enable} />
              </ListItemText>
            </MenuItem>
          )}
          <MenuItem onClick={() => choose('delete')} sx={{ color: 'error.main' }}>
            <ListItemIcon sx={{ color: 'inherit' }}>
              <Trash2 size={16} />
            </ListItemIcon>
            <ListItemText>
              <FormattedMessage {...messages.delete} />
            </ListItemText>
          </MenuItem>
        </Menu>
      </ListingTable.Cell>
    </ListingTable.Row>
  );
}

/** Roles a row shows before the rest collapse into a "+N" chip. */
const VISIBLE_ROLES = 2;

/** The first roles as chips, then "+N" naming the rest in its tooltip. */
function RoleChips({ roles }: { roles: string[] }) {
  const intl = useIntl();
  const shown = roles.slice(0, VISIBLE_ROLES);
  const hidden = roles.slice(VISIBLE_ROLES);
  const hiddenLabel = intl.formatMessage(messages.moreRolesLabel, {
    count: hidden.length,
    roles: hidden.join(', '),
  });

  return (
    <Stack alignItems="center" direction="row" gap={0.5}>
      {shown.map((role) => (
        <Chip key={role} label={role} size="small" />
      ))}
      {hidden.length > 0 && (
        <Tooltip title={hidden.join(', ')}>
          <Chip
            aria-label={hiddenLabel}
            label={intl.formatMessage(messages.moreRoles, { count: hidden.length })}
            size="small"
            tabIndex={0}
            variant="outlined"
          />
        </Tooltip>
      )}
    </Stack>
  );
}
