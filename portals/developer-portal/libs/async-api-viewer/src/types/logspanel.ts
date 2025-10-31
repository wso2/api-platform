/*
 * Copyright (c) 2025, WSO2 LLC. (http://www.wso2.com). All Rights Reserved.
 *
 * This software is the property of WSO2 LLC. and its suppliers, if any.
 * Dissemination of any information or reproduction of any material contained
 * herein is strictly forbidden, unless permitted by WSO2 in accordance with
 * the WSO2 Commercial License available at http://wso2.com/licenses.
 * For specific language governing the permissions and limitations under
 * this license, please see the license as well as any agreement you’ve
 * entered into with WSO2 governing the purchase of this software and any
 * associated services.
 */

export const LOG_LEVEL_TRACE = 'TRACE';
export const LOG_LEVEL_DEBUG = 'DEBUG';
export const LOG_LEVEL_INFO = 'INFO';
export const LOG_LEVEL_WARN = 'WARN';
export const LOG_LEVEL_ERROR = 'ERROR';
export type LogLevel =
  | typeof LOG_LEVEL_TRACE
  | typeof LOG_LEVEL_DEBUG
  | typeof LOG_LEVEL_INFO
  | typeof LOG_LEVEL_WARN
  | typeof LOG_LEVEL_ERROR;
export const LOG_INFO = 'INFO';
export const LOG_WARN = 'WARN';
export const LOG_ERROR = 'ERROR';

export const LOG_FAILED = 'FAILED';
export const LOG_SUCCEEDED = 'SUCCEEDED';
