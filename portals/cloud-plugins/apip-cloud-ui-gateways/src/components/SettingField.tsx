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

import { useId, type FC } from 'react';
import {
  Box,
  FormHelperText,
  IconButton,
  MenuItem,
  Select,
  Switch,
  TextField,
  Tooltip,
  Typography,
} from '@wso2/oxygen-ui';
import { CircleHelp } from '@wso2/oxygen-ui-icons-react';
import { helperText, optionLabel } from '../config/display';
import type { EditableField } from '../types';
import { inputSx, secondaryTextSx } from './styles';

export type SettingFieldProps = {
  field: EditableField;
  /** The value to show: the pending edit if there is one, else the stored value. */
  value: unknown;
  error?: string;
  readOnly?: boolean;
  /** Whether the shown value is an unsaved edit; marks the label and the control. */
  modified?: boolean;
  onChange: (value: unknown) => void;
};

/**
 * One `editable` entry: its label above the control and one line below it.
 *
 * Everything shown comes from the response. `label` and `description` are the
 * platform's own user-facing copy and are used verbatim, not shortened. In
 * particular two settings with similar names ("controller" vs "policy engine"
 * log level) name DIFFERENT components and must never be shortened to one.
 *
 * The line below is the platform's short hint and default, until there is an
 * error, which takes its place: the error is what the user has to act on, and it
 * states the bound that was broken. The full description is behind the `?`.
 *
 * Every string is one line. A TOML block is not rendered at all (see
 * `isTomlBlock`).
 */

/**
 * Whether a field is gateway.config_toml from a platform that still offers it,
 * as `toml` or, before `format` existed, with none. The console does not show
 * it: raw TOML could repoint the gateway's database, and the platform is
 * withdrawing it.
 */
export const isTomlBlock = (field: EditableField): boolean =>
  field.type === 'string' && (field.format ?? 'toml') === 'toml';

/** What to put in a text input for a value that may not be a string yet. */
const inputText = (value: unknown): string =>
  value === undefined || value === null ? '' : String(value);

const SettingField: FC<SettingFieldProps> = ({
  field,
  value,
  error,
  readOnly = false,
  modified = false,
  onChange,
}) => {
  const label = field.label || field.path;
  const labelId = useId();
  const outline = inputSx(modified);

  /*
   * Every control is named by the visible label through `aria-labelledby`, so a
   * screen reader announces the enum as "Controller log level", not as an
   * unnamed combobox.
   */
  const control = () => {
    switch (field.type) {
      case 'enum':
        return (
          <Select
            inputProps={{ 'aria-labelledby': labelId }}
            disabled={readOnly}
            displayEmpty
            fullWidth
            size="small"
            sx={outline}
            value={inputText(value)}
            onChange={(event) => onChange(event.target.value)}
          >
            {(field.values ?? []).map((option) => (
              <MenuItem key={option} value={option}>
                {optionLabel(field, option)}
              </MenuItem>
            ))}
          </Select>
        );

      case 'boolean':
        return (
          <Box sx={{ alignItems: 'center', display: 'flex', gap: 1, minHeight: 40 }}>
            <Switch
              checked={value === true}
              disabled={readOnly}
              size="small"
              inputProps={{ 'aria-labelledby': labelId }}
              onChange={(event) => onChange(event.target.checked)}
            />
            <Typography
              variant="body2"
              sx={readOnly ? { color: 'text.disabled' } : value === true ? undefined : secondaryTextSx}
            >
              {value === true ? 'On' : 'Off'}
            </Typography>
          </Box>
        );

      case 'integer':
        return (
          <TextField
            disabled={readOnly}
            error={Boolean(error)}
            fullWidth
            size="small"
            type="number"
            sx={outline}
            slotProps={{
              htmlInput: { 'aria-labelledby': labelId, min: field.min, max: field.max },
            }}
            value={inputText(value)}
            // An emptied input becomes '' rather than 0 or "unedited": the
            // former would save a number nobody typed, the latter would put the
            // old value back under the cursor. As '' it fails validation with
            // the platform's own "must be a whole number".
            onChange={(event) =>
              onChange(event.target.value === '' ? '' : Number(event.target.value))
            }
          />
        );

      // Free text with a syntax the platform parses. A duration is stored
      // exactly as spelled ("5m" stays "5m") and a quantity IS canonicalized on
      // write, which is why the drawer re-seeds from the PUT response.
      //
      // NO PLACEHOLDER. An example value in a greyed-out input is read as the
      // setting's current value; the default is spelled out in the line below.
      case 'duration':
      case 'quantity':
      case 'string':
        if (isTomlBlock(field)) return null;
        return (
          <TextField
            disabled={readOnly}
            error={Boolean(error)}
            fullWidth
            size="small"
            sx={outline}
            slotProps={{ htmlInput: { 'aria-labelledby': labelId } }}
            value={inputText(value)}
            onChange={(event) => onChange(event.target.value)}
          />
        );

      default:
        return null;
    }
  };

  const rendered = control();
  if (!rendered) return null;
  const help = helperText(field);

  return (
    <Box>
      <Box sx={{ alignItems: 'center', display: 'flex', gap: 0.75, mb: 0.75 }}>
        <Typography id={labelId} variant="body2">
          {label}
        </Typography>
        {field.description ? (
          // An IconButton rather than a bare icon: a tooltip on something
          // unfocusable is unreachable by keyboard.
          <Tooltip title={field.description} describeChild enterTouchDelay={0} leaveTouchDelay={8000}>
            <IconButton aria-label={`About ${label}`} size="small" sx={{ ...secondaryTextSx, p: 0.25 }}>
              <CircleHelp size={14} />
            </IconButton>
          </Tooltip>
        ) : null}
        {modified ? (
          <Box
            aria-label="Unsaved change"
            role="img"
            sx={{ bgcolor: 'primary.main', borderRadius: '50%', height: 7, width: 7 }}
          />
        ) : null}
      </Box>
      {rendered}
      {error ? (
        <FormHelperText error sx={{ mx: 0 }}>
          {error}
        </FormHelperText>
      ) : help ? (
        <FormHelperText sx={{ ...secondaryTextSx, mx: 0 }}>{help}</FormHelperText>
      ) : null}
    </Box>
  );
};

export default SettingField;
