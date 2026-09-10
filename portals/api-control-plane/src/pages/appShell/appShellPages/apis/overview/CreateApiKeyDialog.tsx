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

import {
  Alert,
  AlertTitle,
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
  MenuItem,
  OutlinedInput,
  Select,
  Stack,
  Tooltip,
  Typography,
  alpha,
} from '@wso2/oxygen-ui';
import {
  CalendarDays,
  Check,
  Copy,
  Eye,
  EyeOff,
  KeyRound,
  TriangleAlert,
  X,
} from '@wso2/oxygen-ui-icons-react';
import { useEffect, useState, type FormEvent } from 'react';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';

import { useCreateApiKey, type CreateApiKeyBody } from '@/api/resources/apiKeys';
import { useNotifications } from '@/components/Notifications';
import { useFormatters } from '@/i18n/useFormatters';

const messages = defineMessages({
  cancel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.CreateApiKeyDialog.action.cancel',
    defaultMessage: 'Cancel',
    description: 'Closes the dialog without issuing a key.',
  },
  close: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.CreateApiKeyDialog.action.close',
    defaultMessage: 'Close',
    description: 'Accessible name of the icon button that dismisses the dialog.',
  },
  copy: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.CreateApiKeyDialog.action.copy',
    defaultMessage: 'Copy',
    description: 'Copies the generated key to the clipboard.',
  },
  copied: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.CreateApiKeyDialog.action.copied',
    defaultMessage: 'Copied',
    description: 'Replaces the copy label once the key is on the clipboard.',
  },
  copyFailed: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.CreateApiKeyDialog.notification.copyFailed',
    defaultMessage: 'Could not copy the key. Select the value and copy it manually.',
    description: 'Shown when the browser refuses clipboard access.',
  },
  copySucceeded: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.CreateApiKeyDialog.notification.copySucceeded',
    defaultMessage: 'API key copied to the clipboard.',
  },
  create: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.CreateApiKeyDialog.action.create',
    defaultMessage: 'Create key',
    description: 'Submits the form and issues the key.',
  },
  createFailed: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.CreateApiKeyDialog.notification.createFailed',
    defaultMessage: 'Failed to create the API key.',
    description: 'Fallback toast when the server gives no reason for the failure.',
  },
  created: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.CreateApiKeyDialog.notification.created',
    defaultMessage: 'API key "{name}" created.',
    description: 'Toast confirming a new key; {name} is the name the user typed.',
  },
  creating: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.CreateApiKeyDialog.action.creating',
    defaultMessage: 'Creating…',
    description: 'Label of the submit button while the key is being issued.',
  },
  done: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.CreateApiKeyDialog.action.done',
    defaultMessage: 'I have copied the key',
    description: 'Dismisses the dialog showing the one-time key value.',
  },
  expiryDurationLabel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.CreateApiKeyDialog.expiry.durationLabel',
    defaultMessage: 'Expiry duration',
    description: 'Accessible name of the number field beside the time-unit selector.',
  },
  expiryInvalid: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.CreateApiKeyDialog.expiry.invalid',
    defaultMessage: 'Enter a whole number between 1 and {max}.',
    description: 'Validation error for the expiry duration field.',
  },
  expiryLabel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.CreateApiKeyDialog.expiry.label',
    defaultMessage: 'Expires in',
    description: 'Label of the field pair choosing how long the key stays valid.',
  },
  expiryPreview: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.CreateApiKeyDialog.expiry.preview',
    defaultMessage:
      'Expires on {date} ({unit, select, hours {{duration, plural, one {# hour} other {# hours}}} days {{duration, plural, one {# day} other {# days}}} weeks {{duration, plural, one {# week} other {# weeks}}} months {{duration, plural, one {# month} other {# months}}} other {{duration, plural, one {# day} other {# days}}}} from today)',
    description:
      'Restates the chosen duration as a calendar date. {date} is already formatted for the locale.',
  },
  expiryUnitLabel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.CreateApiKeyDialog.expiry.unitLabel',
    defaultMessage: 'Expiry time unit',
    description: 'Accessible name of the time-unit selector beside the duration field.',
  },
  hideKey: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.CreateApiKeyDialog.action.hideKey',
    defaultMessage: 'Hide key',
  },
  issuedExpiresLabel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.CreateApiKeyDialog.issued.expiresLabel',
    defaultMessage: 'Expires',
    description: 'Column heading above the new key’s expiry date. A noun, not a command.',
  },
  issuedKeyLabel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.CreateApiKeyDialog.issued.keyLabel',
    defaultMessage: 'API key',
    description: 'Label of the read-only field holding the generated key value.',
  },
  issuedNameLabel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.CreateApiKeyDialog.issued.nameLabel',
    defaultMessage: 'Key name',
    description: 'Column heading above the new key’s name. A noun, not a command.',
  },
  issuedSubtitle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.CreateApiKeyDialog.issued.subtitle',
    defaultMessage: 'Store it somewhere safe before you close this dialog.',
  },
  issuedTitle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.CreateApiKeyDialog.issued.title',
    defaultMessage: 'API key created',
  },
  issuedWarningBody: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.CreateApiKeyDialog.issued.warningBody',
    defaultMessage:
      'The key is stored hashed and pushed to the deployed gateways. Once you close this dialog it cannot be fetched or displayed again.',
  },
  issuedWarningTitle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.CreateApiKeyDialog.issued.warningTitle',
    defaultMessage: 'Copy this key now, it is shown only once',
  },
  nameHelper: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.CreateApiKeyDialog.name.helper',
    defaultMessage: 'Used to identify this key in the list. You cannot change it later.',
  },
  nameLabel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.CreateApiKeyDialog.name.label',
    defaultMessage: 'Key name',
    description: 'Label of the field naming the new key. A noun, not a command.',
  },
  namePlaceholder: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.CreateApiKeyDialog.name.placeholder',
    defaultMessage: 'Ex: Production key',
    description: 'Example name shown in the empty key-name field.',
  },
  nameRequired: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.CreateApiKeyDialog.name.required',
    defaultMessage: 'Enter a name for this key.',
  },
  nameTooLong: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.CreateApiKeyDialog.name.tooLong',
    defaultMessage: 'Name must be {max} characters or fewer.',
  },
  showKey: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.CreateApiKeyDialog.action.showKey',
    defaultMessage: 'Show key',
  },
  subtitle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.CreateApiKeyDialog.subtitle',
    defaultMessage: 'The key is generated on save and shown once.',
  },
  title: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.CreateApiKeyDialog.title',
    defaultMessage: 'Add API key',
  },
  unitDays: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.CreateApiKeyDialog.unit.days',
    defaultMessage: 'Days',
    description: 'Time-unit option. Plural noun, as it follows a number field.',
  },
  unitHours: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.CreateApiKeyDialog.unit.hours',
    defaultMessage: 'Hours',
    description: 'Time-unit option. Plural noun, as it follows a number field.',
  },
  unitMonths: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.CreateApiKeyDialog.unit.months',
    defaultMessage: 'Months',
    description: 'Time-unit option. Plural noun, as it follows a number field.',
  },
  unitWeeks: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.CreateApiKeyDialog.unit.weeks',
    defaultMessage: 'Weeks',
    description: 'Time-unit option. Plural noun, as it follows a number field.',
  },
});

/** Shared ids tying labels, inputs and helper text together. */
const NAME_FIELD = 'create-api-key-name';
const EXPIRY_FIELD = 'create-api-key-expiry-duration';
const KEY_FIELD = 'create-api-key-value';

const NAME_MAX = 120;

/** Bound on the duration field. Wide enough for "36 months", narrow enough that
 * a mistyped value can't ask the server for a key valid past any useful date. */
const DURATION_MAX = 3650;

/** Prefilled duration, matching the platform's usual key lifetime. */
const DEFAULT_DURATION = '90';

/** The subset of the spec's `TimeUnit` this dialog offers. Typed against the
 * generated request body, so dropping a unit from the spec fails the build here
 * rather than at runtime. */
type ExpiryUnit = NonNullable<CreateApiKeyBody['expiresIn']>['unit'];

const EXPIRY_UNITS = ['hours', 'days', 'weeks', 'months'] as const satisfies readonly ExpiryUnit[];

type OfferedUnit = (typeof EXPIRY_UNITS)[number];

const UNIT_LABELS: Record<OfferedUnit, typeof messages.unitDays> = {
  days: messages.unitDays,
  hours: messages.unitHours,
  months: messages.unitMonths,
  weeks: messages.unitWeeks,
};

/** The date a key created now with this duration would expire on. Calendar
 * arithmetic, not `duration * 86400000`: adding months has to land on the same
 * day-of-month, and adding days has to survive a DST transition. */
const expiryDate = (from: Date, duration: number, unit: OfferedUnit): Date => {
  const result = new Date(from.getTime());
  switch (unit) {
    case 'hours':
      result.setHours(result.getHours() + duration);
      break;
    case 'days':
      result.setDate(result.getDate() + duration);
      break;
    case 'weeks':
      result.setDate(result.getDate() + duration * 7);
      break;
    case 'months':
      result.setMonth(result.getMonth() + duration);
      break;
  }
  return result;
};

/** A whole number in `[1, DURATION_MAX]`, or `null` if the field isn't one yet. */
const parseDuration = (raw: string): number | null => {
  if (!/^\d+$/.test(raw.trim())) return null;
  const value = Number(raw.trim());
  return value >= 1 && value <= DURATION_MAX ? value : null;
};

/** What the server handed back, held only for as long as the dialog shows it. */
type IssuedKey = { apiKey: string; displayName: string; expiresAt: Date };

export type CreateApiKeyDialogProps = {
  open: boolean;
  restApiId: string;
  onClose: () => void;
};

/**
 * Issues an API key for a REST API.
 *
 * Two steps in one dialog, because the value only exists for the length of the
 * second one: the form asks for a name and a lifetime, the server generates the
 * key, and the response is the only place that plaintext will ever appear. That
 * is why nothing here writes it to storage and why the second step can't be
 * reopened — see `useCreateApiKey`.
 */
export function CreateApiKeyDialog({ open, restApiId, onClose }: CreateApiKeyDialogProps) {
  const intl = useIntl();
  const { shortDate } = useFormatters();
  const { notify } = useNotifications();
  const createMutation = useCreateApiKey();

  const [name, setName] = useState('');
  const [duration, setDuration] = useState(DEFAULT_DURATION);
  const [unit, setUnit] = useState<OfferedUnit>('days');
  // Errors stay quiet until a field has been visited or the form submitted, so
  // an untouched dialog doesn't open covered in red.
  const [nameTouched, setNameTouched] = useState(false);
  const [durationTouched, setDurationTouched] = useState(false);
  const [issued, setIssued] = useState<IssuedKey | null>(null);
  const [revealed, setRevealed] = useState(true);
  const [copied, setCopied] = useState(false);

  useEffect(() => {
    if (!open) return;
    setName('');
    setDuration(DEFAULT_DURATION);
    setUnit('days');
    setNameTouched(false);
    setDurationTouched(false);
    setIssued(null);
    setRevealed(true);
    setCopied(false);
  }, [open]);

  const trimmedName = name.trim();
  const nameError =
    trimmedName.length === 0
      ? intl.formatMessage(messages.nameRequired)
      : trimmedName.length > NAME_MAX
        ? intl.formatMessage(messages.nameTooLong, { max: NAME_MAX })
        : null;
  const parsedDuration = parseDuration(duration);
  const durationError =
    parsedDuration === null
      ? intl.formatMessage(messages.expiryInvalid, { max: DURATION_MAX })
      : null;

  const showNameError = nameTouched && nameError !== null;
  const showDurationError = durationTouched && durationError !== null;
  const canSubmit = nameError === null && durationError === null && !createMutation.isPending;

  const handleSubmit = (event: FormEvent) => {
    // A real submit, so Enter in the name field creates the key.
    event.preventDefault();
    setNameTouched(true);
    setDurationTouched(true);
    if (!canSubmit || parsedDuration === null) return;

    // The response carries no expiry, so the date shown next is the one this
    // request asked for.
    const expiresAt = expiryDate(new Date(), parsedDuration, unit);

    createMutation.mutate(
      {
        restApiId,
        body: { displayName: trimmedName, expiresIn: { duration: parsedDuration, unit } },
      },
      {
        onSuccess: (response) => {
          notify(intl.formatMessage(messages.created, { name: trimmedName }), 'success');
          if (!response.apiKey) {
            // `apiKey` is only returned for a server-generated key. Nothing to
            // reveal, so don't hold the user on a step with nothing to copy.
            onClose();
            return;
          }
          setIssued({ apiKey: response.apiKey, displayName: trimmedName, expiresAt });
        },
        onError: (error) =>
          notify(error.message || intl.formatMessage(messages.createFailed), 'error'),
      },
    );
  };

  const copyKey = () => {
    if (!issued) return;
    navigator.clipboard
      ?.writeText(issued.apiKey)
      .then(() => {
        setCopied(true);
        notify(intl.formatMessage(messages.copySucceeded), 'success');
      })
      .catch(() => notify(intl.formatMessage(messages.copyFailed), 'error'));
  };

  const header = (
    <DialogTitle sx={{ pb: 1.5 }}>
      <Stack alignItems="flex-start" direction="row" spacing={1.5}>
        <Box
          sx={(theme) => ({
            alignItems: 'center',
            bgcolor: alpha(theme.palette.primary.main, 0.12),
            borderRadius: 1,
            color: 'primary.main',
            display: 'flex',
            flexShrink: 0,
            height: 40,
            justifyContent: 'center',
            width: 40,
          })}
        >
          <KeyRound size={20} />
        </Box>
        <Box sx={{ flex: 1, minWidth: 0 }}>
          <Typography component="span" sx={{ display: 'block', fontWeight: 600 }} variant="h6">
            <FormattedMessage {...(issued ? messages.issuedTitle : messages.title)} />
          </Typography>
          <Typography
            color="text.secondary"
            component="span"
            sx={{ display: 'block' }}
            variant="body2"
          >
            <FormattedMessage {...(issued ? messages.issuedSubtitle : messages.subtitle)} />
          </Typography>
        </Box>
        <IconButton
          aria-label={intl.formatMessage(messages.close)}
          disabled={createMutation.isPending}
          onClick={onClose}
          size="small"
        >
          <X size={18} />
        </IconButton>
      </Stack>
    </DialogTitle>
  );

  return (
    <Dialog
      fullWidth
      maxWidth="sm"
      // Neither a backdrop click nor Escape may dismiss the issued key: it is
      // unrecoverable, so the only ways out are the explicit buttons.
      onClose={createMutation.isPending || issued ? undefined : onClose}
      open={open}
      slotProps={{ transition: { onExited: () => setIssued(null) } }}
    >
      {header}

      {issued ? (
        <>
          <DialogContent>
            <Stack spacing={2.5}>
              <Alert icon={<TriangleAlert size={18} />} severity="error">
                <AlertTitle sx={{ fontWeight: 600 }}>
                  <FormattedMessage {...messages.issuedWarningTitle} />
                </AlertTitle>
                <FormattedMessage {...messages.issuedWarningBody} />
              </Alert>

              <FormControl fullWidth>
                <FormLabel htmlFor={KEY_FIELD}>
                  <FormattedMessage {...messages.issuedKeyLabel} />
                </FormLabel>
                <OutlinedInput
                  endAdornment={
                    <InputAdornment position="end">
                      <Stack alignItems="center" direction="row" spacing={1}>
                        <Tooltip
                          arrow
                          title={intl.formatMessage(revealed ? messages.hideKey : messages.showKey)}
                        >
                          <IconButton
                            aria-label={intl.formatMessage(
                              revealed ? messages.hideKey : messages.showKey,
                            )}
                            onClick={() => setRevealed((current) => !current)}
                            size="small"
                          >
                            {revealed ? <EyeOff size={16} /> : <Eye size={16} />}
                          </IconButton>
                        </Tooltip>
                        <Tooltip
                          arrow
                          title={intl.formatMessage(copied ? messages.copied : messages.copy)}
                        >
                          <IconButton
                            aria-label={intl.formatMessage(
                              copied ? messages.copied : messages.copy,
                            )}
                            onClick={copyKey}
                            size="small"
                          >
                            {copied ? <Check size={16} /> : <Copy size={16} />}
                          </IconButton>
                        </Tooltip>
                      </Stack>
                    </InputAdornment>
                  }
                  fullWidth
                  id={KEY_FIELD}
                  inputProps={{ autoComplete: 'off', readOnly: true, spellCheck: false }}
                  type={revealed ? 'text' : 'password'}
                  value={issued.apiKey}
                />
              </FormControl>

              <Stack direction="row" spacing={5}>
                <Box sx={{ minWidth: 0 }}>
                  <Typography
                    color="text.secondary"
                    component="p"
                    variant="overline"
                    sx={{ opacity: 0.7 }}
                  >
                    <FormattedMessage {...messages.issuedNameLabel} />
                  </Typography>
                  <Typography variant="body2">{issued.displayName}</Typography>
                </Box>
                <Box sx={{ minWidth: 0 }}>
                  <Typography
                    color="text.secondary"
                    component="p"
                    variant="overline"
                    sx={{ opacity: 0.7 }}
                  >
                    <FormattedMessage {...messages.issuedExpiresLabel} />
                  </Typography>
                  <Typography variant="body2">{shortDate(issued.expiresAt)}</Typography>
                </Box>
              </Stack>
            </Stack>
          </DialogContent>
          <Divider />
          <DialogActions>
            <Button onClick={onClose} variant="contained">
              <FormattedMessage {...messages.done} />
            </Button>
          </DialogActions>
        </>
      ) : (
        <Box component="form" noValidate onSubmit={handleSubmit}>
          <DialogContent>
            <Form.Stack>
              <FormControl error={showNameError} fullWidth required>
                <FormLabel htmlFor={NAME_FIELD}>
                  <FormattedMessage {...messages.nameLabel} />
                </FormLabel>
                <OutlinedInput
                  aria-describedby={`${NAME_FIELD}-helper-text`}
                  autoFocus
                  id={NAME_FIELD}
                  inputProps={{ maxLength: NAME_MAX }}
                  name="displayName"
                  onBlur={() => setNameTouched(true)}
                  onChange={(event) => setName(event.target.value)}
                  placeholder={intl.formatMessage(messages.namePlaceholder)}
                  value={name}
                />
                <FormHelperText id={`${NAME_FIELD}-helper-text`}>
                  {showNameError ? nameError : <FormattedMessage {...messages.nameHelper} />}
                </FormHelperText>
              </FormControl>

              <FormControl error={showDurationError} fullWidth required>
                <FormLabel htmlFor={EXPIRY_FIELD}>
                  <FormattedMessage {...messages.expiryLabel} />
                </FormLabel>
                <Stack direction="row" spacing={2}>
                  <OutlinedInput
                    aria-describedby={`${EXPIRY_FIELD}-helper-text`}
                    id={EXPIRY_FIELD}
                    inputProps={{
                      'aria-label': intl.formatMessage(messages.expiryDurationLabel),
                      inputMode: 'numeric',
                      max: DURATION_MAX,
                      min: 1,
                      step: 1,
                    }}
                    name="expiresInDuration"
                    onBlur={() => setDurationTouched(true)}
                    onChange={(event) => setDuration(event.target.value)}
                    sx={{ flex: 1 }}
                    type="number"
                    value={duration}
                  />
                  <Select
                    inputProps={{ 'aria-label': intl.formatMessage(messages.expiryUnitLabel) }}
                    name="expiresInUnit"
                    onChange={(event) => setUnit(event.target.value as OfferedUnit)}
                    sx={{ flex: 1 }}
                    value={unit}
                  >
                    {EXPIRY_UNITS.map((candidate) => (
                      <MenuItem key={candidate} value={candidate}>
                        <FormattedMessage {...UNIT_LABELS[candidate]} />
                      </MenuItem>
                    ))}
                  </Select>
                </Stack>
                <FormHelperText id={`${EXPIRY_FIELD}-helper-text`}>
                  {showDurationError ? (
                    durationError
                  ) : parsedDuration !== null ? (
                    <Stack alignItems="center" component="span" direction="row" spacing={0.75}>
                      <CalendarDays size={14} />
                      <span>
                        <FormattedMessage
                          {...messages.expiryPreview}
                          values={{
                            date: shortDate(expiryDate(new Date(), parsedDuration, unit)),
                            duration: parsedDuration,
                            unit,
                          }}
                        />
                      </span>
                    </Stack>
                  ) : null}
                </FormHelperText>
              </FormControl>
            </Form.Stack>
          </DialogContent>
          <Divider />
          <DialogActions>
            <Button disabled={createMutation.isPending} onClick={onClose} variant="outlined">
              <FormattedMessage {...messages.cancel} />
            </Button>
            <Button disabled={!canSubmit} type="submit" variant="contained">
              {createMutation.isPending ? (
                <FormattedMessage {...messages.creating} />
              ) : (
                <FormattedMessage {...messages.create} />
              )}
            </Button>
          </DialogActions>
        </Box>
      )}
    </Dialog>
  );
}
