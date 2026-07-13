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

import { DEBUG } from '../config.env';

/**
 * Logger utility that respects the APIP_AIW_DEBUG environment variable.
 * All console output is suppressed when DEBUG is disabled (default).
 * 
 * Usage:
 * ```ts
 * import { logger } from '../utils/logger';
 * 
 * logger.log('Debug message');
 * logger.warn('Warning message');
 * logger.error('Error message');
 * logger.info('Info message');
 * logger.debug('Debug message');
 * logger.group('Group label');
 * logger.groupEnd();
 * logger.table(data);
 * ```
 */

type LogLevel = 'log' | 'warn' | 'error' | 'info' | 'debug';

const isDebugEnabled = (): boolean => {
  // DEBUG is already converted to boolean by getEnvOrDefault
  // but also handle string values for runtime config edge cases
  if (typeof DEBUG === 'string') {
    return (DEBUG as string).toLowerCase() === 'true';
  }
  return Boolean(DEBUG);
};

const noop = (): void => {};

const createLogMethod = (level: LogLevel) => {
  return (...args: unknown[]): void => {
    if (isDebugEnabled()) {
      console[level](...args);
    }
  };
};

export const logger = {
  /** Log general messages */
  log: createLogMethod('log'),
  
  /** Log warning messages */
  warn: createLogMethod('warn'),
  
  /** Log error messages */
  error: createLogMethod('error'),
  
  /** Log info messages */
  info: createLogMethod('info'),
  
  /** Log debug messages */
  debug: createLogMethod('debug'),
  
  /** Start a collapsed console group */
  group: (label?: string): void => {
    if (isDebugEnabled()) {
      console.group(label);
    }
  },
  
  /** Start a collapsed console group */
  groupCollapsed: (label?: string): void => {
    if (isDebugEnabled()) {
      console.groupCollapsed(label);
    }
  },
  
  /** End a console group */
  groupEnd: (): void => {
    if (isDebugEnabled()) {
      console.groupEnd();
    }
  },
  
  /** Log tabular data */
  table: (data: unknown, columns?: string[]): void => {
    if (isDebugEnabled()) {
      console.table(data, columns);
    }
  },
  
  /** Log with time tracking */
  time: (label?: string): void => {
    if (isDebugEnabled()) {
      console.time(label);
    }
  },
  
  /** End time tracking and log result */
  timeEnd: (label?: string): void => {
    if (isDebugEnabled()) {
      console.timeEnd(label);
    }
  },
  
  /** Log stack trace */
  trace: (...args: unknown[]): void => {
    if (isDebugEnabled()) {
      console.trace(...args);
    }
  },
  
  /** Assert a condition */
  assert: (condition?: boolean, ...args: unknown[]): void => {
    if (isDebugEnabled()) {
      console.assert(condition, ...args);
    }
  },
  
  /** Count occurrences */
  count: (label?: string): void => {
    if (isDebugEnabled()) {
      console.count(label);
    }
  },
  
  /** Reset count */
  countReset: (label?: string): void => {
    if (isDebugEnabled()) {
      console.countReset(label);
    }
  },
  
  /** Clear console */
  clear: (): void => {
    if (isDebugEnabled()) {
      console.clear();
    }
  },
  
  /** Check if debug mode is enabled */
  isEnabled: isDebugEnabled,
};

export default logger;
