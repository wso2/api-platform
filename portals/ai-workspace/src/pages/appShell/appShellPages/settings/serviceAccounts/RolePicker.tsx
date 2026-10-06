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

import type { ReactNode } from 'react';
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
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';
import type { ServiceAccountRole } from '../../../../../apis/serviceAccountApis';

const P = 'aiWorkspace.pages.appShell.appShellPages.settings.serviceAccounts.RolePicker';
const messages = defineMessages({
  label: { id: `${P}.label`, defaultMessage: 'Roles' },
  helper: { id: `${P}.helper`, defaultMessage: 'Each role lists the scopes it grants. Pick the fewest that do the job.' },
  grants: { id: `${P}.grants`, defaultMessage: '{count, plural, one {Grants # scope.} other {Grants # scopes.}}' },
  loading: { id: `${P}.loading`, defaultMessage: 'Loading roles…' },
  loadFailed: { id: `${P}.loadFailed`, defaultMessage: 'Could not load the roles. Close the dialog and try again.' },
  none: {
    id: `${P}.none`,
    defaultMessage: 'No roles are defined. Add one to the role-to-scope mapping on the server.',
  },
  required: { id: `${P}.required`, defaultMessage: 'Select at least one role.' },
  scopeCount: { id: `${P}.scopeCount`, defaultMessage: '{count, plural, one {# scope} other {# scopes}}' },
});

const FIELD_ID = 'service-account-roles';

/** Preselected for a new account when the server defines it. */
export const DEFAULT_ROLE = 'ap_service_account';

interface RolePickerProps {
  roles: readonly ServiceAccountRole[];
  value: string[];
  onChange: (next: string[]) => void;
  loading?: boolean;
  failed?: boolean;
  showRequiredError?: boolean;
  /** Extra guidance under the field, e.g. what removing a role does. */
  warning?: ReactNode;
}

export default function RolePicker({
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
  const granted = new Set(roles.filter((role) => value.includes(role.name)).flatMap((role) => role.scopes)).size;

  const helper = failed
    ? intl.formatMessage(messages.loadFailed)
    : loading
      ? intl.formatMessage(messages.loading)
      : roles.length === 0
        ? intl.formatMessage(messages.none)
        : showRequiredError
          ? intl.formatMessage(messages.required)
          : value.length > 0
            ? intl.formatMessage(messages.grants, { count: granted })
            : intl.formatMessage(messages.helper);

  return (
    <FormControl error={failed || (!loading && roles.length === 0) || showRequiredError} fullWidth required>
      <FormLabel id={`${FIELD_ID}-label`}>
        <FormattedMessage {...messages.label} />
      </FormLabel>
      <Select<string[]>
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
