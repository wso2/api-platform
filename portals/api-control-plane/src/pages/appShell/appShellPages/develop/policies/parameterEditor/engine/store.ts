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

import type {
  BuildOptions,
  FormState,
  MapEntry,
  Path,
  PolicyDefinition,
  PolicyFormModel,
  Resolution,
} from './types';
import { buildModel } from './model';
import { coerceInput, emptyValue, fieldAt, initialState, toState } from './state';
import { resolve } from './resolve';
import { getIn, pathKey, setIn } from './utils';

export interface PolicyFormOptions extends BuildOptions {
  /** an existing config to edit */
  initialConfig?: Record<string, unknown>;
}

export interface PolicyForm {
  model: PolicyFormModel;
  getState(): FormState;
  /** memoised: same object until the state changes */
  getResolution(): Resolution;
  /** Set a value from an input. Strings are coerced to the field's type; '' clears it. */
  setValue(path: Path, raw: unknown): void;
  /** Turn an optional section on (filled with defaults) or off. */
  setEnabled(path: Path, enabled: boolean): void;
  setChoice(instanceKey: string, optionIds: string[]): void;
  addItem(listPath: Path): void;
  removeItem(listPath: Path, index: number): void;
  moveItem(listPath: Path, from: number, to: number): void;
  /** maps */
  addEntry(mapPath: Path): void;
  setEntryKey(mapPath: Path, index: number, key: string): void;
  /** Replace everything, e.g. when switching policies or loading a saved config. */
  reset(config?: Record<string, unknown>): void;
  subscribe(listener: () => void): () => void;
}

export function createPolicyForm(
  definition: PolicyDefinition | PolicyFormModel,
  options: PolicyFormOptions = {},
): PolicyForm {
  const model: PolicyFormModel =
    'root' in definition && 'fields' in definition
      ? (definition as PolicyFormModel)
      : buildModel(definition as PolicyDefinition, options);
  let state = initialState(model, options.initialConfig);
  let cached: Resolution | null = null;
  const listeners = new Set<() => void>();

  const commit = (next: FormState) => {
    state = next;
    cached = null;
    listeners.forEach((l) => l());
  };
  const setValues = (path: Path, v: unknown) =>
    commit({ ...state, values: (setIn(state.values, path, v) ?? {}) as Record<string, unknown> });

  return {
    model,
    getState: () => state,
    getResolution() {
      if (!cached) cached = resolve(model, state);
      return cached;
    },
    setValue(path, raw) {
      const f = fieldAt(model, path);
      setValues(path, f ? coerceInput(f, raw) : raw);
    },
    setEnabled(path, enabled) {
      const f = fieldAt(model, path);
      if (!f) return;
      setValues(path, enabled ? emptyValue(f) : undefined);
    },
    setChoice(instanceKey, optionIds) {
      commit({ ...state, choices: { ...state.choices, [instanceKey]: optionIds } });
    },
    addItem(listPath) {
      const f = fieldAt(model, listPath);
      if (!f?.item) return;
      const arr = (getIn(state.values, listPath) as unknown[]) ?? [];
      const blank =
        f.kind === 'objectList'
          ? emptyValue(f.item)
          : f.kind === 'multiselect'
            ? f.options?.[0]?.value
            : undefined;
      setValues(listPath, [...arr, blank]);
    },
    removeItem(listPath, index) {
      const arr = ((getIn(state.values, listPath) as unknown[]) ?? []).slice();
      arr.splice(index, 1);
      // choices for items after the removed one would shift; drop choices under this list
      const prefix = `${pathKey(listPath)}[`;
      const choices = Object.fromEntries(
        Object.entries(state.choices).filter(([k]) => !k.startsWith(prefix)),
      );
      commit({ choices, values: setIn(state.values, listPath, arr) as Record<string, unknown> });
    },
    moveItem(listPath, from, to) {
      const arr = ((getIn(state.values, listPath) as unknown[]) ?? []).slice();
      if (to < 0 || to >= arr.length) return;
      const [x] = arr.splice(from, 1);
      arr.splice(to, 0, x);
      setValues(listPath, arr);
    },
    addEntry(mapPath) {
      const f = fieldAt(model, mapPath);
      if (!f?.valueField) return;
      const arr = (getIn(state.values, mapPath) as MapEntry[]) ?? [];
      setValues(mapPath, [...arr, { key: '', value: emptyValue(f.valueField) }]);
    },
    setEntryKey(mapPath, index, key) {
      setValues([...mapPath, index, 'key'], key);
    },
    reset(config) {
      commit(initialState(model, config));
    },
    subscribe(listener) {
      listeners.add(listener);
      return () => listeners.delete(listener);
    },
  };
}

export { toState };
