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

import type { PolicyForm, ViewNode } from './engine';

/** Form values: the nested object the policy's parameters resolve to. */
export type ParameterValues = Record<string, unknown>;

/**
 * Props for field renderers. `node` is the engine's view of one field: its
 * schema, current value, whether it is visible/required, and its errors.
 */
export interface FieldRendererProps {
  node: ViewNode;
  form: PolicyForm;
  error?: string;
  disabled?: boolean;
}
