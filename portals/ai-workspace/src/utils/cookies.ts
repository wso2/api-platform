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

const FIDP_COOKIE_KEY = 'choreo_fidp_id';

/**
 * Get a cookie value by name
 */
export const getCookie = (name: string): string | null => {
  const value = `; ${document.cookie}`;
  const parts = value.split(`; ${name}=`);
  if (parts.length === 2) {
    return parts.pop()?.split(';').shift() || null;
  }
  return null;
};

/**
 * Set a cookie with optional expiry (default 7 days)
 */
export const setCookie = (
  name: string,
  value: string,
  days: number = 7
): void => {
  const expires = new Date();
  expires.setTime(expires.getTime() + days * 24 * 60 * 60 * 1000);
  document.cookie = `${name}=${value};expires=${expires.toUTCString()};path=/;SameSite=Lax;Secure`;
};

/**
 * Delete a cookie
 */
export const deleteCookie = (name: string): void => {
  document.cookie = `${name}=;expires=Thu, 01 Jan 1970 00:00:00 GMT;path=/`;
};

/**
 * Get the stored federated IDP identifier (e.g., 'google', 'github')
 * Stored in a cookie so it's shared across browser tabs
 */
export const getFidpId = (): string | null => getCookie(FIDP_COOKIE_KEY);

/**
 * Store the federated IDP identifier
 */
export const setFidpId = (fidpId: string | undefined): void => {
  if (fidpId) {
    setCookie(FIDP_COOKIE_KEY, fidpId);
  }
};

/**
 * Remove the stored federated IDP identifier
 */
export const removeFidpId = (): void => {
  deleteCookie(FIDP_COOKIE_KEY);
};
