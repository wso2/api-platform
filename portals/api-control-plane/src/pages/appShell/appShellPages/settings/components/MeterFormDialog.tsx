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
  Autocomplete,
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
  MenuItem,
  OutlinedInput,
  Select,
  Stack,
  TextField,
  Typography,
} from '@wso2/oxygen-ui';
import { Plus, X } from '@wso2/oxygen-ui-icons-react';
import { defineMessages, FormattedMessage, useIntl, type MessageDescriptor } from 'react-intl';

import { useCreateMeter, useUpdateMeter, type Meter, type MeterBody } from '@/api/resources/meters';
import { useNotifications } from '@/components/Notifications';
import {
  DEFAULT_TOKEN_KEY,
  FILTER_FIELDS,
  FILTER_RULES,
  FIXED_FIELDS,
  MAX_METER_FIELDS,
  METER_TYPES,
  TOKEN_KEYS,
  fieldMessages,
  isEventKey,
  meterTypeMessages,
  tokenFieldLabel,
  type FilterField,
  type MeterType,
} from './meterLabels';

/** One field the meter's prices may filter on, as a row of the form, with the event key it reads. */
type FilterFieldRow = { key: number; field: FilterField; eventKey: string };

type FormState = {
  displayName: string;
  description: string;
  type: MeterType;
  /** Tokens only: the event key whose number each request adds up. */
  typeKey: string;
  /** The fields the meter's prices may filter on: each fixed field once, Token once per key. */
  filterFields: FilterFieldRow[];
};

const NAME_FIELD = 'meter-name';
const DESCRIPTION_FIELD = 'meter-description';
const TYPE_FIELD = 'meter-type';
const FIELDS_FIELD = 'meter-fields';
const TOKEN_KEY_FIELD = 'meter-token-key';
const LOCKED_NOTE = 'meter-locked-note';
/** Filter rows and their headings: property, key, remove. */
const FIELD_COLUMNS = 'minmax(0,1fr) minmax(0,1fr) 32px';

type TokenKeyOption = (typeof TOKEN_KEYS)[number];

/** What is wrong with a key, for one message under the fields; `key` names the key for the Token rules. */
type KeyError = { message: MessageDescriptor; key?: string };

const messages = defineMessages({
  addFilter: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.MeterFormDialog.addFilter',
    defaultMessage: 'Add filter',
    description: "Adds a row to a meter's filters: a property its prices may filter on.",
  },
  cancel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.MeterFormDialog.cancel',
    defaultMessage: 'Cancel',
  },
  close: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.MeterFormDialog.close',
    defaultMessage: 'Close',
  },
  created: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.MeterFormDialog.created',
    defaultMessage: 'Added meter "{name}".',
  },
  createFailed: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.MeterFormDialog.createFailed',
    defaultMessage: 'Failed to add the meter.',
  },
  descriptionLabel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.MeterFormDialog.descriptionLabel',
    defaultMessage: 'Description',
  },
  editTitle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.MeterFormDialog.editTitle',
    defaultMessage: 'Edit meter',
  },
  eventKey: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.MeterFormDialog.eventKey',
    defaultMessage: 'Key',
    description:
      'The field in the gateway\'s events that a Tokens meter adds up or a filter reads, such as "metadata.aiTokenUsage.promptTokens" or "request.verb".',
  },
  eventKeyInvalid: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.MeterFormDialog.eventKeyInvalid',
    defaultMessage: 'Use 1 to 255 characters with no spaces.',
    description: 'Shown under an event key that is too long or has spaces.',
  },
  tokenFieldKeyRequired: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.MeterFormDialog.tokenFieldKeyRequired',
    defaultMessage: 'A Token property needs an event key.',
    description:
      'Shown under a Token field row whose event key is empty; a Token field has no default key.',
  },
  tokenKeyIsTokenRow: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.MeterFormDialog.tokenKeyIsTokenRow',
    defaultMessage: "The key {key} is already the meter's Token key.",
    description:
      "Shown when a Token filter repeats a Tokens meter's Token key; {key} is the event key.",
  },
  tokenKeyRepeated: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.MeterFormDialog.tokenKeyRepeated',
    defaultMessage: 'The meter lists the Token key {key} more than once.',
    description:
      'Shown when two Token fields of a meter have the same event key; {key} is the event key.',
  },
  eventKeyRequired: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.MeterFormDialog.eventKeyRequired',
    defaultMessage: 'Enter the event key to add up.',
    description: 'Shown under the empty event key of a Tokens meter.',
  },
  filterField: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.MeterFormDialog.filterField',
    defaultMessage: 'Property',
    description:
      'Accessible name of the select of a filter row, whose value is the property, such as "HTTP method".',
  },
  filterHeading: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.MeterFormDialog.filterHeading',
    defaultMessage: 'Property',
    description:
      "Column heading over the property of each row of a meter's filters (Token, or a property its prices may filter on).",
  },
  filtersLabel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.MeterFormDialog.filtersLabel',
    defaultMessage: 'Filters',
    description:
      'Heading of the list of event properties a meter reads: for a Tokens meter the token count it adds up, then the properties its prices may filter on.',
  },
  lockedSave: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.MeterFormDialog.lockedSave',
    defaultMessage: 'There are plans using this meter.',
    description:
      'Shown next to the turned-off Save of a meter that plans price, which cannot be updated.',
  },
  nameLabel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.MeterFormDialog.nameLabel',
    defaultMessage: 'Name',
  },
  namePlaceholder: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.MeterFormDialog.namePlaceholder',
    defaultMessage: 'e.g. Requests',
  },
  nameRequired: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.MeterFormDialog.nameRequired',
    defaultMessage: 'Enter a name.',
  },
  removeFilter: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.MeterFormDialog.removeFilter',
    defaultMessage: 'Remove {field}',
    description:
      'Accessible name of the remove button of a filter row; {field} is the field, such as "HTTP method".',
  },
  saveChanges: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.MeterFormDialog.saveChanges',
    defaultMessage: 'Save changes',
  },
  saving: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.MeterFormDialog.saving',
    defaultMessage: 'Saving…',
  },
  tokenKeyLabel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.MeterFormDialog.tokenKeyLabel',
    defaultMessage: 'Token key',
    description:
      "Label of a Tokens meter's key: the event key whose number of tokens the meter adds up.",
  },

  submitCreate: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.MeterFormDialog.submitCreate',
    defaultMessage: 'Add Meter',
  },
  titleCreate: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.MeterFormDialog.titleCreate',
    defaultMessage: 'Add Meter',
  },
  typeLabel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.MeterFormDialog.typeLabel',
    defaultMessage: 'Type',
    description: 'What a meter counts: requests or tokens.',
  },
  updated: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.MeterFormDialog.updated',
    defaultMessage: 'Saved meter "{name}".',
  },
  updateFailed: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.MeterFormDialog.updateFailed',
    defaultMessage: 'Failed to save the meter.',
  },
});

const emptyForm = (): FormState => ({
  description: '',
  displayName: '',
  filterFields: [],
  type: 'REQUEST_COUNT',
  typeKey: DEFAULT_TOKEN_KEY,
});

type TokenKeyInputProps = {
  value: string;
  onChange: (key: string) => void;
  disabled: boolean;
  error: boolean;
  placeholder?: string;
  /** The input's id, for a label of its own; a filter row's input is named Key instead. */
  id?: string;
};

/**
 * A Tokens meter's Token key, or a Token filter's key: our gateway's token keys to
 * pick from, each with its label, or any other key typed in. Every suggestion
 * is offered whatever the field holds; there are only three.
 */
function TokenKeyInput({ value, onChange, disabled, error, placeholder, id }: TokenKeyInputProps) {
  const intl = useIntl();
  return (
    <Autocomplete<TokenKeyOption, false, true, true>
      disableClearable
      disabled={disabled}
      filterOptions={(options) => options}
      freeSolo
      getOptionLabel={(option) => (typeof option === 'string' ? option : option.key)}
      inputValue={value}
      isOptionEqualToValue={(option, selected) =>
        option.key === (typeof selected === 'string' ? selected : selected.key)
      }
      onChange={(_event, selected) =>
        onChange(typeof selected === 'string' ? selected : selected.key)
      }
      onInputChange={(_event, typed) => onChange(typed)}
      options={TOKEN_KEYS}
      renderInput={({ InputLabelProps, InputProps, inputProps, ...params }) => (
        <TextField
          {...params}
          error={error}
          placeholder={placeholder}
          required
          slotProps={{
            htmlInput: id
              ? { ...inputProps, id, spellCheck: false }
              : {
                  ...inputProps,
                  'aria-label': intl.formatMessage(messages.eventKey),
                  spellCheck: false,
                },
            input: { ...InputProps, sx: { fontFamily: 'monospace' } },
            inputLabel: InputLabelProps,
          }}
        />
      )}
      renderOption={({ key, ...optionProps }, option) => (
        <li key={key} {...optionProps}>
          <Stack>
            <Typography variant="body2">
              <FormattedMessage {...option.label} />
            </Typography>
            <Typography color="text.secondary" sx={{ fontFamily: 'monospace' }} variant="caption">
              {option.key}
            </Typography>
          </Stack>
        </li>
      )}
      size="small"
      value={value}
    />
  );
}

export type MeterFormDialogProps = {
  open: boolean;
  /** `null` opens the dialog in create mode; a meter opens it in edit mode. */
  meter: Meter | null;
  /** A meter plans price: every field is shown, none can change, and Save is off, saying why. */
  readOnly?: boolean;
  onClose: () => void;
};

/**
 * Add/edit dialog for a meter: what it counts, and its fields, each with the
 * event key it reads.
 *   - A Tokens meter has a Token key under its type: the event key whose number
 *     it adds up, sent as `typeKey`, never as a filter field.
 *   - The filter rows are the fields its plan prices may filter on: each fixed
 *     field once, and Token (a number such as a token count, which prices set
 *     conditions on) once per key.
 *   - Keys start as our gateway's defaults and can be edited. A row whose field
 *     changes gets that field's default, and a fixed field's key left empty gets
 *     its default from the Platform API. A Token field has no default: it needs a
 *     key of its own.
 *
 * Once a plan prices a meter, nothing about it can change, keys included, so it
 * opens read-only. The Platform API checks meters with the API Portal's rules,
 * and a refusal is shown with the server's reason.
 */
export function MeterFormDialog({ open, meter, readOnly = false, onClose }: MeterFormDialogProps) {
  const intl = useIntl();
  const { notify } = useNotifications();
  const createMutation = useCreateMeter();
  const updateMutation = useUpdateMeter();
  const isEdit = meter !== null;
  const mutation = isEdit ? updateMutation : createMutation;

  // Stable keys for filter rows across add/remove; a field can change within a row.
  const nextRowKey = useRef(0);

  const [form, setForm] = useState<FormState>(emptyForm);
  const [submitted, setSubmitted] = useState(false);

  useEffect(() => {
    if (!open) return;
    setForm(
      meter
        ? {
            description: meter.description ?? '',
            displayName: meter.displayName ?? '',
            filterFields: (meter.filterFields ?? []).map(({ eventKey, field }) => ({
              eventKey: eventKey ?? FILTER_RULES[field].eventKey,
              field,
              key: nextRowKey.current++,
            })),
            type: (meter.type ?? 'REQUEST_COUNT') as MeterType,
            typeKey: meter.typeKey ?? DEFAULT_TOKEN_KEY,
          }
        : emptyForm(),
    );
    setSubmitted(false);
  }, [open, meter]);

  // Switching to Tokens fills in the default Token key; switching away removes it.
  const changeType = (type: MeterType) =>
    setForm((current) => ({
      ...current,
      type,
      typeKey: type === 'TOKENS' && current.type !== 'TOKENS' ? DEFAULT_TOKEN_KEY : current.typeKey,
    }));

  // Each fixed field once; Token as often as wanted, each with its own key.
  const usedFields = new Set<FilterField>(
    form.filterFields.map((row) => row.field).filter((field) => field !== 'TOKEN'),
  );
  const addFilterField = () => {
    const field = FIXED_FIELDS.find((candidate) => !usedFields.has(candidate)) ?? 'TOKEN';
    setForm((current) => ({
      ...current,
      filterFields: [
        ...current.filterFields,
        { eventKey: FILTER_RULES[field].eventKey, field, key: nextRowKey.current++ },
      ],
    }));
  };
  // A row given another field reads that field's default key; Token has none.
  const setFilterField = (key: number, field: FilterField) =>
    setForm((current) => ({
      ...current,
      filterFields: current.filterFields.map((row) =>
        row.key === key ? { ...row, eventKey: FILTER_RULES[field].eventKey, field } : row,
      ),
    }));
  const setFilterKey = (key: number, eventKey: string) =>
    setForm((current) => ({
      ...current,
      filterFields: current.filterFields.map((row) =>
        row.key === key ? { ...row, eventKey } : row,
      ),
    }));
  const removeFilterField = (key: number) =>
    setForm((current) => ({
      ...current,
      filterFields: current.filterFields.filter((row) => row.key !== key),
    }));

  const trimmedName = form.displayName.trim();
  const hasNameError = trimmedName.length === 0;
  const isTokens = form.type === 'TOKENS';
  const tokenKey = form.typeKey.trim();
  const tokenKeyError = !isTokens
    ? null
    : !tokenKey
      ? messages.eventKeyRequired
      : !isEventKey(tokenKey)
        ? messages.eventKeyInvalid
        : null;
  // A fixed field's key left empty is not sent, and gets its default. A Token
  // field needs a key of its own: not the Token key, nor another Token field's.
  const tokenFieldKeys = form.filterFields
    .filter((row) => row.field === 'TOKEN')
    .map((row) => row.eventKey.trim());
  const keyErrorOf = (row: FilterFieldRow): KeyError | null => {
    const key = row.eventKey.trim();
    if (row.field !== 'TOKEN')
      return key && !isEventKey(key) ? { message: messages.eventKeyInvalid } : null;
    if (!key) return { message: messages.tokenFieldKeyRequired };
    if (!isEventKey(key)) return { message: messages.eventKeyInvalid };
    if (isTokens && key === tokenKey) return { key, message: messages.tokenKeyIsTokenRow };
    return tokenFieldKeys.filter((other) => other === key).length > 1
      ? { key, message: messages.tokenKeyRepeated }
      : null;
  };
  const fieldKeyErrors = new Map(
    form.filterFields.flatMap((row) => {
      const error = keyErrorOf(row);
      return error ? [[row.key, error] as const] : [];
    }),
  );
  // Each filter problem once under the filters.
  const keyErrors = [
    ...new Map(
      [...fieldKeyErrors.values()].map(
        (error: KeyError) => [`${error.message.id}:${error.key ?? ''}`, error] as const,
      ),
    ).values(),
  ];
  const canSubmit = !mutation.isPending;

  const handleSubmit = (event: FormEvent) => {
    event.preventDefault();
    setSubmitted(true);
    if (
      readOnly ||
      hasNameError ||
      tokenKeyError !== null ||
      keyErrors.length > 0 ||
      mutation.isPending
    )
      return;

    const body: MeterBody = {
      description: form.description.trim() || undefined,
      displayName: trimmedName,
      type: form.type,
      // The Token key is the meter's event key, not a filter field.
      ...(isTokens ? { typeKey: tokenKey } : {}),
      filterFields: form.filterFields.map(({ eventKey, field }) =>
        eventKey.trim() ? { eventKey: eventKey.trim(), field } : { field },
      ),
    };
    const onError = (error: { message?: string }) =>
      notify(
        error.message || intl.formatMessage(isEdit ? messages.updateFailed : messages.createFailed),
        'error',
      );

    if (isEdit && meter?.id) {
      updateMutation.mutate(
        { body, meterId: meter.id },
        {
          onError,
          onSuccess: () => {
            notify(intl.formatMessage(messages.updated, { name: trimmedName }), 'success');
            onClose();
          },
        },
      );
      return;
    }
    createMutation.mutate(body, {
      onError,
      onSuccess: () => {
        notify(intl.formatMessage(messages.created, { name: trimmedName }), 'success');
        onClose();
      },
    });
  };

  return (
    <Dialog fullWidth maxWidth="sm" onClose={mutation.isPending ? undefined : onClose} open={open}>
      <DialogTitle>
        <Stack alignItems="flex-start" direction="row" spacing={1.5}>
          <Box sx={{ flex: 1 }}>
            <FormattedMessage {...(isEdit ? messages.editTitle : messages.titleCreate)} />
          </Box>
          <IconButton
            aria-label={intl.formatMessage(messages.close)}
            disabled={mutation.isPending}
            onClick={onClose}
            size="small"
          >
            <X size={18} />
          </IconButton>
        </Stack>
      </DialogTitle>
      <Box component="form" noValidate onSubmit={handleSubmit}>
        <DialogContent>
          <Form.Stack spacing={3}>
            <FormControl error={submitted && hasNameError} fullWidth required>
              <FormLabel htmlFor={NAME_FIELD}>
                <FormattedMessage {...messages.nameLabel} />
              </FormLabel>
              <OutlinedInput
                autoFocus
                disabled={readOnly}
                id={NAME_FIELD}
                onChange={(event) =>
                  setForm((current) => ({ ...current, displayName: event.target.value }))
                }
                placeholder={intl.formatMessage(messages.namePlaceholder)}
                value={form.displayName}
              />
              <FormHelperText>
                {submitted && hasNameError ? <FormattedMessage {...messages.nameRequired} /> : null}
              </FormHelperText>
            </FormControl>

            <FormControl fullWidth>
              <FormLabel htmlFor={DESCRIPTION_FIELD}>
                <FormattedMessage {...messages.descriptionLabel} />
              </FormLabel>
              <OutlinedInput
                disabled={readOnly}
                id={DESCRIPTION_FIELD}
                multiline
                onChange={(event) =>
                  setForm((current) => ({ ...current, description: event.target.value }))
                }
                rows={2}
                value={form.description}
              />
            </FormControl>

            <FormControl fullWidth>
              <FormLabel id={`${TYPE_FIELD}-label`}>
                <FormattedMessage {...messages.typeLabel} />
              </FormLabel>
              <Select
                disabled={readOnly}
                labelId={`${TYPE_FIELD}-label`}
                onChange={(event) => changeType(event.target.value as MeterType)}
                size="small"
                value={form.type}
              >
                {METER_TYPES.map((type) => (
                  <MenuItem key={type} value={type}>
                    <FormattedMessage {...meterTypeMessages[type]} />
                  </MenuItem>
                ))}
              </Select>
            </FormControl>

            {isTokens && (
              <FormControl error={submitted && tokenKeyError !== null} fullWidth>
                <FormLabel htmlFor={TOKEN_KEY_FIELD} required>
                  <FormattedMessage {...messages.tokenKeyLabel} />
                </FormLabel>
                <TokenKeyInput
                  disabled={readOnly}
                  error={submitted && tokenKeyError !== null}
                  id={TOKEN_KEY_FIELD}
                  onChange={(typeKey) => setForm((current) => ({ ...current, typeKey }))}
                  placeholder={DEFAULT_TOKEN_KEY}
                  value={form.typeKey}
                />
                <FormHelperText>
                  {submitted && tokenKeyError ? <FormattedMessage {...tokenKeyError} /> : null}
                </FormHelperText>
              </FormControl>
            )}

            <Stack aria-labelledby={`${FIELDS_FIELD}-label`} role="group" spacing={1}>
              <Stack alignItems="center" direction="row" justifyContent="space-between" spacing={1}>
                <Form.Header
                  id={`${FIELDS_FIELD}-label`}
                  sx={{ fontWeight: 600, typography: 'body2' }}
                >
                  <FormattedMessage {...messages.filtersLabel} />
                </Form.Header>
                {!readOnly && form.filterFields.length < MAX_METER_FIELDS && (
                  <Button
                    onClick={addFilterField}
                    size="small"
                    startIcon={<Plus size={16} />}
                    sx={{ flexShrink: 0 }}
                    variant="text"
                  >
                    <FormattedMessage {...messages.addFilter} />
                  </Button>
                )}
              </Stack>
              {form.filterFields.length > 0 && (
                <Box
                  aria-hidden
                  sx={{ display: 'grid', gap: 1, gridTemplateColumns: FIELD_COLUMNS }}
                >
                  <Typography color="text.secondary" variant="caption">
                    <FormattedMessage {...messages.filterHeading} />
                  </Typography>
                  <Typography color="text.secondary" variant="caption">
                    <FormattedMessage {...messages.eventKey} />
                  </Typography>
                  <span />
                </Box>
              )}
              {form.filterFields.map((row) => (
                <Box
                  key={row.key}
                  sx={{
                    alignItems: 'center',
                    display: 'grid',
                    gap: 1,
                    gridTemplateColumns: FIELD_COLUMNS,
                  }}
                >
                  <Select
                    disabled={readOnly}
                    inputProps={{ 'aria-label': intl.formatMessage(messages.filterField) }}
                    onChange={(event) => setFilterField(row.key, event.target.value as FilterField)}
                    renderValue={(field) => intl.formatMessage(fieldMessages[field])}
                    size="small"
                    value={row.field}
                  >
                    {FILTER_FIELDS.map((field) => (
                      <MenuItem
                        disabled={field !== row.field && usedFields.has(field)}
                        key={field}
                        value={field}
                      >
                        <FormattedMessage {...fieldMessages[field]} />
                      </MenuItem>
                    ))}
                  </Select>
                  {row.field === 'TOKEN' ? (
                    <TokenKeyInput
                      disabled={readOnly}
                      error={submitted && fieldKeyErrors.has(row.key)}
                      onChange={(eventKey) => setFilterKey(row.key, eventKey)}
                      value={row.eventKey}
                    />
                  ) : (
                    <OutlinedInput
                      disabled={readOnly}
                      error={submitted && fieldKeyErrors.has(row.key)}
                      inputProps={{
                        'aria-label': intl.formatMessage(messages.eventKey),
                        spellCheck: false,
                      }}
                      onChange={(event) => setFilterKey(row.key, event.target.value)}
                      placeholder={FILTER_RULES[row.field].eventKey}
                      size="small"
                      sx={{ fontFamily: 'monospace' }}
                      value={row.eventKey}
                    />
                  )}
                  {readOnly ? (
                    <span />
                  ) : (
                    <IconButton
                      aria-label={intl.formatMessage(messages.removeFilter, {
                        field:
                          row.field === 'TOKEN' && row.eventKey.trim()
                            ? tokenFieldLabel(intl, row.eventKey.trim())
                            : intl.formatMessage(fieldMessages[row.field]),
                      })}
                      onClick={() => removeFilterField(row.key)}
                      size="small"
                    >
                      <X size={16} />
                    </IconButton>
                  )}
                </Box>
              ))}
              {submitted &&
                keyErrors.map((error) => (
                  <Typography
                    color="error"
                    key={`${error.message.id}:${error.key ?? ''}`}
                    variant="caption"
                  >
                    <FormattedMessage {...error.message} values={{ key: error.key }} />
                  </Typography>
                ))}
            </Stack>
          </Form.Stack>
        </DialogContent>
        <Divider />
        <DialogActions>
          {readOnly && (
            <Typography color="text.secondary" id={LOCKED_NOTE} sx={{ mr: 'auto' }} variant="body2">
              <FormattedMessage {...messages.lockedSave} />
            </Typography>
          )}
          <Button disabled={mutation.isPending} onClick={onClose} variant="outlined">
            <FormattedMessage {...messages.cancel} />
          </Button>
          <Button
            aria-describedby={readOnly ? LOCKED_NOTE : undefined}
            disabled={readOnly || !canSubmit}
            type="submit"
            variant="contained"
          >
            {mutation.isPending ? (
              <FormattedMessage {...messages.saving} />
            ) : (
              <FormattedMessage {...(isEdit ? messages.saveChanges : messages.submitCreate)} />
            )}
          </Button>
        </DialogActions>
      </Box>
    </Dialog>
  );
}
