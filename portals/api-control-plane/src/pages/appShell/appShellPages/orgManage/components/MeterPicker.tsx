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

import { Autocomplete, TextField } from '@wso2/oxygen-ui';
import { useState } from 'react';
import { defineMessages, useIntl } from 'react-intl';

import { useMeters, type Meter } from '@/api/resources/meters';
import { useDebouncedValue } from '@/hooks/useDebouncedValue';

// How many meters the picker offers for what is typed: the Platform API searches, so any number of
// meters can be reached by typing part of a name or id.
const SUGGESTIONS = 20;
const SEARCH_DEBOUNCE_MS = 300;

const messages = defineMessages({
  loading: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.MeterPicker.loading',
    defaultMessage: 'Loading meters',
  },
  noMatches: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.MeterPicker.noMatches',
    defaultMessage: 'No matching meters',
  },
  placeholder: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.MeterPicker.placeholder',
    defaultMessage: 'Search meters',
    description: 'Placeholder of the meter field of a usage price, before a meter is picked.',
  },
});

export type MeterPickerProps = {
  /** The price's meter id, '' before one is picked. */
  meterId: string;
  /** The price's meter, once it has been read. */
  meter: Meter | undefined;
  /** The id of the field's label. */
  labelId: string;
  disabled?: boolean;
  error?: boolean;
  onChange: (meter: Meter) => void;
};

/** The meter a usage price charges for, found by typing part of its name or id. */
export function MeterPicker({
  meterId,
  meter,
  labelId,
  disabled,
  error,
  onChange,
}: MeterPickerProps) {
  const intl = useIntl();
  const [text, setText] = useState('');
  const query = useDebouncedValue(text.trim(), SEARCH_DEBOUNCE_MS);
  const metersQuery = useMeters({ limit: SUGGESTIONS, query: query || undefined });

  // A picked meter not read yet shows its id; it is offered too, so the field always has its value.
  const value: Meter | null = meter ?? (meterId ? { displayName: meterId, id: meterId } : null);
  const found = metersQuery.data?.list ?? [];
  const options =
    value && !found.some((candidate) => candidate.id === value.id) ? [value, ...found] : found;

  return (
    <Autocomplete<Meter, false, false, false>
      disabled={disabled}
      // The Platform API has already searched: every meter it returns is offered.
      filterOptions={(all) => all}
      getOptionLabel={(option) => option.displayName ?? option.id ?? ''}
      isOptionEqualToValue={(option, selected) => option.id === selected.id}
      loading={metersQuery.isFetching}
      loadingText={intl.formatMessage(messages.loading)}
      noOptionsText={intl.formatMessage(messages.noMatches)}
      onChange={(_event, picked) => {
        if (picked) onChange(picked);
      }}
      // What is typed searches; showing the picked meter's name, or clearing, starts from all
      // meters.
      onInputChange={(_event, typed, reason) => setText(reason === 'input' ? typed : '')}
      options={options}
      renderInput={({ InputLabelProps, InputProps, inputProps, ...params }) => (
        <TextField
          {...params}
          error={error}
          placeholder={intl.formatMessage(messages.placeholder)}
          slotProps={{
            htmlInput: { ...inputProps, 'aria-labelledby': labelId },
            input: InputProps,
            inputLabel: InputLabelProps,
          }}
        />
      )}
      size="small"
      value={value}
    />
  );
}
