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
import { OutlinedInput } from '@wso2/oxygen-ui';

export type ReadOnlyFieldProps = {
  /** The field's accessible name; a visible label above it is the caller's. */
  label: string;
  value: string;
  id?: string;
  /** For keys and other values read as code. */
  monospace?: boolean;
};

/**
 * A value shown beside the fields it belongs to, not edited: the size and
 * border of a small input, on a subtle fill.
 */
export function ReadOnlyField({ label, value, id, monospace }: ReadOnlyFieldProps) {
  return (
    <OutlinedInput
      fullWidth
      id={id}
      inputProps={{ 'aria-label': label, readOnly: true, spellCheck: false }}
      size="small"
      sx={{ bgcolor: 'action.hover', ...(monospace ? { fontFamily: 'monospace' } : {}) }}
      value={value}
    />
  );
}
