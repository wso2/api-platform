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
  Autocomplete,
  Avatar,
  Box,
  Chip,
  Divider,
  Form,
  FormControl,
  FormControlLabel,
  FormHelperText,
  FormLabel,
  Grid,
  IconButton,
  InputAdornment,
  OutlinedInput,
  Stack,
  Switch,
  TextField,
  Tooltip,
  Typography,
} from '@wso2/oxygen-ui';
import { Boxes, ChevronDown, Info, List, SquarePen, X } from '@wso2/oxygen-ui-icons-react';
import { useState, type MouseEvent } from 'react';
import { defineMessages, FormattedMessage, useIntl, type MessageDescriptor } from 'react-intl';

import { apiInitials } from '../../apis/utils/restApiDisplay';
import type { DraftFormField, DraftFormValues, FormFieldErrors } from '../utils/publicationForm';

const messages = defineMessages({
  nameLabel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.ApiDetailsTab.nameLabel',
    defaultMessage: 'Name',
  },
  versionLabel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.ApiDetailsTab.versionLabel',
    defaultMessage: 'Version',
  },
  descriptionLabel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.ApiDetailsTab.descriptionLabel',
    defaultMessage: 'Description',
  },
  descriptionPlaceholder: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.ApiDetailsTab.descriptionPlaceholder',
    defaultMessage: 'What this API does, in one or two sentences.',
  },
  productionUrlLabel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.ApiDetailsTab.productionUrlLabel',
    defaultMessage: 'Production URL',
  },
  sandboxUrlLabel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.ApiDetailsTab.sandboxUrlLabel',
    defaultMessage: 'Sandbox URL',
  },
  clearUrl: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.ApiDetailsTab.clearUrl',
    defaultMessage: 'Clear',
  },
  showGatewayUrls: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.ApiDetailsTab.showGatewayUrls',
    defaultMessage: 'Show gateway URLs',
  },
  enterCustomUrl: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.ApiDetailsTab.enterCustomUrl',
    defaultMessage: 'Enter a custom URL',
  },
  chooseGatewayUrl: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.ApiDetailsTab.chooseGatewayUrl',
    defaultMessage: 'Choose from gateway URLs',
  },
  agentVisibilityTitle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.ApiDetailsTab.agentVisibilityTitle',
    defaultMessage: 'Make this API discoverable by AI agents',
  },
  agentVisibilityDescription: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.ApiDetailsTab.agentVisibilityDescription',
    defaultMessage: 'Let agents in the API Portal discover and invoke this API on behalf of users.',
  },
});

const THUMBNAIL_WIDTH = 72;
/** Stands in for the Name field's height when the thumbnail sits on a row of its own. */
const THUMBNAIL_MIN_HEIGHT = 64;
const THUMBNAIL_FONT_SIZE = 28;
const THUMBNAIL_ICON_SIZE = 24;

/** A URL the Production URL can be picked from, and the gateway that serves it. */
export type GatewayUrlOption = { gatewayName: string; url: string };

export type ApiDetailsTabProps = {
  disabled?: boolean;
  errors?: FormFieldErrors;
  /** Shows the values without letting them be edited; `onBlurField` and `onChange` are then never called. */
  readOnly?: boolean;
  onBlurField?: (field: DraftFormField) => void;
  onChange?: (values: DraftFormValues) => void;
  /** URLs the Production URL can be picked from; any other URL can still be typed. */
  productionUrlOptions?: GatewayUrlOption[];
  values: DraftFormValues;
};

/** One line of helper text (MUI's caption size and line height) plus the gap above it. */
const ERROR_LINE_HEIGHT = 'calc(1.66 * 0.75rem + 3px)';

/** A field's validation message. Its line is always reserved, so a message appearing never moves what is below. */
function FieldError({ error, id }: { error?: MessageDescriptor; id: string }) {
  return (
    <FormHelperText id={`${id}-error`} sx={{ minHeight: '1.66em' }}>
      {error && <FormattedMessage {...error} />}
    </FormHelperText>
  );
}

type DetailFieldProps = {
  disabled?: boolean;
  error?: MessageDescriptor;
  field: DraftFormField;
  id: string;
  label: MessageDescriptor;
  onBlur?: (field: DraftFormField) => void;
  onChange: (field: DraftFormField, value: string) => void;
  readOnly?: boolean;
  required?: boolean;
  value: string;
};

/** A single-line text field with its label and validation message. */
function DetailField({ disabled, error, field, id, label, onBlur, onChange, readOnly, required, value }: DetailFieldProps) {
  return (
    <FormControl disabled={disabled} error={Boolean(error)} fullWidth required={required}>
      <FormLabel htmlFor={id}>
        <FormattedMessage {...label} />
      </FormLabel>
      <OutlinedInput
        aria-describedby={`${id}-error`}
        id={id}
        onBlur={() => onBlur?.(field)}
        onChange={(event) => onChange(field, event.target.value)}
        readOnly={readOnly}
        value={value}
      />
      <FieldError error={error} id={id} />
    </FormControl>
  );
}

type UrlFieldProps = Omit<DetailFieldProps, 'required'> & {
  /** URLs to pick from; any other URL can still be typed, and the field cleared. */
  options?: GatewayUrlOption[];
};

/**
 * A URL field: typed or cleared, and picked from `options` when there are any.
 * The toggle switches between picking a gateway URL and typing one; without
 * options (the Sandbox URL, for now) there is nothing to pick, so it changes
 * only the icon, keeping both URL fields alike.
 */
function UrlField({
  disabled,
  error,
  field,
  id,
  label,
  onBlur,
  onChange,
  options = [],
  readOnly,
  value,
}: UrlFieldProps) {
  const intl = useIntl();
  const [open, setOpen] = useState(false);
  const [custom, setCustom] = useState(false);
  const gatewayNames = new Map(options.map((option) => [option.url, option.gatewayName]));
  const hasOptions = options.length > 0;
  const pickable = hasOptions && !custom;
  const editable = !disabled && !readOnly;
  // Keeps focus in the input while a button inside it is pressed.
  const keepFocus = (event: MouseEvent) => event.preventDefault();

  const toggleLabel = intl.formatMessage(custom ? messages.chooseGatewayUrl : messages.enterCustomUrl);

  return (
    <FormControl disabled={disabled} error={Boolean(error)} fullWidth>
      <FormLabel htmlFor={id}>
        <FormattedMessage {...label} />
      </FormLabel>
      <Autocomplete
        disableClearable
        disabled={disabled}
        filterOptions={(urls) => urls}
        forcePopupIcon={false}
        freeSolo
        inputValue={value}
        onClose={() => setOpen(false)}
        onInputChange={(_event, next) => onChange(field, next)}
        onOpen={() => editable && setOpen(true)}
        open={open && pickable}
        options={options.map((option) => option.url)}
        readOnly={readOnly}
        renderInput={(params) => (
          <TextField
            {...params}
            error={Boolean(error)}
            inputProps={{ ...params.inputProps, 'aria-describedby': `${id}-error`, id }}
            InputProps={{
              ...params.InputProps,
              endAdornment: editable && (
                <InputAdornment position="end">
                  {value !== '' && (
                    <IconButton
                      aria-label={intl.formatMessage(messages.clearUrl)}
                      data-clear
                      onClick={() => onChange(field, '')}
                      onMouseDown={keepFocus}
                      size="small"
                    >
                      <X size={18} />
                    </IconButton>
                  )}
                  {!custom && (
                    <IconButton
                      aria-label={intl.formatMessage(messages.showGatewayUrls)}
                      onClick={() => setOpen((current) => !current)}
                      onMouseDown={keepFocus}
                      size="small"
                    >
                      <ChevronDown size={18} />
                    </IconButton>
                  )}
                  <Divider flexItem orientation="vertical" sx={{ mx: 0.5 }} />
                  <IconButton
                    aria-label={toggleLabel}
                    onClick={() => {
                      setCustom((current) => !current);
                      setOpen(false);
                    }}
                    onMouseDown={keepFocus}
                    size="small"
                    title={toggleLabel}
                  >
                    {custom ? <List size={16} /> : <SquarePen size={16} />}
                  </IconButton>
                </InputAdornment>
              ),
            }}
            onBlur={() => onBlur?.(field)}
            size="medium"
          />
        )}
        renderOption={({ key, ...props }, url) => (
          <Box component="li" key={key} {...props}>
            <Stack alignItems="center" direction="row" spacing={1.5} sx={{ minWidth: 0, width: '100%' }}>
              <Typography sx={{ flex: 1, fontFamily: 'monospace', minWidth: 0, overflowWrap: 'anywhere' }} variant="body2">
                {url}
              </Typography>
              <Chip label={gatewayNames.get(url)} size="small" variant="outlined" />
            </Stack>
          </Box>
        )}
        // The clear button shows only while the field is hovered or focused, so an idle field is just its value and two icons.
        sx={{ '&:not(:hover, :focus-within) [data-clear]': { visibility: 'hidden' } }}
        value={value}
      />
      <FieldError error={error} id={id} />
    </FormControl>
  );
}

/**
 * Placeholder for the API's portal thumbnail, drawn like the API Overview's
 * avatar. It is not editable, uploaded or saved.
 */
function ThumbnailPlaceholder({ displayName }: { displayName: string }) {
  return (
    <Avatar
      sx={{
        bgcolor: 'primary.light',
        color: 'primary.contrastText',
        alignSelf: 'stretch',
        fontSize: THUMBNAIL_FONT_SIZE,
        // Fills the row's height, label to input, less the reserved error line under the input.
        height: 'auto',
        mb: ERROR_LINE_HEIGHT,
        minHeight: THUMBNAIL_MIN_HEIGHT,
        width: THUMBNAIL_WIDTH,
      }}
      variant="rounded"
    >
      {apiInitials(displayName) || <Boxes size={THUMBNAIL_ICON_SIZE} />}
    </Avatar>
  );
}

/**
 * "API Details": thumbnail placeholder, name, version, description, the two
 * endpoint URLs and agent visibility in a single card. The document picker is
 * left out of this release.
 */
export function ApiDetailsTab({
  disabled,
  errors = {},
  onBlurField,
  onChange,
  productionUrlOptions = [],
  readOnly,
  values,
}: ApiDetailsTabProps) {
  const intl = useIntl();

  const setField = <K extends keyof DraftFormValues>(field: K, value: DraftFormValues[K]) =>
    onChange?.({ ...values, [field]: value });

  const fieldProps = (field: DraftFormField, id: string, label: MessageDescriptor) => ({
    disabled,
    error: errors[field],
    field,
    id,
    label,
    onBlur: onBlurField,
    onChange: setField,
    readOnly,
    value: values[field],
  });

  return (
    <Box component="section" sx={{ p: 3 }}>
      {/* The rows above and below a field's reserved error line need only a small gap of their own. */}
      <Form.Stack spacing={1}>
        <Grid container spacing={2}>
          <Grid size={{ md: 'auto', xs: 12 }} sx={{ display: 'flex' }}>
            <ThumbnailPlaceholder displayName={values.displayName} />
          </Grid>
          <Grid size={{ md: 'grow', xs: 12 }}>
            <DetailField {...fieldProps('displayName', 'publicationDisplayName', messages.nameLabel)} required />
          </Grid>
          <Grid size={{ md: 3, xs: 12 }}>
            <DetailField {...fieldProps('version', 'publicationVersion', messages.versionLabel)} required />
          </Grid>
        </Grid>

        <FormControl disabled={disabled} fullWidth sx={{ pb: 1 }}>
          <FormLabel htmlFor="publicationDescription">{intl.formatMessage(messages.descriptionLabel)}</FormLabel>
          <OutlinedInput
            id="publicationDescription"
            multiline
            onChange={(event) => setField('description', event.target.value)}
            placeholder={readOnly ? undefined : intl.formatMessage(messages.descriptionPlaceholder)}
            readOnly={readOnly}
            rows={3}
            value={values.description}
          />
        </FormControl>

        <Grid container spacing={2}>
          <Grid size={{ md: 6, xs: 12 }}>
            <UrlField
              {...fieldProps('productionUrl', 'publicationProductionUrl', messages.productionUrlLabel)}
              options={productionUrlOptions}
            />
          </Grid>
          <Grid size={{ md: 6, xs: 12 }}>
            <UrlField {...fieldProps('sandboxUrl', 'publicationSandboxUrl', messages.sandboxUrlLabel)} />
          </Grid>
        </Grid>

        <Stack alignItems="center" direction="row" spacing={1}>
          <FormControlLabel
            control={
              <Switch
                checked={values.agentVisibility === 'VISIBLE'}
                disabled={disabled || readOnly}
                onChange={(event) => setField('agentVisibility', event.target.checked ? 'VISIBLE' : 'HIDDEN')}
              />
            }
            label={<FormattedMessage {...messages.agentVisibilityTitle} />}
            sx={{ m: 0 }}
          />
          <Tooltip title={intl.formatMessage(messages.agentVisibilityDescription)}>
            <Box sx={{ color: 'text.secondary', display: 'flex' }}>
              <Info size={16} />
            </Box>
          </Tooltip>
        </Stack>
      </Form.Stack>
    </Box>
  );
}
