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

import yaml from 'js-yaml';
import { PolicyDefinition, ParameterSchema } from './types';

/**
 * Raw policy definition as parsed from YAML
 */
interface RawPolicyYaml {
  name: string;
  version: string;
  description: string;
  parameters?: ParameterSchema;
  systemParameters?: ParameterSchema;
}

/**
 * Parse a YAML string into a PolicyDefinition
 */
export function parsePolicyYaml(yamlContent: string): PolicyDefinition {
  const parsed = yaml.load(yamlContent) as RawPolicyYaml;

  if (!parsed.name) {
    throw new Error('Policy definition must have a name');
  }

  if (!parsed.version) {
    throw new Error('Policy definition must have a version. eg: 1.0.0');
  }

  // Ensure parameters has a valid schema structure
  const parameters: ParameterSchema = parsed.parameters || {
    type: 'object',
    properties: {},
  };

  return {
    name: parsed.name,
    version: parsed.version,
    description: parsed.description || '',
    parameters,
    systemParameters: parsed.systemParameters,
  };
}

/**
 * Parse multiple YAML policy definitions from a combined string
 * (separated by ---)
 */
export function parseMultiplePolicyYaml(
  yamlContent: string
): PolicyDefinition[] {
  const documents = yaml.loadAll(yamlContent) as RawPolicyYaml[];
  return documents.map((doc) => parsePolicyYaml(yaml.dump(doc)));
}

/**
 * Validate that a parsed schema has the expected structure
 */
export function validateParameterSchema(schema: ParameterSchema): string[] {
  const errors: string[] = [];

  if (!schema.type) {
    errors.push('Schema must have a type');
  }

  if (
    schema.type === 'object' &&
    schema.properties === undefined &&
    schema.type !== 'object'
  ) {
    errors.push('Object schema should have properties');
  }

  if (schema.type === 'array' && !schema.items) {
    errors.push('Array schema must have items definition');
  }

  // Recursively validate nested schemas
  if (schema.properties) {
    Object.entries(schema.properties).forEach(([key, propSchema]) => {
      const propErrors = validateParameterSchema(propSchema);
      propErrors.forEach((err) => errors.push(`${key}: ${err}`));
    });
  }

  if (schema.items) {
    const itemErrors = validateParameterSchema(schema.items);
    itemErrors.forEach((err) => errors.push(`items: ${err}`));
  }

  return errors;
}
