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
  ButtonBase,
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
  Popover,
  Select,
  Stack,
  TextField,
  Typography,
} from '@wso2/oxygen-ui';
import {
  Activity,
  AlignLeft,
  ArrowLeftToLine,
  ArrowRightToLine,
  Braces,
  ChevronRight,
  Code,
  CornerDownRight,
  Crosshair,
  PencilLine,
  Plus,
  Search,
  Sigma,
  X,
} from '@wso2/oxygen-ui-icons-react';
import type { ReactNode } from 'react';
import { defineMessages, FormattedMessage, useIntl, type MessageDescriptor } from 'react-intl';

import { useCreateMeter, useUpdateMeter, type Meter, type MeterBody } from '@/api/resources/meters';
import { useNotifications } from '@/components/Notifications';
import { useTrackFeature } from '@/featureUsage';
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
type FilterFieldRow = {
  key: number;
  field: string;
  eventKey: string;
  /** Picked from our gateway's list: its event key is fixed. */
  fromTemplate?: boolean;
};

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
  customFilter: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.MeterFormDialog.customFilter',
    defaultMessage: 'Custom filter',
    description:
      'The last choice when adding a filter property: a property with an empty name and key to fill in.',
  },
  customFilterHint: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.MeterFormDialog.customFilterHint',
    defaultMessage: 'Filter on any attribute path',
    description: 'Under the Custom filter choice: it reads any event key the administrator types.',
  },
  groupMcp: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.MeterFormDialog.groupMcp',
    defaultMessage: 'MCP',
    description: 'Heading over the properties of requests to MCP servers. Keep "MCP".',
  },
  groupRequest: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.MeterFormDialog.groupRequest',
    defaultMessage: 'Request',
    description: 'Heading over the properties every request has, such as its status code.',
  },
  groupTokens: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.MeterFormDialog.groupTokens',
    defaultMessage: 'AI token usage',
    description: 'Heading over the token counts of AI API requests.',
  },
  noFiltersRequests: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.MeterFormDialog.noFiltersRequests',
    defaultMessage: 'No filters. The meter counts every request.',
    description: 'Shown in place of the filters of a Requests meter that has none.',
  },
  noFiltersTokens: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.MeterFormDialog.noFiltersTokens',
    defaultMessage: 'No filters. The meter adds up the tokens of every request.',
    description: 'Shown in place of the filters of a Tokens meter that has none.',
  },
  noTemplates: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.MeterFormDialog.noTemplates',
    defaultMessage: 'No templates match your search.',
    description: 'Shown in the list of filter properties when the search finds none.',
  },
  searchTemplates: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.MeterFormDialog.searchTemplates',
    defaultMessage: 'Search templates',
    description: 'Placeholder and accessible name of the search box over the filter properties.',
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
  onBlur: () => void;
  /** The input's id, for its label. */
  id: string;
};

/** How each group of properties is shown in the Add filter list: its heading and tint. */
const PRESET_GROUPS: readonly {
  group: PropertyPreset['group'];
  title: MessageDescriptor;
  color: string;
}[] = [
  { color: '#f0813a', group: 'request', title: messages.groupRequest },
  { color: '#8b9cf7', group: 'mcp', title: messages.groupMcp },
  { color: '#6fcf97', group: 'tokens', title: messages.groupTokens },
];

/** The icon each of our gateway's properties is listed with. */
const PRESET_ICONS: Record<string, ReactNode> = {
  'metadata.aiTokenUsage.completionTokens': <ArrowLeftToLine size={16} />,
  'metadata.aiTokenUsage.promptTokens': <ArrowRightToLine size={16} />,
  'metadata.aiTokenUsage.totalTokens': <Sigma size={16} />,
  'metadata.apiType': <Braces size={16} />,
  'metadata.mcpAnalytics.capabilityName': <PencilLine size={16} />,
  'metadata.mcpAnalytics.jsonRpcMethod': <Crosshair size={16} />,
  'request.route': <CornerDownRight size={16} />,
  'request.verb': <AlignLeft size={16} />,
  'response.status': <Activity size={16} />,
};

/** A small rounded square holding an icon, tinted with `color`. */
function IconTile({ children, color }: { children: ReactNode; color: string }) {
  return (
    <Box
      aria-hidden
      sx={{
        alignItems: 'center',
        bgcolor: `${color}26`,
        borderRadius: 1.5,
        color,
        display: 'flex',
        flexShrink: 0,
        height: 36,
        justifyContent: 'center',
        width: 36,
      }}
    >
      {children}
    </Box>
  );
}

const pickerItemSx = {
  alignItems: 'center',
  borderRadius: 1.5,
  display: 'flex',
  gap: 1.5,
  justifyContent: 'flex-start',
  px: 1,
  py: 0.75,
  textAlign: 'left',
  width: '100%',
} as const;

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
function TokenKeyInput({ value, onChange, onBlur, disabled, error, id }: TokenKeyInputProps) {
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
            htmlInput: { ...inputProps, id, onBlur, spellCheck: false },
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
  const trackFeature = useTrackFeature();
  const isEdit = meter !== null;
  const mutation = isEdit ? updateMutation : createMutation;

  // Stable keys for filter rows across add/remove.
  const nextRowKey = useRef(0);

  const [form, setForm] = useState<FormState>(emptyForm);
  const [submitted, setSubmitted] = useState(false);
  // The fields the user has left: their errors show from then on, not while they are first filled in.
  const [touched, setTouched] = useState<ReadonlySet<string>>(new Set());
  // The Add filter button while its list of properties is open.
  const [addAnchor, setAddAnchor] = useState<HTMLElement | null>(null);
  // What is typed in that list's search box.
  const [search, setSearch] = useState('');

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
    setTouched(new Set());
    setAddAnchor(null);
    setSearch('');
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
  const query = search.trim().toLowerCase();
  const matchingPresets = presets.filter(
    (preset) =>
      !query ||
      intl.formatMessage(preset.name).toLowerCase().includes(query) ||
      preset.eventKey.toLowerCase().includes(query),
  );
  const addFilterField = (field: string, eventKey: string, fromTemplate = false) => {
    setAddAnchor(null);
    setSearch('');
    setForm((current) => ({
      ...current,
      filterFields: [
        ...current.filterFields,
        { eventKey, field, fromTemplate, key: nextRowKey.current++ },
      ],
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
  const touch = (id: string) =>
    setTouched((current) => (current.has(id) ? current : new Set(current).add(id)));
  const shown = (id: string) => submitted || touched.has(id);
  // Each filter problem once under the filters, in the rows' order.
  const uniqueErrors = (errors: (RowError | undefined)[]) => [
    ...new Map(
      errors
        .filter((error): error is RowError => error !== undefined)
        .map((error) => [`${error.message.id}:${error.value ?? ''}`, error] as const),
    ).values(),
  ];
  const rowErrors = uniqueErrors(
    form.filterFields.flatMap((row) => [nameErrors.get(row.key), fieldKeyErrors.get(row.key)]),
  );
  // Those of the fields the user has left.
  const shownRowErrors = uniqueErrors(
    form.filterFields.flatMap((row) => [
      shown(`name:${row.key}`) ? nameErrors.get(row.key) : undefined,
      shown(`key:${row.key}`) ? fieldKeyErrors.get(row.key) : undefined,
    ]),
  );
  // Add is on once every required field is filled in and valid.
  const canSubmit =
    !mutation.isPending && !hasNameError && tokenKeyError === null && rowErrors.length === 0;

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
      trackFeature('apip.monetization.meter-update-click', {});
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
    trackFeature('apip.monetization.meter-create-click', {});
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
              <FormLabel
                htmlFor={NAME_FIELD}
                sx={{ '& .MuiFormLabel-asterisk': { color: 'primary.main' } }}
              >
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
              <FormControl error={shown('token') && tokenKeyError !== null} fullWidth>
                <FormLabel htmlFor={TOKEN_KEY_FIELD} required>
                  <FormattedMessage {...messages.tokenKeyLabel} />
                </FormLabel>
                <TokenKeyInput
                  disabled={readOnly}
                  error={shown('token') && tokenKeyError !== null}
                  onBlur={() => touch('token')}
                  id={TOKEN_KEY_FIELD}
                  onChange={(typeKey) => setForm((current) => ({ ...current, typeKey }))}
                  value={form.typeKey}
                />
                <FormHelperText>
                  {shown('token') && tokenKeyError ? <FormattedMessage {...tokenKeyError} /> : null}
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
              <Popover
                anchorEl={addAnchor}
                anchorOrigin={{ horizontal: 'right', vertical: 'bottom' }}
                onClose={() => {
                  setAddAnchor(null);
                  setSearch('');
                }}
                open={addAnchor !== null}
                slotProps={{
                  paper: {
                    sx: {
                      display: 'flex',
                      flexDirection: 'column',
                      maxHeight: 440,
                      mt: 0.5,
                      width: 360,
                    },
                  },
                }}
                transformOrigin={{ horizontal: 'right', vertical: 'top' }}
              >
                <Box sx={{ p: 1.5 }}>
                  <OutlinedInput
                    autoFocus
                    fullWidth
                    inputProps={{ 'aria-label': intl.formatMessage(messages.searchTemplates) }}
                    onChange={(event) => setSearch(event.target.value)}
                    placeholder={intl.formatMessage(messages.searchTemplates)}
                    size="small"
                    startAdornment={
                      <InputAdornment position="start">
                        <Search size={16} />
                      </InputAdornment>
                    }
                    value={search}
                  />
                </Box>
                <Divider />
                <Box role="menu" sx={{ flex: 1, overflowY: 'auto', px: 1, py: 1 }}>
                  {matchingPresets.length === 0 && (
                    <Typography color="text.secondary" sx={{ px: 1, py: 1.5 }} variant="body2">
                      <FormattedMessage {...messages.noTemplates} />
                    </Typography>
                  )}
                  {PRESET_GROUPS.map(({ color, group, title }) => {
                    const inGroup = matchingPresets.filter((preset) => preset.group === group);
                    if (inGroup.length === 0) return null;
                    return (
                      <Box key={group} role="group" sx={{ mb: 1 }}>
                        <Stack
                          alignItems="center"
                          direction="row"
                          spacing={1.5}
                          sx={{ px: 1, py: 1 }}
                        >
                          <Typography
                            color="text.secondary"
                            sx={{ fontWeight: 600, letterSpacing: 1, textTransform: 'uppercase' }}
                            variant="caption"
                          >
                            <FormattedMessage {...title} />
                          </Typography>
                          <Divider sx={{ flex: 1 }} />
                        </Stack>
                        {inGroup.map((preset) => (
                          <ButtonBase
                            key={preset.eventKey}
                            onClick={() =>
                              addFilterField(intl.formatMessage(preset.name), preset.eventKey, true)
                            }
                            role="menuitem"
                            sx={pickerItemSx}
                            title={preset.eventKey}
                          >
                            <IconTile color={color}>{PRESET_ICONS[preset.eventKey]}</IconTile>
                            <Typography variant="body1">
                              <FormattedMessage {...preset.name} />
                            </Typography>
                          </ButtonBase>
                        ))}
                      </Box>
                    );
                  })}
                </Box>
                <Divider />
                <Box sx={{ p: 1 }}>
                  <ButtonBase
                    onClick={() => addFilterField('', '')}
                    role="menuitem"
                    sx={pickerItemSx}
                  >
                    <Box
                      aria-hidden
                      sx={{
                        alignItems: 'center',
                        border: '1px dashed',
                        borderColor: 'text.disabled',
                        borderRadius: '50%',
                        color: 'text.secondary',
                        display: 'flex',
                        flexShrink: 0,
                        height: 44,
                        justifyContent: 'center',
                        width: 44,
                      }}
                    >
                      <Code size={16} />
                    </Box>
                    <Box sx={{ flex: 1 }}>
                      <Typography sx={{ fontWeight: 600 }} variant="body1">
                        <FormattedMessage {...messages.customFilter} />
                      </Typography>
                      <Typography color="text.secondary" variant="body2">
                        <FormattedMessage {...messages.customFilterHint} />
                      </Typography>
                    </Box>
                    <ChevronRight size={16} />
                  </ButtonBase>
                </Box>
              </Popover>
              {form.filterFields.length === 0 && (
                <Box
                  sx={{
                    border: '1px dashed',
                    borderColor: 'divider',
                    borderRadius: 1,
                    color: 'text.secondary',
                    px: 2,
                    py: 2,
                  }}
                >
                  <Typography color="inherit" variant="body2">
                    <FormattedMessage
                      {...(isTokens ? messages.noFiltersTokens : messages.noFiltersRequests)}
                    />
                  </Typography>
                </Box>
              )}
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
                    error={shown(`name:${row.key}`) && nameErrors.has(row.key)}
                    onBlur={() => touch(`name:${row.key}`)}
                    inputProps={{ 'aria-label': intl.formatMessage(messages.filterField) }}
                    onChange={(event) => setFilterRow(row.key, { field: event.target.value })}
                    size="small"
                    value={row.field}
                  />
                  <OutlinedInput
                    disabled={readOnly}
                    error={shown(`key:${row.key}`) && fieldKeyErrors.has(row.key)}
                    onBlur={() => touch(`key:${row.key}`)}
                    inputProps={{
                      'aria-label': intl.formatMessage(messages.eventKey),
                      readOnly: row.fromTemplate,
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
              {shownRowErrors.map((error) => (
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
