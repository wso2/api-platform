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

import React, { ReactNode, useEffect, useMemo, useState } from 'react';
import {
  Box,
  ComplexSelect,
  InputAdornment,
  ListSubheader,
  SxProps,
  TextField,
  Theme,
} from '@wso2/oxygen-ui';
import { Search } from '@wso2/oxygen-ui-icons-react';

const DEFAULT_MENU_MAX_HEIGHT = 320;

export type SearchableComplexSelectOption = {
  id: string;
  name: string;
  description?: string;
};

type Props<T extends SearchableComplexSelectOption> = {
  value: string;
  selectedOption: T | null;
  options: T[];
  label: string;
  onChange: (value: string) => void | Promise<void>;
  renderOptionContent: (option: T) => ReactNode;
  disabled?: boolean;
  loading?: boolean;
  error?: unknown;
  emptyMessage: string;
  errorMessage?: string;
  noResultsMessage?: string;
  loadingMessage?: string;
  searchPlaceholder?: string;
  menuMaxHeight?: number;
  sx?: SxProps<Theme>;
  getSearchText?: (option: T) => string;
  openOnFieldClick?: boolean;
  onFieldClick?: () => void;
  fieldClickAriaLabel?: string;
  dropdownClickAriaLabel?: string;
};

export default function SearchableComplexSelect<
  T extends SearchableComplexSelectOption,
>({
  value,
  selectedOption,
  options,
  label,
  onChange,
  renderOptionContent,
  disabled = false,
  loading = false,
  error,
  emptyMessage,
  errorMessage = 'Failed to load items',
  noResultsMessage = 'No matching results',
  loadingMessage = 'Loading...',
  searchPlaceholder = 'Search',
  menuMaxHeight = DEFAULT_MENU_MAX_HEIGHT,
  sx,
  getSearchText,
  openOnFieldClick = true,
  onFieldClick,
  fieldClickAriaLabel,
  dropdownClickAriaLabel,
}: Props<T>) {
  const [isOpen, setIsOpen] = useState(false);
  const [searchQuery, setSearchQuery] = useState('');

  const filteredOptions = useMemo(() => {
    const query = searchQuery.trim().toLowerCase();

    if (!query) {
      return options;
    }

    return options.filter((option) => {
      const haystack = getSearchText
        ? getSearchText(option)
        : [option.name, option.description, option.id].filter(Boolean).join(' ');

      return haystack.toLowerCase().includes(query);
    });
  }, [getSearchText, options, searchQuery]);

  const handleClose = () => {
    setIsOpen(false);
    setSearchQuery('');
  };

  useEffect(() => {
    if (!disabled) {
      return;
    }

    setIsOpen(false);
    setSearchQuery('');
  }, [disabled]);

  const canSearch = !loading && options.length > 0;
  const effectiveValue = loading ? '__loading__' : value;
  const isSelectedOptionFilteredOut =
    Boolean(value) &&
    Boolean(selectedOption) &&
    !filteredOptions.some((option) => option.id === value);
  const usesSplitTrigger = !disabled && (!openOnFieldClick || Boolean(onFieldClick));

  const selectNode = (
    <ComplexSelect
      value={effectiveValue}
      onChange={(event) => onChange(event.target.value as string)}
      onOpen={openOnFieldClick ? () => setIsOpen(true) : undefined}
      onClose={handleClose}
      open={isOpen}
      size="small"
      sx={sx}
      label={label}
      disabled={disabled}
      MenuProps={{
        autoFocus: false,
        disableAutoFocusItem: true,
        slotProps: {
          paper: {
            sx: {
              maxHeight: menuMaxHeight,
            },
          },
        },
      }}
    >
      {isSelectedOptionFilteredOut && selectedOption ? (
        <ComplexSelect.MenuItem value={selectedOption.id} sx={{ display: 'none' }} aria-hidden>
          {renderOptionContent(selectedOption)}
        </ComplexSelect.MenuItem>
      ) : null}

      {canSearch ? (
        <ListSubheader
          component="div"
          sx={{
            py: 1,
            px: 1,
            bgcolor: 'background.paper',
            borderBottom: '1px solid',
            borderColor: 'divider',
            zIndex: 1,
          }}
        >
          <TextField
            size="small"
            fullWidth
            autoFocus
            placeholder={searchPlaceholder}
            value={searchQuery}
            onChange={(event) => setSearchQuery(event.target.value)}
            onKeyDown={(event) => event.stopPropagation()}
            slotProps={{
              input: {
                startAdornment: (
                  <InputAdornment position="start">
                    <Search size={16} />
                  </InputAdornment>
                ),
              },
            }}
          />
        </ListSubheader>
      ) : null}

      {loading ? (
        <ComplexSelect.MenuItem value="__loading__" disabled>
          <ComplexSelect.MenuItem.Text primary={loadingMessage} />
        </ComplexSelect.MenuItem>
      ) : options.length === 0 ? (
        <ComplexSelect.MenuItem value="" disabled>
          <ComplexSelect.MenuItem.Text
            primary={error ? errorMessage : emptyMessage}
            secondary={error ? String(error) : undefined}
          />
        </ComplexSelect.MenuItem>
      ) : filteredOptions.length === 0 ? (
        <ComplexSelect.MenuItem value="__no_results__" disabled>
          <ComplexSelect.MenuItem.Text primary={noResultsMessage} />
        </ComplexSelect.MenuItem>
      ) : (
        filteredOptions.map((option) => (
          <ComplexSelect.MenuItem key={option.id} value={option.id}>
            {renderOptionContent(option)}
          </ComplexSelect.MenuItem>
        ))
      )}
    </ComplexSelect>
  );

  if (!usesSplitTrigger) {
    return selectNode;
  }

  return (
    <Box sx={{ position: 'relative', minWidth: 'fit-content' }}>
      {selectNode}
      <Box
        component="button"
        type="button"
        aria-label={fieldClickAriaLabel ?? label}
        onClick={() => onFieldClick?.()}
        sx={{
          position: 'absolute',
          top: 0,
          left: 0,
          right: 40,
          bottom: 0,
          border: 0,
          p: 0,
          m: 0,
          bgcolor: 'transparent',
          cursor: onFieldClick ? 'pointer' : 'default',
          borderRadius: 1,
          zIndex: 1,
        }}
      />
      <Box
        component="button"
        type="button"
        aria-label={dropdownClickAriaLabel ?? `Open ${label}`}
        onClick={() => setIsOpen(true)}
        sx={{
          position: 'absolute',
          top: 0,
          right: 0,
          width: 40,
          bottom: 0,
          border: 0,
          p: 0,
          m: 0,
          bgcolor: 'transparent',
          cursor: 'pointer',
          borderRadius: 1,
          zIndex: 1,
        }}
      />
    </Box>
  );
}
