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

/** The slice of the host's Port this feature reads: the user the host already has from its session. */
export type UserProfileHostPort = {
  user?: { name?: string };
};

/**
 * The signed-in person's first name from the host's session (the platform
 * greets by first name only, as other platforms do), or `null` when there is
 * none to show (no user, no name, or a blank one), so the host keeps what it
 * already shows. Makes no request of its own: the host has read the session.
 */
export function nameFromPort(port: UserProfileHostPort): string | null {
  const name = port.user?.name;
  const firstName = typeof name === 'string' ? name.trim().split(/\s+/)[0] : '';
  return firstName || null;
}
