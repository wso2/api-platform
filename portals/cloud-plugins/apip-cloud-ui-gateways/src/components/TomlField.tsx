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

import { useState, type FC } from 'react';
import {
  Alert,
  Box,
  Collapse,
  FormHelperText,
  IconButton,
  TextField,
  Tooltip,
  Typography,
} from '@wso2/oxygen-ui';
import { ChevronDown, TriangleAlert } from '@wso2/oxygen-ui-icons-react';
import { redeclaredSections } from '../config/toml';
import { characterCount } from '../config/validate';
import type { EditableField } from '../types';

export type TomlFieldProps = {
  field: EditableField;
  value: unknown;
  error?: string;
  readOnly?: boolean;
  onChange: (value: string) => void;
};

/**
 * `gateway.config_toml` — the raw-TOML append hatch, and the one field where a
 * value that passes every check can stop the gateway MINUTES after a 200.
 *
 * The chart appends this text, verbatim and last, to a config.toml it has
 * already written. TOML forbids declaring a table twice, so a block repeating a
 * section the structured values already emitted is a startup parse error rather
 * than an override. Nothing on the server checks for that yet — it is the known
 * gap this field shipped with, and nothing here closes it.
 *
 * The append-not-merge warning sits behind the triangle beside the label. It
 * used to be a persistent Alert on the argument that a tooltip is not read by
 * someone already typing — which is true, and the reason the REDECLARED-SECTION
 * Alert below stays persistent. That one fires on the actual mistake, naming
 * the offending sections, so the case that kills a gateway is still called out
 * inline; the general caution is what moved behind the hover.
 *
 * No Monaco: the console does not ship it, and this component renders in both
 * hosts.
 */

/**
 * Why this field is not an override. Plain text rather than markup: a tooltip
 * title is read out as its text content, and the emphasis the Alert carried
 * cannot survive the move anyway.
 */
const APPEND_WARNING =
  'This text is appended to the configuration the gateway already generates — ' +
  'it does not merge with it. A section the gateway already configures cannot ' +
  'be redefined here, and repeating one stops the gateway from starting. ' +
  'Clear the field to remove it.';

const TomlField: FC<TomlFieldProps> = ({
  field,
  value,
  error,
  readOnly = false,
  onChange,
}) => {
  const [open, setOpen] = useState(false);

  const text = typeof value === 'string' ? value : '';
  // Code points, matching the platform's `len([]rune(text))` — a ceiling on how
  // much a person may type, and a multi-byte character is one character to them.
  const length = characterCount(text);
  const max = field.max ? Number.parseInt(field.max, 10) : null;
  const redeclared = redeclaredSections(text);

  return (
    <Box sx={{ borderTop: 1, borderColor: 'divider', mt: 1, pt: 1 }}>
      <Box
        onClick={() => setOpen((current) => !current)}
        sx={{
          alignItems: 'center',
          cursor: 'pointer',
          display: 'flex',
          gap: 1,
          py: 1,
        }}
      >
        <Typography sx={{ fontWeight: 500 }} variant="body2">
          {field.label || field.path}
        </Typography>
        <Tooltip
          describeChild
          enterTouchDelay={0}
          leaveTouchDelay={8000}
          title={APPEND_WARNING}
        >
          <IconButton
            aria-label="About this field"
            // The whole row toggles the section, so the icon must not: a hover
            // that collapses the editor under the cursor is worse than no
            // tooltip at all.
            onClick={(event) => event.stopPropagation()}
            size="small"
            sx={{ p: 0.25, color: 'warning.main' }}
          >
            <TriangleAlert size={14} />
          </IconButton>
        </Tooltip>
        <Typography
          color={error ? 'error.main' : 'text.secondary'}
          sx={{ ml: 'auto' }}
          variant="caption"
        >
          {max === null ? length : `${length}/${max}`}
        </Typography>
        <IconButton
          aria-expanded={open}
          aria-label={open ? 'Collapse advanced configuration' : 'Expand advanced configuration'}
          size="small"
          sx={{
            transform: open ? 'rotate(180deg)' : 'none',
            transition: 'transform 150ms',
          }}
        >
          <ChevronDown size={16} />
        </IconButton>
      </Box>

      <Collapse in={open}>
        {redeclared.length > 0 ? (
          <Alert severity="error" sx={{ mb: 1.5 }}>
            {redeclared.join(' and ')}{' '}
            {redeclared.length === 1 ? 'is' : 'are'} already configured by the
            platform. Re-declaring{' '}
            {redeclared.length === 1 ? 'it' : 'them'} here will stop the gateway
            from starting.
          </Alert>
        ) : null}

        <TextField
          disabled={readOnly}
          error={Boolean(error)}
          fullWidth
          maxRows={20}
          minRows={6}
          multiline
          onChange={(event) => onChange(event.target.value)}
          placeholder={'[some.section]\nkey = "value"'}
          slotProps={{
            htmlInput: {
              spellCheck: false,
              style: { fontFamily: 'monospace', fontSize: 12 },
            },
          }}
          value={text}
        />

        {error ? (
          <FormHelperText error sx={{ mx: 0 }}>
            {error}
          </FormHelperText>
        ) : null}
      </Collapse>
    </Box>
  );
};

export default TomlField;
