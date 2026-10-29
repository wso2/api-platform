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
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import { useRef, useState, type ChangeEvent, type MouseEvent } from 'react';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';
import {
  alpha,
  Autocomplete,
  Avatar,
  Box,
  Chip,
  Divider,
  FormControl,
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
import { Boxes, Camera, ChevronDown, Info, List, SquarePen, X } from '@wso2/oxygen-ui-icons-react';

import { nameInitials, type DraftFormField, type DraftFormValues, type GatewayUrlOption } from './mcpPublishForm';

const messages = defineMessages({
  chooseGatewayUrl: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.details.chooseGatewayUrl',
    defaultMessage: 'Choose from gateway URLs',
  },
  customUrl: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.details.customUrl',
    defaultMessage: 'Enter a custom URL',
  },
  clear: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.details.clear',
    defaultMessage: 'Clear',
  },
  showGatewayUrls: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.details.showGatewayUrls',
    defaultMessage: 'Show gateway URLs',
  },
  name: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.details.name',
    defaultMessage: 'Name',
  },
  version: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.details.version',
    defaultMessage: 'Version',
  },
  descriptionPlaceholder: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.details.descriptionPlaceholder',
    defaultMessage: 'What this MCP proxy does, in one or two sentences.',
  },
  productionUrl: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.details.productionUrl',
    defaultMessage: 'Production URL',
  },
  sandboxUrl: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.details.sandboxUrl',
    defaultMessage: 'Sandbox URL',
  },
  agentVisibilityHint: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.details.agentVisibilityHint',
    defaultMessage: 'Let agents in the MCP Hub discover and invoke this MCP proxy on behalf of users.',
  },
  description: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.details.description',
    defaultMessage: 'Description',
  },
  changeThumbnail: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.details.changeThumbnail',
    defaultMessage: 'Change thumbnail',
  },
  agentVisibility: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.details.agentVisibility',
    defaultMessage: 'Make this MCP proxy discoverable by AI agents',
  },
});

const THUMBNAIL_WIDTH = 72;
/** Stands in for the Name field's height when the thumbnail sits on a row of its own. */
const THUMBNAIL_MIN_HEIGHT = 64;
const THUMBNAIL_FONT_SIZE = 28;
const THUMBNAIL_ICON_SIZE = 24;

type FieldErrorText = Partial<Record<DraftFormField, string>>;

export type McpProxyDetailsTabProps = {
  disabled?: boolean;
  errors?: FieldErrorText;
  readOnly?: boolean;
  onBlurField?: (field: DraftFormField) => void;
  onChange?: (values: DraftFormValues) => void;
  /** Without it the thumbnail is read-only. */
  onThumbnailPick?: (file: File) => void;
  /** URLs the Production URL can be picked from; any other can still be typed. */
  productionUrlOptions?: GatewayUrlOption[];
  /** The name's monogram stands in without one. */
  thumbnailUrl?: string;
  values: DraftFormValues;
};

/** One line of helper text (MUI's caption size and line height) plus the gap above it. */
const ERROR_LINE_HEIGHT = 'calc(1.66 * 0.75rem + 3px)';

/** Its line is always reserved, so a message appearing never moves what is below. */
function FieldError({ error, id }: { error?: string; id: string }) {
  return (
    <FormHelperText id={`${id}-error`} sx={{ minHeight: '1.66em' }}>
      {error}
    </FormHelperText>
  );
}

type DetailFieldProps = {
  disabled?: boolean;
  error?: string;
  field: DraftFormField;
  id: string;
  label: string;
  onBlur?: (field: DraftFormField) => void;
  onChange: (field: DraftFormField, value: string) => void;
  readOnly?: boolean;
  required?: boolean;
  value: string;
};

function DetailField({ disabled, error, field, id, label, onBlur, onChange, readOnly, required, value }: DetailFieldProps) {
  return (
    <FormControl disabled={disabled} error={Boolean(error)} fullWidth required={required}>
      <FormLabel htmlFor={id}>{label}</FormLabel>
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
  options?: GatewayUrlOption[];
};

/**
 * A URL typed, cleared or picked from `options`. Without options (the Sandbox URL) the
 * pick/type toggle changes only its icon, so both URL fields look alike.
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
  const toggleLabel = custom ? intl.formatMessage(messages.chooseGatewayUrl) : intl.formatMessage(messages.customUrl);

  return (
    <FormControl disabled={disabled} error={Boolean(error)} fullWidth>
      <FormLabel htmlFor={id}>{label}</FormLabel>
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
                      aria-label={intl.formatMessage(messages.clear)}
                      data-clear
                      onClick={() => onChange(field, '')}
                      onMouseDown={keepFocus}
                      size="small"
                    >
                      <X size={18} />
                    </IconButton>
                  )}
                  {pickable && (
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
        // The clear button shows only while the field is hovered or focused.
        sx={{ '&:not(:hover, :focus-within) [data-clear]': { visibility: 'hidden' } }}
        value={value}
      />
      <FieldError error={error} id={id} />
    </FormControl>
  );
}

type ThumbnailProps = {
  disabled?: boolean;
  displayName: string;
  onPick?: (file: File) => void;
  url?: string;
};

/** The listing's thumbnail, or the name's monogram. With `onPick`, hovering it offers a replacement. */
function Thumbnail({ disabled, displayName, onPick, url }: ThumbnailProps) {
  const intl = useIntl();
  const inputRef = useRef<HTMLInputElement>(null);
  const editable = Boolean(onPick) && !disabled;

  const handleFile = (event: ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0];
    // Cleared so choosing the same file again still fires a change.
    event.target.value = '';
    if (file) onPick?.(file);
  };

  return (
    <Box
      sx={{
        alignSelf: 'stretch',
        // Fills the row's height, label to input, less the reserved error line under the input.
        display: 'flex',
        mb: ERROR_LINE_HEIGHT,
        minHeight: THUMBNAIL_MIN_HEIGHT,
        position: 'relative',
        width: THUMBNAIL_WIDTH,
        '&:hover .thumbnail-overlay, &:focus-within .thumbnail-overlay': { opacity: 1 },
      }}
    >
      <Avatar
        alt={displayName}
        imgProps={{ style: { objectFit: 'contain' } }}
        src={url}
        sx={{
          bgcolor: url ? 'background.paper' : 'primary.light',
          color: 'primary.contrastText',
          fontSize: THUMBNAIL_FONT_SIZE,
          height: 'auto',
          width: '100%',
        }}
        variant="rounded"
      >
        {nameInitials(displayName) || <Boxes size={THUMBNAIL_ICON_SIZE} />}
      </Avatar>
      {editable && (
        <>
          <Box
            className="thumbnail-overlay"
            sx={(theme) => ({
              alignItems: 'center',
              bgcolor: alpha(theme.palette.common.black, 0.5),
              borderRadius: 1,
              display: 'flex',
              inset: 0,
              justifyContent: 'center',
              opacity: 0,
              position: 'absolute',
              transition: 'opacity 150ms ease',
            })}
          >
            <IconButton
              aria-label={intl.formatMessage(messages.changeThumbnail)}
              onClick={() => inputRef.current?.click()}
              size="small"
              sx={{ color: 'common.white' }}
            >
              <Camera size={THUMBNAIL_ICON_SIZE} />
            </IconButton>
          </Box>
          <Box
            accept="image/png,image/jpeg"
            component="input"
            onChange={handleFile}
            ref={inputRef}
            sx={{ display: 'none' }}
            type="file"
          />
        </>
      )}
    </Box>
  );
}

export function McpProxyDetailsTab({
  disabled,
  errors = {},
  onBlurField,
  onChange,
  onThumbnailPick,
  productionUrlOptions = [],
  readOnly,
  thumbnailUrl,
  values,
}: McpProxyDetailsTabProps) {
  const intl = useIntl();

  const setField = <K extends keyof DraftFormValues>(field: K, value: DraftFormValues[K]) =>
    onChange?.({ ...values, [field]: value });

  const fieldProps = (field: DraftFormField, id: string, label: string) => ({
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
      <Stack spacing={1}>
        <Grid container spacing={2}>
          <Grid size={{ md: 'auto', xs: 12 }} sx={{ display: 'flex' }}>
            <Thumbnail
              disabled={disabled}
              displayName={values.displayName}
              onPick={readOnly ? undefined : onThumbnailPick}
              url={thumbnailUrl}
            />
          </Grid>
          <Grid size={{ md: 'grow', xs: 12 }}>
            <DetailField {...fieldProps('displayName', 'publicationDisplayName', intl.formatMessage(messages.name))} required />
          </Grid>
          <Grid size={{ md: 3, xs: 12 }}>
            <DetailField {...fieldProps('version', 'publicationVersion', intl.formatMessage(messages.version))} required />
          </Grid>
        </Grid>
        <FormControl disabled={disabled} fullWidth sx={{ pb: 1 }}>
          <FormLabel htmlFor="publicationDescription">
            <FormattedMessage {...messages.description} />
          </FormLabel>
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
              {...fieldProps('productionUrl', 'publicationProductionUrl', intl.formatMessage(messages.productionUrl))}
              options={productionUrlOptions}
            />
          </Grid>
          <Grid size={{ md: 6, xs: 12 }}>
            <UrlField {...fieldProps('sandboxUrl', 'publicationSandboxUrl', intl.formatMessage(messages.sandboxUrl))} />
          </Grid>
        </Grid>
        <Stack alignItems="center" direction="row" spacing={1}>
          <Typography
            color={disabled || readOnly ? 'text.disabled' : 'text.primary'}
            component="label"
            htmlFor="publicationAgentVisibility"
          >
            <FormattedMessage {...messages.agentVisibility} />
          </Typography>
          <Tooltip title={intl.formatMessage(messages.agentVisibilityHint)}>
            <Box sx={{ color: 'text.secondary', display: 'flex' }}>
              <Info size={16} />
            </Box>
          </Tooltip>
          <Switch
            checked={values.agentVisibility === 'VISIBLE'}
            disabled={disabled || readOnly}
            id="publicationAgentVisibility"
            onChange={(event) => setField('agentVisibility', event.target.checked ? 'VISIBLE' : 'HIDDEN')}
          />
        </Stack>
      </Stack>
    </Box>
  );
}
