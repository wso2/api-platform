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
  Menu,
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
  MAX_METER_FIELDS,
  METER_TYPES,
  PROPERTY_PRESETS,
  TOKEN_KEYS,
  isEventKey,
  isPropertyName,
  meterTypeMessages,
  sameName,
  type MeterType,
  type PropertyPreset,
} from './meterLabels';

/** One property the meter's prices may filter on, as a row of the form: its name and the event key it reads. */
type FilterFieldRow = { key: number; field: string; eventKey: string };

type FormState = {
  displayName: string;
  description: string;
  type: MeterType;
  /** Tokens only: the event key whose number each request adds up. */
  typeKey: string;
  /** The properties the meter's prices may filter on, in order. */
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

/**
 * What is wrong with a property's name or key, for one message under the filters; `value` is the
 * repeated name or key the message names.
 */
type RowError = { message: MessageDescriptor; value?: string };

const messages = defineMessages({
  addFilter: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.MeterFormDialog.addFilter',
    defaultMessage: 'Add filter',
    description:
      "Opens the list of properties to add to a meter's filters: our gateway's, then Custom.",
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
  custom: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.MeterFormDialog.custom',
    defaultMessage: 'Custom',
    description:
      'The last choice when adding a filter property: a property with an empty name and key to fill in.',
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
      'The field in the gateway\'s events that a filter property reads, such as "request.verb".',
  },
  eventKeyInvalid: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.MeterFormDialog.eventKeyInvalid',
    defaultMessage: 'Use 1 to 255 characters with no spaces.',
    description: 'Shown under an event key that is too long or has spaces.',
  },
  eventKeyRepeated: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.MeterFormDialog.eventKeyRepeated',
    defaultMessage: 'More than one property reads the event key {value}.',
    description:
      'Shown under the filters of a meter; {value} is the event key, such as "request.verb".',
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
      'Accessible name of the name of a filter row: the name of a property, such as "HTTP method".',
  },
  filterHeading: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.MeterFormDialog.filterHeading',
    defaultMessage: 'Property',
    description:
      'Column heading over the name of each property in a meter\'s filters, such as "HTTP method".',
  },
  filtersLabel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.MeterFormDialog.filtersLabel',
    defaultMessage: 'Filters',
    description: "Heading of the list of properties a meter's prices may filter on.",
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
  propertyKeyRequired: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.MeterFormDialog.propertyKeyRequired',
    defaultMessage: 'Enter the event key of each property.',
    description: "Shown under a meter's filters when a property's event key is empty.",
  },
  propertyNameInvalid: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.MeterFormDialog.propertyNameInvalid',
    defaultMessage: 'Give each property a name of 1 to 40 characters.',
    description: "Shown under a meter's filters when a property's name is empty or too long.",
  },
  propertyNameRepeated: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.MeterFormDialog.propertyNameRepeated',
    defaultMessage: 'The meter has the property {value} more than once.',
    description:
      'Shown under the filters of a meter; {value} is the name, such as "Region", which counts once whatever its case.',
  },
  removeFilter: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.MeterFormDialog.removeFilter',
    defaultMessage: 'Remove {field}',
    description:
      'Accessible name of the remove button of a filter row; {field} is the property, such as "HTTP method".',
  },
  removeUnnamed: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.MeterFormDialog.removeUnnamed',
    defaultMessage: 'Remove this property',
    description: 'Accessible name of the remove button of a filter row whose name is empty.',
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
  /** The input's id, for its label. */
  id: string;
};

/** A property preset as offered: its name, and the event key it reads under it. */
function PresetLabel({ preset }: { preset: PropertyPreset }) {
  return (
    <Stack>
      <Typography variant="body2">
        <FormattedMessage {...preset.name} />
      </Typography>
      <Typography color="text.secondary" sx={{ fontFamily: 'monospace' }} variant="caption">
        {preset.eventKey}
      </Typography>
    </Stack>
  );
}

/**
 * A Tokens meter's Token key: our gateway's token keys to pick from, each with
 * its label, or any other key typed in.
 */
function TokenKeyInput({ value, onChange, disabled, error, id }: TokenKeyInputProps) {
  return (
    <Autocomplete<PropertyPreset, false, true, true>
      disableClearable
      disabled={disabled}
      filterOptions={(options) => options}
      freeSolo
      getOptionLabel={(option) => (typeof option === 'string' ? option : option.eventKey)}
      inputValue={value}
      isOptionEqualToValue={(option, selected) =>
        option.eventKey === (typeof selected === 'string' ? selected : selected.eventKey)
      }
      onChange={(_event, selected) =>
        onChange(typeof selected === 'string' ? selected : selected.eventKey)
      }
      onInputChange={(_event, typed) => onChange(typed)}
      options={TOKEN_KEYS}
      renderInput={({ InputLabelProps, InputProps, inputProps, ...params }) => (
        <TextField
          {...params}
          error={error}
          placeholder={DEFAULT_TOKEN_KEY}
          required
          slotProps={{
            htmlInput: { ...inputProps, id, spellCheck: false },
            input: { ...InputProps, sx: { fontFamily: 'monospace' } },
            inputLabel: InputLabelProps,
          }}
        />
      )}
      renderOption={({ key, ...optionProps }, option) => (
        <li key={key} {...optionProps}>
          <PresetLabel preset={option} />
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
 * Add/edit dialog for a meter: what it counts, and the properties its plan
 * prices may filter on, each with the event key it reads.
 *   - A Tokens meter has a Token key under its type: the event key whose number
 *     it adds up, sent as `typeKey`, not as a filter property.
 *   - Each filter row is a property: a name, unique ignoring case, and an event
 *     key, unique too. A property added from our gateway's list starts with its
 *     name and key, and both can be edited; a custom one starts empty. A
 *     property our gateway reports is not offered again once the meter has its
 *     name or its key.
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

  // Stable keys for filter rows across add/remove.
  const nextRowKey = useRef(0);

  const [form, setForm] = useState<FormState>(emptyForm);
  const [submitted, setSubmitted] = useState(false);
  // The Add filter button while its list of properties is open.
  const [addAnchor, setAddAnchor] = useState<HTMLElement | null>(null);

  useEffect(() => {
    if (!open) return;
    setForm(
      meter
        ? {
            description: meter.description ?? '',
            displayName: meter.displayName ?? '',
            filterFields: (meter.filterFields ?? []).map(({ eventKey, field }) => ({
              eventKey,
              field,
              key: nextRowKey.current++,
            })),
            type: (meter.type ?? 'REQUEST_COUNT') as MeterType,
            typeKey: meter.typeKey ?? DEFAULT_TOKEN_KEY,
          }
        : emptyForm(),
    );
    setSubmitted(false);
    setAddAnchor(null);
  }, [open, meter]);

  // Switching to Tokens fills in the default Token key; switching away removes it.
  const changeType = (type: MeterType) =>
    setForm((current) => ({
      ...current,
      type,
      typeKey: type === 'TOKENS' && current.type !== 'TOKENS' ? DEFAULT_TOKEN_KEY : current.typeKey,
    }));

  // Our gateway's properties the meter does not have yet, by name or by key.
  const presets = PROPERTY_PRESETS.filter(
    (preset) =>
      !form.filterFields.some(
        (row) =>
          sameName(row.field, intl.formatMessage(preset.name)) ||
          row.eventKey.trim() === preset.eventKey,
      ),
  );
  const addFilterField = (field: string, eventKey: string) => {
    setAddAnchor(null);
    setForm((current) => ({
      ...current,
      filterFields: [...current.filterFields, { eventKey, field, key: nextRowKey.current++ }],
    }));
  };
  const setFilterRow = (key: number, patch: Partial<FilterFieldRow>) =>
    setForm((current) => ({
      ...current,
      filterFields: current.filterFields.map((row) =>
        row.key === key ? { ...row, ...patch } : row,
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
  // Each property needs a name and an event key, neither shared with another property; a name is
  // the same whatever its case.
  const nameErrorOf = (row: FilterFieldRow): RowError | null => {
    if (!isPropertyName(row.field)) return { message: messages.propertyNameInvalid };
    const same = form.filterFields.filter((other) => sameName(other.field, row.field));
    return same.length > 1
      ? { message: messages.propertyNameRepeated, value: same[0].field.trim() }
      : null;
  };
  const keyErrorOf = (row: FilterFieldRow): RowError | null => {
    const key = row.eventKey.trim();
    if (!key) return { message: messages.propertyKeyRequired };
    if (!isEventKey(key)) return { message: messages.eventKeyInvalid };
    return form.filterFields.filter((other) => other.eventKey.trim() === key).length > 1
      ? { message: messages.eventKeyRepeated, value: key }
      : null;
  };
  const nameErrors = new Map(
    form.filterFields.flatMap((row) => {
      const error = nameErrorOf(row);
      return error ? [[row.key, error] as const] : [];
    }),
  );
  const fieldKeyErrors = new Map(
    form.filterFields.flatMap((row) => {
      const error = keyErrorOf(row);
      return error ? [[row.key, error] as const] : [];
    }),
  );
  // Each filter problem once under the filters, in the rows' order.
  const rowErrors = [
    ...new Map(
      form.filterFields
        .flatMap((row) => [nameErrors.get(row.key), fieldKeyErrors.get(row.key)])
        .filter((error): error is RowError => error !== undefined)
        .map((error) => [`${error.message.id}:${error.value ?? ''}`, error] as const),
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
      rowErrors.length > 0 ||
      mutation.isPending
    )
      return;

    const body: MeterBody = {
      description: form.description.trim() || undefined,
      displayName: trimmedName,
      type: form.type,
      // The Token key is the meter's event key, not a filter field.
      ...(isTokens ? { typeKey: tokenKey } : {}),
      filterFields: form.filterFields.map(({ eventKey, field }) => ({
        eventKey: eventKey.trim(),
        field: field.trim(),
      })),
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
                    aria-expanded={addAnchor !== null}
                    aria-haspopup="menu"
                    onClick={(event) => setAddAnchor(event.currentTarget)}
                    size="small"
                    startIcon={<Plus size={16} />}
                    sx={{ flexShrink: 0 }}
                    variant="text"
                  >
                    <FormattedMessage {...messages.addFilter} />
                  </Button>
                )}
              </Stack>
              <Menu
                anchorEl={addAnchor}
                onClose={() => setAddAnchor(null)}
                open={addAnchor !== null}
              >
                {presets.map((preset) => (
                  <MenuItem
                    key={preset.eventKey}
                    onClick={() => addFilterField(intl.formatMessage(preset.name), preset.eventKey)}
                  >
                    <PresetLabel preset={preset} />
                  </MenuItem>
                ))}
                {presets.length > 0 && <Divider />}
                <MenuItem onClick={() => addFilterField('', '')}>
                  <FormattedMessage {...messages.custom} />
                </MenuItem>
              </Menu>
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
                  <OutlinedInput
                    disabled={readOnly}
                    error={submitted && nameErrors.has(row.key)}
                    inputProps={{ 'aria-label': intl.formatMessage(messages.filterField) }}
                    onChange={(event) => setFilterRow(row.key, { field: event.target.value })}
                    size="small"
                    value={row.field}
                  />
                  <OutlinedInput
                    disabled={readOnly}
                    error={submitted && fieldKeyErrors.has(row.key)}
                    inputProps={{
                      'aria-label': intl.formatMessage(messages.eventKey),
                      spellCheck: false,
                    }}
                    onChange={(event) => setFilterRow(row.key, { eventKey: event.target.value })}
                    size="small"
                    sx={{ fontFamily: 'monospace' }}
                    value={row.eventKey}
                  />
                  {readOnly ? (
                    <span />
                  ) : (
                    <IconButton
                      aria-label={
                        row.field.trim()
                          ? intl.formatMessage(messages.removeFilter, { field: row.field.trim() })
                          : intl.formatMessage(messages.removeUnnamed)
                      }
                      onClick={() => removeFilterField(row.key)}
                      size="small"
                    >
                      <X size={16} />
                    </IconButton>
                  )}
                </Box>
              ))}
              {submitted &&
                rowErrors.map((error) => (
                  <Typography
                    color="error"
                    key={`${error.message.id}:${error.value ?? ''}`}
                    variant="caption"
                  >
                    <FormattedMessage {...error.message} values={{ value: error.value }} />
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
