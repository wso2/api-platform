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

import { useEffect, useRef, useState, type FormEvent } from 'react';
import {
  Alert,
  Box,
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Divider,
  Form,
  FormControl,
  FormHelperText,
  FormControlLabel,
  FormLabel,
  IconButton,
  InputAdornment,
  OutlinedInput,
  Stack,
  Switch,
  Typography,
} from '@wso2/oxygen-ui';
import { Pencil, X } from '@wso2/oxygen-ui-icons-react';
import { defineMessages, FormattedMessage, useIntl, type MessageDescriptor } from 'react-intl';

import { isErrorCode, type ApiError } from '@/api/core/errors';
import {
  isServiceAccountConflict,
  rolesOf,
  useCreateServiceAccount,
  useServiceAccount,
  useServiceAccountRoles,
  useUpdateServiceAccount,
  type ServiceAccount,
  type ServiceAccountCredentials,
  type UpdateServiceAccountBody,
} from '@/api/resources/serviceAccounts';
import { useNotifications } from '@/components/Notifications';
import { ErrorState, LoadingState } from '@/components/StateViews';
import {
  HANDLE_MAX_LENGTH,
  HANDLE_MIN_LENGTH,
  handleProblem,
  toHandle,
  type HandleProblem,
} from '../serviceAccountHandle';
import { DEFAULT_ROLE, RolePicker } from './RolePicker';
import { CredentialsPanel, SectionLabel, ServiceAccountHeader } from './ServiceAccountEditParts';

const messages = defineMessages({
  createTitle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.components.ServiceAccountFormDialog.createTitle',
    defaultMessage: 'New service account',
  },
  createSubtitle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.components.ServiceAccountFormDialog.createSubtitle',
    defaultMessage:
      'An identity for a script, pipeline or tool. You get a client ID and secret once it is created.',
  },
  close: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.components.ServiceAccountFormDialog.close',
    defaultMessage: 'Close',
  },
  cancel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.components.ServiceAccountFormDialog.cancel',
    defaultMessage: 'Cancel',
  },
  create: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.components.ServiceAccountFormDialog.create',
    defaultMessage: 'Create',
  },
  creating: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.components.ServiceAccountFormDialog.creating',
    defaultMessage: 'Creating…',
  },
  save: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.components.ServiceAccountFormDialog.save',
    defaultMessage: 'Save changes',
  },
  saving: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.components.ServiceAccountFormDialog.saving',
    defaultMessage: 'Saving…',
  },
  displayName: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.components.ServiceAccountFormDialog.displayName',
    defaultMessage: 'Display name',
  },
  handle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.components.ServiceAccountFormDialog.handle',
    defaultMessage: 'ID',
  },
  handleHelper: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.components.ServiceAccountFormDialog.handleHelper',
    defaultMessage:
      'Made from the display name and used in the client ID. Cannot be changed later.',
  },
  handleEditedHelper: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.components.ServiceAccountFormDialog.handleEditedHelper',
    defaultMessage: 'Used in the client ID. Cannot be changed later.',
  },
  editHandle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.components.ServiceAccountFormDialog.editHandle',
    defaultMessage: 'Edit ID',
  },
  handleLength: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.components.ServiceAccountFormDialog.handleLength',
    defaultMessage: 'Use {min} to {max} characters.',
  },
  handleFormat: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.components.ServiceAccountFormDialog.handleFormat',
    defaultMessage: 'Use lowercase letters, digits and single hyphens, not at the start or end.',
  },
  handleReserved: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.components.ServiceAccountFormDialog.handleReserved',
    defaultMessage: 'This ID is reserved. Choose another.',
  },
  handleTaken: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.components.ServiceAccountFormDialog.handleTaken',
    defaultMessage: 'An account with this ID already exists.',
  },
  description: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.components.ServiceAccountFormDialog.description',
    defaultMessage: 'Description',
  },
  descriptionPlaceholder: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.components.ServiceAccountFormDialog.descriptionPlaceholder',
    defaultMessage: 'What the account is for',
  },
  details: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.components.ServiceAccountFormDialog.details',
    defaultMessage: 'Details',
  },
  credentials: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.components.ServiceAccountFormDialog.credentials',
    defaultMessage: 'Credentials',
  },
  enabled: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.components.ServiceAccountFormDialog.enabled',
    defaultMessage: 'Account enabled',
  },
  disableWarning: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.components.ServiceAccountFormDialog.disableWarning',
    defaultMessage:
      'Disabling stops every token already issued to this account, and it cannot get new ones until enabled.',
  },
  required: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.components.ServiceAccountFormDialog.required',
    defaultMessage: 'Required.',
  },
  tooLong: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.components.ServiceAccountFormDialog.tooLong',
    defaultMessage: 'Use at most {max} characters.',
  },
  roleRemovedWarning: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.components.ServiceAccountFormDialog.roleRemovedWarning',
    defaultMessage:
      'Removing a role stops every token already issued to this account. The workload must request a new one.',
  },
  conflict: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.components.ServiceAccountFormDialog.conflict',
    defaultMessage:
      'Someone else changed this account while you were editing. Reload to see their changes.',
  },
  reload: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.components.ServiceAccountFormDialog.reload',
    defaultMessage: 'Reload',
  },
  created: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.components.ServiceAccountFormDialog.created',
    defaultMessage: 'Created "{name}".',
  },
  updated: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.components.ServiceAccountFormDialog.updated',
    defaultMessage: 'Saved "{name}".',
  },
  createFailed: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.components.ServiceAccountFormDialog.createFailed',
    defaultMessage: 'Could not create the service account.',
  },
  loading: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.components.ServiceAccountFormDialog.loading',
    defaultMessage: 'Loading the service account',
  },
  loadFailed: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.components.ServiceAccountFormDialog.loadFailed',
    defaultMessage: 'Could not load the service account. Close this and try again.',
  },
  updateFailed: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.components.ServiceAccountFormDialog.updateFailed',
    defaultMessage: 'Could not save the service account.',
  },
});

const HANDLE_PROBLEM: Record<HandleProblem, MessageDescriptor> = {
  format: messages.handleFormat,
  length: messages.handleLength,
  reserved: messages.handleReserved,
};

/** Matches the server's limits, in characters; the server counts bytes. */
const MAX = { description: 1023, displayName: 255 } as const;

type TextField = keyof typeof MAX;

/** Only the display name is required. */
const textProblem = (value: string, field: TextField): MessageDescriptor | null => {
  const length = value.trim().length;
  if (length === 0) return field === 'displayName' ? messages.required : null;
  return length > MAX[field] ? messages.tooLong : null;
};

const sameRoles = (a: readonly string[], b: readonly string[]) =>
  a.length === b.length && a.every((role) => b.includes(role));

export type ServiceAccountFormDialogProps = {
  /** `null` creates a new account. */
  account: ServiceAccount | null;
  onClose: () => void;
  /** Called with the only copy of the new account's secret. */
  onCreated: (credentials: ServiceAccountCredentials) => void;
  /** Asks for a new secret; the page confirms it and shows the result. */
  onRegenerate: (account: ServiceAccount) => void;
};

/**
 * Create and edit. Mount it per account (keyed). An edit starts from a fresh
 * read of the account, not the list's cached copy, because the update carries
 * no version: a stale form would silently overwrite another admin's change.
 */
export function ServiceAccountFormDialog(props: ServiceAccountFormDialogProps) {
  if (props.account === null) return <ServiceAccountForm {...props} account={null} />;
  return <EditLoader {...props} account={props.account} />;
}

function EditLoader({
  account,
  ...rest
}: ServiceAccountFormDialogProps & { account: ServiceAccount }) {
  const intl = useIntl();
  const detail = useServiceAccount(account.id);

  if (detail.data) return <ServiceAccountForm {...rest} account={detail.data} />;
  return (
    <Dialog fullWidth maxWidth="sm" onClose={rest.onClose} open>
      <DialogContent>
        {detail.error ? (
          <ErrorState message={intl.formatMessage(messages.loadFailed)} />
        ) : (
          <LoadingState label={intl.formatMessage(messages.loading)} />
        )}
      </DialogContent>
      <DialogActions>
        <Button onClick={rest.onClose} variant="outlined">
          <FormattedMessage {...messages.cancel} />
        </Button>
      </DialogActions>
    </Dialog>
  );
}

function ServiceAccountForm({
  account,
  onClose,
  onCreated,
  onRegenerate,
}: ServiceAccountFormDialogProps) {
  const intl = useIntl();
  const { notify } = useNotifications();
  const isEdit = account !== null;

  const rolesQuery = useServiceAccountRoles();
  const createMutation = useCreateServiceAccount();
  const updateMutation = useUpdateServiceAccount();
  const pending = createMutation.isPending || updateMutation.isPending;

  // What the edit is diffed against; replaced by a fresh copy after a 409.
  const [base, setBase] = useState<ServiceAccount | null>(account);
  const [conflict, setConflict] = useState(false);
  const reloadQuery = useServiceAccount(account?.id);

  const [displayName, setDisplayName] = useState(account?.displayName ?? '');
  const [handle, setHandle] = useState(account?.id ?? '');
  const [handleEdited, setHandleEdited] = useState(isEdit);
  const [handleTaken, setHandleTaken] = useState(false);
  const handleInput = useRef<HTMLInputElement>(null);
  const [description, setDescription] = useState(account?.description ?? '');
  const [roles, setRoles] = useState<string[]>(account ? rolesOf(account) : []);
  const [defaulted, setDefaulted] = useState(isEdit);
  const [enabled, setEnabled] = useState(account ? account.status === 'active' : true);
  const [submitted, setSubmitted] = useState(false);

  const effectiveHandle = handleEdited ? handle : toHandle(displayName);
  const problems = {
    description: textProblem(description, 'description'),
    displayName: textProblem(displayName, 'displayName'),
    handle: isEdit ? null : handleProblem(effectiveHandle),
    roles: roles.length === 0,
  };
  const valid =
    !problems.description && !problems.displayName && !problems.handle && !problems.roles;
  const removedRole = base !== null && rolesOf(base).some((role) => !roles.includes(role));
  const disabling = base !== null && base.status === 'active' && !enabled;

  const textError = (field: TextField) => {
    const problem = problems[field];
    // "Required" waits for a submit; a too-long value shows while typing.
    if (!problem || (problem === messages.required && !submitted)) return null;
    return intl.formatMessage(problem, { max: MAX[field] });
  };

  // A new account starts with the default role; the admin may remove it.
  useEffect(() => {
    if (defaulted || !rolesQuery.data) return;
    setDefaulted(true);
    if (rolesQuery.data.list.some((role) => role.name === DEFAULT_ROLE)) setRoles([DEFAULT_ROLE]);
  }, [defaulted, rolesQuery.data]);

  // The server's copy moved: a Reload after a 409, or a regenerate from the
  // Credentials panel. Fields the admin has not touched follow it; edits are kept.
  useEffect(() => {
    if (!account || !base || account === base) return;
    if (displayName === base.displayName) setDisplayName(account.displayName);
    if (description === base.description) setDescription(account.description);
    if (sameRoles(roles, rolesOf(base))) setRoles(rolesOf(account));
    if (enabled === (base.status === 'active')) setEnabled(account.status === 'active');
    setBase(account);
  }, [account, base, description, displayName, enabled, roles]);

  // Focus once the field is enabled; a disabled input cannot take focus.
  useEffect(() => {
    if (handleEdited && !isEdit) handleInput.current?.focus();
  }, [handleEdited, isEdit]);

  const editHandle = () => {
    setHandle(effectiveHandle);
    setHandleEdited(true);
  };

  // The fresh copy arrives through `account`, which shares this query's cache.
  const reload = () => {
    void reloadQuery.refetch().then((result) => {
      if (result.data) setConflict(false);
    });
  };

  const failWith = (error: ApiError, fallback: MessageDescriptor) => {
    if (!isEdit && isErrorCode(error, 'SERVICE_ACCOUNT_EXISTS')) {
      setHandleTaken(true);
      return;
    }
    if (isEdit && isServiceAccountConflict(error)) {
      setConflict(true);
      return;
    }
    notify(error.message || intl.formatMessage(fallback), 'error');
  };

  const submit = (event: FormEvent) => {
    event.preventDefault();
    setSubmitted(true);
    if (!valid || pending) return;
    const name = displayName.trim();

    if (isEdit && base) {
      const body: UpdateServiceAccountBody = {};
      if (name !== base.displayName) body.displayName = name;
      if (description.trim() !== base.description) body.description = description.trim();
      if (!sameRoles(roles, rolesOf(base))) body.roles = roles;
      const status = enabled ? 'active' : 'disabled';
      if (status !== base.status) body.status = status;
      if (Object.keys(body).length === 0) {
        onClose();
        return;
      }
      updateMutation.mutate(
        { body, id: base.id },
        {
          onError: (error) => failWith(error, messages.updateFailed),
          onSuccess: () => {
            notify(intl.formatMessage(messages.updated, { name }), 'success');
            onClose();
          },
        },
      );
      return;
    }

    createMutation.mutate(
      {
        displayName: name,
        id: effectiveHandle,
        roles,
        ...(description.trim() ? { description: description.trim() } : {}),
      },
      {
        onError: (error) => failWith(error, messages.createFailed),
        onSuccess: (credentials) => {
          notify(intl.formatMessage(messages.created, { name }), 'success');
          onCreated(credentials);
        },
      },
    );
  };

  const handleError = handleTaken
    ? intl.formatMessage(messages.handleTaken)
    : (submitted || handleEdited || displayName.trim()) && problems.handle
      ? intl.formatMessage(HANDLE_PROBLEM[problems.handle], {
          max: HANDLE_MAX_LENGTH,
          min: HANDLE_MIN_LENGTH,
        })
      : null;

  return (
    <Dialog fullWidth maxWidth="sm" onClose={pending ? undefined : onClose} open>
      {isEdit && base ? (
        <>
          <ServiceAccountHeader account={base} disabled={pending} onClose={onClose} />
          <Divider />
        </>
      ) : (
        <DialogTitle>
          <Stack alignItems="flex-start" direction="row" spacing={1.5}>
            <Box sx={{ flex: 1, minWidth: 0 }}>
              <Typography component="span" sx={{ display: 'block', fontWeight: 600 }} variant="h6">
                <FormattedMessage {...messages.createTitle} />
              </Typography>
              <Typography
                color="text.secondary"
                component="span"
                sx={{ display: 'block' }}
                variant="body2"
              >
                <FormattedMessage {...messages.createSubtitle} />
              </Typography>
            </Box>
            <IconButton
              aria-label={intl.formatMessage(messages.close)}
              disabled={pending}
              onClick={onClose}
              size="small"
            >
              <X size={18} />
            </IconButton>
          </Stack>
        </DialogTitle>
      )}
      {/* The fields scroll; the header and footer stay put. */}
      <Box
        component="form"
        noValidate
        onSubmit={submit}
        sx={{ display: 'flex', flexDirection: 'column', minHeight: 0 }}
      >
        <DialogContent>
          <Stack spacing={2.5}>
            {conflict && (
              <Alert
                action={
                  <Button
                    color="inherit"
                    disabled={reloadQuery.isFetching}
                    onClick={reload}
                    size="small"
                  >
                    <FormattedMessage {...messages.reload} />
                  </Button>
                }
                severity="error"
              >
                <FormattedMessage {...messages.conflict} />
              </Alert>
            )}

            <Form.Stack spacing={2}>
              {isEdit && (
                <SectionLabel>
                  <FormattedMessage {...messages.details} />
                </SectionLabel>
              )}
              <FormControl error={Boolean(textError('displayName'))} fullWidth required>
                <FormLabel htmlFor="service-account-display-name">
                  <FormattedMessage {...messages.displayName} />
                </FormLabel>
                <OutlinedInput
                  autoFocus
                  id="service-account-display-name"
                  onChange={(event) => setDisplayName(event.target.value)}
                  value={displayName}
                />
                {textError('displayName') && (
                  <FormHelperText>{textError('displayName')}</FormHelperText>
                )}
              </FormControl>

              {!isEdit && (
                <FormControl error={Boolean(handleError)} fullWidth required>
                  <FormLabel htmlFor="service-account-handle">
                    <FormattedMessage {...messages.handle} />
                  </FormLabel>
                  <OutlinedInput
                    disabled={!handleEdited}
                    endAdornment={
                      handleEdited ? undefined : (
                        <InputAdornment position="end">
                          <IconButton
                            aria-label={intl.formatMessage(messages.editHandle)}
                            edge="end"
                            onClick={editHandle}
                            size="small"
                          >
                            <Pencil size={16} />
                          </IconButton>
                        </InputAdornment>
                      )
                    }
                    id="service-account-handle"
                    inputProps={{ maxLength: HANDLE_MAX_LENGTH, spellCheck: false }}
                    inputRef={handleInput}
                    onChange={(event) => {
                      setHandle(event.target.value);
                      setHandleTaken(false);
                    }}
                    sx={{ fontFamily: 'monospace' }}
                    value={effectiveHandle}
                  />
                  <FormHelperText>
                    {handleError ??
                      intl.formatMessage(
                        handleEdited ? messages.handleEditedHelper : messages.handleHelper,
                      )}
                  </FormHelperText>
                </FormControl>
              )}

              <FormControl error={Boolean(textError('description'))} fullWidth>
                <FormLabel htmlFor="service-account-description">
                  <FormattedMessage {...messages.description} />
                </FormLabel>
                <OutlinedInput
                  id="service-account-description"
                  minRows={2}
                  multiline
                  onChange={(event) => setDescription(event.target.value)}
                  placeholder={intl.formatMessage(messages.descriptionPlaceholder)}
                  value={description}
                />
                {textError('description') && (
                  <FormHelperText>{textError('description')}</FormHelperText>
                )}
              </FormControl>

              <RolePicker
                failed={Boolean(rolesQuery.error)}
                loading={rolesQuery.isPending}
                onChange={setRoles}
                roles={rolesQuery.data?.list ?? []}
                showRequiredError={submitted && problems.roles}
                value={roles}
                warning={
                  isEdit && removedRole ? (
                    <Alert severity="warning">
                      <FormattedMessage {...messages.roleRemovedWarning} />
                    </Alert>
                  ) : undefined
                }
              />
            </Form.Stack>

            {isEdit && base && (
              <Stack spacing={1.5}>
                <SectionLabel>
                  <FormattedMessage {...messages.credentials} />
                </SectionLabel>
                <CredentialsPanel
                  account={base}
                  disabled={pending}
                  onRegenerate={() => onRegenerate(base)}
                />
              </Stack>
            )}
            {disabling && (
              <Alert severity="warning">
                <FormattedMessage {...messages.disableWarning} />
              </Alert>
            )}
          </Stack>
        </DialogContent>
        <Divider />
        <DialogActions sx={{ justifyContent: 'space-between', px: 3, py: 2 }}>
          {isEdit ? (
            <FormControlLabel
              control={
                <Switch
                  checked={enabled}
                  disabled={pending}
                  onChange={(event) => setEnabled(event.target.checked)}
                />
              }
              label={intl.formatMessage(messages.enabled)}
            />
          ) : (
            <span />
          )}
          <Stack direction="row" spacing={1}>
            <Button disabled={pending} onClick={onClose} variant="outlined">
              <FormattedMessage {...messages.cancel} />
            </Button>
            <Button
              disabled={pending || conflict || !rolesQuery.data || !valid}
              type="submit"
              variant="contained"
            >
              <FormattedMessage
                {...(isEdit
                  ? pending
                    ? messages.saving
                    : messages.save
                  : pending
                    ? messages.creating
                    : messages.create)}
              />
            </Button>
          </Stack>
        </DialogActions>
      </Box>
    </Dialog>
  );
}
