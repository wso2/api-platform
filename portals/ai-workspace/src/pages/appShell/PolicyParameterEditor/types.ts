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

/**
 * JSON Schema-like structure parsed from YAML policy definitions
 */
export interface ParameterSchema {
  type: 'object' | 'array' | 'string' | 'boolean' | 'number' | 'integer';
  description?: string;
  default?: unknown;
  enum?: string[];
  properties?: Record<string, ParameterSchema>;
  items?: ParameterSchema;
  required?: string[];
  additionalProperties?: ParameterSchema | boolean; // For dynamic key-value objects
  minLength?: number;
  maxLength?: number;
  pattern?: string;
  minItems?: number;
  maxItems?: number;
  minimum?: number;
  maximum?: number;
  'x-wso2-policy-advanced-param'?: boolean;
  anyOf?: Array<{
    required?: string[];
    properties?: Record<string, { const?: unknown }>;
  }>;
}

/**
 * Tree node for rendering the parameter schema
 */
export interface SchemaTreeNode {
  id: string; // Unique identifier for React keys
  path: string; // Full dot-notation path: "request.jsonPath" or "requestHeaders.0.action"
  name: string; // Display name (property key)
  schema: ParameterSchema;
  depth: number;
  isRequired: boolean;
  isExpanded?: boolean;
  children?: SchemaTreeNode[];
  // Array-specific
  isArrayContainer?: boolean;
  isArrayItem?: boolean;
  arrayIndex?: number;
  parentArrayPath?: string;
  isAdvanced?: boolean;
}

/**
 * Policy definition parsed from YAML
 */
export interface PolicyDefinition {
  name: string;
  version: string;
  description: string;
  parameters: ParameterSchema;
  systemParameters?: ParameterSchema;
}

/**
 * Form values - nested object structure
 */
export type ParameterValues = Record<string, unknown>;

/**
 * Validation error
 */
export interface ValidationError {
  path: string;
  message: string;
}

/**
 * Props for field renderers
 */
export interface FieldRendererProps {
  node: SchemaTreeNode;
  value: unknown;
  onChange: (path: string, value: unknown) => void;
  error?: string;
  disabled?: boolean;
}
