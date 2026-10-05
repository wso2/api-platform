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
  FormLabel,
  IconButton,
  InputAdornment,
  OutlinedInput,
  Stack,
  Tooltip,
  Typography,
} from '@wso2/oxygen-ui';
import { Check, Copy, Pencil, X } from '@wso2/oxygen-ui-icons-react';
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
import { useFormatters } from '@/i18n/useFormatters';
import {
  HANDLE_MAX_LENGTH,
  HANDLE_MIN_LENGTH,
  handleProblem,
  toHandle,
  type HandleProblem,
} from '../serviceAccountHandle';
import { RolePicker } from './RolePicker';
import { useCopy } from './useCopy';

const messages = defineMessages({
  createTitle: { id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.components.ServiceAccountFormDialog.createTitle', defaultMessage: 'New service account' },
  createSubtitle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.components.ServiceAccountFormDialog.createSubtitle',
    defaultMessage: 'An identity for a script, pipeline or tool. You get a client ID and secret once it is created.',
  },
  editTitle: { id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.components.ServiceAccountFormDialog.editTitle', defaultMessage: 'Edit service account' },
  close: { id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.components.ServiceAccountFormDialog.close', defaultMessage: 'Close' },
  cancel: { id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.components.ServiceAccountFormDialog.cancel', defaultMessage: 'Cancel' },
  create: { id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.components.ServiceAccountFormDialog.create', defaultMessage: 'Create' },
  creating: { id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.components.ServiceAccountFormDialog.creating', defaultMessage: 'Creating…' },
  save: { id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.components.ServiceAccountFormDialog.save', defaultMessage: 'Save' },
  saving: { id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.components.ServiceAccountFormDialog.saving', defaultMessage: 'Saving…' },
  displayName: { id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.components.ServiceAccountFormDialog.displayName', defaultMessage: 'Display name' },
  handle: { id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.components.ServiceAccountFormDialog.handle', defaultMessage: 'ID' },
  handleHelper: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.components.ServiceAccountFormDialog.handleHelper',
    defaultMessage: 'Made from the display name and used in the client ID. Cannot be changed later.',
  },
  handleEditedHelper: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.components.ServiceAccountFormDialog.handleEditedHelper',
    defaultMessage: 'Used in the client ID. Cannot be changed later.',
  },
  editHandle: { id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.components.ServiceAccountFormDialog.editHandle', defaultMessage: 'Edit ID' },
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
  owner: { id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.components.ServiceAccountFormDialog.owner', defaultMessage: 'Owner' },
  ownerHelper: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.components.ServiceAccountFormDialog.ownerHelper',
    defaultMessage: 'Who answers for this account. Leave empty for you.',
  },
  ownerEditHelper: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.components.ServiceAccountFormDialog.ownerEditHelper',
    defaultMessage: 'Who answers for this account. Leave empty for the creator, {creator}.',
    description: '{creator} is the user name of whoever created the account.',
  },
  description: { id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.components.ServiceAccountFormDialog.description', defaultMessage: 'Description' },
  descriptionHelper: { id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.components.ServiceAccountFormDialog.descriptionHelper', defaultMessage: 'What the account is for.' },
  required: { id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.components.ServiceAccountFormDialog.required', defaultMessage: 'Required.' },
  tooLong: { id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.components.ServiceAccountFormDialog.tooLong', defaultMessage: 'Use at most {max} characters.' },
  roleRemovedWarning: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.components.ServiceAccountFormDialog.roleRemovedWarning',
    defaultMessage:
      'Removing a role stops every token already issued to this account. The workload must request a new one.',
  },
  conflict: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.components.ServiceAccountFormDialog.conflict',
    defaultMessage: 'Someone else changed this account while you were editing. Reload to see their changes.',
  },
  reload: { id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.components.ServiceAccountFormDialog.reload', defaultMessage: 'Reload' },
  copyClientId: { id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.components.ServiceAccountFormDialog.copyClientId', defaultMessage: 'Copy client ID' },
  clientIdCopied: { id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.components.ServiceAccountFormDialog.clientIdCopied', defaultMessage: 'Copied' },
  clientId: { id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.components.ServiceAccountFormDialog.clientId', defaultMessage: 'Client ID' },
  secret: { id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.components.ServiceAccountFormDialog.secret', defaultMessage: 'Secret' },
  createdBy: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.components.ServiceAccountFormDialog.createdBy',
    defaultMessage: 'Created by {who} on {date}',
    description: '{who} is a user name, {date} a formatted date.',
  },
  secretRegenerated: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.components.ServiceAccountFormDialog.secretRegenerated',
    defaultMessage: 'Secret last replaced {date}',
  },
  created: { id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.components.ServiceAccountFormDialog.created', defaultMessage: 'Created "{name}".' },
  updated: { id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.components.ServiceAccountFormDialog.updated', defaultMessage: 'Saved "{name}".' },
  createFailed: { id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.components.ServiceAccountFormDialog.createFailed', defaultMessage: 'Could not create the service account.' },
  loading: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.components.ServiceAccountFormDialog.loading',
    defaultMessage: 'Loading the service account',
  },
  loadFailed: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.components.ServiceAccountFormDialog.loadFailed',
    defaultMessage: 'Could not load the service account. Close this and try again.',
  },
  updateFailed: { id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.components.ServiceAccountFormDialog.updateFailed', defaultMessage: 'Could not save the service account.' },
});

const HANDLE_PROBLEM: Record<HandleProblem, MessageDescriptor> = {
  format: messages.handleFormat,
  length: messages.handleLength,
  reserved: messages.handleReserved,
};

/** Matches the server's limits, in characters; the server counts bytes. */
const MAX = { description: 1023, displayName: 255, owner: 255 } as const;

type TextField = keyof typeof MAX;

/** Only the display name is required; the server falls back to the creator for a blank owner. */
const textProblem = (value: string, field: TextField): MessageDescriptor | null => {
  const length = value.trim().length;
  if (length === 0) return field === 'displayName' ? messages.required : null;
  return length > MAX[field] ? messages.tooLong : null;
};

export type ServiceAccountFormDialogProps = {
  /** `null` creates a new account. */
  account: ServiceAccount | null;
  onClose: () => void;
  /** Called with the only copy of the new account's secret. */
  onCreated: (credentials: ServiceAccountCredentials) => void;
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

function EditLoader({ account, ...rest }: ServiceAccountFormDialogProps & { account: ServiceAccount }) {
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

function ServiceAccountForm({ account, onClose, onCreated }: ServiceAccountFormDialogProps) {
  const intl = useIntl();
  const { notify } = useNotifications();
  const { shortDate } = useFormatters();
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
  const [owner, setOwner] = useState(account?.owner ?? '');
  const [description, setDescription] = useState(account?.description ?? '');
  const [roles, setRoles] = useState<string[]>(account ? rolesOf(account) : []);
  const [submitted, setSubmitted] = useState(false);

  const effectiveHandle = handleEdited ? handle : toHandle(displayName);
  const problems = {
    description: textProblem(description, 'description'),
    displayName: textProblem(displayName, 'displayName'),
    handle: isEdit ? null : handleProblem(effectiveHandle),
    owner: textProblem(owner, 'owner'),
    roles: roles.length === 0,
  };
  const valid =
    !problems.description && !problems.displayName && !problems.handle && !problems.owner && !problems.roles;
  const removedRole = base !== null && rolesOf(base).some((role) => !roles.includes(role));

  const textError = (field: TextField) => {
    const problem = problems[field];
    // "Required" waits for a submit; a too-long value shows while typing.
    if (!problem || (problem === messages.required && !submitted)) return null;
    return intl.formatMessage(problem, { max: MAX[field] });
  };

  // Focus once the field is enabled; a disabled input cannot take focus.
  useEffect(() => {
    if (handleEdited && !isEdit) handleInput.current?.focus();
  }, [handleEdited, isEdit]);

  const editHandle = () => {
    setHandle(effectiveHandle);
    setHandleEdited(true);
  };

  const reload = () => {
    void reloadQuery.refetch().then((result) => {
      if (!result.data) return;
      setBase(result.data);
      setConflict(false);
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
      if (owner.trim() !== base.owner) body.owner = owner.trim();
      if (description.trim() !== base.description) body.description = description.trim();
      const before = rolesOf(base);
      if (roles.length !== before.length || roles.some((role) => !before.includes(role))) body.roles = roles;
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
        ...(owner.trim() ? { owner: owner.trim() } : {}),
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
      <DialogTitle>
        <Stack alignItems="flex-start" direction="row" spacing={1.5}>
          <Box sx={{ flex: 1, minWidth: 0 }}>
            <Typography component="span" sx={{ display: 'block', fontWeight: 600 }} variant="h6">
              <FormattedMessage {...(isEdit ? messages.editTitle : messages.createTitle)} />
            </Typography>
            <Typography color="text.secondary" component="span" sx={{ display: 'block' }} variant="body2">
              {isEdit ? base?.displayName : <FormattedMessage {...messages.createSubtitle} />}
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
      <Box component="form" noValidate onSubmit={submit}>
        <DialogContent>
          <Stack spacing={2.5}>
            {conflict && (
              <Alert
                action={
                  <Button color="inherit" disabled={reloadQuery.isFetching} onClick={reload} size="small">
                    <FormattedMessage {...messages.reload} />
                  </Button>
                }
                severity="error"
              >
                <FormattedMessage {...messages.conflict} />
              </Alert>
            )}

            {isEdit && base && (
              <Box>
                <Identity clientId={base.clientId} id={base.id} secret={base.maskedSecret} />
                {base.createdBy && base.createdAt && (
                  <Typography color="text.secondary" variant="caption" component="p">
                    <FormattedMessage
                      {...messages.createdBy}
                      values={{ date: shortDate(base.createdAt), who: base.createdBy }}
                    />
                  </Typography>
                )}
                {base.secretRegeneratedAt && (
                  <Typography color="text.secondary" variant="caption" component="p">
                    <FormattedMessage
                      {...messages.secretRegenerated}
                      values={{ date: shortDate(base.secretRegeneratedAt) }}
                    />
                  </Typography>
                )}
              </Box>
            )}

            <Form.Stack spacing={2}>
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
                {textError('displayName') && <FormHelperText>{textError('displayName')}</FormHelperText>}
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
                      intl.formatMessage(handleEdited ? messages.handleEditedHelper : messages.handleHelper)}
                  </FormHelperText>
                </FormControl>
              )}

              <FormControl error={Boolean(textError('owner'))} fullWidth>
                <FormLabel htmlFor="service-account-owner">
                  <FormattedMessage {...messages.owner} />
                </FormLabel>
                <OutlinedInput
                  id="service-account-owner"
                  onChange={(event) => setOwner(event.target.value)}
                  placeholder="platform-team@example.com"
                  value={owner}
                />
                <FormHelperText>
                  {textError('owner') ??
                    (isEdit && base?.createdBy
                      ? intl.formatMessage(messages.ownerEditHelper, { creator: base.createdBy })
                      : intl.formatMessage(messages.ownerHelper))}
                </FormHelperText>
              </FormControl>

              <FormControl error={Boolean(textError('description'))} fullWidth>
                <FormLabel htmlFor="service-account-description">
                  <FormattedMessage {...messages.description} />
                </FormLabel>
                <OutlinedInput
                  id="service-account-description"
                  minRows={2}
                  multiline
                  onChange={(event) => setDescription(event.target.value)}
                  value={description}
                />
                <FormHelperText>
                  {textError('description') ?? intl.formatMessage(messages.descriptionHelper)}
                </FormHelperText>
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
          </Stack>
        </DialogContent>
        <Divider />
        <DialogActions>
          <Button disabled={pending} onClick={onClose} variant="outlined">
            <FormattedMessage {...messages.cancel} />
          </Button>
          <Button disabled={pending || conflict || !rolesQuery.data || !valid} type="submit" variant="contained">
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
        </DialogActions>
      </Box>
    </Dialog>
  );
}

/** The fixed facts about an account: label on the left, value on the right. */
function Identity({ clientId, id, secret }: { clientId: string; id: string; secret: string }) {
  const intl = useIntl();
  const { copied, copy } = useCopy();
  const rows: [MessageDescriptor, string][] = [
    [messages.handle, id],
    [messages.clientId, clientId],
    [messages.secret, secret],
  ];

  return (
    <Box
      component="dl"
      sx={{ alignItems: 'center', columnGap: 2, display: 'grid', gridTemplateColumns: 'max-content 1fr', m: 0, mb: 1, rowGap: 0.5 }}
    >
      {rows.map(([label, value]) => (
        <Box key={label.id} sx={{ display: 'contents' }}>
          <Typography color="text.secondary" component="dt" variant="body2">
            <FormattedMessage {...label} />
          </Typography>
          <Stack alignItems="center" component="dd" direction="row" spacing={0.5} sx={{ m: 0, minWidth: 0 }}>
            <Typography sx={{ fontFamily: 'monospace', wordBreak: 'break-all' }} variant="body2">
              {value}
            </Typography>
            {label === messages.clientId && (
              <Tooltip title={intl.formatMessage(copied ? messages.clientIdCopied : messages.copyClientId)}>
                <IconButton
                  aria-label={intl.formatMessage(messages.copyClientId)}
                  onClick={() => copy(clientId)}
                  size="small"
                >
                  {copied ? <Check size={14} /> : <Copy size={14} />}
                </IconButton>
              </Tooltip>
            )}
          </Stack>
        </Box>
      ))}
    </Box>
  );
}
