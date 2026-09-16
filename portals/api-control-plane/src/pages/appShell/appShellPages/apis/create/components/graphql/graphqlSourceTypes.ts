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

import type { GraphqlSchemaSource } from '../../types';

/**
 * What either half of the source step (`GraphqlUrlUploadForm`,
 * `GraphqlIntrospectionForm`) reports once its schema resolves — enough for
 * `GraphqlDefinePanel` to feed the explorer and, on Next, hand the wizard a
 * draft to prefill the configure step from.
 */
export type GraphqlResolvedSchema = {
  schemaSource: GraphqlSchemaSource;
  /** Resolved SDL text, however it was obtained. */
  sdl: string;
  /** Only set when `schemaSource` is `'url'`. */
  sdlUrl?: string;
  /** Only set when `schemaSource` is `'file'`. */
  sdlFile?: File;
  /** Only set when `schemaSource` is `'introspection'` — the endpoint that was queried. */
  endpointUrl?: string;
};
