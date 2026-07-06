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

import React, { useState, useCallback, useEffect, useMemo } from 'react';
import { Alert, Box, Button, Typography } from '@wso2/oxygen-ui';
import {
  ParameterSchema,
  ParameterValues,
  PolicyDefinition,
  ValidationError,
} from './types';
import {
  initializeDefaultValues,
  setValueByPath,
  getValueByPath,
  createDefaultArrayItem,
  deleteValueByPath,
  coerceValuesToSchemaTypes,
} from './schemaUtils';
import SchemaTree from './SchemaTree';
import { useStyles } from './styles';

// Max lines to show before truncating description
const MAX_DESCRIPTION_LINES = 5;

/**
 * Format description text:
 * - Collapse single newlines within paragraphs to spaces
 * - Preserve paragraph breaks (double newlines)
 * - Preserve bullet points (lines starting with - or *)
 */
function formatDescriptionText(text: string): string {
  if (!text) return '';

  const trimmed = text.trim();

  // Split by double newlines (paragraph breaks)
  const paragraphs = trimmed.split(/\n\n+/);

  return paragraphs
    .map((paragraph) => {
      // Check if paragraph contains bullet points
      const lines = paragraph.split('\n');
      const hasBullets = lines.some((line) => /^\s*[-*•]/.test(line));

      if (hasBullets) {
        // Keep bullet point formatting, but collapse non-bullet continuation lines
        const result: string[] = [];
        let currentLine = '';

        for (const line of lines) {
          if (/^\s*[-*•]/.test(line)) {
            // This is a bullet point line
            if (currentLine) {
              result.push(currentLine);
            }
            currentLine = line;
          } else if (line.trim() === '') {
            // Empty line
            if (currentLine) {
              result.push(currentLine);
              currentLine = '';
            }
          } else {
            // Continuation of previous line
            currentLine = currentLine ? `${currentLine} ${line.trim()}` : line;
          }
        }
        if (currentLine) {
          result.push(currentLine);
        }
        return result.join('\n');
      } else {
        // Regular paragraph - collapse all newlines to spaces
        return paragraph.replace(/\n/g, ' ').replace(/\s+/g, ' ').trim();
      }
    })
    .join('\n\n');
}

/**
 * Component to render header description with show more/less
 */
const TruncatedHeaderDescription: React.FC<{
  description: string;
  classes: ReturnType<typeof useStyles>;
}> = ({ description, classes }) => {
  const [isExpanded, setIsExpanded] = useState(false);
  const formatted = formatDescriptionText(description);
  const lineCount = formatted.split('\n').length;
  const needsTruncation = lineCount > MAX_DESCRIPTION_LINES;

  if (!needsTruncation) {
    return <Typography sx={{ fontSize: '0.8rem' }}>{formatted}</Typography>;
  }

  return (
    <>
      <Typography
        variant="body1"
        component="div"
        sx={
          isExpanded
            ? classes.headerDescription
            : classes.headerDescriptionTruncated
        }
      >
        {formatted}
      </Typography>
      <Box
        component="span"
        sx={classes.showMoreButton}
        onClick={() => setIsExpanded(!isExpanded)}
        style={{ cursor: 'pointer' }}
      >
        {isExpanded ? 'Show less' : 'Show more'}
      </Box>
    </>
  );
};

interface PolicyParameterEditorProps {
  policyDefinition: PolicyDefinition;
  policyDisplayName?: string;
  existingValues?: ParameterValues;
  onCancel: () => void;
  onSubmit: (values: ParameterValues) => void;
  disabled?: boolean;
  // readOnly renders the parameters for viewing only: every field is
  // non-editable and the submit action is hidden, but the values remain visible
  // and the close/cancel action stays enabled.
  readOnly?: boolean;
}

/**
 * Validates required fields in the schema
 */
function validateRequiredFields(
  schema: ParameterSchema,
  values: ParameterValues,
  parentPath: string = ''
): ValidationError[] {
  const errors: ValidationError[] = [];

  if (schema.type === 'object' && schema.properties) {
    const required = schema.required || [];

    Object.entries(schema.properties).forEach(([key, propSchema]) => {
      const path = parentPath ? `${parentPath}.${key}` : key;
      const value = getValueByPath(values, path);

      // Check required fields
      if (required.includes(key)) {
        if (value === undefined || value === null || value === '') {
          errors.push({
            path,
            message: `This field is required`,
          });
        }
      }

      // Recursively validate nested objects
      if (propSchema.type === 'object' && propSchema.properties) {
        const nestedErrors = validateRequiredFields(propSchema, values, path);
        errors.push(...nestedErrors);
      }

      // Validate array items
      if (
        propSchema.type === 'array' &&
        propSchema.items &&
        Array.isArray(value)
      ) {
        value.forEach((_, index) => {
          const itemPath = `${path}.${index}`;
          if (propSchema.items!.type === 'object') {
            const itemErrors = validateRequiredFields(
              propSchema.items!,
              values,
              itemPath
            );
            errors.push(...itemErrors);
          }
        });
      }
    });
  }

  return errors;
}

/**
 * Validates ONLY level-one required fields.
 * Also validates non-advanced optional objects with anyOf constraints:
 * if a boolean field (e.g. "enabled") is set to true, all other
 * anyOf-required fields (e.g. "schema") must also be filled.
 */
function validateLevelOneRequiredFields(
  schema: ParameterSchema,
  values: ParameterValues
): boolean {
  if (schema.type !== 'object' || !schema.properties) {
    return true;
  }

  const required = schema.required || [];

  // Check only top-level required fields
  for (const key of required) {
    const value = getValueByPath(values, key);

    // Check if value is truly empty
    if (value === undefined || value === null || value === '') {
      return false; // Validation failed
    }

    // Check if array is empty
    if (Array.isArray(value) && value.length === 0) {
      return false; // Validation failed
    }
  }

  // For non-advanced optional objects with anyOf constraints:
  // 1. At least one such object must have an anyOf field satisfied
  // 2. If a boolean field is enabled (true), other anyOf-required fields must also be filled
  let anyOfObjects = 0;
  let anyOfSatisfied = 0;

  for (const [key, propSchema] of Object.entries(schema.properties)) {
    // Skip advanced params and required (already validated above)
    if (propSchema['x-wso2-policy-advanced-param'] === true) continue;
    if (required.includes(key)) continue;
    if (propSchema.type !== 'object' || !propSchema.properties) continue;
    if (!propSchema.anyOf || propSchema.anyOf.length === 0) continue;

    anyOfObjects++;

    const objectValue = getValueByPath(values, key) as
      | ParameterValues
      | undefined;
    if (!objectValue) continue;

    // Collect all anyOf required field names
    const anyOfRequiredFields: string[] = [];
    propSchema.anyOf.forEach((entry) => {
      if (entry.required) {
        anyOfRequiredFields.push(...entry.required);
      }
    });

    if (anyOfRequiredFields.length === 0) continue;

    // Check if any boolean field in the anyOf set is enabled (true)
    const hasEnabledBoolean = anyOfRequiredFields.some((fieldName) => {
      const fieldSchema = propSchema.properties![fieldName];
      return fieldSchema?.type === 'boolean' && objectValue[fieldName] === true;
    });

    // Check if any non-boolean anyOf field has a value
    const hasNonBooleanValue = anyOfRequiredFields.some((fieldName) => {
      const fieldSchema = propSchema.properties![fieldName];
      if (fieldSchema?.type === 'boolean') return false;
      const fieldValue = objectValue[fieldName];
      return (
        fieldValue !== undefined &&
        fieldValue !== null &&
        fieldValue !== '' &&
        !(Array.isArray(fieldValue) && fieldValue.length === 0)
      );
    });

    if (hasEnabledBoolean || hasNonBooleanValue) {
      anyOfSatisfied++;
    }

    if (hasEnabledBoolean) {
      // All other anyOf-required fields must be filled
      for (const fieldName of anyOfRequiredFields) {
        const fieldSchema = propSchema.properties![fieldName];
        if (fieldSchema?.type === 'boolean') continue; // boolean is already satisfied
        const fieldValue = objectValue[fieldName];
        if (
          fieldValue === undefined ||
          fieldValue === null ||
          fieldValue === ''
        ) {
          return false;
        }
        if (Array.isArray(fieldValue) && fieldValue.length === 0) {
          return false;
        }
      }
    }
  }

  // If there are anyOf objects but none have any field satisfied, disable the button
  if (anyOfObjects > 0 && anyOfSatisfied === 0) {
    return false;
  }

  return true; // All level-one required fields are filled
}

/**
 * PolicyParameterEditor - A dynamic form editor for policy parameters
 * based on JSON Schema-like definitions from YAML policy files.
 */
const PolicyParameterEditor: React.FC<PolicyParameterEditorProps> = ({
  policyDefinition,
  policyDisplayName,
  existingValues,
  onCancel,
  onSubmit,
  disabled = false,
  readOnly = false,
}) => {
  const classes = useStyles();
  const { name, description, parameters } = policyDefinition;

  // Use policyDisplayName if provided, otherwise fall back to name from definition
  const displayName = policyDisplayName || name;

  // Initialize form values with defaults
  const [values, setValues] = useState<ParameterValues>(() =>
    initializeDefaultValues(parameters, existingValues)
  );

  // Validation errors
  const [errors, setErrors] = useState<Record<string, string>>({});

  // Check if level-one required fields are filled (for button disable state)
  const isLevelOneValid = useMemo(() => {
    return validateLevelOneRequiredFields(parameters, values);
  }, [parameters, values]);

  // Compute anyOf alert message
  const anyOfMessage = useMemo(() => {
    // Case 1: Parent-level anyOf (e.g. anyOf: [{ required: [request] }, { required: [response] }])
    if (parameters.anyOf && parameters.anyOf.length > 0) {
      const requiredItems = parameters.anyOf
        .flatMap((entry) => entry.required || [])
        .filter((v, i, a) => a.indexOf(v) === i);
      if (requiredItems.length > 0) {
        return `At least one of ${requiredItems.join(
          ', '
        )} must be configured.`;
      }
    }

    // Case 2: Property-level anyOf (anyOf inside child properties like request/response)
    // Collect property names that have anyOf constraints
    if (parameters.properties) {
      const propsWithAnyOf = Object.keys(parameters.properties).filter(
        (key) => {
          const propSchema = parameters.properties![key];
          return (
            propSchema.type === 'object' &&
            propSchema.anyOf &&
            propSchema.anyOf.length > 0
          );
        }
      );
      if (propsWithAnyOf.length > 0) {
        return `At least one of ${propsWithAnyOf.join(
          ', '
        )} must be configured.`;
      }
    }

    return null;
  }, [parameters]);

  useEffect(() => {
    if (existingValues) {
      setValues(initializeDefaultValues(parameters, existingValues));
    }
  }, [existingValues, parameters]);

  // Handle field value changes
  const handleChange = useCallback((path: string, value: unknown) => {
    setValues((prev) => setValueByPath(prev, path, value));
    // Clear error for this field
    setErrors((prev) => {
      if (prev[path]) {
        const newErrors = { ...prev };
        delete newErrors[path];
        return newErrors;
      }
      return prev;
    });
  }, []);

  // Handle adding array items
  const handleAddArrayItem = useCallback(
    (arrayPath: string, itemSchema: ParameterSchema) => {
      setValues((prev) => {
        const currentArray =
          (getValueByPath(prev, arrayPath) as unknown[]) || [];
        const newItem = createDefaultArrayItem(itemSchema);
        return setValueByPath(prev, arrayPath, [...currentArray, newItem]);
      });
    },
    []
  );

  // Handle deleting array items
  const handleDeleteArrayItem = useCallback(
    (arrayPath: string, index: number) => {
      setValues((prev) => {
        const currentArray =
          (getValueByPath(prev, arrayPath) as unknown[]) || [];
        const newArray = currentArray.filter((_, i) => i !== index);
        return setValueByPath(prev, arrayPath, newArray);
      });
    },
    []
  );

  // Handle cancel: close drawer if editing, reset form if adding new
  const handleCancel = useCallback(() => {
    if (existingValues) {
      onCancel();
    } else {
      setValues(initializeDefaultValues(parameters));
      setErrors({});
    }
  }, [existingValues, onCancel, parameters]);

  // Handle form submission
  const handleSubmit = useCallback(() => {
    // Validate required fields
    const validationErrors = validateRequiredFields(parameters, values);

    if (validationErrors.length > 0) {
      const errorMap: Record<string, string> = {};
      validationErrors.forEach((err) => {
        errorMap[err.path] = err.message;
      });
      setErrors(errorMap);
      return;
    }

    // Clear errors and submit
    setErrors({});
    // Coerce values to their schema-declared types so booleans, numbers,
    // and objects are not accidentally sent as strings
    const coercedValues = coerceValuesToSchemaTypes(parameters, values);
    onSubmit(coercedValues);
  }, [parameters, values, onSubmit]);

  return (
    <Box sx={classes.root}>
      {/* Header */}
      <Box sx={classes.header}>
        <Typography variant="h5">{displayName}</Typography>
        {description && (
          <TruncatedHeaderDescription
            description={description}
            classes={classes}
          />
        )}
      </Box>

      {/* AnyOf Info Alert */}
      {anyOfMessage && (
        <Alert severity="info" sx={{ mb: 1 }}>
          {anyOfMessage}
        </Alert>
      )}

      {/* Schema Tree */}
      <SchemaTree
        schema={parameters}
        values={values}
        onChange={handleChange}
        onAddArrayItem={handleAddArrayItem}
        onDeleteArrayItem={handleDeleteArrayItem}
        errors={errors}
        disabled={disabled || readOnly}
      />

      {/* Action Buttons */}
      <Box sx={classes.buttonContainer}>
        <Button
          variant="outlined"
          color="secondary"
          onClick={onCancel}
          data-testid="policy-param-cancel"
          disabled={disabled}
        >
          {readOnly ? 'Close' : 'Cancel'}
        </Button>
        {!readOnly && (
          <Button
            variant="contained"
            color="primary"
            onClick={handleSubmit}
            data-testid="policy-param-submit"
            disabled={disabled || !isLevelOneValid}
          >
            {existingValues ? 'Update' : 'Add'}
          </Button>
        )}
      </Box>
    </Box>
  );
};

export default PolicyParameterEditor;
