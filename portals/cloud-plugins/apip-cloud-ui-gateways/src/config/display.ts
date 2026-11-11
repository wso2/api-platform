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

import type { EditableField } from '../types';

/** An enum value as the form shows it: the platform's label, else the value. */
export const optionLabel = (field: EditableField, value: string): string =>
  field.labels?.[value] ?? value;

/** The platform default as the form shows it, or `undefined` when none is published. */
export const defaultText = (field: EditableField): string | undefined => {
  if (field.default === undefined) return undefined;
  if (typeof field.default === 'boolean') return field.default ? 'On' : 'Off';
  if (field.type === 'enum') return optionLabel(field, String(field.default));
  return String(field.default);
};

/**
 * The line under a field: the platform's short hint, then what an unset field
 * runs with. The longer description is not part of it; it sits behind the label.
 */
export const helperText = (field: EditableField): string => {
  const shownDefault = defaultText(field);
  return [field.hint, shownDefault === undefined ? '' : `Default: ${shownDefault}.`]
    .filter(Boolean)
    .join(' ');
};
