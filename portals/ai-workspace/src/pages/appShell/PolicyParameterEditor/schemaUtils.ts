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

import {
  ParameterSchema,
  PresenceCondition,
  SchemaTreeNode,
  ParameterValues,
  ValidationError,
} from './types';

/**
 * Whether a value contains a Go-template expression resolved by the gateway.
 */
export function isTemplateExpression(value: unknown): value is string {
  return typeof value === 'string' && /\{\{[\s\S]+?\}\}/.test(value);
}

/**
 * Whether a value counts as set. Empty values, including objects whose
 * children are all empty, are dropped on submit (see omitOptionalEmptyValues).
 */
export function hasConfiguredValue(value: unknown): boolean {
  if (value === undefined || value === null || value === '') return false;
  if (Array.isArray(value)) return value.length > 0;
  if (typeof value === 'object') return Object.values(value).some(hasConfiguredValue);
  return true;
}

// The only keywords a oneOf option may use for the editor to evaluate it
const PRESENCE_KEYWORDS = new Set(['required', 'anyOf', 'not', 'title', 'description', '$comment']);

function isPresenceOnly(condition: unknown): boolean {
  if (typeof condition !== 'object' || condition === null) return false;
  const { anyOf, not } = condition as PresenceCondition;
  return (
    Object.keys(condition).every((key) => PRESENCE_KEYWORDS.has(key)) &&
    (anyOf === undefined || anyOf.every(isPresenceOnly)) &&
    (not === undefined || isPresenceOnly(not))
  );
}

/**
 * The schema's oneOf options, or none if any option has no required field or
 * uses more than required / anyOf / not. Such a oneOf is left to the gateway.
 */
export function getSupportedOneOfOptions(schema: ParameterSchema): PresenceCondition[] {
  const options = schema.oneOf ?? [];
  return options.every((option) => option.required?.length && isPresenceOnly(option))
    ? options
    : [];
}

/** Required-field groups of the schema's supported oneOf options. */
export function getOneOfGroups(schema: ParameterSchema): string[][] {
  return getSupportedOneOfOptions(schema).map((option) => option.required!);
}

/** Formats groups for messages, e.g. "a, b and c or d". */
export function formatOneOfGroups(groups: string[][]): string {
  return groups
    .map((group) =>
      group.length === 1
        ? group[0]
        : `${group.slice(0, -1).join(', ')} and ${group[group.length - 1]}`
    )
    .join(' or ');
}

/**
 * Paths of all oneOf fields in the groups that `path`, or an object containing
 * it, belongs to. Used to clear group-wide errors when one field changes.
 */
export function getOneOfSiblingPaths(schema: ParameterSchema, path: string): string[] {
  const keys = path.split('.');
  const paths = new Set<string>();
  let parent: ParameterSchema | undefined = schema;
  keys.forEach((key, depth) => {
    if (!parent) return;
    const fields = getOneOfGroups(parent).flat();
    if (fields.includes(key)) {
      const prefix = keys.slice(0, depth).join('.');
      fields.forEach((field) => paths.add(prefix ? `${prefix}.${field}` : field));
    }
    parent = /^\d+$/.test(key) ? parent.items : parent.properties?.[key];
  });
  return [...paths];
}

/**
 * Generate a unique ID for tree nodes
 */
let nodeIdCounter = 0;
export const generateNodeId = (): string => {
  nodeIdCounter += 1;
  return `node-${nodeIdCounter}`;
};

/**
 * Reset node ID counter (useful for testing)
 */
export const resetNodeIdCounter = (): void => {
  nodeIdCounter = 0;
};

/**
 * Convert a parameter schema to tree nodes
 */
export function schemaToTreeNodes(
  schema: ParameterSchema,
  parentPath: string = '',
  depth: number = 0,
  requiredFields: string[] = []
): SchemaTreeNode[] {
  const nodes: SchemaTreeNode[] = [];

  if (schema.type === 'object' && schema.properties) {
    const required = schema.required || [];

    // Collect required fields from anyOf entries
    const anyOfRequired: string[] = [];
    if (schema.anyOf) {
      schema.anyOf.forEach((entry) => {
        if (entry.required) {
          anyOfRequired.push(...entry.required);
        }
      });
    }

    // oneOf fields are required only when their group is the chosen one
    const oneOfFields = new Set(getOneOfGroups(schema).flat());

    Object.entries(schema.properties).forEach(([key, propSchema]) => {
      const path = parentPath ? `${parentPath}.${key}` : key;
      const isRequired = required.includes(key) || requiredFields.includes(key) || anyOfRequired.includes(key);

      const isAdvanced = propSchema['x-wso2-policy-advanced-param'] === true;

      const node: SchemaTreeNode = {
        id: path, // Use path as stable ID
        path,
        name: key,
        schema: propSchema,
        depth,
        isRequired,
        isOneOfField: !isRequired && oneOfFields.has(key),
        isExpanded: false, // All collapsed by default
        isAdvanced,
      };

      // Handle nested structures
      if (propSchema.type === 'object' && propSchema.properties) {
        node.children = schemaToTreeNodes(
          propSchema,
          path,
          depth + 1,
          propSchema.required || []
        );
      } else if (propSchema.type === 'array') {
        node.isArrayContainer = true;
        // Children will be added dynamically based on values
      }

      nodes.push(node);
    });
  }

  return nodes;
}

/**
 * Create tree nodes for array items based on current values
 */
export function createArrayItemNodes(
  arrayPath: string,
  itemSchema: ParameterSchema,
  items: unknown[],
  depth: number
): SchemaTreeNode[] {
  return items.map((_, index) => {
    const itemPath = `${arrayPath}.${index}`;
    const node: SchemaTreeNode = {
      id: generateNodeId(),
      path: itemPath,
      name: `Item ${index + 1}`,
      schema: itemSchema,
      depth,
      isRequired: false,
      isExpanded: false,
      isArrayItem: true,
      arrayIndex: index,
      parentArrayPath: arrayPath,
    };

    // If array item is an object, add its properties as children
    if (itemSchema.type === 'object' && itemSchema.properties) {
      node.children = schemaToTreeNodes(
        itemSchema,
        itemPath,
        depth + 1,
        itemSchema.required || []
      );
    }

    return node;
  });
}

/**
 * Get a nested value from an object using dot notation path
 */
export function getValueByPath(obj: ParameterValues, path: string): unknown {
  if (!path) return obj;

  const keys = path.split('.');
  let current: unknown = obj;

  for (const key of keys) {
    if (current === null || current === undefined) {
      return undefined;
    }
    if (typeof current === 'object') {
      current = (current as Record<string, unknown>)[key];
    } else {
      return undefined;
    }
  }

  return current;
}

/**
 * Set a nested value in an object using dot notation path
 * Returns a new object (immutable update)
 */
export function setValueByPath(
  obj: ParameterValues,
  path: string,
  value: unknown
): ParameterValues {
  if (!path) return value as ParameterValues;

  const keys = path.split('.');
  const result = { ...obj };
  let current: Record<string, unknown> = result;

  for (let i = 0; i < keys.length - 1; i++) {
    const key = keys[i];
    const nextKey = keys[i + 1];
    const isNextKeyArrayIndex = /^\d+$/.test(nextKey);

    if (current[key] === undefined || current[key] === null) {
      current[key] = isNextKeyArrayIndex ? [] : {};
    } else if (Array.isArray(current[key])) {
      current[key] = [...(current[key] as unknown[])];
    } else if (typeof current[key] === 'object') {
      current[key] = { ...(current[key] as Record<string, unknown>) };
    }

    current = current[key] as Record<string, unknown>;
  }

  const lastKey = keys[keys.length - 1];
  current[lastKey] = value;

  return result;
}

/**
 * Delete a value at a path (for removing array items)
 */
export function deleteValueByPath(
  obj: ParameterValues,
  path: string
): ParameterValues {
  const keys = path.split('.');
  const result = { ...obj };

  if (keys.length === 1) {
    delete result[keys[0]];
    return result;
  }

  const parentPath = keys.slice(0, -1).join('.');
  const lastKey = keys[keys.length - 1];
  const parent = getValueByPath(result, parentPath);

  if (Array.isArray(parent)) {
    const index = parseInt(lastKey, 10);
    const newArray = [...parent];
    newArray.splice(index, 1);
    return setValueByPath(result, parentPath, newArray);
  }

  if (typeof parent === 'object' && parent !== null) {
    const newParent = { ...(parent as Record<string, unknown>) };
    delete newParent[lastKey];
    return setValueByPath(result, parentPath, newParent);
  }

  return result;
}

/**
 * Initialize default values from schema
 */
export function initializeDefaultValues(
  schema: ParameterSchema,
  existingValues?: ParameterValues
): ParameterValues {
  const result: ParameterValues = existingValues ? { ...existingValues } : {};

  if (schema.type === 'object' && schema.properties) {
    Object.entries(schema.properties).forEach(([key, propSchema]) => {
      if (result[key] === undefined) {
        if (propSchema.default !== undefined) {
          result[key] = propSchema.default;
        } else if (propSchema.type === 'object' && propSchema.properties) {
          result[key] = initializeDefaultValues(propSchema);
        } else if (propSchema.type === 'array') {
          result[key] = [];
        } else if (propSchema.type === 'boolean') {
          result[key] = false;
        } else if (
          propSchema.type === 'number' ||
          propSchema.type === 'integer'
        ) {
          result[key] = propSchema.default ?? '';
        } else {
          result[key] = '';
        }
      } else if (
        propSchema.type === 'object' &&
        propSchema.properties &&
        typeof result[key] === 'object'
      ) {
        result[key] = initializeDefaultValues(
          propSchema,
          result[key] as ParameterValues
        );
      }
    });
  }

  return result;
}

const presenceFields = (condition?: PresenceCondition): string[] =>
  condition
    ? [
        ...(condition.required ?? []),
        ...(condition.anyOf ?? []).flatMap(presenceFields),
        ...presenceFields(condition.not),
      ]
    : [];

function matchesPresence(
  condition: PresenceCondition,
  isFilled: (key: string) => boolean
): boolean {
  return (
    (condition.required ?? []).every(isFilled) &&
    (condition.anyOf?.some((entry) => matchesPresence(entry, isFilled)) ?? true) &&
    !(condition.not && matchesPresence(condition.not, isFilled))
  );
}

/**
 * Validate oneOf constraints anywhere in the schema. Valid when exactly one
 * option is satisfied as the gateway evaluates it (required set, `not` unmet).
 * Otherwise the option the user started (a field unique to it is set) drives
 * the messages. An optional nested object is checked only once it holds a value.
 */
export function validateOneOf(
  schema: ParameterSchema,
  values: ParameterValues,
  parentPath: string = '',
  isRequired: boolean = true
): ValidationError[] {
  const errors: ValidationError[] = [];
  if (schema.type !== 'object' || !schema.properties) return errors;

  const pathOf = (key: string) => (parentPath ? `${parentPath}.${key}` : key);
  const isFilled = (key: string) => hasConfiguredValue(getValueByPath(values, pathOf(key)));
  const add = (keys: string[], message: string) =>
    keys.forEach((key) => errors.push({ path: pathOf(key), message }));
  const conflict = (groups: string[][]) =>
    groups.forEach((group) =>
      add(
        group.filter(isFilled),
        `Cannot be combined with ${formatOneOfGroups(groups.filter((g) => g !== group))}`
      )
    );

  const options = getSupportedOneOfOptions(schema);
  const groups = options.map((option) => option.required!);
  const satisfied = groups.filter((_, i) => matchesPresence(options[i], isFilled));

  if (satisfied.length > 1) {
    conflict(satisfied);
  } else if (options.length > 0 && satisfied.length === 0) {
    const started = options.filter(({ required: group }) => {
      const unique = group!.filter((key) => !groups.some((g) => g !== group && g.includes(key)));
      return (unique.length > 0 ? unique : group!).some(isFilled);
    });
    if (started.length === 0) {
      if (isRequired) {
        add(groups.map((group) => group[0]), `Configure ${formatOneOfGroups(groups)}`);
      }
    } else if (started.length > 1) {
      conflict(started.map((option) => option.required!));
    } else {
      const group = started[0].required!;
      const missing = group.filter((key) => !isFilled(key));
      // A complete group failing its `not`: name the set fields it rules out
      const excluded = [...new Set(presenceFields(started[0].not))].filter(
        (key) => isFilled(key) && !group.includes(key)
      );
      if (missing.length > 0) {
        add(missing, 'This field is required');
      } else if (excluded.length === 0) {
        add(group, 'This combination of fields is not allowed');
      } else {
        add(group, `Cannot be combined with ${formatOneOfGroups([excluded])}`);
        add(
          excluded.filter((key) => schema.properties![key]),
          `Cannot be combined with ${formatOneOfGroups([group])}`
        );
      }
    }
  }

  const required = schema.required ?? [];
  Object.entries(schema.properties).forEach(([key, propSchema]) => {
    const path = pathOf(key);
    if (propSchema.type === 'object' && propSchema.properties) {
      // A set optional object is submitted, so its oneOf applies
      const nestedRequired = (isRequired && required.includes(key)) || isFilled(key);
      errors.push(...validateOneOf(propSchema, values, path, nestedRequired));
    } else if (propSchema.type === 'array' && propSchema.items?.properties) {
      const items = getValueByPath(values, path);
      if (Array.isArray(items)) {
        items.forEach((_, i) =>
          errors.push(...validateOneOf(propSchema.items!, values, `${path}.${i}`))
        );
      }
    }
  });

  return errors;
}

/** Whether the schema's top-level oneOf, if any, has a fully set group. */
export function hasCompleteOneOfGroup(schema: ParameterSchema, values: ParameterValues): boolean {
  const groups = getOneOfGroups(schema);
  return (
    groups.length === 0 ||
    groups.some((group) => group.every((key) => hasConfiguredValue(getValueByPath(values, key))))
  );
}

/**
 * Coerce a single value to its declared schema type.
 * Template expressions (e.g. {{ env "LIMIT" }}) are left as strings.
 */
function coerceValue(value: unknown, schema: ParameterSchema): unknown {
  if (value === null || value === undefined) return value;

  if (isTemplateExpression(value)) return value;

  switch (schema.type) {
    case 'boolean': {
      if (typeof value === 'boolean') return value;
      if (typeof value === 'string') {
        if (value.toLowerCase() === 'true') return true;
        if (value.toLowerCase() === 'false') return false;
      }
      return value;
    }
    case 'number': {
      if (typeof value === 'number') return value;
      if (typeof value === 'string' && value !== '') {
        const parsed = parseFloat(value);
        if (!Number.isNaN(parsed)) return parsed;
      }
      return value;
    }
    case 'integer': {
      if (typeof value === 'number') return value;
      if (typeof value === 'string' && value !== '') {
        const parsed = parseInt(value, 10);
        if (!Number.isNaN(parsed)) return parsed;
      }
      return value;
    }
    case 'object': {
      if (typeof value === 'object' && !Array.isArray(value)) return value;
      if (typeof value === 'string' && value === '') return {};
      return value;
    }
    default:
      return value;
  }
}

/**
 * Walk the schema and coerce all values to their declared types.
 * Ensures booleans, numbers, integers, and objects are not accidentally
 * submitted as strings.
 */
export function coerceValuesToSchemaTypes(
  schema: ParameterSchema,
  values: ParameterValues
): ParameterValues {
  if (schema.type !== 'object' || !schema.properties) return values;

  const result: ParameterValues = { ...values };

  Object.entries(schema.properties).forEach(([key, propSchema]) => {
    const val = result[key];
    if (val === undefined) return;

    if (
      propSchema.type === 'object' &&
      propSchema.properties &&
      typeof val === 'object' &&
      val !== null &&
      !Array.isArray(val)
    ) {
      result[key] = coerceValuesToSchemaTypes(
        propSchema,
        val as ParameterValues
      );
    } else if (
      propSchema.type === 'array' &&
      Array.isArray(val) &&
      propSchema.items
    ) {
      result[key] = val.map((item) => {
        if (
          propSchema.items!.type === 'object' &&
          propSchema.items!.properties &&
          typeof item === 'object' &&
          item !== null
        ) {
          return coerceValuesToSchemaTypes(
            propSchema.items!,
            item as ParameterValues
          );
        }
        return coerceValue(item, propSchema.items!);
      });
    } else {
      result[key] = coerceValue(val, propSchema);
    }
  });

  return result;
}

/**
 * Remove empty optional values before policy parameters are submitted.
 *
 * Optional schema properties must be omitted when they are not configured.
 * Sending an empty value still makes the property present and causes schema
 * constraints such as minLength or minItems to be evaluated by the gateway.
 * Required empty values are retained so the form validation can report them.
 */
export function omitOptionalEmptyValues(
  schema: ParameterSchema,
  values: ParameterValues
): ParameterValues {
  if (schema.type !== 'object' || !schema.properties) return values;

  const required = new Set(schema.required ?? []);
  const result: ParameterValues = {};

  Object.entries(values).forEach(([key, originalValue]) => {
    const propSchema = schema.properties?.[key];

    // Preserve values not described by the schema for parameter objects that
    // allow arbitrary additional properties.
    if (!propSchema) {
      result[key] = originalValue;
      return;
    }

    let value = originalValue;

    if (
      propSchema.type === 'object' &&
      propSchema.properties &&
      typeof value === 'object' &&
      value !== null &&
      !Array.isArray(value)
    ) {
      value = omitOptionalEmptyValues(
        propSchema,
        value as ParameterValues
      );
    } else if (
      propSchema.type === 'array' &&
      propSchema.items?.type === 'object' &&
      propSchema.items.properties &&
      Array.isArray(value)
    ) {
      value = value.map((item) => {
        if (
          typeof item !== 'object' ||
          item === null ||
          Array.isArray(item)
        ) {
          return item;
        }

        return omitOptionalEmptyValues(
          propSchema.items!,
          item as ParameterValues
        );
      });
    }

    const isEmptyObject =
      typeof value === 'object' &&
      value !== null &&
      !Array.isArray(value) &&
      Object.keys(value as ParameterValues).length === 0;
    const isEmpty =
      value === undefined ||
      value === null ||
      value === '' ||
      (Array.isArray(value) && value.length === 0) ||
      isEmptyObject;

    if (isEmpty && !required.has(key)) return;

    result[key] = value;
  });

  return result;
}

/**
 * Create a new array item with default values based on item schema
 */
export function createDefaultArrayItem(itemSchema: ParameterSchema): unknown {
  if (itemSchema.type === 'object' && itemSchema.properties) {
    return initializeDefaultValues(itemSchema);
  }
  if (itemSchema.type === 'string') {
    return itemSchema.default ?? '';
  }
  if (itemSchema.type === 'boolean') {
    return itemSchema.default ?? false;
  }
  if (itemSchema.type === 'number' || itemSchema.type === 'integer') {
    return itemSchema.default ?? 0;
  }
  if (itemSchema.type === 'array') {
    return [];
  }
  return null;
}
