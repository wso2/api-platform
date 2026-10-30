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

/**
 * Picks the format a proxy accepts from its clients.
 *
 * The list is the template catalogue, narrowed to what a proxy can actually be
 * built on: templates that are enabled, and only the newest of each family. A
 * disabled template is not offered because choosing it would build a proxy on
 * something withdrawn.
 *
 * The user sees display names; the proxy stores handles. The two are kept apart
 * deliberately — a display name can be edited without the template being
 * renamed, so storing one would eventually store something that resolves to
 * nothing.
 */

import React, { useMemo } from 'react';
import { FormattedMessage, useIntl } from 'react-intl';
import {
  Alert,
  FormControl,
  FormHelperText,
  FormLabel,
  MenuItem,
  Select,
} from '@wso2/oxygen-ui';
import type { ProviderTemplate } from '../../utils/types';

export type InboundInterfaceSelectProps = {
  /** The handle in effect. Empty only before one is known. */
  value: string;
  onChange: (templateHandle: string) => void;
  templates: ProviderTemplate[];
  isLoading?: boolean;
  error?: unknown;
  disabled?: boolean;
  /** Shown under the field. Callers phrase this for their own screen. */
  helperText?: React.ReactNode;
  'data-cyid'?: string;
};

/**
 * The templates a proxy can be built on.
 *
 * `enabled` is treated as opt-out: a template that does not say it is disabled
 * is offered, because the catalogue has carried templates without the flag.
 * Treating an absent flag as disabled would empty the list.
 */
export const selectableInterfaceTemplates = (
  templates: ProviderTemplate[]
): ProviderTemplate[] =>
  templates.filter(
    (template) => template.enabled !== false && template.isLatest !== false
  );

export default function InboundInterfaceSelect({
  value,
  onChange,
  templates,
  isLoading = false,
  error,
  disabled = false,
  helperText,
  'data-cyid': dataCyId = 'inbound-interface-select',
}: InboundInterfaceSelectProps) {
  const intl = useIntl();
  const options = useMemo(
    () => selectableInterfaceTemplates(templates),
    [templates]
  );

  // A stored template that is no longer offered — withdrawn, or superseded —
  // is still shown as the current value, and without a warning. The proxy works;
  // drawing attention to it would report a problem the user cannot act on and
  // did not cause.
  const storedButUnlisted =
    value !== '' && !options.some((template) => template.id === value);

  if (error) {
    return (
      <FormControl fullWidth>
        <FormLabel sx={{ mb: 0.5 }}>
          <FormattedMessage
            id="aiWorkspace.components.inboundInterface.label"
            defaultMessage="Inbound Interface"
          />
        </FormLabel>
        <Alert severity="warning" data-cyid={`${dataCyId}-error`}>
          <FormattedMessage
            id="aiWorkspace.components.inboundInterface.error"
            defaultMessage="Could not load interface templates. Your other entries are unaffected."
          />
        </Alert>
      </FormControl>
    );
  }

  // The label is a sibling of the field rather than wrapping it, so the two are
  // tied together by id. Without it a screen reader reaches the combobox with
  // no name at all and can only say that it is a combobox.
  const labelId = `${dataCyId}-label`;

  return (
    <FormControl fullWidth>
      <FormLabel id={labelId} sx={{ mb: 0.5 }}>
        <FormattedMessage
          id="aiWorkspace.components.inboundInterface.label"
          defaultMessage="Inbound Interface"
        />
      </FormLabel>
      <Select
        value={value}
        onChange={(event: { target: { value: unknown } }) =>
          onChange(String(event.target.value))
        }
        displayEmpty
        disabled={disabled || isLoading}
        labelId={labelId}
        data-cyid={dataCyId}
      >
        {/*
          A placeholder, not a choice: it stands in while the catalogue is
          still loading or a proxy predates the setting, and cannot be
          selected. Every selectable entry names a real interface.
        */}
        {value === '' && (
          <MenuItem value="" disabled>
            {intl.formatMessage({
              id: 'aiWorkspace.components.inboundInterface.placeholder',
              defaultMessage: 'Select an interface',
            })}
          </MenuItem>
        )}
        {isLoading ? (
          <MenuItem value="" disabled>
            <FormattedMessage
              id="aiWorkspace.components.inboundInterface.loading"
              defaultMessage="Loading interfaces..."
            />
          </MenuItem>
        ) : options.length === 0 ? (
          <MenuItem value="" disabled>
            <FormattedMessage
              id="aiWorkspace.components.inboundInterface.empty"
              defaultMessage="No interface templates available"
            />
          </MenuItem>
        ) : null}
        {storedButUnlisted && (
          <MenuItem value={value} data-cyid={`${dataCyId}-stored-option`}>
            {value}
          </MenuItem>
        )}
        {options.map((template) => (
          <MenuItem key={template.id} value={template.id}>
            {template.displayName || template.id}
          </MenuItem>
        ))}
      </Select>
      {helperText ? <FormHelperText>{helperText}</FormHelperText> : null}
    </FormControl>
  );
}
