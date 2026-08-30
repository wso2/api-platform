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
  Button,
  Checkbox,
  Collapse,
  FormControlLabel,
  IconButton,
  MenuItem,
  Select,
  Stack,
  TextField,
  Typography,
} from '@wso2/oxygen-ui';
import { ChevronDown, ChevronRight, Plus, Trash2 } from '@wso2/oxygen-ui-icons-react';
import { useState } from 'react';

import { defaultForSchema, getByPath, type ParameterSchema } from '@/api/policyHub/policySchema';

type FieldProps = {
  schema: ParameterSchema;
  path: string;
  label?: string;
  required?: boolean;
  values: Record<string, unknown>;
  onChange: (path: string, value: unknown) => void;
  /** Adds a default item to the array at `path`. */
  onAddItem: (path: string, itemSchema: ParameterSchema) => void;
  onRemoveItem: (path: string, index: number) => void;
};

/** Title if authored, else the key with its first letter capitalized. */
const propLabel = (key: string, title?: string): string =>
  title || (key ? key.charAt(0).toUpperCase() + key.slice(1) : key);

const FieldLabel = ({
  label,
  required,
  description,
}: {
  label?: string;
  required?: boolean;
  description?: string;
}) => {
  if (!label) return null;
  return (
    <Box sx={{ mb: 0.5 }}>
      <Typography component="span" sx={{ fontWeight: 600 }} variant="body2">
        {label}
        {required ? ' *' : ''}
      </Typography>
      {description && (
        <Typography color="text.secondary" sx={{ display: 'block' }} variant="caption">
          {description}
        </Typography>
      )}
    </Box>
  );
};

/**
 * Editor for a dynamic key→value map (object with `additionalProperties` and no
 * fixed `properties`, e.g. claimMappings / requiredClaims). Holds the row list
 * locally so blank/duplicate keys can be typed; only non-empty keys are
 * committed to the model object.
 */
function MapField({
  schema,
  path,
  label,
  required,
  value,
  onChange,
}: {
  schema: ParameterSchema;
  path: string;
  label?: string;
  required?: boolean;
  value: unknown;
  onChange: (path: string, value: unknown) => void;
}) {
  const valueSchema = schema.additionalProperties ?? { type: 'string' };
  const [entries, setEntries] = useState<Array<[string, unknown]>>(() =>
    value && typeof value === 'object' && !Array.isArray(value)
      ? Object.entries(value as Record<string, unknown>)
      : [],
  );

  const commit = (next: Array<[string, unknown]>) => {
    setEntries(next);
    const obj: Record<string, unknown> = {};
    for (const [k, v] of next) if (k.trim() !== '') obj[k] = v;
    onChange(path, obj);
  };

  const setKey = (i: number, key: string) =>
    commit(entries.map((e, idx) => (idx === i ? [key, e[1]] : e)));
  const setVal = (i: number, val: unknown) =>
    commit(entries.map((e, idx) => (idx === i ? [e[0], val] : e)));
  const remove = (i: number) => commit(entries.filter((_e, idx) => idx !== i));
  const add = () => commit([...entries, ['', defaultForSchema(valueSchema) ?? '']]);

  const isNum = valueSchema.type === 'number' || valueSchema.type === 'integer';
  const isBool = valueSchema.type === 'boolean';

  return (
    <Box>
      <FieldLabel description={schema.description} label={label} required={required} />
      <Stack spacing={1}>
        {entries.map(([key, val], index) => (
          <Box key={index} sx={{ alignItems: 'center', display: 'flex', gap: 1 }}>
            <TextField
              onChange={(event) => setKey(index, event.target.value)}
              placeholder="Key"
              size="small"
              sx={{ flex: 1 }}
              value={key}
            />
            {isBool ? (
              <Checkbox
                checked={Boolean(val)}
                onChange={(event) => setVal(index, event.target.checked)}
              />
            ) : (
              <TextField
                onChange={(event) =>
                  setVal(
                    index,
                    isNum
                      ? event.target.value === ''
                        ? ''
                        : Number(event.target.value)
                      : event.target.value,
                  )
                }
                placeholder="Value"
                size="small"
                sx={{ flex: 1 }}
                type={isNum ? 'number' : 'text'}
                value={(val as string | number) ?? ''}
              />
            )}
            <IconButton
              aria-label="Remove entry"
              color="error"
              onClick={() => remove(index)}
              size="small"
            >
              <Trash2 size={14} />
            </IconButton>
          </Box>
        ))}
        <Button
          onClick={add}
          size="small"
          startIcon={<Plus size={14} />}
          sx={{ alignSelf: 'flex-start' }}
        >
          Add entry
        </Button>
      </Stack>
    </Box>
  );
}

/** Recursively renders a policy parameter schema as form fields. */
export function SchemaField({
  schema,
  path,
  label,
  required,
  values,
  onChange,
  onAddItem,
  onRemoveItem,
}: FieldProps) {
  const [showAdvanced, setShowAdvanced] = useState(false);
  const value = getByPath(values, path);

  // --- dynamic key→value map (object w/ additionalProperties, no fixed props) ---
  if (
    schema.type === 'object' &&
    schema.additionalProperties &&
    Object.keys(schema.properties || {}).length === 0
  ) {
    return (
      <MapField
        label={label}
        onChange={onChange}
        path={path}
        required={required}
        schema={schema}
        value={value}
      />
    );
  }

  // --- object: render each property ---
  if (schema.type === 'object' && schema.properties) {
    const entries = Object.entries(schema.properties);
    const requiredSet = new Set(schema.required || []);
    // Top-level only (matching legacy): split into a basic section and a
    // collapsible "Advanced Settings". A property is advanced when explicitly
    // flagged, or — absent a flag — when it is not required.
    const isAdvanced = ([key, child]: [string, ParameterSchema]) =>
      child.advanced === true || (child.advanced !== false && !requiredSet.has(key));
    const isRoot = path === '';
    const basic = isRoot ? entries.filter((e) => !isAdvanced(e)) : entries;
    const advanced = isRoot ? entries.filter(isAdvanced) : [];

    const renderProp = ([key, child]: [string, ParameterSchema]) => (
      <Box key={key} sx={{ mb: 2 }}>
        <SchemaField
          label={propLabel(key, child.title)}
          onAddItem={onAddItem}
          onChange={onChange}
          onRemoveItem={onRemoveItem}
          path={path ? `${path}.${key}` : key}
          required={requiredSet.has(key)}
          schema={child}
          values={values}
        />
      </Box>
    );

    return (
      <Box>
        {label && <FieldLabel description={schema.description} label={label} required={required} />}
        <Box
          sx={
            label
              ? { borderLeft: '2px solid', borderColor: 'divider', ml: 0.5, pl: 1.5 }
              : undefined
          }
        >
          {basic.map(renderProp)}
          {advanced.length > 0 && (
            <>
              <Button
                onClick={() => setShowAdvanced((s) => !s)}
                size="small"
                startIcon={showAdvanced ? <ChevronDown size={16} /> : <ChevronRight size={16} />}
                sx={{ mb: 1 }}
                variant="text"
              >
                Advanced Settings
              </Button>
              <Collapse in={showAdvanced}>
                <Box sx={{ pt: 0.5 }}>{advanced.map(renderProp)}</Box>
              </Collapse>
            </>
          )}
        </Box>
      </Box>
    );
  }

  // --- array: list of items (primitive or object) ---
  if (schema.type === 'array' && schema.items) {
    const items = Array.isArray(value) ? value : [];
    const itemSchema = schema.items;
    return (
      <Box>
        <FieldLabel description={schema.description} label={label} required={required} />
        <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1 }}>
          {items.map((_item, index) => (
            <Box
              key={index}
              sx={{
                alignItems: 'flex-start',
                border: '1px solid',
                borderColor: 'divider',
                borderRadius: 1,
                display: 'flex',
                gap: 1,
                p: 1,
              }}
            >
              <Box sx={{ flex: 1, minWidth: 0 }}>
                <SchemaField
                  onAddItem={onAddItem}
                  onChange={onChange}
                  onRemoveItem={onRemoveItem}
                  path={`${path}.${index}`}
                  schema={itemSchema}
                  values={values}
                />
              </Box>
              <IconButton
                aria-label="Remove item"
                color="error"
                onClick={() => onRemoveItem(path, index)}
                size="small"
              >
                <Trash2 size={14} />
              </IconButton>
            </Box>
          ))}
          <Button
            onClick={() => onAddItem(path, itemSchema)}
            size="small"
            startIcon={<Plus size={14} />}
            sx={{ alignSelf: 'flex-start' }}
          >
            Add item
          </Button>
        </Box>
      </Box>
    );
  }

  // --- enum string → select ---
  if (schema.type === 'string' && schema.enum && schema.enum.length > 0) {
    return (
      <Box>
        <FieldLabel description={schema.description} label={label} required={required} />
        <Select
          displayEmpty
          fullWidth
          onChange={(event) => onChange(path, event.target.value)}
          size="small"
          value={(value as string) ?? ''}
        >
          <MenuItem value="">
            <em>Select…</em>
          </MenuItem>
          {schema.enum.map((option) => (
            <MenuItem key={option} value={option}>
              {option}
            </MenuItem>
          ))}
        </Select>
      </Box>
    );
  }

  // --- boolean → checkbox ---
  if (schema.type === 'boolean') {
    return (
      <FormControlLabel
        control={
          <Checkbox
            checked={Boolean(value)}
            onChange={(event) => onChange(path, event.target.checked)}
          />
        }
        label={
          <Box>
            <Typography component="span" variant="body2">
              {label}
              {required ? ' *' : ''}
            </Typography>
            {schema.description && (
              <Typography color="text.secondary" sx={{ display: 'block' }} variant="caption">
                {schema.description}
              </Typography>
            )}
          </Box>
        }
      />
    );
  }

  // --- number / integer / string ---
  const isNumber = schema.type === 'number' || schema.type === 'integer';
  return (
    <Box>
      <FieldLabel description={schema.description} label={label} required={required} />
      <TextField
        fullWidth
        onChange={(event) =>
          onChange(
            path,
            isNumber
              ? event.target.value === ''
                ? ''
                : Number(event.target.value)
              : event.target.value,
          )
        }
        placeholder={schema.default !== undefined ? String(schema.default) : ''}
        size="small"
        type={isNumber ? 'number' : 'text'}
        value={(value as string | number) ?? ''}
      />
    </Box>
  );
}

/** Re-exported so the drawer can spawn default array items. */
export { defaultForSchema };
