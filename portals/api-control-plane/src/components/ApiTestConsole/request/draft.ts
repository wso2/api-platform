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

import type { ConsoleOperation } from '../types';
import { paramKey, type RequestDraft } from './buildRequest';

/** The starting point for an operation: spec examples filled in, nothing sent yet. */
export const initialDraft = (operation: ConsoleOperation): RequestDraft => {
  const params: Record<string, string> = {};
  operation.parameters.forEach((parameter) => {
    if (parameter.example !== undefined) params[paramKey(parameter)] = parameter.example;
  });

  const option = operation.body?.options[0];
  const formValues: Record<string, string> = {};
  option?.formFields.forEach((field) => {
    if (!field.binary && field.example !== '') formValues[field.name] = field.example;
  });

  // Ask for the first representation the operation documents, if it documents one.
  const accept = operation.responses.find((response) => response.mediaType)?.mediaType;

  return {
    params,
    headers: accept ? [{ id: 'accept', name: 'Accept', value: accept }] : [],
    bodyMediaType: option?.mediaType ?? '',
    bodyText: option?.example ?? '',
    formValues,
    files: {},
  };
};
