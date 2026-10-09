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

import type { TestCallMode } from '../../utils/callMode';

/**
 * Shares the call mode with the switch swagger renders beside Execute.
 *
 * A store rather than props, because swagger-ui-react builds its system once
 * and keeps the first `plugins` value it was given — so the component inside
 * that plugin is created at mount and can never be handed a newer `callMode`.
 * The same constraint already forces the relay transport to read its context
 * through a ref; a ref is enough there because nothing re-renders, but this
 * one is rendered, so it needs to notify as well as hold.
 *
 * Subscription rather than React context: in the browser swagger resolves the
 * bundle that shares our React, but nothing guarantees a context provider
 * crosses into its tree, and a plain store sidesteps the question entirely.
 */
export type CallModeStore = {
  /** `useSyncExternalStore` subscribe. Returns the unsubscribe function. */
  subscribe: (listener: () => void) => () => void;
  /** `useSyncExternalStore` snapshot — the mode currently in force. */
  getSnapshot: () => TestCallMode;
  /** Publishes a new mode to subscribers. Called by the page, not the switch. */
  setMode: (mode: TestCallMode) => void;
  /** Asks the page to change mode. The page owns the state and persists it. */
  requestMode: (mode: TestCallMode) => void;
  /** Installs the handler `requestMode` delegates to. */
  setRequestHandler: (handler: (mode: TestCallMode) => void) => void;
};

export const createCallModeStore = (initial: TestCallMode): CallModeStore => {
  let mode = initial;
  let requestHandler: ((mode: TestCallMode) => void) | undefined;
  const listeners = new Set<() => void>();

  return {
    subscribe: (listener) => {
      listeners.add(listener);
      return () => listeners.delete(listener);
    },
    getSnapshot: () => mode,
    setMode: (next) => {
      // Guard the no-op: `setMode` is driven from an effect that runs on every
      // render of the viewer, and notifying unconditionally would re-render
      // every mounted switch for a value that did not change.
      if (next === mode) return;
      mode = next;
      listeners.forEach((listener) => listener());
    },
    requestMode: (next) => requestHandler?.(next),
    setRequestHandler: (handler) => {
      requestHandler = handler;
    },
  };
};
