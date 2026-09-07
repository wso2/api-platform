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

import type { ReactNode } from 'react';
import { type MessageDescriptor } from 'react-intl';

/**
 * Catalog for the API creation wizard's first step.
 *
 * Every option the platform will ever offer is listed here, released or not:
 * `enabled: false` keeps an option visible but unpickable, so the wizard can
 * show the full shape of the product instead of hiding what is coming.
 */

export type ApiType = {
  key: string;
  title: MessageDescriptor;
  description: MessageDescriptor;
  icon: ReactNode;
  enabled: boolean;
};

export interface UpstreamAuth {
  type: 'basic' | 'bearer' | 'none';
  header: string;
  value: string;
}

export interface UpStreamTarget {
  url: string;
  ref?: string;
  auth?: UpstreamAuth;
}

export interface Operationrequest {
  method: 'GET' | 'POST' | 'PUT' | 'DELETE' | 'PATCH';
  path: string;
}

export interface ApiOperation {
  name: string;
  description?: string;
  request: Operationrequest;
}

export interface GeneralApiCreationFormState {
  id: string;
  displayName: string;
  description?: string;
  version: string;
  context: string;
  readOnly: boolean;
  upstream: {
    main: UpStreamTarget;
    sandbox?: UpStreamTarget;
  };
  lifeCycleStatus: 'CREATED'; // for
  kind: 'RestApis'; // currently support only RestApis
  transports: Array<'http' | 'https'>;
  operations: ApiOperation[];
}

// This is what Api Creation wizard holds and all the sub compoennt emits.
export type ApiCreationWizardDraftState = Partial<GeneralApiCreationFormState>;
