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

/**
 * An environment's name becomes its OpenChoreo resource name, so it has to be a
 * DNS-1123 label: the backend rejects anything else. Checking it here means the
 * rule is visible while the name is being typed rather than after submitting.
 */

/** The longest name Kubernetes accepts for a single object. */
const MAX_ENVIRONMENT_NAME_LENGTH = 63;

const DNS1123_LABEL = /^[a-z0-9]([-a-z0-9]*[a-z0-9])?$/;

/**
 * Returns the message to show for `name`, or `undefined` when it is usable. An
 * empty name is left to the submit button's own disabled state rather than
 * reported as an error the moment the field is focused.
 */
export function validateEnvironmentName(name: string): string | undefined {
  const trimmed = name.trim();
  if (!trimmed) return undefined;

  if (trimmed.length > MAX_ENVIRONMENT_NAME_LENGTH) {
    return `Use at most ${MAX_ENVIRONMENT_NAME_LENGTH} characters.`;
  }
  if (!DNS1123_LABEL.test(trimmed)) {
    return 'Use only lowercase letters, numbers and hyphens, starting and ending with a letter or number.';
  }
  return undefined;
}
