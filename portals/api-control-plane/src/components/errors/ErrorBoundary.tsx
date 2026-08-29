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

import { Component, type ErrorInfo, type ReactNode } from 'react';

import { AppErrorFallback } from './ErrorFallback';

type ErrorBoundaryProps = {
  children: ReactNode;
  fallback?: (error: Error, reset: () => void) => ReactNode;
  /**
   * Values that, when any of them changes, clear a caught error and re-render
   * `children`.
   *
   * The page-level boundary passes the current pathname: without it the
   * fallback stays latched after the user navigates away, so the shell's
   * sidebar would appear to be broken too.
   */
  resetKeys?: readonly unknown[];
};

type ErrorBoundaryState = {
  error?: Error;
  /** The `resetKeys` this state was derived against; compared, never rendered. */
  resetKeys: readonly unknown[];
};

const EMPTY_RESET_KEYS: readonly unknown[] = [];

const sameResetKeys = (a: readonly unknown[], b: readonly unknown[]) =>
  a.length === b.length && a.every((value, index) => Object.is(value, b[index]));

/**
 * Catches a render-time throw in its subtree and renders a fallback in its
 * place, so the failure is contained to that subtree rather than unmounting
 * everything above it.
 *
 * Mounted at two levels, and the difference matters:
 *
 * - Around the routed page in `AppLayout`, which is what keeps a page fault
 *   from taking the header, sidebar and footer down with it. This is the one
 *   that catches almost everything in practice, including a `lazy()` chunk that
 *   fails to load after a deploy.
 * - At the top of `App`, above `BrowserRouter`, as the last resort for anything
 *   that throws outside the routed page (the auth provider, the scope provider,
 *   the shell itself).
 *
 * It does not catch errors thrown from event handlers, timers, or unawaited
 * promises, React boundaries never do. Those still need handling at the call
 * site.
 */
export class ErrorBoundary extends Component<
  ErrorBoundaryProps,
  ErrorBoundaryState
> {
  state: ErrorBoundaryState = { resetKeys: EMPTY_RESET_KEYS };

  static getDerivedStateFromError(error: Error): Partial<ErrorBoundaryState> {
    return { error };
  }

  /**
   * Drops a caught error once `resetKeys` change.
   *
   * Done here rather than in an effect because a boundary showing its fallback
   * renders no children, so a child effect can never run to clear it — the
   * decision has to happen during the render that follows the key change.
   */
  static getDerivedStateFromProps(
    props: ErrorBoundaryProps,
    state: ErrorBoundaryState
  ): Partial<ErrorBoundaryState> | null {
    const keys = props.resetKeys ?? EMPTY_RESET_KEYS;
    if (sameResetKeys(keys, state.resetKeys)) return null;
    return { error: undefined, resetKeys: keys };
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    // Kept in every build: the fallback shows the user sterile copy, so the
    // console is the only place the real message and component stack survive.
    // eslint-disable-next-line no-console
    console.error('Unhandled error in oxygen-console', error, info);
  }

  reset = () => {
    this.setState({ error: undefined });
  };

  render() {
    const { error } = this.state;
    const { children, fallback } = this.props;

    if (!error) return children;
    if (fallback) return fallback(error, this.reset);

    return <AppErrorFallback error={error} reset={this.reset} />;
  }
}
