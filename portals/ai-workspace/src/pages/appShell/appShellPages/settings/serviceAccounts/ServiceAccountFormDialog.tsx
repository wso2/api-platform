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

import { useCallback, useEffect, useRef, useState, type FormEvent } from 'react';
import {
  Alert,
  Box,
  Button,
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Divider,
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
import {
  createServiceAccount,
  getServiceAccount,
  SERVICE_ACCOUNT_EXISTS,
  updateServiceAccount,
  type ServiceAccount,
  type ServiceAccountCredentials,
  type ServiceAccountRole,
  type UpdateServiceAccountRequest,
} from '../../../../../apis/serviceAccountApis';
import { useAIWorkspaceSnackbar } from '../../../../../hooks/aiWorkspaceSnackbar';
import { getErrorCode, getErrorMessage } from '../../../../../utils/apiError';
import RolePicker, { DEFAULT_ROLE } from './RolePicker';
import { CredentialsPanel, SectionLabel, ServiceAccountHeader } from './ServiceAccountEditParts';
import {
  HANDLE_MAX_LENGTH,
  HANDLE_MIN_LENGTH,
  handleProblem,
  toHandle,
  type HandleProblem,
} from './serviceAccountUtils';

const P = 'aiWorkspace.pages.appShell.appShellPages.settings.serviceAccounts.ServiceAccountFormDialog';
const messages = defineMessages({
  createTitle: { id: `${P}.createTitle`, defaultMessage: 'New service account' },
  createSubtitle: {
    id: `${P}.createSubtitle`,
    defaultMessage: 'An identity for a script, pipeline or tool. You get a client ID and secret once it is created.',
  },
  close: { id: `${P}.close`, defaultMessage: 'Close' },
  cancel: { id: `${P}.cancel`, defaultMessage: 'Cancel' },
  create: { id: `${P}.create`, defaultMessage: 'Create' },
  creating: { id: `${P}.creating`, defaultMessage: 'Creating…' },
  save: { id: `${P}.save`, defaultMessage: 'Save changes' },
  saving: { id: `${P}.saving`, defaultMessage: 'Saving…' },
  displayName: { id: `${P}.displayName`, defaultMessage: 'Display name' },
  handle: { id: `${P}.handle`, defaultMessage: 'ID' },
  handleHelper: {
    id: `${P}.handleHelper`,
    defaultMessage: 'Made from the display name and used in the client ID. Cannot be changed later.',
  },
  handleEditedHelper: { id: `${P}.handleEditedHelper`, defaultMessage: 'Used in the client ID. Cannot be changed later.' },
  editHandle: { id: `${P}.editHandle`, defaultMessage: 'Edit ID' },
  handleLength: { id: `${P}.handleLength`, defaultMessage: 'Use {min} to {max} characters.' },
  handleFormat: {
    id: `${P}.handleFormat`,
    defaultMessage: 'Use lowercase letters, digits and single hyphens, not at the start or end.',
  },
  handleReserved: { id: `${P}.handleReserved`, defaultMessage: 'This ID is reserved. Choose another.' },
  handleTaken: { id: `${P}.handleTaken`, defaultMessage: 'An account with this ID already exists.' },
  description: { id: `${P}.description`, defaultMessage: 'Description' },
  descriptionPlaceholder: { id: `${P}.descriptionPlaceholder`, defaultMessage: 'What the account is for' },
  details: { id: `${P}.details`, defaultMessage: 'Details' },
  credentials: { id: `${P}.credentials`, defaultMessage: 'Credentials' },
  enabled: { id: `${P}.enabled`, defaultMessage: 'Account enabled' },
  disableWarning: {
    id: `${P}.disableWarning`,
    defaultMessage: 'Disabling stops every token already issued to this account, and it cannot get new ones until enabled.',
  },
  required: { id: `${P}.required`, defaultMessage: 'Required.' },
  tooLong: { id: `${P}.tooLong`, defaultMessage: 'Use at most {max} characters.' },
  roleRemovedWarning: {
    id: `${P}.roleRemovedWarning`,
    defaultMessage:
      'Removing a role stops every token already issued to this account. The workload must request a new one.',
  },
  conflict: {
    id: `${P}.conflict`,
    defaultMessage: 'Someone else changed this account while you were editing. Reload to see their changes.',
  },
  reload: { id: `${P}.reload`, defaultMessage: 'Reload' },
  created: { id: `${P}.created`, defaultMessage: 'Created "{name}".' },
  updated: { id: `${P}.updated`, defaultMessage: 'Saved "{name}".' },
  createFailed: { id: `${P}.createFailed`, defaultMessage: 'Could not create the service account.' },
  updateFailed: { id: `${P}.updateFailed`, defaultMessage: 'Could not save the service account.' },
  loadFailed: {
    id: `${P}.loadFailed`,
    defaultMessage: 'Could not load the service account. Close this and try again.',
  },
});

const HANDLE_PROBLEM: Record<HandleProblem, MessageDescriptor> = {
  format: messages.handleFormat,
  length: messages.handleLength,
  reserved: messages.handleReserved,
};

const MAX = { description: 1023, displayName: 255 } as const;

type TextField = keyof typeof MAX;

const textProblem = (value: string, field: TextField): MessageDescriptor | null => {
  const length = value.trim().length;
  if (length === 0) return field === 'displayName' ? messages.required : null;
  return length > MAX[field] ? messages.tooLong : null;
};

const sameRoles = (a: readonly string[], b: readonly string[]) =>
  a.length === b.length && a.every((role) => b.includes(role));

interface RolesState {
  list: ServiceAccountRole[];
  loading: boolean;
  failed: boolean;
}

interface ServiceAccountFormDialogProps {
  /** `null` creates a new account. */
  account: ServiceAccount | null;
  roles: RolesState;
  onClose: () => void;
  /** Called with the only copy of the new account's secret. */
  onCreated: (credentials: ServiceAccountCredentials) => void;
  onSaved: () => void;
  /** Asks for a new secret; the page confirms it and shows the result. */
  onRegenerate: (account: ServiceAccount) => void;
}

export default function ServiceAccountFormDialog(props: ServiceAccountFormDialogProps) {
  if (props.account === null) return <ServiceAccountForm {...props} account={null} />;
  return <EditLoader {...props} account={props.account} />;
}

// The list row may be stale; edit the server's current copy.
function EditLoader({ account, ...rest }: ServiceAccountFormDialogProps & { account: ServiceAccount }) {
  const intl = useIntl();
  const [fresh, setFresh] = useState<ServiceAccount | null>(null);
  const [failed, setFailed] = useState(false);

  const load = useCallback(() => getServiceAccount(account.id).then(setFresh), [account.id]);

  useEffect(() => {
    load().catch(() => setFailed(true));
  }, [load]);

  // The page hands back the copy a regenerate returned; the form follows it.
  useEffect(() => {
    setFresh((current) => (current ? account : current));
  }, [account]);

  if (fresh) return <ServiceAccountForm {...rest} account={fresh} reload={load} />;
  return (
    <Dialog fullWidth maxWidth="sm" onClose={rest.onClose} open>
      <DialogContent>
        {failed ? (
          <Alert severity="error">{intl.formatMessage(messages.loadFailed)}</Alert>
        ) : (
          <Box sx={{ display: 'flex', justifyContent: 'center', py: 4 }}>
            <CircularProgress size={28} />
          </Box>
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
  roles: rolesState,
  onClose,
  onCreated,
  onSaved,
  onRegenerate,
  reload: fetchFresh,
}: ServiceAccountFormDialogProps & { reload?: () => Promise<unknown> }) {
  const intl = useIntl();
  const showSnackbar = useAIWorkspaceSnackbar();
  const isEdit = account !== null;
  const [pending, setPending] = useState(false);

  // What the edit is diffed against; replaced by a fresh copy after a 409.
  const [base, setBase] = useState<ServiceAccount | null>(account);
  const [conflict, setConflict] = useState(false);
  const [reloading, setReloading] = useState(false);

  const [displayName, setDisplayName] = useState(account?.displayName ?? '');
  const [handle, setHandle] = useState(account?.id ?? '');
  const [handleEdited, setHandleEdited] = useState(isEdit);
  const [handleTaken, setHandleTaken] = useState(false);
  const handleInput = useRef<HTMLInputElement>(null);
  const [description, setDescription] = useState(account?.description ?? '');
  const [roles, setRoles] = useState<string[]>(account?.roles ?? []);
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
  const removedRole = base !== null && base.roles.some((role) => !roles.includes(role));
  const disabling = base !== null && base.status === 'active' && !enabled;

  const textError = (field: TextField) => {
    const problem = problems[field];
    // "Required" waits for a submit; a too-long value shows while typing.
    if (!problem || (problem === messages.required && !submitted)) return null;
    return intl.formatMessage(problem, { max: MAX[field] });
  };

  // A new account starts with the default role; the admin may remove it.
  useEffect(() => {
    if (defaulted || rolesState.loading || rolesState.failed) return;
    setDefaulted(true);
    if (rolesState.list.some((role) => role.name === DEFAULT_ROLE)) setRoles([DEFAULT_ROLE]);
  }, [defaulted, rolesState]);

  // The server's copy moved: a Reload after a 409, or a regenerate from the
  // Credentials panel. Fields the admin has not touched follow it; edits are kept.
  useEffect(() => {
    if (!account || !base || account === base) return;
    if (displayName === base.displayName) setDisplayName(account.displayName);
    if (description === base.description) setDescription(account.description);
    if (sameRoles(roles, base.roles)) setRoles(account.roles);
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

  const reload = async () => {
    if (!fetchFresh) return;
    setReloading(true);
    try {
      await fetchFresh();
      setConflict(false);
    } catch (err) {
      showSnackbar(getErrorMessage(err, intl.formatMessage(messages.loadFailed)), 'error');
    } finally {
      setReloading(false);
    }
  };

  const fail = (err: unknown, fallback: MessageDescriptor) => {
    if (!isEdit && getErrorCode(err) === SERVICE_ACCOUNT_EXISTS) {
      setHandleTaken(true);
      return;
    }
    if (isEdit && getErrorCode(err) === 'CONFLICT') {
      setConflict(true);
      return;
    }
    showSnackbar(getErrorMessage(err, intl.formatMessage(fallback)), 'error');
  };

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    setSubmitted(true);
    if (!valid || pending) return;
    const name = displayName.trim();

    if (isEdit && base) {
      const body: UpdateServiceAccountRequest = {};
      if (name !== base.displayName) body.displayName = name;
      if (description.trim() !== base.description) body.description = description.trim();
      if (!sameRoles(roles, base.roles)) body.roles = roles;
      const status = enabled ? 'active' : 'disabled';
      if (status !== base.status) body.status = status;
      if (Object.keys(body).length === 0) {
        onClose();
        return;
      }
      setPending(true);
      try {
        await updateServiceAccount(base.id, body);
        showSnackbar(intl.formatMessage(messages.updated, { name }), 'success');
        onSaved();
        onClose();
      } catch (err) {
        fail(err, messages.updateFailed);
      } finally {
        setPending(false);
      }
      return;
    }

    setPending(true);
    try {
      const credentials = await createServiceAccount({
        displayName: name,
        id: effectiveHandle,
        roles,
        ...(description.trim() ? { description: description.trim() } : {}),
      });
      showSnackbar(intl.formatMessage(messages.created, { name }), 'success');
      onCreated(credentials);
    } catch (err) {
      fail(err, messages.createFailed);
    } finally {
      setPending(false);
    }
  };

  const handleError = handleTaken
    ? intl.formatMessage(messages.handleTaken)
    : (submitted || handleEdited || displayName.trim()) && problems.handle
      ? intl.formatMessage(HANDLE_PROBLEM[problems.handle], { max: HANDLE_MAX_LENGTH, min: HANDLE_MIN_LENGTH })
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
              <Typography color="text.secondary" component="span" sx={{ display: 'block' }} variant="body2">
                <FormattedMessage {...messages.createSubtitle} />
              </Typography>
            </Box>
            <IconButton aria-label={intl.formatMessage(messages.close)} disabled={pending} onClick={onClose} size="small">
              <X size={18} />
            </IconButton>
          </Stack>
        </DialogTitle>
      )}
      {/* The fields scroll; the header and footer stay put. */}
      <Box component="form" noValidate onSubmit={submit} sx={{ display: 'flex', flexDirection: 'column', minHeight: 0 }}>
        <DialogContent>
          <Stack spacing={2.5}>
            {conflict && (
              <Alert
                action={
                  <Button color="inherit" disabled={reloading} onClick={reload} size="small">
                    <FormattedMessage {...messages.reload} />
                  </Button>
                }
                severity="error"
              >
                <FormattedMessage {...messages.conflict} />
              </Alert>
            )}

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
              {textError('description') && <FormHelperText>{textError('description')}</FormHelperText>}
            </FormControl>

            <RolePicker
              failed={rolesState.failed}
              loading={rolesState.loading}
              onChange={setRoles}
              roles={rolesState.list}
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

            {isEdit && base && (
              <Stack spacing={1.5}>
                <SectionLabel>
                  <FormattedMessage {...messages.credentials} />
                </SectionLabel>
                <CredentialsPanel account={base} disabled={pending} onRegenerate={() => onRegenerate(base)} />
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
              control={<Switch checked={enabled} disabled={pending} onChange={(event) => setEnabled(event.target.checked)} />}
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
              disabled={pending || conflict || rolesState.loading || rolesState.failed || !valid}
              type="submit"
              variant="contained"
            >
              <FormattedMessage
                {...(isEdit
                  ? pending ? messages.saving : messages.save
                  : pending ? messages.creating : messages.create)}
              />
            </Button>
          </Stack>
        </DialogActions>
      </Box>
    </Dialog>
  );
}
