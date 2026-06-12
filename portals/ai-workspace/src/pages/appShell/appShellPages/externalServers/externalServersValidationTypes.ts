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

export type ResourceItem = {
  uri: string;
  name?: string;
  title?: string;
  mimeType?: string;
  blob?: string;
};

export type PromptArgument = {
  description: string;
  name: string;
  required: boolean;
};

export type PromptItem = {
  description: string;
  name: string;
  arguments?: PromptArgument[];
};

export type ToolInputSchema = {
  $schema?: string;
  additionalProperties?: boolean;
  properties: Record<string, unknown>;
  required?: string[];
  type: string;
};

export type ToolItem = {
  description: string;
  inputSchema: ToolInputSchema;
  name: string;
};

export type EndpointValidationResponse = {
  endpointUrl: string;
  prompts: PromptItem[];
  resources: ResourceItem[];
  serverInfo: {
    name: string;
    version: string;
  };
  tools: ToolItem[];
};
