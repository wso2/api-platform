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
  Box,
  Checkbox,
  Chip,
  FormControl,
  FormHelperText,
  FormLabel,
  ListItemText,
  MenuItem,
  Select,
  Stack,
} from '@wso2/oxygen-ui';
import type { ReactNode } from 'react';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';

import type { ServiceAccountRole } from '@/api/resources/serviceAccounts';

const messages = defineMessages({
  label: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.components.RolePicker.label',
    defaultMessage: 'Roles',
  },
  helper: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.components.RolePicker.helper',
    defaultMessage: 'Only service account roles (the ap_sa_* roles) are listed.',
  },
  loading: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.components.RolePicker.loading',
    defaultMessage: 'Loading roles…',
  },
  loadFailed: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.components.RolePicker.loadFailed',
    defaultMessage: 'Could not load the roles. Close the dialog and try again.',
  },
  none: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.components.RolePicker.none',
    defaultMessage:
      'No service-account roles are defined. Add an ap_sa_ role to the role-to-scope mapping on the server.',
    description: 'Shown when the server lists no roles. "ap_sa_" is a literal role-name prefix — do not translate.',
  },
  required: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.components.RolePicker.required',
    defaultMessage: 'Select at least one role.',
  },
  scopeCount: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.components.RolePicker.scopeCount',
    defaultMessage: '{count, plural, one {# scope} other {# scopes}}',
  },
});

const FIELD_ID = 'service-account-roles';

export type RolePickerProps = {
  roles: readonly ServiceAccountRole[];
  value: string[];
  onChange: (next: string[]) => void;
  loading?: boolean;
  failed?: boolean;
  showRequiredError?: boolean;
  /** Extra guidance under the field, e.g. what removing a role does. */
  warning?: ReactNode;
};

/** Multi-select of the server's ap_sa_* roles, each showing the scopes it grants. */
export function RolePicker({
  roles,
  value,
  onChange,
  loading = false,
  failed = false,
  showRequiredError = false,
  warning,
}: RolePickerProps) {
  const intl = useIntl();
  const unavailable = loading || failed || roles.length === 0;

  const helper = failed
    ? intl.formatMessage(messages.loadFailed)
    : loading
      ? intl.formatMessage(messages.loading)
      : roles.length === 0
        ? intl.formatMessage(messages.none)
        : showRequiredError
          ? intl.formatMessage(messages.required)
          : intl.formatMessage(messages.helper);

  return (
    <FormControl error={failed || (!loading && roles.length === 0) || showRequiredError} fullWidth required>
      <FormLabel id={`${FIELD_ID}-label`}>
        <FormattedMessage {...messages.label} />
      </FormLabel>
      <Select
        disabled={unavailable}
        labelId={`${FIELD_ID}-label`}
        multiple
        onChange={(event) => {
          const next = event.target.value;
          onChange(typeof next === 'string' ? next.split(',') : next);
        }}
        renderValue={(selected) => (
          <Stack direction="row" flexWrap="wrap" gap={0.5}>
            {selected.map((name) => (
              <Chip key={name} label={name} size="small" />
            ))}
          </Stack>
        )}
        value={value}
      >
        {roles.map((role) => (
          <MenuItem key={role.name} title={role.scopes.join('\n')} value={role.name}>
            <Checkbox checked={value.includes(role.name)} size="small" />
            <ListItemText
              primary={role.name}
              secondary={intl.formatMessage(messages.scopeCount, { count: role.scopes.length })}
            />
          </MenuItem>
        ))}
      </Select>
      <FormHelperText>{helper}</FormHelperText>
      {warning && <Box sx={{ mt: 1 }}>{warning}</Box>}
    </FormControl>
  );
}
