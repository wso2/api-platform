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

import type { UiSchema } from '@rjsf/utils';
import { ParameterValues } from '../types';

/*
 * A policy's uiSchema can come from a user-authored custom policy, so only a
 * known-safe subset is kept. Options that pass props or styles straight into
 * MUI components, swap in fields or templates, or enable Markdown are dropped.
 */
const ALLOWED_OPTIONS = new Set([
  'title',
  'description',
  'help',
  'placeholder',
  'order',
  'enumNames',
  'enumOrder',
  'enumDisabled',
  'readonly',
  'disabled',
  'hideError',
  'label',
  'rows',
  'emptyValue',
  'inline',
  'addable',
  'removable',
  'orderable',
  'copyable',
  'enableOptionalDataFieldForType',
]);

const ALLOWED_WIDGETS = new Set([
  'text',
  'textarea',
  'select',
  'radio',
  'checkbox',
  'checkboxes',
  'updown',
  'hidden',
]);

const UNSAFE_KEYS = new Set(['__proto__', 'constructor', 'prototype']);

function isPlainObject(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function isSafeValue(value: unknown): boolean {
  // Options only take plain data; nested objects would be spread into props.
  if (Array.isArray(value)) {
    return value.every((v) => ['string', 'number', 'boolean'].includes(typeof v));
  }
  if (['string', 'number', 'boolean'].includes(typeof value)) {
    return true;
  }
  // e.g. ui:enumNames given as a value-to-label map.
  return isPlainObject(value) && Object.values(value).every((v) => typeof v === 'string');
}

function sanitizeNode(node: Record<string, unknown>): Record<string, unknown> {
  const out: Record<string, unknown> = {};
  for (const [key, value] of Object.entries(node)) {
    if (UNSAFE_KEYS.has(key)) {
      continue;
    }
    if (key === 'ui:widget') {
      if (typeof value === 'string' && ALLOWED_WIDGETS.has(value)) {
        out[key] = value;
      }
    } else if (key === 'ui:options') {
      if (isPlainObject(value)) {
        const options: Record<string, unknown> = {};
        for (const [option, optionValue] of Object.entries(value)) {
          if (option === 'widget') {
            if (typeof optionValue === 'string' && ALLOWED_WIDGETS.has(optionValue)) {
              options.widget = optionValue;
            }
          } else if (ALLOWED_OPTIONS.has(option) && isSafeValue(optionValue)) {
            options[option] = optionValue;
          }
        }
        out[key] = options;
      }
    } else if (key.startsWith('ui:')) {
      if (ALLOWED_OPTIONS.has(key.slice(3)) && isSafeValue(value)) {
        out[key] = value;
      }
    } else if (isPlainObject(value)) {
      // A property name, or `items`, holding the uiSchema for that field.
      out[key] = sanitizeNode(value);
    }
  }
  return out;
}

/**
 * Builds the uiSchema for a rich policy form: the policy's uiSchema reduced to
 * safe options, plus the form's own settings (no RJSF submit button, no Markdown,
 * and no add/remove/reorder buttons when read-only).
 */
export function buildUiSchema(raw: Record<string, unknown> | undefined, readOnly: boolean): UiSchema<ParameterValues> {
  const uiSchema = raw ? sanitizeNode(raw) : {};
  uiSchema['ui:submitButtonOptions'] = { norender: true };
  uiSchema['ui:globalOptions'] = {
    enableMarkdownInDescription: false,
    enableMarkdownInHelp: false,
    ...(readOnly ? { addable: false, removable: false, orderable: false, copyable: false } : {}),
  };
  return uiSchema as UiSchema<ParameterValues>;
}
