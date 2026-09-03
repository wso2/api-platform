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

import type { FC } from 'react';
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
import type { EditableField } from '../types';

export type SettingFieldProps = {
  field: EditableField;
  /** The value to show: the pending edit if there is one, else the stored value. */
  value: unknown;
  /** Whether the platform carries a stored value for this path at all. */
  present: boolean;
  error?: string;
  readOnly?: boolean;
  onChange: (value: unknown) => void;
};

/**
 * One `editable` entry rendered as one row.
 *
 * Everything shown comes from the response — `label` and `description` are the
 * platform's own user-facing copy and are used verbatim, not shortened. In
 * particular the two replica labels ("Gateway controller replicas" vs "Gateway
 * runtime replicas") name DIFFERENT pods and must never both become "Replicas".
 *
 * The three things that used to share one helper line are split by WHEN they
 * are needed, because sixteen fields of prose is three screens of scrolling:
 *
 *   bounds       needed WHILE editing  -> inline under the control
 *   error        needed always         -> inline, and never behind a hover
 *   description  needed once, before   -> behind the `?`
 *
 * `string` is not handled here: its one field today is a multi-line TOML block
 * whose copy is an operational warning rather than a description, so it keeps
 * persistent text and its own component (`TomlField`).
 */

/** What to put in a text input for a value that may not be a string yet. */
const inputText = (value: unknown): string =>
  value === undefined || value === null ? '' : String(value);

const SettingField: FC<SettingFieldProps> = ({
  field,
  value,
  present,
  error,
  readOnly = false,
  onChange,
}) => {
  const label = field.label || field.path;

  // A path the values document does not carry is not "set to nothing": the
  // chart's own values.yaml is the merge base, so what runs is the chart
  // default. Say so, rather than letting a blank input imply an empty value.
  const hint =
    error ??
    [
      field.min !== undefined && field.max !== undefined
        ? `${field.min} – ${field.max}`
        : null,
      present ? null : 'Not set — the platform default applies.',
    ]
      .filter(Boolean)
      .join('  ');

  const control = () => {
    switch (field.type) {
      case 'enum':
        return (
          <Select
            disabled={readOnly}
            displayEmpty
            fullWidth
            size="small"
            value={inputText(value)}
            onChange={(event) => onChange(event.target.value)}
          >
            {(field.values ?? []).map((option) => (
              <MenuItem key={option} value={option}>
                {option}
              </MenuItem>
            ))}
          </Select>
        );

      case 'boolean':
        return (
          <Switch
            checked={value === true}
            disabled={readOnly}
            size="small"
            inputProps={{ 'aria-label': label }}
            onChange={(event) => onChange(event.target.checked)}
          />
        );

      case 'integer':
        return (
          <TextField
            disabled={readOnly}
            error={Boolean(error)}
            fullWidth
            size="small"
            type="number"
            slotProps={{ htmlInput: { min: field.min, max: field.max } }}
            value={inputText(value)}
            // An emptied input becomes '' rather than 0 or "unedited": the
            // former would save a number nobody typed, the latter would put the
            // old value back under the cursor. As '' it fails validation with
            // the platform's own "must be a whole number".
            onChange={(event) =>
              onChange(
                event.target.value === '' ? '' : Number(event.target.value)
              )
            }
          />
        );

      // Free text with a syntax the platform parses. A duration is stored
      // exactly as spelled ("5m" stays "5m") and a quantity IS canonicalized on
      // write, which is why the drawer re-seeds from the PUT response.
      //
      // NO PLACEHOLDER. An example value in a greyed-out input is read as the
      // setting's current value, and for the ten paths the response carries no
      // value for, that is exactly the wrong thing to imply. The bounds hint
      // ("30s – 1h") already shows the syntax, and it does not look like data.
      case 'duration':
      case 'quantity':
        return (
          <TextField
            disabled={readOnly}
            error={Boolean(error)}
            fullWidth
            size="small"
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

  return (
    <Box sx={{ display: 'flex', alignItems: 'flex-start', gap: 2, py: 1 }}>
      <Box
        sx={{
          flex: 1,
          minWidth: 0,
          minHeight: 40,
          display: 'flex',
          alignItems: 'center',
          gap: 0.25,
        }}
      >
        <Typography variant="body2" sx={{ fontWeight: 500 }}>
          {label}
        </Typography>
        {field.description ? (
          // An IconButton rather than a bare span: a tooltip on something
          // unfocusable is unreachable by keyboard. `describeChild` keeps the
          // button's own label and exposes the text as its description, and the
          // touch delays make a tap show it and hold it long enough to read.
          <Tooltip
            title={field.description}
            describeChild
            enterTouchDelay={0}
            leaveTouchDelay={8000}
          >
            <IconButton
              aria-label={`About ${label}`}
              size="small"
              sx={{ p: 0.25, color: 'text.disabled' }}
            >
              <CircleHelp size={14} />
            </IconButton>
          </Tooltip>
        ) : null}
      </Box>

      <Box sx={{ flex: '0 0 200px', minHeight: 40 }}>
        {rendered}
        {hint ? (
          <FormHelperText error={Boolean(error)} sx={{ mx: 0 }}>
            {hint}
          </FormHelperText>
        ) : null}
      </Box>
    </Box>
  );
};

export default SettingField;
