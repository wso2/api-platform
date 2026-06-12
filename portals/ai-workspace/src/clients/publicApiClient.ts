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

import { API_BASE_URLS } from '../config.env';
import { logger } from '../utils/logger';
import { HttpMethod, ApiRequestConfig } from '../utils/types';

export type { HttpMethod, ApiRequestConfig };

/**
 * Default headers for API requests
 */
const getDefaultHeaders = (): Record<string, string> => ({
  Accept: 'application/json',
  'Content-Type': 'application/json',
});

/**
 * Build the full URL with query parameters
 */
const buildUrl = (baseUrl: string, path: string, params?: Record<string, unknown>): string => {
  // For absolute URLs (starting with http/https), use as-is
  // For relative paths, append to base URL
  const fullUrl = path.startsWith('http') ? path : `${baseUrl}${path}`;
  
  if (!params || Object.keys(params).length === 0) {
    return fullUrl;
  }
  
  const url = new URL(fullUrl);
  Object.entries(params).forEach(([key, value]) => {
    if (value !== undefined && value !== null) {
      url.searchParams.append(key, String(value));
    }
  });
  
  return url.toString();
};

/**
 * Make a public HTTP request using native fetch (no authentication)
 * This is for public APIs that don't require auth tokens
 * 
 * @param config - Request configuration
 * @returns Promise with the response data
 */
export const request = async <T>(config: ApiRequestConfig): Promise<T> => {
  const { path, method, data, params, headers, baseUrl } = config;
  
  // Use the provided baseUrl or default to policyHubApi
  const resolvedBaseUrl = baseUrl || API_BASE_URLS.policyHubApi;
  const url = buildUrl(resolvedBaseUrl, path, params);
  
  const requestInit: RequestInit = {
    method,
    headers: {
      ...getDefaultHeaders(),
      ...headers,
    },
  };
  
  if (data && (method === 'POST' || method === 'PUT' || method === 'PATCH')) {
    requestInit.body = JSON.stringify(data);
  }
  
  try {
    logger.info('Public API request:', method, url);
    const response = await fetch(url, requestInit);
    
    if (!response.ok) {
      throw new Error(`HTTP error! status: ${response.status}`);
    }
    
    const responseData = await response.json();
    logger.info('Public API response:', responseData);
    return responseData as T;
  } catch (error) {
    logger.error('Public API request failed:', error);
    throw error;
  }
};

/**
 * Make a public HTTP request and return a raw text response
 */
export const requestText = async (config: ApiRequestConfig): Promise<string> => {
  const { path, method, data, params, headers, baseUrl } = config;

  const resolvedBaseUrl = baseUrl || API_BASE_URLS.policyHubApi;
  const url = buildUrl(resolvedBaseUrl, path, params);

  const requestInit: RequestInit = {
    method,
    headers: {
      ...getDefaultHeaders(),
      ...headers,
    },
  };

  if (data && (method === 'POST' || method === 'PUT' || method === 'PATCH')) {
    requestInit.body = JSON.stringify(data);
  }

  try {
    logger.info('Public API request (text):', method, url);
    const response = await fetch(url, requestInit);

    if (!response.ok) {
      throw new Error(`HTTP error! status: ${response.status}`);
    }

    const responseText = await response.text();
    logger.info('Public API response (text) received.');
    return responseText;
  } catch (error) {
    logger.error('Public API request (text) failed:', error);
    throw error;
  }
};

/**
 * Convenience method for GET requests
 */
export const get = async <T>(
  path: string,
  params?: Record<string, unknown>,
  baseUrl?: string
): Promise<T> => {
  return request<T>({
    path,
    method: 'GET',
    params,
    baseUrl,
  });
};

/**
 * Convenience method for GET requests returning raw text
 */
export const getText = async (
  path: string,
  params?: Record<string, unknown>,
  baseUrl?: string,
  headers?: Record<string, string>
): Promise<string> => {
  return requestText({
    path,
    method: 'GET',
    params,
    baseUrl,
    headers,
  });
};

/**
 * Convenience method for POST requests
 */
export const post = async <T>(
  path: string,
  data?: unknown,
  baseUrl?: string
): Promise<T> => {
  return request<T>({
    path,
    method: 'POST',
    data,
    baseUrl,
  });
};

/**
 * Convenience method for PUT requests
 */
export const put = async <T>(
  path: string,
  data?: unknown,
  baseUrl?: string
): Promise<T> => {
  return request<T>({
    path,
    method: 'PUT',
    data,
    baseUrl,
  });
};

/**
 * Convenience method for DELETE requests
 */
export const del = async <T>(
  path: string,
  params?: Record<string, unknown>,
  baseUrl?: string
): Promise<T> => {
  return request<T>({
    path,
    method: 'DELETE',
    params,
    baseUrl,
  });
};

/**
 * Convenience method for PATCH requests
 */
export const patch = async <T>(
  path: string,
  data?: unknown,
  baseUrl?: string
): Promise<T> => {
  return request<T>({
    path,
    method: 'PATCH',
    data,
    baseUrl,
  });
};

// Export a default client object for convenience
const publicApiClient = {
  request,
  requestText,
  get,
  getText,
  post,
  put,
  del,
  patch,
  API_BASE_URLS,
};

export default publicApiClient;
