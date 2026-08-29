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

// The BFF checks for this header on every state-mutating request (GET/HEAD/OPTIONS
// are exempt) — a fixed contract between the BFF and this SPA, not something either
// side can vary independently. Must match api-control-plane-bff's CSRFHeaderName.
export const CSRF_HEADER = 'X-Requested-By';
export const CSRF_HEADER_VALUE = 'api-control-plane';
