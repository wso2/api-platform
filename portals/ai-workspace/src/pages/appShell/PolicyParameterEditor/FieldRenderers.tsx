/*
 * Copyright (c) 2026, WSO2 LLC. (http://www.wso2.com). All Rights Reserved.
 *
 * This software is the property of WSO2 LLC. and its suppliers, if any.
 * Dissemination of any information or reproduction of any material contained
 * herein is strictly forbidden, unless permitted by WSO2 in accordance with
 * the WSO2 Commercial License available at http://wso2.com/licenses.
 * For specific language governing the permissions and limitations under
 * this license, please see the license as well as any agreement you've
 * entered into with WSO2 governing the purchase of this software and any
 * associated services.
 */

import React, { useMemo, useState } from 'react';
import {
  Box,
  Button,
  Chip,
  FormControl,
  FormHelperText,
  IconButton,
  MenuItem,
  Select,
  Switch,
  TextField,
  Typography,
} from '@wso2/oxygen-ui';
import { Plus, Trash2 } from '@wso2/oxygen-ui-icons-react';
import { FieldRendererProps } from './types';
import { useStyles } from './styles';

type SimpleTagInputProps = {
  value: string[];
  onChange: (value: string[]) => void;
  placeholder?: string;
  disabled?: boolean;
  error?: boolean;
  helperText?: string;
  testId?: string;
};

const SimpleTagInput: React.FC<SimpleTagInputProps> = ({
  value,
  onChange,
  placeholder,
  disabled,
  error,
  helperText,
  testId,
}) => {
  const classes = useStyles();
  const [inputValue, setInputValue] = useState('');

  const normalizedValues = useMemo(
    () => value.map((item) => item.trim()).filter(Boolean),
    [value]
  );

  const addTag = (raw: string) => {
    const trimmed = raw.trim();
    if (!trimmed) {
      return;
    }
    if (normalizedValues.includes(trimmed)) {
      return;
    }
    onChange([...normalizedValues, trimmed]);
  };

  const handleKeyDown = (event: React.KeyboardEvent<HTMLInputElement>) => {
    if (event.key === 'Enter' || event.key === ',') {
      event.preventDefault();
      addTag(inputValue);
      setInputValue('');
      return;
    }

    if (event.key === 'Backspace' && inputValue === '' && value.length > 0) {
      onChange(normalizedValues.slice(0, -1));
    }
  };

  const handleDelete = (tagToDelete: string) => {
    onChange(normalizedValues.filter((tag) => tag !== tagToDelete));
  };

  return (
    <Box sx={classes.tagInputContainer} data-testid={testId}>
      {normalizedValues.length > 0 && (
        <Box sx={classes.tagInputChips}>
          {normalizedValues.map((tag) => (
            <Chip
              key={tag}
              label={tag}
              size="small"
              onDelete={
                disabled ? undefined : () => handleDelete(tag)
              }
            />
          ))}
        </Box>
      )}
      <TextField
        value={inputValue}
        onChange={(event) => setInputValue(event.target.value)}
        onKeyDown={handleKeyDown}
        disabled={disabled}
        placeholder={placeholder}
        size="small"
        error={!!error}
        sx={classes.tagInputField}
        inputProps={{
          'data-testid': testId ? `${testId}-input` : undefined,
        }}
      />
      {helperText && (
        <FormHelperText error={!!error} sx={classes.tagInputHelperText}>
          {helperText}
        </FormHelperText>
      )}
    </Box>
  );
};

/**
 * Renders a string field - either as a dropdown (if enum) or text input
 */
export const StringFieldRenderer: React.FC<FieldRendererProps> = ({
  node,
  value,
  onChange,
  error,
  disabled,
}) => {
  const classes = useStyles();
  const hasEnum = node.schema.enum && node.schema.enum.length > 0;

  if (hasEnum) {
    return (
      <Box sx={classes.fieldContainer}>
        <FormControl fullWidth size="small" error={!!error} disabled={disabled}>
          <Select
            value={(value as string) || ''}
            onChange={(e) => onChange(node.path, e.target.value as string)}
            displayEmpty
            variant="outlined"
            data-testid={`field-${node.path}`}
            sx={classes.enumSelect}
          >
            <MenuItem value="">
              <em>Select...</em>
            </MenuItem>
            {node.schema.enum!.map((option) => (
              <MenuItem key={option} value={option}>
                {option}
              </MenuItem>
            ))}
          </Select>
        </FormControl>
        {error && <Box sx={classes.errorText}>{error}</Box>}
      </Box>
    );
  }

  return (
    <Box sx={classes.fieldContainer}>
      <TextField
        value={(value as string) || ''}
        onChange={(e) => onChange(node.path, e.target.value)}
        disabled={disabled}
        error={!!error}
        helperText={error}
        fullWidth
        size="small"
        sx={classes.fieldInput}
        placeholder={node.schema.default ? String(node.schema.default) : ''}
        inputProps={{
          'data-testid': `field-${node.path}`,
        }}
      />
    </Box>
  );
};

/**
 * Renders a boolean field as a toggle switch
 */
export const BooleanFieldRenderer: React.FC<FieldRendererProps> = ({
  node,
  value,
  onChange,
  error,
  disabled,
}) => {
  const classes = useStyles();

  const checked = value === true || value === 'true';

  return (
    <Box sx={classes.fieldContainer}>
      <Box sx={classes.toggleContainer}>
        <Switch
          checked={checked}
          onChange={(e) => onChange(node.path, e.target.checked)}
          disabled={disabled}
          size="small"
          data-testid={`field-${node.path}`}
        />
        <Typography variant="body2" sx={{ fontSize: '0.8rem' }}>
          {checked ? 'true' : 'false'}
        </Typography>
      </Box>
      {error && <Box sx={classes.errorText}>{error}</Box>}
    </Box>
  );
};

/**
 * Renders a number/integer field
 */
export const NumberFieldRenderer: React.FC<FieldRendererProps> = ({
  node,
  value,
  onChange,
  error,
  disabled,
}) => {
  const classes = useStyles();
  const isInteger = node.schema.type === 'integer';
  const [displayValue, setDisplayValue] = useState<string>(
    value !== undefined && value !== '' ? String(value) : ''
  );

  const handleChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const inputValue = e.target.value;
    setDisplayValue(inputValue);
    if (inputValue === '') {
      onChange(node.path, '');
      return;
    }
    const parsed = isInteger
      ? parseInt(inputValue, 10)
      : parseFloat(inputValue);
    if (!Number.isNaN(parsed)) {
      onChange(node.path, parsed);
    }
  };

  return (
    <Box sx={classes.fieldContainer}>
      <TextField
        type="number"
        value={displayValue}
        onChange={handleChange}
        disabled={disabled}
        error={!!error}
        helperText={error}
        fullWidth
        size="small"
        sx={classes.fieldInput}
        placeholder={
          node.schema.default !== undefined ? String(node.schema.default) : ''
        }
        inputProps={{
          'data-testid': `field-${node.path}`,
          min: node.schema.minimum,
          max: node.schema.maximum,
          step: isInteger ? 1 : 'any',
        }}
      />
    </Box>
  );
};

/**
 * Renders a simple array field (array of strings)
 */
export const SimpleArrayFieldRenderer: React.FC<FieldRendererProps> = ({
  node,
  value,
  onChange,
  error,
  disabled,
}) => {
  const classes = useStyles();
  const itemType = node.schema.items?.type;
  const arrayValue = Array.isArray(value)
    ? (value as Array<string | number>).map((item) => String(item))
    : [];

  const handleArrayChange = (newValue: string[]) => {
    if (itemType === 'number' || itemType === 'integer') {
      const parsed = newValue.map((item) => {
        const numberValue = Number(item);
        return Number.isNaN(numberValue) ? item : numberValue;
      });
      onChange(node.path, parsed);
      return;
    }
    onChange(node.path, newValue);
  };

  return (
    <Box sx={classes.fieldContainer}>
      <SimpleTagInput
        testId={`field-${node.path}`}
        placeholder="Type a value and press enter"
        value={arrayValue}
        onChange={handleArrayChange}
        helperText={error}
        error={!!error}
        disabled={disabled}
      />
    </Box>
  );
};

/**
 * Renders a dynamic object field with key-value pairs (for objects with additionalProperties)
 */
export const KeyValueFieldRenderer: React.FC<FieldRendererProps> = ({
  node,
  value,
  onChange,
  disabled,
}) => {
  const classes = useStyles();
  const [newKey, setNewKey] = useState('');
  const [newValue, setNewValue] = useState('');

  // Get current entries as array of [key, value] pairs
  const objectValue =
    value && typeof value === 'object' && !Array.isArray(value)
      ? (value as Record<string, string>)
      : {};
  const entries = Object.entries(objectValue);

  const handleAdd = () => {
    if (newKey.trim()) {
      const updated = { ...objectValue, [newKey.trim()]: newValue };
      onChange(node.path, updated);
      setNewKey('');
      setNewValue('');
    }
  };

  const handleDelete = (keyToDelete: string) => {
    const updated = { ...objectValue };
    delete updated[keyToDelete];
    onChange(node.path, updated);
  };

  const handleValueChange = (key: string, newVal: string) => {
    const updated = { ...objectValue, [key]: newVal };
    onChange(node.path, updated);
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter') {
      e.preventDefault();
      handleAdd();
    }
  };

  return (
    <Box sx={classes.fieldContainer}>
      {/* Existing entries */}
      {entries.map(([key, val]) => (
        <Box key={key} sx={classes.keyValueRow}>
          <TextField
            value={key}
            disabled={disabled}
            size="small"
            sx={classes.keyValueKey}
            inputProps={{
              'data-testid': `field-${node.path}-key-${key}`,
            }}
          />
          <TextField
            value={val}
            onChange={(e) => handleValueChange(key, e.target.value)}
            disabled={disabled}
            size="small"
            sx={classes.keyValueValue}
            inputProps={{
              'data-testid': `field-${node.path}-value-${key}`,
            }}
          />
          <IconButton
            size="small"
            onClick={() => handleDelete(key)}
            disabled={disabled}
            sx={classes.keyValueDeleteButton}
          >
            <Trash2 size={14} />
          </IconButton>
        </Box>
      ))}
      {/* Add new entry row */}
      <Box sx={classes.keyValueRow}>
        <TextField
          value={newKey}
          onChange={(e) => setNewKey(e.target.value)}
          placeholder="Key"
          disabled={disabled}
          size="small"
          sx={classes.keyValueKey}
          onKeyDown={handleKeyDown}
          inputProps={{
            'data-testid': `field-${node.path}-new-key`,
          }}
        />
        <TextField
          value={newValue}
          onChange={(e) => setNewValue(e.target.value)}
          placeholder="Value"
          disabled={disabled}
          size="small"
          sx={classes.keyValueValue}
          onKeyDown={handleKeyDown}
          inputProps={{
            'data-testid': `field-${node.path}-new-value`,
          }}
        />
        <Button
          variant="outlined"
          size="small"
          onClick={handleAdd}
          disabled={disabled || !newKey.trim()}
          data-testid={`field-${node.path}-add`}
          startIcon={<Plus size={14} />}
        >
          Add
        </Button>
      </Box>
    </Box>
  );
};

/**
 * Get the appropriate field renderer based on schema type
 */
export const getFieldRenderer = (
  node: FieldRendererProps['node']
): React.FC<FieldRendererProps> | null => {
  const { schema } = node;

  switch (schema.type) {
    case 'string':
      return StringFieldRenderer;
    case 'boolean':
      return BooleanFieldRenderer;
    case 'number':
    case 'integer':
      return NumberFieldRenderer;
    case 'array':
      // Simple arrays (strings, numbers) use TagInput
      // Complex arrays (objects) are handled separately
      if (schema.items?.type === 'string' || schema.items?.type === 'number') {
        return SimpleArrayFieldRenderer;
      }
      return null;
    case 'object':
      // Objects with additionalProperties (dynamic key-value maps)
      if (schema.additionalProperties && !schema.properties) {
        return KeyValueFieldRenderer;
      }
      // Objects with fixed properties are rendered as expandable tree nodes
      return null;
    default:
      return StringFieldRenderer;
  }
};
