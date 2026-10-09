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
 * A stand-in signal that the user has tested an API, so the overview's
 * progress banner can mark its Test step done.
 *
 * The real signal would be the gateway seeing a first successful request
 * (wso2-enterprise/apim-saas#2673). Until that exists, the step completes when
 * the user copies a ready-to-run command from the Test page, the last thing
 * the console can observe before the request leaves for the terminal. It is
 * per browser by design: a hint for this user's progress, not a record.
 */

const KEY_PREFIX = 'apiControlPlane.tested.';

/** Records that the API's test command was copied. Never throws. */
export const markApiTested = (apiId: string): void => {
  try {
    window.localStorage.setItem(`${KEY_PREFIX}${apiId}`, '1');
  } catch {
    // Storage can be unavailable (private mode, blocked site data); the banner
    // then simply keeps showing Test as the next step.
  }
};

/** Whether the API's test command has been copied in this browser. */
export const wasApiTested = (apiId: string): boolean => {
  try {
    return window.localStorage.getItem(`${KEY_PREFIX}${apiId}`) === '1';
  } catch {
    return false;
  }
};
