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

/*
 * Utility: getEnvOrDefault
 */

export const getEnvOrDefault = <T>(envKey: string, defaultValue: T): T => {
  // 1. Check runtime config (highest priority)
  const runtimeValue = typeof window !== 'undefined' && (window as any).__RUNTIME_CONFIG__
    ? (window as any).__RUNTIME_CONFIG__[envKey]
    : undefined;

  if (runtimeValue !== undefined && runtimeValue !== null && runtimeValue !== '') {
    // Handle boolean conversion
    if (typeof defaultValue === 'boolean') {
      return (runtimeValue === 'true' || runtimeValue === '1') as T;
    }

    // Handle number conversion
    if (typeof defaultValue === 'number') {
      const num = Number(runtimeValue);
      return (isNaN(num) ? defaultValue : num) as T;
    }

    return runtimeValue as T;
  }

  // 2. Check build-time environment variables
  const envValue = import.meta.env[envKey];

  if (envValue !== undefined && envValue !== null && envValue !== '') {
    // Handle boolean conversion
    if (typeof defaultValue === 'boolean') {
      return (envValue === 'true' || envValue === '1') as T;
    }

    // Handle number conversion
    if (typeof defaultValue === 'number') {
      const num = Number(envValue);
      return (isNaN(num) ? defaultValue : num) as T;
    }

    return envValue as T;
  }

  // 3. Return default value
  return defaultValue;
};
