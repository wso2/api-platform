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

import type { ConfigValues, EditableField } from '../types';

/**
 * The value a field shows: the pending edit, else the stored value, else the
 * platform default. The default is display only and never becomes a draft.
 */
export const shownValue = (
  field: EditableField,
  values: ConfigValues,
  drafts: ConfigValues
): unknown => {
  if (field.path in drafts) return drafts[field.path];
  if (field.path in values) return values[field.path];
  return field.default;
};

/**
 * Whether a draft changes what the gateway runs with.
 *
 * On a path the organization has not set, emptying the input or picking the
 * default is not a change: the default still stands, and saving it would pin
 * today's platform default onto the binding for good.
 */
export const isChange = (
  field: EditableField,
  values: ConfigValues,
  draft: unknown
): boolean => {
  if (!(field.path in values)) {
    return draft !== '' && draft !== undefined && draft !== field.default;
  }
  return draft !== values[field.path];
};
