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
 * Reading a response safely.
 *
 * A response is untrusted input of unknown size and type. It is read up to a
 * fixed ceiling and never past it, decoded only when it is really text, and
 * handed on as a plain string — the console renders it as text, never as markup.
 */

/** Most of a response body the console will hold or show. */
export const MAX_DISPLAY_BYTES = 2 * 1024 * 1024;

/** Largest request body (file or text) the console will read into memory to send. */
export const MAX_REQUEST_BODY_BYTES = 16 * 1024 * 1024;

/** Content types that are text whatever the bytes happen to look like. */
const TEXT_TYPES = /^(text\/|application\/(json|xml|javascript|x-www-form-urlencoded|yaml|x-yaml|graphql)|[^;]*\+(json|xml|yaml))/i;

/** Content types that are never shown as text. */
const BINARY_TYPES = /^(image|audio|video|font)\/|^application\/(octet-stream|pdf|zip|gzip|x-tar|vnd\.|msword)/i;

export type ReadBody = {
  bytes: Uint8Array;
  /** The body was longer than the ceiling and was cut. */
  truncated: boolean;
  /** Bytes actually received, which may exceed `bytes.length` only when truncated. */
  totalSeen: number;
};

/** Reads up to `limit` bytes of a body, cancelling the stream once the ceiling is hit. */
export const readLimited = async (response: Response, limit: number): Promise<ReadBody> => {
  const reader = response.body?.getReader();
  if (!reader) {
    // No stream (an empty body, or a runtime without streaming): read it whole, then cap.
    const all = new Uint8Array(await response.arrayBuffer());
    return { bytes: all.slice(0, limit), truncated: all.length > limit, totalSeen: all.length };
  }

  const chunks: Uint8Array[] = [];
  let kept = 0;
  let seen = 0;
  let truncated = false;

  for (;;) {
    const { done, value } = await reader.read();
    if (done) break;
    seen += value.length;
    if (kept + value.length > limit) {
      chunks.push(value.slice(0, limit - kept));
      kept = limit;
      truncated = true;
      await reader.cancel();
      break;
    }
    chunks.push(value);
    kept += value.length;
  }

  const bytes = new Uint8Array(kept);
  let offset = 0;
  chunks.forEach((chunk) => {
    bytes.set(chunk, offset);
    offset += chunk.length;
  });
  return { bytes, truncated, totalSeen: seen };
};

/** Decodes bytes as UTF-8 text, or reports they are binary. */
export const decodeBody = (
  bytes: Uint8Array,
  contentType: string | null,
): { text: string; binary: boolean } => {
  if (bytes.length === 0) return { text: '', binary: false };
  const type = (contentType ?? '').trim();
  if (BINARY_TYPES.test(type)) return { text: '', binary: true };

  try {
    // A body cut mid-character must not turn a text response into "binary".
    return { text: new TextDecoder('utf-8', { fatal: !TEXT_TYPES.test(type) }).decode(bytes), binary: false };
  } catch {
    return { text: '', binary: true };
  }
};
