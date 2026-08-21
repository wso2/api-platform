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

import React, { useEffect, useState } from 'react';
import { Autocomplete, Box, CircularProgress, IconButton, TextField, Typography } from '@wso2/oxygen-ui';
import { Eye, EyeOff, KeyRound } from '@wso2/oxygen-ui-icons-react';
import { buildSecretPlaceholder, extractSecretHandle, listSecrets, type SecretMetadata } from '../../apis/secretApis';

type SecretOptionOrText = SecretMetadata | string;

export interface SecretValueFieldProps {
  value: string;
  onChange: (value: string) => void;
  onFocus?: React.FocusEventHandler<HTMLInputElement>;
  onBlur?: React.FocusEventHandler<HTMLInputElement>;
  disabled?: boolean;
  placeholder?: string;
  required?: boolean;
  error?: boolean;
  helperText?: React.ReactNode;
  size?: 'small' | 'medium';
  'data-cyid'?: string;
  'data-testid'?: string;
}

function getOptionLabel(option: SecretOptionOrText): string {
  return typeof option === 'string' ? option : option.displayName;
}

/**
 * Combobox for an upstream credential's Value field — same free-entry-plus-picker
 * affordance as GitHub's label picker, applied to secrets: existing secrets are
 * offered in a dropdown (selecting one substitutes {{ secret "handle" }}), or the
 * user can type a brand-new plaintext value directly (encrypted into a new secret
 * by the caller on submit, same as before) or a {{ secret "handle" }} placeholder
 * by hand, which is passed through unchanged.
 */
export default function SecretValueField({
  value,
  onChange,
  onFocus,
  onBlur,
  disabled,
  placeholder,
  required,
  error,
  helperText,
  size,
  'data-cyid': dataCyId,
  'data-testid': dataTestId,
}: SecretValueFieldProps): React.JSX.Element {
  const [options, setOptions] = useState<SecretMetadata[]>([]);
  const [isLoadingOptions, setIsLoadingOptions] = useState(true);
  const [showValue, setShowValue] = useState(false);
  const [isInputFocused, setIsInputFocused] = useState(false);

  useEffect(() => {
    let isMounted = true;
    (async () => {
      try {
        const response = await listSecrets({ limit: 100 });
        if (!isMounted) return;
        setOptions((response.list ?? []).filter((s) => s.status === 'ACTIVE' && s.type === 'GENERIC'));
      } catch {
        // Best-effort: a dropdown that fails to load still lets the user type a
        // value directly — this field never blocks on the existing-secrets list.
      } finally {
        if (isMounted) setIsLoadingOptions(false);
      }
    })();
    return () => {
      isMounted = false;
    };
  }, []);

  const selectedHandle = extractSecretHandle(value);
  const selectedOption = selectedHandle ? (options.find((s) => s.id === selectedHandle) ?? null) : null;
  const autocompleteValue: SecretOptionOrText = selectedOption ?? value;

  // Pre-filtered rather than left to MUI's own filterOptions, so `open` below can
  // stay false when nothing matches — an open-but-empty popup would otherwise sit
  // on top of (and swallow clicks meant for) whatever follows this field on the page.
  const query = value.trim().toLowerCase();
  const visibleOptions = selectedOption
    ? []
    : query
      ? options.filter((opt) => `${opt.displayName} ${opt.id}`.toLowerCase().includes(query))
      : options;
  const isOpen = isInputFocused && visibleOptions.length > 0;

  return (
    <Autocomplete<SecretOptionOrText, false, false, true>
      freeSolo
      fullWidth
      size={size}
      disabled={disabled}
      open={isOpen}
      options={visibleOptions}
      filterOptions={(opts) => opts}
      loading={isLoadingOptions}
      value={autocompleteValue}
      getOptionLabel={getOptionLabel}
      isOptionEqualToValue={(option, val) => typeof val !== 'string' && typeof option !== 'string' && option.id === val.id}
      onChange={(_event, newValue) => {
        if (newValue === null) {
          onChange('');
        } else if (typeof newValue === 'string') {
          onChange(newValue);
        } else {
          onChange(buildSecretPlaceholder(newValue.id));
        }
      }}
      onInputChange={(_event, newInputValue, reason) => {
        // 'reset' fires on selection/programmatic value changes — already handled by onChange.
        if (reason === 'input') {
          onChange(newInputValue);
        }
      }}
      renderOption={(props, option) => {
        if (typeof option === 'string') return null;
        return (
          <Box component="li" {...props} key={option.id} sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
            <KeyRound size={14} style={{ opacity: 0.6, flexShrink: 0 }} />
            <Box sx={{ minWidth: 0 }}>
              <Typography variant="body2" noWrap>
                {option.displayName}
              </Typography>
              <Typography variant="caption" color="text.secondary" sx={{ fontFamily: 'monospace' }} noWrap>
                {option.id}
              </Typography>
            </Box>
          </Box>
        );
      }}
      renderInput={(params) => (
        <TextField
          {...params}
          required={required}
          error={error}
          helperText={helperText}
          placeholder={placeholder}
          data-cyid={dataCyId}
          type={showValue ? 'text' : 'password'}
          slotProps={{
            htmlInput: {
              ...params.inputProps,
              autoComplete: 'off',
              'data-testid': dataTestId,
              onFocus: (event: React.FocusEvent<HTMLInputElement>) => {
                params.inputProps.onFocus?.(event);
                setIsInputFocused(true);
                onFocus?.(event);
              },
              onBlur: (event: React.FocusEvent<HTMLInputElement>) => {
                params.inputProps.onBlur?.(event);
                setIsInputFocused(false);
                onBlur?.(event);
              },
            },
            input: {
              ...params.InputProps,
              endAdornment: (
                <>
                  {isLoadingOptions ? <CircularProgress size={16} sx={{ mr: 1 }} /> : null}
                  <IconButton
                    size="small"
                    onClick={() => setShowValue((prev) => !prev)}
                    aria-label={showValue ? 'Hide value' : 'Show value'}
                  >
                    {showValue ? <EyeOff size={18} /> : <Eye size={18} />}
                  </IconButton>
                  {params.InputProps.endAdornment}
                </>
              ),
            },
          }}
        />
      )}
    />
  );
}
