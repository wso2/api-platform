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
 * Whether a document body can be shown as Markdown and edited as text.
 *
 * The content endpoint returns raw bytes labelled with the stored content type,
 * so a future non-text format (PDF, DOCX) must not be rendered — or worse,
 * saved back — as Markdown. Markdown and plain text qualify; so does a missing
 * type, which is what an empty (204) body carries.
 */
export const isTextContent = (contentType: string): boolean => {
  const mediaType = contentType.split(';')[0].trim().toLowerCase();
  return mediaType === '' || mediaType === 'text/markdown' || mediaType === 'text/x-markdown' || mediaType === 'text/plain';
};
